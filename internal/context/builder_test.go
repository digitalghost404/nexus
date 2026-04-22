package context

import (
	"strings"
	"testing"

	"github.com/digitalghost404/nexus/internal/db"
)

func TestBuildContextProjectState(t *testing.T) {
	project := &db.Project{
		Name:          "axon",
		Branch:        "main",
		Status:        "active",
		LastCommitMsg: "fix: shader pipeline",
	}

	sessions := []db.Session{
		{Summary: "Implemented shader pipeline refactor"},
	}

	output := FormatProjectState(project, sessions, nil)
	if !strings.Contains(output, "## Project: axon") {
		t.Errorf("expected project header, got:\n%s", output)
	}
	if !strings.Contains(output, "main") {
		t.Errorf("expected branch info, got:\n%s", output)
	}
}

func TestBuildContextWithPreferences(t *testing.T) {
	prefs := []db.Preference{
		{Category: "workflow", Content: "Always run clippy before push", Source: "stated", Confidence: 1.0},
		{Category: "style", Content: "Terse responses", Source: "stated", Confidence: 1.0},
	}

	output := FormatPreferences(prefs)
	if !strings.Contains(output, "### Preferences") {
		t.Errorf("expected preferences header, got:\n%s", output)
	}
	if !strings.Contains(output, "Always run clippy") {
		t.Errorf("expected preference content, got:\n%s", output)
	}
}

func TestBuildContextWithRecall(t *testing.T) {
	results := []RecallResult{
		{SourceType: "session", SourceID: 1, Content: "Auth system design — decided on JWT", Score: 0.92},
		{SourceType: "note", SourceID: 5, Content: "Use ollama for local embeddings", Score: 0.85},
	}

	output := FormatRecall(results)
	if !strings.Contains(output, "### Recall: Related Context") {
		t.Errorf("expected recall header, got:\n%s", output)
	}
	if !strings.Contains(output, "Auth system design") {
		t.Errorf("expected recall content, got:\n%s", output)
	}
}

func TestBuildContextFull(t *testing.T) {
	project := &db.Project{
		Name:   "axon",
		Branch: "main",
		Status: "active",
	}

	sessions := []db.Session{
		{Summary: "Implemented shader pipeline refactor"},
	}

	recallResults := []RecallResult{
		{SourceType: "session", SourceID: 1, Content: "Auth system design", Score: 0.92},
	}

	prefs := []db.Preference{
		{Category: "workflow", Content: "Always run clippy before push", Source: "stated", Confidence: 1.0},
	}

	output := BuildContext(ContextOptions{
		Project:        project,
		RecentSessions: sessions,
		RecallResults:  recallResults,
		Preferences:    prefs,
	})

	if !strings.Contains(output, "## Project: axon") {
		t.Errorf("expected project header, got:\n%s", output)
	}
	if !strings.Contains(output, "### Recall: Related Context") {
		t.Errorf("expected recall section, got:\n%s", output)
	}
	if !strings.Contains(output, "### Preferences") {
		t.Errorf("expected preferences section, got:\n%s", output)
	}
}

func TestBuildContextOllamaNotAvailable(t *testing.T) {
	project := &db.Project{
		Name:   "axon",
		Branch: "main",
		Status: "active",
	}

	output := BuildContext(ContextOptions{
		Project:         project,
		RecentSessions:  nil,
		RecallResults:   nil,
		Preferences:     nil,
		OllamaAvailable: false,
	})

	if !strings.Contains(output, "semantic recall skipped") {
		t.Errorf("expected ollama not running message, got:\n%s", output)
	}
}

func TestBuildContextOllamaAvailableButNoResults(t *testing.T) {
	project := &db.Project{
		Name:   "axon",
		Branch: "main",
		Status: "active",
	}

	output := BuildContext(ContextOptions{
		Project:         project,
		RecentSessions:  nil,
		RecallResults:   nil,
		Preferences:     nil,
		OllamaAvailable: true,
	})

	if strings.Contains(output, "semantic recall skipped") {
		t.Errorf("should not show skip message when ollama is available, got:\n%s", output)
	}
}

func TestFormatPreferencesSkipsLowConfidence(t *testing.T) {
	prefs := []db.Preference{
		{Category: "workflow", Content: "High confidence pref", Source: "stated", Confidence: 1.0},
		{Category: "style", Content: "Low confidence pref", Source: "inferred", Confidence: 0.2},
	}

	output := FormatPreferences(prefs)
	if !strings.Contains(output, "High confidence pref") {
		t.Errorf("expected high confidence preference, got:\n%s", output)
	}
	if strings.Contains(output, "Low confidence pref") {
		t.Errorf("should skip low confidence preference, got:\n%s", output)
	}
}

func TestFormatPreferencesSourceTag(t *testing.T) {
	prefs := []db.Preference{
		{Category: "workflow", Content: "Stated preference", Source: "stated", Confidence: 1.0},
		{Category: "style", Content: "Observed preference", Source: "observed", Confidence: 0.7},
	}

	output := FormatPreferences(prefs)
	if strings.Contains(output, "Stated preference (stated") {
		t.Errorf("stated preferences should not have source tag, got:\n%s", output)
	}
	if !strings.Contains(output, "Observed preference (observed") {
		t.Errorf("non-stated preferences should have source tag, got:\n%s", output)
	}
}

func TestFormatRecallLabels(t *testing.T) {
	results := []RecallResult{
		{SourceType: "session", SourceID: 1, Content: "session content", Score: 0.9},
		{SourceType: "note", SourceID: 2, Content: "note content", Score: 0.8},
		{SourceType: "preference", SourceID: 3, Content: "pref content", Score: 0.7},
	}

	output := FormatRecall(results)
	if !strings.Contains(output, "[session]") {
		t.Errorf("expected [session] label, got:\n%s", output)
	}
	if !strings.Contains(output, "[note]") {
		t.Errorf("expected [note] label, got:\n%s", output)
	}
	if !strings.Contains(output, "[pref]") {
		t.Errorf("expected [pref] label, got:\n%s", output)
	}
}

func TestFormatRecallWithDate(t *testing.T) {
	results := []RecallResult{
		{SourceType: "session", SourceID: 1, Content: "content", Score: 0.9, Date: "2026-04-20"},
	}

	output := FormatRecall(results)
	if !strings.Contains(output, "2026-04-20:") {
		t.Errorf("expected date in recall output, got:\n%s", output)
	}
}

func TestFormatProjectStateWithNilProject(t *testing.T) {
	output := FormatProjectState(nil, nil, nil)
	if output == "" {
		t.Error("expected non-empty output for nil project")
	}
}

func TestBuildContextJoinsWithDoubleNewline(t *testing.T) {
	project := &db.Project{
		Name:   "axon",
		Branch: "main",
		Status: "active",
	}

	prefs := []db.Preference{
		{Category: "workflow", Content: "Test pref", Source: "stated", Confidence: 1.0},
	}

	output := BuildContext(ContextOptions{
		Project:     project,
		Preferences: prefs,
	})

	if !strings.Contains(output, "\n\n") {
		t.Errorf("expected sections joined with double newline, got:\n%s", output)
	}
}
