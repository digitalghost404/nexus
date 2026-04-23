// internal/db/projects.go
package db

import (
	"database/sql"
	"fmt"
)

type Project struct {
	ID            int64
	Name          string
	Path          string
	Languages     string
	Branch        string
	Dirty         bool
	DirtyFiles    int
	LastCommitAt  NullTime
	LastCommitMsg string
	Ahead         int
	Behind        int
	Status        string
	DiscoveredAt  NullTime
	LastScannedAt NullTime
}

func (d *DB) UpsertProject(p Project) (int64, error) {
	result, err := d.db.Exec(`
		INSERT INTO projects (name, path, languages, branch, dirty, dirty_files,
			last_commit_at, last_commit_msg, ahead, behind, status, discovered_at, last_scanned_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET
			name=excluded.name, languages=excluded.languages, branch=excluded.branch,
			dirty=excluded.dirty, dirty_files=excluded.dirty_files,
			last_commit_at=excluded.last_commit_at, last_commit_msg=excluded.last_commit_msg,
			ahead=excluded.ahead, behind=excluded.behind, status=excluded.status,
			last_scanned_at=excluded.last_scanned_at`,
		p.Name, p.Path, p.Languages, p.Branch, p.Dirty, p.DirtyFiles,
		p.LastCommitAt, p.LastCommitMsg, p.Ahead, p.Behind, p.Status,
		p.DiscoveredAt, p.LastScannedAt)
	if err != nil {
		return 0, fmt.Errorf("upsert project: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	// LastInsertId returns 0 on update, get the real ID
	if id == 0 {
		if err := d.db.QueryRow("SELECT id FROM projects WHERE path = ?", p.Path).Scan(&id); err != nil {
			return 0, fmt.Errorf("get project id after upsert: %w", err)
		}
	}
	return id, nil
}

func (d *DB) GetProjectByPath(path string) (*Project, error) {
	p := &Project{}
	err := d.db.QueryRow(`
		SELECT id, name, path, languages, branch, dirty, dirty_files,
			last_commit_at, last_commit_msg, ahead, behind, status,
			discovered_at, last_scanned_at
		FROM projects WHERE path = ?`, path).Scan(
		&p.ID, &p.Name, &p.Path, &p.Languages, &p.Branch, &p.Dirty, &p.DirtyFiles,
		&p.LastCommitAt, &p.LastCommitMsg, &p.Ahead, &p.Behind, &p.Status,
		&p.DiscoveredAt, &p.LastScannedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get project: %w", err)
	}
	return p, nil
}

func (d *DB) GetProjectByName(name string) (*Project, error) {
	p := &Project{}
	err := d.db.QueryRow(`
		SELECT id, name, path, languages, branch, dirty, dirty_files,
			last_commit_at, last_commit_msg, ahead, behind, status,
			discovered_at, last_scanned_at
		FROM projects WHERE name = ?`, name).Scan(
		&p.ID, &p.Name, &p.Path, &p.Languages, &p.Branch, &p.Dirty, &p.DirtyFiles,
		&p.LastCommitAt, &p.LastCommitMsg, &p.Ahead, &p.Behind, &p.Status,
		&p.DiscoveredAt, &p.LastScannedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get project by name: %w", err)
	}
	return p, nil
}

func (d *DB) ListProjects(status string) ([]Project, error) {
	var rows *sql.Rows
	var err error

	if status == "" {
		rows, err = d.db.Query(`
			SELECT id, name, path, languages, branch, dirty, dirty_files,
				last_commit_at, last_commit_msg, ahead, behind, status,
				discovered_at, last_scanned_at
			FROM projects WHERE status != 'archived' ORDER BY name`)
	} else {
		rows, err = d.db.Query(`
			SELECT id, name, path, languages, branch, dirty, dirty_files,
				last_commit_at, last_commit_msg, ahead, behind, status,
				discovered_at, last_scanned_at
			FROM projects WHERE status = ? ORDER BY name`, status)
	}
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var projects []Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.Name, &p.Path, &p.Languages, &p.Branch,
			&p.Dirty, &p.DirtyFiles, &p.LastCommitAt, &p.LastCommitMsg,
			&p.Ahead, &p.Behind, &p.Status, &p.DiscoveredAt, &p.LastScannedAt); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		projects = append(projects, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate rows: %w", err)
	}
	return projects, nil
}

func (d *DB) ArchiveProject(id int64) error {
	_, err := d.db.Exec("UPDATE projects SET status = 'archived' WHERE id = ?", id)
	return err
}

func (d *DB) DeleteProject(name string) (int, error) {
	var id int64
	err := d.db.QueryRow("SELECT id FROM projects WHERE name = ?", name).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("get project id: %w", err)
	}

	tx, err := d.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.Exec("DELETE FROM session_conversations WHERE session_id IN (SELECT id FROM sessions WHERE project_id = ?)", id)
	if err != nil {
		return 0, fmt.Errorf("delete conversations: %w", err)
	}
	_, err = tx.Exec("DELETE FROM session_tags WHERE session_id IN (SELECT id FROM sessions WHERE project_id = ?)", id)
	if err != nil {
		return 0, fmt.Errorf("delete session tags: %w", err)
	}
	_, err = tx.Exec("DELETE FROM embedding_meta WHERE source_type IN ('session', 'note') AND source_id IN (SELECT id FROM sessions WHERE project_id = ?) OR source_id IN (SELECT id FROM notes WHERE project_id = ?)", id, id)
	if err != nil {
		return 0, fmt.Errorf("delete embedding meta: %w", err)
	}
	_, err = tx.Exec("DELETE FROM embeddings WHERE id IN (SELECT id FROM embedding_meta WHERE source_type IN ('session', 'note') AND source_id IN (SELECT id FROM sessions WHERE project_id = ?) OR source_id IN (SELECT id FROM notes WHERE project_id = ?))", id, id)
	if err != nil {
		return 0, fmt.Errorf("delete embeddings: %w", err)
	}
	_, err = tx.Exec("DELETE FROM sessions WHERE project_id = ?", id)
	if err != nil {
		return 0, fmt.Errorf("delete sessions: %w", err)
	}

	result, err := tx.Exec("DELETE FROM notes WHERE project_id = ?", id)
	if err != nil {
		return 0, fmt.Errorf("delete notes: %w", err)
	}
	notesDeleted, _ := result.RowsAffected()

	_, err = tx.Exec("DELETE FROM preferences WHERE project_id = ?", id)
	if err != nil {
		return 0, fmt.Errorf("delete preferences: %w", err)
	}
	_, err = tx.Exec("DELETE FROM project_links WHERE project_id = ? OR linked_project_id = ?", id, id)
	if err != nil {
		return 0, fmt.Errorf("delete project links: %w", err)
	}
	_, err = tx.Exec("DELETE FROM projects WHERE id = ?", id)
	if err != nil {
		return 0, fmt.Errorf("delete project: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}

	return int(notesDeleted), nil
}

func (d *DB) ListDirtyProjects() ([]Project, error) {
	rows, err := d.db.Query(`
		SELECT id, name, path, languages, branch, dirty, dirty_files,
			last_commit_at, last_commit_msg, ahead, behind, status,
			discovered_at, last_scanned_at
		FROM projects WHERE dirty = 1 ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list dirty: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var projects []Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.Name, &p.Path, &p.Languages, &p.Branch,
			&p.Dirty, &p.DirtyFiles, &p.LastCommitAt, &p.LastCommitMsg,
			&p.Ahead, &p.Behind, &p.Status, &p.DiscoveredAt, &p.LastScannedAt); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		projects = append(projects, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate rows: %w", err)
	}
	return projects, nil
}
