// internal/db/tags.go
package db

import "fmt"

func (d *DB) AddSessionTag(sessionID int64, tag string) error {
	_, err := d.db.Exec(`
		INSERT OR IGNORE INTO session_tags (session_id, tag) VALUES (?, ?)`,
		sessionID, tag)
	if err != nil {
		return fmt.Errorf("add tag: %w", err)
	}
	return nil
}

func (d *DB) RemoveSessionTag(sessionID int64, tag string) error {
	_, err := d.db.Exec("DELETE FROM session_tags WHERE session_id = ? AND tag = ?", sessionID, tag)
	return err
}

func (d *DB) ListSessionTags(sessionID int64) ([]string, error) {
	rows, err := d.db.Query("SELECT tag FROM session_tags WHERE session_id = ? ORDER BY tag", sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var tag string
		rows.Scan(&tag)
		tags = append(tags, tag)
	}
	return tags, nil
}

func (d *DB) ListSessionsByTag(tag string) ([]Session, error) {
	rows, err := d.db.Query(`
		SELECT s.id, s.project_id, COALESCE(s.claude_session_id, ''),
			s.started_at, s.ended_at, s.duration_secs, COALESCE(s.summary, ''),
			s.files_changed, s.commits_made, s.tags, s.source, s.created_at,
			p.name
		FROM session_tags st
		JOIN sessions s ON s.id = st.session_id
		JOIN projects p ON p.id = s.project_id
		WHERE st.tag = ?
		ORDER BY s.started_at DESC`, tag)
	if err != nil {
		return nil, fmt.Errorf("sessions by tag: %w", err)
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var s Session
		if err := rows.Scan(&s.ID, &s.ProjectID, &s.ClaudeSessionID,
			&s.StartedAt, &s.EndedAt, &s.DurationSecs, &s.Summary,
			&s.FilesChanged, &s.CommitsMade, &s.Tags, &s.Source, &s.CreatedAt,
			&s.ProjectName); err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	return sessions, nil
}
