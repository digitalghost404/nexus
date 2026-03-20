# Nexus v2 Feature Expansion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add 12 new CLI commands to Nexus: resume, diff, stale, deps, report, watch, context, hook, streak, where, link, tag — plus help improvements.

**Architecture:** All commands compose existing internal packages (db, scanner, display, capture). New DB methods added to existing files. Two new tables via schema migration v1→v2. Cobra command grouping for help.

**Tech Stack:** Go, SQLite (modernc.org/sqlite), Cobra, gopkg.in/yaml.v3 — no new dependencies.

**Spec:** `docs/superpowers/specs/2026-03-19-nexus-v2-features-design.md`

---

## File Structure

```
nexus/
├── cmd/
│   ├── root.go             # MODIFY: dynamic subcommand routing, command groups
│   ├── resume.go            # CREATE: nexus resume
│   ├── diff.go              # CREATE: nexus diff
│   ├── stale.go             # CREATE: nexus stale [--cleanup]
│   ├── deps.go              # CREATE: nexus deps [--project]
│   ├── report.go            # CREATE: nexus report [--week|--month]
│   ├── watch.go             # CREATE: nexus watch
│   ├── context_cmd.go       # CREATE: nexus context (named to avoid Go keyword)
│   ├── hook.go              # CREATE: nexus hook install/uninstall
│   ├── streak.go            # CREATE: nexus streak
│   ├── where.go             # CREATE: nexus where
│   ├── link.go              # CREATE: nexus link
│   ├── tag.go               # CREATE: nexus tag
│   └── sessions.go          # MODIFY: add --tag flag
├── internal/
│   ├── db/
│   │   ├── db.go            # MODIFY: migrate() v1→v2 path
│   │   ├── schema.sql       # MODIFY: add v2 tables
│   │   ├── migration_v2.sql # CREATE: delta migration for v1→v2
│   │   ├── sessions.go      # MODIFY: add GetLatestSession, GetSessionsInRange, GetDistinctSessionDates
│   │   ├── sessions_test.go # MODIFY: tests for new methods
│   │   ├── links.go         # CREATE: project_links CRUD
│   │   ├── links_test.go    # CREATE: tests
│   │   ├── tags.go          # CREATE: session_tags CRUD
│   │   └── tags_test.go     # CREATE: tests
│   ├── display/
│   │   └── display.go       # MODIFY: add FormatResume, FormatDiff, FormatReport, FormatStreak, FormatWhere, FormatContext, FormatDeps, FormatStale
│   └── scanner/
│       └── git.go           # MODIFY: add DeleteBranch, GetDirtyFileDetails
```

---

### Task 1: New DB Query Methods for Tier 1 Commands

**Files:**
- Modify: `internal/db/sessions.go`
- Modify: `internal/db/sessions_test.go`

- [ ] **Step 1: Write failing tests**

```go
// Add to internal/db/sessions_test.go

func TestGetLatestSession(t *testing.T) {
	d := testDB(t)
	now := time.Now()
	pID, _ := d.UpsertProject(Project{Name: "proj", Path: "/a", Status: "active", DiscoveredAt: now})

	earlier := now.Add(-2 * time.Hour)
	d.InsertSession(Session{ProjectID: pID, Summary: "first", Source: "wrapper", StartedAt: &earlier})
	d.InsertSession(Session{ProjectID: pID, Summary: "latest", Source: "wrapper", StartedAt: &now})

	s, err := d.GetLatestSession(pID)
	if err != nil {
		t.Fatalf("get latest: %v", err)
	}
	if s == nil || s.Summary != "latest" {
		t.Errorf("expected 'latest', got %v", s)
	}
}

func TestGetLatestSessionEmpty(t *testing.T) {
	d := testDB(t)
	now := time.Now()
	pID, _ := d.UpsertProject(Project{Name: "proj", Path: "/a", Status: "active", DiscoveredAt: now})

	s, err := d.GetLatestSession(pID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s != nil {
		t.Error("expected nil for empty project")
	}
}

func TestGetSessionsInRange(t *testing.T) {
	d := testDB(t)
	now := time.Now()
	pID, _ := d.UpsertProject(Project{Name: "proj", Path: "/a", Status: "active", DiscoveredAt: now})

	old := now.Add(-10 * 24 * time.Hour)
	recent := now.Add(-1 * time.Hour)
	d.InsertSession(Session{ProjectID: pID, Summary: "old", Source: "wrapper", StartedAt: &old})
	d.InsertSession(Session{ProjectID: pID, Summary: "recent", Source: "wrapper", StartedAt: &recent})

	since := now.Add(-7 * 24 * time.Hour)
	sessions, err := d.GetSessionsInRange(pID, since, now)
	if err != nil {
		t.Fatalf("range: %v", err)
	}
	if len(sessions) != 1 || sessions[0].Summary != "recent" {
		t.Errorf("expected 1 recent session, got %d", len(sessions))
	}
}

func TestGetDistinctSessionDates(t *testing.T) {
	d := testDB(t)
	now := time.Now()
	pID, _ := d.UpsertProject(Project{Name: "proj", Path: "/a", Status: "active", DiscoveredAt: now})

	day1 := now.Add(-48 * time.Hour)
	day2 := now.Add(-24 * time.Hour)
	d.InsertSession(Session{ProjectID: pID, Summary: "s1", Source: "wrapper", StartedAt: &day1})
	d.InsertSession(Session{ProjectID: pID, Summary: "s2", Source: "wrapper", StartedAt: &day2})
	d.InsertSession(Session{ProjectID: pID, Summary: "s3", Source: "wrapper", StartedAt: &now})

	dates, err := d.GetDistinctSessionDates()
	if err != nil {
		t.Fatalf("dates: %v", err)
	}
	if len(dates) != 3 {
		t.Errorf("expected 3 distinct dates, got %d", len(dates))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd ~/projects-wsl/nexus && go test ./internal/db/ -v -run "TestGetLatest|TestGetSessionsInRange|TestGetDistinct"`
Expected: FAIL — methods not defined

- [ ] **Step 3: Implement new methods**

Add to `internal/db/sessions.go`:

```go
func (d *DB) GetLatestSession(projectID int64) (*Session, error) {
	var s Session
	err := d.db.QueryRow(`
		SELECT s.id, s.project_id, COALESCE(s.claude_session_id, ''),
			s.started_at, s.ended_at, s.duration_secs, COALESCE(s.summary, ''),
			s.files_changed, s.commits_made, s.tags, s.source, s.created_at,
			p.name
		FROM sessions s
		JOIN projects p ON p.id = s.project_id
		WHERE s.project_id = ?
		ORDER BY s.started_at DESC
		LIMIT 1`, projectID).Scan(
		&s.ID, &s.ProjectID, &s.ClaudeSessionID,
		&s.StartedAt, &s.EndedAt, &s.DurationSecs, &s.Summary,
		&s.FilesChanged, &s.CommitsMade, &s.Tags, &s.Source, &s.CreatedAt,
		&s.ProjectName)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get latest session: %w", err)
	}
	return &s, nil
}

func (d *DB) GetSessionsInRange(projectID int64, since, until time.Time) ([]Session, error) {
	rows, err := d.db.Query(`
		SELECT s.id, s.project_id, COALESCE(s.claude_session_id, ''),
			s.started_at, s.ended_at, s.duration_secs, COALESCE(s.summary, ''),
			s.files_changed, s.commits_made, s.tags, s.source, s.created_at,
			p.name
		FROM sessions s
		JOIN projects p ON p.id = s.project_id
		WHERE s.project_id = ? AND s.started_at >= ? AND s.started_at <= ?
		ORDER BY s.started_at DESC`, projectID, since, until)
	if err != nil {
		return nil, fmt.Errorf("sessions in range: %w", err)
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

func (d *DB) GetDistinctSessionDates() ([]string, error) {
	rows, err := d.db.Query(`
		SELECT DISTINCT date(started_at, 'localtime')
		FROM sessions
		WHERE started_at IS NOT NULL
		ORDER BY 1 DESC`)
	if err != nil {
		return nil, fmt.Errorf("distinct dates: %w", err)
	}
	defer rows.Close()

	var dates []string
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return nil, err
		}
		dates = append(dates, d)
	}
	return dates, nil
}
```

Note: `GetDistinctSessionDates` returns `[]string` (date strings like "2026-03-19") instead of `[]time.Time` — simpler for streak calculation which just needs to compare consecutive date strings.

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd ~/projects-wsl/nexus && go test ./internal/db/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/db/sessions.go internal/db/sessions_test.go
git commit -m "feat: add GetLatestSession, GetSessionsInRange, GetDistinctSessionDates"
```

---

### Task 2: Scanner Helpers — DeleteBranch, GetDirtyFileDetails

**Files:**
- Modify: `internal/scanner/git.go`
- Modify: `internal/scanner/git_test.go`

- [ ] **Step 1: Write failing tests**

```go
// Add to internal/scanner/git_test.go

func TestDeleteBranch(t *testing.T) {
	repo := CreateTestRepo(t, filepath.Join(t.TempDir(), "test-repo"))

	// Create and switch to a new branch, then switch back
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		cmd.CombinedOutput()
	}
	run("checkout", "-b", "feature-old")
	run("checkout", "main")

	err := DeleteBranch(repo, "feature-old")
	if err != nil {
		t.Fatalf("DeleteBranch: %v", err)
	}

	// Verify branch is gone
	out, _ := exec.Command("git", "-C", repo, "branch").Output()
	if strings.Contains(string(out), "feature-old") {
		t.Error("branch should have been deleted")
	}
}

func TestGetDirtyFileDetails(t *testing.T) {
	repo := CreateTestRepo(t, filepath.Join(t.TempDir(), "test-repo"))

	// Create modified and untracked files
	os.WriteFile(filepath.Join(repo, "README.md"), []byte("modified"), 0644)
	os.WriteFile(filepath.Join(repo, "new.txt"), []byte("new"), 0644)

	details, err := GetDirtyFileDetails(repo)
	if err != nil {
		t.Fatalf("GetDirtyFileDetails: %v", err)
	}
	if len(details) != 2 {
		t.Errorf("expected 2 dirty files, got %d", len(details))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd ~/projects-wsl/nexus && go test ./internal/scanner/ -v -run "TestDeleteBranch|TestGetDirtyFileDetails"`
Expected: FAIL

- [ ] **Step 3: Implement**

Add to `internal/scanner/git.go`:

```go
func DeleteBranch(dir, branch string) error {
	_, err := gitCmd(dir, "branch", "-d", branch)
	return err
}

type DirtyFile struct {
	Status string // "M", "A", "D", "??"
	Path   string
}

func GetDirtyFileDetails(dir string) ([]DirtyFile, error) {
	out, err := gitCmd(dir, "status", "--porcelain")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	var files []DirtyFile
	for _, line := range strings.Split(out, "\n") {
		if len(line) < 4 {
			continue
		}
		files = append(files, DirtyFile{
			Status: strings.TrimSpace(line[:2]),
			Path:   line[3:],
		})
	}
	return files, nil
}
```

- [ ] **Step 4: Run tests**

Run: `cd ~/projects-wsl/nexus && go test ./internal/scanner/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/scanner/git.go internal/scanner/git_test.go
git commit -m "feat: add DeleteBranch and GetDirtyFileDetails to scanner"
```

---

### Task 3: Display Functions for New Commands

**Files:**
- Modify: `internal/display/display.go`
- Modify: `internal/display/display_test.go`

- [ ] **Step 1: Write failing tests**

```go
// Add to internal/display/display_test.go

func TestFormatResume(t *testing.T) {
	var buf bytes.Buffer
	now := time.Now()
	s := db.Session{
		ProjectName:  "wraith",
		Summary:      "Added retry logic",
		StartedAt:    &now,
		DurationSecs: 2700,
		CommitsMade:  `[{"hash":"abc123","message":"feat: add retry"}]`,
		FilesChanged: `["cmd/scan.go","internal/scanner.go"]`,
	}
	dirtyFiles := []string{"M internal/test.go", "?? new.txt"}

	FormatResume(&buf, &s, dirtyFiles)
	output := buf.String()

	if !strings.Contains(output, "RESUME") {
		t.Error("expected RESUME header")
	}
	if !strings.Contains(output, "wraith") {
		t.Error("expected project name")
	}
	if !strings.Contains(output, "retry") {
		t.Error("expected summary content")
	}
}

func TestFormatStreak(t *testing.T) {
	var buf bytes.Buffer
	FormatStreak(&buf, 12, 23, "2026-02-10", "2026-03-04", []bool{true, true, true, true, true, true, false}, []bool{true, true, true, true, true, true, true})
	output := buf.String()

	if !strings.Contains(output, "12") {
		t.Error("expected current streak")
	}
	if !strings.Contains(output, "23") {
		t.Error("expected longest streak")
	}
}

func TestFormatReport(t *testing.T) {
	var buf bytes.Buffer
	r := ReportData{
		StartDate:     "Mar 12",
		EndDate:       "Mar 19",
		TotalSessions: 12,
		TotalProjects: 5,
		TotalCommits:  34,
		TotalFiles:    47,
		TotalHours:    8.5,
		TopProjects:   []ProjectActivity{{Name: "wraith", Sessions: 6, Commits: 18}},
		Languages:     []LangPercent{{Lang: "Go", Percent: 62}},
		Highlights:    []string{"Added retry logic"},
	}
	FormatReport(&buf, r)
	output := buf.String()

	if !strings.Contains(output, "REPORT") {
		t.Error("expected REPORT header")
	}
	if !strings.Contains(output, "12 sessions") {
		t.Error("expected session count")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd ~/projects-wsl/nexus && go test ./internal/display/ -v -run "TestFormatResume|TestFormatStreak|TestFormatReport"`
Expected: FAIL

- [ ] **Step 3: Implement all new display functions**

Add to `internal/display/display.go`:

```go
// Data types for report
type ProjectActivity struct {
	Name     string
	Sessions int
	Commits  int
}

type LangPercent struct {
	Lang    string
	Percent int
}

type ReportData struct {
	StartDate, EndDate string
	TotalSessions      int
	TotalProjects      int
	TotalCommits       int
	TotalFiles         int
	TotalHours         float64
	TopProjects        []ProjectActivity
	Languages          []LangPercent
	Highlights         []string
}

type DepInfo struct {
	Name       string
	Current    string
	Available  string
}

type ProjectDeps struct {
	ProjectName string
	Manager     string // "Go", "npm", "pip"
	Outdated    []DepInfo
}

type WhereResult struct {
	ProjectName string
	Files       []WhereFile
}

type WhereFile struct {
	Path     string
	Sessions []string // dates
}

func FormatResume(w io.Writer, s *db.Session, dirtyFiles []string) {
	fmt.Fprintf(w, "\n┌ RESUME: %s ────────────────────────────\n│\n", s.ProjectName)

	if s.StartedAt != nil {
		duration := ""
		if s.DurationSecs > 0 {
			duration = fmt.Sprintf(" (%s)", formatDuration(s.DurationSecs))
		}
		fmt.Fprintf(w, "│  Last session: %s%s\n", RelativeTime(*s.StartedAt), duration)
	}
	fmt.Fprintf(w, "│  Summary: \"%s\"\n", s.Summary)

	// Parse and show commits
	type commitEntry struct {
		Hash    string `json:"hash"`
		Message string `json:"message"`
	}
	var commits []commitEntry
	json.Unmarshal([]byte(s.CommitsMade), &commits)
	if len(commits) > 0 {
		fmt.Fprintf(w, "│\n│  Commits:\n")
		for _, c := range commits {
			fmt.Fprintf(w, "│    %s  %s\n", c.Hash, c.Message)
		}
	}

	// Parse and show files
	var files []string
	json.Unmarshal([]byte(s.FilesChanged), &files)
	if len(files) > 0 {
		fmt.Fprintf(w, "│\n│  Files changed:\n│    %s\n", strings.Join(files, ", "))
	}

	// Dirty files from live git status
	if len(dirtyFiles) > 0 {
		fmt.Fprintf(w, "│\n│  Uncommitted changes: %d files\n", len(dirtyFiles))
		for _, f := range dirtyFiles {
			fmt.Fprintf(w, "│    %s\n", f)
		}
	}

	fmt.Fprintf(w, "│\n└────────────────────────────────────────────\n\n")
}

func FormatDiff(w io.Writer, projectName string, sinceDays int, sessions []db.Session) {
	fmt.Fprintf(w, "\n┌ DIFF: %s (last %d days) ────────────────\n│\n", projectName, sinceDays)

	// Aggregate stats
	totalCommits := 0
	fileSet := map[string]int{}
	for _, s := range sessions {
		var commits []struct{ Hash, Message string }
		json.Unmarshal([]byte(s.CommitsMade), &commits)
		totalCommits += len(commits)

		var files []string
		json.Unmarshal([]byte(s.FilesChanged), &files)
		for _, f := range files {
			fileSet[f]++
		}
	}

	fmt.Fprintf(w, "│  %d sessions, %d commits, %d files touched\n│\n", len(sessions), totalCommits, len(fileSet))

	// Timeline
	fmt.Fprintf(w, "│  Timeline:\n")
	for _, s := range sessions {
		dateStr := ""
		if s.StartedAt != nil {
			dateStr = s.StartedAt.Format("Jan 02")
		}
		fmt.Fprintf(w, "│    %-8s \"%s\"\n", dateStr, truncate(s.Summary, 50))
	}

	// Most active files (top 5)
	if len(fileSet) > 0 {
		fmt.Fprintf(w, "│\n│  Most active files:\n")
		type fileCount struct {
			path  string
			count int
		}
		var sorted []fileCount
		for p, c := range fileSet {
			sorted = append(sorted, fileCount{p, c})
		}
		// Simple sort by count desc
		for i := 0; i < len(sorted); i++ {
			for j := i + 1; j < len(sorted); j++ {
				if sorted[j].count > sorted[i].count {
					sorted[i], sorted[j] = sorted[j], sorted[i]
				}
			}
		}
		limit := 5
		if len(sorted) < limit {
			limit = len(sorted)
		}
		for _, fc := range sorted[:limit] {
			fmt.Fprintf(w, "│    %-40s (modified in %d sessions)\n", fc.path, fc.count)
		}
	}

	fmt.Fprintf(w, "│\n└────────────────────────────────────────────\n\n")
}

func FormatReport(w io.Writer, r ReportData) {
	fmt.Fprintf(w, "\n┌ REPORT: %s – %s ──────────────────\n│\n", r.StartDate, r.EndDate)

	fmt.Fprintf(w, "│  %d sessions across %d projects\n", r.TotalSessions, r.TotalProjects)
	fmt.Fprintf(w, "│  %d commits, %d files changed\n", r.TotalCommits, r.TotalFiles)
	fmt.Fprintf(w, "│  ~%.1f hours of Claude sessions\n", r.TotalHours)

	if len(r.TopProjects) > 0 {
		fmt.Fprintf(w, "│\n│  Most active projects:\n")
		for _, p := range r.TopProjects {
			fmt.Fprintf(w, "│    %-16s %d sessions, %d commits\n", p.Name, p.Sessions, p.Commits)
		}
	}

	if len(r.Languages) > 0 {
		fmt.Fprintf(w, "│\n│  Languages:\n│    ")
		parts := make([]string, len(r.Languages))
		for i, l := range r.Languages {
			parts[i] = fmt.Sprintf("%s %d%%", l.Lang, l.Percent)
		}
		fmt.Fprintf(w, "%s\n", strings.Join(parts, "  "))
	}

	if len(r.Highlights) > 0 {
		fmt.Fprintf(w, "│\n│  Highlights:\n")
		for _, h := range r.Highlights {
			fmt.Fprintf(w, "│    \"%s\"\n", truncate(h, 60))
		}
	}

	fmt.Fprintf(w, "│\n└────────────────────────────────────────────\n\n")
}

func FormatStreak(w io.Writer, current, longest int, longestStart, longestEnd string, thisWeek, lastWeek []bool) {
	fmt.Fprintf(w, "\nCurrent streak: %d days\n", current)
	fmt.Fprintf(w, "Longest streak: %d days (%s – %s)\n", longest, longestStart, longestEnd)

	fmt.Fprintf(w, "\nThis week: %s  %d/7 days\n", weekBar(thisWeek), countTrue(thisWeek))
	fmt.Fprintf(w, "Last week: %s  %d/7 days\n\n", weekBar(lastWeek), countTrue(lastWeek))
}

func FormatWhere(w io.Writer, results []WhereResult) {
	if len(results) == 0 {
		fmt.Fprintln(w, "No results found.")
		return
	}
	fmt.Fprintln(w)
	for _, r := range results {
		fmt.Fprintln(w, r.ProjectName)
		for _, f := range r.Files {
			fmt.Fprintf(w, "  %-40s (sessions: %s)\n", f.Path, strings.Join(f.Sessions, ", "))
		}
		fmt.Fprintln(w)
	}
}

func FormatContext(w io.Writer, p *db.Project, sessions []db.Session, notes []db.Note, linkedProjects []db.Project) {
	fmt.Fprintf(w, "## Project: %s\n", p.Name)
	fmt.Fprintf(w, "Path: %s\n", p.Path)

	branchInfo := p.Branch
	if p.Dirty {
		branchInfo += fmt.Sprintf(" (%d files dirty)", p.DirtyFiles)
	}
	fmt.Fprintf(w, "Branch: %s\n", branchInfo)

	if p.Languages != "" && p.Languages != "[]" {
		var langs []string
		json.Unmarshal([]byte(p.Languages), &langs)
		fmt.Fprintf(w, "Languages: %s\n", strings.Join(langs, ", "))
	}

	if len(sessions) > 0 {
		fmt.Fprintf(w, "\n## Recent Sessions (last 7 days)\n")
		for _, s := range sessions {
			dateStr := ""
			if s.StartedAt != nil {
				dateStr = s.StartedAt.Format("Jan 02")
			}
			var commits []struct{ Hash, Message string }
			json.Unmarshal([]byte(s.CommitsMade), &commits)
			fmt.Fprintf(w, "- %s: \"%s\" (%d commits)\n", dateStr, s.Summary, len(commits))
		}
	}

	if len(notes) > 0 {
		fmt.Fprintf(w, "\n## Notes\n")
		for _, n := range notes {
			fmt.Fprintf(w, "- \"%s\"\n", n.Content)
		}
	}

	if len(linkedProjects) > 0 {
		fmt.Fprintf(w, "\n## Linked Projects\n")
		for _, lp := range linkedProjects {
			fmt.Fprintf(w, "- %s\n", lp.Name)
		}
	}
	fmt.Fprintln(w)
}

func FormatDeps(w io.Writer, results []ProjectDeps, cleanCount int) {
	fmt.Fprintf(w, "\n┌ DEPENDENCIES ──────────────────────────────\n│\n")

	for _, pd := range results {
		fmt.Fprintf(w, "│  %s (%s)\n", pd.ProjectName, pd.Manager)
		for _, d := range pd.Outdated {
			fmt.Fprintf(w, "│    %-40s %s → %s\n", d.Name, d.Current, d.Available)
		}
		fmt.Fprintf(w, "│\n")
	}

	if cleanCount > 0 {
		fmt.Fprintf(w, "│  %d projects clean\n│\n", cleanCount)
	}

	fmt.Fprintf(w, "└────────────────────────────────────────────\n\n")
}

func FormatStale(w io.Writer, projects []db.Project, branches map[string][]StaleBranchInfo, dirtyDetails map[string][]string) {
	if len(branches) == 0 && len(dirtyDetails) == 0 {
		fmt.Fprintln(w, "Nothing stale or dirty.")
		return
	}

	if len(branches) > 0 {
		fmt.Fprintln(w, "\nStale branches:\n")
		for project, brs := range branches {
			fmt.Fprintf(w, "  %s\n", project)
			for _, b := range brs {
				fmt.Fprintf(w, "    %s  (last commit: %s)\n", b.Name, b.Age)
			}
			fmt.Fprintln(w)
		}
	}

	if len(dirtyDetails) > 0 {
		fmt.Fprintln(w, "Dirty projects (uncommitted changes):\n")
		for project, files := range dirtyDetails {
			fmt.Fprintf(w, "  %s\n", project)
			for _, f := range files {
				fmt.Fprintf(w, "    %s\n", f)
			}
			fmt.Fprintln(w)
		}
	}
}

type StaleBranchInfo struct {
	Name string
	Age  string
}

func formatDuration(secs int) string {
	if secs < 60 {
		return fmt.Sprintf("%ds", secs)
	}
	if secs < 3600 {
		return fmt.Sprintf("%dm", secs/60)
	}
	h := secs / 3600
	m := (secs % 3600) / 60
	if m == 0 {
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dh%dm", h, m)
}

func weekBar(days []bool) string {
	var b strings.Builder
	for _, d := range days {
		if d {
			b.WriteString("█")
		} else {
			b.WriteString("░")
		}
	}
	return b.String()
}

func countTrue(days []bool) int {
	c := 0
	for _, d := range days {
		if d {
			c++
		}
	}
	return c
}
```

Add `"encoding/json"` to the imports in display.go.

- [ ] **Step 4: Run tests**

Run: `cd ~/projects-wsl/nexus && go test ./internal/display/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/display/
git commit -m "feat: display functions for resume, diff, report, streak, where, context, deps, stale"
```

---

### Task 4: Dynamic Subcommand Routing in root.go

**Files:**
- Modify: `cmd/root.go`

- [ ] **Step 1: Replace hardcoded subcommands map with dynamic detection**

In `cmd/root.go`, replace the `subcommands` map and the routing logic in `rootCmd.RunE`:

```go
// Remove the subcommands map entirely

// Replace the RunE function body:
RunE: func(cmd *cobra.Command, args []string) error {
    if len(args) > 0 {
        // Check if first arg is a registered subcommand
        for _, sub := range cmd.Commands() {
            if sub.Name() == args[0] || sub.HasAlias(args[0]) {
                // It's a subcommand, let cobra handle it
                return nil
            }
        }
        // Not a subcommand — try as project name
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
    return smartSummary()
},
```

- [ ] **Step 2: Add command groups**

Add to the `init()` function in `cmd/root.go`:

```go
rootCmd.AddGroup(
    &cobra.Group{ID: "core", Title: "Core Commands:"},
    &cobra.Group{ID: "query", Title: "Query Commands:"},
    &cobra.Group{ID: "workflow", Title: "Workflow Commands:"},
    &cobra.Group{ID: "maintenance", Title: "Maintenance Commands:"},
)
```

- [ ] **Step 3: Update existing commands with group IDs**

Update `init()` in each existing command file to set `GroupID`:
- `init.go`: `initCmd.GroupID = "core"`
- `scan.go`: `scanCmd.GroupID = "core"`
- `capture.go`: `captureCmd.GroupID = "core"`
- `projects.go`: `projectsCmd.GroupID = "query"`
- `sessions.go`: `sessionsCmd.GroupID = "query"`
- `search.go`: `searchCmd.GroupID = "query"`
- `show.go`: `showCmd.GroupID = "query"`
- `note.go`: `noteCmd.GroupID = "workflow"`
- `config.go`: `configCmd.GroupID = "maintenance"`

- [ ] **Step 4: Build and verify**

Run: `cd ~/projects-wsl/nexus && go build -o nexus . && ./nexus --help`
Expected: Commands organized by group

- [ ] **Step 5: Commit**

```bash
git add cmd/
git commit -m "feat: dynamic subcommand routing, command groups for help"
```

---

### Task 5: Tier 1 Commands — resume, diff, stale, watch, streak

**Files:**
- Create: `cmd/resume.go`
- Create: `cmd/diff.go`
- Create: `cmd/stale.go`
- Create: `cmd/watch.go`
- Create: `cmd/streak.go`

- [ ] **Step 1: Implement resume command**

```go
// cmd/resume.go
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/digitalghost404/nexus/internal/config"
	"github.com/digitalghost404/nexus/internal/db"
	"github.com/digitalghost404/nexus/internal/display"
	"github.com/digitalghost404/nexus/internal/scanner"
	"github.com/spf13/cobra"
)

var resumeCmd = &cobra.Command{
	Use:   "resume [project]",
	Short: "Pick up where you left off on a project",
	Long:  "Shows the last Claude session for a project with commits, files changed, and current uncommitted changes.",
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := db.Open(config.DBPath())
		if err != nil {
			return err
		}
		defer database.Close()

		// Resolve project
		var project *db.Project
		if len(args) > 0 {
			project, err = database.GetProjectByName(args[0])
		} else {
			cwd, _ := os.Getwd()
			absDir, _ := filepath.Abs(cwd)
			project, err = database.GetProjectByPath(absDir)
		}
		if err != nil {
			return err
		}
		if project == nil {
			return fmt.Errorf("project not found — run from inside a project or specify a name")
		}

		session, err := database.GetLatestSession(project.ID)
		if err != nil {
			return err
		}
		if session == nil {
			fmt.Printf("No sessions recorded for %s\n", project.Name)
			return nil
		}

		// Get live dirty files
		var dirtyFiles []string
		if scanner.IsGitRepo(project.Path) {
			details, _ := scanner.GetDirtyFileDetails(project.Path)
			for _, d := range details {
				dirtyFiles = append(dirtyFiles, fmt.Sprintf("%s %s", d.Status, d.Path))
			}
		}

		display.FormatResume(os.Stdout, session, dirtyFiles)
		return nil
	},
}

func init() {
	resumeCmd.GroupID = "workflow"
	rootCmd.AddCommand(resumeCmd)
}
```

- [ ] **Step 2: Implement diff command**

```go
// cmd/diff.go
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/digitalghost404/nexus/internal/config"
	"github.com/digitalghost404/nexus/internal/db"
	"github.com/digitalghost404/nexus/internal/display"
	"github.com/spf13/cobra"
)

var diffSince string

var diffCmd = &cobra.Command{
	Use:   "diff [project]",
	Short: "Summarize changes across sessions in a time window",
	Long:  "Aggregates activity across all sessions for a project within a time range, showing commits, files, and a timeline.",
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := db.Open(config.DBPath())
		if err != nil {
			return err
		}
		defer database.Close()

		var project *db.Project
		if len(args) > 0 {
			project, err = database.GetProjectByName(args[0])
		} else {
			cwd, _ := os.Getwd()
			absDir, _ := filepath.Abs(cwd)
			project, err = database.GetProjectByPath(absDir)
		}
		if err != nil {
			return err
		}
		if project == nil {
			return fmt.Errorf("project not found")
		}

		since, err := parseDuration(diffSince)
		if err != nil {
			return fmt.Errorf("invalid --since: %w", err)
		}

		days := 7
		fmt.Sscanf(diffSince, "%d", &days)

		sessions, err := database.GetSessionsInRange(project.ID, *since, time.Now())
		if err != nil {
			return err
		}

		display.FormatDiff(os.Stdout, project.Name, days, sessions)
		return nil
	},
}

func init() {
	diffCmd.Flags().StringVar(&diffSince, "since", "7d", "Time window (e.g. 7d, 30d)")
	diffCmd.GroupID = "workflow"
	rootCmd.AddCommand(diffCmd)
}
```

- [ ] **Step 3: Implement stale command**

```go
// cmd/stale.go
package cmd

import (
	"bufio"
	"fmt"
	"os"
	"time"

	"github.com/digitalghost404/nexus/internal/config"
	"github.com/digitalghost404/nexus/internal/db"
	"github.com/digitalghost404/nexus/internal/display"
	"github.com/digitalghost404/nexus/internal/scanner"
	"github.com/spf13/cobra"
)

var staleCleanup bool

var staleCmd = &cobra.Command{
	Use:   "stale",
	Short: "Show stale branches and idle projects",
	Long:  "Lists stale branches and dirty projects. Use --cleanup for interactive branch deletion.",
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := db.Open(config.DBPath())
		if err != nil {
			return err
		}
		defer database.Close()

		// Get idle and stale projects
		idle, _ := database.ListProjects("idle")
		staleProjs, _ := database.ListProjects("stale")
		allProjs := append(idle, staleProjs...)

		// Collect stale branches and dirty details
		branches := map[string][]display.StaleBranchInfo{}
		dirtyDetails := map[string][]string{}

		for _, p := range allProjs {
			if !scanner.IsGitRepo(p.Path) {
				continue
			}
			staleBranches, _ := scanner.GetStaleBranches(p.Path, 7*24*time.Hour)
			if len(staleBranches) > 0 {
				var infos []display.StaleBranchInfo
				for _, b := range staleBranches {
					infos = append(infos, display.StaleBranchInfo{Name: b, Age: "7+ days"})
				}
				branches[p.Name] = infos
			}

			if p.Dirty {
				details, _ := scanner.GetDirtyFileDetails(p.Path)
				var lines []string
				for _, d := range details {
					lines = append(lines, fmt.Sprintf("%s %s", d.Status, d.Path))
				}
				dirtyDetails[p.Name] = lines
			}
		}

		// Also check active+dirty projects
		dirtyProjs, _ := database.ListDirtyProjects()
		for _, p := range dirtyProjs {
			if _, exists := dirtyDetails[p.Name]; exists {
				continue
			}
			details, _ := scanner.GetDirtyFileDetails(p.Path)
			var lines []string
			for _, d := range details {
				lines = append(lines, fmt.Sprintf("%s %s", d.Status, d.Path))
			}
			if len(lines) > 0 {
				dirtyDetails[p.Name] = lines
			}
		}

		if !staleCleanup {
			display.FormatStale(os.Stdout, allProjs, branches, dirtyDetails)
			return nil
		}

		// Interactive cleanup
		reader := bufio.NewReader(os.Stdin)
		for project, brs := range branches {
			// Find the project to get its path
			var projPath string
			for _, p := range allProjs {
				if p.Name == project {
					projPath = p.Path
					break
				}
			}
			if projPath == "" {
				continue
			}

			fmt.Printf("\n%s\n", project)
			for _, b := range brs {
				fmt.Printf("  %s  (last commit: %s)\n", b.Name, b.Age)
				fmt.Printf("  Delete? [y/n/q] ")
				input, _ := reader.ReadString('\n')
				switch input[0] {
				case 'y', 'Y':
					err := scanner.DeleteBranch(projPath, b.Name)
					if err != nil {
						fmt.Printf("  ✗ Failed: %v\n", err)
					} else {
						fmt.Printf("  ✓ Deleted %s\n", b.Name)
					}
				case 'q', 'Q':
					fmt.Println("  Quitting cleanup.")
					return nil
				default:
					fmt.Println("  ⏭ Skipped")
				}
			}
		}

		// Show dirty projects (no auto-cleanup)
		if len(dirtyDetails) > 0 {
			fmt.Println("\nDirty projects (uncommitted changes):")
			for project, files := range dirtyDetails {
				fmt.Printf("\n  %s\n", project)
				for _, f := range files {
					fmt.Printf("    %s\n", f)
				}
				fmt.Println("    ⚠ Has uncommitted changes — review manually")
			}
		}

		return nil
	},
}

func init() {
	staleCmd.Flags().BoolVar(&staleCleanup, "cleanup", false, "Interactive branch cleanup")
	staleCmd.GroupID = "maintenance"
	rootCmd.AddCommand(staleCmd)
}
```

- [ ] **Step 4: Implement watch command**

```go
// cmd/watch.go
package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/digitalghost404/nexus/internal/config"
	"github.com/digitalghost404/nexus/internal/db"
	"github.com/digitalghost404/nexus/internal/display"
	"github.com/spf13/cobra"
)

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Live-updating project dashboard",
	Long:  "Auto-refreshing terminal display of project status. Updates every 30 seconds.",
	RunE: func(cmd *cobra.Command, args []string) error {
		for {
			clearScreen()

			database, err := db.Open(config.DBPath())
			if err != nil {
				fmt.Fprintf(os.Stderr, "db error: %v\n", err)
				time.Sleep(30 * time.Second)
				continue
			}

			dirty, _ := database.ListDirtyProjects()
			sessions, _ := database.ListSessions(db.SessionFilter{Limit: 5})
			stale, _ := database.ListProjects("stale")

			cwd, _ := os.Getwd()
			absDir, _ := filepath.Abs(cwd)
			currentProject := ""
			p, _ := database.GetProjectByPath(absDir)
			if p != nil {
				currentProject = p.Name
			}

			display.FormatSmartSummary(os.Stdout, dirty, sessions, stale, currentProject)
			fmt.Println("Refreshing every 30s — Ctrl+C to exit")
			fmt.Printf("Last refresh: %s\n", time.Now().Format("15:04:05"))

			database.Close()
			time.Sleep(30 * time.Second)
		}
	},
}

func clearScreen() {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", "cls")
	} else {
		cmd = exec.Command("clear")
	}
	cmd.Stdout = os.Stdout
	cmd.Run()
}

func init() {
	watchCmd.GroupID = "maintenance"
	rootCmd.AddCommand(watchCmd)
}
```

- [ ] **Step 5: Implement streak command**

```go
// cmd/streak.go
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

var streakCmd = &cobra.Command{
	Use:   "streak",
	Short: "Show your coding streak",
	Long:  "Shows consecutive days with Claude sessions, plus weekly activity bars.",
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := db.Open(config.DBPath())
		if err != nil {
			return err
		}
		defer database.Close()

		dates, err := database.GetDistinctSessionDates()
		if err != nil {
			return err
		}

		if len(dates) == 0 {
			fmt.Println("No sessions recorded yet.")
			return nil
		}

		today := time.Now().Format("2006-01-02")
		dateSet := map[string]bool{}
		for _, d := range dates {
			dateSet[d] = true
		}

		// Calculate current streak
		current := 0
		day := today
		for dateSet[day] {
			current++
			t, _ := time.Parse("2006-01-02", day)
			day = t.AddDate(0, 0, -1).Format("2006-01-02")
		}

		// Calculate longest streak
		longest := 0
		longestStart, longestEnd := "", ""
		streak := 0
		streakStart := ""
		for i := len(dates) - 1; i >= 0; i-- {
			if i == len(dates)-1 {
				streak = 1
				streakStart = dates[i]
			} else {
				prev, _ := time.Parse("2006-01-02", dates[i+1])
				curr, _ := time.Parse("2006-01-02", dates[i])
				if curr.Sub(prev) == 24*time.Hour {
					streak++
				} else {
					if streak > longest {
						longest = streak
						longestStart = streakStart
						longestEnd = dates[i+1]
					}
					streak = 1
					streakStart = dates[i]
				}
			}
		}
		if streak > longest {
			longest = streak
			longestStart = streakStart
			longestEnd = dates[0]
		}

		// Format longest dates
		fmtDate := func(s string) string {
			t, err := time.Parse("2006-01-02", s)
			if err != nil {
				return s
			}
			return t.Format("Jan 02")
		}

		// Weekly bars
		now := time.Now()
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}

		thisWeek := make([]bool, 7)
		lastWeek := make([]bool, 7)

		for i := 0; i < 7; i++ {
			d := now.AddDate(0, 0, -(weekday-1-i))
			thisWeek[i] = dateSet[d.Format("2006-01-02")]
		}
		for i := 0; i < 7; i++ {
			d := now.AddDate(0, 0, -(weekday-1-i+7))
			lastWeek[i] = dateSet[d.Format("2006-01-02")]
		}

		display.FormatStreak(os.Stdout, current, longest, fmtDate(longestStart), fmtDate(longestEnd), thisWeek, lastWeek)
		return nil
	},
}

func init() {
	streakCmd.GroupID = "workflow"
	rootCmd.AddCommand(streakCmd)
}
```

- [ ] **Step 6: Build and test all commands**

Run:
```bash
cd ~/projects-wsl/nexus
go build -o nexus .
./nexus resume
./nexus diff --since 30d
./nexus stale
./nexus streak
```

- [ ] **Step 7: Commit**

```bash
git add cmd/resume.go cmd/diff.go cmd/stale.go cmd/watch.go cmd/streak.go
git commit -m "feat: add resume, diff, stale, watch, streak commands"
```

---

### Task 6: Tier 1 Commands — deps, report, context, hook, where

**Files:**
- Create: `cmd/deps.go`
- Create: `cmd/report.go`
- Create: `cmd/context_cmd.go`
- Create: `cmd/hook.go`
- Create: `cmd/where.go`

- [ ] **Step 1: Implement deps command**

```go
// cmd/deps.go
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/digitalghost404/nexus/internal/config"
	"github.com/digitalghost404/nexus/internal/db"
	"github.com/digitalghost404/nexus/internal/display"
	"github.com/spf13/cobra"
)

var depsProject string

var depsCmd = &cobra.Command{
	Use:   "deps",
	Short: "Check for outdated dependencies",
	Long:  "Scans all tracked projects for outdated Go, npm, and pip dependencies.",
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := db.Open(config.DBPath())
		if err != nil {
			return err
		}
		defer database.Close()

		var projects []db.Project
		if depsProject != "" {
			p, _ := database.GetProjectByName(depsProject)
			if p == nil {
				return fmt.Errorf("project not found: %s", depsProject)
			}
			projects = []db.Project{*p}
		} else {
			projects, _ = database.ListProjects("")
		}

		hasGo, _ := exec.LookPath("go")
		hasNpm, _ := exec.LookPath("npm")
		hasPip, _ := exec.LookPath("pip3")

		var results []display.ProjectDeps
		cleanCount := 0

		for _, p := range projects {
			fmt.Fprintf(os.Stderr, "Checking %s...\n", p.Name)
			var outdated []display.DepInfo
			var manager string

			goMod := filepath.Join(p.Path, "go.mod")
			pkgJSON := filepath.Join(p.Path, "package.json")
			reqTxt := filepath.Join(p.Path, "requirements.txt")

			if _, err := os.Stat(goMod); err == nil && hasGo != "" {
				manager = "Go"
				outdated = checkGoDeps(p.Path)
			} else if _, err := os.Stat(pkgJSON); err == nil && hasNpm != "" {
				manager = "npm"
				outdated = checkNpmDeps(p.Path)
			} else if _, err := os.Stat(reqTxt); err == nil && hasPip != "" {
				manager = "pip"
				outdated = checkPipDeps(p.Path)
			}

			if len(outdated) > 0 {
				results = append(results, display.ProjectDeps{
					ProjectName: p.Name,
					Manager:     manager,
					Outdated:    outdated,
				})
			} else if manager != "" {
				cleanCount++
			}
		}

		display.FormatDeps(os.Stdout, results, cleanCount)
		return nil
	},
}

func checkGoDeps(dir string) []display.DepInfo {
	cmd := exec.Command("go", "list", "-m", "-u", "-json", "all")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	var deps []display.DepInfo
	dec := json.NewDecoder(strings.NewReader(string(out)))
	for dec.More() {
		var m struct {
			Path    string
			Version string
			Update  *struct {
				Version string
			}
			Main    bool
			Indirect bool
		}
		if err := dec.Decode(&m); err != nil {
			break
		}
		if m.Update != nil && !m.Main {
			deps = append(deps, display.DepInfo{
				Name:      m.Path,
				Current:   m.Version,
				Available: m.Update.Version,
			})
		}
	}
	return deps
}

func checkNpmDeps(dir string) []display.DepInfo {
	cmd := exec.Command("npm", "outdated", "--json")
	cmd.Dir = dir
	out, _ := cmd.Output() // npm outdated exits 1 when outdated packages exist

	var result map[string]struct {
		Current string `json:"current"`
		Latest  string `json:"latest"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return nil
	}

	var deps []display.DepInfo
	for name, info := range result {
		if info.Current != info.Latest {
			deps = append(deps, display.DepInfo{
				Name:      name,
				Current:   info.Current,
				Available: info.Latest,
			})
		}
	}
	return deps
}

func checkPipDeps(dir string) []display.DepInfo {
	cmd := exec.Command("pip3", "list", "--outdated", "--format=json")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	var result []struct {
		Name           string `json:"name"`
		Version        string `json:"version"`
		LatestVersion  string `json:"latest_version"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return nil
	}

	var deps []display.DepInfo
	for _, p := range result {
		deps = append(deps, display.DepInfo{
			Name:      p.Name,
			Current:   p.Version,
			Available: p.LatestVersion,
		})
	}
	return deps
}

func init() {
	depsCmd.Flags().StringVar(&depsProject, "project", "", "Check a single project")
	depsCmd.GroupID = "maintenance"
	rootCmd.AddCommand(depsCmd)
}
```

- [ ] **Step 2: Implement report command**

```go
// cmd/report.go
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/digitalghost404/nexus/internal/config"
	"github.com/digitalghost404/nexus/internal/db"
	"github.com/digitalghost404/nexus/internal/display"
	"github.com/spf13/cobra"
)

var (
	reportWeek  bool
	reportMonth bool
)

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Generate activity summary for a time period",
	Long:  "Shows sessions, commits, files changed, most active projects, and language breakdown.",
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := db.Open(config.DBPath())
		if err != nil {
			return err
		}
		defer database.Close()

		now := time.Now()
		var since time.Time
		if reportMonth {
			since = now.AddDate(0, -1, 0)
		} else {
			since = now.AddDate(0, 0, -7) // default: week
		}

		sessions, _ := database.ListSessions(db.SessionFilter{Since: &since, Limit: 1000})

		// Aggregate
		projectMap := map[string]*display.ProjectActivity{}
		langCount := map[string]int{}
		totalCommits := 0
		fileSet := map[string]bool{}
		totalSecs := 0

		for _, s := range sessions {
			// Project activity
			pa, ok := projectMap[s.ProjectName]
			if !ok {
				pa = &display.ProjectActivity{Name: s.ProjectName}
				projectMap[s.ProjectName] = pa
			}
			pa.Sessions++

			var commits []struct{ Hash, Message string }
			json.Unmarshal([]byte(s.CommitsMade), &commits)
			pa.Commits += len(commits)
			totalCommits += len(commits)

			var files []string
			json.Unmarshal([]byte(s.FilesChanged), &files)
			for _, f := range files {
				fileSet[f] = true
			}

			var tags []string
			json.Unmarshal([]byte(s.Tags), &tags)
			for _, t := range tags {
				if t != s.ProjectName { // skip project name tag
					langCount[t]++
				}
			}

			totalSecs += s.DurationSecs
		}

		// Sort projects by session count
		var topProjects []display.ProjectActivity
		for _, pa := range projectMap {
			topProjects = append(topProjects, *pa)
		}
		for i := 0; i < len(topProjects); i++ {
			for j := i + 1; j < len(topProjects); j++ {
				if topProjects[j].Sessions > topProjects[i].Sessions {
					topProjects[i], topProjects[j] = topProjects[j], topProjects[i]
				}
			}
		}
		if len(topProjects) > 5 {
			topProjects = topProjects[:5]
		}

		// Language percentages
		totalLang := 0
		for _, c := range langCount {
			totalLang += c
		}
		var languages []display.LangPercent
		for lang, count := range langCount {
			if totalLang > 0 {
				languages = append(languages, display.LangPercent{
					Lang:    lang,
					Percent: count * 100 / totalLang,
				})
			}
		}
		// Sort by percent desc
		for i := 0; i < len(languages); i++ {
			for j := i + 1; j < len(languages); j++ {
				if languages[j].Percent > languages[i].Percent {
					languages[i], languages[j] = languages[j], languages[i]
				}
			}
		}

		// Highlights: top 3 sessions by files changed
		type sessionFiles struct {
			summary string
			count   int
		}
		var sf []sessionFiles
		for _, s := range sessions {
			var files []string
			json.Unmarshal([]byte(s.FilesChanged), &files)
			sf = append(sf, sessionFiles{s.Summary, len(files)})
		}
		for i := 0; i < len(sf); i++ {
			for j := i + 1; j < len(sf); j++ {
				if sf[j].count > sf[i].count {
					sf[i], sf[j] = sf[j], sf[i]
				}
			}
		}
		var highlights []string
		limit := 3
		if len(sf) < limit {
			limit = len(sf)
		}
		for _, s := range sf[:limit] {
			if s.summary != "" {
				highlights = append(highlights, s.summary)
			}
		}

		r := display.ReportData{
			StartDate:     since.Format("Jan 02"),
			EndDate:       now.Format("Jan 02"),
			TotalSessions: len(sessions),
			TotalProjects: len(projectMap),
			TotalCommits:  totalCommits,
			TotalFiles:    len(fileSet),
			TotalHours:    float64(totalSecs) / 3600.0,
			TopProjects:   topProjects,
			Languages:     languages,
			Highlights:    highlights,
		}

		display.FormatReport(os.Stdout, r)
		return nil
	},
}

func init() {
	reportCmd.Flags().BoolVar(&reportWeek, "week", false, "Report for last week (default)")
	reportCmd.Flags().BoolVar(&reportMonth, "month", false, "Report for last month")
	reportCmd.GroupID = "workflow"
	rootCmd.AddCommand(reportCmd)
}
```

- [ ] **Step 3: Implement context command**

```go
// cmd/context_cmd.go
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

var contextCmd = &cobra.Command{
	Use:   "context <project>",
	Short: "Export project context for pasting into Claude",
	Long:  "Outputs everything Nexus knows about a project in markdown format, optimized for sharing with Claude.",
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

		since := time.Now().AddDate(0, 0, -7)
		sessions, _ := database.GetSessionsInRange(p.ID, since, time.Now())
		notes, _ := database.ListNotes(p.ID, 10)

		// Linked projects — conditional on v2 migration
		var linkedProjects []db.Project
		// Try to query project_links; if table doesn't exist, just skip
		linkedProjects, _ = database.GetLinkedProjects(p.ID)

		display.FormatContext(os.Stdout, p, sessions, notes, linkedProjects)
		return nil
	},
}

func init() {
	contextCmd.GroupID = "workflow"
	rootCmd.AddCommand(contextCmd)
}
```

- [ ] **Step 4: Implement hook command**

```go
// cmd/hook.go
package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

const canonicalWrapper = `claude() { command claude "$@"; local rc=$?; nexus capture --dir "$PWD"; return $rc; }`

var hookCmd = &cobra.Command{
	Use:   "hook",
	Short: "Install or uninstall shell wrapper and cron job",
}

var hookInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install claude() wrapper and nexus scan cron job",
	RunE: func(cmd *cobra.Command, args []string) error {
		home, _ := os.UserHomeDir()
		bashrc := home + "/.bashrc"

		// Read .bashrc
		data, err := os.ReadFile(bashrc)
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("read .bashrc: %w", err)
		}
		content := string(data)

		// Check for existing claude() function
		if strings.Contains(content, "claude()") || strings.Contains(content, "claude ()") {
			if strings.Contains(content, canonicalWrapper) {
				fmt.Println("✓ Shell wrapper already installed")
			} else {
				fmt.Println("⚠ Existing claude() function found in .bashrc — not overwriting. Add manually:")
				fmt.Printf("\n  %s\n\n", canonicalWrapper)
			}
		} else {
			// Append wrapper
			f, err := os.OpenFile(bashrc, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				return fmt.Errorf("open .bashrc: %w", err)
			}
			fmt.Fprintf(f, "\n# Nexus: auto-capture Claude sessions\n%s\n", canonicalWrapper)
			f.Close()
			fmt.Println("✓ Added claude() wrapper to ~/.bashrc")
		}

		// Check cron
		cronOut, _ := exec.Command("crontab", "-l").Output()
		if strings.Contains(string(cronOut), "nexus scan") {
			fmt.Println("✓ Cron job already installed")
		} else {
			nexusPath := home + "/go/bin/nexus"
			nexusDir := home + "/.nexus"
			cronLine := fmt.Sprintf("*/30 * * * * %s scan >> %s/nexus.log 2>&1", nexusPath, nexusDir)

			newCron := string(cronOut) + cronLine + "\n"
			cronCmd := exec.Command("crontab", "-")
			cronCmd.Stdin = strings.NewReader(newCron)
			if err := cronCmd.Run(); err != nil {
				return fmt.Errorf("install cron: %w", err)
			}
			fmt.Println("✓ Installed cron job: nexus scan every 30 minutes")
		}

		fmt.Println("\nRun: source ~/.bashrc")
		return nil
	},
}

var hookUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove claude() wrapper and nexus scan cron job",
	RunE: func(cmd *cobra.Command, args []string) error {
		home, _ := os.UserHomeDir()
		bashrc := home + "/.bashrc"

		// Remove wrapper from .bashrc
		data, err := os.ReadFile(bashrc)
		if err == nil {
			lines := strings.Split(string(data), "\n")
			var filtered []string
			for _, line := range lines {
				if strings.Contains(line, "nexus capture") || strings.Contains(line, "# Nexus: auto-capture") {
					continue
				}
				filtered = append(filtered, line)
			}
			os.WriteFile(bashrc, []byte(strings.Join(filtered, "\n")), 0644)
			fmt.Println("✓ Removed claude() wrapper from ~/.bashrc")
		}

		// Remove cron line
		cronOut, _ := exec.Command("crontab", "-l").Output()
		if strings.Contains(string(cronOut), "nexus scan") {
			lines := strings.Split(string(cronOut), "\n")
			var filtered []string
			for _, line := range lines {
				if !strings.Contains(line, "nexus scan") {
					filtered = append(filtered, line)
				}
			}
			cronCmd := exec.Command("crontab", "-")
			cronCmd.Stdin = strings.NewReader(strings.Join(filtered, "\n"))
			cronCmd.Run()
			fmt.Println("✓ Removed nexus scan cron job")
		}

		return nil
	},
}

func init() {
	hookCmd.AddCommand(hookInstallCmd)
	hookCmd.AddCommand(hookUninstallCmd)
	hookCmd.GroupID = "maintenance"
	rootCmd.AddCommand(hookCmd)
}
```

- [ ] **Step 5: Implement where command**

```go
// cmd/where.go
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

var whereCmd = &cobra.Command{
	Use:   "where <query>",
	Short: "Find which projects and files match a query",
	Long:  "Searches session summaries and file paths, then groups results by project and file.",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := db.Open(config.DBPath())
		if err != nil {
			return err
		}
		defer database.Close()

		query := strings.Join(args, " ")

		// Search two sources: FTS on summary + LIKE on files_changed
		ftsResults, _ := database.SearchSessions(query)

		allSessions, _ := database.ListSessions(db.SessionFilter{Limit: 1000})
		var fileResults []db.Session
		for _, s := range allSessions {
			if strings.Contains(strings.ToLower(s.FilesChanged), strings.ToLower(query)) {
				fileResults = append(fileResults, s)
			}
		}

		// Merge and deduplicate
		seen := map[int64]bool{}
		var merged []db.Session
		for _, s := range ftsResults {
			if !seen[s.ID] {
				seen[s.ID] = true
				merged = append(merged, s)
			}
		}
		for _, s := range fileResults {
			if !seen[s.ID] {
				seen[s.ID] = true
				merged = append(merged, s)
			}
		}

		// Group by project → file → sessions
		type fileEntry struct {
			dates []string
		}
		projectFiles := map[string]map[string]*fileEntry{}

		for _, s := range merged {
			if _, ok := projectFiles[s.ProjectName]; !ok {
				projectFiles[s.ProjectName] = map[string]*fileEntry{}
			}
			var files []string
			json.Unmarshal([]byte(s.FilesChanged), &files)
			dateStr := ""
			if s.StartedAt != nil {
				dateStr = s.StartedAt.Format("Jan 02")
			}
			for _, f := range files {
				if _, ok := projectFiles[s.ProjectName][f]; !ok {
					projectFiles[s.ProjectName][f] = &fileEntry{}
				}
				projectFiles[s.ProjectName][f].dates = append(projectFiles[s.ProjectName][f].dates, dateStr)
			}
		}

		// Build display results
		var results []display.WhereResult
		for project, files := range projectFiles {
			wr := display.WhereResult{ProjectName: project}
			for path, entry := range files {
				wr.Files = append(wr.Files, display.WhereFile{
					Path:     path,
					Sessions: entry.dates,
				})
			}
			results = append(results, wr)
		}

		display.FormatWhere(os.Stdout, results)
		return nil
	},
}

func init() {
	whereCmd.GroupID = "query"
	rootCmd.AddCommand(whereCmd)
}
```

- [ ] **Step 6: Build and test all commands**

Run:
```bash
cd ~/projects-wsl/nexus
go build -o nexus .
./nexus deps --project nexus
./nexus report
./nexus context nexus
./nexus where "scanner"
./nexus --help
```

- [ ] **Step 7: Commit**

```bash
git add cmd/deps.go cmd/report.go cmd/context_cmd.go cmd/hook.go cmd/where.go
git commit -m "feat: add deps, report, context, hook, where commands"
```

---

### Task 7: Schema Migration v1→v2

**Files:**
- Modify: `internal/db/schema.sql`
- Create: `internal/db/migration_v2.sql`
- Modify: `internal/db/db.go`
- Modify: `internal/db/db_test.go`

- [ ] **Step 1: Write failing test for migration**

```go
// Add to internal/db/db_test.go

func TestMigrationV1ToV2(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	// Create a v1 database manually
	sqlDB, _ := sql.Open("sqlite", dbPath)
	sqlDB.Exec("PRAGMA journal_mode=WAL")
	sqlDB.Exec(schemaSQL) // current schema
	sqlDB.Exec("PRAGMA user_version = 1")

	// Insert some v1 data
	sqlDB.Exec("INSERT INTO projects (name, path, status, discovered_at) VALUES ('test', '/test', 'active', datetime('now'))")
	sqlDB.Close()

	// Open with new migration code
	d, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open with migration failed: %v", err)
	}
	defer d.Close()

	// Verify v2 tables exist
	var count int
	err = d.db.QueryRow("SELECT count(*) FROM project_links").Scan(&count)
	if err != nil {
		t.Fatalf("project_links table missing: %v", err)
	}
	err = d.db.QueryRow("SELECT count(*) FROM session_tags").Scan(&count)
	if err != nil {
		t.Fatalf("session_tags table missing: %v", err)
	}

	// Verify version is now 2
	var version int
	d.db.QueryRow("PRAGMA user_version").Scan(&version)
	if version != 2 {
		t.Errorf("expected user_version=2, got %d", version)
	}

	// Verify existing data survived
	var projName string
	d.db.QueryRow("SELECT name FROM projects WHERE path = '/test'").Scan(&projName)
	if projName != "test" {
		t.Errorf("existing data lost during migration")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd ~/projects-wsl/nexus && go test ./internal/db/ -v -run TestMigrationV1ToV2`
Expected: FAIL (migration_v2.sql doesn't exist, migrate doesn't handle version 1)

- [ ] **Step 3: Create migration_v2.sql**

```sql
-- internal/db/migration_v2.sql
-- Delta migration from schema v1 to v2

CREATE TABLE IF NOT EXISTS project_links (
    id                INTEGER PRIMARY KEY,
    project_id        INTEGER NOT NULL REFERENCES projects(id),
    linked_project_id INTEGER NOT NULL REFERENCES projects(id),
    created_at        DATETIME NOT NULL DEFAULT (datetime('now')),
    UNIQUE(project_id, linked_project_id)
);

CREATE INDEX IF NOT EXISTS idx_project_links_project ON project_links(project_id);

CREATE TABLE IF NOT EXISTS session_tags (
    id          INTEGER PRIMARY KEY,
    session_id  INTEGER NOT NULL REFERENCES sessions(id),
    tag         TEXT NOT NULL,
    created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
    UNIQUE(session_id, tag)
);

CREATE INDEX IF NOT EXISTS idx_session_tags_tag ON session_tags(tag);
```

- [ ] **Step 4: Update schema.sql to include v2 tables**

Append the same CREATE TABLE and CREATE INDEX statements to the end of `schema.sql` so fresh installs get everything.

- [ ] **Step 5: Update db.go migrate() function**

```go
//go:embed migration_v2.sql
var migrationV2SQL string

func (d *DB) migrate() error {
	var version int
	d.db.QueryRow("PRAGMA user_version").Scan(&version)

	if version == 0 {
		if _, err := d.db.Exec(schemaSQL); err != nil {
			return fmt.Errorf("apply schema: %w", err)
		}
		if _, err := d.db.Exec("PRAGMA user_version = 2"); err != nil {
			return fmt.Errorf("set version: %w", err)
		}
	}

	if version == 1 {
		if _, err := d.db.Exec(migrationV2SQL); err != nil {
			return fmt.Errorf("apply v2 migration: %w", err)
		}
		if _, err := d.db.Exec("PRAGMA user_version = 2"); err != nil {
			return fmt.Errorf("set version: %w", err)
		}
	}

	return nil
}
```

- [ ] **Step 6: Update TestSchemaVersion to expect version 2**

- [ ] **Step 7: Run tests**

Run: `cd ~/projects-wsl/nexus && go test ./internal/db/ -v`
Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add internal/db/
git commit -m "feat: schema migration v1→v2 with project_links and session_tags tables"
```

---

### Task 8: DB Methods for Links and Tags

**Files:**
- Create: `internal/db/links.go`
- Create: `internal/db/links_test.go`
- Create: `internal/db/tags.go`
- Create: `internal/db/tags_test.go`

- [ ] **Step 1: Write failing tests for links**

```go
// internal/db/links_test.go
package db

import (
	"testing"
	"time"
)

func TestLinkAndGetProjects(t *testing.T) {
	d := testDB(t)
	now := time.Now()
	id1, _ := d.UpsertProject(Project{Name: "wraith", Path: "/a", Status: "active", DiscoveredAt: now})
	id2, _ := d.UpsertProject(Project{Name: "dashboard", Path: "/b", Status: "active", DiscoveredAt: now})

	err := d.LinkProjects(id1, id2)
	if err != nil {
		t.Fatalf("link: %v", err)
	}

	// Check from both directions
	linked1, _ := d.GetLinkedProjects(id1)
	if len(linked1) != 1 || linked1[0].Name != "dashboard" {
		t.Errorf("expected dashboard linked to wraith, got %v", linked1)
	}

	linked2, _ := d.GetLinkedProjects(id2)
	if len(linked2) != 1 || linked2[0].Name != "wraith" {
		t.Errorf("expected wraith linked to dashboard, got %v", linked2)
	}
}

func TestUnlinkProjects(t *testing.T) {
	d := testDB(t)
	now := time.Now()
	id1, _ := d.UpsertProject(Project{Name: "a", Path: "/a", Status: "active", DiscoveredAt: now})
	id2, _ := d.UpsertProject(Project{Name: "b", Path: "/b", Status: "active", DiscoveredAt: now})

	d.LinkProjects(id1, id2)
	d.UnlinkProjects(id1, id2)

	linked, _ := d.GetLinkedProjects(id1)
	if len(linked) != 0 {
		t.Errorf("expected no links after unlink, got %d", len(linked))
	}
}

func TestLinkIdempotent(t *testing.T) {
	d := testDB(t)
	now := time.Now()
	id1, _ := d.UpsertProject(Project{Name: "a", Path: "/a", Status: "active", DiscoveredAt: now})
	id2, _ := d.UpsertProject(Project{Name: "b", Path: "/b", Status: "active", DiscoveredAt: now})

	d.LinkProjects(id1, id2)
	err := d.LinkProjects(id1, id2) // should not error
	if err != nil {
		t.Errorf("duplicate link should not error: %v", err)
	}
}
```

- [ ] **Step 2: Write failing tests for tags**

```go
// internal/db/tags_test.go
package db

import (
	"testing"
	"time"
)

func TestAddAndListSessionTags(t *testing.T) {
	d := testDB(t)
	now := time.Now()
	pID, _ := d.UpsertProject(Project{Name: "proj", Path: "/a", Status: "active", DiscoveredAt: now})
	sID, _ := d.InsertSession(Session{ProjectID: pID, Summary: "test", Source: "wrapper", StartedAt: &now})

	d.AddSessionTag(sID, "breakthrough")
	d.AddSessionTag(sID, "important")

	tags, _ := d.ListSessionTags(sID)
	if len(tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(tags))
	}
}

func TestRemoveSessionTag(t *testing.T) {
	d := testDB(t)
	now := time.Now()
	pID, _ := d.UpsertProject(Project{Name: "proj", Path: "/a", Status: "active", DiscoveredAt: now})
	sID, _ := d.InsertSession(Session{ProjectID: pID, Summary: "test", Source: "wrapper", StartedAt: &now})

	d.AddSessionTag(sID, "remove-me")
	d.RemoveSessionTag(sID, "remove-me")

	tags, _ := d.ListSessionTags(sID)
	if len(tags) != 0 {
		t.Errorf("expected 0 tags after remove, got %d", len(tags))
	}
}

func TestListSessionsByTag(t *testing.T) {
	d := testDB(t)
	now := time.Now()
	pID, _ := d.UpsertProject(Project{Name: "proj", Path: "/a", Status: "active", DiscoveredAt: now})
	s1, _ := d.InsertSession(Session{ProjectID: pID, Summary: "tagged", Source: "wrapper", StartedAt: &now})
	d.InsertSession(Session{ProjectID: pID, Summary: "not tagged", Source: "wrapper", StartedAt: &now})

	d.AddSessionTag(s1, "special")

	sessions, _ := d.ListSessionsByTag("special")
	if len(sessions) != 1 || sessions[0].Summary != "tagged" {
		t.Errorf("expected 1 tagged session, got %d", len(sessions))
	}
}
```

- [ ] **Step 3: Implement links.go**

```go
// internal/db/links.go
package db

import "fmt"

func (d *DB) LinkProjects(projectID, linkedProjectID int64) error {
	_, err := d.db.Exec(`
		INSERT OR IGNORE INTO project_links (project_id, linked_project_id) VALUES (?, ?)`,
		projectID, linkedProjectID)
	if err != nil {
		return fmt.Errorf("link: %w", err)
	}
	_, err = d.db.Exec(`
		INSERT OR IGNORE INTO project_links (project_id, linked_project_id) VALUES (?, ?)`,
		linkedProjectID, projectID)
	return err
}

func (d *DB) UnlinkProjects(projectID, linkedProjectID int64) error {
	d.db.Exec("DELETE FROM project_links WHERE project_id = ? AND linked_project_id = ?", projectID, linkedProjectID)
	d.db.Exec("DELETE FROM project_links WHERE project_id = ? AND linked_project_id = ?", linkedProjectID, projectID)
	return nil
}

func (d *DB) GetLinkedProjects(projectID int64) ([]Project, error) {
	rows, err := d.db.Query(`
		SELECT p.id, p.name, p.path, p.languages, p.branch, p.dirty, p.dirty_files,
			p.last_commit_at, p.last_commit_msg, p.ahead, p.behind, p.status,
			p.discovered_at, p.last_scanned_at
		FROM project_links pl
		JOIN projects p ON p.id = pl.linked_project_id
		WHERE pl.project_id = ?
		ORDER BY p.name`, projectID)
	if err != nil {
		return nil, fmt.Errorf("get linked: %w", err)
	}
	defer rows.Close()

	var projects []Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.Name, &p.Path, &p.Languages, &p.Branch,
			&p.Dirty, &p.DirtyFiles, &p.LastCommitAt, &p.LastCommitMsg,
			&p.Ahead, &p.Behind, &p.Status, &p.DiscoveredAt, &p.LastScannedAt); err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	return projects, nil
}
```

- [ ] **Step 4: Implement tags.go**

```go
// internal/db/tags.go
package db

import "fmt"

func (d *DB) AddSessionTag(sessionID int64, tag string) error {
	_, err := d.db.Exec(`
		INSERT OR IGNORE INTO session_tags (session_id, tag) VALUES (?, ?)`,
		sessionID, tag)
	if err != nil {
		return fmt.Errorf("add tag: %w", err)
	}
	return nil
}

func (d *DB) RemoveSessionTag(sessionID int64, tag string) error {
	_, err := d.db.Exec("DELETE FROM session_tags WHERE session_id = ? AND tag = ?", sessionID, tag)
	return err
}

func (d *DB) ListSessionTags(sessionID int64) ([]string, error) {
	rows, err := d.db.Query("SELECT tag FROM session_tags WHERE session_id = ? ORDER BY tag", sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var tag string
		rows.Scan(&tag)
		tags = append(tags, tag)
	}
	return tags, nil
}

func (d *DB) ListSessionsByTag(tag string) ([]Session, error) {
	rows, err := d.db.Query(`
		SELECT s.id, s.project_id, COALESCE(s.claude_session_id, ''),
			s.started_at, s.ended_at, s.duration_secs, COALESCE(s.summary, ''),
			s.files_changed, s.commits_made, s.tags, s.source, s.created_at,
			p.name
		FROM session_tags st
		JOIN sessions s ON s.id = st.session_id
		JOIN projects p ON p.id = s.project_id
		WHERE st.tag = ?
		ORDER BY s.started_at DESC`, tag)
	if err != nil {
		return nil, fmt.Errorf("sessions by tag: %w", err)
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var s Session
		if err := rows.Scan(&s.ID, &s.ProjectID, &s.ClaudeSessionID,
			&s.StartedAt, &s.EndedAt, &s.DurationSecs, &s.Summary,
			&s.FilesChanged, &s.CommitsMade, &s.Tags, &s.Source, &s.CreatedAt,
			&s.ProjectName); err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	return sessions, nil
}
```

- [ ] **Step 5: Run all tests**

Run: `cd ~/projects-wsl/nexus && go test ./internal/db/ -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/db/links.go internal/db/links_test.go internal/db/tags.go internal/db/tags_test.go
git commit -m "feat: project links and session tags CRUD"
```

---

### Task 9: Tier 2 Commands — link, tag + sessions --tag flag

**Files:**
- Create: `cmd/link.go`
- Create: `cmd/tag.go`
- Modify: `cmd/sessions.go`

- [ ] **Step 1: Implement link command**

```go
// cmd/link.go
package cmd

import (
	"fmt"

	"github.com/digitalghost404/nexus/internal/config"
	"github.com/digitalghost404/nexus/internal/db"
	"github.com/spf13/cobra"
)

var linkUnlink string

var linkCmd = &cobra.Command{
	Use:   "link <project-a> [project-b]",
	Short: "Link related projects together",
	Long:  "Creates a bidirectional link between projects. With one arg, shows existing links.",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := db.Open(config.DBPath())
		if err != nil {
			return err
		}
		defer database.Close()

		p1, err := database.GetProjectByName(args[0])
		if err != nil || p1 == nil {
			return fmt.Errorf("project not found: %s", args[0])
		}

		// Unlink
		if linkUnlink != "" {
			p2, err := database.GetProjectByName(linkUnlink)
			if err != nil || p2 == nil {
				return fmt.Errorf("project not found: %s", linkUnlink)
			}
			database.UnlinkProjects(p1.ID, p2.ID)
			fmt.Printf("✓ Unlinked %s ↔ %s\n", p1.Name, p2.Name)
			return nil
		}

		// Show links (one arg)
		if len(args) == 1 {
			linked, _ := database.GetLinkedProjects(p1.ID)
			if len(linked) == 0 {
				fmt.Printf("No linked projects for %s\n", p1.Name)
				return nil
			}
			fmt.Printf("Linked projects for %s:\n", p1.Name)
			for _, lp := range linked {
				fmt.Printf("  %s\n", lp.Name)
			}
			return nil
		}

		// Create link (two args)
		p2, err := database.GetProjectByName(args[1])
		if err != nil || p2 == nil {
			return fmt.Errorf("project not found: %s", args[1])
		}

		if err := database.LinkProjects(p1.ID, p2.ID); err != nil {
			return err
		}
		fmt.Printf("✓ Linked %s ↔ %s\n", p1.Name, p2.Name)
		return nil
	},
}

func init() {
	linkCmd.Flags().StringVar(&linkUnlink, "unlink", "", "Unlink a project")
	linkCmd.GroupID = "maintenance"
	rootCmd.AddCommand(linkCmd)
}
```

- [ ] **Step 2: Implement tag command**

```go
// cmd/tag.go
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/digitalghost404/nexus/internal/config"
	"github.com/digitalghost404/nexus/internal/db"
	"github.com/spf13/cobra"
)

var tagRemove string

var tagCmd = &cobra.Command{
	Use:   "tag [session-id] <label>",
	Short: "Tag sessions with labels",
	Long:  "Adds a user tag to a session. Without a session ID, tags the latest session for the current project.",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := db.Open(config.DBPath())
		if err != nil {
			return err
		}
		defer database.Close()

		var sessionID int64
		var label string

		// Parse args: first arg might be a session ID (numeric) or a label
		if len(args) >= 2 {
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err == nil {
				sessionID = id
				label = args[1]
			} else {
				// Both args are strings — error
				return fmt.Errorf("first arg must be a session ID or omit it to use latest session")
			}
		} else {
			// Single arg = label, use latest session for current project
			label = args[0]

			cwd, _ := os.Getwd()
			absDir, _ := filepath.Abs(cwd)
			p, _ := database.GetProjectByPath(absDir)
			if p == nil {
				return fmt.Errorf("not inside a tracked project — specify a session ID")
			}

			latest, _ := database.GetLatestSession(p.ID)
			if latest == nil {
				return fmt.Errorf("no sessions found for %s", p.Name)
			}
			sessionID = latest.ID
		}

		if tagRemove != "" {
			database.RemoveSessionTag(sessionID, tagRemove)
			fmt.Printf("✓ Removed tag \"%s\" from session #%d\n", tagRemove, sessionID)
			return nil
		}

		if err := database.AddSessionTag(sessionID, label); err != nil {
			return err
		}
		fmt.Printf("✓ Tagged session #%d with \"%s\"\n", sessionID, label)
		return nil
	},
}

func init() {
	tagCmd.Flags().StringVar(&tagRemove, "remove", "", "Remove a tag instead of adding")
	tagCmd.GroupID = "maintenance"
	rootCmd.AddCommand(tagCmd)
}
```

- [ ] **Step 3: Add --tag flag to sessions command**

In `cmd/sessions.go`, add a new flag and filter logic:

Add variable: `var sessionsTag string`

Add flag in `init()`: `sessionsCmd.Flags().StringVar(&sessionsTag, "tag", "", "Filter by user tag")`

Add filtering in RunE, after the existing filter logic:

```go
if sessionsTag != "" {
    taggedSessions, err := database.ListSessionsByTag(sessionsTag)
    if err != nil {
        return err
    }
    display.FormatSessionList(os.Stdout, taggedSessions)
    return nil
}
```

- [ ] **Step 4: Build and test**

Run:
```bash
cd ~/projects-wsl/nexus
go build -o nexus .
./nexus link nexus
./nexus tag "test-tag"
./nexus sessions --tag "test-tag"
```

- [ ] **Step 5: Commit**

```bash
git add cmd/link.go cmd/tag.go cmd/sessions.go
git commit -m "feat: add link, tag commands and sessions --tag filter"
```

---

### Task 10: Update Command Descriptions for Help

**Files:**
- Modify: All `cmd/*.go` files

- [ ] **Step 1: Update Short descriptions on all existing commands**

Update each command's `Short` field to be clear and concise. Update `Long` descriptions for commands with flags. This is a text-only change across all command files.

Key updates:
- `init`: "Initialize Nexus (~/.nexus/ setup, first scan)"
- `scan`: "Scan roots for projects and update health data"
- `capture`: "Capture a Claude session (called by shell wrapper)" (hidden)
- `projects`: "List all tracked projects with health status"
- `sessions`: "List Claude session history with filtering"
- `search`: "Search sessions and notes by keyword"
- `show`: "Show detailed info for a specific project"
- `note`: "Add a note to the current project"
- `config`: "Manage Nexus configuration (roots, exclusions)"

- [ ] **Step 2: Build and verify help output**

Run:
```bash
cd ~/projects-wsl/nexus
go build -o nexus .
./nexus --help
```
Expected: Commands organized in 4 groups with clear descriptions

- [ ] **Step 3: Commit**

```bash
git add cmd/
git commit -m "feat: improve command descriptions and help output"
```

---

### Task 11: Full Test Suite + Install

- [ ] **Step 1: Run full test suite**

Run: `cd ~/projects-wsl/nexus && go test ./... -v -count=1`
Expected: All tests PASS

- [ ] **Step 2: Run go vet**

Run: `cd ~/projects-wsl/nexus && go vet ./...`
Expected: No issues

- [ ] **Step 3: Install binary**

Run: `cd ~/projects-wsl/nexus && go install .`
Expected: Binary at ~/go/bin/nexus

- [ ] **Step 4: Test all new commands end-to-end**

Run:
```bash
nexus resume
nexus diff --since 30d
nexus stale
nexus deps --project nexus
nexus report
nexus context nexus
nexus streak
nexus where "scanner"
nexus link nexus
nexus tag "v2-complete"
nexus sessions --tag "v2-complete"
nexus --help
```

- [ ] **Step 5: Commit any fixes**

```bash
git add -A
git commit -m "chore: v2 final test pass and cleanup"
```
