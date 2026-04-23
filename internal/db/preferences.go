package db

import (
	"database/sql"
	"fmt"
	"math"
	"time"
)

type Preference struct {
	ID               int64
	ProjectID        *int64
	Category         string
	Content          string
	Source           string
	Confidence       float64
	CreatedAt        time.Time
	UpdatedAt        time.Time
	LastReferencedAt *time.Time
	AccessCount      int
	SupersededBy     *int64
	SupersededAt     *time.Time
}

const (
	halfLifeStated   = 90.0
	halfLifeObserved = 90.0
	halfLifeInferred = 45.0
)

func (d *DB) InsertPreference(p Preference) (int64, error) {
	var existingID int64
	err := d.db.QueryRow(
		"SELECT id FROM preferences WHERE category = ? AND content = ? AND (project_id = ? OR (project_id IS NULL AND ? IS NULL)) AND superseded_by IS NULL",
		p.Category, p.Content, p.ProjectID, p.ProjectID,
	).Scan(&existingID)

	if err == nil {
		_, err := d.db.Exec(
			"UPDATE preferences SET confidence = ?, source = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
			p.Confidence, p.Source, existingID,
		)
		return existingID, err
	}
	if err != sql.ErrNoRows {
		return 0, fmt.Errorf("check duplicate preference: %w", err)
	}

	now := time.Now()
	createdAt := now
	if !p.CreatedAt.IsZero() {
		createdAt = p.CreatedAt
	}
	updatedAt := now
	if !p.UpdatedAt.IsZero() {
		updatedAt = p.UpdatedAt
	}
	result, err := d.db.Exec(
		`INSERT INTO preferences (project_id, category, content, source, confidence, created_at, updated_at, last_referenced_at, access_count)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0)`,
		p.ProjectID, p.Category, p.Content, p.Source, p.Confidence, createdAt, updatedAt, p.LastReferencedAt,
	)
	if err != nil {
		return 0, fmt.Errorf("insert preference: %w", err)
	}
	return result.LastInsertId()
}

func (d *DB) GetPreference(id int64) (*Preference, error) {
	p := &Preference{}
	var lastRef sql.NullTime
	var projID sql.NullInt64
	var supersededBy sql.NullInt64
	var supersededAt sql.NullTime

	err := d.db.QueryRow(
		"SELECT id, project_id, category, content, source, confidence, created_at, updated_at, last_referenced_at, access_count, superseded_by, superseded_at FROM preferences WHERE id = ?",
		id,
	).Scan(&p.ID, &projID, &p.Category, &p.Content, &p.Source, &p.Confidence, &p.CreatedAt, &p.UpdatedAt, &lastRef, &p.AccessCount, &supersededBy, &supersededAt)

	if err != nil {
		return nil, fmt.Errorf("get preference: %w", err)
	}
	if projID.Valid {
		p.ProjectID = &projID.Int64
	}
	if lastRef.Valid {
		p.LastReferencedAt = &lastRef.Time
	}
	if supersededBy.Valid {
		p.SupersededBy = &supersededBy.Int64
	}
	if supersededAt.Valid {
		p.SupersededAt = &supersededAt.Time
	}
	return p, nil
}

func (d *DB) ListPreferencesByProject(projectID *int64) ([]Preference, error) {
	rows, err := d.db.Query(
		"SELECT id, project_id, category, content, source, confidence, created_at, updated_at, last_referenced_at, access_count, superseded_by, superseded_at FROM preferences WHERE confidence > 0.3 AND superseded_by IS NULL AND (project_id = ? OR project_id IS NULL) ORDER BY confidence DESC",
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("list preferences by project: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanPreferences(rows)
}

func (d *DB) SearchPreferences(query string) ([]Preference, error) {
	rows, err := d.db.Query(
		"SELECT p.id, p.project_id, p.category, p.content, p.source, p.confidence, p.created_at, p.updated_at, p.last_referenced_at, p.access_count, p.superseded_by, p.superseded_at FROM preferences p JOIN preferences_fts f ON p.id = f.rowid WHERE preferences_fts MATCH ? AND p.confidence > 0.3 AND p.superseded_by IS NULL ORDER BY p.confidence DESC",
		sanitizeFTS(query),
	)
	if err != nil {
		return nil, fmt.Errorf("search preferences: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanPreferences(rows)
}

func (d *DB) DecayPreferences() error {
	rows, err := d.db.Query("SELECT id, source, confidence, last_referenced_at FROM preferences WHERE superseded_by IS NULL")
	if err != nil {
		return fmt.Errorf("query preferences for decay: %w", err)
	}
	defer func() { _ = rows.Close() }()

	now := time.Now()
	for rows.Next() {
		var id int64
		var source string
		var confidence float64
		var lastRef sql.NullTime

		if err := rows.Scan(&id, &source, &confidence, &lastRef); err != nil {
			return fmt.Errorf("scan preference %d: %w", id, err)
		}

		halfLife := halfLifeStated
		switch source {
		case "observed":
			halfLife = halfLifeObserved
		case "inferred":
			halfLife = halfLifeInferred
		}

		lastAccess := now
		if lastRef.Valid {
			lastAccess = lastRef.Time
		}
		daysSinceAccess := now.Sub(lastAccess).Hours() / 24.0

		initialConf := 1.0
		switch source {
		case "observed":
			initialConf = 0.7
		case "inferred":
			initialConf = 0.4
		}

		newConf := initialConf * math.Pow(0.5, daysSinceAccess/halfLife)
		_, err := d.db.Exec("UPDATE preferences SET confidence = ?, updated_at = ? WHERE id = ?", newConf, now, id)
		if err != nil {
			return fmt.Errorf("decay preference %d: %w", id, err)
		}
	}
	return rows.Err()
}

func (d *DB) DeleteLowConfidencePreferences(threshold float64) (int64, error) {
	result, err := d.db.Exec("DELETE FROM preferences WHERE confidence < ? AND source = 'inferred'", threshold)
	if err != nil {
		return 0, fmt.Errorf("delete low confidence preferences: %w", err)
	}
	return result.RowsAffected()
}

func (d *DB) DeleteContradictingInferredPreferences() (int64, error) {
	result, err := d.db.Exec(
		`DELETE FROM preferences WHERE source = 'inferred' AND category IN (
			SELECT p2.category FROM preferences p2 WHERE p2.source = 'stated' AND p2.superseded_by IS NULL
		)`,
	)
	if err != nil {
		return 0, fmt.Errorf("delete contradicting inferred preferences: %w", err)
	}
	return result.RowsAffected()
}

func (d *DB) SupersedePreference(oldID, newID int64) error {
	now := time.Now()
	_, err := d.db.Exec("UPDATE preferences SET superseded_by = ?, superseded_at = ? WHERE id = ?", newID, now, oldID)
	if err != nil {
		return fmt.Errorf("supersede preference: %w", err)
	}
	return nil
}

func (d *DB) BumpPreferenceAccess(id int64) error {
	_, err := d.db.Exec("UPDATE preferences SET access_count = access_count + 1, last_referenced_at = CURRENT_TIMESTAMP WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("bump preference access: %w", err)
	}
	return nil
}

func (d *DB) DeleteSupersededPreferences(olderThanDays int) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -olderThanDays)
	result, err := d.db.Exec("DELETE FROM preferences WHERE superseded_at IS NOT NULL AND superseded_at < ?", cutoff)
	if err != nil {
		return 0, fmt.Errorf("delete superseded preferences: %w", err)
	}
	return result.RowsAffected()
}

func (d *DB) UpdatePreference(id int64, category, content, source string, confidence float64) error {
	_, err := d.db.Exec(
		"UPDATE preferences SET category = ?, content = ?, source = ?, confidence = ?, updated_at = ? WHERE id = ?",
		category, content, source, confidence, time.Now(), id,
	)
	if err != nil {
		return fmt.Errorf("update preference: %w", err)
	}
	return nil
}

func (d *DB) DeletePreference(id int64) error {
	_, err := d.db.Exec("DELETE FROM preferences WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete preference: %w", err)
	}
	return nil
}

func scanPreferences(rows *sql.Rows) ([]Preference, error) {
	var prefs []Preference
	for rows.Next() {
		var p Preference
		var projID sql.NullInt64
		var lastRef sql.NullTime
		var supersededBy sql.NullInt64
		var supersededAt sql.NullTime

		err := rows.Scan(&p.ID, &projID, &p.Category, &p.Content, &p.Source, &p.Confidence, &p.CreatedAt, &p.UpdatedAt, &lastRef, &p.AccessCount, &supersededBy, &supersededAt)
		if err != nil {
			return prefs, fmt.Errorf("scan preference: %w", err)
		}
		if projID.Valid {
			p.ProjectID = &projID.Int64
		}
		if lastRef.Valid {
			p.LastReferencedAt = &lastRef.Time
		}
		if supersededBy.Valid {
			p.SupersededBy = &supersededBy.Int64
		}
		if supersededAt.Valid {
			p.SupersededAt = &supersededAt.Time
		}
		prefs = append(prefs, p)
	}
	if err := rows.Err(); err != nil {
		return prefs, fmt.Errorf("iterate preferences: %w", err)
	}
	return prefs, nil
}
