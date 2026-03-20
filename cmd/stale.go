// cmd/stale.go
package cmd

import (
	"bufio"
	"fmt"
	"os"
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
		idle, _ := database.ListProjects("idle")
		staleProjs, _ := database.ListProjects("stale")
		allProjs := append(idle, staleProjs...)

		// Collect stale branches and dirty details
		branches := map[string][]display.StaleBranchInfo{}
		dirtyDetails := map[string][]string{}

		for _, p := range allProjs {
			if !scanner.IsGitRepo(p.Path) {
				continue
			}
			staleBranches, _ := scanner.GetStaleBranchesWithDates(p.Path, 7*24*time.Hour)
			if len(staleBranches) > 0 {
				var infos []display.StaleBranchInfo
				for _, b := range staleBranches {
					infos = append(infos, display.StaleBranchInfo{Name: b.Name, Age: display.RelativeTime(b.LastCommit)})
				}
				branches[p.Name] = infos
			}

			if p.Dirty {
				details, _ := scanner.GetDirtyFileDetails(p.Path)
				var lines []string
				for _, d := range details {
					lines = append(lines, fmt.Sprintf("%s %s", d.Status, d.Path))
				}
				dirtyDetails[p.Name] = lines
			}
		}

		// Also check active+dirty projects
		dirtyProjs, _ := database.ListDirtyProjects()
		for _, p := range dirtyProjs {
			if _, exists := dirtyDetails[p.Name]; exists {
				continue
			}
			details, _ := scanner.GetDirtyFileDetails(p.Path)
			var lines []string
			for _, d := range details {
				lines = append(lines, fmt.Sprintf("%s %s", d.Status, d.Path))
			}
			if len(lines) > 0 {
				dirtyDetails[p.Name] = lines
			}
		}

		if !staleCleanup {
			display.FormatStale(os.Stdout, allProjs, branches, dirtyDetails)
			return nil
		}

		// Interactive cleanup
		reader := bufio.NewReader(os.Stdin)
		for project, brs := range branches {
			// Find the project to get its path
			var projPath string
			for _, p := range allProjs {
				if p.Name == project {
					projPath = p.Path
					break
				}
			}
			if projPath == "" {
				continue
			}

			fmt.Printf("\n%s\n", project)
			for _, b := range brs {
				fmt.Printf("  %s  (last commit: %s)\n", b.Name, b.Age)
				fmt.Printf("  Delete? [y/n/q] ")
				input, _ := reader.ReadString('\n')
				if len(input) == 0 {
					continue
				}
				switch input[0] {
				case 'y', 'Y':
					err := scanner.DeleteBranch(projPath, b.Name)
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
