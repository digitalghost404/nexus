package db

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func testDB(t *testing.T) *DB {
	t.Helper()
	dir := t.TempDir()
	d, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func TestOpenCreatesDatabase(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	d, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer d.Close()

	var count int
	err = d.db.QueryRow("SELECT count(*) FROM projects").Scan(&count)
	if err != nil {
		t.Fatalf("projects table missing: %v", err)
	}

	err = d.db.QueryRow("SELECT count(*) FROM sessions").Scan(&count)
	if err != nil {
		t.Fatalf("sessions table missing: %v", err)
	}

	err = d.db.QueryRow("SELECT count(*) FROM notes").Scan(&count)
	if err != nil {
		t.Fatalf("notes table missing: %v", err)
	}
}

func TestOpenSetsWALMode(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	d, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer d.Close()

	var mode string
	d.db.QueryRow("PRAGMA journal_mode").Scan(&mode)
	if mode != "wal" {
		t.Errorf("expected WAL mode, got: %s", mode)
	}
}

func TestSchemaVersion(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	d, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer d.Close()

	var version int
	d.db.QueryRow("PRAGMA user_version").Scan(&version)
	if version != 2 {
		t.Errorf("expected user_version=2, got: %d", version)
	}
}

func TestMigrationV1ToV2(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	// Create a v1 database manually
	sqlDB, _ := sql.Open("sqlite", dbPath)
	sqlDB.Exec("PRAGMA journal_mode=WAL")
	sqlDB.Exec(schemaSQL) // current schema
	sqlDB.Exec("PRAGMA user_version = 1")

	// Insert some v1 data
	sqlDB.Exec("INSERT INTO projects (name, path, status, discovered_at) VALUES ('test', '/test', 'active', datetime('now'))")
	sqlDB.Close()

	// Open with new migration code
	d, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open with migration failed: %v", err)
	}
	defer d.Close()

	// Verify v2 tables exist
	var count int
	err = d.db.QueryRow("SELECT count(*) FROM project_links").Scan(&count)
	if err != nil {
		t.Fatalf("project_links table missing: %v", err)
	}
	err = d.db.QueryRow("SELECT count(*) FROM session_tags").Scan(&count)
	if err != nil {
		t.Fatalf("session_tags table missing: %v", err)
	}

	// Verify version is now 2
	var version int
	d.db.QueryRow("PRAGMA user_version").Scan(&version)
	if version != 2 {
		t.Errorf("expected user_version=2, got %d", version)
	}

	// Verify existing data survived
	var projName string
	d.db.QueryRow("SELECT name FROM projects WHERE path = '/test'").Scan(&projName)
	if projName != "test" {
		t.Errorf("existing data lost during migration")
	}
}
