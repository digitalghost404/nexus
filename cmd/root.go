// cmd/root.go
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/digitalghost404/nexus/internal/config"
	"github.com/digitalghost404/nexus/internal/db"
	"github.com/digitalghost404/nexus/internal/display"
	"github.com/digitalghost404/nexus/internal/logger"
	"github.com/spf13/cobra"
)

var Version = "0.2.0"

var (
	debug bool
	log   *logger.Logger
)

var rootCmd = &cobra.Command{
	Use:     "nexus",
	Short:   "Track project health and Claude sessions",
	Long:    "Nexus gives you a single pane of glass into all your projects and Claude Code sessions.",
	Version: Version,
	Args:  cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			// Check if first arg is a registered subcommand
			for _, sub := range cmd.Commands() {
				if sub.Name() == args[0] || sub.HasAlias(args[0]) {
					// It's a subcommand, let cobra handle it
					return nil
				}
			}
			// Not a subcommand — try as project name
			database, err := db.Open(config.DBPath())
			if err == nil {
				defer database.Close()
				p, _ := database.GetProjectByName(args[0])
				if p != nil {
					showCmd.SetArgs(args)
					return showCmd.RunE(showCmd, args)
				}
			}
		}
		return smartSummary()
	},
}

func smartSummary() error {
	database, err := db.Open(config.DBPath())
	if err != nil {
		fmt.Println("Nexus not initialized. Run 'nexus init' to get started.")
		return nil
	}
	defer database.Close()

	dirty, _ := database.ListDirtyProjects()
	sessions, _ := database.ListSessions(db.SessionFilter{Limit: 5})
	stale, _ := database.ListProjects("stale")

	// Detect current project for context
	cwd, _ := os.Getwd()
	absDir, _ := filepath.Abs(cwd)
	currentProject := ""
	p, _ := database.GetProjectByPath(absDir)
	if p != nil {
		currentProject = p.Name
	}

	display.FormatSmartSummary(os.Stdout, dirty, sessions, stale, currentProject)
	return nil
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
	if log != nil {
		log.Close()
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Enable debug output to stderr")

	cobra.OnInitialize(func() {
		log = logger.New(logger.Config{
			Debug:   debug,
			LogFile: config.LogPath(),
		})
	})

	rootCmd.AddGroup(
		&cobra.Group{ID: "core", Title: "Core Commands:"},
		&cobra.Group{ID: "query", Title: "Query Commands:"},
		&cobra.Group{ID: "workflow", Title: "Workflow Commands:"},
		&cobra.Group{ID: "maintenance", Title: "Maintenance Commands:"},
	)
}
