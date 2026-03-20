// cmd/hook.go
package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

const canonicalWrapper = `claude() { command claude "$@"; local rc=$?; nexus capture --dir "$PWD"; return $rc; }`

var hookCmd = &cobra.Command{
	Use:   "hook",
	Short: "Install or uninstall shell wrapper and cron job",
}

var hookInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install claude() wrapper and nexus scan cron job",
	RunE: func(cmd *cobra.Command, args []string) error {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("cannot determine home directory: %w", err)
		}
		bashrc := home + "/.bashrc"

		// Read .bashrc
		data, err := os.ReadFile(bashrc)
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("read .bashrc: %w", err)
		}
		content := string(data)

		// Check for existing claude() function
		if strings.Contains(content, "claude()") || strings.Contains(content, "claude ()") {
			if strings.Contains(content, canonicalWrapper) {
				fmt.Println("✓ Shell wrapper already installed")
			} else {
				fmt.Println("⚠ Existing claude() function found in .bashrc — not overwriting. Add manually:")
				fmt.Printf("\n  %s\n\n", canonicalWrapper)
			}
		} else {
			// Append wrapper
			f, err := os.OpenFile(bashrc, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				return fmt.Errorf("open .bashrc: %w", err)
			}
			fmt.Fprintf(f, "\n# Nexus: auto-capture Claude sessions\n%s\n", canonicalWrapper)
			if err := f.Close(); err != nil {
				return fmt.Errorf("write .bashrc: %w", err)
			}
			fmt.Println("✓ Added claude() wrapper to ~/.bashrc")
		}

		// Check cron
		cronOut, _ := exec.Command("crontab", "-l").Output()
		if strings.Contains(string(cronOut), "nexus scan") {
			fmt.Println("✓ Cron job already installed")
		} else {
			nexusPath, exeErr := os.Executable()
			if exeErr != nil {
				nexusPath = home + "/go/bin/nexus"
			}
			// Validate path has no newlines or shell metacharacters
			if strings.ContainsAny(nexusPath, "\n\r`$") {
				return fmt.Errorf("nexus binary path contains unsafe characters: %s", nexusPath)
			}
			nexusDir := home + "/.nexus"
			cronLine := fmt.Sprintf("*/30 * * * * %s scan >> %s/nexus.log 2>&1", nexusPath, nexusDir)

			newCron := string(cronOut) + cronLine + "\n"
			cronCmd := exec.Command("crontab", "-")
			cronCmd.Stdin = strings.NewReader(newCron)
			if err := cronCmd.Run(); err != nil {
				return fmt.Errorf("install cron: %w", err)
			}
			fmt.Println("✓ Installed cron job: nexus scan every 30 minutes")
		}

		fmt.Println("\nRun: source ~/.bashrc")
		return nil
	},
}

var hookUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove claude() wrapper and nexus scan cron job",
	RunE: func(cmd *cobra.Command, args []string) error {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("cannot determine home directory: %w", err)
		}
		bashrc := home + "/.bashrc"

		// Remove wrapper from .bashrc
		data, err := os.ReadFile(bashrc)
		if err == nil {
			lines := strings.Split(string(data), "\n")
			var filtered []string
			for _, line := range lines {
				if strings.Contains(line, "nexus capture") || strings.Contains(line, "# Nexus: auto-capture") {
					continue
				}
				filtered = append(filtered, line)
			}
			if err := os.WriteFile(bashrc, []byte(strings.Join(filtered, "\n")), 0644); err != nil {
				return fmt.Errorf("write .bashrc: %w", err)
			}
			fmt.Println("✓ Removed claude() wrapper from ~/.bashrc")
		}

		// Remove cron line
		cronOut, _ := exec.Command("crontab", "-l").Output()
		if strings.Contains(string(cronOut), "nexus scan") {
			lines := strings.Split(string(cronOut), "\n")
			var filtered []string
			for _, line := range lines {
				if !strings.Contains(line, "nexus scan") {
					filtered = append(filtered, line)
				}
			}
			cronCmd := exec.Command("crontab", "-")
			cronCmd.Stdin = strings.NewReader(strings.Join(filtered, "\n"))
			if err := cronCmd.Run(); err != nil {
				return fmt.Errorf("remove cron: %w", err)
			}
			fmt.Println("✓ Removed nexus scan cron job")
		}

		return nil
	},
}

func init() {
	hookCmd.AddCommand(hookInstallCmd)
	hookCmd.AddCommand(hookUninstallCmd)
	hookCmd.GroupID = "maintenance"
	rootCmd.AddCommand(hookCmd)
}
