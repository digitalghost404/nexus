# Nexus v2 — Feature Expansion Design Spec

**Date:** 2026-03-19
**Status:** Draft
**Author:** xcoleman + Claude
**Depends on:** `docs/superpowers/specs/2026-03-19-nexus-design.md` (v1 spec)

## Overview

12 new CLI commands and improved help output for Nexus. All additive — no breaking changes to existing functionality. Commands compose existing internal packages (`db`, `scanner`, `display`, `capture`).

## Goals

- Answer "where was I?" instantly (`resume`)
- Provide higher-level views of work over time (`diff`, `report`)
- Make maintenance frictionless (`stale --cleanup`, `deps`)
- Enable richer context sharing with Claude (`context`)
- Reduce setup friction (`hook install`)
- Add searchability and discoverability (`tag`, `where`, `streak`)
- Support multi-project awareness (`link`, `watch`)

## Non-Goals (for v2)

- No Obsidian export (still deferred)
- No web dashboard
- No TUI library — simple terminal output only
- No clipboard integration

## Database Changes

### New table: `project_links`

```sql
CREATE TABLE IF NOT EXISTS project_links (
    id                INTEGER PRIMARY KEY,
    project_id        INTEGER NOT NULL REFERENCES projects(id),
    linked_project_id INTEGER NOT NULL REFERENCES projects(id),
    created_at        DATETIME NOT NULL DEFAULT (datetime('now')),
    UNIQUE(project_id, linked_project_id)
);

CREATE INDEX IF NOT EXISTS idx_project_links_project ON project_links(project_id);
```

Bidirectional — linking A to B inserts two rows (A→B and B→A). Query either side with a single WHERE clause.

### New table: `session_tags`

```sql
CREATE TABLE IF NOT EXISTS session_tags (
    id          INTEGER PRIMARY KEY,
    session_id  INTEGER NOT NULL REFERENCES sessions(id),
    tag         TEXT NOT NULL,
    created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
    UNIQUE(session_id, tag)
);

CREATE INDEX IF NOT EXISTS idx_session_tags_tag ON session_tags(tag);
```

Separate from the existing `tags` JSON column (which holds auto-generated tags). This table holds user-applied labels, queryable and manageable.

### Schema Migration

Bump `PRAGMA user_version` from 1 to 2. The `migrate()` function checks version and runs new CREATE TABLE/INDEX statements only when upgrading from 1. Existing data is untouched.

### New DB Methods

**project_links:**
- `LinkProjects(projectID, linkedProjectID int64) error` — inserts both directions
- `UnlinkProjects(projectID, linkedProjectID int64) error` — removes both directions
- `GetLinkedProjects(projectID int64) ([]Project, error)` — returns all linked projects

**session_tags:**
- `AddSessionTag(sessionID int64, tag string) error`
- `RemoveSessionTag(sessionID int64, tag string) error`
- `ListSessionTags(sessionID int64) ([]string, error)`
- `ListSessionsByTag(tag string) ([]Session, error)`

**queries for new commands:**
- `GetLatestSession(projectID int64) (*Session, error)` — most recent session for a project
- `GetSessionsInRange(projectID int64, since, until time.Time) ([]Session, error)` — for diff/report
- `GetDistinctSessionDates() ([]time.Time, error)` — for streak calculation
- `CountSessionsByProject(since time.Time) (map[int64]int, error)` — for report aggregation

## Command Specifications

### Tier 1 — No New Tables Required

#### `nexus resume [project]`

Shows the last session for the current or specified project with full context.

**Output:**
```
┌ RESUME: wraith ────────────────────────────
│
│  Last session: 2h ago (45 min)
│  Summary: "Added retry logic to DNS scanner, fixed timeout bug"
│
│  Commits:
│    abc123  feat: add retry logic
│    def456  fix: resolve DNS timeout
│
│  Files changed:
│    cmd/scan.go, internal/scanner.go, internal/retry.go
│
│  Uncommitted changes: 2 files
│    M internal/scanner_test.go
│    ? internal/retry_test.go
│
└────────────────────────────────────────────
```

**Data sources:** Last session from DB (summary, commits, files) + live `git status` for uncommitted changes.

**Behavior:** If inside a project directory, uses that project. If project name given as arg, uses that. If neither, error.

#### `nexus diff [project] [--since 7d]`

Aggregates activity across all sessions in a time window.

```
┌ DIFF: wraith (last 7 days) ────────────────
│
│  5 sessions, 12 commits, 8 files touched
│
│  Timeline:
│    Mar 19  "Added retry logic to DNS scanner"
│    Mar 18  "Refactored HTTP client"
│    Mar 17  "Fixed database migration bug"
│
│  Most active files:
│    internal/scanner.go  (modified in 4 sessions)
│    cmd/scan.go          (modified in 3 sessions)
│
└────────────────────────────────────────────
```

**Data sources:** Sessions in time range, `files_changed` JSON parsed and counted per file.

**Defaults:** Current project if inside one, `--since 7d`.

#### `nexus stale [--cleanup]`

Without `--cleanup`: lists stale branches and dirty projects with detail (same data as `nexus projects --stale` but richer output including branch ages and dirty file details).

With `--cleanup`: interactive mode that walks through stale branches:

1. For each stale branch across all projects, show branch name, last commit date
2. Prompt: `Delete? [y/n/q]`
3. `y` = delete branch via `git branch -d`, `n` = skip, `q` = quit
4. For dirty projects: show uncommitted files, warn "review manually" — never auto-discard work

**Branch deletion:** Uses `git branch -d` (safe delete, refuses if unmerged). If user wants force delete, they do it manually.

#### `nexus deps`

Scans all tracked projects for outdated dependencies.

```
┌ DEPENDENCIES ──────────────────────────────
│
│  wraith (Go)
│    github.com/spf13/cobra  v1.8.0 → v1.10.2
│
│  playvoidterm (npm)
│    electron  28.0.0 → 31.2.0
│
│  8 projects clean
│
└────────────────────────────────────────────
```

**Detection per project:**
- `go.mod` exists → run `go list -m -u -json all` in that directory
- `package.json` exists → run `npm outdated --json` in that directory
- `requirements.txt` exists → run `pip list --outdated --format=json` in that directory

**Tool tolerance:** Check `exec.LookPath("go")`, `exec.LookPath("npm")`, `exec.LookPath("pip3")` before each checker. Skip with message if tool not found. Never crash.

#### `nexus report [--week|--month]`

Activity summary for a time period. Defaults to `--week`.

```
┌ REPORT: Mar 12 – Mar 19 ──────────────────
│
│  12 sessions across 5 projects
│  34 commits, 47 files changed
│  ~8.5 hours of Claude sessions
│
│  Most active projects:
│    wraith        6 sessions, 18 commits
│    nexus         4 sessions, 12 commits
│
│  Languages:
│    Go 62%  TypeScript 28%  Python 10%
│
│  Highlights:
│    "Added retry logic to DNS scanner"
│    "Built Nexus CLI with project tracking"
│
└────────────────────────────────────────────
```

**Data:** All from sessions table with time range filter. Highlights = top 3 sessions by files changed count. Language percentages from counting sessions per language tag. Hours from summing `duration_secs`.

#### `nexus watch`

Live auto-refreshing terminal dashboard. Reuses `FormatSmartSummary`:

```go
for {
    clearScreen()
    // query fresh data from DB
    display.FormatSmartSummary(os.Stdout, dirty, sessions, stale, currentProject)
    fmt.Println("\nRefreshing every 30s — Ctrl+C to exit")
    time.Sleep(30 * time.Second)
}
```

30-second refresh interval. No TUI library. Simple clear-and-redraw loop.

#### `nexus context <project>`

Outputs everything Nexus knows about a project, formatted as markdown for pasting into Claude:

```markdown
## Project: wraith
Path: ~/projects-wsl/wraith
Branch: main (3 files dirty)
Languages: Go, TypeScript

## Recent Sessions (last 7 days)
- Mar 19: "Added retry logic to DNS scanner" (3 commits)
- Mar 18: "Refactored HTTP client" (2 commits)

## Recent Commits
- abc123 feat: add retry logic
- def456 fix: resolve DNS timeout

## Notes
- "migrated auth to JWT, still need tests"

## Linked Projects
- wraith-dashboard (last session: yesterday)
```

Output to stdout. No clipboard integration — user pipes to `clip.exe` if needed.

#### `nexus hook install` / `nexus hook uninstall`

**Install:**
1. Check for any existing `claude()` function in `~/.bashrc` (not just our wrapper)
2. If found with different content, warn and abort: "Existing claude() function found in .bashrc — not overwriting. Add manually."
3. If not found, append the wrapper function
4. Check `crontab -l` for existing `nexus scan` entry
5. If not found, append to crontab
6. Safe to run multiple times — never duplicates

**Uninstall:**
1. Remove the `claude()` wrapper from `~/.bashrc`
2. Remove the `nexus scan` line from crontab

#### `nexus streak`

```
Current streak: 12 days
Longest streak: 23 days (Feb 10 – Mar 4)

This week: ██████░  6/7 days
Last week: ███████  7/7 days
```

**Calculation:** `SELECT DISTINCT date(started_at) FROM sessions ORDER BY date(started_at) DESC`. Walk backward from today counting consecutive dates. For longest streak, find the longest run in the full date list.

Weekly bars: 7 characters, `█` for days with sessions, `░` for days without. Current week and last week shown.

#### `nexus where <query>`

```
nexus where "retry"

wraith
  internal/scanner.go     (sessions: Mar 19, Mar 18)
  internal/retry.go       (session: Mar 19)

nexus
  internal/capture/capture.go  (session: Mar 17)
```

**Logic:**
1. FTS search on `sessions.summary` for the query
2. For matching sessions, parse `files_changed` JSON
3. Group by project, then by file path
4. Show which sessions touched each file

Answers "where did I implement X?" instead of "when did I work on X?"

### Tier 2 — Requires New Tables

#### `nexus link <project-a> [project-b]` / `nexus link <project> --unlink <project>`

```bash
nexus link wraith wraith-dashboard    # create link
nexus link wraith                     # show links
nexus link wraith --unlink wraith-dashboard  # remove link
```

**One arg:** list linked projects.
**Two args:** create bidirectional link (inserts two rows).
**--unlink:** remove bidirectional link (deletes two rows).

**Integration:** `nexus show <project>` displays a "Linked projects" section with recent activity from linked projects.

#### `nexus tag ["label"]` / `nexus tag <session-id> "label"`

```bash
nexus tag "breakthrough"              # tags latest session for current project
nexus tag 42 "dead-end"               # tags session #42
nexus tag 42 --remove "dead-end"      # removes tag
nexus sessions --tag "breakthrough"   # filter sessions by tag
```

**No session ID or "latest":** resolves to most recent session for current project (detected from `$PWD`).

**Numeric first arg:** treated as session ID.

**Integration:** `nexus search` results include tags. `nexus sessions --tag` filter added.

## Help & Command Grouping

Cobra `AddGroup()` organizes commands into logical sections:

```
Core Commands:
  init, scan, capture

Query Commands:
  projects, sessions, search, show, where

Workflow Commands:
  resume, diff, context, report, note, streak

Maintenance Commands:
  stale, deps, link, tag, hook, watch, config
```

All commands get clear `Short` descriptions. Commands with flags get `Long` descriptions explaining usage.

## Implementation Order

1. **Tier 1 commands** (no schema changes): resume, diff, stale, deps, report, watch, context, hook, streak, where
2. **Schema migration** v1→v2 (new tables + indexes)
3. **Tier 2 commands** (need new tables): link, tag
4. **Help improvements** (update all command descriptions, add grouping)

## Testing Strategy

### Unit tests
- DB methods: CRUD on `project_links` and `session_tags`, migration v1→v2
- Streak calculation: consecutive days, gaps, empty DB, single day
- Where: JSON parsing of `files_changed`, grouping logic
- Report: aggregation, percentage calculation
- Deps: parsing `go list`, `npm outdated`, `pip list` output

### Integration tests
- Schema migration: create v1 DB with data, migrate to v2, verify new tables exist and old data intact
- Hook install: verify `.bashrc` append, cron install, idempotency, existing function detection
- Stale --cleanup: test repos with stale branches, verify deletion

### Skip
- `nexus watch` — test underlying queries, not the display loop
- `nexus context` — test data gathering, not formatting
- No mocking external tools — use `exec.LookPath` checks, skip tests when tools unavailable

## Tech Stack (unchanged)

Same as v1: Go, SQLite (modernc.org/sqlite), Cobra, gopkg.in/yaml.v3. No new dependencies.
