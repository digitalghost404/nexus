// internal/scanner/scanner.go
package scanner

import (
	"os"
	"path/filepath"
	"strings"
)

// Discover walks the given roots and returns absolute paths to directories
// containing a .git folder, skipping paths matching exclusion patterns.
func Discover(roots []string, exclude []string) ([]string, error) {
	var projects []string
	seen := map[string]bool{}

	for _, root := range roots {
		root, err := filepath.Abs(root)
		if err != nil {
			continue
		}

		filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}

			if !d.IsDir() {
				return nil
			}

			// Note: WalkDir does not follow symlinks for directories,
			// so symlink traversal is prevented by WalkDir's semantics.

			// Check exclusions
			for _, pattern := range exclude {
				if matched, _ := filepath.Match(pattern, path); matched {
					return filepath.SkipDir
				}
				// Also check if any path component matches
				if matchPathPattern(path, pattern) {
					return filepath.SkipDir
				}
			}

			// Check if this dir has a .git folder
			gitDir := filepath.Join(path, ".git")
			if info, err := os.Stat(gitDir); err == nil && info.IsDir() {
				absPath, _ := filepath.Abs(path)
				if !seen[absPath] {
					seen[absPath] = true
					projects = append(projects, absPath)
				}
				return filepath.SkipDir // Don't descend into git repos
			}

			return nil
		})
	}

	return projects, nil
}

func matchPathPattern(path, pattern string) bool {
	// Handle patterns like "*/node_modules/*" by checking if non-wildcard
	// pattern parts appear as consecutive path components in order.
	patternParts := strings.Split(pattern, "/")
	pathParts := strings.Split(path, "/")

	// Extract non-wildcard pattern parts
	var concrete []string
	for _, pp := range patternParts {
		if pp != "*" && pp != "" {
			concrete = append(concrete, pp)
		}
	}
	if len(concrete) == 0 {
		return false
	}

	// Check if concrete parts appear consecutively in pathParts
	for i := 0; i <= len(pathParts)-len(concrete); i++ {
		allMatch := true
		for j, c := range concrete {
			matched, _ := filepath.Match(c, pathParts[i+j])
			if !matched {
				allMatch = false
				break
			}
		}
		if allMatch {
			return true
		}
	}
	return false
}
