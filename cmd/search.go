// cmd/search.go
package cmd

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/digitalghost404/nexus/internal/config"
	"github.com/digitalghost404/nexus/internal/db"
	"github.com/digitalghost404/nexus/internal/display"
	"github.com/spf13/cobra"
)

var (
	searchProject string
	searchFiles   string
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search sessions and notes by keyword",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := db.Open(config.DBPath())
		if err != nil {
			return err
		}
		defer database.Close()

		query := strings.Join(args, " ")

		// FTS search on sessions and notes
		sessions, err := database.SearchSessions(query)
		if err != nil {
			return err
		}

		notes, err := database.SearchNotes(query)
		if err != nil {
			return err
		}

		// Filter by project if specified
		if searchProject != "" {
			p, err := database.GetProjectByName(searchProject)
			if err != nil {
				return err
			}
			if p != nil {
				var filtered []db.Session
				for _, s := range sessions {
					if s.ProjectID == p.ID {
						filtered = append(filtered, s)
					}
				}
				sessions = filtered
			}
		}

		// Filter by files pattern if specified
		if searchFiles != "" {
			var filtered []db.Session
			for _, s := range sessions {
				var files []string
				json.Unmarshal([]byte(s.FilesChanged), &files)
				for _, f := range files {
					if matchFilePattern(f, searchFiles) {
						filtered = append(filtered, s)
						break
					}
				}
			}
			sessions = filtered
		}

		display.FormatSearchResults(os.Stdout, sessions, notes)
		return nil
	},
}

func matchFilePattern(file, pattern string) bool {
	// Simple glob: "*.go" matches "main.go", "cmd/root.go"
	if strings.HasPrefix(pattern, "*.") {
		ext := pattern[1:] // ".go"
		return strings.HasSuffix(file, ext)
	}
	return strings.Contains(file, pattern)
}

func init() {
	searchCmd.GroupID = "query"
	searchCmd.Flags().StringVar(&searchProject, "project", "", "Filter by project")
	searchCmd.Flags().StringVar(&searchFiles, "files", "", "Filter by file pattern")
	rootCmd.AddCommand(searchCmd)
}
