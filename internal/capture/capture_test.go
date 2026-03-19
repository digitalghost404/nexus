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
