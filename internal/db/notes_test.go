// internal/db/notes_test.go
package db

import (
	"testing"
	"time"
)

func TestInsertAndSearchNotes(t *testing.T) {
	d := testDB(t)

	now := time.Now()
	pID, _ := d.UpsertProject(Project{Name: "proj", Path: "/a", Status: "active", DiscoveredAt: now})

	_, err := d.InsertNote(Note{ProjectID: &pID, Content: "migrated auth to JWT"})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	_, err = d.InsertNote(Note{ProjectID: &pID, Content: "refactored database layer"})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	results, err := d.SearchNotes("JWT")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestGlobalNote(t *testing.T) {
	d := testDB(t)

	_, err := d.InsertNote(Note{Content: "general thought"})
	if err != nil {
		t.Fatalf("insert global note: %v", err)
	}

	notes, err := d.ListNotes(0, 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(notes) != 1 {
		t.Errorf("expected 1 note, got %d", len(notes))
	}
}
