// cmd/deps.go
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/digitalghost404/nexus/internal/config"
	"github.com/digitalghost404/nexus/internal/db"
	"github.com/digitalghost404/nexus/internal/display"
	"github.com/spf13/cobra"
)

var depsProject string

var depsCmd = &cobra.Command{
	Use:   "deps",
	Short: "Check for outdated dependencies",
	Long:  "Scans all tracked projects for outdated Go, npm, and pip dependencies.",
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := db.Open(config.DBPath())
		if err != nil {
			return err
		}
		defer database.Close()

		var projects []db.Project
		if depsProject != "" {
			p, _ := database.GetProjectByName(depsProject)
			if p == nil {
				return fmt.Errorf("project not found: %s", depsProject)
			}
			projects = []db.Project{*p}
		} else {
			projects, _ = database.ListProjects("")
		}

		hasGo, _ := exec.LookPath("go")
		hasNpm, _ := exec.LookPath("npm")
		hasPip, _ := exec.LookPath("pip3")

		var results []display.ProjectDeps
		cleanCount := 0

		for _, p := range projects {
			fmt.Fprintf(os.Stderr, "Checking %s...\n", p.Name)
			var outdated []display.DepInfo
			var manager string

			goMod := filepath.Join(p.Path, "go.mod")
			pkgJSON := filepath.Join(p.Path, "package.json")
			reqTxt := filepath.Join(p.Path, "requirements.txt")

			if _, err := os.Stat(goMod); err == nil && hasGo != "" {
				manager = "Go"
				outdated = checkGoDeps(p.Path)
			} else if _, err := os.Stat(pkgJSON); err == nil && hasNpm != "" {
				manager = "npm"
				outdated = checkNpmDeps(p.Path)
			} else if _, err := os.Stat(reqTxt); err == nil && hasPip != "" {
				manager = "pip"
				outdated = checkPipDeps(p.Path)
			}

			if len(outdated) > 0 {
				results = append(results, display.ProjectDeps{
					ProjectName: p.Name,
					Manager:     manager,
					Outdated:    outdated,
				})
			} else if manager != "" {
				cleanCount++
			}
		}

		display.FormatDeps(os.Stdout, results, cleanCount)
		return nil
	},
}

func checkGoDeps(dir string) []display.DepInfo {
	cmd := exec.Command("go", "list", "-m", "-u", "-json", "all")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	var deps []display.DepInfo
	dec := json.NewDecoder(strings.NewReader(string(out)))
	for dec.More() {
		var m struct {
			Path     string
			Version  string
			Update   *struct{ Version string }
			Main     bool
			Indirect bool
		}
		if err := dec.Decode(&m); err != nil {
			break
		}
		if m.Update != nil && !m.Main {
			deps = append(deps, display.DepInfo{
				Name:      m.Path,
				Current:   m.Version,
				Available: m.Update.Version,
			})
		}
	}
	return deps
}

func checkNpmDeps(dir string) []display.DepInfo {
	cmd := exec.Command("npm", "outdated", "--json")
	cmd.Dir = dir
	out, _ := cmd.Output() // npm outdated exits 1 when outdated packages exist

	var result map[string]struct {
		Current string `json:"current"`
		Latest  string `json:"latest"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return nil
	}

	var deps []display.DepInfo
	for name, info := range result {
		if info.Current != info.Latest {
			deps = append(deps, display.DepInfo{
				Name:      name,
				Current:   info.Current,
				Available: info.Latest,
			})
		}
	}
	return deps
}

func checkPipDeps(dir string) []display.DepInfo {
	cmd := exec.Command("pip3", "list", "--outdated", "--format=json")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	var result []struct {
		Name          string `json:"name"`
		Version       string `json:"version"`
		LatestVersion string `json:"latest_version"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return nil
	}

	var deps []display.DepInfo
	for _, p := range result {
		deps = append(deps, display.DepInfo{
			Name:      p.Name,
			Current:   p.Version,
			Available: p.LatestVersion,
		})
	}
	return deps
}

func init() {
	depsCmd.Flags().StringVar(&depsProject, "project", "", "Check a single project")
	depsCmd.GroupID = "maintenance"
	rootCmd.AddCommand(depsCmd)
}
