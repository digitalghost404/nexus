package cmd

import (
	"fmt"

	"github.com/digitalghost404/nexus/internal/db"
	"github.com/spf13/cobra"
)

var rememberCmd = &cobra.Command{
	Use:   "remember <content>",
	Short: "Save a preference, decision, or pattern",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		content := args[0]
		category, _ := cmd.Flags().GetString("category")
		source, _ := cmd.Flags().GetString("source")
		project, _ := cmd.Flags().GetString("project")

		database, err := db.Open(cfg.DBPath())
		if err != nil {
			return err
		}
		defer func() { _ = database.Close() }()

		var projectID *int64
		if project != "" {
			p, err := database.GetProjectByName(project)
			if err != nil {
				return fmt.Errorf("project %q not found: %w", project, err)
			}
			if p == nil {
				return fmt.Errorf("project %q not found", project)
			}
			projectID = &p.ID
		}

		confidence := 1.0
		switch source {
		case "observed":
			confidence = 0.7
		case "inferred":
			confidence = 0.4
		}

		id, err := database.InsertPreference(db.Preference{
			ProjectID:  projectID,
			Category:   category,
			Content:    content,
			Source:     source,
			Confidence: confidence,
		})
		if err != nil {
			return err
		}

		fmt.Printf("Remembered [%s]: %s (confidence: %.0f%%, id: %d)\n", category, content, confidence*100, id)
		return nil
	},
}

func init() {
	rememberCmd.GroupID = "workflow"
	rememberCmd.Flags().String("category", "preference", "Category: workflow, style, tool, preference, pattern")
	rememberCmd.Flags().String("source", "stated", "Source: stated, observed, inferred")
	rememberCmd.Flags().StringP("project", "p", "", "Project scope (optional)")
	rootCmd.AddCommand(rememberCmd)
}
