// internal/db/links_test.go
package db

import (
	"testing"
	"time"
)

func TestLinkAndGetProjects(t *testing.T) {
	d := testDB(t)
	now := time.Now()
	id1, _ := d.UpsertProject(Project{Name: "wraith", Path: "/a", Status: "active", DiscoveredAt: NullTime{Time: now, Valid: true}})
	id2, _ := d.UpsertProject(Project{Name: "dashboard", Path: "/b", Status: "active", DiscoveredAt: NullTime{Time: now, Valid: true}})

	err := d.LinkProjects(id1, id2)
	if err != nil {
		t.Fatalf("link: %v", err)
	}

	// Check from both directions
	linked1, _ := d.GetLinkedProjects(id1)
	if len(linked1) != 1 || linked1[0].Name != "dashboard" {
		t.Errorf("expected dashboard linked to wraith, got %v", linked1)
	}

	linked2, _ := d.GetLinkedProjects(id2)
	if len(linked2) != 1 || linked2[0].Name != "wraith" {
		t.Errorf("expected wraith linked to dashboard, got %v", linked2)
	}
}

func TestUnlinkProjects(t *testing.T) {
	d := testDB(t)
	now := time.Now()
	id1, _ := d.UpsertProject(Project{Name: "a", Path: "/a", Status: "active", DiscoveredAt: NullTime{Time: now, Valid: true}})
	id2, _ := d.UpsertProject(Project{Name: "b", Path: "/b", Status: "active", DiscoveredAt: NullTime{Time: now, Valid: true}})

	if err := d.LinkProjects(id1, id2); err != nil {
		t.Fatalf("LinkProjects: %v", err)
	}
	if err := d.UnlinkProjects(id1, id2); err != nil {
		t.Fatalf("UnlinkProjects: %v", err)
	}

	linked, _ := d.GetLinkedProjects(id1)
	if len(linked) != 0 {
		t.Errorf("expected no links after unlink, got %d", len(linked))
	}
}

func TestLinkIdempotent(t *testing.T) {
	d := testDB(t)
	now := time.Now()
	id1, _ := d.UpsertProject(Project{Name: "a", Path: "/a", Status: "active", DiscoveredAt: NullTime{Time: now, Valid: true}})
	id2, _ := d.UpsertProject(Project{Name: "b", Path: "/b", Status: "active", DiscoveredAt: NullTime{Time: now, Valid: true}})

	d.LinkProjects(id1, id2)
	err := d.LinkProjects(id1, id2) // should not error
	if err != nil {
		t.Errorf("duplicate link should not error: %v", err)
	}
}
