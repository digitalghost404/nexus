// cmd/scan.go
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/digitalghost404/nexus/internal/capture"
	"github.com/digitalghost404/nexus/internal/config"
	"github.com/digitalghost404/nexus/internal/db"
	"github.com/digitalghost404/nexus/internal/scanner"
	"github.com/spf13/cobra"
)

var scanVerbose bool

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan roots for projects and update health data",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(config.ConfigPath())
		if err != nil {
			return err
		}
		return runScan(cfg, scanVerbose)
	},
}

func runScan(cfg config.Config, verbose bool) error {
	database, err := db.Open(config.DBPath())
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer database.Close()

	// Discover projects
	paths, err := scanner.Discover(cfg.Roots, cfg.Exclude)
	if err != nil {
		return fmt.Errorf("discover: %w", err)
	}

	if verbose {
		fmt.Printf("Found %d projects\n", len(paths))
	}

	now := time.Now()
	for _, path := range paths {
		name := filepath.Base(path)

		branch, _ := scanner.GetBranch(path)
		dirtyCount, _ := scanner.GetDirtyFileCount(path)
		commitMsg, commitTime, _ := scanner.GetLastCommit(path)
		ahead, behind, _ := scanner.GetAheadBehind(path)
		languages := scanner.DetectLanguages(path)

		// Determine status
		status := "stale"
		lastActivity := commitTime
		if !lastActivity.IsZero() {
			daysSince := int(time.Since(lastActivity).Hours() / 24)
			switch {
			case daysSince <= cfg.Thresholds.Idle:
				status = "active"
			case daysSince <= cfg.Thresholds.Stale:
				status = "idle"
			}
		}

		langsJSON, _ := json.Marshal(languages)

		var lastCommitAt *time.Time
		if !commitTime.IsZero() {
			lastCommitAt = &commitTime
		}

		_, err := database.UpsertProject(db.Project{
			Name:          name,
			Path:          path,
			Languages:     string(langsJSON),
			Branch:        branch,
			Dirty:         dirtyCount > 0,
			DirtyFiles:    dirtyCount,
			LastCommitAt:  lastCommitAt,
			LastCommitMsg: commitMsg,
			Ahead:         ahead,
			Behind:        behind,
			Status:        status,
			DiscoveredAt:  now,
			LastScannedAt: &now,
		})

		if verbose {
			if err != nil {
				fmt.Printf("  ✗ %s: %v\n", name, err)
			} else {
				fmt.Printf("  ✓ %s (%s, %s)\n", name, branch, status)
			}
		}
	}

	// Archive disappeared projects
	allProjects, _ := database.ListProjects("")
	for _, p := range allProjects {
		if _, err := os.Stat(p.Path); os.IsNotExist(err) {
			database.ArchiveProject(p.ID)
			if verbose {
				fmt.Printf("  ⚠ Archived: %s (path no longer exists)\n", p.Name)
			}
		}
	}

	// Safety net: create inferred sessions from unrecorded git activity
	inferredCount := 0
	allProjects, _ = database.ListProjects("")
	for _, proj := range allProjects {
		if !scanner.IsGitRepo(proj.Path) {
			continue
		}
		// Look at commits in the last 24 hours
		since := now.Add(-24 * time.Hour)
		commits, _ := scanner.GetCommitsSince(proj.Path, since)
		if len(commits) == 0 {
			continue
		}

		// Check if any existing session covers this time range
		hasOverlap, _ := database.HasOverlappingSession(proj.ID, since, now)
		if hasOverlap {
			continue
		}

		// Create an inferred session
		files, _ := scanner.GetChangedFiles(proj.Path, since)
		summary := capture.GenerateSummary(commits, files)
		languages := scanner.DetectLanguages(proj.Path)
		tags := capture.GenerateTags(proj.Name, languages)

		database.InsertSession(db.Session{
			ProjectID:    proj.ID,
			StartedAt:    &since,
			EndedAt:      &now,
			DurationSecs: int(now.Sub(since).Seconds()),
			Summary:      summary,
			FilesChanged: capture.FilesToJSON(files),
			CommitsMade:  capture.CommitsToJSON(commits),
			Tags:         capture.TagsToJSON(tags),
			Source:       "scan",
		})
		inferredCount++
	}

	fmt.Printf("Scanned %d projects", len(paths))
	if inferredCount > 0 {
		fmt.Printf(", inferred %d sessions", inferredCount)
	}
	fmt.Println()
	return nil
}

func init() {
	scanCmd.GroupID = "core"
	scanCmd.Flags().BoolVarP(&scanVerbose, "verbose", "v", false, "Show scan details")
	rootCmd.AddCommand(scanCmd)
}
