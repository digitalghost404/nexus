// cmd/stale.go
package cmd

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/digitalghost404/nexus/internal/config"
	"github.com/digitalghost404/nexus/internal/db"
	"github.com/digitalghost404/nexus/internal/display"
	"github.com/digitalghost404/nexus/internal/scanner"
	"github.com/spf13/cobra"
)

var staleCleanup bool

var staleCmd = &cobra.Command{
	Use:   "stale",
	Short: "Show stale branches and idle projects",
	Long:  "Lists stale branches and dirty projects. Use --cleanup for interactive branch deletion.",
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := db.Open(config.DBPath())
		if err != nil {
			return err
		}
		defer database.Close()

		// Get idle and stale projects
		idle, err := database.ListProjects("idle")
		if err != nil {
			return err
		}
		staleProjs, err := database.ListProjects("stale")
		if err != nil {
			return err
		}
		allProjs := append(idle, staleProjs...)

		// Collect stale branches and dirty details
		// Use path as key internally to avoid name collisions, display with name
		type projectInfo struct {
			name string
			path string
		}
		branchesByPath := map[string][]display.StaleBranchInfo{}
		dirtyDetailsByPath := map[string][]string{}
		projectsByPath := map[string]projectInfo{}

		for _, p := range allProjs {
			projectsByPath[p.Path] = projectInfo{name: p.Name, path: p.Path}
			if !scanner.IsGitRepo(p.Path) {
				continue
			}
			staleBranches, _ := scanner.GetStaleBranchesWithDates(p.Path, 7*24*time.Hour)
			if len(staleBranches) > 0 {
				var infos []display.StaleBranchInfo
				for _, b := range staleBranches {
					infos = append(infos, display.StaleBranchInfo{Name: b.Name, Age: display.RelativeTime(b.LastCommit)})
				}
				branchesByPath[p.Path] = infos
			}

			if p.Dirty {
				details, _ := scanner.GetDirtyFileDetails(p.Path)
				var lines []string
				for _, d := range details {
					lines = append(lines, fmt.Sprintf("%s %s", d.Status, d.Path))
				}
				dirtyDetailsByPath[p.Path] = lines
			}
		}

		// Also check active+dirty projects
		dirtyProjs, _ := database.ListDirtyProjects()
		for _, p := range dirtyProjs {
			if _, exists := dirtyDetailsByPath[p.Path]; exists {
				continue
			}
			details, _ := scanner.GetDirtyFileDetails(p.Path)
			var lines []string
			for _, d := range details {
				lines = append(lines, fmt.Sprintf("%s %s", d.Status, d.Path))
			}
			if len(lines) > 0 {
				dirtyDetailsByPath[p.Path] = lines
				projectsByPath[p.Path] = projectInfo{name: p.Name, path: p.Path}
			}
		}

		// Convert to name-keyed maps for display
		branches := map[string][]display.StaleBranchInfo{}
		for path, brs := range branchesByPath {
			branches[projectsByPath[path].name] = brs
		}
		dirtyDetails := map[string][]string{}
		for path, details := range dirtyDetailsByPath {
			dirtyDetails[projectsByPath[path].name] = details
		}

		if !staleCleanup {
			display.FormatStale(os.Stdout, branches, dirtyDetails)
			return nil
		}

		// Interactive cleanup — sort for deterministic order
		var sortedPaths []string
		for p := range branchesByPath {
			sortedPaths = append(sortedPaths, p)
		}
		sort.Strings(sortedPaths)

		reader := bufio.NewReader(os.Stdin)
		for _, projPath := range sortedPaths {
			brs := branchesByPath[projPath]
			info := projectsByPath[projPath]

			fmt.Printf("\n%s\n", info.name)
			for _, b := range brs {
				fmt.Printf("  %s  (last commit: %s)\n", b.Name, b.Age)
				fmt.Printf("  Delete? [y/n/q] ")
				input, _ := reader.ReadString('\n')
				if len(input) == 0 {
					continue
				}
				switch input[0] {
				case 'y', 'Y':
					err := scanner.DeleteBranch(info.path, b.Name)
					if err != nil {
						fmt.Printf("  ✗ Failed: %v\n", err)
					} else {
						fmt.Printf("  ✓ Deleted %s\n", b.Name)
					}
				case 'q', 'Q':
					fmt.Println("  Quitting cleanup.")
					return nil
				default:
					fmt.Println("  ⏭ Skipped")
				}
			}
		}

		// Show dirty projects (no auto-cleanup)
		if len(dirtyDetails) > 0 {
			fmt.Println("\nDirty projects (uncommitted changes):")
			for project, files := range dirtyDetails {
				fmt.Printf("\n  %s\n", project)
				for _, f := range files {
					fmt.Printf("    %s\n", f)
				}
				fmt.Println("    ⚠ Has uncommitted changes — review manually")
			}
		}

		return nil
	},
}

func init() {
	staleCmd.Flags().BoolVar(&staleCleanup, "cleanup", false, "Interactive branch cleanup")
	staleCmd.GroupID = "maintenance"
	rootCmd.AddCommand(staleCmd)
}
