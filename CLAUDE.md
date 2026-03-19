## Nexus — Project Health & Session Tracker

### Project Overview
Go CLI tool that tracks project health and Claude Code sessions from a single command.
- **Spec:** `docs/superpowers/specs/2026-03-19-nexus-design.md`
- **Plan:** `docs/superpowers/plans/2026-03-19-nexus-implementation.md`

### Tech Stack
- **Core:** Go 1.22+, Cobra CLI
- **Database:** SQLite via modernc.org/sqlite (pure Go, no CGO)
- **Config:** gopkg.in/yaml.v3
- **Distribution:** Single binary via `go install`

### Architecture
Single Go binary, no daemon. Shell wrapper + periodic scanner.
- `main.go` — entry point
- `cmd/` — Cobra command definitions (thin, delegates to internal packages)
- `internal/db/` — SQLite schema, migrations, CRUD for projects/sessions/notes, FTS5
- `internal/scanner/` — Git operations, directory walking, project discovery
- `internal/capture/` — Session capture orchestrator, Claude session reader, summary generation
- `internal/config/` — YAML config loading, defaults, path expansion
- `internal/display/` — CLI output formatting, tables, smart summary
- `internal/logger/` — Debug stderr, file logging, rotation

### Conventions
- **TDD:** Write failing test first, implement, verify, commit.
- **SQL:** Hand-written database/sql queries, embedded schema.sql
- **Errors:** Wrap with `fmt.Errorf("context: %w", err)`
- **Tests:** Use `t.TempDir()` for test databases, `scanner.CreateTestRepo()` for git repos
- **Commits:** One commit per task, descriptive message
- **JSON arrays:** Stored as TEXT columns with `DEFAULT '[]'`

### Data
- Config: `~/.nexus/config.yaml`
- Database: `~/.nexus/nexus.db` (WAL mode)
- Logs: `~/.nexus/nexus.log`

### Commands
```
go build -o nexus .    # Build
go test ./... -v       # Run all tests
go install .           # Install to ~/go/bin/
nexus init             # First-time setup
nexus scan --verbose   # Scan projects
nexus                  # Smart summary
```

## Workflow Orchestration

### 1. Plan Mode Default
- Enter plan mode for ANY non-trivial task (3+ steps or architectural decisions)
- If something goes sideways, STOP and re-plan immediately

### 2. Homebrew default installer
- Use homebrew as the default installation for tools/dependencies when available

### 3. Subagent Strategy
- Use subagents liberally to keep main context window clean
- One task per subagent for focused execution

### 4. Verification Before Done
- Never mark a task complete without proving it works
- Run tests, check logs, demonstrate correctness

### 5. Core Principles
- **Simplicity First**: Make every change as simple as possible
- **No Laziness**: Find root causes. No temporary fixes.
- **Minimal Impact**: Changes should only touch what's necessary
- **Latest Versions**: Always use latest stable versions for packages
