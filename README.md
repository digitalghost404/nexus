# Nexus

A CLI tool that tracks your project health and Claude Code sessions from a single command. One binary, no daemon, no dependencies beyond Go.

Nexus answers two questions: **"what's the state of everything?"** and **"what did I do last?"**

## Install

```bash
go install github.com/digitalghost404/nexus@latest
```

Requires Go 1.22+.

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

### Auto-capture Claude Sessions

Add this to your `~/.bashrc` to automatically log every Claude Code session:

```bash
claude() { command claude "$@"; local rc=$?; nexus capture --dir "$PWD"; return $rc; }
```

Or let Nexus do it for you:

```bash
nexus hook install
source ~/.bashrc
```

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

nexus context wraith              # Export project context as markdown for Claude

nexus report                      # Weekly activity summary
nexus report --month              # Monthly summary

nexus note "fixed the auth bug"   # Add a note to the current project

nexus streak                      # Show your coding streak
```

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

Nexus has no background daemon. It uses two capture mechanisms:

1. **Shell wrapper** -- A bash function that runs `nexus capture` after every Claude Code session exits. Captures session data in real time.

2. **Periodic scanner** -- `nexus scan` (via cron or manual) crawls your project directories, updates health data, and backfills any missed sessions from git history.

All data is stored in a single SQLite database at `~/.nexus/nexus.db` (WAL mode for concurrent access).

### What Gets Tracked

**Per project:**
- Git branch, dirty files, last commit
- Ahead/behind remote
- Health status (active / idle / stale)
- Detected languages
- Stale branches

**Per session:**
- Start/end time, duration
- Files changed, commits made
- Auto-generated summary from git data
- Auto-tags (project name, languages)
- User tags
- Claude session ID (for correlation)

### Session Summary Generation

Summaries are generated in layers:

1. **Git-based** (always available) -- Commits and diffs from the session window
2. **Claude session data** (opportunistic) -- Parsed from `~/.claude/` if available
3. **Manual notes** -- `nexus note "message"` for your own context

### Health Status

| Status | Condition |
|--------|-----------|
| Active | Session or commit in last 3 days |
| Idle | Last activity 3-14 days ago |
| Stale | Last activity 14+ days ago |

Dirty (uncommitted changes) is tracked independently -- a project can be Active+Dirty.

Thresholds are configurable in `~/.nexus/config.yaml`.

### Auto-Discovery

When `nexus scan` runs, it walks configured root directories looking for `.git/` folders and registers new projects automatically. Default exclusions skip `node_modules`, `vendor`, `.cache`, `go/pkg`, `snap`, and `.nvm`.

Projects that disappear from disk are automatically archived.

## Configuration

Config lives at `~/.nexus/config.yaml`:

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
```

Default exclusions are always merged with your custom patterns -- you won't lose them by adding your own.

## Data Storage

All data lives in `~/.nexus/`:

| File | Purpose |
|------|---------|
| `nexus.db` | SQLite database (projects, sessions, notes, links, tags) |
| `config.yaml` | Configuration |
| `nexus.log` | Error log from unattended captures (1MB rotation) |

## Dependency Checking

`nexus deps` checks for outdated packages across three ecosystems:

| File Detected | Tool Used | Command |
|---------------|-----------|---------|
| `go.mod` | `go` | `go list -m -u -json all` |
| `package.json` | `npm` | `npm outdated --json` |
| `requirements.txt` | `pip3` | `pip3 list --outdated --format=json` |

Missing tools are silently skipped -- if you don't have npm installed, Go and pip projects are still checked.

## Search

Nexus uses SQLite FTS5 for full-text search across session summaries and notes:

```bash
nexus search "retry logic"        # Search summaries and notes
nexus where "retry"               # Search summaries AND file paths, grouped by project/file
```

## Tech Stack

- **Language:** Go (pure, no CGO)
- **Database:** SQLite via [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite)
- **CLI:** [Cobra](https://github.com/spf13/cobra)
- **Config:** [gopkg.in/yaml.v3](https://pkg.go.dev/gopkg.in/yaml.v3)

Single binary, no external dependencies at runtime.

## License

MIT
