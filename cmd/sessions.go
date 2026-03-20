// cmd/sessions.go
package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/digitalghost404/nexus/internal/config"
	"github.com/digitalghost404/nexus/internal/db"
	"github.com/digitalghost404/nexus/internal/display"
	"github.com/spf13/cobra"
)

var (
	sessionsProject string
	sessionsSince   string
	sessionsToday   bool
)

var sessionsCmd = &cobra.Command{
	Use:   "sessions [project]",
	Short: "List Claude session history",
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := db.Open(config.DBPath())
		if err != nil {
			return err
		}
		defer database.Close()

		filter := db.SessionFilter{Limit: 10}

		// Handle positional project arg
		project := sessionsProject
		if project == "" && len(args) > 0 {
			project = args[0]
		}

		if project != "" {
			p, err := database.GetProjectByName(project)
			if err != nil {
				return err
			}
			if p == nil {
				return fmt.Errorf("project not found: %s", project)
			}
			filter.ProjectID = p.ID
		}

		if sessionsToday {
			now := time.Now()
			today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
			filter.Since = &today
		} else if sessionsSince != "" {
			since, err := parseDuration(sessionsSince)
			if err != nil {
				return fmt.Errorf("invalid --since: %w", err)
			}
			filter.Since = since
		}

		sessions, err := database.ListSessions(filter)
		if err != nil {
			return err
		}

		display.FormatSessionList(os.Stdout, sessions)
		return nil
	},
}

func parseDuration(s string) (*time.Time, error) {
	// Parse "7d", "24h", "30m" etc
	if len(s) < 2 {
		return nil, fmt.Errorf("invalid duration: %s", s)
	}
	unit := s[len(s)-1]
	var n int
	fmt.Sscanf(s[:len(s)-1], "%d", &n)

	var d time.Duration
	switch unit {
	case 'd':
		d = time.Duration(n) * 24 * time.Hour
	case 'h':
		d = time.Duration(n) * time.Hour
	case 'm':
		d = time.Duration(n) * time.Minute
	default:
		return nil, fmt.Errorf("unknown unit: %c", unit)
	}

	t := time.Now().Add(-d)
	return &t, nil
}

func init() {
	sessionsCmd.Flags().StringVar(&sessionsProject, "project", "", "Filter by project")
	sessionsCmd.Flags().StringVar(&sessionsSince, "since", "", "Show sessions since duration (e.g. 7d)")
	sessionsCmd.Flags().BoolVar(&sessionsToday, "today", false, "Show today's sessions only")
	rootCmd.AddCommand(sessionsCmd)
}
