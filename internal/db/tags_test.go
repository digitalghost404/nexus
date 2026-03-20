// internal/db/tags_test.go
package db

import (
	"testing"
	"time"
)

func TestAddAndListSessionTags(t *testing.T) {
	d := testDB(t)
	now := time.Now()
	pID, _ := d.UpsertProject(Project{Name: "proj", Path: "/a", Status: "active", DiscoveredAt: now})
	sID, _ := d.InsertSession(Session{ProjectID: pID, Summary: "test", Source: "wrapper", StartedAt: &now})

	d.AddSessionTag(sID, "breakthrough")
	d.AddSessionTag(sID, "important")

	tags, _ := d.ListSessionTags(sID)
	if len(tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(tags))
	}
}

func TestRemoveSessionTag(t *testing.T) {
	d := testDB(t)
	now := time.Now()
	pID, _ := d.UpsertProject(Project{Name: "proj", Path: "/a", Status: "active", DiscoveredAt: now})
	sID, _ := d.InsertSession(Session{ProjectID: pID, Summary: "test", Source: "wrapper", StartedAt: &now})

	d.AddSessionTag(sID, "remove-me")
	d.RemoveSessionTag(sID, "remove-me")

	tags, _ := d.ListSessionTags(sID)
	if len(tags) != 0 {
		t.Errorf("expected 0 tags after remove, got %d", len(tags))
	}
}

func TestListSessionsByTag(t *testing.T) {
	d := testDB(t)
	now := time.Now()
	pID, _ := d.UpsertProject(Project{Name: "proj", Path: "/a", Status: "active", DiscoveredAt: now})
	s1, _ := d.InsertSession(Session{ProjectID: pID, Summary: "tagged", Source: "wrapper", StartedAt: &now})
	d.InsertSession(Session{ProjectID: pID, Summary: "not tagged", Source: "wrapper", StartedAt: &now})

	d.AddSessionTag(s1, "special")

	sessions, _ := d.ListSessionsByTag("special")
	if len(sessions) != 1 || sessions[0].Summary != "tagged" {
		t.Errorf("expected 1 tagged session, got %d", len(sessions))
	}
}
