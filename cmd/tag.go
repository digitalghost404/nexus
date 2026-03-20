// cmd/tag.go
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/digitalghost404/nexus/internal/config"
	"github.com/digitalghost404/nexus/internal/db"
	"github.com/spf13/cobra"
)

var tagRemove string

var tagCmd = &cobra.Command{
	Use:   "tag [session-id] <label>",
	Short: "Tag sessions with labels",
	Long:  "Adds a user tag to a session. Without a session ID, tags the latest session for the current project.",
	Args:  cobra.MinimumNArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := db.Open(config.DBPath())
		if err != nil {
			return err
		}
		defer database.Close()

		var sessionID int64
		var label string

		// --remove with no positional arg: resolve latest session for current project
		if tagRemove != "" && len(args) == 0 {
			cwd, _ := os.Getwd()
			absDir, _ := filepath.Abs(cwd)
			p, _ := database.GetProjectByPath(absDir)
			if p == nil {
				return fmt.Errorf("not inside a tracked project — specify a session ID")
			}
			latest, _ := database.GetLatestSession(p.ID)
			if latest == nil {
				return fmt.Errorf("no sessions found for %s", p.Name)
			}
			database.RemoveSessionTag(latest.ID, tagRemove)
			fmt.Printf("✓ Removed tag \"%s\" from session #%d\n", tagRemove, latest.ID)
			return nil
		}

		if len(args) == 0 {
			return fmt.Errorf("provide a label, or use --remove <tag> to remove a tag from the latest session")
		}

		// Parse args: first arg might be a session ID (numeric) or a label
		if len(args) >= 2 {
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err == nil {
				sessionID = id
				label = args[1]
			} else {
				// Both args are strings — error
				return fmt.Errorf("first arg must be a session ID or omit it to use latest session")
			}
		} else {
			// Single arg = label, use latest session for current project
			label = args[0]

			cwd, _ := os.Getwd()
			absDir, _ := filepath.Abs(cwd)
			p, _ := database.GetProjectByPath(absDir)
			if p == nil {
				return fmt.Errorf("not inside a tracked project — specify a session ID")
			}

			latest, _ := database.GetLatestSession(p.ID)
			if latest == nil {
				return fmt.Errorf("no sessions found for %s", p.Name)
			}
			sessionID = latest.ID
		}

		if tagRemove != "" {
			database.RemoveSessionTag(sessionID, tagRemove)
			fmt.Printf("✓ Removed tag \"%s\" from session #%d\n", tagRemove, sessionID)
			return nil
		}

		if err := database.AddSessionTag(sessionID, label); err != nil {
			return err
		}
		fmt.Printf("✓ Tagged session #%d with \"%s\"\n", sessionID, label)
		return nil
	},
}

func init() {
	tagCmd.Flags().StringVar(&tagRemove, "remove", "", "Remove a tag instead of adding")
	tagCmd.GroupID = "maintenance"
	rootCmd.AddCommand(tagCmd)
}
