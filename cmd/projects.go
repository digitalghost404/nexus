// cmd/projects.go
package cmd

import (
	"os"

	"github.com/digitalghost404/nexus/internal/config"
	"github.com/digitalghost404/nexus/internal/db"
	"github.com/digitalghost404/nexus/internal/display"
	"github.com/spf13/cobra"
)

var (
	projectsActive bool
	projectsDirty  bool
	projectsStale  bool // includes both idle and stale
)

var projectsCmd = &cobra.Command{
	Use:   "projects",
	Short: "List all tracked projects with health status",
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := db.Open(config.DBPath())
		if err != nil {
			return err
		}
		defer database.Close()

		var projects []db.Project

		switch {
		case projectsDirty:
			projects, err = database.ListDirtyProjects()
		case projectsActive:
			projects, err = database.ListProjects("active")
		case projectsStale:
			idle, err := database.ListProjects("idle")
			if err != nil {
				return err
			}
			staleOnly, err := database.ListProjects("stale")
			if err != nil {
				return err
			}
			projects = append(idle, staleOnly...)
		default:
			projects, err = database.ListProjects("")
		}
		if err != nil {
			return err
		}

		display.FormatProjectTable(os.Stdout, projects)
		return nil
	},
}

func init() {
	projectsCmd.GroupID = "query"
	projectsCmd.Flags().BoolVar(&projectsActive, "active", false, "Show active projects only")
	projectsCmd.Flags().BoolVar(&projectsDirty, "dirty", false, "Show dirty projects only")
	projectsCmd.Flags().BoolVar(&projectsStale, "stale", false, "Show stale projects only")
	rootCmd.AddCommand(projectsCmd)
}
