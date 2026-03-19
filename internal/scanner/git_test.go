// internal/scanner/git_test.go
package scanner

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGetBranch(t *testing.T) {
	repo := CreateTestRepo(t, filepath.Join(t.TempDir(), "test-repo"))
	branch, err := GetBranch(repo)
	if err != nil {
		t.Fatalf("GetBranch: %v", err)
	}
	if branch != "main" {
		t.Errorf("expected main, got %s", branch)
	}
}

func TestGetDirtyFiles(t *testing.T) {
	repo := CreateTestRepo(t, filepath.Join(t.TempDir(), "test-repo"))

	// Clean initially
	count, err := GetDirtyFileCount(repo)
	if err != nil {
		t.Fatalf("GetDirtyFileCount: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 dirty, got %d", count)
	}

	// Create untracked file
	os.WriteFile(filepath.Join(repo, "new.txt"), []byte("new"), 0644)
	count, _ = GetDirtyFileCount(repo)
	if count != 1 {
		t.Errorf("expected 1 dirty, got %d", count)
	}
}

func TestGetLastCommit(t *testing.T) {
	repo := CreateTestRepo(t, filepath.Join(t.TempDir(), "test-repo"))

	msg, ts, err := GetLastCommit(repo)
	if err != nil {
		t.Fatalf("GetLastCommit: %v", err)
	}
	if msg != "initial commit" {
		t.Errorf("expected 'initial commit', got '%s'", msg)
	}
	if time.Since(ts) > time.Minute {
		t.Errorf("commit time too old: %v", ts)
	}
}

func TestDetectLanguages(t *testing.T) {
	repo := CreateTestRepo(t, filepath.Join(t.TempDir(), "test-repo"))
	os.WriteFile(filepath.Join(repo, "main.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(repo, "app.ts"), []byte("console.log()"), 0644)

	langs := DetectLanguages(repo)
	found := map[string]bool{}
	for _, l := range langs {
		found[l] = true
	}
	if !found["go"] {
		t.Error("expected go in languages")
	}
	if !found["typescript"] {
		t.Error("expected typescript in languages")
	}
}
