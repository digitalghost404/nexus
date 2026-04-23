package cmd

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/digitalghost404/nexus/internal/db"
	"github.com/spf13/cobra"
)

func parseIntEnv(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

var maintainCmd = &cobra.Command{
	Use:   "maintain",
	Short: "Run maintenance tasks (decay, prune, vacuum)",
	Long:  "Applies confidence decay, removes low-confidence and superseded preferences, prunes old data, and vacuums the database.",
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := db.Open(cfg.DBPath())
		if err != nil {
			return err
		}
		defer func() { _ = database.Close() }()

		if err := database.DecayPreferences(); err != nil {
			return fmt.Errorf("decay preferences: %w", err)
		}
		fmt.Println("Applied confidence decay.")

		deleted, err := database.DeleteLowConfidencePreferences(0.15)
		if err != nil {
			return fmt.Errorf("delete low-confidence preferences: %w", err)
		}
		fmt.Printf("Deleted %d low-confidence inferred preferences.\n", deleted)

		deleted, err = database.DeleteContradictingInferredPreferences()
		if err != nil {
			return fmt.Errorf("delete contradicting preferences: %w", err)
		}
		fmt.Printf("Deleted %d contradicting inferred preferences.\n", deleted)

		deleted, err = database.DeleteSupersededPreferences(30)
		if err != nil {
			return fmt.Errorf("delete superseded preferences: %w", err)
		}
		fmt.Printf("Deleted %d superseded preferences older than 30 days.\n", deleted)

		sessionsDays := parseIntEnv("NEXUS_RETENTION_SESSIONS_DAYS", 0)
		if sessionsDays > 0 {
			before := time.Now().AddDate(0, 0, -sessionsDays)
			deleted, err := database.PruneSessions(before)
			if err != nil {
				return fmt.Errorf("prune sessions: %w", err)
			}
			fmt.Printf("Pruned %d sessions older than %d days.\n", deleted, sessionsDays)
		}

		notesDays := parseIntEnv("NEXUS_RETENTION_NOTES_DAYS", 0)
		if notesDays > 0 {
			before := time.Now().AddDate(0, 0, -notesDays)
			deleted, err := database.PruneNotes(before)
			if err != nil {
				return fmt.Errorf("prune notes: %w", err)
			}
			fmt.Printf("Pruned %d notes older than %d days.\n", deleted, notesDays)
		}

		prefsDays := parseIntEnv("NEXUS_RETENTION_PREFERENCES_DAYS", 0)
		if prefsDays > 0 {
			before := time.Now().AddDate(0, 0, -prefsDays)
			deleted, err := database.PrunePreferences(before)
			if err != nil {
				return fmt.Errorf("prune preferences: %w", err)
			}
			fmt.Printf("Pruned %d preferences older than %d days.\n", deleted, prefsDays)
		}

		if _, err := database.Conn().Exec("VACUUM"); err != nil {
			return fmt.Errorf("vacuum: %w", err)
		}
		fmt.Println("Database vacuumed.")

		return nil
	},
}

func init() {
	maintainCmd.GroupID = "maintenance"
	rootCmd.AddCommand(maintainCmd)
}
