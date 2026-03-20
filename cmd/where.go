// cmd/where.go
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

var whereCmd = &cobra.Command{
	Use:   "where <query>",
	Short: "Find which projects and files match a query",
	Long:  "Searches session summaries and file paths, then groups results by project and file.",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := db.Open(config.DBPath())
		if err != nil {
			return err
		}
		defer database.Close()

		query := strings.Join(args, " ")

		// Search two sources: FTS on summary + LIKE on files_changed
		ftsResults, err := database.SearchSessions(query)
		if err != nil {
			return err
		}

		allSessions, err := database.ListSessions(db.SessionFilter{Limit: 1000})
		if err != nil {
			return err
		}
		var fileResults []db.Session
		for _, s := range allSessions {
			if strings.Contains(strings.ToLower(s.FilesChanged), strings.ToLower(query)) {
				fileResults = append(fileResults, s)
			}
		}

		// Merge and deduplicate
		seen := map[int64]bool{}
		var merged []db.Session
		for _, s := range ftsResults {
			if !seen[s.ID] {
				seen[s.ID] = true
				merged = append(merged, s)
			}
		}
		for _, s := range fileResults {
			if !seen[s.ID] {
				seen[s.ID] = true
				merged = append(merged, s)
			}
		}

		// Group by project → file → sessions
		type fileEntry struct {
			dates []string
		}
		projectFiles := map[string]map[string]*fileEntry{}

		for _, s := range merged {
			if _, ok := projectFiles[s.ProjectName]; !ok {
				projectFiles[s.ProjectName] = map[string]*fileEntry{}
			}
			var files []string
			json.Unmarshal([]byte(s.FilesChanged), &files)
			dateStr := ""
			if s.StartedAt != nil {
				dateStr = s.StartedAt.Format("Jan 02")
			}
			for _, f := range files {
				if _, ok := projectFiles[s.ProjectName][f]; !ok {
					projectFiles[s.ProjectName][f] = &fileEntry{}
				}
				projectFiles[s.ProjectName][f].dates = append(projectFiles[s.ProjectName][f].dates, dateStr)
			}
		}

		// Build display results
		var results []display.WhereResult
		for project, files := range projectFiles {
			wr := display.WhereResult{ProjectName: project}
			for path, entry := range files {
				wr.Files = append(wr.Files, display.WhereFile{
					Path:     path,
					Sessions: entry.dates,
				})
			}
			results = append(results, wr)
		}

		display.FormatWhere(os.Stdout, results)
		return nil
	},
}

func init() {
	whereCmd.GroupID = "query"
	rootCmd.AddCommand(whereCmd)
}
