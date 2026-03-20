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
	msg, err := gitCmd(dir, "log", "-1", "--format=%s")
	if err != nil {
		return "", time.Time{}, err
	}

	ts, err := gitCmd(dir, "log", "-1", "--format=%aI")
	if err != nil {
		return "", time.Time{}, err
	}

	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("parse time: %w", err)
	}

	return msg, t, nil
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
	out, err := gitCmd(dir, "log", "--since="+since.Format(time.RFC3339),
		"--format=%H|%s|%aI", "--no-merges")
	if err != nil || out == "" {
		return nil, err
	}

	var commits []CommitInfo
	for _, line := range strings.Split(out, "\n") {
		parts := strings.SplitN(line, "|", 3)
		if len(parts) != 3 {
			continue
		}
		ts, _ := time.Parse(time.RFC3339, parts[2])
		commits = append(commits, CommitInfo{
			Hash:    parts[0][:8],
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
		"--format=%(refname:short)|%(committerdate:iso-strict)", "refs/heads/")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}

	cutoff := time.Now().Add(-olderThan)
	var stale []StaleBranch
	for _, line := range strings.Split(out, "\n") {
		parts := strings.SplitN(line, "|", 2)
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
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		// Skip hidden dirs and common noise
		name := info.Name()
		if info.IsDir() && (strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor") {
			return filepath.SkipDir
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
