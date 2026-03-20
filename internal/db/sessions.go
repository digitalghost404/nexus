// internal/db/sessions.go
package db

import (
	"database/sql"
	"fmt"
	"time"
)

type Session struct {
	ID              int64
	ProjectID       int64
	ClaudeSessionID string
	StartedAt       *time.Time
	EndedAt         *time.Time
	DurationSecs    int
	Summary         string
	FilesChanged    string
	CommitsMade     string
	Tags            string
	Source          string
	CreatedAt       time.Time
	// Joined fields
	ProjectName string
}

type SessionFilter struct {
	ProjectID int64
	Since     *time.Time
	Limit     int
}

func (d *DB) InsertSession(s Session) (int64, error) {
	result, err := d.db.Exec(`
		INSERT INTO sessions (project_id, claude_session_id, started_at, ended_at,
			duration_secs, summary, files_changed, commits_made, tags, source)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.ProjectID, nilIfEmpty(s.ClaudeSessionID), s.StartedAt, s.EndedAt,
		s.DurationSecs, s.Summary, defaultJSON(s.FilesChanged),
		defaultJSON(s.CommitsMade), defaultJSON(s.Tags), s.Source)
	if err != nil {
		return 0, fmt.Errorf("insert session: %w", err)
	}
	return result.LastInsertId()
}

func (d *DB) SessionExists(claudeSessionID string) (bool, error) {
	var count int
	err := d.db.QueryRow(
		"SELECT count(*) FROM sessions WHERE claude_session_id = ?",
		claudeSessionID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (d *DB) HasOverlappingSession(projectID int64, start, end time.Time) (bool, error) {
	var count int
	err := d.db.QueryRow(`
		SELECT count(*) FROM sessions
		WHERE project_id = ?
		AND started_at <= ? AND ended_at >= ?`,
		projectID, end, start).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (d *DB) ListSessions(f SessionFilter) ([]Session, error) {
	query := `
		SELECT s.id, s.project_id, COALESCE(s.claude_session_id, ''),
			s.started_at, s.ended_at, s.duration_secs, COALESCE(s.summary, ''),
			s.files_changed, s.commits_made, s.tags, s.source, s.created_at,
			p.name
		FROM sessions s
		JOIN projects p ON p.id = s.project_id
		WHERE 1=1`
	var args []interface{}

	if f.ProjectID > 0 {
		query += " AND s.project_id = ?"
		args = append(args, f.ProjectID)
	}
	if f.Since != nil {
		query += " AND s.started_at >= ?"
		args = append(args, f.Since)
	}

	query += " ORDER BY s.started_at DESC"

	if f.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", f.Limit)
	}

	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var s Session
		if err := rows.Scan(&s.ID, &s.ProjectID, &s.ClaudeSessionID,
			&s.StartedAt, &s.EndedAt, &s.DurationSecs, &s.Summary,
			&s.FilesChanged, &s.CommitsMade, &s.Tags, &s.Source, &s.CreatedAt,
			&s.ProjectName); err != nil {
			return nil, fmt.Errorf("scan session: %w", err)
		}
		sessions = append(sessions, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate rows: %w", err)
	}
	return sessions, nil
}

func (d *DB) SearchSessions(query string) ([]Session, error) {
	rows, err := d.db.Query(`
		SELECT s.id, s.project_id, COALESCE(s.claude_session_id, ''),
			s.started_at, s.ended_at, s.duration_secs, COALESCE(s.summary, ''),
			s.files_changed, s.commits_made, s.tags, s.source, s.created_at,
			p.name
		FROM sessions_fts fts
		JOIN sessions s ON s.id = fts.rowid
		JOIN projects p ON p.id = s.project_id
		WHERE sessions_fts MATCH ?
		ORDER BY rank`, query)
	if err != nil {
		return nil, fmt.Errorf("search sessions: %w", err)
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var s Session
		if err := rows.Scan(&s.ID, &s.ProjectID, &s.ClaudeSessionID,
			&s.StartedAt, &s.EndedAt, &s.DurationSecs, &s.Summary,
			&s.FilesChanged, &s.CommitsMade, &s.Tags, &s.Source, &s.CreatedAt,
			&s.ProjectName); err != nil {
			return nil, fmt.Errorf("scan search result: %w", err)
		}
		sessions = append(sessions, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate rows: %w", err)
	}
	return sessions, nil
}

func (d *DB) GetLatestSession(projectID int64) (*Session, error) {
	var s Session
	err := d.db.QueryRow(`
		SELECT s.id, s.project_id, COALESCE(s.claude_session_id, ''),
			s.started_at, s.ended_at, s.duration_secs, COALESCE(s.summary, ''),
			s.files_changed, s.commits_made, s.tags, s.source, s.created_at,
			p.name
		FROM sessions s
		JOIN projects p ON p.id = s.project_id
		WHERE s.project_id = ?
		ORDER BY s.started_at DESC
		LIMIT 1`, projectID).Scan(
		&s.ID, &s.ProjectID, &s.ClaudeSessionID,
		&s.StartedAt, &s.EndedAt, &s.DurationSecs, &s.Summary,
		&s.FilesChanged, &s.CommitsMade, &s.Tags, &s.Source, &s.CreatedAt,
		&s.ProjectName)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get latest session: %w", err)
	}
	return &s, nil
}

func (d *DB) GetSessionsInRange(projectID int64, since, until time.Time) ([]Session, error) {
	rows, err := d.db.Query(`
		SELECT s.id, s.project_id, COALESCE(s.claude_session_id, ''),
			s.started_at, s.ended_at, s.duration_secs, COALESCE(s.summary, ''),
			s.files_changed, s.commits_made, s.tags, s.source, s.created_at,
			p.name
		FROM sessions s
		JOIN projects p ON p.id = s.project_id
		WHERE s.project_id = ? AND s.started_at >= ? AND s.started_at <= ?
		ORDER BY s.started_at DESC`, projectID, since, until)
	if err != nil {
		return nil, fmt.Errorf("sessions in range: %w", err)
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var s Session
		if err := rows.Scan(&s.ID, &s.ProjectID, &s.ClaudeSessionID,
			&s.StartedAt, &s.EndedAt, &s.DurationSecs, &s.Summary,
			&s.FilesChanged, &s.CommitsMade, &s.Tags, &s.Source, &s.CreatedAt,
			&s.ProjectName); err != nil {
			return nil, fmt.Errorf("scan session: %w", err)
		}
		sessions = append(sessions, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate rows: %w", err)
	}
	return sessions, nil
}

func (d *DB) GetDistinctSessionDates() ([]string, error) {
	// Note: SQLite stores timestamps in UTC. The substr(CAST...) approach extracts
	// the date portion in UTC, which may differ from local date for sessions recorded
	// near midnight in negative-offset timezones (e.g. 11PM EST appears as next day UTC).
	// For a personal single-machine tool this is an acceptable limitation.
	rows, err := d.db.Query(`
		SELECT DISTINCT substr(CAST(started_at AS TEXT), 1, 10)
		FROM sessions
		WHERE started_at IS NOT NULL
		ORDER BY 1 DESC`)
	if err != nil {
		return nil, fmt.Errorf("distinct dates: %w", err)
	}
	defer rows.Close()

	var dates []string
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return nil, err
		}
		dates = append(dates, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate rows: %w", err)
	}
	return dates, nil
}

func nilIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func defaultJSON(s string) string {
	if s == "" {
		return "[]"
	}
	return s
}

// Ensure sql package is used (for sql.ErrNoRows in other files)
var _ = sql.ErrNoRows
