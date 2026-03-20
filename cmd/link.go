// cmd/link.go
package cmd

import (
	"fmt"

	"github.com/digitalghost404/nexus/internal/config"
	"github.com/digitalghost404/nexus/internal/db"
	"github.com/spf13/cobra"
)

var linkUnlink string

func lookupProject(database *db.DB, name string) (*db.Project, error) {
	p, err := database.GetProjectByName(name)
	if err != nil {
		return nil, fmt.Errorf("get project %s: %w", name, err)
	}
	if p == nil {
		return nil, fmt.Errorf("project not found: %s", name)
	}
	return p, nil
}

var linkCmd = &cobra.Command{
	Use:   "link <project-a> [project-b]",
	Short: "Link related projects together",
	Long:  "Creates a bidirectional link between projects. With one arg, shows existing links.",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := db.Open(config.DBPath())
		if err != nil {
			return err
		}
		defer database.Close()

		p1, err := lookupProject(database, args[0])
		if err != nil {
			return err
		}

		// Unlink
		if linkUnlink != "" {
			p2, err := lookupProject(database, linkUnlink)
			if err != nil {
				return err
			}
			if err := database.UnlinkProjects(p1.ID, p2.ID); err != nil {
				return err
			}
			fmt.Printf("✓ Unlinked %s ↔ %s\n", p1.Name, p2.Name)
			return nil
		}

		// Show links (one arg)
		if len(args) == 1 {
			linked, err := database.GetLinkedProjects(p1.ID)
			if err != nil {
				return err
			}
			if len(linked) == 0 {
				fmt.Printf("No linked projects for %s\n", p1.Name)
				return nil
			}
			fmt.Printf("Linked projects for %s:\n", p1.Name)
			for _, lp := range linked {
				fmt.Printf("  %s\n", lp.Name)
			}
			return nil
		}

		// Create link (two args)
		p2, err := lookupProject(database, args[1])
		if err != nil {
			return err
		}

		if err := database.LinkProjects(p1.ID, p2.ID); err != nil {
			return err
		}
		fmt.Printf("✓ Linked %s ↔ %s\n", p1.Name, p2.Name)
		return nil
	},
}

func init() {
	linkCmd.Flags().StringVar(&linkUnlink, "unlink", "", "Unlink a project")
	linkCmd.GroupID = "maintenance"
	rootCmd.AddCommand(linkCmd)
}
