-- Note: PRAGMA journal_mode=WAL is set in db.go Open(), not here.

CREATE TABLE IF NOT EXISTS projects (
    id              INTEGER PRIMARY KEY,
    name            TEXT NOT NULL,
    path            TEXT NOT NULL UNIQUE,
    languages       TEXT DEFAULT '[]',
    branch          TEXT,
    dirty           INTEGER DEFAULT 0,
    dirty_files     INTEGER DEFAULT 0,
    last_commit_at  DATETIME,
    last_commit_msg TEXT,
    ahead           INTEGER DEFAULT 0,
    behind          INTEGER DEFAULT 0,
    status          TEXT NOT NULL DEFAULT 'active',
    discovered_at   DATETIME NOT NULL,
    last_scanned_at DATETIME
);

CREATE TABLE IF NOT EXISTS sessions (
    id                  INTEGER PRIMARY KEY,
    project_id          INTEGER NOT NULL REFERENCES projects(id),
    claude_session_id   TEXT,
    started_at          DATETIME,
    ended_at            DATETIME,
    duration_secs       INTEGER,
    summary             TEXT,
    files_changed       TEXT DEFAULT '[]',
    commits_made        TEXT DEFAULT '[]',
    tags                TEXT DEFAULT '[]',
    source              TEXT NOT NULL,
    created_at          DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS notes (
    id              INTEGER PRIMARY KEY,
    project_id      INTEGER REFERENCES projects(id),
    session_id      INTEGER REFERENCES sessions(id),
    content         TEXT NOT NULL,
    created_at      DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_sessions_project_started ON sessions(project_id, started_at);
CREATE INDEX IF NOT EXISTS idx_sessions_started ON sessions(started_at);
CREATE INDEX IF NOT EXISTS idx_projects_status ON projects(status);

-- FTS5 for full-text search
CREATE VIRTUAL TABLE IF NOT EXISTS sessions_fts USING fts5(
    summary,
    content=sessions,
    content_rowid=id
);

CREATE VIRTUAL TABLE IF NOT EXISTS notes_fts USING fts5(
    content,
    content=notes,
    content_rowid=id
);

-- Triggers to keep FTS in sync
CREATE TRIGGER IF NOT EXISTS sessions_ai AFTER INSERT ON sessions BEGIN
    INSERT INTO sessions_fts(rowid, summary) VALUES (new.id, new.summary);
END;
CREATE TRIGGER IF NOT EXISTS sessions_ad AFTER DELETE ON sessions BEGIN
    INSERT INTO sessions_fts(sessions_fts, rowid, summary) VALUES ('delete', old.id, old.summary);
END;
CREATE TRIGGER IF NOT EXISTS sessions_au AFTER UPDATE ON sessions BEGIN
    INSERT INTO sessions_fts(sessions_fts, rowid, summary) VALUES ('delete', old.id, old.summary);
    INSERT INTO sessions_fts(rowid, summary) VALUES (new.id, new.summary);
END;

CREATE TRIGGER IF NOT EXISTS notes_ai AFTER INSERT ON notes BEGIN
    INSERT INTO notes_fts(rowid, content) VALUES (new.id, new.content);
END;
CREATE TRIGGER IF NOT EXISTS notes_ad AFTER DELETE ON notes BEGIN
    INSERT INTO notes_fts(notes_fts, rowid, content) VALUES ('delete', old.id, old.content);
END;
CREATE TRIGGER IF NOT EXISTS notes_au AFTER UPDATE ON notes BEGIN
    INSERT INTO notes_fts(notes_fts, rowid, content) VALUES ('delete', old.id, old.content);
    INSERT INTO notes_fts(rowid, content) VALUES (new.id, new.content);
END;

-- v2 tables
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
