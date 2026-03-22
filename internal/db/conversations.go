package db

import "database/sql"

func (d *DB) InsertConversationDigest(sessionID int64, digestJSON string) error {
	_, err := d.db.Exec(
		`INSERT OR REPLACE INTO session_conversations (session_id, digest) VALUES (?, ?)`,
		sessionID, digestJSON)
	return err
}

func (d *DB) GetConversationDigest(sessionID int64) (string, error) {
	var digest string
	err := d.db.QueryRow(
		`SELECT digest FROM session_conversations WHERE session_id = ?`,
		sessionID).Scan(&digest)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return digest, err
}
