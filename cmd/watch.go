// cmd/watch.go
package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"time"

	"github.com/digitalghost404/nexus/internal/config"
	"github.com/digitalghost404/nexus/internal/db"
	"github.com/digitalghost404/nexus/internal/display"
	"github.com/spf13/cobra"
)

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Live-updating project dashboard",
	Long:  "Auto-refreshing terminal display of project status. Updates every 30 seconds.",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()

		database, err := db.Open(config.DBPath())
		if err != nil {
			return err
		}
		defer database.Close()

		for {
			clearScreen()

			dirty, _ := database.ListDirtyProjects()
			sessions, _ := database.ListSessions(db.SessionFilter{Limit: 5})
			stale, _ := database.ListProjects("stale")

			cwd, _ := os.Getwd()
			absDir, _ := filepath.Abs(cwd)
			currentProject := ""
			p, _ := database.GetProjectByPath(absDir)
			if p != nil {
				currentProject = p.Name
			}

			display.FormatSmartSummary(os.Stdout, dirty, sessions, stale, currentProject)
			fmt.Println("Refreshing every 30s — Ctrl+C to exit")
			fmt.Printf("Last refresh: %s\n", time.Now().Format("15:04:05"))

			select {
			case <-ctx.Done():
				return nil
			case <-time.After(30 * time.Second):
			}
		}
	},
}

func clearScreen() {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", "cls")
	} else {
		cmd = exec.Command("clear")
	}
	cmd.Stdout = os.Stdout
	cmd.Run()
}

func init() {
	watchCmd.GroupID = "maintenance"
	rootCmd.AddCommand(watchCmd)
}
