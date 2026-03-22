// cmd/context_cmd.go
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/digitalghost404/nexus/internal/capture"
	"github.com/digitalghost404/nexus/internal/config"
	"github.com/digitalghost404/nexus/internal/db"
	"github.com/digitalghost404/nexus/internal/display"
	"github.com/spf13/cobra"
)

var contextCmd = &cobra.Command{
	Use:   "context <project>",
	Short: "Export project context for pasting into Claude",
	Long:  "Outputs everything Nexus knows about a project in markdown format, optimized for sharing with Claude.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := db.Open(config.DBPath())
		if err != nil {
			return err
		}
		defer database.Close()

		p, err := database.GetProjectByName(args[0])
		if err != nil {
			return err
		}
		if p == nil {
			return fmt.Errorf("project not found: %s", args[0])
		}

		since := time.Now().AddDate(0, 0, -7)
		sessions, _ := database.GetSessionsInRange(p.ID, since, time.Now())
		notes, _ := database.ListNotes(p.ID, 10)

		// Linked projects — conditional on v2 migration
		var linkedProjects []db.Project
		// Try to query project_links; if table doesn't exist, just skip
		linkedProjects, _ = database.GetLinkedProjects(p.ID)

		digests := make(map[int64]string)
		for _, s := range sessions {
			if d, err := database.GetConversationDigest(s.ID); err == nil && d != "" {
				digests[s.ID] = d
			} else if s.ClaudeSessionID != "" {
				claudeDir := capture.DefaultClaudeDir()
				jsonlPath := capture.FindSessionJSONL(claudeDir, s.ClaudeSessionID, p.Path)
				if jsonlPath != "" {
					if parsed, err := capture.ParseJSONL(jsonlPath); err == nil && parsed != nil {
						if digestJSON, err := json.Marshal(parsed); err == nil {
							digestStr := string(digestJSON)
							_ = database.InsertConversationDigest(s.ID, digestStr)
							digests[s.ID] = digestStr
						}
					}
				}
			}
		}

		display.FormatContext(os.Stdout, p, sessions, notes, linkedProjects, digests)
		return nil
	},
}

func init() {
	contextCmd.GroupID = "workflow"
	rootCmd.AddCommand(contextCmd)
}
