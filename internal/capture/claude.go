// internal/capture/claude.go
package capture

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ClaudeSession struct {
	SessionID string
	CWD       string
	StartedAt time.Time
}

type claudeSessionFile struct {
	PID       int    `json:"pid"`
	SessionID string `json:"sessionId"`
	CWD       string `json:"cwd"`
	StartedAt string `json:"startedAt"`
}

// FindLatestSession looks in the Claude sessions directory for the most recent
// session matching the given working directory. Returns nil if none found.
func FindLatestSession(claudeDir string, workDir string) (*ClaudeSession, error) {
	sessionsDir := filepath.Join(claudeDir, "sessions")
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var latest *ClaudeSession
	var latestTime time.Time

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		data, err := os.ReadFile(filepath.Join(sessionsDir, entry.Name()))
		if err != nil {
			continue
		}

		var sf claudeSessionFile
		if err := json.Unmarshal(data, &sf); err != nil {
			continue
		}

		if sf.CWD != workDir {
			continue
		}

		startedAt, err := time.Parse(time.RFC3339, sf.StartedAt)
		if err != nil {
			continue
		}

		if latest == nil || startedAt.After(latestTime) {
			latest = &ClaudeSession{
				SessionID: sf.SessionID,
				CWD:       sf.CWD,
				StartedAt: startedAt,
			}
			latestTime = startedAt
		}
	}

	return latest, nil
}

// DefaultClaudeDir returns the default Claude config directory.
func DefaultClaudeDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude")
}

// FindSessionJSONL locates the JSONL conversation log for a Claude session.
// Returns the path if found, empty string otherwise.
func FindSessionJSONL(claudeDir, sessionID, workDir string) string {
	if sessionID == "" {
		return ""
	}
	// Claude stores JSONL at: ~/.claude/projects/<slug>/<session-id>.jsonl
	// where <slug> is the workDir path with "/" replaced by "-"
	slug := strings.ReplaceAll(workDir, "/", "-")
	jsonlPath := filepath.Join(claudeDir, "projects", slug, sessionID+".jsonl")
	if _, err := os.Stat(jsonlPath); err == nil {
		return jsonlPath
	}
	return ""
}
