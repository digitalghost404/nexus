package db

import (
	"testing"
	"time"
)

func TestPruneSessions(t *testing.T) {
	d := testDB(t)

	oldDate := time.Now().AddDate(0, 0, -400).UTC()
	recentDate := time.Now().AddDate(0, 0, -10).UTC()

	_, err := d.db.Exec("INSERT INTO projects (name, path, status, discovered_at) VALUES ('test', '/test', 'active', ?)", time.Now().UTC())
	if err != nil {
		t.Fatalf("insert project: %v", err)
	}

	_, err = d.db.Exec("INSERT INTO sessions (project_id, summary, source, created_at) VALUES (1, 'old session', 'cli', ?)", oldDate)
	if err != nil {
		t.Fatalf("insert old session: %v", err)
	}
	_, err = d.db.Exec("INSERT INTO sessions (project_id, summary, source, created_at) VALUES (1, 'recent session', 'cli', ?)", recentDate)
	if err != nil {
		t.Fatalf("insert recent session: %v", err)
	}

	before := time.Now().AddDate(0, 0, -365)
	deleted, err := d.PruneSessions(before)
	if err != nil {
		t.Fatalf("prune sessions: %v", err)
	}
	if deleted != 1 {
		t.Errorf("expected 1 deleted, got %d", deleted)
	}

	var count int
	if err := d.db.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&count); err != nil {
		t.Fatalf("count sessions: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 session remaining, got %d", count)
	}
}

func TestPruneNotes(t *testing.T) {
	d := testDB(t)

	oldDate := time.Now().AddDate(0, 0, -400).UTC()
	recentDate := time.Now().AddDate(0, 0, -10).UTC()

	_, err := d.db.Exec("INSERT INTO projects (name, path, status, discovered_at) VALUES ('test', '/test', 'active', ?)", time.Now().UTC())
	if err != nil {
		t.Fatalf("insert project: %v", err)
	}

	_, err = d.db.Exec("INSERT INTO notes (project_id, content, created_at) VALUES (1, 'old note', ?)", oldDate)
	if err != nil {
		t.Fatalf("insert old note: %v", err)
	}
	_, err = d.db.Exec("INSERT INTO notes (project_id, content, created_at) VALUES (1, 'recent note', ?)", recentDate)
	if err != nil {
		t.Fatalf("insert recent note: %v", err)
	}

	before := time.Now().AddDate(0, 0, -365)
	deleted, err := d.PruneNotes(before)
	if err != nil {
		t.Fatalf("prune notes: %v", err)
	}
	if deleted != 1 {
		t.Errorf("expected 1 deleted, got %d", deleted)
	}

	var count int
	if err := d.db.QueryRow("SELECT COUNT(*) FROM notes").Scan(&count); err != nil {
		t.Fatalf("count notes: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 note remaining, got %d", count)
	}
}

func TestPrunePreferences(t *testing.T) {
	d := testDB(t)

	oldDate := time.Now().AddDate(0, 0, -400).UTC()
	recentDate := time.Now().AddDate(0, 0, -10).UTC()

	_, err := d.db.Exec("INSERT INTO projects (name, path, status, discovered_at) VALUES ('test', '/test', 'active', ?)", time.Now().UTC())
	if err != nil {
		t.Fatalf("insert project: %v", err)
	}

	_, err = d.db.Exec("INSERT INTO preferences (project_id, category, content, source, created_at) VALUES (1, 'workflow', 'old pref', 'stated', ?)", oldDate)
	if err != nil {
		t.Fatalf("insert old pref: %v", err)
	}
	_, err = d.db.Exec("INSERT INTO preferences (project_id, category, content, source, created_at) VALUES (1, 'workflow', 'recent pref', 'stated', ?)", recentDate)
	if err != nil {
		t.Fatalf("insert recent pref: %v", err)
	}

	before := time.Now().AddDate(0, 0, -365)
	deleted, err := d.PrunePreferences(before)
	if err != nil {
		t.Fatalf("prune preferences: %v", err)
	}
	if deleted != 1 {
		t.Errorf("expected 1 deleted, got %d", deleted)
	}

	var count int
	if err := d.db.QueryRow("SELECT COUNT(*) FROM preferences").Scan(&count); err != nil {
		t.Fatalf("count preferences: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 preference remaining, got %d", count)
	}
}
