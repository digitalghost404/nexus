-- internal/db/migration_v2.sql
-- Delta migration from schema v1 to v2

CREATE TABLE IF NOT EXISTS project_links (
    id                INTEGER PRIMARY KEY,
    project_id        INTEGER NOT NULL REFERENCES projects(id),
    linked_project_id INTEGER NOT NULL REFERENCES projects(id),
    created_at        DATETIME NOT NULL DEFAULT (datetime('now')),
    UNIQUE(project_id, linked_project_id)
);

CREATE INDEX IF NOT EXISTS idx_project_links_project ON project_links(project_id);

CREATE TABLE IF NOT EXISTS session_tags (
    id          INTEGER PRIMARY KEY,
    session_id  INTEGER NOT NULL REFERENCES sessions(id),
    tag         TEXT NOT NULL,
    created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
    UNIQUE(session_id, tag)
);

CREATE INDEX IF NOT EXISTS idx_session_tags_tag ON session_tags(tag);
