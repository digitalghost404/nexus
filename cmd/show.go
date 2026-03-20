// cmd/show.go
package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/digitalghost404/nexus/internal/config"
	"github.com/digitalghost404/nexus/internal/db"
	"github.com/digitalghost404/nexus/internal/display"
	"github.com/digitalghost404/nexus/internal/scanner"
	"github.com/spf13/cobra"
)

var showCmd = &cobra.Command{
	Use:   "show <project>",
	Short: "Show detailed info for a specific project",
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

		sessions, _ := database.ListSessions(db.SessionFilter{ProjectID: p.ID, Limit: 5})
		staleBranches, _ := scanner.GetStaleBranches(p.Path, 7*24*time.Hour)

		display.FormatProjectDetail(os.Stdout, p, sessions, staleBranches)
		return nil
	},
}

func init() {
	showCmd.GroupID = "query"
	rootCmd.AddCommand(showCmd)
}
