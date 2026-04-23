-- Migration v5 -> v6: Add embedding status tracking and model metadata

ALTER TABLE embedding_meta ADD COLUMN status TEXT NOT NULL DEFAULT 'pending';
ALTER TABLE embedding_meta ADD COLUMN model_name TEXT;
ALTER TABLE embedding_meta ADD COLUMN dimensions INTEGER;

UPDATE embedding_meta SET status = 'done'
WHERE id IN (SELECT id FROM embeddings);
