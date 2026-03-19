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

### Approach: Hooks + Periodic Scanner

Two capture mechanisms:

1. **Claude Code post-session hook** — triggers `nexus capture` when a Claude session ends. Captures session data in real time.
2. **Periodic scanner** (`nexus scan`) — crawls configured root directories, updates project health data. Run manually or via cron. Acts as a safety net for missed hooks.

No persistent daemon. Hook fires on events, scanner runs on schedule.

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
│   (hook-triggered)   │    (cron/manual)          │
├──────────────────────┴──────────────────────────┤
│              SQLite (~/.nexus/nexus.db)          │
└─────────────────────────────────────────────────┘
```

## Session Capture

### Trigger

Claude Code post-session hook calls `nexus capture --dir $PWD`.

### Data captured per session

| Field | Source |
|-------|--------|
| `project_path` | Working directory |
| `project_name` | Directory name |
| `started_at` | Hook/inferred timestamp |
| `ended_at` | Hook timestamp |
| `duration_secs` | Calculated |
| `files_changed` | Git diff at session end |
| `commits_made` | Git log during session window |
| `summary` | Generated (see below) |
| `tags` | Auto-detected |
| `source` | "hook", "scan", or "manual" |

### Summary generation (layered)

1. **Git-based (always available)** — commits and diffs from the session window. Reliable baseline.
2. **Claude session data (opportunistic)** — parsed from `~/.claude/` session files if accessible. Richer context (the "why" behind changes). Fragile — format may change.
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
| `stale_branches` | Branches with no activity 7+ days |

### Health status

| Status | Condition |
|--------|-----------|
| **Active** | Session or commit in last 3 days |
| **Idle** | Last activity 3–14 days ago |
| **Stale** | Last activity 14+ days ago |
| **Dirty** | Uncommitted changes (any age) |

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

`nexus wraith` works as shorthand — CLI detects when first arg is a known project name.

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
- Displays Claude Code hook installation instructions (does not auto-modify Claude config)
- Runs initial scan to discover existing projects
- Optionally displays cron setup instructions

## Data Model

### SQLite Schema

Database at `~/.nexus/nexus.db`. Versioned from day one using `PRAGMA user_version`.

**`projects`**
```sql
id              INTEGER PRIMARY KEY
name            TEXT
path            TEXT UNIQUE
languages       TEXT            -- JSON array
branch          TEXT
dirty_files     INTEGER
last_commit_at  DATETIME
last_commit_msg TEXT
ahead           INTEGER
behind          INTEGER
status          TEXT            -- "active", "idle", "stale", "archived"
discovered_at   DATETIME
last_scanned_at DATETIME
```

**`sessions`**
```sql
id              INTEGER PRIMARY KEY
project_id      INTEGER REFERENCES projects(id)
started_at      DATETIME
ended_at        DATETIME
duration_secs   INTEGER
summary         TEXT
files_changed   TEXT            -- JSON array
commits_made    TEXT            -- JSON array of {hash, message}
tags            TEXT            -- JSON array
source          TEXT            -- "hook", "scan", "manual"
```

**`notes`**
```sql
id              INTEGER PRIMARY KEY
project_id      INTEGER REFERENCES projects(id)  -- nullable
session_id      INTEGER REFERENCES sessions(id)  -- nullable
content         TEXT
created_at      DATETIME
```

**`stale_branches`**
```sql
id              INTEGER PRIMARY KEY
project_id      INTEGER REFERENCES projects(id)
branch_name     TEXT
last_commit_at  DATETIME
last_scanned_at DATETIME
```

### Indexes

- `sessions(project_id, started_at)` — project-scoped session queries
- `sessions(started_at)` — time-range queries
- `projects(status)` — filtered project lists
- FTS5 index on `sessions.summary` and `notes.content` — powers `nexus search`

### Schema versioning

```sql
PRAGMA user_version = 1;
```

Each release checks version and runs migrations if needed.

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
- Explicit roots only (no `~` scanning) — hooks capture sessions from any directory regardless
- Obsidian config stubbed but commented — ready when needed

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
