// internal/db/projects.go
package db

import (
	"database/sql"
	"fmt"
	"time"
)

type Project struct {
	ID            int64
	Name          string
	Path          string
	Languages     string
	Branch        string
	Dirty         bool
	DirtyFiles    int
	LastCommitAt  *time.Time
	LastCommitMsg string
	Ahead         int
	Behind        int
	Status        string
	DiscoveredAt  time.Time
	LastScannedAt *time.Time
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
		row := d.db.QueryRow("SELECT id FROM projects WHERE path = ?", p.Path)
		row.Scan(&id)
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
	defer rows.Close()

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
	return projects, nil
}

func (d *DB) ArchiveProject(id int64) error {
	_, err := d.db.Exec("UPDATE projects SET status = 'archived' WHERE id = ?", id)
	return err
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
	defer rows.Close()

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
	return projects, nil
}
