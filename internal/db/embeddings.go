package db

import (
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"sort"
	"time"
	"unicode/utf8"
)

type EmbeddingResult struct {
	ID         int64
	SourceType string
	SourceID   int64
	Score      float64
	Content    string
}

type candidate struct {
	embID      int64
	sourceType string
	sourceID   int64
	score      float64
}

func (d *DB) StoreEmbedding(sourceType string, sourceID int64, content string, vec []float64, model string, truncated bool) error {
	blob := float64SliceToBlob(vec)

	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	result, err := tx.Exec("INSERT INTO embeddings (embedding) VALUES (?)", blob)
	if err != nil {
		return fmt.Errorf("insert embedding: %w", err)
	}
	embID, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("get embedding id: %w", err)
	}

	content, wasTruncated := truncateContent(content, 30000)
	hash := contentHash(content)

	_, err = tx.Exec(
		`INSERT INTO embedding_meta (id, source_type, source_id, content_hash, content_truncated, embedded_at, embedding_model, status)
		 VALUES (?, ?, ?, ?, ?, ?, ?, 'done')`,
		embID, sourceType, sourceID, hash, truncated || wasTruncated, time.Now(), model,
	)
	if err != nil {
		return fmt.Errorf("insert meta: %w", err)
	}

	return tx.Commit()
}

func (d *DB) SearchSimilar(queryVec []float64, sourceType string, limit int, minScore float64) ([]EmbeddingResult, error) {
	rows, err := d.db.Query(
		`SELECT em.source_type, em.source_id, e.embedding
		 FROM embeddings e
		 JOIN embedding_meta em ON e.id = em.id
		 WHERE em.source_type = ?`,
		sourceType,
	)
	if err != nil {
		return nil, fmt.Errorf("query embeddings: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var candidates []candidate

	for rows.Next() {
		var st string
		var sourceID int64
		var blob []byte

		if err := rows.Scan(&st, &sourceID, &blob); err != nil {
			return nil, fmt.Errorf("scan embedding: %w", err)
		}

		vec := blobToFloat64Slice(blob)
		if vec == nil {
			continue
		}

		score := CosineSimilarity(queryVec, vec)
		if score >= minScore {
			candidates = append(candidates, candidate{sourceType: st, sourceID: sourceID, score: score})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate embeddings: %w", err)
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	if limit > 0 && len(candidates) > limit {
		candidates = candidates[:limit]
	}

	results := make([]EmbeddingResult, len(candidates))
	contentMap := batchGetContent(d.db, candidates)
	for i, c := range candidates {
		results[i] = EmbeddingResult{
			ID:         c.embID,
			SourceType: c.sourceType,
			SourceID:   c.sourceID,
			Score:      c.score,
			Content:    contentMap[fmt.Sprintf("%s:%d", c.sourceType, c.sourceID)],
		}
	}
	return results, nil
}

func batchGetContent(db *sql.DB, candidates []candidate) map[string]string {
	sources := make(map[string][]int64)
	for _, c := range candidates {
		sources[c.sourceType] = append(sources[c.sourceType], c.sourceID)
	}

	contentMap := make(map[string]string)
	columnMap := map[string]string{
		"session":    "summary",
		"note":       "content",
		"preference": "content",
	}
	tableMap := map[string]string{
		"session":    "sessions",
		"note":       "notes",
		"preference": "preferences",
	}

	for sourceType, ids := range sources {
		column := columnMap[sourceType]
		table := tableMap[sourceType]
		if column == "" || table == "" {
			continue
		}

		placeholders := make([]string, len(ids))
		args := make([]any, len(ids))
		for i, id := range ids {
			placeholders[i] = "?"
			args[i] = id
		}
		query := fmt.Sprintf("SELECT id, %s FROM %s WHERE id IN (%s)", column, table, joinStrings(placeholders, ","))

		rows, err := db.Query(query, args...)
		if err != nil {
			continue
		}
		for rows.Next() {
			var id int64
			var content string
			if err := rows.Scan(&id, &content); err == nil {
				contentMap[fmt.Sprintf("%s:%d", sourceType, id)] = content
			}
		}
		_ = rows.Close()
	}

	return contentMap
}

func joinStrings(strs []string, sep string) string {
	result := ""
	for i, s := range strs {
		if i > 0 {
			result += sep
		}
		result += s
	}
	return result
}

func CosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0
	}
	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

func float64SliceToBlob(vec []float64) []byte {
	blob := make([]byte, len(vec)*8)
	for i, v := range vec {
		binary.LittleEndian.PutUint64(blob[i*8:], math.Float64bits(v))
	}
	return blob
}

func blobToFloat64Slice(blob []byte) []float64 {
	if len(blob)%8 != 0 {
		return nil
	}
	vec := make([]float64, len(blob)/8)
	for i := range vec {
		vec[i] = math.Float64frombits(binary.LittleEndian.Uint64(blob[i*8:]))
	}
	return vec
}

func truncateContent(content string, maxChars int) (string, bool) {
	if utf8.RuneCountInString(content) <= maxChars {
		return content, false
	}
	runes := []rune(content)
	return string(runes[:maxChars]), true
}

func contentHash(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:])
}
