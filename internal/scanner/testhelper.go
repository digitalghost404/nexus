// internal/scanner/testhelper.go
package scanner

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// CreateTestRepo creates a git repo at the given absolute path for testing.
// Exported for use by other packages (e.g., capture tests).
func CreateTestRepo(t *testing.T, dir string) string {
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
