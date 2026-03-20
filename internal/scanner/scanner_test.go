// internal/scanner/scanner_test.go
package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverProjects(t *testing.T) {
	root := t.TempDir()

	// Create 3 git repos
	CreateTestRepo(t, filepath.Join(root, "project-a"))
	CreateTestRepo(t, filepath.Join(root, "project-b"))
	os.MkdirAll(filepath.Join(root, "not-a-repo"), 0755) // no .git

	// Nested repo
	CreateTestRepo(t, filepath.Join(root, "nested", "project-c"))

	projects, err := Discover([]string{root}, nil)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if len(projects) != 3 {
		t.Errorf("expected 3 projects, got %d: %v", len(projects), projects)
	}
}

func TestDiscoverRespectsExclusions(t *testing.T) {
	root := t.TempDir()

	CreateTestRepo(t, filepath.Join(root, "project-a"))
	CreateTestRepo(t, filepath.Join(root, "scratch-temp"))

	projects, err := Discover([]string{root}, []string{"*/scratch-*"})
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if len(projects) != 1 {
		t.Errorf("expected 1 project (excluded scratch), got %d", len(projects))
	}
}

func TestDiscoverSkipsNestedGit(t *testing.T) {
	root := t.TempDir()
	repo := CreateTestRepo(t, filepath.Join(root, "project"))

	// Create node_modules with its own .git (shouldn't be discovered)
	os.MkdirAll(filepath.Join(repo, "node_modules", "some-pkg", ".git"), 0755)

	projects, err := Discover([]string{root}, []string{"*/node_modules/*"})
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if len(projects) != 1 {
		t.Errorf("expected 1 project, got %d", len(projects))
	}
}
