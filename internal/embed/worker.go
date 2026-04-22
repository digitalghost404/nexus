package embed

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/digitalghost404/nexus/internal/db"
)

type Worker struct {
	client *Client
	db     *sql.DB
	stop   chan struct{}
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

func (w *Worker) Stop() {
	close(w.stop)
}

func (w *Worker) run() {
	ticker := time.NewTicker(30 * time.Second)
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
	id         string
	content    string
}

func (w *Worker) embedPending() {
	if w.db == nil {
		return
	}

	items, err := w.getUnembeddedItems(10)
	if err != nil {
		log.Printf("embed worker: get unembedded items: %v", err)
		return
	}

	if len(items) == 0 {
		return
	}

	texts := make([]string, len(items))
	for i, item := range items {
		content, _ := TruncateContent(item.content, 8000)
		texts[i] = content
	}

	vecs, err := w.client.EmbedBatch(context.Background(), texts)
	if err != nil {
		log.Printf("embed worker: embed batch: %v", err)
		return
	}

	for i, item := range items {
		if i >= len(vecs) {
			break
		}
		if err := w.storeEmbedding(item, vecs[i]); err != nil {
			log.Printf("embed worker: store embedding for %s %s: %v", item.sourceType, item.id, err)
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
		LIMIT ?
	`

	rows, err := w.db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("query unembedded: %w", err)
	}
	defer rows.Close()

	var items []unembeddedItem
	for rows.Next() {
		var item unembeddedItem
		if err := rows.Scan(&item.sourceType, &item.id, &item.content); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (w *Worker) storeEmbedding(item unembeddedItem, vec []float64) error {
	vecBlob := float64SliceToBlob(vec)
	contentHash := ContentHash(item.content)

	tx, err := w.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec(
		"INSERT INTO embeddings (source_type, source_id, vector, created_at) VALUES (?, ?, ?, ?)",
		item.sourceType, item.id, vecBlob, time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("insert embedding: %w", err)
	}

	_, err = tx.Exec(
		"INSERT INTO embedding_meta (source_type, source_id, content_hash, embedded_at) VALUES (?, ?, ?, ?)",
		item.sourceType, item.id, contentHash, time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("insert meta: %w", err)
	}

	return tx.Commit()
}
