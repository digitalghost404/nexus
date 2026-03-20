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

var (
	debug bool
	log   *logger.Logger
)

// subcommands that take precedence over project name routing
var subcommands = map[string]bool{
	"init": true, "scan": true, "capture": true, "projects": true,
	"sessions": true, "search": true, "show": true, "note": true,
	"config": true, "help": true,
}

var rootCmd = &cobra.Command{
	Use:   "nexus",
	Short: "Track project health and Claude sessions",
	Long:  "Nexus gives you a single pane of glass into all your projects and Claude Code sessions.",
	Args:  cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Route to show if first arg is a known project name (not a subcommand)
		if len(args) > 0 && !subcommands[args[0]] {
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

		// Smart summary
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
}
