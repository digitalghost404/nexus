package db

import (
	"fmt"
	"time"
)

func (d *DB) PruneSessions(before time.Time) (int64, error) {
	result, err := d.db.Exec("DELETE FROM sessions WHERE created_at < ?", before.UTC())
	if err != nil {
		return 0, fmt.Errorf("prune sessions: %w", err)
	}
	return result.RowsAffected()
}

func (d *DB) PruneNotes(before time.Time) (int64, error) {
	result, err := d.db.Exec("DELETE FROM notes WHERE created_at < ?", before.UTC())
	if err != nil {
		return 0, fmt.Errorf("prune notes: %w", err)
	}
	return result.RowsAffected()
}

func (d *DB) PrunePreferences(before time.Time) (int64, error) {
	result, err := d.db.Exec("DELETE FROM preferences WHERE created_at < ?", before.UTC())
	if err != nil {
		return 0, fmt.Errorf("prune preferences: %w", err)
	}
	return result.RowsAffected()
}
