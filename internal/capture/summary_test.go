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
