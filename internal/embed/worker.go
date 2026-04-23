package embed

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/digitalghost404/nexus/internal/db"
)

const (
	embedPollInterval = 30 * time.Second
	embedBatchSize    = 10
	maxContentChars   = 30000
)

type Worker struct {
	client   *Client
	db       *sql.DB
	stop     chan struct{}
	stopOnce sync.Once
}

func NewWorker(client *Client, database *db.DB) *Worker {
	var dbConn *sql.DB
	if database != nil {
		dbConn = database.Conn()
	}
	return &Worker{
		client: client,
		db:     dbConn,
		stop:   make(chan struct{}),
	}
}

func (w *Worker) Start() {
	go w.run()
}

func (w *Worker) CheckModelCompatibility() error {
	if w.db == nil {
		return nil
	}
	var existingModel sql.NullString
	err := w.db.QueryRow("SELECT DISTINCT embedding_model FROM embedding_meta WHERE embedding_model IS NOT NULL AND embedding_model != '' LIMIT 1").Scan(&existingModel)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return fmt.Errorf("check existing model: %w", err)
	}
	if existingModel.Valid && existingModel.String != w.client.Model() {
		return fmt.Errorf("model mismatch: existing=%s, configured=%s. Run 'nexus embed --migrate-model'", existingModel.String, w.client.Model())
	}
	return nil
}

func (w *Worker) Stop() {
	w.stopOnce.Do(func() { close(w.stop) })
}

func (w *Worker) run() {
	ticker := time.NewTicker(embedPollInterval)
	defer ticker.Stop()

	w.embedPending()

	for {
		select {
		case <-w.stop:
			return
		case <-ticker.C:
			w.embedPending()
		}
	}
}

type unembeddedItem struct {
	sourceType string
	sourceID   int64
	content    string
}

func (w *Worker) embedPending() {
	if w.db == nil {
		return
	}

	if !w.isOllamaAvailable() {
		return
	}

	items, err := w.getUnembeddedItems(embedBatchSize)
	if err != nil {
		log.Printf("embed worker: get unembedded items: %v", err)
		return
	}

	if len(items) == 0 {
		return
	}

	texts := make([]string, len(items))
	for i, item := range items {
		content, _ := TruncateContent(item.content, maxContentChars)
		texts[i] = content
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	vecs, err := w.client.EmbedBatch(ctx, texts)
	if err != nil {
		for retry := 0; retry < 3; retry++ {
			delay := time.Duration(1<<uint(retry)) * time.Second
			if delay > 30*time.Second {
				delay = 30 * time.Second
			}
			log.Printf("embed worker: retry %d/%d after %v: %v", retry+1, 3, delay, err)
			select {
			case <-time.After(delay):
			case <-w.stop:
				return
			}
			vecs, err = w.client.EmbedBatch(ctx, texts)
			if err == nil {
				break
			}
		}
		if err != nil {
			log.Printf("embed worker: all retries exhausted, marking %d items as failed: %v", len(items), err)
			w.markFailed(items)
			return
		}
	}

	for i, item := range items {
		if i >= len(vecs) {
			break
		}
		if err := w.storeEmbedding(item, vecs[i]); err != nil {
			log.Printf("embed worker: store embedding for %s %d: %v", item.sourceType, item.sourceID, err)
		}
	}
}

func (w *Worker) isOllamaAvailable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, w.client.baseURL+"/api/tags", nil)
	if err != nil {
		return false
	}
	resp, err := w.client.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode == http.StatusOK
}

func (w *Worker) markFailed(items []unembeddedItem) {
	if w.db == nil || len(items) == 0 {
		return
	}
	for _, item := range items {
		_, err := w.db.Exec(
			"UPDATE embedding_meta SET status = 'failed' WHERE source_type = ? AND source_id = ?",
			item.sourceType, item.sourceID,
		)
		if err != nil {
			log.Printf("embed worker: mark failed %s %d: %v", item.sourceType, item.sourceID, err)
		}
	}
}

func (w *Worker) getUnembeddedItems(limit int) ([]unembeddedItem, error) {
	query := `
		SELECT 'session' as source_type, s.id, s.summary as content
		FROM sessions s
		LEFT JOIN embedding_meta em ON em.source_type = 'session' AND em.source_id = s.id
		WHERE em.id IS NULL AND s.summary IS NOT NULL AND s.summary != ''
		UNION ALL
		SELECT 'note' as source_type, n.id, n.content
		FROM notes n
		LEFT JOIN embedding_meta em ON em.source_type = 'note' AND em.source_id = n.id
		WHERE em.id IS NULL AND n.content IS NOT NULL AND n.content != ''
		UNION ALL
		SELECT 'preference' as source_type, p.id, p.content
		FROM preferences p
		LEFT JOIN embedding_meta em ON em.source_type = 'preference' AND em.source_id = p.id
		WHERE em.id IS NULL AND p.content IS NOT NULL AND p.content != '' AND p.superseded_by IS NULL
		UNION ALL
		SELECT em.source_type, em.source_id,
			CASE em.source_type
				WHEN 'session' THEN (SELECT summary FROM sessions WHERE id = em.source_id)
				WHEN 'note' THEN (SELECT content FROM notes WHERE id = em.source_id)
				WHEN 'preference' THEN (SELECT content FROM preferences WHERE id = em.source_id)
			END
		FROM embedding_meta em
		WHERE em.status = 'pending'
		LIMIT ?
	`

	rows, err := w.db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("query unembedded: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var items []unembeddedItem
	for rows.Next() {
		var item unembeddedItem
		var idStr string
		if err := rows.Scan(&item.sourceType, &idStr, &item.content); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		item.sourceID, _ = strconv.ParseInt(idStr, 10, 64)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (w *Worker) storeEmbedding(item unembeddedItem, vec []float64) error {
	vecBlob := float64SliceToBlob(vec)
	content, truncated := TruncateContent(item.content, maxContentChars)
	contentHash := ContentHash(content)

	tx, err := w.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	result, err := tx.Exec("INSERT INTO embeddings (embedding) VALUES (?)", vecBlob)
	if err != nil {
		return fmt.Errorf("insert embedding: %w", err)
	}
	embID, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("get embedding id: %w", err)
	}

	_, err = tx.Exec(
		"INSERT INTO embedding_meta (id, source_type, source_id, content_hash, content_truncated, embedded_at, embedding_model, status) VALUES (?, ?, ?, ?, ?, ?, ?, 'done')",
		embID, item.sourceType, item.sourceID, contentHash, truncated, time.Now().UTC(), w.client.Model(),
	)
	if err != nil {
		return fmt.Errorf("insert meta: %w", err)
	}

	return tx.Commit()
}
