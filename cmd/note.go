// cmd/note.go
package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/digitalghost404/nexus/internal/db"
	"github.com/spf13/cobra"
)

var noteCmd = &cobra.Command{
	Use:   "note <message>",
	Short: "Add a note to the current project",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		message := strings.Join(args, " ")

		// Try to find current project
		cwd, _ := os.Getwd()
		absDir, _ := filepath.Abs(cwd)
		var projectID *int64
		var projectName string

		database, err := db.Open(cfg.DBPath())
		if err != nil {
			return err
		}
		defer database.Close()

		p, _ := database.GetProjectByPath(absDir)
		if p != nil {
			projectID = &p.ID
			projectName = p.Name
		}

		// Probe-before-write: try HTTP POST to serve endpoint first
		probeURL := fmt.Sprintf("http://127.0.0.1:%d/api/notes", cfg.ServePort)
		reqBody, _ := json.Marshal(map[string]interface{}{
			"project": projectName,
			"text":    message,
		})
		client := &http.Client{Timeout: 2 * time.Second}
		resp, err := client.Post(probeURL, "application/json", bytes.NewReader(reqBody))
		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				if debug {
					fmt.Printf("Note added via serve API (online mode)\n")
				}
				return nil
			}
		}

		// Connection refused or error — fall back to direct DB write (offline mode)
		if debug {
			fmt.Printf("Serve unavailable, writing directly to DB (offline mode)\n")
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
	noteCmd.GroupID = "workflow"
	rootCmd.AddCommand(noteCmd)
}
