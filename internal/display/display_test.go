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
