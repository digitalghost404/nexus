// cmd/resume.go
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/digitalghost404/nexus/internal/capture"
	"github.com/digitalghost404/nexus/internal/config"
	"github.com/digitalghost404/nexus/internal/db"
	"github.com/digitalghost404/nexus/internal/display"
	"github.com/digitalghost404/nexus/internal/scanner"
	"github.com/spf13/cobra"
)

var resumeCmd = &cobra.Command{
	Use:   "resume [project]",
	Short: "Pick up where you left off on a project",
	Long:  "Shows the last Claude session for a project with commits, files changed, and current uncommitted changes.",
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := db.Open(config.DBPath())
		if err != nil {
			return err
		}
		defer database.Close()

		// Resolve project
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
			return fmt.Errorf("project not found — run from inside a project or specify a name")
		}

		session, err := database.GetLatestSession(project.ID)
		if err != nil {
			return err
		}
		if session == nil {
			fmt.Printf("No sessions recorded for %s\n", project.Name)
			return nil
		}

		digestJSON, _ := database.GetConversationDigest(session.ID)
		if digestJSON == "" && session.ClaudeSessionID != "" {
			claudeDir := capture.DefaultClaudeDir()
			jsonlPath := capture.FindSessionJSONL(claudeDir, session.ClaudeSessionID, project.Path)
			if jsonlPath != "" {
				if parsed, err := capture.ParseJSONL(jsonlPath); err == nil && parsed != nil {
					if dj, err := json.Marshal(parsed); err == nil {
						digestJSON = string(dj)
						_ = database.InsertConversationDigest(session.ID, digestJSON)
					}
				}
			}
		}

		// Get live dirty files
		var dirtyFiles []string
		if scanner.IsGitRepo(project.Path) {
			details, _ := scanner.GetDirtyFileDetails(project.Path)
			for _, d := range details {
				dirtyFiles = append(dirtyFiles, fmt.Sprintf("%s %s", d.Status, d.Path))
			}
		}

		display.FormatResume(os.Stdout, session, dirtyFiles, digestJSON)
		return nil
	},
}

func init() {
	resumeCmd.GroupID = "workflow"
	rootCmd.AddCommand(resumeCmd)
}
