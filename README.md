# Nexus

A CLI tool that gives AI agents persistent memory across sessions. One binary, no daemon, no dependencies beyond Go.

Nexus solves the biggest limitation of AI-assisted development: **context loss between sessions.** Every time you start a new AI coding conversation, the AI starts from zero — no memory of what you built yesterday, what decisions you made, or where you left off. Nexus fixes that by automatically tracking every session, every project, and every change, then making all of it instantly available in future conversations.

## Why Nexus Exists

AI coding tools are powerful within a single session, but across sessions they have no memory. You end up re-explaining your project, re-briefing context, and losing momentum. Nexus closes that gap:

- **Before Nexus:** "Hey Claude, last week we refactored auth and decided to use JWTs instead of session tokens. The migration is half done. Here's what's left..."
- **After Nexus:** Claude runs `nexus resume` and instantly knows what happened, what changed, and what's next.

Nexus answers three questions: **"What's the state of everything?"**, **"What did I do last?"**, and **"What should the AI know before we start?"**

## How It Makes AI Coding Better

### Persistent Memory for AI Sessions

Nexus automatically captures every coding session — what files were changed, what commits were made, and what the session accomplished. This data persists in a local SQLite database that survives across conversations. When you start a new session, the AI can query Nexus to understand:

- What you worked on recently across all your projects
- The current health and status of every tracked project
- What files were modified, what branches are active, and what's dirty
- Full-text searchable history of all past sessions and notes

### Semantic Search & Smart Context

Nexus goes beyond keyword search with vector embeddings powered by local Ollama:

```bash
nexus recall "authentication system"     # Semantic search across sessions, notes, preferences
nexus inject myproject --task "add rate limiting"  # Smart context: state + recall + preferences
```

The `inject` command assembles context in three passes:
1. **Project State** — Current branch, status, last commit, recent sessions
2. **Semantic Recall** — Vector-similar past work based on your task description
3. **Preferences** — Active preferences (coding style, workflow patterns, decisions) scoped to the project

### Preference Learning

Nexus learns and remembers your preferences over time:

```bash
nexus remember "Always run tests before committing" --category workflow
nexus remember "Prefer Go for backend services" --category tool --project myproject
```

Preferences support confidence decay, deduplication, supersession, and automatic pruning — so outdated preferences fade while important ones persist.

### Context Export for AI

The `nexus context` command exports a project's full state as markdown — recent sessions, notes, linked projects, and git status — formatted specifically for pasting into AI tools. This gives the AI a structured briefing that replaces the manual re-explanation you'd otherwise have to do.

```bash
nexus context myproject    # Export recent history + state as markdown
nexus context --inject myproject  # Include semantic recall and preferences
nexus resume myproject     # Get a "pick up where you left off" summary
```

### Cross-Session Search

Forgot which project had that retry logic? Can't remember when you refactored the database layer? Nexus provides both full-text and semantic search across all session summaries, notes, and file paths:

```bash
nexus search "retry logic"        # Full-text search across all projects
nexus recall "database migration" # Semantic search with vector similarity
nexus where "database migration"  # Find which projects and files match
```

### The Memory Loop

The real power is the workflow loop this enables:

1. **You work with AI** — Nexus captures the session automatically
2. **You come back later** — AI queries Nexus for context via `inject` or MCP tools
3. **AI picks up where you left off** — no re-briefing needed
4. **Repeat** — context accumulates over days, weeks, months

This transforms stateless AI tools into something that genuinely knows your projects.

## Install

```bash
go install github.com/digitalghost404/nexus@latest
```

Requires Go 1.24+.

## Quick Start

```bash
# Initialize Nexus (creates ~/.nexus/)
nexus init

# Add your project directories
nexus config roots add ~/projects

# Scan for projects
nexus scan

# See what needs attention
nexus
```

### Auto-capture AI Sessions

Add this to your `~/.bashrc` to automatically log every AI coding session:

```bash
claude() { command claude "$@"; local rc=$?; nexus capture --dir "$PWD"; return $rc; }
```

Or let Nexus do it for you:

```bash
nexus hook install
source ~/.bashrc
```

This is the critical piece — once the hook is installed, every session is automatically recorded. No manual effort required.

### Periodic Scanning

Set up a cron job to keep project health data fresh:

```bash
# Nexus hook install does this automatically, or manually:
crontab -e
# Add: */30 * * * * ~/go/bin/nexus scan >> ~/.nexus/nexus.log 2>&1
```

## Commands

### Smart Summary

```bash
nexus              # Dashboard: dirty projects, recent sessions, stale projects
nexus watch        # Live auto-refreshing dashboard (30s interval)
```

### Query Commands

```bash
nexus projects                    # List all tracked projects
nexus projects --active           # Only active projects
nexus projects --dirty            # Projects with uncommitted changes
nexus projects --stale            # Idle and stale projects

nexus sessions                    # Last 10 sessions
nexus sessions --project wraith   # Filter by project
nexus sessions --since 7d         # Last 7 days
nexus sessions --today            # Today only
nexus sessions --tag "bugfix"     # Filter by user tag

nexus show wraith                 # Detailed project view
nexus wraith                      # Shorthand for show

nexus search "retry logic"        # Full-text search across sessions and notes
nexus search --project wraith     # Scoped search
nexus search --files "*.go"       # Find sessions that touched Go files

nexus where "retry"               # Find which projects and files match a query
```

### Workflow Commands

```bash
nexus resume                      # Pick up where you left off (current project)
nexus resume wraith               # Resume a specific project

nexus diff                        # Changes across sessions (default: last 7 days)
nexus diff --since 30d            # Last 30 days

nexus context wraith              # Export project context as markdown for AI
nexus context --inject wraith     # Include semantic recall and preferences

nexus report                      # Weekly activity summary
nexus report --month              # Monthly summary

nexus note "fixed the auth bug"   # Add a note to the current project

nexus streak                      # Show your coding streak
```

### Persistent Memory Commands

```bash
nexus remember "Always run tests"              # Save a preference
nexus remember "Prefer Go" --category tool     # With category
nexus remember "Works evenings" --source inferred  # Inferred pattern

nexus recall "authentication system"           # Semantic search
nexus recall "database" --limit 10             # Limit results
nexus recall "style" --types session,note      # Filter types

nexus inject myproject                         # Smart context for session start
nexus inject myproject --task "add rate limiting"  # Task-aware context

nexus embed --backfill                         # Process all pending embeddings
nexus embed --reembed                          # Re-embed changed content

nexus maintain                                 # Run decay, prune, vacuum
nexus maintain --decay-only                    # Confidence decay only
nexus maintain --prune-only                    # Pruning only

nexus preferences                              # List all preferences
nexus preferences --project myproject          # Project-scoped
nexus preferences --category workflow          # Filter by category
```

### Server Mode

```bash
nexus serve                    # Start HTTP API server (default port 7600)
nexus serve --port 8080        # Custom port
```

The server runs an async embedding worker and exposes REST endpoints for MCP integration.

### Maintenance Commands

```bash
nexus stale                       # Show stale branches and dirty projects
nexus stale --cleanup             # Interactive branch cleanup (y/n/q per branch)

nexus deps                        # Check outdated dependencies (Go/npm/pip)
nexus deps --project wraith       # Check a single project

nexus link wraith wraith-dashboard    # Link related projects
nexus link wraith                     # Show links for a project
nexus link wraith --unlink dashboard  # Remove a link

nexus tag "breakthrough"          # Tag latest session (current project)
nexus tag 42 "important"          # Tag a specific session by ID
nexus tag 42 --remove "important" # Remove a tag

nexus hook install                # Install shell wrapper + cron job
nexus hook uninstall              # Remove both

nexus config show                 # Show current config
nexus config roots add ~/projects # Add a scan root
nexus config exclude add "*/tmp/*" # Add an exclusion pattern

nexus scan                        # Manual project scan
nexus scan --verbose              # Show discovery details
```

## How It Works

### Architecture

Nexus has no background daemon by default. It uses two capture mechanisms:

1. **Shell wrapper** -- A bash function that runs `nexus capture` after every AI session exits. Captures session data in real time.

2. **Periodic scanner** -- `nexus scan` (via cron or manual) crawls your project directories, updates health data, and backfills any missed sessions from git history.

3. **Server mode** (optional) -- `nexus serve` starts an HTTP API server with an async embedding worker that processes pending items in the background.

All data is stored in a SQLite database at `~/.nexus/<agent>/nexus.db` (WAL mode for concurrent access).

### What Gets Tracked

**Per project:**
- Git branch, dirty files, last commit
- Ahead/behind remote
- Health status (active / idle / stale)
- Detected languages
- Stale branches
- Links to related projects

**Per session:**
- Start/end time, duration
- Files changed, commits made
- Auto-generated summary from git data
- Auto-tags (project name, languages)
- User tags for categorization
- AI session ID (for correlation with tool-specific data)

**Per preference:**
- Content, category, source (stated/observed/inferred)
- Confidence score with time-based decay
- Project scope (or global)
- Supersession tracking (newer preferences replace older ones)

**Per embedding:**
- Vector (768-dim float array stored as BLOB)
- Source type (session/note/preference) and ID
- Content hash for change detection
- Model metadata

### Session Summary Generation

Summaries are generated in layers:

1. **Git-based** (always available) -- Commits and diffs from the session window
2. **AI session data** (opportunistic) -- Parsed from tool-specific directories if available
3. **Manual notes** -- `nexus note "message"` for your own context

### Semantic Search & Embeddings

Nexus uses Ollama's `nomic-embed-text` model to generate vector embeddings for sessions, notes, and preferences. Search works via cosine similarity between the query vector and stored vectors.

If Ollama is unavailable, Nexus falls back to SQLite FTS5 keyword search — so search always works, just with different quality.

**Embedding worker** (runs in `nexus serve`):
- Polls every 30 seconds for unembedded items
- Batches up to 10 items at a time
- Sends to Ollama's `/api/embed` endpoint
- Stores vectors as BLOBs with metadata
- Queues items when Ollama is down, processes when it returns

### Context Builder (3-Pass Injection)

The `inject` command assembles context in three passes:

1. **Project State** — Current branch, status, last commit, and recent 5 sessions
2. **Semantic Recall** — Vector-similar sessions, notes, and preferences based on the task description (3 per type, min similarity 0.7)
3. **Preferences** — Active preferences (confidence > 0.3, not superseded) scoped to the project plus global

Output is formatted as markdown, ready to paste into an AI conversation.

### Preference Lifecycle

- **Creation**: Via `nexus remember` or POST `/api/preferences`
- **Deduplication**: Duplicate content in the same category updates the existing record
- **Decay**: Inferred preferences decay with a 45-day half-life; stated/observed use 90 days
- **Supersession**: Newer preferences can supersede older ones
- **Pruning**: Low-confidence inferred preferences are deleted during `nexus maintain`
- **Access tracking**: Each reference bumps access count and updates last-referenced timestamp

### Agent Isolation

Each AI agent gets its own database under `~/.nexus/<agent>/`:

| Agent | Database Path | Config Path |
|-------|--------------|-------------|
| `claude` (default) | `~/.nexus/claude/nexus.db` | `~/.nexus/claude/config.yaml` |
| `opencode` | `~/.nexus/opencode/nexus.db` | `~/.nexus/opencode/config.yaml` |

Use `--agent` or `-a` flag to select: `nexus --agent opencode remember "..."`

Legacy data at `~/.nexus/nexus.db` is automatically migrated to `~/.nexus/claude/` on first run.

### Integration with AI Memory Systems

Nexus complements AI tools' built-in memory systems. While per-project memory captures preferences and feedback within a project, Nexus provides the **cross-project, cross-session timeline** that per-project memory can't:

| Capability | Per-Project Memory | Nexus |
|---|---|---|
| User preferences | Yes | Yes |
| Per-project context | Yes | Yes |
| Cross-project overview | No | Yes |
| Session history & timeline | No | Yes |
| File change tracking | No | Yes |
| Semantic search over history | No | Yes |
| Full-text search over history | No | Yes |
| Git health monitoring | No | Yes |
| Dependency status | No | Yes |
| Preference learning & decay | Limited | Yes |

The ideal setup uses both: per-project memory for *how* to work with you, and Nexus for *what* you've been working on.

### Health Status

| Status | Condition |
|--------|-----------|
| Active | Session or commit in last 3 days |
| Idle | Last activity 3-14 days ago |
| Stale | Last activity 14+ days ago |

Dirty (uncommitted changes) is tracked independently -- a project can be Active+Dirty.

Thresholds are configurable in `~/.nexus/<agent>/config.yaml`.

### Auto-Discovery

When `nexus scan` runs, it walks configured root directories looking for `.git/` folders and registers new projects automatically. Default exclusions skip `node_modules`, `vendor`, `.cache`, `go/pkg`, `snap`, and `.nvm`.

Projects that disappear from disk are automatically archived.

## API Endpoints (nexus serve)

When running `nexus serve`, these REST endpoints are available:

| Endpoint | Method | Purpose |
|---|---|---|
| `/api/capture` | POST | Capture session from project directory |
| `/api/notes` | GET, POST | List/create notes |
| `/api/preferences` | GET, POST | List/create preferences |
| `/api/preferences/{id}` | PATCH, DELETE | Update/delete preference |
| `/api/recall` | POST | Semantic search (FTS5 fallback) |
| `/api/inject` | POST | Build smart 3-pass context |
| `/api/embed/status` | GET | Embedding queue status |

All endpoints support CORS and return JSON (except `/api/inject` which returns `text/plain` markdown).

### Probe-Before-Write

The `nexus capture` and `nexus note` CLI commands use a probe-before-write pattern:
1. Try HTTP POST to the serve endpoint first
2. If connection refused (server not running), fall back to direct DB write
3. Log which mode was used (online vs offline)

This means CLI commands work whether or not `nexus serve` is running.

## Configuration

Config lives at `~/.nexus/<agent>/config.yaml`:

```yaml
roots:
  - ~/projects

exclude:
  - "*/node_modules/*"
  - "*/vendor/*"
  - "*/.cache/*"
  - "*/go/pkg/*"
  - "*/snap/*"
  - "*/.nvm/*"

thresholds:
  idle: 3     # days
  stale: 14   # days

scan_interval: 30m

# Persistent memory settings
ollama_url: http://localhost:11434
ollama_model: nomic-embed-text
serve_port: 7600
embed_batch_size: 10
embed_poll_interval: 30
```

Default exclusions are always merged with your custom patterns -- you won't lose them by adding your own.

## Data Storage

All data lives in `~/.nexus/<agent>/`:

| File | Purpose |
|------|---------|
| `nexus.db` | SQLite database (projects, sessions, notes, links, tags, preferences, embeddings) |
| `config.yaml` | Configuration |
| `nexus.log` | Error log from unattended captures (1MB rotation) |

### Schema

Current schema version: **5**. Tables:

| Table | Purpose |
|---|---|
| `projects` | Tracked projects with health data |
| `sessions` | AI session captures with summaries |
| `notes` | Free-form text notes |
| `notes_fts` | FTS5 index for notes |
| `sessions_fts` | FTS5 index for session summaries |
| `preferences` | Learned/stated preferences with confidence |
| `preferences_fts` | FTS5 index for preferences |
| `embedding_meta` | Embedding metadata (source, hash, model) |
| `embeddings` | Vector BLOBs (768-dim float arrays) |
| `links` | Project relationships |
| `tags` | Session tags |
| `session_files` | Files touched per session |

## Dependency Checking

`nexus deps` checks for outdated packages across three ecosystems:

| File Detected | Tool Used | Command |
|---------------|-----------|---------|
| `go.mod` | `go` | `go list -m -u -json all` |
| `package.json` | `npm` | `npm outdated --json` |
| `requirements.txt` | `pip3` | `pip3 list --outdated --format=json` |

Missing tools are silently skipped -- if you don't have npm installed, Go and pip projects are still checked.

## Search

Nexus provides two search modes:

**Full-text search** (always available, via SQLite FTS5):
```bash
nexus search "retry logic"        # Search summaries and notes
nexus where "retry"               # Search summaries AND file paths
```

**Semantic search** (requires Ollama):
```bash
nexus recall "authentication system"  # Vector similarity search
nexus recall "style" --types session,note  # Filter by type
```

If Ollama is unavailable, `recall` automatically falls back to FTS5 keyword search.

## MCP Integration

Nexus integrates with AI tools via [nexus-mcp](https://github.com/digitalghost404/nexus-mcp), an MCP server that exposes all nexus functionality as tools:

```json
{
  "mcpServers": {
    "nexus": {
      "command": "node",
      "args": ["/path/to/nexus-mcp/index.js"],
      "environment": {
        "NEXUS_AGENT": "opencode"
      }
    }
  }
}
```

Available MCP tools: `context`, `resume`, `note`, `search`, `where`, `report`, `projects`, `show`, `sessions`, `recall`, `remember`, `preferences`, `inject`.

## Tech Stack

- **Language:** Go (pure, no CGO)
- **Database:** SQLite via [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite)
- **CLI:** [Cobra](https://github.com/spf13/cobra)
- **Config:** [gopkg.in/yaml.v3](https://pkg.go.dev/gopkg.in/yaml.v3)
- **Embeddings:** Ollama API (`nomic-embed-text` model) with Go-side cosine similarity
- **Full-text search:** SQLite FTS5

Single binary, no external dependencies at runtime.

## License

MIT
