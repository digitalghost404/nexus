-- v4: Change DATETIME columns to TEXT to fix modernc.org/sqlite driver issues
-- The driver tries to auto-convert DATETIME columns, bypassing custom Scanners

BEGIN TRANSACTION;

CREATE TABLE IF NOT EXISTS projects_new (
    id              INTEGER PRIMARY KEY,
    name            TEXT NOT NULL,
    path            TEXT NOT NULL UNIQUE,
    languages       TEXT DEFAULT '[]',
    branch          TEXT,
    dirty           INTEGER DEFAULT 0,
    dirty_files     INTEGER DEFAULT 0,
    last_commit_at  TEXT,
    last_commit_msg TEXT,
    ahead           INTEGER DEFAULT 0,
    behind          INTEGER DEFAULT 0,
    status          TEXT NOT NULL DEFAULT 'active',
    discovered_at   TEXT NOT NULL,
    last_scanned_at TEXT
);

INSERT INTO projects_new SELECT * FROM projects;
DROP TABLE projects;
ALTER TABLE projects_new RENAME TO projects;
CREATE INDEX IF NOT EXISTS idx_projects_status ON projects(status);

COMMIT;
