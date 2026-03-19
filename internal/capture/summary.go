// internal/capture/summary.go
package capture

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/digitalghost404/nexus/internal/scanner"
)

// GenerateSummary creates a summary string from commits and/or changed files.
func GenerateSummary(commits []scanner.CommitInfo, files []string) string {
	if len(commits) > 0 {
		var msgs []string
		for _, c := range commits {
			msgs = append(msgs, c.Message)
		}
		return strings.Join(msgs, "; ")
	}

	if len(files) > 0 {
		if len(files) <= 3 {
			return fmt.Sprintf("Changed: %s", strings.Join(files, ", "))
		}
		return fmt.Sprintf("Changed %d files including %s", len(files), strings.Join(files[:3], ", "))
	}

	return ""
}

// GenerateTags creates tags from project name and detected languages.
func GenerateTags(projectName string, languages []string) []string {
	tags := []string{projectName}
	tags = append(tags, languages...)
	return tags
}

// TagsToJSON converts a string slice to a JSON array string.
func TagsToJSON(tags []string) string {
	if len(tags) == 0 {
		return "[]"
	}
	data, _ := json.Marshal(tags)
	return string(data)
}

// CommitsToJSON converts commits to a JSON array string.
func CommitsToJSON(commits []scanner.CommitInfo) string {
	if len(commits) == 0 {
		return "[]"
	}
	type entry struct {
		Hash    string `json:"hash"`
		Message string `json:"message"`
	}
	var entries []entry
	for _, c := range commits {
		entries = append(entries, entry{Hash: c.Hash, Message: c.Message})
	}
	data, _ := json.Marshal(entries)
	return string(data)
}

// FilesToJSON converts a file list to a JSON array string.
func FilesToJSON(files []string) string {
	if len(files) == 0 {
		return "[]"
	}
	data, _ := json.Marshal(files)
	return string(data)
}
