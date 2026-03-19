# Nexus Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build Nexus, a Go CLI that tracks project health and Claude Code sessions from a single command.

**Architecture:** Shell wrapper triggers `nexus capture` after Claude sessions; periodic `nexus scan` crawls roots for project health. SQLite (WAL mode) stores everything. Cobra CLI with context-aware commands.

**Tech Stack:** Go, SQLite (modernc.org/sqlite), Cobra, Viper, FTS5

**Spec:** `docs/superpowers/specs/2026-03-19-nexus-design.md`

---

## File Structure

```
nexus/
├── main.go                         # Entry point, root command setup
├── go.mod
├── go.sum
├── cmd/
│   ├── root.go                     # Root cobra command, --debug flag, context detection
│   ├── init.go                     # nexus init
│   ├── scan.go                     # nexus scan
│   ├── capture.go                  # nexus capture --dir <path>
│   ├── projects.go                 # nexus projects [--active|--dirty|--stale]
│   ├── sessions.go                 # nexus sessions [--project|--since|--today]
│   ├── search.go                   # nexus search <query> [--project|--files]
│   ├── show.go                     # nexus show <project> + shorthand routing
│   ├── note.go                     # nexus note "message"
│   └── config.go                   # nexus config roots/exclude/show
├── internal/
│   ├── db/
│   │   ├── db.go                   # Open, migrate, close. WAL mode, schema versioning.
│   │   ├── db_test.go
│   │   ├── schema.sql              # Embedded SQL: all CREATE TABLE/INDEX/TRIGGER statements
│   │   ├── projects.go             # CRUD for projects table
│   │   ├── projects_test.go
│   │   ├── sessions.go             # CRUD for sessions table + FTS queries
│   │   ├── sessions_test.go
│   │   ├── notes.go                # CRUD for notes table + FTS queries
│   │   └── notes_test.go
│   ├── scanner/
│   │   ├── scanner.go              # Walk roots, discover .git dirs, apply exclusions
│   │   ├── scanner_test.go
│   │   ├── git.go                  # Git status, branch, log, ahead/behind, stale branches
│   │   └── git_test.go
│   ├── capture/
│   │   ├── capture.go              # Orchestrate session capture: timestamps, summary, tags
│   │   ├── capture_test.go
│   │   ├── claude.go               # Read Claude session metadata + JSONL (opportunistic)
│   │   ├── claude_test.go
│   │   ├── summary.go              # Generate summary from git commits/diffs
│   │   └── summary_test.go
│   ├── config/
│   │   ├── config.go               # Load/save config, defaults, expand ~
│   │   └── config_test.go
│   ├── display/
│   │   ├── display.go              # Table/box formatting, color, smart summary layout
│   │   └── display_test.go
│   └── logger/
│       ├── logger.go               # --debug stderr, file logging, rotation
│       └── logger_test.go
```

---

### Task 1: Project Scaffolding + Go Module

**Files:**
- Create: `main.go`
- Create: `go.mod`
- Create: `cmd/root.go`

- [ ] **Step 1: Initialize Go module**

Run:
```bash
cd ~/projects-wsl/nexus
go mod init github.com/digitalghost404/nexus
```

- [ ] **Step 2: Create main.go**

```go
// main.go
package main

import "github.com/digitalghost404/nexus/cmd"

func main() {
	cmd.Execute()
}
```

- [ ] **Step 3: Create root command**

```go
// cmd/root.go
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var debug bool

var rootCmd = &cobra.Command{
	Use:   "nexus",
	Short: "Track project health and Claude sessions",
	Long:  "Nexus gives you a single pane of glass into all your projects and Claude Code sessions.",
	RunE: func(cmd *cobra.Command, args []string) error {
		// If first arg matches a known project name, route to show
		if len(args) > 0 {
			// TODO: check if args[0] is a project name and route to show
		}
		// TODO: smart summary
		fmt.Println("Nexus — run 'nexus init' to get started")
		return nil
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Enable debug output to stderr")
}
```

- [ ] **Step 4: Install dependencies**

Run:
```bash
cd ~/projects-wsl/nexus
go get github.com/spf13/cobra
go get github.com/spf13/viper
```

- [ ] **Step 5: Verify it builds and runs**

Run:
```bash
cd ~/projects-wsl/nexus
go build -o nexus . && ./nexus
```
Expected: prints "Nexus — run 'nexus init' to get started"

- [ ] **Step 6: Commit**

```bash
git add main.go go.mod go.sum cmd/root.go
git commit -m "feat: project scaffolding with root cobra command"
```

---

### Task 2: Logger

**Files:**
- Create: `internal/logger/logger.go`
- Create: `internal/logger/logger_test.go`

- [ ] **Step 1: Write failing test**

```go
// internal/logger/logger_test.go
package logger

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDebugWritesToStderr(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{Debug: true, Stderr: &buf})
	l.Debug("test message")
	if !strings.Contains(buf.String(), "test message") {
		t.Errorf("expected debug output, got: %s", buf.String())
	}
}

func TestDebugSilentWhenDisabled(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{Debug: false, Stderr: &buf})
	l.Debug("test message")
	if buf.Len() != 0 {
		t.Errorf("expected no output, got: %s", buf.String())
	}
}

func TestFileLogging(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "nexus.log")
	l := New(Config{LogFile: logPath})
	l.Error("file error message")
	l.Close()

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log: %v", err)
	}
	if !strings.Contains(string(data), "file error message") {
		t.Errorf("expected log content, got: %s", string(data))
	}
}

func TestLogRotation(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "nexus.log")
	l := New(Config{LogFile: logPath, MaxSize: 100}) // 100 bytes max

	// Write enough to trigger rotation
	for i := 0; i < 20; i++ {
		l.Error("this is a long enough message to fill the log file quickly")
	}
	l.Close()

	info, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("log file missing: %v", err)
	}
	if info.Size() > 200 { // some slack for last write
		t.Errorf("log file too large after rotation: %d bytes", info.Size())
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd ~/projects-wsl/nexus && go test ./internal/logger/ -v`
Expected: FAIL — package doesn't exist yet

- [ ] **Step 3: Implement logger**

```go
// internal/logger/logger.go
package logger

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

type Config struct {
	Debug   bool
	Stderr  io.Writer // defaults to os.Stderr
	LogFile string    // path to log file, empty = no file logging
	MaxSize int64     // max log file size in bytes, default 1MB
}

type Logger struct {
	cfg     Config
	stderr  io.Writer
	file    *os.File
	mu      sync.Mutex
	written int64
}

func New(cfg Config) *Logger {
	if cfg.Stderr == nil {
		cfg.Stderr = os.Stderr
	}
	if cfg.MaxSize == 0 {
		cfg.MaxSize = 1 << 20 // 1MB
	}

	l := &Logger{cfg: cfg, stderr: cfg.Stderr}

	if cfg.LogFile != "" {
		f, err := os.OpenFile(cfg.LogFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err == nil {
			l.file = f
			info, _ := f.Stat()
			if info != nil {
				l.written = info.Size()
			}
		}
	}

	return l
}

func (l *Logger) Debug(msg string, args ...interface{}) {
	if !l.cfg.Debug {
		return
	}
	formatted := fmt.Sprintf(msg, args...)
	fmt.Fprintf(l.stderr, "[DEBUG] %s\n", formatted)
}

func (l *Logger) Error(msg string, args ...interface{}) {
	formatted := fmt.Sprintf(msg, args...)
	l.writeToFile(fmt.Sprintf("[ERROR] %s %s\n", time.Now().Format(time.RFC3339), formatted))
}

func (l *Logger) Info(msg string, args ...interface{}) {
	formatted := fmt.Sprintf(msg, args...)
	l.writeToFile(fmt.Sprintf("[INFO]  %s %s\n", time.Now().Format(time.RFC3339), formatted))
}

func (l *Logger) writeToFile(line string) {
	if l.file == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.written >= l.cfg.MaxSize {
		l.rotate()
	}

	n, _ := l.file.WriteString(line)
	l.written += int64(n)
}

func (l *Logger) rotate() {
	l.file.Close()
	os.Truncate(l.cfg.LogFile, 0)
	f, err := os.OpenFile(l.cfg.LogFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err == nil {
		l.file = f
		l.written = 0
	}
}

func (l *Logger) Close() {
	if l.file != nil {
		l.file.Close()
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd ~/projects-wsl/nexus && go test ./internal/logger/ -v`
Expected: PASS (all 4 tests)

- [ ] **Step 5: Commit**

```bash
git add internal/logger/
git commit -m "feat: add logger with debug stderr, file logging, and rotation"
```

---

### Task 3: Config

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/config/config_test.go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := Default()
	if len(cfg.Roots) != 0 {
		t.Errorf("expected no default roots, got %v", cfg.Roots)
	}
	if cfg.Thresholds.Idle != 3 {
		t.Errorf("expected idle=3, got %d", cfg.Thresholds.Idle)
	}
	if cfg.Thresholds.Stale != 14 {
		t.Errorf("expected stale=14, got %d", cfg.Thresholds.Stale)
	}
	if cfg.ScanInterval != "30m" {
		t.Errorf("expected scan_interval=30m, got %s", cfg.ScanInterval)
	}
}

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	os.WriteFile(cfgPath, []byte(`
roots:
  - /tmp/projects
exclude:
  - "*/node_modules/*"
thresholds:
  idle: 5
  stale: 21
scan_interval: 15m
`), 0644)

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if len(cfg.Roots) != 1 || cfg.Roots[0] != "/tmp/projects" {
		t.Errorf("unexpected roots: %v", cfg.Roots)
	}
	if cfg.Thresholds.Idle != 5 {
		t.Errorf("expected idle=5, got %d", cfg.Thresholds.Idle)
	}
	if cfg.ScanInterval != "15m" {
		t.Errorf("expected 15m, got %s", cfg.ScanInterval)
	}
}

func TestLoadMissingFileReturnsDefault(t *testing.T) {
	cfg, err := Load("/nonexistent/path/config.yaml")
	if err != nil {
		t.Fatalf("expected no error for missing file, got: %v", err)
	}
	if cfg.Thresholds.Idle != 3 {
		t.Errorf("expected default idle=3, got %d", cfg.Thresholds.Idle)
	}
}

func TestExpandTilde(t *testing.T) {
	home, _ := os.UserHomeDir()
	result := ExpandPath("~/projects")
	expected := filepath.Join(home, "projects")
	if result != expected {
		t.Errorf("expected %s, got %s", expected, result)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd ~/projects-wsl/nexus && go test ./internal/config/ -v`
Expected: FAIL

- [ ] **Step 3: Implement config**

```go
// internal/config/config.go
package config

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Thresholds struct {
	Idle  int `yaml:"idle"`
	Stale int `yaml:"stale"`
}

type Config struct {
	Roots        []string   `yaml:"roots"`
	Exclude      []string   `yaml:"exclude"`
	Thresholds   Thresholds `yaml:"thresholds"`
	ScanInterval string     `yaml:"scan_interval"`
}

func Default() Config {
	return Config{
		Roots:   []string{},
		Exclude: []string{},
		Thresholds: Thresholds{
			Idle:  3,
			Stale: 14,
		},
		ScanInterval: "30m",
	}
}

func Load(path string) (Config, error) {
	cfg := Default()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}

	// Apply defaults for zero values
	if cfg.Thresholds.Idle == 0 {
		cfg.Thresholds.Idle = 3
	}
	if cfg.Thresholds.Stale == 0 {
		cfg.Thresholds.Stale = 14
	}
	if cfg.ScanInterval == "" {
		cfg.ScanInterval = "30m"
	}

	// Expand ~ in roots
	for i, r := range cfg.Roots {
		cfg.Roots[i] = ExpandPath(r)
	}

	return cfg, nil
}

func Save(path string, cfg Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func ExpandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

func NexusDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".nexus")
}

func ConfigPath() string {
	return filepath.Join(NexusDir(), "config.yaml")
}

func DBPath() string {
	return filepath.Join(NexusDir(), "nexus.db")
}

func LogPath() string {
	return filepath.Join(NexusDir(), "nexus.log")
}
```

- [ ] **Step 4: Install yaml dependency and run tests**

Run:
```bash
cd ~/projects-wsl/nexus
go get gopkg.in/yaml.v3
go test ./internal/config/ -v
```
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/config/ go.mod go.sum
git commit -m "feat: add config loading with YAML, defaults, and path expansion"
```

---

### Task 4: Database Layer — Schema + Open/Migrate/Close

**Files:**
- Create: `internal/db/schema.sql`
- Create: `internal/db/db.go`
- Create: `internal/db/db_test.go`

- [ ] **Step 1: Create embedded schema**

```sql
-- internal/db/schema.sql
PRAGMA journal_mode=WAL;

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
```

- [ ] **Step 2: Write failing tests**

```go
// internal/db/db_test.go
package db

import (
	"path/filepath"
	"testing"
)

func TestOpenCreatesDatabase(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	d, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer d.Close()

	// Verify tables exist by querying them
	var count int
	err = d.db.QueryRow("SELECT count(*) FROM projects").Scan(&count)
	if err != nil {
		t.Fatalf("projects table missing: %v", err)
	}

	err = d.db.QueryRow("SELECT count(*) FROM sessions").Scan(&count)
	if err != nil {
		t.Fatalf("sessions table missing: %v", err)
	}

	err = d.db.QueryRow("SELECT count(*) FROM notes").Scan(&count)
	if err != nil {
		t.Fatalf("notes table missing: %v", err)
	}
}

func TestOpenSetsWALMode(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	d, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer d.Close()

	var mode string
	d.db.QueryRow("PRAGMA journal_mode").Scan(&mode)
	if mode != "wal" {
		t.Errorf("expected WAL mode, got: %s", mode)
	}
}

func TestSchemaVersion(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	d, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer d.Close()

	var version int
	d.db.QueryRow("PRAGMA user_version").Scan(&version)
	if version != 1 {
		t.Errorf("expected user_version=1, got: %d", version)
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `cd ~/projects-wsl/nexus && go test ./internal/db/ -v`
Expected: FAIL

- [ ] **Step 4: Implement db.go**

```go
// internal/db/db.go
package db

import (
	"database/sql"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

type DB struct {
	db *sql.DB
}

func Open(path string) (*DB, error) {
	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	sqlDB, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	// Enable WAL mode
	if _, err := sqlDB.Exec("PRAGMA journal_mode=WAL"); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}

	d := &DB{db: sqlDB}

	if err := d.migrate(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return d, nil
}

func (d *DB) migrate() error {
	var version int
	d.db.QueryRow("PRAGMA user_version").Scan(&version)

	if version == 0 {
		if _, err := d.db.Exec(schemaSQL); err != nil {
			return fmt.Errorf("apply schema: %w", err)
		}
		if _, err := d.db.Exec("PRAGMA user_version = 1"); err != nil {
			return fmt.Errorf("set version: %w", err)
		}
	}

	return nil
}

func (d *DB) Close() error {
	return d.db.Close()
}

// Conn returns the underlying sql.DB for use by other packages.
func (d *DB) Conn() *sql.DB {
	return d.db
}
```

- [ ] **Step 5: Install SQLite dependency and run tests**

Run:
```bash
cd ~/projects-wsl/nexus
go get modernc.org/sqlite
go test ./internal/db/ -v
```
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/db/schema.sql internal/db/db.go internal/db/db_test.go go.mod go.sum
git commit -m "feat: database layer with schema, WAL mode, and migrations"
```

---

### Task 5: Database Layer — Projects CRUD

**Files:**
- Create: `internal/db/projects.go`
- Create: `internal/db/projects_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/db/projects_test.go
package db

import (
	"path/filepath"
	"testing"
	"time"
)

type Project struct {
	ID            int64
	Name          string
	Path          string
	Languages     string
	Branch        string
	Dirty         bool
	DirtyFiles    int
	LastCommitAt  *time.Time
	LastCommitMsg string
	Ahead         int
	Behind        int
	Status        string
	DiscoveredAt  time.Time
	LastScannedAt *time.Time
}

func testDB(t *testing.T) *DB {
	t.Helper()
	dir := t.TempDir()
	d, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func TestUpsertAndGetProject(t *testing.T) {
	d := testDB(t)

	now := time.Now()
	p := Project{
		Name:         "wraith",
		Path:         "/home/user/projects/wraith",
		Languages:    `["go","typescript"]`,
		Branch:       "main",
		Dirty:        true,
		DirtyFiles:   3,
		Status:       "active",
		DiscoveredAt: now,
	}

	id, err := d.UpsertProject(p)
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if id == 0 {
		t.Fatal("expected non-zero id")
	}

	got, err := d.GetProjectByPath("/home/user/projects/wraith")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "wraith" {
		t.Errorf("expected wraith, got %s", got.Name)
	}
	if !got.Dirty {
		t.Error("expected dirty=true")
	}
}

func TestListProjectsByStatus(t *testing.T) {
	d := testDB(t)

	now := time.Now()
	d.UpsertProject(Project{Name: "active1", Path: "/a", Status: "active", DiscoveredAt: now})
	d.UpsertProject(Project{Name: "stale1", Path: "/b", Status: "stale", DiscoveredAt: now})
	d.UpsertProject(Project{Name: "active2", Path: "/c", Status: "active", DiscoveredAt: now})

	active, err := d.ListProjects("active")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(active) != 2 {
		t.Errorf("expected 2 active, got %d", len(active))
	}

	all, err := d.ListProjects("")
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3 total, got %d", len(all))
	}
}

func TestListDirtyProjects(t *testing.T) {
	d := testDB(t)

	now := time.Now()
	d.UpsertProject(Project{Name: "clean", Path: "/a", Dirty: false, Status: "active", DiscoveredAt: now})
	d.UpsertProject(Project{Name: "dirty", Path: "/b", Dirty: true, DirtyFiles: 2, Status: "active", DiscoveredAt: now})

	dirty, err := d.ListDirtyProjects()
	if err != nil {
		t.Fatalf("list dirty: %v", err)
	}
	if len(dirty) != 1 {
		t.Errorf("expected 1 dirty, got %d", len(dirty))
	}
}

func TestUpsertProjectUpdatesExisting(t *testing.T) {
	d := testDB(t)

	now := time.Now()
	d.UpsertProject(Project{Name: "proj", Path: "/a", Branch: "main", Status: "active", DiscoveredAt: now})
	d.UpsertProject(Project{Name: "proj", Path: "/a", Branch: "develop", Status: "idle", DiscoveredAt: now})

	got, err := d.GetProjectByPath("/a")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Branch != "develop" {
		t.Errorf("expected develop, got %s", got.Branch)
	}

	all, _ := d.ListProjects("")
	if len(all) != 1 {
		t.Errorf("expected 1 project (upsert), got %d", len(all))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd ~/projects-wsl/nexus && go test ./internal/db/ -v -run TestUpsert`
Expected: FAIL — UpsertProject/GetProjectByPath/ListProjects not defined

- [ ] **Step 3: Implement projects.go**

```go
// internal/db/projects.go
package db

import (
	"database/sql"
	"fmt"
	"time"
)

type Project struct {
	ID            int64
	Name          string
	Path          string
	Languages     string
	Branch        string
	Dirty         bool
	DirtyFiles    int
	LastCommitAt  *time.Time
	LastCommitMsg string
	Ahead         int
	Behind        int
	Status        string
	DiscoveredAt  time.Time
	LastScannedAt *time.Time
}

func (d *DB) UpsertProject(p Project) (int64, error) {
	result, err := d.db.Exec(`
		INSERT INTO projects (name, path, languages, branch, dirty, dirty_files,
			last_commit_at, last_commit_msg, ahead, behind, status, discovered_at, last_scanned_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET
			name=excluded.name, languages=excluded.languages, branch=excluded.branch,
			dirty=excluded.dirty, dirty_files=excluded.dirty_files,
			last_commit_at=excluded.last_commit_at, last_commit_msg=excluded.last_commit_msg,
			ahead=excluded.ahead, behind=excluded.behind, status=excluded.status,
			last_scanned_at=excluded.last_scanned_at`,
		p.Name, p.Path, p.Languages, p.Branch, p.Dirty, p.DirtyFiles,
		p.LastCommitAt, p.LastCommitMsg, p.Ahead, p.Behind, p.Status,
		p.DiscoveredAt, p.LastScannedAt)
	if err != nil {
		return 0, fmt.Errorf("upsert project: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	// LastInsertId returns 0 on update, get the real ID
	if id == 0 {
		row := d.db.QueryRow("SELECT id FROM projects WHERE path = ?", p.Path)
		row.Scan(&id)
	}
	return id, nil
}

func (d *DB) GetProjectByPath(path string) (*Project, error) {
	p := &Project{}
	err := d.db.QueryRow(`
		SELECT id, name, path, languages, branch, dirty, dirty_files,
			last_commit_at, last_commit_msg, ahead, behind, status,
			discovered_at, last_scanned_at
		FROM projects WHERE path = ?`, path).Scan(
		&p.ID, &p.Name, &p.Path, &p.Languages, &p.Branch, &p.Dirty, &p.DirtyFiles,
		&p.LastCommitAt, &p.LastCommitMsg, &p.Ahead, &p.Behind, &p.Status,
		&p.DiscoveredAt, &p.LastScannedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get project: %w", err)
	}
	return p, nil
}

func (d *DB) GetProjectByName(name string) (*Project, error) {
	p := &Project{}
	err := d.db.QueryRow(`
		SELECT id, name, path, languages, branch, dirty, dirty_files,
			last_commit_at, last_commit_msg, ahead, behind, status,
			discovered_at, last_scanned_at
		FROM projects WHERE name = ?`, name).Scan(
		&p.ID, &p.Name, &p.Path, &p.Languages, &p.Branch, &p.Dirty, &p.DirtyFiles,
		&p.LastCommitAt, &p.LastCommitMsg, &p.Ahead, &p.Behind, &p.Status,
		&p.DiscoveredAt, &p.LastScannedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get project by name: %w", err)
	}
	return p, nil
}

func (d *DB) ListProjects(status string) ([]Project, error) {
	var rows *sql.Rows
	var err error

	if status == "" {
		rows, err = d.db.Query(`
			SELECT id, name, path, languages, branch, dirty, dirty_files,
				last_commit_at, last_commit_msg, ahead, behind, status,
				discovered_at, last_scanned_at
			FROM projects WHERE status != 'archived' ORDER BY name`)
	} else {
		rows, err = d.db.Query(`
			SELECT id, name, path, languages, branch, dirty, dirty_files,
				last_commit_at, last_commit_msg, ahead, behind, status,
				discovered_at, last_scanned_at
			FROM projects WHERE status = ? ORDER BY name`, status)
	}
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	var projects []Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.Name, &p.Path, &p.Languages, &p.Branch,
			&p.Dirty, &p.DirtyFiles, &p.LastCommitAt, &p.LastCommitMsg,
			&p.Ahead, &p.Behind, &p.Status, &p.DiscoveredAt, &p.LastScannedAt); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		projects = append(projects, p)
	}
	return projects, nil
}

func (d *DB) ListDirtyProjects() ([]Project, error) {
	rows, err := d.db.Query(`
		SELECT id, name, path, languages, branch, dirty, dirty_files,
			last_commit_at, last_commit_msg, ahead, behind, status,
			discovered_at, last_scanned_at
		FROM projects WHERE dirty = 1 ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list dirty: %w", err)
	}
	defer rows.Close()

	var projects []Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.Name, &p.Path, &p.Languages, &p.Branch,
			&p.Dirty, &p.DirtyFiles, &p.LastCommitAt, &p.LastCommitMsg,
			&p.Ahead, &p.Behind, &p.Status, &p.DiscoveredAt, &p.LastScannedAt); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		projects = append(projects, p)
	}
	return projects, nil
}
```

- [ ] **Step 4: Remove duplicate Project struct from test file**

The test file defined its own `Project` struct — remove it since `projects.go` defines the real one.

- [ ] **Step 5: Run tests**

Run: `cd ~/projects-wsl/nexus && go test ./internal/db/ -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/db/projects.go internal/db/projects_test.go
git commit -m "feat: projects CRUD with upsert, list by status, and dirty filter"
```

---

### Task 6: Database Layer — Sessions CRUD + FTS

**Files:**
- Create: `internal/db/sessions.go`
- Create: `internal/db/sessions_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/db/sessions_test.go
package db

import (
	"testing"
	"time"
)

func TestInsertAndListSessions(t *testing.T) {
	d := testDB(t)

	now := time.Now()
	pID, _ := d.UpsertProject(Project{Name: "proj", Path: "/a", Status: "active", DiscoveredAt: now})

	s := Session{
		ProjectID: pID,
		StartedAt: &now,
		Summary:   "Added retry logic to DNS scanner",
		Source:    "wrapper",
		Tags:      `["go","security"]`,
	}

	id, err := d.InsertSession(s)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if id == 0 {
		t.Fatal("expected non-zero id")
	}

	sessions, err := d.ListSessions(SessionFilter{Limit: 10})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].Summary != "Added retry logic to DNS scanner" {
		t.Errorf("unexpected summary: %s", sessions[0].Summary)
	}
}

func TestListSessionsByProject(t *testing.T) {
	d := testDB(t)

	now := time.Now()
	p1, _ := d.UpsertProject(Project{Name: "proj1", Path: "/a", Status: "active", DiscoveredAt: now})
	p2, _ := d.UpsertProject(Project{Name: "proj2", Path: "/b", Status: "active", DiscoveredAt: now})

	d.InsertSession(Session{ProjectID: p1, Summary: "session 1", Source: "wrapper", StartedAt: &now})
	d.InsertSession(Session{ProjectID: p2, Summary: "session 2", Source: "wrapper", StartedAt: &now})
	d.InsertSession(Session{ProjectID: p1, Summary: "session 3", Source: "wrapper", StartedAt: &now})

	sessions, err := d.ListSessions(SessionFilter{ProjectID: p1, Limit: 10})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(sessions) != 2 {
		t.Errorf("expected 2 sessions for proj1, got %d", len(sessions))
	}
}

func TestSearchSessions(t *testing.T) {
	d := testDB(t)

	now := time.Now()
	pID, _ := d.UpsertProject(Project{Name: "proj", Path: "/a", Status: "active", DiscoveredAt: now})

	d.InsertSession(Session{ProjectID: pID, Summary: "Added retry logic to DNS scanner", Source: "wrapper", StartedAt: &now})
	d.InsertSession(Session{ProjectID: pID, Summary: "Fixed database migration bug", Source: "wrapper", StartedAt: &now})
	d.InsertSession(Session{ProjectID: pID, Summary: "Refactored HTTP client with retry", Source: "wrapper", StartedAt: &now})

	results, err := d.SearchSessions("retry")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results for 'retry', got %d", len(results))
	}
}

func TestSessionDedup(t *testing.T) {
	d := testDB(t)

	now := time.Now()
	pID, _ := d.UpsertProject(Project{Name: "proj", Path: "/a", Status: "active", DiscoveredAt: now})

	_, err := d.InsertSession(Session{
		ProjectID:      pID,
		ClaudeSessionID: "abc-123",
		Summary:        "first",
		Source:         "wrapper",
		StartedAt:      &now,
	})
	if err != nil {
		t.Fatalf("first insert: %v", err)
	}

	// Same claude_session_id should be detected as duplicate
	exists, err := d.SessionExists("abc-123")
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if !exists {
		t.Error("expected session to exist")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd ~/projects-wsl/nexus && go test ./internal/db/ -v -run TestInsert`
Expected: FAIL

- [ ] **Step 3: Implement sessions.go**

```go
// internal/db/sessions.go
package db

import (
	"database/sql"
	"fmt"
	"time"
)

type Session struct {
	ID              int64
	ProjectID       int64
	ClaudeSessionID string
	StartedAt       *time.Time
	EndedAt         *time.Time
	DurationSecs    int
	Summary         string
	FilesChanged    string
	CommitsMade     string
	Tags            string
	Source          string
	CreatedAt       time.Time
	// Joined fields
	ProjectName string
}

type SessionFilter struct {
	ProjectID int64
	Since     *time.Time
	Limit     int
}

func (d *DB) InsertSession(s Session) (int64, error) {
	result, err := d.db.Exec(`
		INSERT INTO sessions (project_id, claude_session_id, started_at, ended_at,
			duration_secs, summary, files_changed, commits_made, tags, source)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.ProjectID, nilIfEmpty(s.ClaudeSessionID), s.StartedAt, s.EndedAt,
		s.DurationSecs, s.Summary, defaultJSON(s.FilesChanged),
		defaultJSON(s.CommitsMade), defaultJSON(s.Tags), s.Source)
	if err != nil {
		return 0, fmt.Errorf("insert session: %w", err)
	}
	return result.LastInsertId()
}

func (d *DB) SessionExists(claudeSessionID string) (bool, error) {
	var count int
	err := d.db.QueryRow(
		"SELECT count(*) FROM sessions WHERE claude_session_id = ?",
		claudeSessionID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (d *DB) HasOverlappingSession(projectID int64, start, end time.Time) (bool, error) {
	var count int
	err := d.db.QueryRow(`
		SELECT count(*) FROM sessions
		WHERE project_id = ?
		AND started_at <= ? AND ended_at >= ?`,
		projectID, end, start).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (d *DB) ListSessions(f SessionFilter) ([]Session, error) {
	query := `
		SELECT s.id, s.project_id, COALESCE(s.claude_session_id, ''),
			s.started_at, s.ended_at, s.duration_secs, COALESCE(s.summary, ''),
			s.files_changed, s.commits_made, s.tags, s.source, s.created_at,
			p.name
		FROM sessions s
		JOIN projects p ON p.id = s.project_id
		WHERE 1=1`
	var args []interface{}

	if f.ProjectID > 0 {
		query += " AND s.project_id = ?"
		args = append(args, f.ProjectID)
	}
	if f.Since != nil {
		query += " AND s.started_at >= ?"
		args = append(args, f.Since)
	}

	query += " ORDER BY s.started_at DESC"

	if f.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", f.Limit)
	}

	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var s Session
		if err := rows.Scan(&s.ID, &s.ProjectID, &s.ClaudeSessionID,
			&s.StartedAt, &s.EndedAt, &s.DurationSecs, &s.Summary,
			&s.FilesChanged, &s.CommitsMade, &s.Tags, &s.Source, &s.CreatedAt,
			&s.ProjectName); err != nil {
			return nil, fmt.Errorf("scan session: %w", err)
		}
		sessions = append(sessions, s)
	}
	return sessions, nil
}

func (d *DB) SearchSessions(query string) ([]Session, error) {
	rows, err := d.db.Query(`
		SELECT s.id, s.project_id, COALESCE(s.claude_session_id, ''),
			s.started_at, s.ended_at, s.duration_secs, COALESCE(s.summary, ''),
			s.files_changed, s.commits_made, s.tags, s.source, s.created_at,
			p.name
		FROM sessions_fts fts
		JOIN sessions s ON s.id = fts.rowid
		JOIN projects p ON p.id = s.project_id
		WHERE sessions_fts MATCH ?
		ORDER BY rank`, query)
	if err != nil {
		return nil, fmt.Errorf("search sessions: %w", err)
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var s Session
		if err := rows.Scan(&s.ID, &s.ProjectID, &s.ClaudeSessionID,
			&s.StartedAt, &s.EndedAt, &s.DurationSecs, &s.Summary,
			&s.FilesChanged, &s.CommitsMade, &s.Tags, &s.Source, &s.CreatedAt,
			&s.ProjectName); err != nil {
			return nil, fmt.Errorf("scan search result: %w", err)
		}
		sessions = append(sessions, s)
	}
	return sessions, nil
}

func nilIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func defaultJSON(s string) string {
	if s == "" {
		return "[]"
	}
	return s
}
```

- [ ] **Step 4: Run tests**

Run: `cd ~/projects-wsl/nexus && go test ./internal/db/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/db/sessions.go internal/db/sessions_test.go
git commit -m "feat: sessions CRUD with FTS5 search and deduplication"
```

---

### Task 7: Database Layer — Notes CRUD

**Files:**
- Create: `internal/db/notes.go`
- Create: `internal/db/notes_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/db/notes_test.go
package db

import (
	"testing"
	"time"
)

func TestInsertAndSearchNotes(t *testing.T) {
	d := testDB(t)

	now := time.Now()
	pID, _ := d.UpsertProject(Project{Name: "proj", Path: "/a", Status: "active", DiscoveredAt: now})

	_, err := d.InsertNote(Note{ProjectID: &pID, Content: "migrated auth to JWT"})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	_, err = d.InsertNote(Note{ProjectID: &pID, Content: "refactored database layer"})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	results, err := d.SearchNotes("JWT")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestGlobalNote(t *testing.T) {
	d := testDB(t)

	_, err := d.InsertNote(Note{Content: "general thought"})
	if err != nil {
		t.Fatalf("insert global note: %v", err)
	}

	notes, err := d.ListNotes(0, 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(notes) != 1 {
		t.Errorf("expected 1 note, got %d", len(notes))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd ~/projects-wsl/nexus && go test ./internal/db/ -v -run TestInsertAndSearchNotes`
Expected: FAIL

- [ ] **Step 3: Implement notes.go**

```go
// internal/db/notes.go
package db

import (
	"fmt"
	"time"
)

type Note struct {
	ID        int64
	ProjectID *int64
	SessionID *int64
	Content   string
	CreatedAt time.Time
}

func (d *DB) InsertNote(n Note) (int64, error) {
	result, err := d.db.Exec(`
		INSERT INTO notes (project_id, session_id, content, created_at)
		VALUES (?, ?, ?, datetime('now'))`,
		n.ProjectID, n.SessionID, n.Content)
	if err != nil {
		return 0, fmt.Errorf("insert note: %w", err)
	}
	return result.LastInsertId()
}

func (d *DB) ListNotes(projectID int64, limit int) ([]Note, error) {
	var query string
	var args []interface{}

	if projectID > 0 {
		query = "SELECT id, project_id, session_id, content, created_at FROM notes WHERE project_id = ? ORDER BY created_at DESC LIMIT ?"
		args = []interface{}{projectID, limit}
	} else {
		query = "SELECT id, project_id, session_id, content, created_at FROM notes ORDER BY created_at DESC LIMIT ?"
		args = []interface{}{limit}
	}

	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list notes: %w", err)
	}
	defer rows.Close()

	var notes []Note
	for rows.Next() {
		var n Note
		if err := rows.Scan(&n.ID, &n.ProjectID, &n.SessionID, &n.Content, &n.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan note: %w", err)
		}
		notes = append(notes, n)
	}
	return notes, nil
}

func (d *DB) SearchNotes(query string) ([]Note, error) {
	rows, err := d.db.Query(`
		SELECT n.id, n.project_id, n.session_id, n.content, n.created_at
		FROM notes_fts fts
		JOIN notes n ON n.id = fts.rowid
		WHERE notes_fts MATCH ?
		ORDER BY rank`, query)
	if err != nil {
		return nil, fmt.Errorf("search notes: %w", err)
	}
	defer rows.Close()

	var notes []Note
	for rows.Next() {
		var n Note
		if err := rows.Scan(&n.ID, &n.ProjectID, &n.SessionID, &n.Content, &n.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan note: %w", err)
		}
		notes = append(notes, n)
	}
	return notes, nil
}
```

- [ ] **Step 4: Run tests**

Run: `cd ~/projects-wsl/nexus && go test ./internal/db/ -v`
Expected: PASS (all db tests)

- [ ] **Step 5: Commit**

```bash
git add internal/db/notes.go internal/db/notes_test.go
git commit -m "feat: notes CRUD with FTS5 search"
```

---

### Task 8: Scanner — Git Operations

**Files:**
- Create: `internal/scanner/git.go`
- Create: `internal/scanner/git_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/scanner/git_test.go
package scanner

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// createTestRepo creates a git repo with one commit and returns its path.
func createTestRepo(t *testing.T, name string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), name)
	os.MkdirAll(dir, 0755)

	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s: %v", args, out, err)
		}
	}

	run("init", "-b", "main")
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# test"), 0644)
	run("add", ".")
	run("commit", "-m", "initial commit")

	return dir
}

func TestGetBranch(t *testing.T) {
	repo := createTestRepo(t, "test-repo")
	branch, err := GetBranch(repo)
	if err != nil {
		t.Fatalf("GetBranch: %v", err)
	}
	if branch != "main" {
		t.Errorf("expected main, got %s", branch)
	}
}

func TestGetDirtyFiles(t *testing.T) {
	repo := createTestRepo(t, "test-repo")

	// Clean initially
	count, err := GetDirtyFileCount(repo)
	if err != nil {
		t.Fatalf("GetDirtyFileCount: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 dirty, got %d", count)
	}

	// Create untracked file
	os.WriteFile(filepath.Join(repo, "new.txt"), []byte("new"), 0644)
	count, _ = GetDirtyFileCount(repo)
	if count != 1 {
		t.Errorf("expected 1 dirty, got %d", count)
	}
}

func TestGetLastCommit(t *testing.T) {
	repo := createTestRepo(t, "test-repo")

	msg, ts, err := GetLastCommit(repo)
	if err != nil {
		t.Fatalf("GetLastCommit: %v", err)
	}
	if msg != "initial commit" {
		t.Errorf("expected 'initial commit', got '%s'", msg)
	}
	if time.Since(ts) > time.Minute {
		t.Errorf("commit time too old: %v", ts)
	}
}

func TestDetectLanguages(t *testing.T) {
	repo := createTestRepo(t, "test-repo")
	os.WriteFile(filepath.Join(repo, "main.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(repo, "app.ts"), []byte("console.log()"), 0644)

	langs := DetectLanguages(repo)
	found := map[string]bool{}
	for _, l := range langs {
		found[l] = true
	}
	if !found["go"] {
		t.Error("expected go in languages")
	}
	if !found["typescript"] {
		t.Error("expected typescript in languages")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd ~/projects-wsl/nexus && go test ./internal/scanner/ -v`
Expected: FAIL

- [ ] **Step 3: Implement git.go**

```go
// internal/scanner/git.go
package scanner

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func gitCmd(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}

func IsGitRepo(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

func GetBranch(dir string) (string, error) {
	return gitCmd(dir, "rev-parse", "--abbrev-ref", "HEAD")
}

func GetDirtyFileCount(dir string) (int, error) {
	out, err := gitCmd(dir, "status", "--porcelain")
	if err != nil {
		return 0, err
	}
	if out == "" {
		return 0, nil
	}
	return len(strings.Split(out, "\n")), nil
}

func GetLastCommit(dir string) (message string, when time.Time, err error) {
	msg, err := gitCmd(dir, "log", "-1", "--format=%s")
	if err != nil {
		return "", time.Time{}, err
	}

	ts, err := gitCmd(dir, "log", "-1", "--format=%aI")
	if err != nil {
		return "", time.Time{}, err
	}

	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("parse time: %w", err)
	}

	return msg, t, nil
}

func GetAheadBehind(dir string) (ahead, behind int, err error) {
	out, err := gitCmd(dir, "rev-list", "--left-right", "--count", "HEAD...@{upstream}")
	if err != nil {
		// No upstream configured
		return 0, 0, nil
	}
	fmt.Sscanf(out, "%d\t%d", &ahead, &behind)
	return ahead, behind, nil
}

func GetCommitsSince(dir string, since time.Time) ([]CommitInfo, error) {
	out, err := gitCmd(dir, "log", "--since="+since.Format(time.RFC3339),
		"--format=%H|%s|%aI", "--no-merges")
	if err != nil || out == "" {
		return nil, err
	}

	var commits []CommitInfo
	for _, line := range strings.Split(out, "\n") {
		parts := strings.SplitN(line, "|", 3)
		if len(parts) != 3 {
			continue
		}
		ts, _ := time.Parse(time.RFC3339, parts[2])
		commits = append(commits, CommitInfo{
			Hash:    parts[0][:8],
			Message: parts[1],
			Time:    ts,
		})
	}
	return commits, nil
}

type CommitInfo struct {
	Hash    string
	Message string
	Time    time.Time
}

func GetChangedFiles(dir string, since time.Time) ([]string, error) {
	out, err := gitCmd(dir, "diff", "--name-only", "--since="+since.Format(time.RFC3339), "HEAD")
	if err != nil {
		// Fallback: get status
		out, err = gitCmd(dir, "diff", "--name-only", "HEAD")
		if err != nil {
			return nil, err
		}
	}
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}

func GetStaleBranches(dir string, olderThan time.Duration) ([]string, error) {
	out, err := gitCmd(dir, "for-each-ref", "--sort=-committerdate",
		"--format=%(refname:short)|%(committerdate:iso-strict)", "refs/heads/")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}

	cutoff := time.Now().Add(-olderThan)
	var stale []string
	for _, line := range strings.Split(out, "\n") {
		parts := strings.SplitN(line, "|", 2)
		if len(parts) != 2 {
			continue
		}
		ts, err := time.Parse(time.RFC3339, parts[1])
		if err != nil {
			continue
		}
		if ts.Before(cutoff) {
			stale = append(stale, parts[0])
		}
	}
	return stale, nil
}

var langMap = map[string]string{
	".go":   "go",
	".ts":   "typescript",
	".tsx":  "typescript",
	".js":   "javascript",
	".jsx":  "javascript",
	".py":   "python",
	".rs":   "rust",
	".java": "java",
	".rb":   "ruby",
	".tf":   "terraform",
	".yaml": "yaml",
	".yml":  "yaml",
	".sh":   "shell",
}

func DetectLanguages(dir string) []string {
	seen := map[string]bool{}
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		// Skip hidden dirs and common noise
		name := info.Name()
		if info.IsDir() && (strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor") {
			return filepath.SkipDir
		}
		ext := filepath.Ext(name)
		if lang, ok := langMap[ext]; ok {
			seen[lang] = true
		}
		return nil
	})

	var langs []string
	for l := range seen {
		langs = append(langs, l)
	}
	return langs
}
```

- [ ] **Step 4: Run tests**

Run: `cd ~/projects-wsl/nexus && go test ./internal/scanner/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/scanner/git.go internal/scanner/git_test.go
git commit -m "feat: git operations - branch, status, commits, languages, stale branches"
```

---

### Task 9: Scanner — Directory Walker

**Files:**
- Create: `internal/scanner/scanner.go`
- Create: `internal/scanner/scanner_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/scanner/scanner_test.go
package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverProjects(t *testing.T) {
	root := t.TempDir()

	// Create 3 git repos
	createTestRepo(t, filepath.Join(root, "project-a"))
	createTestRepo(t, filepath.Join(root, "project-b"))
	os.MkdirAll(filepath.Join(root, "not-a-repo"), 0755) // no .git

	// Nested repo
	createTestRepo(t, filepath.Join(root, "nested", "project-c"))

	projects, err := Discover([]string{root}, nil)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if len(projects) != 3 {
		t.Errorf("expected 3 projects, got %d: %v", len(projects), projects)
	}
}

func TestDiscoverRespectsExclusions(t *testing.T) {
	root := t.TempDir()

	createTestRepo(t, filepath.Join(root, "project-a"))
	createTestRepo(t, filepath.Join(root, "scratch-temp"))

	projects, err := Discover([]string{root}, []string{"*/scratch-*"})
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if len(projects) != 1 {
		t.Errorf("expected 1 project (excluded scratch), got %d", len(projects))
	}
}

func TestDiscoverSkipsNestedGit(t *testing.T) {
	root := t.TempDir()
	repo := createTestRepo(t, filepath.Join(root, "project"))

	// Create node_modules with its own .git (shouldn't be discovered)
	os.MkdirAll(filepath.Join(repo, "node_modules", "some-pkg", ".git"), 0755)

	projects, err := Discover([]string{root}, []string{"*/node_modules/*"})
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if len(projects) != 1 {
		t.Errorf("expected 1 project, got %d", len(projects))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd ~/projects-wsl/nexus && go test ./internal/scanner/ -v -run TestDiscover`
Expected: FAIL

- [ ] **Step 3: Implement scanner.go**

```go
// internal/scanner/scanner.go
package scanner

import (
	"os"
	"path/filepath"
	"strings"
)

// Discover walks the given roots and returns absolute paths to directories
// containing a .git folder, skipping paths matching exclusion patterns.
func Discover(roots []string, exclude []string) ([]string, error) {
	var projects []string
	seen := map[string]bool{}

	for _, root := range roots {
		root, err := filepath.Abs(root)
		if err != nil {
			continue
		}

		filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}

			if !info.IsDir() {
				return nil
			}

			// Check exclusions
			for _, pattern := range exclude {
				if matched, _ := filepath.Match(pattern, path); matched {
					return filepath.SkipDir
				}
				// Also check if any path component matches
				if matchPathPattern(path, pattern) {
					return filepath.SkipDir
				}
			}

			// Check if this dir has a .git folder
			gitDir := filepath.Join(path, ".git")
			if info, err := os.Stat(gitDir); err == nil && info.IsDir() {
				absPath, _ := filepath.Abs(path)
				if !seen[absPath] {
					seen[absPath] = true
					projects = append(projects, absPath)
				}
				return filepath.SkipDir // Don't descend into git repos
			}

			return nil
		})
	}

	return projects, nil
}

func matchPathPattern(path, pattern string) bool {
	// Handle patterns like "*/node_modules/*"
	parts := strings.Split(pattern, "/")
	for _, part := range parts {
		if part == "*" {
			continue
		}
		if strings.Contains(path, "/"+part+"/") || strings.HasSuffix(path, "/"+part) {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Fix createTestRepo to accept full path**

Update `createTestRepo` in `git_test.go` to handle the case where the full path is passed instead of just a name:

```go
func createTestRepo(t *testing.T, dir string) string {
	t.Helper()
	os.MkdirAll(dir, 0755)

	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s: %v", args, out, err)
		}
	}

	run("init", "-b", "main")
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# test"), 0644)
	run("add", ".")
	run("commit", "-m", "initial commit")

	return dir
}
```

- [ ] **Step 5: Run tests**

Run: `cd ~/projects-wsl/nexus && go test ./internal/scanner/ -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/scanner/scanner.go internal/scanner/scanner_test.go internal/scanner/git.go internal/scanner/git_test.go
git commit -m "feat: project discovery with exclusion patterns and nested repo handling"
```

---

### Task 10: Capture — Claude Session Reader

**Files:**
- Create: `internal/capture/claude.go`
- Create: `internal/capture/claude_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/capture/claude_test.go
package capture

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFindLatestSession(t *testing.T) {
	// Create mock Claude session dir
	claudeDir := t.TempDir()
	sessionsDir := filepath.Join(claudeDir, "sessions")
	os.MkdirAll(sessionsDir, 0755)

	// Create a session file
	session := map[string]interface{}{
		"pid":       12345,
		"sessionId": "abc-123-def",
		"cwd":       "/home/user/projects/wraith",
		"startedAt": time.Now().Add(-time.Hour).Format(time.RFC3339),
	}
	data, _ := json.Marshal(session)
	os.WriteFile(filepath.Join(sessionsDir, "abc-123-def"), data, 0644)

	result, err := FindLatestSession(claudeDir, "/home/user/projects/wraith")
	if err != nil {
		t.Fatalf("FindLatestSession: %v", err)
	}
	if result == nil {
		t.Fatal("expected session, got nil")
	}
	if result.SessionID != "abc-123-def" {
		t.Errorf("expected abc-123-def, got %s", result.SessionID)
	}
}

func TestFindLatestSessionNoMatch(t *testing.T) {
	claudeDir := t.TempDir()
	sessionsDir := filepath.Join(claudeDir, "sessions")
	os.MkdirAll(sessionsDir, 0755)

	session := map[string]interface{}{
		"sessionId": "abc-123",
		"cwd":       "/home/user/projects/other",
		"startedAt": time.Now().Format(time.RFC3339),
	}
	data, _ := json.Marshal(session)
	os.WriteFile(filepath.Join(sessionsDir, "abc-123"), data, 0644)

	result, err := FindLatestSession(claudeDir, "/home/user/projects/wraith")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Error("expected nil for non-matching cwd")
	}
}

func TestFindLatestSessionMissingDir(t *testing.T) {
	result, err := FindLatestSession("/nonexistent", "/some/path")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Error("expected nil")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd ~/projects-wsl/nexus && go test ./internal/capture/ -v`
Expected: FAIL

- [ ] **Step 3: Implement claude.go**

```go
// internal/capture/claude.go
package capture

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type ClaudeSession struct {
	SessionID string
	CWD       string
	StartedAt time.Time
}

type claudeSessionFile struct {
	PID       int    `json:"pid"`
	SessionID string `json:"sessionId"`
	CWD       string `json:"cwd"`
	StartedAt string `json:"startedAt"`
}

// FindLatestSession looks in the Claude sessions directory for the most recent
// session matching the given working directory. Returns nil if none found.
func FindLatestSession(claudeDir string, workDir string) (*ClaudeSession, error) {
	sessionsDir := filepath.Join(claudeDir, "sessions")
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var latest *ClaudeSession
	var latestTime time.Time

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		data, err := os.ReadFile(filepath.Join(sessionsDir, entry.Name()))
		if err != nil {
			continue
		}

		var sf claudeSessionFile
		if err := json.Unmarshal(data, &sf); err != nil {
			continue
		}

		if sf.CWD != workDir {
			continue
		}

		startedAt, err := time.Parse(time.RFC3339, sf.StartedAt)
		if err != nil {
			continue
		}

		if latest == nil || startedAt.After(latestTime) {
			latest = &ClaudeSession{
				SessionID: sf.SessionID,
				CWD:       sf.CWD,
				StartedAt: startedAt,
			}
			latestTime = startedAt
		}
	}

	return latest, nil
}

// DefaultClaudeDir returns the default Claude config directory.
func DefaultClaudeDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude")
}
```

- [ ] **Step 4: Run tests**

Run: `cd ~/projects-wsl/nexus && go test ./internal/capture/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/capture/claude.go internal/capture/claude_test.go
git commit -m "feat: Claude session reader with cwd matching"
```

---

### Task 11: Capture — Summary Generator

**Files:**
- Create: `internal/capture/summary.go`
- Create: `internal/capture/summary_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/capture/summary_test.go
package capture

import (
	"testing"

	"github.com/digitalghost404/nexus/internal/scanner"
)

func TestGenerateSummaryFromCommits(t *testing.T) {
	commits := []scanner.CommitInfo{
		{Hash: "abc123", Message: "feat: add retry logic"},
		{Hash: "def456", Message: "fix: resolve DNS timeout"},
	}

	summary := GenerateSummary(commits, nil)
	if summary == "" {
		t.Fatal("expected non-empty summary")
	}
	if summary != "feat: add retry logic; fix: resolve DNS timeout" {
		t.Errorf("unexpected summary: %s", summary)
	}
}

func TestGenerateSummaryNoCommits(t *testing.T) {
	summary := GenerateSummary(nil, nil)
	if summary != "" {
		t.Errorf("expected empty summary, got: %s", summary)
	}
}

func TestGenerateSummaryWithFiles(t *testing.T) {
	files := []string{"cmd/scan.go", "internal/scanner.go"}
	summary := GenerateSummary(nil, files)
	if summary == "" {
		t.Fatal("expected non-empty summary from files")
	}
}

func TestGenerateTagsFromLanguagesAndProject(t *testing.T) {
	tags := GenerateTags("wraith", []string{"go", "typescript"})
	tagMap := map[string]bool{}
	for _, tag := range tags {
		tagMap[tag] = true
	}
	if !tagMap["wraith"] {
		t.Error("expected wraith tag")
	}
	if !tagMap["go"] {
		t.Error("expected go tag")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd ~/projects-wsl/nexus && go test ./internal/capture/ -v -run TestGenerate`
Expected: FAIL

- [ ] **Step 3: Implement summary.go**

```go
// internal/capture/summary.go
package capture

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/digitalghost404/nexus/internal/scanner"
)

// GenerateSummary creates a summary string from commits and/or changed files.
func GenerateSummary(commits []scanner.CommitInfo, files []string) string {
	if len(commits) > 0 {
		var msgs []string
		for _, c := range commits {
			msgs = append(msgs, c.Message)
		}
		return strings.Join(msgs, "; ")
	}

	if len(files) > 0 {
		if len(files) <= 3 {
			return fmt.Sprintf("Changed: %s", strings.Join(files, ", "))
		}
		return fmt.Sprintf("Changed %d files including %s", len(files), strings.Join(files[:3], ", "))
	}

	return ""
}

// GenerateTags creates tags from project name and detected languages.
func GenerateTags(projectName string, languages []string) []string {
	tags := []string{projectName}
	tags = append(tags, languages...)
	return tags
}

// TagsToJSON converts a string slice to a JSON array string.
func TagsToJSON(tags []string) string {
	if len(tags) == 0 {
		return "[]"
	}
	data, _ := json.Marshal(tags)
	return string(data)
}

// CommitsToJSON converts commits to a JSON array string.
func CommitsToJSON(commits []scanner.CommitInfo) string {
	if len(commits) == 0 {
		return "[]"
	}
	type entry struct {
		Hash    string `json:"hash"`
		Message string `json:"message"`
	}
	var entries []entry
	for _, c := range commits {
		entries = append(entries, entry{Hash: c.Hash, Message: c.Message})
	}
	data, _ := json.Marshal(entries)
	return string(data)
}

// FilesToJSON converts a file list to a JSON array string.
func FilesToJSON(files []string) string {
	if len(files) == 0 {
		return "[]"
	}
	data, _ := json.Marshal(files)
	return string(data)
}
```

- [ ] **Step 4: Run tests**

Run: `cd ~/projects-wsl/nexus && go test ./internal/capture/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/capture/summary.go internal/capture/summary_test.go
git commit -m "feat: summary and tag generation from git data"
```

---

### Task 12: Capture — Orchestrator

**Files:**
- Create: `internal/capture/capture.go`
- Create: `internal/capture/capture_test.go`

- [ ] **Step 1: Write failing test**

```go
// internal/capture/capture_test.go
package capture

import (
	"path/filepath"
	"testing"

	"github.com/digitalghost404/nexus/internal/db"
	"github.com/digitalghost404/nexus/internal/scanner"
)

func testDB(t *testing.T) *db.DB {
	t.Helper()
	dir := t.TempDir()
	d, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func TestCaptureSession(t *testing.T) {
	d := testDB(t)
	repo := scanner.CreateTestRepo(t, filepath.Join(t.TempDir(), "project"))

	result, err := CaptureSession(d, repo, "")
	if err != nil {
		t.Fatalf("capture: %v", err)
	}

	if result.ProjectName != "project" {
		t.Errorf("expected project, got %s", result.ProjectName)
	}

	// Verify session was stored
	sessions, err := d.ListSessions(db.SessionFilter{Limit: 10})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(sessions) != 1 {
		t.Errorf("expected 1 session, got %d", len(sessions))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd ~/projects-wsl/nexus && go test ./internal/capture/ -v -run TestCaptureSession`
Expected: FAIL

- [ ] **Step 3: Export createTestRepo from scanner package**

Add to `internal/scanner/git_test.go` or create `internal/scanner/testhelper.go`:

```go
// internal/scanner/testhelper.go
package scanner

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// CreateTestRepo creates a git repo for testing. Exported for use by other packages.
func CreateTestRepo(t *testing.T, dir string) string {
	t.Helper()
	os.MkdirAll(dir, 0755)

	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s: %v", args, out, err)
		}
	}

	run("init", "-b", "main")
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# test"), 0644)
	run("add", ".")
	run("commit", "-m", "initial commit")

	return dir
}
```

- [ ] **Step 4: Implement capture.go**

```go
// internal/capture/capture.go
package capture

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/digitalghost404/nexus/internal/db"
	"github.com/digitalghost404/nexus/internal/scanner"
)

type CaptureResult struct {
	ProjectName string
	SessionID   int64
	Summary     string
	Commits     int
	Files       int
}

// CaptureSession captures a Claude session for the given directory.
// claudeDir can be empty to use the default ~/.claude/ path.
func CaptureSession(database *db.DB, workDir string, claudeDir string) (*CaptureResult, error) {
	absDir, err := filepath.Abs(workDir)
	if err != nil {
		return nil, fmt.Errorf("resolve path: %w", err)
	}

	projectName := filepath.Base(absDir)

	// Ensure project exists in DB
	now := time.Now()
	projectID, err := database.UpsertProject(db.Project{
		Name:         projectName,
		Path:         absDir,
		Status:       "active",
		DiscoveredAt: now,
	})
	if err != nil {
		return nil, fmt.Errorf("upsert project: %w", err)
	}

	// Try to find Claude session info
	if claudeDir == "" {
		claudeDir = DefaultClaudeDir()
	}

	var startedAt *time.Time
	var claudeSessionID string

	claudeSession, _ := FindLatestSession(claudeDir, absDir)
	if claudeSession != nil {
		startedAt = &claudeSession.StartedAt
		claudeSessionID = claudeSession.SessionID

		// Check for duplicate
		exists, _ := database.SessionExists(claudeSessionID)
		if exists {
			return &CaptureResult{ProjectName: projectName, Summary: "duplicate session, skipped"}, nil
		}
	}

	// Fallback: use 8-hour window
	if startedAt == nil {
		t := now.Add(-8 * time.Hour)
		startedAt = &t
	}

	// Gather git data
	var commits []scanner.CommitInfo
	var files []string
	var languages []string

	if scanner.IsGitRepo(absDir) {
		commits, _ = scanner.GetCommitsSince(absDir, *startedAt)
		files, _ = scanner.GetChangedFiles(absDir, *startedAt)
		languages = scanner.DetectLanguages(absDir)
	}

	summary := GenerateSummary(commits, files)
	tags := GenerateTags(projectName, languages)

	sessionID, err := database.InsertSession(db.Session{
		ProjectID:       projectID,
		ClaudeSessionID: claudeSessionID,
		StartedAt:       startedAt,
		EndedAt:         &now,
		DurationSecs:    int(now.Sub(*startedAt).Seconds()),
		Summary:         summary,
		FilesChanged:    FilesToJSON(files),
		CommitsMade:     CommitsToJSON(commits),
		Tags:            TagsToJSON(tags),
		Source:          "wrapper",
	})
	if err != nil {
		return nil, fmt.Errorf("insert session: %w", err)
	}

	return &CaptureResult{
		ProjectName: projectName,
		SessionID:   sessionID,
		Summary:     summary,
		Commits:     len(commits),
		Files:       len(files),
	}, nil
}
```

- [ ] **Step 5: Run tests**

Run: `cd ~/projects-wsl/nexus && go test ./internal/capture/ -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/capture/capture.go internal/capture/capture_test.go internal/scanner/testhelper.go
git commit -m "feat: session capture orchestrator with Claude session detection and git data"
```

---

### Task 13: Display Formatting

**Files:**
- Create: `internal/display/display.go`
- Create: `internal/display/display_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/display/display_test.go
package display

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/digitalghost404/nexus/internal/db"
)

func TestFormatSmartSummary(t *testing.T) {
	var buf bytes.Buffer
	now := time.Now()
	lastSession := now.Add(-2 * time.Hour)

	dirty := []db.Project{
		{Name: "wraith", Branch: "main", DirtyFiles: 3},
	}
	sessions := []db.Session{
		{ProjectName: "wraith", Summary: "Added retry logic", StartedAt: &lastSession},
	}
	stale := []db.Project{
		{Name: "vibe-chatbot"},
	}

	FormatSmartSummary(&buf, dirty, sessions, stale, "")
	output := buf.String()

	if !strings.Contains(output, "NEXUS") {
		t.Error("expected NEXUS header")
	}
	if !strings.Contains(output, "wraith") {
		t.Error("expected wraith in output")
	}
	if !strings.Contains(output, "vibe-chatbot") {
		t.Error("expected stale project in output")
	}
}

func TestFormatProjectTable(t *testing.T) {
	var buf bytes.Buffer
	projects := []db.Project{
		{Name: "wraith", Branch: "main", Status: "active", DirtyFiles: 3, Dirty: true},
		{Name: "cortex", Branch: "develop", Status: "idle", DirtyFiles: 0},
	}

	FormatProjectTable(&buf, projects)
	output := buf.String()

	if !strings.Contains(output, "wraith") {
		t.Error("expected wraith")
	}
	if !strings.Contains(output, "cortex") {
		t.Error("expected cortex")
	}
}

func TestFormatRelativeTime(t *testing.T) {
	now := time.Now()

	tests := []struct {
		input    time.Time
		contains string
	}{
		{now.Add(-30 * time.Minute), "30m ago"},
		{now.Add(-2 * time.Hour), "2h ago"},
		{now.Add(-36 * time.Hour), "yesterday"},
		{now.Add(-72 * time.Hour), "3d ago"},
	}

	for _, tt := range tests {
		result := RelativeTime(tt.input)
		if !strings.Contains(result, tt.contains) {
			t.Errorf("RelativeTime(%v): expected '%s' in '%s'", tt.input, tt.contains, result)
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd ~/projects-wsl/nexus && go test ./internal/display/ -v`
Expected: FAIL

- [ ] **Step 3: Implement display.go**

```go
// internal/display/display.go
package display

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/digitalghost404/nexus/internal/db"
)

func FormatSmartSummary(w io.Writer, dirty []db.Project, sessions []db.Session, stale []db.Project, currentProject string) {
	fmt.Fprintf(w, "\n┌ NEXUS ─────────────────────────────────────\n│\n")

	// Dirty projects
	if len(dirty) > 0 {
		fmt.Fprintf(w, "│  ⚠ %d project(s) with uncommitted changes\n", len(dirty))
		for _, p := range dirty {
			fmt.Fprintf(w, "│  %-14s %s  %d dirty file(s)\n", p.Name, p.Branch, p.DirtyFiles)
		}
		fmt.Fprintf(w, "│\n")
	}

	// Recent sessions
	if len(sessions) > 0 {
		fmt.Fprintf(w, "│  Recent Sessions\n")
		for _, s := range sessions {
			timeStr := ""
			if s.StartedAt != nil {
				timeStr = RelativeTime(*s.StartedAt)
			}
			summary := s.Summary
			if len(summary) > 50 {
				summary = summary[:47] + "..."
			}
			fmt.Fprintf(w, "│  %-14s %-12s \"%s\"\n", s.ProjectName, timeStr, summary)
		}
		fmt.Fprintf(w, "│\n")
	}

	// Stale projects
	if len(stale) > 0 {
		fmt.Fprintf(w, "│  Stale (14+ days)\n│  ")
		names := make([]string, len(stale))
		for i, p := range stale {
			names[i] = p.Name
		}
		fmt.Fprintf(w, "%s\n│\n", strings.Join(names, ", "))
	}

	if len(dirty) == 0 && len(sessions) == 0 && len(stale) == 0 {
		fmt.Fprintf(w, "│  No data yet. Run 'nexus scan' to discover projects.\n│\n")
	}

	fmt.Fprintf(w, "└────────────────────────────────────────────\n\n")
}

func FormatProjectTable(w io.Writer, projects []db.Project) {
	if len(projects) == 0 {
		fmt.Fprintln(w, "No projects found.")
		return
	}

	fmt.Fprintf(w, "\n%-16s %-12s %-8s %-6s %s\n", "PROJECT", "BRANCH", "STATUS", "DIRTY", "LAST COMMIT")
	fmt.Fprintf(w, "%s\n", strings.Repeat("─", 70))

	for _, p := range projects {
		dirtyStr := ""
		if p.Dirty {
			dirtyStr = fmt.Sprintf("%d", p.DirtyFiles)
		}
		commitTime := ""
		if p.LastCommitAt != nil {
			commitTime = RelativeTime(*p.LastCommitAt)
		}
		fmt.Fprintf(w, "%-16s %-12s %-8s %-6s %s\n",
			truncate(p.Name, 15), truncate(p.Branch, 11), p.Status, dirtyStr, commitTime)
	}
	fmt.Fprintln(w)
}

func FormatSessionList(w io.Writer, sessions []db.Session) {
	if len(sessions) == 0 {
		fmt.Fprintln(w, "No sessions found.")
		return
	}

	fmt.Fprintf(w, "\n%-16s %-14s %-10s %s\n", "PROJECT", "WHEN", "SOURCE", "SUMMARY")
	fmt.Fprintf(w, "%s\n", strings.Repeat("─", 76))

	for _, s := range sessions {
		timeStr := ""
		if s.StartedAt != nil {
			timeStr = RelativeTime(*s.StartedAt)
		}
		summary := s.Summary
		if len(summary) > 36 {
			summary = summary[:33] + "..."
		}
		fmt.Fprintf(w, "%-16s %-14s %-10s %s\n",
			truncate(s.ProjectName, 15), timeStr, s.Source, summary)
	}
	fmt.Fprintln(w)
}

func FormatProjectDetail(w io.Writer, p *db.Project, sessions []db.Session, staleBranches []string) {
	fmt.Fprintf(w, "\n┌ %s ─────────────────────────────────\n│\n", strings.ToUpper(p.Name))

	fmt.Fprintf(w, "│  Path:      %s\n", p.Path)
	fmt.Fprintf(w, "│  Branch:    %s\n", p.Branch)
	fmt.Fprintf(w, "│  Status:    %s\n", p.Status)

	if p.Dirty {
		fmt.Fprintf(w, "│  Dirty:     %d file(s)\n", p.DirtyFiles)
	}

	if p.LastCommitAt != nil {
		fmt.Fprintf(w, "│  Commit:    %s (%s)\n", RelativeTime(*p.LastCommitAt), p.LastCommitMsg)
	}

	if p.Ahead > 0 || p.Behind > 0 {
		fmt.Fprintf(w, "│  Remote:    %d ahead, %d behind\n", p.Ahead, p.Behind)
	}

	if len(sessions) > 0 {
		fmt.Fprintf(w, "│\n│  Recent Sessions\n")
		for _, s := range sessions {
			timeStr := ""
			if s.StartedAt != nil {
				timeStr = RelativeTime(*s.StartedAt)
			}
			fmt.Fprintf(w, "│  %-14s \"%s\"\n", timeStr, truncate(s.Summary, 50))
		}
	}

	if len(staleBranches) > 0 {
		fmt.Fprintf(w, "│\n│  Stale Branches\n")
		for _, b := range staleBranches {
			fmt.Fprintf(w, "│  %s\n", b)
		}
	}

	fmt.Fprintf(w, "│\n└────────────────────────────────────────────\n\n")
}

func FormatSearchResults(w io.Writer, sessions []db.Session, notes []db.Note) {
	if len(sessions) == 0 && len(notes) == 0 {
		fmt.Fprintln(w, "No results found.")
		return
	}

	if len(sessions) > 0 {
		fmt.Fprintf(w, "\nSessions (%d)\n%s\n", len(sessions), strings.Repeat("─", 40))
		for _, s := range sessions {
			timeStr := ""
			if s.StartedAt != nil {
				timeStr = RelativeTime(*s.StartedAt)
			}
			fmt.Fprintf(w, "  %-14s %-14s %s\n", s.ProjectName, timeStr, s.Summary)
		}
	}

	if len(notes) > 0 {
		fmt.Fprintf(w, "\nNotes (%d)\n%s\n", len(notes), strings.Repeat("─", 40))
		for _, n := range notes {
			fmt.Fprintf(w, "  %-14s %s\n", RelativeTime(n.CreatedAt), truncate(n.Content, 60))
		}
	}
	fmt.Fprintln(w)
}

func RelativeTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 48*time.Hour:
		return "yesterday"
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
```

- [ ] **Step 4: Run tests**

Run: `cd ~/projects-wsl/nexus && go test ./internal/display/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/display/
git commit -m "feat: CLI display formatting - smart summary, tables, detail views"
```

---

### Task 14: CLI Commands — init, scan, capture

**Files:**
- Create: `cmd/init.go`
- Create: `cmd/scan.go`
- Create: `cmd/capture.go`
- Modify: `cmd/root.go`

- [ ] **Step 1: Implement init command**

```go
// cmd/init.go
package cmd

import (
	"fmt"
	"os"

	"github.com/digitalghost404/nexus/internal/config"
	"github.com/digitalghost404/nexus/internal/db"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize Nexus",
	RunE: func(cmd *cobra.Command, args []string) error {
		nexusDir := config.NexusDir()

		// Create directory
		if err := os.MkdirAll(nexusDir, 0755); err != nil {
			return fmt.Errorf("create nexus dir: %w", err)
		}
		fmt.Printf("Created %s\n", nexusDir)

		// Create database
		database, err := db.Open(config.DBPath())
		if err != nil {
			return fmt.Errorf("init db: %w", err)
		}
		database.Close()
		fmt.Printf("Created database at %s\n", config.DBPath())

		// Create default config if missing
		cfgPath := config.ConfigPath()
		if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
			cfg := config.Default()
			home, _ := os.UserHomeDir()
			cfg.Roots = []string{home + "/projects-wsl"}
			if err := config.Save(cfgPath, cfg); err != nil {
				return fmt.Errorf("save config: %w", err)
			}
			fmt.Printf("Created config at %s\n", cfgPath)
		}

		// Print shell wrapper instructions
		fmt.Println("\n── Shell Wrapper ──────────────────────────────")
		fmt.Println("Add this to your ~/.bashrc to auto-capture Claude sessions:")
		fmt.Println()
		fmt.Println(`  claude() { command claude "$@"; local rc=$?; nexus capture --dir "$PWD"; return $rc; }`)
		fmt.Println()
		fmt.Println("Then run: source ~/.bashrc")

		// Print cron instructions
		cfg, _ := config.Load(cfgPath)
		fmt.Println("\n── Periodic Scan ─────────────────────────────")
		fmt.Printf("Add this cron job to run scans every %s:\n\n", cfg.ScanInterval)
		fmt.Printf("  */%s * * * * %s/go/bin/nexus scan >> %s/nexus.log 2>&1\n",
			cronMinutes(cfg.ScanInterval), os.Getenv("HOME"), nexusDir)
		fmt.Println()

		// Run initial scan
		fmt.Println("Running initial scan...")
		return runScan(cfg, false)
	},
}

func cronMinutes(interval string) string {
	// Simple conversion: "30m" -> "30", "15m" -> "15"
	if len(interval) > 1 && interval[len(interval)-1] == 'm' {
		return interval[:len(interval)-1]
	}
	return "30" // default
}

func init() {
	rootCmd.AddCommand(initCmd)
}
```

- [ ] **Step 2: Implement scan command**

```go
// cmd/scan.go
package cmd

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/digitalghost404/nexus/internal/config"
	"github.com/digitalghost404/nexus/internal/db"
	"github.com/digitalghost404/nexus/internal/scanner"
	"github.com/spf13/cobra"
)

var scanVerbose bool

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan roots for projects and update health data",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(config.ConfigPath())
		if err != nil {
			return err
		}
		return runScan(cfg, scanVerbose)
	},
}

func runScan(cfg config.Config, verbose bool) error {
	database, err := db.Open(config.DBPath())
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer database.Close()

	// Discover projects
	paths, err := scanner.Discover(cfg.Roots, cfg.Exclude)
	if err != nil {
		return fmt.Errorf("discover: %w", err)
	}

	if verbose {
		fmt.Printf("Found %d projects\n", len(paths))
	}

	now := time.Now()
	for _, path := range paths {
		name := path[len(path)-len(path)+len(path)-len(path):]
		// Get just the directory name
		for i := len(path) - 1; i >= 0; i-- {
			if path[i] == '/' {
				name = path[i+1:]
				break
			}
		}

		branch, _ := scanner.GetBranch(path)
		dirtyCount, _ := scanner.GetDirtyFileCount(path)
		commitMsg, commitTime, _ := scanner.GetLastCommit(path)
		ahead, behind, _ := scanner.GetAheadBehind(path)
		languages := scanner.DetectLanguages(path)

		// Determine status
		status := "stale"
		lastActivity := commitTime
		if !lastActivity.IsZero() {
			daysSince := int(time.Since(lastActivity).Hours() / 24)
			switch {
			case daysSince <= cfg.Thresholds.Idle:
				status = "active"
			case daysSince <= cfg.Thresholds.Stale:
				status = "idle"
			}
		}

		langsJSON, _ := json.Marshal(languages)

		_, err := database.UpsertProject(db.Project{
			Name:          name,
			Path:          path,
			Languages:     string(langsJSON),
			Branch:        branch,
			Dirty:         dirtyCount > 0,
			DirtyFiles:    dirtyCount,
			LastCommitAt:  &commitTime,
			LastCommitMsg: commitMsg,
			Ahead:         ahead,
			Behind:        behind,
			Status:        status,
			DiscoveredAt:  now,
			LastScannedAt: &now,
		})

		if verbose {
			if err != nil {
				fmt.Printf("  ✗ %s: %v\n", name, err)
			} else {
				fmt.Printf("  ✓ %s (%s, %s)\n", name, branch, status)
			}
		}
	}

	fmt.Printf("Scanned %d projects\n", len(paths))
	return nil
}

func init() {
	scanCmd.Flags().BoolVarP(&scanVerbose, "verbose", "v", false, "Show scan details")
	rootCmd.AddCommand(scanCmd)
}
```

- [ ] **Step 3: Implement capture command**

```go
// cmd/capture.go
package cmd

import (
	"fmt"

	"github.com/digitalghost404/nexus/internal/capture"
	"github.com/digitalghost404/nexus/internal/config"
	"github.com/digitalghost404/nexus/internal/db"
	"github.com/spf13/cobra"
)

var captureDir string

var captureCmd = &cobra.Command{
	Use:    "capture",
	Short:  "Capture a Claude session (called by shell wrapper)",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if captureDir == "" {
			return fmt.Errorf("--dir is required")
		}

		database, err := db.Open(config.DBPath())
		if err != nil {
			return fmt.Errorf("open db: %w", err)
		}
		defer database.Close()

		result, err := capture.CaptureSession(database, captureDir, "")
		if err != nil {
			return fmt.Errorf("capture: %w", err)
		}

		if debug {
			fmt.Printf("Captured session for %s: %s (%d commits, %d files)\n",
				result.ProjectName, result.Summary, result.Commits, result.Files)
		}

		return nil
	},
}

func init() {
	captureCmd.Flags().StringVar(&captureDir, "dir", "", "Working directory of the session")
	rootCmd.AddCommand(captureCmd)
}
```

- [ ] **Step 4: Build and test init + scan manually**

Run:
```bash
cd ~/projects-wsl/nexus
go build -o nexus .
./nexus init
./nexus scan --verbose
```
Expected: Creates ~/.nexus/, discovers projects, shows scan results

- [ ] **Step 5: Commit**

```bash
git add cmd/init.go cmd/scan.go cmd/capture.go
git commit -m "feat: init, scan, and capture CLI commands"
```

---

### Task 15: CLI Commands — projects, sessions, show, note, search, config

**Files:**
- Create: `cmd/projects.go`
- Create: `cmd/sessions.go`
- Create: `cmd/show.go`
- Create: `cmd/note.go`
- Create: `cmd/search.go`
- Create: `cmd/config.go`
- Modify: `cmd/root.go` — add smart summary and project name routing

- [ ] **Step 1: Implement projects command**

```go
// cmd/projects.go
package cmd

import (
	"os"

	"github.com/digitalghost404/nexus/internal/config"
	"github.com/digitalghost404/nexus/internal/db"
	"github.com/digitalghost404/nexus/internal/display"
	"github.com/spf13/cobra"
)

var (
	projectsActive bool
	projectsDirty  bool
	projectsStale  bool
)

var projectsCmd = &cobra.Command{
	Use:   "projects",
	Short: "List all tracked projects",
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := db.Open(config.DBPath())
		if err != nil {
			return err
		}
		defer database.Close()

		var projects []db.Project

		switch {
		case projectsDirty:
			projects, err = database.ListDirtyProjects()
		case projectsActive:
			projects, err = database.ListProjects("active")
		case projectsStale:
			projects, err = database.ListProjects("stale")
		default:
			projects, err = database.ListProjects("")
		}
		if err != nil {
			return err
		}

		display.FormatProjectTable(os.Stdout, projects)
		return nil
	},
}

func init() {
	projectsCmd.Flags().BoolVar(&projectsActive, "active", false, "Show active projects only")
	projectsCmd.Flags().BoolVar(&projectsDirty, "dirty", false, "Show dirty projects only")
	projectsCmd.Flags().BoolVar(&projectsStale, "stale", false, "Show stale projects only")
	rootCmd.AddCommand(projectsCmd)
}
```

- [ ] **Step 2: Implement sessions command**

```go
// cmd/sessions.go
package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/digitalghost404/nexus/internal/config"
	"github.com/digitalghost404/nexus/internal/db"
	"github.com/digitalghost404/nexus/internal/display"
	"github.com/spf13/cobra"
)

var (
	sessionsProject string
	sessionsSince   string
	sessionsToday   bool
)

var sessionsCmd = &cobra.Command{
	Use:   "sessions [project]",
	Short: "List Claude session history",
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := db.Open(config.DBPath())
		if err != nil {
			return err
		}
		defer database.Close()

		filter := db.SessionFilter{Limit: 10}

		// Handle positional project arg
		project := sessionsProject
		if project == "" && len(args) > 0 {
			project = args[0]
		}

		if project != "" {
			p, err := database.GetProjectByName(project)
			if err != nil {
				return err
			}
			if p == nil {
				return fmt.Errorf("project not found: %s", project)
			}
			filter.ProjectID = p.ID
		}

		if sessionsToday {
			now := time.Now()
			today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
			filter.Since = &today
		} else if sessionsSince != "" {
			since, err := parseDuration(sessionsSince)
			if err != nil {
				return fmt.Errorf("invalid --since: %w", err)
			}
			filter.Since = since
		}

		sessions, err := database.ListSessions(filter)
		if err != nil {
			return err
		}

		display.FormatSessionList(os.Stdout, sessions)
		return nil
	},
}

func parseDuration(s string) (*time.Time, error) {
	// Parse "7d", "24h", "30m" etc
	if len(s) < 2 {
		return nil, fmt.Errorf("invalid duration: %s", s)
	}
	unit := s[len(s)-1]
	var n int
	fmt.Sscanf(s[:len(s)-1], "%d", &n)

	var d time.Duration
	switch unit {
	case 'd':
		d = time.Duration(n) * 24 * time.Hour
	case 'h':
		d = time.Duration(n) * time.Hour
	case 'm':
		d = time.Duration(n) * time.Minute
	default:
		return nil, fmt.Errorf("unknown unit: %c", unit)
	}

	t := time.Now().Add(-d)
	return &t, nil
}

func init() {
	sessionsCmd.Flags().StringVar(&sessionsProject, "project", "", "Filter by project")
	sessionsCmd.Flags().StringVar(&sessionsSince, "since", "", "Show sessions since duration (e.g. 7d)")
	sessionsCmd.Flags().BoolVar(&sessionsToday, "today", false, "Show today's sessions only")
	rootCmd.AddCommand(sessionsCmd)
}
```

- [ ] **Step 3: Implement show command**

```go
// cmd/show.go
package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/digitalghost404/nexus/internal/config"
	"github.com/digitalghost404/nexus/internal/db"
	"github.com/digitalghost404/nexus/internal/display"
	"github.com/digitalghost404/nexus/internal/scanner"
	"github.com/spf13/cobra"
)

var showCmd = &cobra.Command{
	Use:   "show <project>",
	Short: "Show detailed project info",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := db.Open(config.DBPath())
		if err != nil {
			return err
		}
		defer database.Close()

		p, err := database.GetProjectByName(args[0])
		if err != nil {
			return err
		}
		if p == nil {
			return fmt.Errorf("project not found: %s", args[0])
		}

		sessions, _ := database.ListSessions(db.SessionFilter{ProjectID: p.ID, Limit: 5})
		staleBranches, _ := scanner.GetStaleBranches(p.Path, 7*24*time.Hour)

		display.FormatProjectDetail(os.Stdout, p, sessions, staleBranches)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(showCmd)
}
```

- [ ] **Step 4: Implement note command**

```go
// cmd/note.go
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/digitalghost404/nexus/internal/config"
	"github.com/digitalghost404/nexus/internal/db"
	"github.com/spf13/cobra"
)

var noteCmd = &cobra.Command{
	Use:   "note <message>",
	Short: "Add a note to the current project",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := db.Open(config.DBPath())
		if err != nil {
			return err
		}
		defer database.Close()

		message := strings.Join(args, " ")

		// Try to find current project
		cwd, _ := os.Getwd()
		absDir, _ := filepath.Abs(cwd)
		var projectID *int64

		p, _ := database.GetProjectByPath(absDir)
		if p != nil {
			projectID = &p.ID
		}

		_, err = database.InsertNote(db.Note{
			ProjectID: projectID,
			Content:   message,
		})
		if err != nil {
			return err
		}

		if p != nil {
			fmt.Printf("Note added to %s\n", p.Name)
		} else {
			fmt.Println("Global note added")
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(noteCmd)
}
```

- [ ] **Step 5: Implement search command**

```go
// cmd/search.go
package cmd

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/digitalghost404/nexus/internal/config"
	"github.com/digitalghost404/nexus/internal/db"
	"github.com/digitalghost404/nexus/internal/display"
	"github.com/spf13/cobra"
)

var (
	searchProject string
	searchFiles   string
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search sessions and notes",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := db.Open(config.DBPath())
		if err != nil {
			return err
		}
		defer database.Close()

		query := strings.Join(args, " ")

		// FTS search on sessions and notes
		sessions, err := database.SearchSessions(query)
		if err != nil {
			return err
		}

		notes, err := database.SearchNotes(query)
		if err != nil {
			return err
		}

		// Filter by project if specified
		if searchProject != "" {
			p, _ := database.GetProjectByName(searchProject)
			if p != nil {
				var filtered []db.Session
				for _, s := range sessions {
					if s.ProjectID == p.ID {
						filtered = append(filtered, s)
					}
				}
				sessions = filtered
			}
		}

		// Filter by files pattern if specified
		if searchFiles != "" {
			var filtered []db.Session
			for _, s := range sessions {
				var files []string
				json.Unmarshal([]byte(s.FilesChanged), &files)
				for _, f := range files {
					if matchFilePattern(f, searchFiles) {
						filtered = append(filtered, s)
						break
					}
				}
			}
			sessions = filtered
		}

		display.FormatSearchResults(os.Stdout, sessions, notes)
		return nil
	},
}

func matchFilePattern(file, pattern string) bool {
	// Simple glob: "*.go" matches "main.go", "cmd/root.go"
	if strings.HasPrefix(pattern, "*.") {
		ext := pattern[1:] // ".go"
		return strings.HasSuffix(file, ext)
	}
	return strings.Contains(file, pattern)
}

func init() {
	searchCmd.Flags().StringVar(&searchProject, "project", "", "Filter by project")
	searchCmd.Flags().StringVar(&searchFiles, "files", "", "Filter by file pattern")
	rootCmd.AddCommand(searchCmd)
}
```

- [ ] **Step 6: Implement config command**

```go
// cmd/config.go
package cmd

import (
	"fmt"

	"github.com/digitalghost404/nexus/internal/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage Nexus configuration",
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(config.ConfigPath())
		if err != nil {
			return err
		}
		data, _ := yaml.Marshal(cfg)
		fmt.Println(string(data))
		return nil
	},
}

var configRootsCmd = &cobra.Command{
	Use:   "roots",
	Short: "Manage scan roots",
}

var configRootsAddCmd = &cobra.Command{
	Use:   "add <path>",
	Short: "Add a scan root",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfgPath := config.ConfigPath()
		cfg, err := config.Load(cfgPath)
		if err != nil {
			return err
		}

		expanded := config.ExpandPath(args[0])
		for _, r := range cfg.Roots {
			if r == expanded {
				fmt.Printf("Root already exists: %s\n", expanded)
				return nil
			}
		}

		cfg.Roots = append(cfg.Roots, expanded)
		if err := config.Save(cfgPath, cfg); err != nil {
			return err
		}
		fmt.Printf("Added root: %s\n", expanded)
		return nil
	},
}

var configExcludeCmd = &cobra.Command{
	Use:   "exclude",
	Short: "Manage exclusion patterns",
}

var configExcludeAddCmd = &cobra.Command{
	Use:   "add <pattern>",
	Short: "Add an exclusion pattern",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfgPath := config.ConfigPath()
		cfg, err := config.Load(cfgPath)
		if err != nil {
			return err
		}

		cfg.Exclude = append(cfg.Exclude, args[0])
		if err := config.Save(cfgPath, cfg); err != nil {
			return err
		}
		fmt.Printf("Added exclusion: %s\n", args[0])
		return nil
	},
}

func init() {
	configRootsCmd.AddCommand(configRootsAddCmd)
	configExcludeCmd.AddCommand(configExcludeAddCmd)
	configCmd.AddCommand(configShowCmd, configRootsCmd, configExcludeCmd)
	rootCmd.AddCommand(configCmd)
}
```

- [ ] **Step 7: Update root command with smart summary and project routing**

Update `cmd/root.go`:

```go
// cmd/root.go
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/digitalghost404/nexus/internal/config"
	"github.com/digitalghost404/nexus/internal/db"
	"github.com/digitalghost404/nexus/internal/display"
	"github.com/spf13/cobra"
)

var debug bool

// subcommands that take precedence over project name routing
var subcommands = map[string]bool{
	"init": true, "scan": true, "capture": true, "projects": true,
	"sessions": true, "search": true, "show": true, "note": true,
	"config": true, "help": true,
}

var rootCmd = &cobra.Command{
	Use:   "nexus",
	Short: "Track project health and Claude sessions",
	Long:  "Nexus gives you a single pane of glass into all your projects and Claude Code sessions.",
	Args:  cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Route to show if first arg is a known project name (not a subcommand)
		if len(args) > 0 && !subcommands[args[0]] {
			database, err := db.Open(config.DBPath())
			if err == nil {
				defer database.Close()
				p, _ := database.GetProjectByName(args[0])
				if p != nil {
					showCmd.SetArgs(args)
					return showCmd.RunE(showCmd, args)
				}
			}
		}

		// Smart summary
		return smartSummary()
	},
}

func smartSummary() error {
	database, err := db.Open(config.DBPath())
	if err != nil {
		fmt.Println("Nexus not initialized. Run 'nexus init' to get started.")
		return nil
	}
	defer database.Close()

	dirty, _ := database.ListDirtyProjects()
	sessions, _ := database.ListSessions(db.SessionFilter{Limit: 5})
	stale, _ := database.ListProjects("stale")

	// Detect current project for context
	cwd, _ := os.Getwd()
	absDir, _ := filepath.Abs(cwd)
	currentProject := ""
	p, _ := database.GetProjectByPath(absDir)
	if p != nil {
		currentProject = p.Name
	}

	display.FormatSmartSummary(os.Stdout, dirty, sessions, stale, currentProject)
	return nil
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Enable debug output to stderr")
}
```

- [ ] **Step 8: Build and test all commands**

Run:
```bash
cd ~/projects-wsl/nexus
go build -o nexus .
./nexus
./nexus projects
./nexus sessions
./nexus scan --verbose
```
Expected: All commands work, smart summary shows data

- [ ] **Step 9: Commit**

```bash
git add cmd/
git commit -m "feat: all CLI commands - projects, sessions, show, note, search, config, smart summary"
```

---

### Task 16: Install Binary + End-to-End Test

**Files:**
- Modify: `go.mod` (ensure module path is correct)

- [ ] **Step 1: Install the binary**

Run:
```bash
cd ~/projects-wsl/nexus
go install .
```
Expected: `nexus` is now in `~/go/bin/` and available globally

- [ ] **Step 2: Run init from any directory**

Run:
```bash
cd ~
nexus init
```
Expected: Sets up ~/.nexus/, creates DB, shows wrapper instructions

- [ ] **Step 3: Test scan**

Run:
```bash
nexus scan --verbose
```
Expected: Discovers projects in ~/projects-wsl/

- [ ] **Step 4: Test smart summary**

Run:
```bash
nexus
```
Expected: Shows dirty projects, recent sessions (empty initially), stale projects

- [ ] **Step 5: Test project commands**

Run:
```bash
nexus projects
nexus projects --dirty
nexus projects --stale
nexus wraith   # shorthand for show
nexus show wraith
```
Expected: All display correctly

- [ ] **Step 6: Test note and search**

Run:
```bash
cd ~/projects-wsl/wraith
nexus note "testing nexus note command"
nexus search "testing"
```
Expected: Note saved, search finds it

- [ ] **Step 7: Test capture (simulating shell wrapper)**

Run:
```bash
cd ~/projects-wsl/nexus
nexus capture --dir $(pwd)
nexus sessions
```
Expected: Session captured and visible in session list

- [ ] **Step 8: Run all tests**

Run:
```bash
cd ~/projects-wsl/nexus
go test ./... -v
```
Expected: All tests PASS

- [ ] **Step 9: Final commit**

```bash
git add -A
git commit -m "feat: install binary and verify end-to-end functionality"
```

---

### Task 17: Run Full Test Suite + Cleanup

- [ ] **Step 1: Run full test suite**

Run:
```bash
cd ~/projects-wsl/nexus
go test ./... -v -count=1
```
Expected: All PASS

- [ ] **Step 2: Run go vet**

Run:
```bash
cd ~/projects-wsl/nexus
go vet ./...
```
Expected: No issues

- [ ] **Step 3: Clean up any TODOs left in code**

Search for TODO comments and resolve or remove them.

- [ ] **Step 4: Verify binary size is reasonable**

Run:
```bash
ls -lh ~/go/bin/nexus
```
Expected: Single binary, reasonable size (< 20MB)

- [ ] **Step 5: Final commit if any cleanup was done**

```bash
git add -A
git commit -m "chore: cleanup, vet, and final test pass"
```
