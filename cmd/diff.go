// cmd/diff.go
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/digitalghost404/nexus/internal/config"
	"github.com/digitalghost404/nexus/internal/db"
	"github.com/digitalghost404/nexus/internal/display"
	"github.com/spf13/cobra"
)

var diffSince string

var diffCmd = &cobra.Command{
	Use:   "diff [project]",
	Short: "Summarize changes across sessions in a time window",
	Long:  "Aggregates activity across all sessions for a project within a time range, showing commits, files, and a timeline.",
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := db.Open(config.DBPath())
		if err != nil {
			return err
		}
		defer database.Close()

		var project *db.Project
		if len(args) > 0 {
			project, err = database.GetProjectByName(args[0])
		} else {
			cwd, _ := os.Getwd()
			absDir, _ := filepath.Abs(cwd)
			project, err = database.GetProjectByPath(absDir)
		}
		if err != nil {
			return err
		}
		if project == nil {
			return fmt.Errorf("project not found")
		}

		since, err := parseDuration(diffSince)
		if err != nil {
			return fmt.Errorf("invalid --since: %w", err)
		}

		sessions, err := database.GetSessionsInRange(project.ID, *since, time.Now())
		if err != nil {
			return err
		}

		display.FormatDiff(os.Stdout, project.Name, diffSince, sessions)
		return nil
	},
}

func init() {
	diffCmd.Flags().StringVar(&diffSince, "since", "7d", "Time window (e.g. 7d, 30d)")
	diffCmd.GroupID = "workflow"
	rootCmd.AddCommand(diffCmd)
}
