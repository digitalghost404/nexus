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

		filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}

			if !info.IsDir() {
				return nil
			}

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
	// Handle patterns like "*/node_modules/*" by checking each path component
	patternParts := strings.Split(pattern, "/")
	pathParts := strings.Split(path, "/")

	for _, pp := range patternParts {
		if pp == "*" {
			continue
		}
		for _, pathPart := range pathParts {
			matched, _ := filepath.Match(pp, pathPart)
			if matched {
				return true
			}
		}
	}
	return false
}
