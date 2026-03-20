// internal/display/display.go
package display

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/digitalghost404/nexus/internal/db"
)

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
	Name      string
	Current   string
	Available string
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

type StaleBranchInfo struct {
	Name string
	Age  string
}

func FormatSmartSummary(w io.Writer, dirty []db.Project, sessions []db.Session, stale []db.Project, currentProject string) {
	fmt.Fprintf(w, "\n┌ NEXUS ─────────────────────────────────────\n│\n")

	// Show current project context if inside one
	if currentProject != "" {
		fmt.Fprintf(w, "│  📍 Current project: %s\n│\n", currentProject)
	}

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

func FormatProjectDetail(w io.Writer, p *db.Project, sessions []db.Session, staleBranches []string, linkedProjects []db.Project) {
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

	if len(linkedProjects) > 0 {
		fmt.Fprintf(w, "│\n│  Linked Projects\n")
		for _, lp := range linkedProjects {
			fmt.Fprintf(w, "│  %s  %s\n", lp.Name, lp.Path)
		}
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
	case d < 0:
		return "just now"
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
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-3]) + "..."
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
	json.Unmarshal([]byte(s.CommitsMade), &commits) //nolint:errcheck
	if len(commits) > 0 {
		fmt.Fprintf(w, "│\n│  Commits:\n")
		for _, c := range commits {
			fmt.Fprintf(w, "│    %s  %s\n", c.Hash, c.Message)
		}
	}

	// Parse and show files
	var files []string
	json.Unmarshal([]byte(s.FilesChanged), &files) //nolint:errcheck
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

func FormatDiff(w io.Writer, projectName string, since string, sessions []db.Session) {
	fmt.Fprintf(w, "\n┌ DIFF: %s (last %s) ────────────────\n│\n", projectName, since)

	// Aggregate stats
	totalCommits := 0
	fileSet := map[string]int{}
	for _, s := range sessions {
		var commits []struct{ Hash, Message string }
		json.Unmarshal([]byte(s.CommitsMade), &commits) //nolint:errcheck
		totalCommits += len(commits)

		var files []string
		json.Unmarshal([]byte(s.FilesChanged), &files) //nolint:errcheck
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
		json.Unmarshal([]byte(p.Languages), &langs) //nolint:errcheck
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
			json.Unmarshal([]byte(s.CommitsMade), &commits) //nolint:errcheck
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

func FormatStale(w io.Writer, branches map[string][]StaleBranchInfo, dirtyDetails map[string][]string) {
	if len(branches) == 0 && len(dirtyDetails) == 0 {
		fmt.Fprintln(w, "Nothing stale or dirty.")
		return
	}

	if len(branches) > 0 {
		fmt.Fprintf(w, "\nStale branches:\n\n")
		for project, brs := range branches {
			fmt.Fprintf(w, "  %s\n", project)
			for _, b := range brs {
				fmt.Fprintf(w, "    %s  (last commit: %s)\n", b.Name, b.Age)
			}
			fmt.Fprintln(w)
		}
	}

	if len(dirtyDetails) > 0 {
		fmt.Fprintf(w, "Dirty projects (uncommitted changes):\n\n")
		for project, files := range dirtyDetails {
			fmt.Fprintf(w, "  %s\n", project)
			for _, f := range files {
				fmt.Fprintf(w, "    %s\n", f)
			}
			fmt.Fprintln(w)
		}
	}
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
