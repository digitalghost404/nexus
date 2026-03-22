-- internal/db/migration_v3.sql
-- Delta migration from schema v2 to v3

CREATE TABLE IF NOT EXISTS session_conversations (
    session_id INTEGER PRIMARY KEY REFERENCES sessions(id) ON DELETE CASCADE,
    digest     TEXT NOT NULL,
    parsed_at  DATETIME NOT NULL DEFAULT (datetime('now'))
);
