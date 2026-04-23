package cmd

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/digitalghost404/nexus/internal/db"
	"github.com/spf13/cobra"
	_ "modernc.org/sqlite"
)

func init() {
	rootCmd.AddCommand(newBackupCmd())
}

func newBackupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Backup and restore nexus database",
	}
	cmd.AddCommand(newBackupRunCmd())
	cmd.AddCommand(newBackupRestoreCmd())
	return cmd
}

func newBackupRunCmd() *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Create a database backup",
		RunE: func(cmd *cobra.Command, args []string) error {
			database, err := db.Open(cfg.DBPath())
			if err != nil {
				return err
			}
			defer func() { _ = database.Close() }()

			dbPath := cfg.DBPath()
			if output == "" {
				backupDir := filepath.Join(cfg.NexusDir(), "backups")
				if err := os.MkdirAll(backupDir, 0700); err != nil {
					return fmt.Errorf("create backup dir: %w", err)
				}
				output = filepath.Join(backupDir, fmt.Sprintf("nexus-backup-%s.db", time.Now().Format("20060102-150405")))
			}

			srcDB, err := sql.Open("sqlite", dbPath)
			if err != nil {
				return fmt.Errorf("open source db: %w", err)
			}
			defer func() { _ = srcDB.Close() }()

			if _, err := srcDB.Exec(fmt.Sprintf("VACUUM INTO '%s'", output)); err != nil {
				return fmt.Errorf("backup database: %w", err)
			}

			fmt.Printf("Backup created: %s\n", output)
			return nil
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "", "output file path")
	return cmd
}

func newBackupRestoreCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "restore <path>",
		Short: "Restore database from backup",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			backupPath := args[0]
			if _, err := os.Stat(backupPath); err != nil {
				return fmt.Errorf("backup file not found: %w", err)
			}

			testDB, err := sql.Open("sqlite", backupPath)
			if err != nil {
				return fmt.Errorf("open backup: %w", err)
			}
			defer func() { _ = testDB.Close() }()

			if err := testDB.Ping(); err != nil {
				return fmt.Errorf("invalid backup file: %w", err)
			}

			dbPath := cfg.DBPath()

			safetyPath := fmt.Sprintf("%s.pre-restore-%s.db", dbPath, time.Now().Format("20060102-150405"))
			srcDB, err := sql.Open("sqlite", dbPath)
			if err == nil {
				_, _ = srcDB.Exec(fmt.Sprintf("VACUUM INTO '%s'", safetyPath))
				_ = srcDB.Close()
				fmt.Printf("Current database backed up to: %s\n", safetyPath)
			}

			data, err := os.ReadFile(backupPath)
			if err != nil {
				return fmt.Errorf("read backup: %w", err)
			}
			if err := os.WriteFile(dbPath, data, 0600); err != nil {
				return fmt.Errorf("restore database: %w", err)
			}

			fmt.Println("Database restored successfully.")
			return nil
		},
	}
}
