package db

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"math"
	"sort"
	"time"
)

type EmbeddingResult struct {
	ID         int64
	SourceType string
	SourceID   int64
	Score      float64
	Content    string
}

func (d *DB) StoreEmbedding(sourceType string, sourceID int64, content string, vec []float64, model string, truncated bool) error {
	blob := float64SliceToBlob(vec)

	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	result, err := tx.Exec("INSERT INTO embeddings (embedding) VALUES (?)", blob)
	if err != nil {
		return err
	}
	embID, _ := result.LastInsertId()

	content, wasTruncated := truncateContent(content, 30000)
	hash := contentHash(content)

	_, err = tx.Exec(
		`INSERT INTO embedding_meta (id, source_type, source_id, content_hash, content_truncated, embedded_at, embedding_model)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		embID, sourceType, sourceID, hash, truncated || wasTruncated, time.Now(), model,
	)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (d *DB) SearchSimilar(queryVec []float64, sourceType string, limit int, minScore float64) ([]EmbeddingResult, error) {
	rows, err := d.db.Query(
		`SELECT em.source_type, em.source_id, e.id
		 FROM embeddings e
		 JOIN embedding_meta em ON e.id = em.id
		 WHERE em.source_type = ?`,
		sourceType,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type candidate struct {
		embID      int64
		sourceType string
		sourceID   int64
		score      float64
	}
	var candidates []candidate

	for rows.Next() {
		var embID int64
		var st string
		var sourceID int64

		if err := rows.Scan(&st, &sourceID, &embID); err != nil {
			continue
		}

		var blob []byte
		if err := d.db.QueryRow("SELECT embedding FROM embeddings WHERE id = ?", embID).Scan(&blob); err != nil {
			continue
		}

		vec := blobToFloat64Slice(blob)
		if vec == nil {
			continue
		}

		score := CosineSimilarity(queryVec, vec)
		if score >= minScore {
			candidates = append(candidates, candidate{embID: embID, sourceType: st, sourceID: sourceID, score: score})
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	if limit > 0 && len(candidates) > limit {
		candidates = candidates[:limit]
	}

	results := make([]EmbeddingResult, len(candidates))
	for i, c := range candidates {
		results[i] = EmbeddingResult{
			ID:         c.embID,
			SourceType: c.sourceType,
			SourceID:   c.sourceID,
			Score:      c.score,
		}
	}
	return results, nil
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
	if len(content) <= maxChars {
		return content, false
	}
	return content[:maxChars], true
}

func contentHash(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:])
}
