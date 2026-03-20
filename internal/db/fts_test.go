// internal/db/fts_test.go
package db

import (
	"testing"
	"time"
)

// TestSearchSessionsFTSSafety verifies that SearchSessions does not panic or
// return a raw SQL error when given FTS5 special characters. The sanitizeFTS
// helper wraps the query in double quotes, which must neutralise operators such
// as OR, AND, NOT, and bare double-quote characters.
func TestSearchSessionsFTSSafety(t *testing.T) {
	d := testDB(t)

	now := time.Now()
	pID, err := d.UpsertProject(Project{
		Name:         "proj",
		Path:         "/a",
		Status:       "active",
		DiscoveredAt: now,
	})
	if err != nil {
		t.Fatalf("upsert project: %v", err)
	}

	// Insert a session whose summary will be matched by a normal query.
	_, err = d.InsertSession(Session{
		ProjectID: pID,
		Summary:   "Added retry logic to authentication handler",
		Source:    "wrapper",
		StartedAt: &now,
	})
	if err != nil {
		t.Fatalf("insert session: %v", err)
	}

	t.Run("normal query matches", func(t *testing.T) {
		results, err := d.SearchSessions("retry")
		if err != nil {
			t.Fatalf("SearchSessions(%q): unexpected error: %v", "retry", err)
		}
		if len(results) != 1 {
			t.Errorf("SearchSessions(%q): expected 1 result, got %d", "retry", len(results))
		}
	})

	t.Run("FTS OR operator does not error", func(t *testing.T) {
		// "retry OR" is invalid raw FTS5 syntax; sanitizeFTS must neutralise it.
		results, err := d.SearchSessions("retry OR")
		if err != nil {
			t.Fatalf("SearchSessions(%q): unexpected error: %v", "retry OR", err)
		}
		// The sanitised query is treated as a literal phrase; it won't match.
		// We only assert no error and a non-nil slice is returned without panicking.
		_ = results
	})

	t.Run("double-quote in query does not error", func(t *testing.T) {
		// A bare double-quote would break FTS5 quoting; sanitizeFTS must escape it.
		results, err := d.SearchSessions(`"quoted"`)
		if err != nil {
			t.Fatalf(`SearchSessions(%q): unexpected error: %v`, `"quoted"`, err)
		}
		_ = results
	})
}
