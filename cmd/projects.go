package cmd

import (
	"fmt"
	"os"

	"github.com/digitalghost404/nexus/internal/db"
	"github.com/digitalghost404/nexus/internal/display"
	"github.com/spf13/cobra"
)

var (
	projectsActive bool
	projectsDirty  bool
	projectsStale  bool
)

func init() {
	projectsCmd.GroupID = "query"
	projectsCmd.Flags().BoolVar(&projectsActive, "active", false, "Show active projects only")
	projectsCmd.Flags().BoolVar(&projectsDirty, "dirty", false, "Show dirty projects only")
	projectsCmd.Flags().BoolVar(&projectsStale, "stale", false, "Show stale projects only")
	projectsCmd.AddCommand(newProjectsArchiveCmd())
	projectsCmd.AddCommand(newProjectsDeleteCmd())
	rootCmd.AddCommand(projectsCmd)
}

var projectsCmd = &cobra.Command{
	Use:   "projects",
	Short: "List and manage tracked projects",
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := db.Open(cfg.DBPath())
		if err != nil {
			return err
		}
		defer func() { _ = database.Close() }()

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

func newProjectsArchiveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "archive <name>",
		Short: "Archive a project (hide from listings)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			database, err := db.Open(cfg.DBPath())
			if err != nil {
				return err
			}
			defer func() { _ = database.Close() }()

			p, err := database.GetProjectByName(args[0])
			if err != nil {
				return err
			}
			if p == nil {
				return fmt.Errorf("project %q not found", args[0])
			}

			if err := database.ArchiveProject(p.ID); err != nil {
				return fmt.Errorf("archive project: %w", err)
			}
			fmt.Printf("Project %q archived.\n", args[0])
			return nil
		},
	}
}

func newProjectsDeleteCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Permanently delete a project and all its data",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			database, err := db.Open(cfg.DBPath())
			if err != nil {
				return err
			}
			defer func() { _ = database.Close() }()

			p, err := database.GetProjectByName(args[0])
			if err != nil {
				return err
			}
			if p == nil {
				return fmt.Errorf("project %q not found", args[0])
			}

			if !force {
				fmt.Printf("This will permanently delete project %q and all associated sessions, notes, preferences, and embeddings.\n", args[0])
				fmt.Print("Type the project name to confirm: ")
				var confirm string
				_, _ = fmt.Scanln(&confirm)
				if confirm != args[0] {
					fmt.Println("Deletion cancelled.")
					return nil
				}
			}

			notesDeleted, err := database.DeleteProject(args[0])
			if err != nil {
				return fmt.Errorf("delete project: %w", err)
			}
			fmt.Printf("Project %q deleted (%d notes removed).\n", args[0], notesDeleted)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation prompt")
	return cmd
}
