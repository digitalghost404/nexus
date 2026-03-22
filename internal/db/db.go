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

type DB struct {
	db *sql.DB
}

func Open(path string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	sqlDB, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	if _, err := sqlDB.Exec("PRAGMA journal_mode=WAL"); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}

	d := &DB{db: sqlDB}

	if err := d.migrate(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return d, nil
}

func (d *DB) migrate() error {
	var version int
	if err := d.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}

	const currentVersion = 3
	if version > currentVersion {
		return fmt.Errorf("database schema version %d is newer than supported version %d — upgrade nexus", version, currentVersion)
	}

	if version == 0 {
		if _, err := d.db.Exec(schemaSQL); err != nil {
			return fmt.Errorf("apply schema: %w", err)
		}
		if _, err := d.db.Exec("PRAGMA user_version = 3"); err != nil {
			return fmt.Errorf("set version: %w", err)
		}
	}

	if version == 1 {
		if _, err := d.db.Exec(migrationV2SQL); err != nil {
			return fmt.Errorf("apply v2 migration: %w", err)
		}
		if _, err := d.db.Exec("PRAGMA user_version = 2"); err != nil {
			return fmt.Errorf("set version: %w", err)
		}
		version = 2
	}

	if version == 2 {
		if _, err := d.db.Exec(migrationV3SQL); err != nil {
			return fmt.Errorf("apply v3 migration: %w", err)
		}
		if _, err := d.db.Exec("PRAGMA user_version = 3"); err != nil {
			return fmt.Errorf("set version: %w", err)
		}
	}

	return nil
}

func (d *DB) Close() error {
	return d.db.Close()
}

func (d *DB) Conn() *sql.DB {
	return d.db
}
