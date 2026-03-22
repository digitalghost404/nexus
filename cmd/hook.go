// cmd/hook.go
package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

var safePathRe = regexp.MustCompile(`^[a-zA-Z0-9/_.\-]+$`)

const canonicalWrapper = `claude() { command claude "$@"; local rc=$?; nexus capture --dir "$PWD"; return $rc; }`

const fishFunction = `function claude
    command claude $argv
    set -l rc $status
    nexus capture --dir $PWD
    return $rc
end
`

// detectShell returns "fish", "zsh", or "bash" based on the $SHELL environment variable.
func detectShell() string {
	shell := os.Getenv("SHELL")
	base := filepath.Base(shell)
	switch base {
	case "fish":
		return "fish"
	case "zsh":
		return "zsh"
	default:
		return "bash"
	}
}

// installCron installs a crontab entry for nexus scan. Returns an error if
// crontab is not available or the install fails.
func installCron(nexusPath, logDir string) error {
	if _, err := exec.LookPath("crontab"); err != nil {
		return fmt.Errorf("crontab not found: %w", err)
	}

	cronOut, _ := exec.Command("crontab", "-l").Output()
	if strings.Contains(string(cronOut), "nexus scan") {
		fmt.Println("✓ Cron job already installed")
		return nil
	}

	cronLine := fmt.Sprintf("*/30 * * * * %s scan >> %s/nexus.log 2>&1", nexusPath, logDir)
	newCron := string(cronOut) + cronLine + "\n"
	cronCmd := exec.Command("crontab", "-")
	cronCmd.Stdin = strings.NewReader(newCron)
	if err := cronCmd.Run(); err != nil {
		return fmt.Errorf("install cron: %w", err)
	}
	fmt.Println("✓ Installed cron job: nexus scan every 30 minutes")
	return nil
}

// installSystemdTimer installs a systemd user timer for nexus scan. Returns an
// error if systemctl is not available or the install fails.
func installSystemdTimer(nexusPath, logDir string) error {
	if _, err := exec.LookPath("systemctl"); err != nil {
		return fmt.Errorf("systemctl not found: %w", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	unitDir := home + "/.config/systemd/user"
	if err := os.MkdirAll(unitDir, 0755); err != nil {
		return fmt.Errorf("create systemd user dir: %w", err)
	}

	serviceContent := fmt.Sprintf(`[Unit]
Description=Nexus project scanner

[Service]
Type=oneshot
ExecStart=%s scan
StandardOutput=append:%s/nexus.log
StandardError=append:%s/nexus.log
`, nexusPath, logDir, logDir)

	timerContent := `[Unit]
Description=Run nexus scan every 30 minutes

[Timer]
OnBootSec=5min
OnUnitActiveSec=30min

[Install]
WantedBy=timers.target
`

	servicePath := unitDir + "/nexus-scan.service"
	timerPath := unitDir + "/nexus-scan.timer"

	if err := os.WriteFile(servicePath, []byte(serviceContent), 0644); err != nil {
		return fmt.Errorf("write nexus-scan.service: %w", err)
	}
	if err := os.WriteFile(timerPath, []byte(timerContent), 0644); err != nil {
		return fmt.Errorf("write nexus-scan.timer: %w", err)
	}

	if err := exec.Command("systemctl", "--user", "daemon-reload").Run(); err != nil {
		return fmt.Errorf("systemctl daemon-reload: %w", err)
	}
	if err := exec.Command("systemctl", "--user", "enable", "--now", "nexus-scan.timer").Run(); err != nil {
		return fmt.Errorf("systemctl enable nexus-scan.timer: %w", err)
	}

	fmt.Println("✓ Installed systemd user timer: nexus scan every 30 minutes")
	return nil
}

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

		shell := detectShell()

		switch shell {
		case "fish":
			fishFuncDir := home + "/.config/fish/functions"
			if err := os.MkdirAll(fishFuncDir, 0755); err != nil {
				return fmt.Errorf("create fish functions dir: %w", err)
			}
			fishFuncPath := fishFuncDir + "/claude.fish"
			existing, readErr := os.ReadFile(fishFuncPath)
			if readErr == nil && strings.Contains(string(existing), "nexus capture") {
				fmt.Println("✓ Fish shell wrapper already installed")
			} else {
				if err := os.WriteFile(fishFuncPath, []byte(fishFunction), 0644); err != nil {
					return fmt.Errorf("write claude.fish: %w", err)
				}
				fmt.Println("✓ Added claude function to ~/.config/fish/functions/claude.fish")
			}

		case "zsh":
			rcFile := home + "/.zshrc"
			data, err := os.ReadFile(rcFile)
			if err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("read .zshrc: %w", err)
			}
			content := string(data)
			if strings.Contains(content, "claude()") || strings.Contains(content, "claude ()") {
				if strings.Contains(content, canonicalWrapper) {
					fmt.Println("✓ Shell wrapper already installed")
				} else {
					fmt.Println("⚠ Existing claude() function found in .zshrc — not overwriting. Add manually:")
					fmt.Printf("\n  %s\n\n", canonicalWrapper)
				}
			} else {
				f, err := os.OpenFile(rcFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
				if err != nil {
					return fmt.Errorf("open .zshrc: %w", err)
				}
				fmt.Fprintf(f, "\n# Nexus: auto-capture Claude sessions\n%s\n", canonicalWrapper)
				if err := f.Close(); err != nil {
					return fmt.Errorf("write .zshrc: %w", err)
				}
				fmt.Println("✓ Added claude() wrapper to ~/.zshrc")
			}

		default: // bash
			bashrc := home + "/.bashrc"
			data, err := os.ReadFile(bashrc)
			if err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("read .bashrc: %w", err)
			}
			content := string(data)
			if strings.Contains(content, "claude()") || strings.Contains(content, "claude ()") {
				if strings.Contains(content, canonicalWrapper) {
					fmt.Println("✓ Shell wrapper already installed")
				} else {
					fmt.Println("⚠ Existing claude() function found in .bashrc — not overwriting. Add manually:")
					fmt.Printf("\n  %s\n\n", canonicalWrapper)
				}
			} else {
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
		}

		// Periodic scanning: try cron, fall back to systemd, then warn
		nexusPath, exeErr := os.Executable()
		if exeErr != nil {
			nexusPath = home + "/go/bin/nexus"
		}
		// Validate path contains only safe characters
		if !safePathRe.MatchString(nexusPath) {
			return fmt.Errorf("nexus binary path contains unsafe characters: %s", nexusPath)
		}
		nexusDir := home + "/.nexus"

		if err := installCron(nexusPath, nexusDir); err != nil {
			if err := installSystemdTimer(nexusPath, nexusDir); err != nil {
				fmt.Println("⚠ Could not install automatic scanning (no crontab or systemd).")
				fmt.Printf("  Run manually: %s scan\n", nexusPath)
				fmt.Printf("  Or add a cron job: */30 * * * * %s scan >> %s/nexus.log 2>&1\n", nexusPath, nexusDir)
			}
		}

		switch shell {
		case "fish":
			fmt.Println("\nNo reload needed — fish loads functions automatically.")
		case "zsh":
			fmt.Println("\nRun: source ~/.zshrc")
		default:
			fmt.Println("\nRun: source ~/.bashrc")
		}

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

		// Clean both .bashrc and .zshrc
		for _, rcName := range []string{".bashrc", ".zshrc"} {
			rcFile := home + "/" + rcName
			info, statErr := os.Stat(rcFile)
			data, readErr := os.ReadFile(rcFile)
			if readErr == nil {
				lines := strings.Split(string(data), "\n")
				var filtered []string
				for _, line := range lines {
					if strings.Contains(line, "nexus capture") || strings.Contains(line, "# Nexus: auto-capture") {
						continue
					}
					filtered = append(filtered, line)
				}
				perm := os.FileMode(0644)
				if statErr == nil {
					perm = info.Mode().Perm()
				}
				if err := os.WriteFile(rcFile, []byte(strings.Join(filtered, "\n")), perm); err != nil {
					return fmt.Errorf("write %s: %w", rcName, err)
				}
				fmt.Printf("✓ Removed claude() wrapper from ~/%s\n", rcName)
			}
		}

		// Remove fish function if present
		fishFuncPath := home + "/.config/fish/functions/claude.fish"
		if err := os.Remove(fishFuncPath); err == nil {
			fmt.Println("✓ Removed ~/.config/fish/functions/claude.fish")
		}

		// Remove cron line if crontab is available
		if _, lookErr := exec.LookPath("crontab"); lookErr == nil {
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
		}

		// Remove systemd timer/service if present
		unitDir := home + "/.config/systemd/user"
		servicePath := unitDir + "/nexus-scan.service"
		timerPath := unitDir + "/nexus-scan.timer"

		// Disable timer first (ignore errors — may not be running)
		_ = exec.Command("systemctl", "--user", "disable", "--now", "nexus-scan.timer").Run()

		removedSystemd := false
		if err := os.Remove(servicePath); err == nil {
			removedSystemd = true
		}
		if err := os.Remove(timerPath); err == nil {
			removedSystemd = true
		}
		if removedSystemd {
			fmt.Println("✓ Removed systemd nexus-scan timer")
			_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
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
