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
	if version != 5 {
		t.Errorf("expected user_version=5, got: %d", version)
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

	// Verify v3 table exists
	err = d.db.QueryRow("SELECT count(*) FROM session_conversations").Scan(&count)
	if err != nil {
		t.Fatalf("session_conversations table missing: %v", err)
	}

	// Verify version is now 5 (v1 migrates through v2, v3, v4, and v5)
	var version int
	d.db.QueryRow("PRAGMA user_version").Scan(&version)
	if version != 5 {
		t.Errorf("expected user_version=5, got %d", version)
	}

	// Verify existing data survived
	var projName string
	d.db.QueryRow("SELECT name FROM projects WHERE path = '/test'").Scan(&projName)
	if projName != "test" {
		t.Errorf("existing data lost during migration")
	}
}

func TestMigrationV4ToV5(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	// Create a v4 database
	sqlDB, _ := sql.Open("sqlite", dbPath)
	sqlDB.Exec("PRAGMA journal_mode=WAL")
	sqlDB.Exec(schemaSQL)
	sqlDB.Exec(migrationV4SQL)
	sqlDB.Exec("PRAGMA user_version = 4")

	// Insert a project so we can verify data survives migration
	sqlDB.Exec("INSERT INTO projects (name, path, status, discovered_at) VALUES ('testproj', '/testproj', 'active', datetime('now'))")
	sqlDB.Close()

	// Open with new migration code
	d, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open with migration failed: %v", err)
	}
	defer d.Close()

	// Verify version is 5
	var version int
	d.db.QueryRow("PRAGMA user_version").Scan(&version)
	if version != 5 {
		t.Errorf("expected user_version=5, got %d", version)
	}

	// Verify preferences table exists
	var name string
	err = d.db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='preferences'").Scan(&name)
	if err != nil {
		t.Errorf("preferences table not found: %v", err)
	}

	// Verify preferences_fts virtual table exists
	err = d.db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='preferences_fts'").Scan(&name)
	if err != nil {
		t.Errorf("preferences_fts table not found: %v", err)
	}

	// Verify embedding_meta table exists
	err = d.db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='embedding_meta'").Scan(&name)
	if err != nil {
		t.Errorf("embedding_meta table not found: %v", err)
	}

	// Verify embeddings table exists
	err = d.db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='embeddings'").Scan(&name)
	if err != nil {
		t.Errorf("embeddings table not found: %v", err)
	}

	// Verify existing project data survived migration
	var projName string
	d.db.QueryRow("SELECT name FROM projects WHERE path = '/testproj'").Scan(&projName)
	if projName != "testproj" {
		t.Errorf("existing data lost during migration")
	}

	// Verify preferences table has correct columns
	rows, err := d.db.Query("SELECT id, project_id, category, content, source, confidence, created_at, updated_at, last_referenced_at, access_count, superseded_by, superseded_at FROM preferences LIMIT 0")
	if err != nil {
		t.Errorf("preferences table missing expected columns: %v", err)
	} else {
		rows.Close()
	}

	// Verify embedding_meta table has correct columns
	rows, err = d.db.Query("SELECT id, source_type, source_id, content_hash, content_truncated, embedded_at, embedding_model FROM embedding_meta LIMIT 0")
	if err != nil {
		t.Errorf("embedding_meta table missing expected columns: %v", err)
	} else {
		rows.Close()
	}

	// Verify embeddings table has correct columns
	rows, err = d.db.Query("SELECT id, embedding FROM embeddings LIMIT 0")
	if err != nil {
		t.Errorf("embeddings table missing expected columns: %v", err)
	} else {
		rows.Close()
	}
}
