package db

import (
	"testing"
	"time"
)

// insertTestSession is a helper that creates a minimal project + session and
// returns the session ID for use in conversation digest tests.
func insertTestSession(t *testing.T, d *DB) int64 {
	t.Helper()
	now := time.Now()
	pID, err := d.UpsertProject(Project{Name: "proj", Path: "/test", Status: "active", DiscoveredAt: now})
	if err != nil {
		t.Fatalf("upsert project: %v", err)
	}
	sID, err := d.InsertSession(Session{ProjectID: pID, Summary: "test session", Source: "wrapper", StartedAt: &now})
	if err != nil {
		t.Fatalf("insert session: %v", err)
	}
	return sID
}

func TestInsertAndGetConversationDigest(t *testing.T) {
	d := testDB(t)
	sID := insertTestSession(t, d)

	want := `{"messages":[{"role":"user","content":"hello"}]}`

	if err := d.InsertConversationDigest(sID, want); err != nil {
		t.Fatalf("InsertConversationDigest: %v", err)
	}

	got, err := d.GetConversationDigest(sID)
	if err != nil {
		t.Fatalf("GetConversationDigest: %v", err)
	}
	if got != want {
		t.Errorf("digest mismatch: got %q, want %q", got, want)
	}
}

func TestGetConversationDigestNotFound(t *testing.T) {
	d := testDB(t)

	got, err := d.GetConversationDigest(99999)
	if err != nil {
		t.Fatalf("expected no error for missing session, got: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty string for missing session, got: %q", got)
	}
}
