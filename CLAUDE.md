# nexus

nexus is a Go CLI tool that gives AI agents persistent memory across sessions. It captures session context, git state, notes, preferences, and semantic embeddings, storing them locally so future sessions can resume with full awareness of past work.

## Tech Stack

- **Language:** Go
- **Database:** SQLite via `modernc.org/sqlite` (pure Go, no CGO required)
- **CLI framework:** Cobra
- **Embeddings:** Ollama API (`nomic-embed-text` model) with cosine similarity search
- **Full-text search:** SQLite FTS5 for keyword-based preference/session/note search

## Key Directories

| Directory | Purpose |
|---|---|
| `cmd/` | CLI command definitions (Cobra commands) |
| `internal/capture/` | Session capture logic |
| `internal/scanner/` | Git operations and repo scanning |
| `internal/db/` | Database layer (SQLite reads/writes, migrations v1→v5) |
| `internal/config/` | Configuration management with agent isolation |
| `internal/display/` | Output formatting and display helpers |
| `internal/embed/` | Embedding client (Ollama), worker goroutine, vector serialization |
| `internal/context/` | 3-pass context builder (project state, semantic recall, preferences) |

## Build & Test

```bash
# Build and install
go build -o nexus . && cp nexus ~/.local/bin/nexus

# Run tests
go test ./...
```

## Persistent Memory Features

### Agent Isolation

Each AI agent gets its own database under `~/.nexus/<agent>/`:
- `claude` (default): `~/.nexus/claude/nexus.db`
- `opencode`: `~/.nexus/opencode/nexus.db`

Use `--agent` or `-a` flag to select: `nexus --agent opencode remember "..."`

### New Commands

| Command | Description |
|---|---|
| `nexus remember <content>` | Save a preference/pattern/decision |
| `nexus recall <query>` | Semantic search across sessions, notes, preferences |
| `nexus inject <project>` | Build smart context (3-pass: state, recall, preferences) |
| `nexus embed --backfill` | Process all pending embeddings |
| `nexus maintain` | Run decay, prune, vacuum maintenance |
| `nexus preferences` | List preferences |
| `nexus serve` | Start HTTP API server with embed worker |

### API Endpoints (nexus serve)

| Endpoint | Method | Purpose |
|---|---|---|
| `/api/capture` | POST | Capture session from project directory |
| `/api/notes` | GET, POST | List/create notes |
| `/api/preferences` | GET, POST | List/create preferences |
| `/api/preferences/{id}` | PATCH, DELETE | Update/delete preference |
| `/api/recall` | POST | Semantic search |
| `/api/inject` | POST | Build smart context |
| `/api/embed/status` | GET | Embedding queue status |

### Schema

Current schema version: **5**. Adds `preferences`, `preferences_fts`, `embedding_meta`, and `embeddings` tables.

## Data Storage

All data lives in `~/.nexus/<agent>/`:
- `nexus.db` — SQLite database (sessions, notes, git snapshots, preferences, embeddings)
- `config.yaml` — user configuration
- `nexus.log` — application log
