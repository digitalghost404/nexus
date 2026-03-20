// internal/capture/capture.go
package capture

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/digitalghost404/nexus/internal/db"
	"github.com/digitalghost404/nexus/internal/scanner"
)

type CaptureResult struct {
	ProjectName string
	SessionID   int64
	Summary     string
	Commits     int
	Files       int
}

// CaptureSession captures a Claude session for the given directory.
// claudeDir can be empty to use the default ~/.claude/ path.
func CaptureSession(database *db.DB, workDir string, claudeDir string) (*CaptureResult, error) {
	absDir, err := filepath.Abs(workDir)
	if err != nil {
		return nil, fmt.Errorf("resolve path: %w", err)
	}

	projectName := filepath.Base(absDir)

	// Try to find Claude session info
	if claudeDir == "" {
		claudeDir = DefaultClaudeDir()
	}

	now := time.Now()
	var startedAt *time.Time
	var claudeSessionID string

	claudeSession, _ := FindLatestSession(claudeDir, absDir)
	if claudeSession != nil {
		startedAt = &claudeSession.StartedAt
		claudeSessionID = claudeSession.SessionID

		// Check for duplicate
		exists, _ := database.SessionExists(claudeSessionID)
		if exists {
			return &CaptureResult{ProjectName: projectName, Summary: "duplicate session, skipped"}, nil
		}
	}

	// Fallback: use 8-hour window
	if startedAt == nil {
		t := now.Add(-8 * time.Hour)
		startedAt = &t
	}

	// Gather git data
	var commits []scanner.CommitInfo
	var files []string
	var languages []string
	var branch string
	var dirtyCount int
	var dirty bool
	var commitMsg string
	var commitTime time.Time

	if scanner.IsGitRepo(absDir) {
		branch, _ = scanner.GetBranch(absDir)
		dirtyCount, _ = scanner.GetDirtyFileCount(absDir)
		dirty = dirtyCount > 0
		commitMsg, commitTime, _ = scanner.GetLastCommit(absDir)
		commits, _ = scanner.GetCommitsSince(absDir, *startedAt)
		files, _ = scanner.GetChangedFiles(absDir, *startedAt)
		languages = scanner.DetectLanguages(absDir)
	}

	// Ensure project exists in DB with full git health data
	proj := db.Project{
		Name:          projectName,
		Path:          absDir,
		Branch:        branch,
		Dirty:         dirty,
		DirtyFiles:    dirtyCount,
		LastCommitMsg: commitMsg,
		Languages:     TagsToJSON(languages),
		Status:        "active",
		DiscoveredAt:  now,
	}
	if !commitTime.IsZero() {
		proj.LastCommitAt = &commitTime
	}
	projectID, err := database.UpsertProject(proj)
	if err != nil {
		return nil, fmt.Errorf("upsert project: %w", err)
	}

	summary := GenerateSummary(commits, files)
	tags := GenerateTags(projectName, languages)

	sessionID, err := database.InsertSession(db.Session{
		ProjectID:       projectID,
		ClaudeSessionID: claudeSessionID,
		StartedAt:       startedAt,
		EndedAt:         &now,
		DurationSecs:    int(now.Sub(*startedAt).Seconds()),
		Summary:         summary,
		FilesChanged:    FilesToJSON(files),
		CommitsMade:     CommitsToJSON(commits),
		Tags:            TagsToJSON(tags),
		Source:          "wrapper",
	})
	if err != nil {
		return nil, fmt.Errorf("insert session: %w", err)
	}

	return &CaptureResult{
		ProjectName: projectName,
		SessionID:   sessionID,
		Summary:     summary,
		Commits:     len(commits),
		Files:       len(files),
	}, nil
}
