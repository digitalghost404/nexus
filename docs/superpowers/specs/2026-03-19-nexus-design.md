# Nexus — Design Spec

**Date:** 2026-03-19
**Status:** Draft
**Author:** xcoleman + Claude

## Overview

Nexus is a Go CLI tool that gives you a single pane of glass into all your projects and Claude Code sessions. It answers two questions: "what's the state of everything?" and "what did I do last?"

It combines a project health scanner with automatic Claude session logging, backed by SQLite, queryable from any directory.

## Goals

- Track the health and status of all projects across configured directories
- Automatically capture Claude Code session activity (files changed, commits, summaries)
- Provide a smart CLI that surfaces what needs attention
- Searchable history across all sessions and projects
- Zero friction — works from any directory, auto-discovers projects, minimal setup

## Non-Goals (for v1)

- No web dashboard — CLI only
- No Obsidian export — deferred to post-launch
- No real-time notifications or alerts
- No multi-machine sync

## Architecture

### Approach: Shell Wrapper + Periodic Scanner

Two capture mechanisms:

1. **Shell wrapper** — a bash function wrapping the `claude` command that runs `nexus capture` after Claude exits. Captures session data in real time. This is NOT a native Claude Code hook — it's a shell-level integration.
2. **Periodic scanner** (`nexus scan`) — crawls configured root directories, updates project health data. Run manually or via cron. Acts as a safety net for missed captures.

No persistent daemon. Wrapper fires on session exit, scanner runs on schedule.

#### Why a shell wrapper (not a Claude Code hook)

Claude Code does not expose a documented post-session hook API. The shell wrapper approach is the most reliable alternative:

```bash
# Added to ~/.bashrc by `nexus init`
claude() { command claude "$@"; nexus capture --dir "$PWD"; }
```

This fires after every Claude session exit, regardless of how the session ended. It's simple, debuggable, and doesn't depend on Claude's internal architecture.

**Limitation:** Sessions started from IDEs or other non-shell contexts won't be captured by the wrapper. The periodic scanner compensates by detecting git activity that doesn't correspond to any recorded session.

### Components

```
┌─────────────────────────────────────────────────┐
│                    CLI Interface                 │
│  nexus | projects | sessions | search | show    │
│  note | scan | config | init                    │
├─────────────────────────────────────────────────┤
│              Query & Format Layer                │
├──────────────────────┬──────────────────────────┤
│   Session Capture    │    Project Scanner        │
│   (shell wrapper)    │    (cron/manual)          │
├──────────────────────┴──────────────────────────┤
│              SQLite (~/.nexus/nexus.db)          │
│              (WAL mode for concurrent access)    │
└─────────────────────────────────────────────────┘
```

## Session Capture

### Trigger

Shell wrapper calls `nexus capture --dir $PWD` after `claude` exits.

### Data captured per session

| Field | Source |
|-------|--------|
| `project_path` | Working directory (from `$PWD`) |
| `project_name` | Directory name |
| `started_at` | From `~/.claude/sessions/<id>` `startedAt` field, or most recent JSONL first entry |
| `ended_at` | Timestamp of last JSONL entry in session transcript |
| `duration_secs` | Calculated from started_at/ended_at |
| `files_changed` | Git diff at session end |
| `commits_made` | Git log during session window |
| `summary` | Generated (see below) |
| `tags` | Auto-detected |
| `source` | "wrapper", "scan", or "manual" |
| `claude_session_id` | UUID from Claude session metadata (for correlation back to raw transcripts) |

### Claude session data format

Claude stores session data in two locations:
- `~/.claude/sessions/<id>` — minimal metadata: `{pid, sessionId, cwd, startedAt}`. No end timestamp.
- `~/.claude/projects/<path-slug>/<session-id>.jsonl` — full conversation transcript as JSONL. Each line is a message object with type, content, timestamp, cwd, git branch.

**Important:** The JSONL files can be large (thousands of lines per session). Parsing them is non-trivial and the format is undocumented/may change. Nexus treats this as an opportunistic data source, not a dependency.

### How `nexus capture` resolves timestamps

1. Look for the most recent session in `~/.claude/sessions/` matching the current `$PWD`
2. Read `startedAt` from session metadata for `started_at`
3. Read the last line of the corresponding JSONL transcript for `ended_at`
4. If Claude session data is unavailable, fall back to: `started_at` = earliest commit in the last reasonable window (e.g., 8 hours), `ended_at` = now

### Summary generation (layered)

1. **Git-based (always available)** — commits and diffs from the session window. Reliable baseline.
2. **Claude session data (opportunistic)** — parsed from JSONL transcript files. Look for tool-use messages (file edits, bash commands) to extract richer context. This is fragile — format may change, files can be large. Fail gracefully.
3. **Manual annotation (optional)** — `nexus note "message"` for user-supplied context.

Capture merges all available layers. Worst case: list of commits. Best case: rich summary with reasoning.

### Auto-tagging

Tags are derived, not manually maintained:
- Project directory name (e.g., `wraith`)
- Languages from file extensions (e.g., `go`, `typescript`)
- Configurable tag mappings in config

### Safety net

When `nexus scan` runs, it checks git log for commits that don't correspond to any recorded session. Creates "inferred" session records (`source: "scan"`) from gaps. Less rich than hook-captured sessions but ensures no activity is lost.

## Project Scanner

### Behavior

- Walks configured root directories looking for `.git/` folders
- Registers new projects automatically
- Respects exclusion patterns from config
- Updates all tracked projects on each run
- Marks disappeared projects as `archived`

### Data tracked per project

| Field | Source |
|-------|--------|
| `path` | Absolute path |
| `name` | Directory name |
| `branch` | Current git branch |
| `dirty_files` | Count of uncommitted files |
| `last_commit_at` | Most recent commit timestamp |
| `last_commit_msg` | Most recent commit message |
| `ahead` / `behind` | Commits vs remote |
| `status` | Calculated health status |
| `languages` | File extension analysis |
| `stale_branches` | Computed on-the-fly from git (not stored) |

### Health status

Two orthogonal dimensions:

**Activity status** (time-based, mutually exclusive):

| Status | Condition |
|--------|-----------|
| **Active** | Session or commit in last 3 days |
| **Idle** | Last activity 3–14 days ago |
| **Stale** | Last activity 14+ days ago |

**Dirty flag** (boolean, independent of activity status):

A project can be Active+Dirty, Stale+Dirty, etc. The `dirty` field is a separate boolean column, not part of the status enum. In CLI output, dirty projects are surfaced prominently regardless of their activity status.

Thresholds configurable in `~/.nexus/config.yaml`.

## CLI Interface

### `nexus` (no args) — Smart Summary

Context-aware: prioritizes current project if run inside one, shows everything regardless.

```
┌ NEXUS ─────────────────────────────────────
│
│  ⚠ 2 projects with uncommitted changes
│  wraith      main  3 dirty files  (last session: 2h ago)
│  cortex      main  1 dirty file   (last session: yesterday)
│
│  Recent Sessions
│  wraith    15:30  "Added retry logic to DNS scanner"
│  nexus     13:00  "Initial project scaffolding"
│  cortex    yesterday  "Fixed sync conflict resolution"
│
│  Stale (14+ days)
│  vibe-chatbot, cosmic-timeline, probe-zero-site
│
└────────────────────────────────────────────
```

### `nexus projects`

```
nexus projects              # all projects
nexus projects --active     # only active
nexus projects --dirty      # only with uncommitted changes
nexus projects --stale      # idle/stale only
```

### `nexus sessions`

```
nexus sessions                    # last 10 sessions
nexus sessions --project wraith   # filter by project
nexus sessions --since 7d         # last 7 days
nexus sessions --today            # today only
nexus sessions wraith             # shorthand for --project
```

### `nexus search`

```
nexus search "DNS retry"          # full-text across sessions and notes
nexus search --project wraith     # scoped to project
nexus search --files "*.go"       # sessions that touched Go files
```

### `nexus show <project>` / `nexus <project>`

Deep dive on a single project: branch, status, recent commits, recent sessions, stale branches, dirty files.

`nexus wraith` works as shorthand — CLI detects when first arg is a known project name. **Subcommand names always take precedence** — if a project is named `config`, `scan`, etc., use `nexus show config` explicitly.

### `nexus note "message"`

Attaches a manual note to the current directory's project (or global if not in a project).

### `nexus scan`

```
nexus scan            # scan all roots
nexus scan --verbose  # show discovery details
```

### `nexus config`

```
nexus config roots add ~/other-projects
nexus config exclude add "*/scratch-*"
nexus config show
```

### `nexus init`

First-time setup:
- Creates `~/.nexus/` directory and SQLite database
- Displays shell wrapper installation instructions (the `claude()` function for `.bashrc`)
- Does not auto-modify shell config — outputs the exact lines to copy/paste
- Runs initial scan to discover existing projects
- Generates a cron line from `scan_interval` config value and displays it for installation

## Data Model

### SQLite Schema

Database at `~/.nexus/nexus.db`. Uses WAL mode for safe concurrent access (e.g., `nexus capture` and `nexus scan` running simultaneously). Versioned from day one using `PRAGMA user_version`.

```sql
PRAGMA journal_mode=WAL;
PRAGMA user_version = 1;
```

**`projects`**
```sql
id              INTEGER PRIMARY KEY
name            TEXT NOT NULL
path            TEXT NOT NULL UNIQUE
languages       TEXT DEFAULT '[]'       -- JSON array
branch          TEXT
dirty           INTEGER DEFAULT 0       -- boolean: has uncommitted changes
dirty_files     INTEGER DEFAULT 0
last_commit_at  DATETIME
last_commit_msg TEXT
ahead           INTEGER DEFAULT 0
behind          INTEGER DEFAULT 0
status          TEXT NOT NULL DEFAULT 'active'  -- "active", "idle", "stale", "archived"
discovered_at   DATETIME NOT NULL
last_scanned_at DATETIME
```

**`sessions`**
```sql
id                  INTEGER PRIMARY KEY
project_id          INTEGER NOT NULL REFERENCES projects(id)
claude_session_id   TEXT                -- UUID from Claude metadata, nullable
started_at          DATETIME
ended_at            DATETIME
duration_secs       INTEGER
summary             TEXT
files_changed       TEXT DEFAULT '[]'   -- JSON array
commits_made        TEXT DEFAULT '[]'   -- JSON array of {hash, message}
tags                TEXT DEFAULT '[]'   -- JSON array
source              TEXT NOT NULL       -- "wrapper", "scan", "manual"
```

**`notes`**
```sql
id              INTEGER PRIMARY KEY
project_id      INTEGER REFERENCES projects(id)  -- nullable for global notes
session_id      INTEGER REFERENCES sessions(id)  -- nullable
content         TEXT NOT NULL
created_at      DATETIME NOT NULL
```

**`sessions_fts` (FTS5 virtual table)**
```sql
CREATE VIRTUAL TABLE sessions_fts USING fts5(
    summary,
    content=sessions,
    content_rowid=id
);

-- Triggers to keep FTS in sync
CREATE TRIGGER sessions_ai AFTER INSERT ON sessions BEGIN
    INSERT INTO sessions_fts(rowid, summary) VALUES (new.id, new.summary);
END;
CREATE TRIGGER sessions_ad AFTER DELETE ON sessions BEGIN
    INSERT INTO sessions_fts(sessions_fts, rowid, summary) VALUES ('delete', old.id, old.summary);
END;
CREATE TRIGGER sessions_au AFTER UPDATE ON sessions BEGIN
    INSERT INTO sessions_fts(sessions_fts, rowid, summary) VALUES ('delete', old.id, old.summary);
    INSERT INTO sessions_fts(rowid, summary) VALUES (new.id, new.summary);
END;
```

**`notes_fts` (FTS5 virtual table)**
```sql
CREATE VIRTUAL TABLE notes_fts USING fts5(
    content_col,
    content=notes,
    content_rowid=id
);

-- Same trigger pattern as sessions_fts
```

**Note on stale branches:** Stale branches are computed on-the-fly during `nexus show` and `nexus scan` from git data rather than stored in a dedicated table. This avoids schema/sync complexity for data that is cheap to recompute.

### Indexes

- `sessions(project_id, started_at)` — project-scoped session queries
- `sessions(started_at)` — time-range queries
- `projects(status)` — filtered project lists

### Schema versioning

Each release checks `PRAGMA user_version` and runs migrations if needed. Migrations are embedded in the binary as sequential SQL files.

## Configuration

### `~/.nexus/config.yaml`

```yaml
# Directories to scan for projects
roots:
  - ~/projects-wsl
  - ~/obsidian-vault

# Patterns to exclude from discovery
exclude:
  - "*/node_modules/*"
  - "*/vendor/*"
  - "*/.cache/*"
  - "*/go/pkg/*"
  - "*/snap/*"
  - "*/.nvm/*"

# Staleness thresholds (days)
thresholds:
  idle: 3
  stale: 14

# Scan schedule (used by cron setup)
scan_interval: 30m

# Obsidian export (future)
# obsidian:
#   vault_path: ~/obsidian-vault/dtg404-vault
#   export_folder: nexus-sessions
```

### Design decisions

- YAML over JSON — easier to hand-edit, supports comments
- Sensible defaults baked into binary — config file optional
- Explicit roots only (no `~` scanning) — wrapper captures sessions from any directory regardless
- `scan_interval` is used by `nexus init` to generate the cron line
- Obsidian config stubbed but commented — ready when needed

## Logging & Debugging

- `nexus --debug` flag on any command writes verbose output to stderr
- `~/.nexus/nexus.log` captures errors from wrapper-triggered captures (which run unattended)
- Log rotation: keep last 1MB, no external dependency
- Errors in capture/scan never crash — log and continue

## Auto-Discovery

- Scans configured roots for `.git/` directories
- New projects registered automatically
- Hooks capture sessions from ANY directory, even outside scan roots
- Scanner enriches hook-discovered projects with health data on next run

## Context Awareness

All commands work from any directory. When run inside a recognized project:
- `nexus` prioritizes that project's info at the top
- `nexus note` auto-associates with that project
- No command ever fails due to wrong directory

## Testing Strategy

### Unit tests
- Database layer — CRUD, FTS queries, edge cases
- Scanner — directory walking, git detection, exclusion matching, git status parsing
- Capture — session creation, diff parsing, summary generation, auto-tagging
- CLI output — formatting, context awareness

### Integration tests
- Full scan cycle with temp repos and known git state
- Capture pipeline with simulated sessions
- Search with pre-populated test data

### Approach
- Real temp directories with real git repos — no filesystem mocking
- No UI tests (CLI only)
- No benchmarking at launch — optimize if scanning feels slow

## Tech Stack

- **Language:** Go
- **Database:** SQLite (via `modernc.org/sqlite` for pure Go, no CGO)
- **CLI framework:** `cobra` (standard for Go CLIs)
- **Config:** `viper` (YAML parsing, defaults)
- **Distribution:** Single binary via `go install`

## Future (post-v1)

- `nexus export` — push session summaries to Obsidian vault
- Dashboard — optional web UI if CLI proves insufficient
- AI-powered summaries — route through Ollama for richer session descriptions
- Cross-machine sync — replicate SQLite or export/import
