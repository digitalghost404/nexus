// internal/db/notes.go
package db

import (
	"fmt"
	"time"
)

type Note struct {
	ID        int64
	ProjectID *int64
	SessionID *int64
	Content   string
	CreatedAt time.Time
}

func (d *DB) InsertNote(n Note) (int64, error) {
	result, err := d.db.Exec(`
		INSERT INTO notes (project_id, session_id, content, created_at)
		VALUES (?, ?, ?, datetime('now'))`,
		n.ProjectID, n.SessionID, n.Content)
	if err != nil {
		return 0, fmt.Errorf("insert note: %w", err)
	}
	return result.LastInsertId()
}

func (d *DB) ListNotes(projectID int64, limit int) ([]Note, error) {
	var query string
	var args []interface{}

	if projectID > 0 {
		query = "SELECT id, project_id, session_id, content, created_at FROM notes WHERE project_id = ? ORDER BY created_at DESC LIMIT ?"
		args = []interface{}{projectID, limit}
	} else {
		query = "SELECT id, project_id, session_id, content, created_at FROM notes ORDER BY created_at DESC LIMIT ?"
		args = []interface{}{limit}
	}

	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list notes: %w", err)
	}
	defer rows.Close()

	var notes []Note
	for rows.Next() {
		var n Note
		if err := rows.Scan(&n.ID, &n.ProjectID, &n.SessionID, &n.Content, &n.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan note: %w", err)
		}
		notes = append(notes, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate rows: %w", err)
	}
	return notes, nil
}

func (d *DB) SearchNotes(query string) ([]Note, error) {
	rows, err := d.db.Query(`
		SELECT n.id, n.project_id, n.session_id, n.content, n.created_at
		FROM notes_fts fts
		JOIN notes n ON n.id = fts.rowid
		WHERE notes_fts MATCH ?
		ORDER BY rank`, query)
	if err != nil {
		return nil, fmt.Errorf("search notes: %w", err)
	}
	defer rows.Close()

	var notes []Note
	for rows.Next() {
		var n Note
		if err := rows.Scan(&n.ID, &n.ProjectID, &n.SessionID, &n.Content, &n.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan note: %w", err)
		}
		notes = append(notes, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate rows: %w", err)
	}
	return notes, nil
}
