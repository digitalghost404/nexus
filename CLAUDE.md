# nexus

nexus is a Go CLI tool that gives Claude Code persistent memory across sessions. It captures session context, git state, and notes, storing them locally so future sessions can resume with full awareness of past work.

## Tech Stack

- **Language:** Go
- **Database:** SQLite via `modernc.org/sqlite` (pure Go, no CGO required)
- **CLI framework:** Cobra

## Key Directories

| Directory | Purpose |
|---|---|
| `cmd/` | CLI command definitions (Cobra commands) |
| `internal/capture/` | Session capture logic |
| `internal/scanner/` | Git operations and repo scanning |
| `internal/db/` | Database layer (SQLite reads/writes) |
| `internal/config/` | Configuration management |
| `internal/display/` | Output formatting and display helpers |

## Build & Test

```bash
# Build and install
go build -o nexus . && cp nexus ~/.local/bin/nexus

# Run tests
go test ./...
```

## Data Storage

All data lives in `~/.nexus/`:
- `nexus.db` — SQLite database (sessions, notes, git snapshots)
- `config.yaml` — user configuration
- `nexus.log` — application log
