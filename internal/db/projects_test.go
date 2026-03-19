// internal/db/projects_test.go
package db

import (
	"testing"
	"time"
)

// Note: Project struct is defined in projects.go, not here.

func TestUpsertAndGetProject(t *testing.T) {
	d := testDB(t)

	now := time.Now()
	p := Project{
		Name:         "wraith",
		Path:         "/home/user/projects/wraith",
		Languages:    `["go","typescript"]`,
		Branch:       "main",
		Dirty:        true,
		DirtyFiles:   3,
		Status:       "active",
		DiscoveredAt: now,
	}

	id, err := d.UpsertProject(p)
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if id == 0 {
		t.Fatal("expected non-zero id")
	}

	got, err := d.GetProjectByPath("/home/user/projects/wraith")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "wraith" {
		t.Errorf("expected wraith, got %s", got.Name)
	}
	if !got.Dirty {
		t.Error("expected dirty=true")
	}
}

func TestListProjectsByStatus(t *testing.T) {
	d := testDB(t)

	now := time.Now()
	d.UpsertProject(Project{Name: "active1", Path: "/a", Status: "active", DiscoveredAt: now})
	d.UpsertProject(Project{Name: "stale1", Path: "/b", Status: "stale", DiscoveredAt: now})
	d.UpsertProject(Project{Name: "active2", Path: "/c", Status: "active", DiscoveredAt: now})

	active, err := d.ListProjects("active")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(active) != 2 {
		t.Errorf("expected 2 active, got %d", len(active))
	}

	all, err := d.ListProjects("")
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3 total, got %d", len(all))
	}
}

func TestListDirtyProjects(t *testing.T) {
	d := testDB(t)

	now := time.Now()
	d.UpsertProject(Project{Name: "clean", Path: "/a", Dirty: false, Status: "active", DiscoveredAt: now})
	d.UpsertProject(Project{Name: "dirty", Path: "/b", Dirty: true, DirtyFiles: 2, Status: "active", DiscoveredAt: now})

	dirty, err := d.ListDirtyProjects()
	if err != nil {
		t.Fatalf("list dirty: %v", err)
	}
	if len(dirty) != 1 {
		t.Errorf("expected 1 dirty, got %d", len(dirty))
	}
}

func TestUpsertProjectUpdatesExisting(t *testing.T) {
	d := testDB(t)

	now := time.Now()
	d.UpsertProject(Project{Name: "proj", Path: "/a", Branch: "main", Status: "active", DiscoveredAt: now})
	d.UpsertProject(Project{Name: "proj", Path: "/a", Branch: "develop", Status: "idle", DiscoveredAt: now})

	got, err := d.GetProjectByPath("/a")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Branch != "develop" {
		t.Errorf("expected develop, got %s", got.Branch)
	}

	all, _ := d.ListProjects("")
	if len(all) != 1 {
		t.Errorf("expected 1 project (upsert), got %d", len(all))
	}
}
