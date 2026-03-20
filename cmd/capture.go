// cmd/capture.go
package cmd

import (
	"fmt"

	"github.com/digitalghost404/nexus/internal/capture"
	"github.com/digitalghost404/nexus/internal/config"
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

		database, err := db.Open(config.DBPath())
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
