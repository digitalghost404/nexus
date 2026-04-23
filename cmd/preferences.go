package cmd

import (
	"fmt"

	"github.com/digitalghost404/nexus/internal/db"
	"github.com/spf13/cobra"
)

var preferencesCmd = &cobra.Command{
	Use:   "preferences",
	Short: "List preferences and patterns",
	RunE: func(cmd *cobra.Command, args []string) error {
		project, _ := cmd.Flags().GetString("project")
		_, _ = cmd.Flags().GetString("category") // reserved for future filtering

		database, err := db.Open(cfg.DBPath())
		if err != nil {
			return err
		}
		defer func() { _ = database.Close() }()

		var projectID *int64
		if project != "" {
			p, err := database.GetProjectByName(project)
			if err != nil || p == nil {
				fmt.Printf("Project %q not found. Showing global preferences.\n", project)
			} else {
				pid := p.ID
				projectID = &pid
			}
		}

		prefs, err := database.ListPreferencesByProject(projectID)
		if err != nil {
			return fmt.Errorf("list preferences: %w", err)
		}

		if len(prefs) == 0 {
			fmt.Println("No preferences found.")
			return nil
		}

		for _, p := range prefs {
			scope := "global"
			if p.ProjectID != nil {
				scope = fmt.Sprintf("project:%d", *p.ProjectID)
			}
			sourceTag := ""
			if p.Source != "stated" {
				sourceTag = fmt.Sprintf(" (%s, %.0f%%)", p.Source, p.Confidence*100)
			}
			fmt.Printf("[%s/%s] %s%s\n", p.Category, scope, p.Content, sourceTag)
		}

		return nil
	},
}

func init() {
	preferencesCmd.GroupID = "query"
	preferencesCmd.Flags().StringP("project", "p", "", "Project scope")
	preferencesCmd.Flags().String("category", "", "Filter by category")
	rootCmd.AddCommand(preferencesCmd)
}
