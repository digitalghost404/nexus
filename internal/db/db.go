package db

import (
	"database/sql"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

//go:embed migration_v2.sql
var migrationV2SQL string

//go:embed migration_v3.sql
var migrationV3SQL string

//go:embed migration_v4.sql
var migrationV4SQL string

//go:embed migration_v5.sql
var migrationV5SQL string

//go:embed migration_v6.sql
var migrationV6SQL string

type DB struct {
	db *sql.DB
}

func Open(path string) (*DB, error) {
	migrateLegacyData(path)

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	sqlDB, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	if _, err := sqlDB.Exec("PRAGMA journal_mode=WAL"); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}

	d := &DB{db: sqlDB}

	if err := d.migrate(); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return d, nil
}

type migration struct {
	version int
	fn      func(*sql.Tx) error
}

var migrations = []migration{
	{1, migrateV1},
	{2, migrateV2},
	{3, migrateV3},
	{4, migrateV4},
	{5, migrateV5},
	{6, migrateV6},
}

func migrateV1(tx *sql.Tx) error {
	_, err := tx.Exec(schemaSQL)
	return err
}

func migrateV2(tx *sql.Tx) error {
	_, err := tx.Exec(migrationV2SQL)
	return err
}

func migrateV3(tx *sql.Tx) error {
	_, err := tx.Exec(migrationV3SQL)
	return err
}

func migrateV4(tx *sql.Tx) error {
	_, err := tx.Exec(migrationV4SQL)
	return err
}

func migrateV5(tx *sql.Tx) error {
	_, err := tx.Exec(migrationV5SQL)
	return err
}

func migrateV6(tx *sql.Tx) error {
	_, err := tx.Exec(migrationV6SQL)
	return err
}

func (d *DB) migrate() error {
	var version int
	if err := d.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}

	const currentVersion = 6
	if version > currentVersion {
		return fmt.Errorf("database schema version %d is newer than supported version %d — upgrade nexus", version, currentVersion)
	}

	for _, m := range migrations {
		if m.version <= version {
			continue
		}
		tx, err := d.db.Begin()
		if err != nil {
			return fmt.Errorf("begin migration %d: %w", m.version, err)
		}
		if err := m.fn(tx); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migration %d: %w", m.version, err)
		}
		if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", m.version)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("set version after migration %d: %w", m.version, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %d: %w", m.version, err)
		}
		version = m.version
	}

	return nil
}

func (d *DB) Close() error {
	return d.db.Close()
}

func (d *DB) Conn() *sql.DB {
	return d.db
}

func migrateLegacyData(newPath string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	legacyDir := filepath.Join(home, ".nexus")
	legacyDB := filepath.Join(legacyDir, "nexus.db")
	legacyCfg := filepath.Join(legacyDir, "config.yaml")
	legacyLog := filepath.Join(legacyDir, "nexus.log")

	newDir := filepath.Dir(newPath)

	legacyExists := fileExists(legacyDB) || fileExists(legacyCfg) || fileExists(legacyLog)
	if !legacyExists {
		return
	}
	if fileExists(newPath) || fileExists(filepath.Join(newDir, "config.yaml")) || fileExists(filepath.Join(newDir, "nexus.log")) {
		return
	}

	if err := os.MkdirAll(newDir, 0700); err != nil {
		return
	}

	_ = moveFile(legacyDB, filepath.Join(newDir, "nexus.db"))
	_ = moveFile(legacyCfg, filepath.Join(newDir, "config.yaml"))
	_ = moveFile(legacyLog, filepath.Join(newDir, "nexus.log"))
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func moveFile(src, dst string) error {
	if !fileExists(src) {
		return nil
	}
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	// Fallback for cross-device moves: copy then delete
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("read %s: %w", src, err)
	}
	if err := os.WriteFile(dst, data, 0600); err != nil {
		return fmt.Errorf("write %s: %w", dst, err)
	}
	return os.Remove(src)
}
