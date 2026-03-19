package db

import (
	"path/filepath"
	"testing"
)

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
	if version != 1 {
		t.Errorf("expected user_version=1, got: %d", version)
	}
}
