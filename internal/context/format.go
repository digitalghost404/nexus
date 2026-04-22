package context

import (
	"fmt"
	"strings"
	"time"

	"github.com/digitalghost404/nexus/internal/db"
)

func FormatProjectState(project *db.Project, sessions []db.Session) string {
	var b strings.Builder

	b.WriteString(formatProjectHeader(project))

	if len(sessions) > 0 {
		b.WriteString("\n### Recent Activity\n")
		for _, s := range sessions {
			if s.StartedAt != nil {
				b.WriteString(fmt.Sprintf("- %s: %s\n", s.StartedAt.Format("2006-01-02"), s.Summary))
			} else {
				b.WriteString(fmt.Sprintf("- %s\n", s.Summary))
			}
		}
	}

	return b.String()
}

var sourceLabels = map[string]string{
	"session":    "session",
	"note":       "note",
	"preference": "pref",
}

func FormatRecall(results []RecallResult) string {
	if len(results) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("### Recall: Related Context\n")

	for _, r := range results {
		label := sourceLabels[r.SourceType]
		if label == "" {
			label = r.SourceType
		}

		dateStr := ""
		if r.Date != nil {
			dateStr = r.Date.Format("2006-01-02") + ": "
		}

		b.WriteString(fmt.Sprintf("- [%s] %s%s\n", label, dateStr, r.Content))
	}

	return b.String()
}

func FormatPreferences(prefs []db.Preference) string {
	var filtered []db.Preference
	for _, p := range prefs {
		if p.Confidence >= 0.3 {
			filtered = append(filtered, p)
		}
	}
	if len(filtered) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("### Preferences\n")

	for _, p := range filtered {
		sourceTag := ""
		if p.Source != "stated" {
			sourceTag = fmt.Sprintf(" (%s, %.0f%%)", p.Source, p.Confidence*100)
		}
		b.WriteString(fmt.Sprintf("- [%s] %s%s\n", p.Category, p.Content, sourceTag))
	}

	return b.String()
}

func formatProjectHeader(project *db.Project) string {
	if project == nil {
		return "## Project: unknown\n**Branch**: unknown | **Status**: unknown\n"
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("## Project: %s\n", project.Name))
	b.WriteString(fmt.Sprintf("**Branch**: %s | **Status**: %s", project.Branch, project.Status))

	if !project.LastCommitAt.Valid {
		b.WriteString(" | **Last commit**: none\n")
	} else {
		daysSince := int(time.Since(project.LastCommitAt.Time).Hours() / 24)
		b.WriteString(fmt.Sprintf(" | **Last commit**: %s (%d days ago)\n", project.LastCommitMsg, daysSince))
	}

	return b.String()
}
