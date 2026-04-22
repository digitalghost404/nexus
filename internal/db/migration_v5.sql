-- Migration v4 -> v5: Persistent memory tables

CREATE TABLE IF NOT EXISTS preferences (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER REFERENCES projects(id),
    category TEXT NOT NULL CHECK(category IN ('workflow', 'style', 'tool', 'preference', 'pattern')),
    content TEXT NOT NULL,
    source TEXT NOT NULL DEFAULT 'stated' CHECK(source IN ('stated', 'observed', 'inferred')),
    confidence REAL NOT NULL DEFAULT 0.5,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_referenced_at DATETIME,
    access_count INTEGER NOT NULL DEFAULT 0,
    superseded_by INTEGER REFERENCES preferences(id),
    superseded_at DATETIME
);

CREATE INDEX idx_preferences_project ON preferences(project_id);
CREATE INDEX idx_preferences_category ON preferences(category);
CREATE INDEX idx_preferences_confidence ON preferences(confidence);

CREATE VIRTUAL TABLE preferences_fts USING fts5(
    content,
    content='preferences',
    content_rowid='id'
);

-- Triggers to keep preferences_fts in sync
CREATE TRIGGER preferences_ai AFTER INSERT ON preferences BEGIN
    INSERT INTO preferences_fts(rowid, content) VALUES (new.id, new.content);
END;

CREATE TRIGGER preferences_ad AFTER DELETE ON preferences BEGIN
    INSERT INTO preferences_fts(preferences_fts, rowid, content) VALUES('delete', old.id, old.content);
END;

CREATE TRIGGER preferences_au AFTER UPDATE ON preferences BEGIN
    INSERT INTO preferences_fts(preferences_fts, rowid, content) VALUES('delete', old.id, old.content);
    INSERT INTO preferences_fts(rowid, content) VALUES (new.id, new.content);
END;

CREATE TABLE IF NOT EXISTS embedding_meta (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source_type TEXT NOT NULL CHECK(source_type IN ('session', 'note', 'preference')),
    source_id INTEGER NOT NULL,
    content_hash TEXT NOT NULL,
    content_truncated BOOLEAN NOT NULL DEFAULT 0,
    embedded_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    embedding_model TEXT NOT NULL DEFAULT 'nomic-embed-text',
    UNIQUE(source_type, source_id)
);

CREATE INDEX idx_embedding_meta_source ON embedding_meta(source_type, source_id);
CREATE INDEX idx_embedding_meta_model ON embedding_meta(embedding_model);

CREATE TABLE IF NOT EXISTS embeddings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    embedding BLOB NOT NULL
);
