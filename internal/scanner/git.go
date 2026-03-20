// internal/scanner/git.go
package scanner

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func gitCmd(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}

func IsGitRepo(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

func GetBranch(dir string) (string, error) {
	return gitCmd(dir, "rev-parse", "--abbrev-ref", "HEAD")
}

func GetDirtyFileCount(dir string) (int, error) {
	out, err := gitCmd(dir, "status", "--porcelain")
	if err != nil {
		return 0, err
	}
	if out == "" {
		return 0, nil
	}
	return len(strings.Split(out, "\n")), nil
}

func GetLastCommit(dir string) (message string, when time.Time, err error) {
	out, err := gitCmd(dir, "log", "-1", "--format=%s%x00%aI")
	if err != nil {
		return "", time.Time{}, err
	}

	parts := strings.SplitN(out, "\x00", 2)
	if len(parts) != 2 {
		return "", time.Time{}, fmt.Errorf("unexpected git log output: %s", out)
	}

	t, err := time.Parse(time.RFC3339, parts[1])
	if err != nil {
		return "", time.Time{}, fmt.Errorf("parse time: %w", err)
	}

	return parts[0], t, nil
}

func GetAheadBehind(dir string) (ahead, behind int, err error) {
	out, err := gitCmd(dir, "rev-list", "--left-right", "--count", "HEAD...@{upstream}")
	if err != nil {
		// No upstream configured
		return 0, 0, nil
	}
	fmt.Sscanf(out, "%d\t%d", &ahead, &behind)
	return ahead, behind, nil
}

func GetCommitsSince(dir string, since time.Time) ([]CommitInfo, error) {
	// Use %x00 (null byte) as separator since commit messages can contain |
	out, err := gitCmd(dir, "log", "--since="+since.Format(time.RFC3339),
		"--format=%H%x00%s%x00%aI", "--no-merges")
	if err != nil || out == "" {
		return nil, err
	}

	var commits []CommitInfo
	for _, line := range strings.Split(out, "\n") {
		parts := strings.SplitN(line, "\x00", 3)
		if len(parts) != 3 {
			continue
		}
		ts, _ := time.Parse(time.RFC3339, parts[2])
		hash := parts[0]
		if len(hash) > 8 {
			hash = hash[:8]
		}
		commits = append(commits, CommitInfo{
			Hash:    hash,
			Message: parts[1],
			Time:    ts,
		})
	}
	return commits, nil
}

type CommitInfo struct {
	Hash    string
	Message string
	Time    time.Time
}

func GetChangedFiles(dir string, since time.Time) ([]string, error) {
	// Use git log to find files changed since the given time
	out, err := gitCmd(dir, "log", "--since="+since.Format(time.RFC3339),
		"--name-only", "--pretty=format:", "--no-merges")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	// Deduplicate file names
	seen := map[string]bool{}
	var files []string
	for _, f := range strings.Split(out, "\n") {
		f = strings.TrimSpace(f)
		if f != "" && !seen[f] {
			seen[f] = true
			files = append(files, f)
		}
	}
	return files, nil
}

func DeleteBranch(dir, branch string) error {
	_, err := gitCmd(dir, "branch", "-d", branch)
	return err
}

type DirtyFile struct {
	Status string // "M", "A", "D", "??"
	Path   string
}

func GetDirtyFileDetails(dir string) ([]DirtyFile, error) {
	out, err := gitCmd(dir, "status", "--porcelain")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	var files []DirtyFile
	for _, line := range strings.Split(out, "\n") {
		if len(line) < 4 {
			continue
		}
		files = append(files, DirtyFile{
			Status: strings.TrimSpace(line[:2]),
			Path:   line[3:],
		})
	}
	return files, nil
}

// StaleBranch holds the branch name and the time of its last commit.
type StaleBranch struct {
	Name       string
	LastCommit time.Time
}

func GetStaleBranches(dir string, olderThan time.Duration) ([]string, error) {
	branches, err := GetStaleBranchesWithDates(dir, olderThan)
	if err != nil {
		return nil, err
	}
	names := make([]string, len(branches))
	for i, b := range branches {
		names[i] = b.Name
	}
	return names, nil
}

// GetStaleBranchesWithDates returns stale branches together with their last commit time.
func GetStaleBranchesWithDates(dir string, olderThan time.Duration) ([]StaleBranch, error) {
	out, err := gitCmd(dir, "for-each-ref", "--sort=-committerdate",
		"--format=%(refname:short)%00%(committerdate:iso-strict)", "refs/heads/")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}

	cutoff := time.Now().Add(-olderThan)
	var stale []StaleBranch
	for _, line := range strings.Split(out, "\n") {
		parts := strings.SplitN(line, "\x00", 2)
		if len(parts) != 2 {
			continue
		}
		ts, err := time.Parse(time.RFC3339, parts[1])
		if err != nil {
			continue
		}
		if ts.Before(cutoff) {
			stale = append(stale, StaleBranch{Name: parts[0], LastCommit: ts})
		}
	}
	return stale, nil
}

var langMap = map[string]string{
	".go":   "go",
	".ts":   "typescript",
	".tsx":  "typescript",
	".js":   "javascript",
	".jsx":  "javascript",
	".py":   "python",
	".rs":   "rust",
	".java": "java",
	".rb":   "ruby",
	".tf":   "terraform",
	".yaml": "yaml",
	".yml":  "yaml",
	".sh":   "shell",
}

func DetectLanguages(dir string) []string {
	seen := map[string]bool{}
	filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			// Skip hidden dirs, common noise, and symlinks
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" {
				return filepath.SkipDir
			}
			if d.Type()&os.ModeSymlink != 0 {
				return filepath.SkipDir
			}
			return nil
		}
		ext := filepath.Ext(name)
		if lang, ok := langMap[ext]; ok {
			seen[lang] = true
		}
		return nil
	})

	var langs []string
	for l := range seen {
		langs = append(langs, l)
	}
	return langs
}
