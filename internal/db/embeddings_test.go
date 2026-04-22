package db

import (
	"testing"
	"time"
)

func TestStoreAndGetEmbedding(t *testing.T) {
	d := testDB(t)

	s := Session{ProjectID: 1, Summary: "test summary", Source: "test"}
	sID, err := d.InsertSession(s)
	if err != nil {
		t.Fatalf("InsertSession: %v", err)
	}

	vec := []float64{0.1, 0.2, 0.3, 0.4}
	err = d.StoreEmbedding("session", sID, "test content", vec, "nomic-embed-text", false)
	if err != nil {
		t.Fatalf("StoreEmbedding: %v", err)
	}
}

func TestCosineSimilarity(t *testing.T) {
	vec1 := []float64{1.0, 0.0, 0.0}
	vec2 := []float64{0.0, 1.0, 0.0}
	vec3 := []float64{1.0, 0.0, 0.0}

	sim12 := CosineSimilarity(vec1, vec2)
	if sim12 != 0.0 {
		t.Errorf("expected 0.0 for orthogonal vectors, got %f", sim12)
	}

	sim13 := CosineSimilarity(vec1, vec3)
	if sim13 != 1.0 {
		t.Errorf("expected 1.0 for identical vectors, got %f", sim13)
	}
}

func TestCosineSimilarityMismatchedLengths(t *testing.T) {
	a := []float64{1.0, 0.0}
	b := []float64{1.0, 0.0, 0.0}
	if CosineSimilarity(a, b) != 0 {
		t.Error("expected 0 for mismatched lengths")
	}
}

func TestCosineSimilarityZeroVector(t *testing.T) {
	zero := []float64{0.0, 0.0, 0.0}
	nonzero := []float64{1.0, 0.0, 0.0}
	if CosineSimilarity(zero, nonzero) != 0 {
		t.Error("expected 0 when one vector is zero")
	}
	if CosineSimilarity(zero, zero) != 0 {
		t.Error("expected 0 when both vectors are zero")
	}
}

func TestSearchSimilar(t *testing.T) {
	d := testDB(t)

	now := time.Now()
	proj := Project{Name: "test", Path: "/tmp/test", Status: "active", DiscoveredAt: NullTime{Time: now, Valid: true}}
	projID, err := d.UpsertProject(proj)
	if err != nil {
		t.Fatalf("UpsertProject: %v", err)
	}

	s1 := Session{ProjectID: projID, Summary: "auth system design", Source: "test"}
	s2 := Session{ProjectID: projID, Summary: "database migration", Source: "test"}
	s1ID, err := d.InsertSession(s1)
	if err != nil {
		t.Fatalf("InsertSession s1: %v", err)
	}
	s2ID, err := d.InsertSession(s2)
	if err != nil {
		t.Fatalf("InsertSession s2: %v", err)
	}

	vec1 := []float64{1.0, 0.0, 0.0}
	vec2 := []float64{0.0, 1.0, 0.0}

	if err := d.StoreEmbedding("session", s1ID, "auth system design", vec1, "nomic-embed-text", false); err != nil {
		t.Fatalf("StoreEmbedding s1: %v", err)
	}
	if err := d.StoreEmbedding("session", s2ID, "database migration", vec2, "nomic-embed-text", false); err != nil {
		t.Fatalf("StoreEmbedding s2: %v", err)
	}

	results, err := d.SearchSimilar([]float64{0.9, 0.1, 0.0}, "session", 5, 0.0)
	if err != nil {
		t.Fatalf("SearchSimilar: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least 1 result")
	}
	if results[0].SourceID != s1ID {
		t.Errorf("expected first result to be session %d, got %d", s1ID, results[0].SourceID)
	}
	if len(results) < 2 || results[0].Score <= results[1].Score {
		t.Errorf("expected results sorted by score desc: %f <= %f", results[0].Score, results[1].Score)
	}
}

func TestSearchSimilarWithMinScore(t *testing.T) {
	d := testDB(t)

	now := time.Now()
	proj := Project{Name: "test", Path: "/tmp/test", Status: "active", DiscoveredAt: NullTime{Time: now, Valid: true}}
	projID, _ := d.UpsertProject(proj)

	s := Session{ProjectID: projID, Summary: "test", Source: "test"}
	sID, _ := d.InsertSession(s)

	vec := []float64{0.0, 1.0, 0.0}
	d.StoreEmbedding("session", sID, "test", vec, "nomic-embed-text", false)

	queryVec := []float64{1.0, 0.0, 0.0}
	results, err := d.SearchSimilar(queryVec, "session", 5, 0.99)
	if err != nil {
		t.Fatalf("SearchSimilar: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results with high minScore, got %d", len(results))
	}
}

func TestSearchSimilarLimit(t *testing.T) {
	d := testDB(t)

	now := time.Now()
	proj := Project{Name: "test", Path: "/tmp/test", Status: "active", DiscoveredAt: NullTime{Time: now, Valid: true}}
	projID, _ := d.UpsertProject(proj)

	for i := 0; i < 5; i++ {
		s := Session{ProjectID: projID, Summary: "test", Source: "test"}
		sID, _ := d.InsertSession(s)
		vec := []float64{0.2, 0.2, 0.2}
		d.StoreEmbedding("session", sID, "test", vec, "nomic-embed-text", false)
	}

	results, err := d.SearchSimilar([]float64{0.2, 0.2, 0.2}, "session", 2, 0.0)
	if err != nil {
		t.Fatalf("SearchSimilar: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results with limit=2, got %d", len(results))
	}
}

func TestSearchSimilarEmptyTable(t *testing.T) {
	d := testDB(t)

	results, err := d.SearchSimilar([]float64{0.1, 0.2, 0.3}, "session", 5, 0.0)
	if err != nil {
		t.Fatalf("SearchSimilar: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results from empty table, got %d", len(results))
	}
}

func TestSearchSimilarSourceTypeFilter(t *testing.T) {
	d := testDB(t)

	now := time.Now()
	proj := Project{Name: "test", Path: "/tmp/test", Status: "active", DiscoveredAt: NullTime{Time: now, Valid: true}}
	projID, _ := d.UpsertProject(proj)

	s := Session{ProjectID: projID, Summary: "session test", Source: "test"}
	sID, _ := d.InsertSession(s)

	n := Note{ProjectID: &projID, Content: "note test"}
	nID, err := d.InsertNote(n)
	if err != nil {
		t.Fatalf("InsertNote: %v", err)
	}

	vec := []float64{0.5, 0.5, 0.0}
	d.StoreEmbedding("session", sID, "session test", vec, "nomic-embed-text", false)
	d.StoreEmbedding("note", nID, "note test", vec, "nomic-embed-text", false)

	sessionResults, _ := d.SearchSimilar(vec, "session", 5, 0.0)
	noteResults, _ := d.SearchSimilar(vec, "note", 5, 0.0)

	if len(sessionResults) != 1 {
		t.Errorf("expected 1 session result, got %d", len(sessionResults))
	}
	if len(noteResults) != 1 {
		t.Errorf("expected 1 note result, got %d", len(noteResults))
	}
}
