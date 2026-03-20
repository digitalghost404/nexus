// cmd/config.go
package cmd

import (
	"fmt"

	"github.com/digitalghost404/nexus/internal/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage Nexus configuration (roots, exclusions)",
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(config.ConfigPath())
		if err != nil {
			return err
		}
		data, _ := yaml.Marshal(cfg)
		fmt.Println(string(data))
		return nil
	},
}

var configRootsCmd = &cobra.Command{
	Use:   "roots",
	Short: "Manage scan roots",
}

var configRootsAddCmd = &cobra.Command{
	Use:   "add <path>",
	Short: "Add a scan root",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfgPath := config.ConfigPath()
		cfg, err := config.Load(cfgPath)
		if err != nil {
			return err
		}

		expanded := config.ExpandPath(args[0])
		for _, r := range cfg.Roots {
			if r == expanded {
				fmt.Printf("Root already exists: %s\n", expanded)
				return nil
			}
		}

		cfg.Roots = append(cfg.Roots, expanded)
		if err := config.Save(cfgPath, cfg); err != nil {
			return err
		}
		fmt.Printf("Added root: %s\n", expanded)
		return nil
	},
}

var configExcludeCmd = &cobra.Command{
	Use:   "exclude",
	Short: "Manage exclusion patterns",
}

var configExcludeAddCmd = &cobra.Command{
	Use:   "add <pattern>",
	Short: "Add an exclusion pattern",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfgPath := config.ConfigPath()
		cfg, err := config.Load(cfgPath)
		if err != nil {
			return err
		}

		cfg.Exclude = append(cfg.Exclude, args[0])
		if err := config.Save(cfgPath, cfg); err != nil {
			return err
		}
		fmt.Printf("Added exclusion: %s\n", args[0])
		return nil
	},
}

func init() {
	configCmd.GroupID = "maintenance"
	configRootsCmd.AddCommand(configRootsAddCmd)
	configExcludeCmd.AddCommand(configExcludeAddCmd)
	configCmd.AddCommand(configShowCmd, configRootsCmd, configExcludeCmd)
	rootCmd.AddCommand(configCmd)
}
