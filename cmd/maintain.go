package cmd

import (
	"fmt"

	"github.com/digitalghost404/nexus/internal/db"
	"github.com/spf13/cobra"
)

var maintainCmd = &cobra.Command{
	Use:   "maintain",
	Short: "Run maintenance tasks (decay, prune, vacuum)",
	Long:  "Applies confidence decay, removes low-confidence and superseded preferences, and vacuums the database.",
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
