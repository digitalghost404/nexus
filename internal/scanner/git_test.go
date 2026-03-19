// internal/scanner/git_test.go
package scanner

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func createTestRepo(t *testing.T, dir string) string {
	t.Helper()
	os.MkdirAll(dir, 0755)

	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s: %v", args, out, err)
		}
	}

	run("init", "-b", "main")
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# test"), 0644)
	run("add", ".")
	run("commit", "-m", "initial commit")

	return dir
}

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
