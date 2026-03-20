// internal/db/sessions_test.go
package db

import (
	"testing"
	"time"
)

func TestInsertAndListSessions(t *testing.T) {
	d := testDB(t)

	now := time.Now()
	pID, _ := d.UpsertProject(Project{Name: "proj", Path: "/a", Status: "active", DiscoveredAt: now})

	s := Session{
		ProjectID: pID,
		StartedAt: &now,
		Summary:   "Added retry logic to DNS scanner",
		Source:    "wrapper",
		Tags:      `["go","security"]`,
	}

	id, err := d.InsertSession(s)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if id == 0 {
		t.Fatal("expected non-zero id")
	}

	sessions, err := d.ListSessions(SessionFilter{Limit: 10})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].Summary != "Added retry logic to DNS scanner" {
		t.Errorf("unexpected summary: %s", sessions[0].Summary)
	}
}

func TestListSessionsByProject(t *testing.T) {
	d := testDB(t)

	now := time.Now()
	p1, _ := d.UpsertProject(Project{Name: "proj1", Path: "/a", Status: "active", DiscoveredAt: now})
	p2, _ := d.UpsertProject(Project{Name: "proj2", Path: "/b", Status: "active", DiscoveredAt: now})

	d.InsertSession(Session{ProjectID: p1, Summary: "session 1", Source: "wrapper", StartedAt: &now})
	d.InsertSession(Session{ProjectID: p2, Summary: "session 2", Source: "wrapper", StartedAt: &now})
	d.InsertSession(Session{ProjectID: p1, Summary: "session 3", Source: "wrapper", StartedAt: &now})

	sessions, err := d.ListSessions(SessionFilter{ProjectID: p1, Limit: 10})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(sessions) != 2 {
		t.Errorf("expected 2 sessions for proj1, got %d", len(sessions))
	}
}

func TestSearchSessions(t *testing.T) {
	d := testDB(t)

	now := time.Now()
	pID, _ := d.UpsertProject(Project{Name: "proj", Path: "/a", Status: "active", DiscoveredAt: now})

	d.InsertSession(Session{ProjectID: pID, Summary: "Added retry logic to DNS scanner", Source: "wrapper", StartedAt: &now})
	d.InsertSession(Session{ProjectID: pID, Summary: "Fixed database migration bug", Source: "wrapper", StartedAt: &now})
	d.InsertSession(Session{ProjectID: pID, Summary: "Refactored HTTP client with retry", Source: "wrapper", StartedAt: &now})

	results, err := d.SearchSessions("retry")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results for 'retry', got %d", len(results))
	}
}

func TestSessionDedup(t *testing.T) {
	d := testDB(t)

	now := time.Now()
	pID, _ := d.UpsertProject(Project{Name: "proj", Path: "/a", Status: "active", DiscoveredAt: now})

	_, err := d.InsertSession(Session{
		ProjectID:       pID,
		ClaudeSessionID: "abc-123",
		Summary:         "first",
		Source:          "wrapper",
		StartedAt:       &now,
	})
	if err != nil {
		t.Fatalf("first insert: %v", err)
	}

	// Same claude_session_id should be detected as duplicate
	exists, err := d.SessionExists("abc-123")
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if !exists {
		t.Error("expected session to exist")
	}
}

func TestGetLatestSession(t *testing.T) {
	d := testDB(t)
	now := time.Now()
	pID, _ := d.UpsertProject(Project{Name: "proj", Path: "/a", Status: "active", DiscoveredAt: now})

	earlier := now.Add(-2 * time.Hour)
	d.InsertSession(Session{ProjectID: pID, Summary: "first", Source: "wrapper", StartedAt: &earlier})
	d.InsertSession(Session{ProjectID: pID, Summary: "latest", Source: "wrapper", StartedAt: &now})

	s, err := d.GetLatestSession(pID)
	if err != nil {
		t.Fatalf("get latest: %v", err)
	}
	if s == nil || s.Summary != "latest" {
		t.Errorf("expected 'latest', got %v", s)
	}
}

func TestGetLatestSessionEmpty(t *testing.T) {
	d := testDB(t)
	now := time.Now()
	pID, _ := d.UpsertProject(Project{Name: "proj", Path: "/a", Status: "active", DiscoveredAt: now})

	s, err := d.GetLatestSession(pID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s != nil {
		t.Error("expected nil for empty project")
	}
}

func TestGetSessionsInRange(t *testing.T) {
	d := testDB(t)
	now := time.Now()
	pID, _ := d.UpsertProject(Project{Name: "proj", Path: "/a", Status: "active", DiscoveredAt: now})

	old := now.Add(-10 * 24 * time.Hour)
	recent := now.Add(-1 * time.Hour)
	d.InsertSession(Session{ProjectID: pID, Summary: "old", Source: "wrapper", StartedAt: &old})
	d.InsertSession(Session{ProjectID: pID, Summary: "recent", Source: "wrapper", StartedAt: &recent})

	since := now.Add(-7 * 24 * time.Hour)
	sessions, err := d.GetSessionsInRange(pID, since, now)
	if err != nil {
		t.Fatalf("range: %v", err)
	}
	if len(sessions) != 1 || sessions[0].Summary != "recent" {
		t.Errorf("expected 1 recent session, got %d", len(sessions))
	}
}

func TestGetDistinctSessionDates(t *testing.T) {
	d := testDB(t)
	now := time.Now()
	pID, _ := d.UpsertProject(Project{Name: "proj", Path: "/a", Status: "active", DiscoveredAt: now})

	day1 := now.Add(-48 * time.Hour)
	day2 := now.Add(-24 * time.Hour)
	d.InsertSession(Session{ProjectID: pID, Summary: "s1", Source: "wrapper", StartedAt: &day1})
	d.InsertSession(Session{ProjectID: pID, Summary: "s2", Source: "wrapper", StartedAt: &day2})
	d.InsertSession(Session{ProjectID: pID, Summary: "s3", Source: "wrapper", StartedAt: &now})

	dates, err := d.GetDistinctSessionDates()
	if err != nil {
		t.Fatalf("dates: %v", err)
	}
	if len(dates) != 3 {
		t.Errorf("expected 3 distinct dates, got %d", len(dates))
	}
}
