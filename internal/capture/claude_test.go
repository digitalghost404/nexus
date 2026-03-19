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
