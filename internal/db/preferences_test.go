package db

import (
	"testing"
	"time"
)

func TestInsertPreference(t *testing.T) {
	d := testDB(t)

	p := Preference{
		Category:   "workflow",
		Content:    "Always run tests before push",
		Source:     "stated",
		Confidence: 1.0,
	}

	id, err := d.InsertPreference(p)
	if err != nil {
		t.Fatalf("InsertPreference: %v", err)
	}
	if id == 0 {
		t.Error("expected non-zero id")
	}
}

func TestInsertPreferenceWithProject(t *testing.T) {
	d := testDB(t)

	proj := Project{Name: "test-project", Path: "/tmp/test", Status: "active"}
	projID, _ := d.UpsertProject(proj)

	p := Preference{
		ProjectID:  &projID,
		Category:   "workflow",
		Content:    "Use conventional commits",
		Source:     "observed",
		Confidence: 0.7,
	}

	id, err := d.InsertPreference(p)
	if err != nil {
		t.Fatalf("InsertPreference: %v", err)
	}

	fetched, err := d.GetPreference(id)
	if err != nil {
		t.Fatalf("GetPreference: %v", err)
	}
	if *fetched.ProjectID != projID {
		t.Errorf("expected project_id %d, got %d", projID, *fetched.ProjectID)
	}
}

func TestSearchPreferences(t *testing.T) {
	d := testDB(t)

	d.InsertPreference(Preference{Category: "style", Content: "Prefer terse responses", Source: "stated", Confidence: 1.0})
	d.InsertPreference(Preference{Category: "tool", Content: "Use ollama for embeddings", Source: "stated", Confidence: 1.0})
	d.InsertPreference(Preference{Category: "workflow", Content: "Always run clippy", Source: "observed", Confidence: 0.7})

	results, err := d.SearchPreferences("embeddings")
	if err != nil {
		t.Fatalf("SearchPreferences: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestListPreferencesByProject(t *testing.T) {
	d := testDB(t)

	proj := Project{Name: "axon", Path: "/tmp/axon", Status: "active"}
	projID, _ := d.UpsertProject(proj)

	d.InsertPreference(Preference{ProjectID: &projID, Category: "workflow", Content: "Run clippy first", Source: "stated", Confidence: 1.0})
	d.InsertPreference(Preference{Category: "style", Content: "Terse responses", Source: "stated", Confidence: 1.0})

	projPrefs, err := d.ListPreferencesByProject(&projID)
	if err != nil {
		t.Fatalf("ListPreferencesByProject(projID): %v", err)
	}
	globalPrefs, err := d.ListPreferencesByProject(nil)
	if err != nil {
		t.Fatalf("ListPreferencesByProject(nil): %v", err)
	}

	if len(projPrefs) != 2 {
		t.Errorf("expected 2 project-including prefs, got %d", len(projPrefs))
	}
	if len(globalPrefs) != 1 {
		t.Errorf("expected 1 global pref, got %d", len(globalPrefs))
	}
}

func TestDecayPreference(t *testing.T) {
	d := testDB(t)

	p := Preference{
		Category:         "pattern",
		Content:          "Works evenings",
		Source:           "inferred",
		Confidence:       0.4,
		CreatedAt:        time.Now().Add(-60 * 24 * time.Hour),
		UpdatedAt:        time.Now().Add(-60 * 24 * time.Hour),
		LastReferencedAt: ptrTime(time.Now().Add(-60 * 24 * time.Hour)),
	}

	id, _ := d.InsertPreference(p)

	err := d.DecayPreferences()
	if err != nil {
		t.Fatalf("DecayPreferences: %v", err)
	}

	fetched, _ := d.GetPreference(id)
	if fetched.Confidence >= 0.4 {
		t.Errorf("expected confidence to decay below 0.4, got %f", fetched.Confidence)
	}
}

func TestDeleteLowConfidencePreferences(t *testing.T) {
	d := testDB(t)

	d.InsertPreference(Preference{Category: "pattern", Content: "Low confidence", Source: "inferred", Confidence: 0.1})
	d.InsertPreference(Preference{Category: "workflow", Content: "High confidence", Source: "stated", Confidence: 1.0})

	deleted, err := d.DeleteLowConfidencePreferences(0.15)
	if err != nil {
		t.Fatalf("DeleteLowConfidencePreferences: %v", err)
	}
	if deleted != 1 {
		t.Errorf("expected 1 deleted, got %d", deleted)
	}
}

func TestDeduplicatePreference(t *testing.T) {
	d := testDB(t)

	id1, _ := d.InsertPreference(Preference{Category: "workflow", Content: "Run tests before push", Source: "stated", Confidence: 1.0})
	id2, err := d.InsertPreference(Preference{Category: "workflow", Content: "Run tests before push", Source: "observed", Confidence: 0.7})

	if err != nil {
		t.Fatalf("second insert should succeed by updating: %v", err)
	}
	if id1 != id2 {
		t.Errorf("expected same id for duplicate, got %d and %d", id1, id2)
	}
}

func TestSupersedePreference(t *testing.T) {
	d := testDB(t)

	id1, _ := d.InsertPreference(Preference{Category: "workflow", Content: "Use gofmt", Source: "stated", Confidence: 1.0})
	id2, _ := d.InsertPreference(Preference{Category: "workflow", Content: "Use gofmt and goimports", Source: "stated", Confidence: 1.0})

	err := d.SupersedePreference(id1, id2)
	if err != nil {
		t.Fatalf("SupersedePreference: %v", err)
	}

	old, _ := d.GetPreference(id1)
	if old.SupersededBy == nil || *old.SupersededBy != id2 {
		t.Error("expected old preference to be superseded by new one")
	}
}

func TestBumpPreferenceAccess(t *testing.T) {
	d := testDB(t)

	id, _ := d.InsertPreference(Preference{Category: "style", Content: "Terse", Source: "stated", Confidence: 1.0})

	err := d.BumpPreferenceAccess(id)
	if err != nil {
		t.Fatalf("BumpPreferenceAccess: %v", err)
	}

	fetched, _ := d.GetPreference(id)
	if fetched.AccessCount != 1 {
		t.Errorf("expected access_count 1, got %d", fetched.AccessCount)
	}
}

func TestDeleteContradictingInferredPreferences(t *testing.T) {
	d := testDB(t)

	d.InsertPreference(Preference{Category: "workflow", Content: "Use tabs", Source: "inferred", Confidence: 0.4})
	d.InsertPreference(Preference{Category: "workflow", Content: "Use spaces", Source: "stated", Confidence: 1.0})

	deleted, err := d.DeleteContradictingInferredPreferences()
	if err != nil {
		t.Fatalf("DeleteContradictingInferredPreferences: %v", err)
	}
	if deleted != 1 {
		t.Errorf("expected 1 inferred deleted, got %d", deleted)
	}
}

func TestDeleteSupersededPreferences(t *testing.T) {
	d := testDB(t)

	id1, _ := d.InsertPreference(Preference{Category: "workflow", Content: "Old way", Source: "stated", Confidence: 1.0})
	id2, _ := d.InsertPreference(Preference{Category: "workflow", Content: "New way", Source: "stated", Confidence: 1.0})

	d.SupersedePreference(id1, id2)

	deleted, err := d.DeleteSupersededPreferences(0)
	if err != nil {
		t.Fatalf("DeleteSupersededPreferences: %v", err)
	}
	if deleted != 1 {
		t.Errorf("expected 1 superseded deleted, got %d", deleted)
	}
}

func ptrTime(t time.Time) *time.Time {
	return &t
}
