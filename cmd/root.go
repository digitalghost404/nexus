package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var debug bool

var rootCmd = &cobra.Command{
	Use:   "nexus",
	Short: "Track project health and Claude sessions",
	Long:  "Nexus gives you a single pane of glass into all your projects and Claude Code sessions.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			// TODO: check if args[0] is a project name and route to show
		}
		// TODO: smart summary
		fmt.Println("Nexus — run 'nexus init' to get started")
		return nil
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Enable debug output to stderr")
}
