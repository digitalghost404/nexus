// cmd/capture.go
package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/digitalghost404/nexus/internal/capture"
	"github.com/digitalghost404/nexus/internal/db"
	"github.com/spf13/cobra"
)

var captureDir string

var captureCmd = &cobra.Command{
	Use:    "capture",
	Short:  "Capture a Claude session (called by shell wrapper)",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if captureDir == "" {
			return fmt.Errorf("--dir is required")
		}

		// Probe-before-write: try HTTP POST to serve endpoint first
		probeURL := fmt.Sprintf("http://127.0.0.1:%d/api/capture", cfg.ServePort)
		body, err := json.Marshal(map[string]string{"dir": captureDir})
		if err == nil {
			client := &http.Client{Timeout: 2 * time.Second}
			resp, err := client.Post(probeURL, "application/json", bytes.NewReader(body))
			if err == nil {
				defer resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					if debug {
						fmt.Printf("Captured session via serve API (online mode)\n")
					}
					return nil
				}
			}
		}

		// Connection refused or error — fall back to direct DB write (offline mode)
		if debug {
			fmt.Printf("Serve unavailable, writing directly to DB (offline mode)\n")
		}

		database, err := db.Open(cfg.DBPath())
		if err != nil {
			return fmt.Errorf("open db: %w", err)
		}
		defer database.Close()

		result, err := capture.CaptureSession(database, captureDir, "")
		if err != nil {
			return fmt.Errorf("capture: %w", err)
		}

		if debug {
			fmt.Printf("Captured session for %s: %s (%d commits, %d files)\n",
				result.ProjectName, result.Summary, result.Commits, result.Files)
		}

		return nil
	},
}

func init() {
	captureCmd.GroupID = "core"
	captureCmd.Flags().StringVar(&captureDir, "dir", "", "Working directory of the session")
	rootCmd.AddCommand(captureCmd)
}
