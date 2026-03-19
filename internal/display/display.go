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
