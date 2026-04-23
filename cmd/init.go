// cmd/init.go
package cmd

import (
	"fmt"
	"os"

	"github.com/digitalghost404/nexus/internal/config"
	"github.com/digitalghost404/nexus/internal/db"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize Nexus (~/.nexus/ setup, first scan)",
	RunE: func(cmd *cobra.Command, args []string) error {
		nexusDir := cfg.NexusDir()

		// Create directory
		if err := os.MkdirAll(nexusDir, 0700); err != nil {
			return fmt.Errorf("create nexus dir: %w", err)
		}
		fmt.Printf("Created %s\n", nexusDir)

		// Create database
		database, err := db.Open(cfg.DBPath())
		if err != nil {
			return fmt.Errorf("init db: %w", err)
		}
		_ = database.Close()
		fmt.Printf("Created database at %s\n", cfg.DBPath())

		// Create default config if missing
		cfgPath := cfg.ConfigPath()
		if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
			defaultCfg := config.Default()
			defaultCfg.Roots = []string{}
			if err := config.Save(cfgPath, defaultCfg); err != nil {
				return fmt.Errorf("save config: %w", err)
			}
			fmt.Printf("Created config at %s\n", cfgPath)
			fmt.Println("No scan roots configured. Add one with: nexus config roots add ~/your-projects")
		}

		// Print shell wrapper instructions
		fmt.Println("\n── Shell Wrapper ──────────────────────────────")
		fmt.Println("Add this to your ~/.bashrc to auto-capture Claude sessions:")
		fmt.Println()
		fmt.Println(`  claude() { command claude "$@"; local rc=$?; nexus capture --dir "$PWD"; return $rc; }`)
		fmt.Println()
		fmt.Println("Then run: source ~/.bashrc")

		// Print cron instructions
		loadedCfg, _ := config.Load(cfgPath)
		fmt.Println("\n── Periodic Scan ─────────────────────────────")
		fmt.Printf("Add this cron job to run scans every %s:\n\n", loadedCfg.ScanInterval)
		fmt.Printf("  %s %s/go/bin/nexus scan >> %s/nexus.log 2>&1\n",
			cronExpr(loadedCfg.ScanInterval), os.Getenv("HOME"), nexusDir)
		fmt.Println()

		// Run initial scan
		fmt.Println("Running initial scan...")
		return runScan(loadedCfg, false)
	},
}

func cronExpr(interval string) string {
	if len(interval) < 2 {
		return "*/30 * * * *"
	}
	unit := interval[len(interval)-1]
	num := interval[:len(interval)-1]
	switch unit {
	case 'm':
		return fmt.Sprintf("*/%s * * * *", num)
	case 'h':
		return fmt.Sprintf("0 */%s * * *", num)
	default:
		return "*/30 * * * *"
	}
}

func init() {
	initCmd.GroupID = "core"
	rootCmd.AddCommand(initCmd)
}
