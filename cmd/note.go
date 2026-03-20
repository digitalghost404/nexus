// cmd/note.go
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/digitalghost404/nexus/internal/config"
	"github.com/digitalghost404/nexus/internal/db"
	"github.com/spf13/cobra"
)

var noteCmd = &cobra.Command{
	Use:   "note <message>",
	Short: "Add a note to the current project",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := db.Open(config.DBPath())
		if err != nil {
			return err
		}
		defer database.Close()

		message := strings.Join(args, " ")

		// Try to find current project
		cwd, _ := os.Getwd()
		absDir, _ := filepath.Abs(cwd)
		var projectID *int64

		p, _ := database.GetProjectByPath(absDir)
		if p != nil {
			projectID = &p.ID
		}

		_, err = database.InsertNote(db.Note{
			ProjectID: projectID,
			Content:   message,
		})
		if err != nil {
			return err
		}

		if p != nil {
			fmt.Printf("Note added to %s\n", p.Name)
		} else {
			fmt.Println("Global note added")
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(noteCmd)
}
