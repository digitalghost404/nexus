// internal/db/links.go
package db

import "fmt"

func (d *DB) LinkProjects(projectID, linkedProjectID int64) error {
	_, err := d.db.Exec(`
		INSERT OR IGNORE INTO project_links (project_id, linked_project_id) VALUES (?, ?)`,
		projectID, linkedProjectID)
	if err != nil {
		return fmt.Errorf("link: %w", err)
	}
	_, err = d.db.Exec(`
		INSERT OR IGNORE INTO project_links (project_id, linked_project_id) VALUES (?, ?)`,
		linkedProjectID, projectID)
	return err
}

func (d *DB) UnlinkProjects(projectID, linkedProjectID int64) error {
	if _, err := d.db.Exec("DELETE FROM project_links WHERE project_id = ? AND linked_project_id = ?", projectID, linkedProjectID); err != nil {
		return fmt.Errorf("unlink: %w", err)
	}
	if _, err := d.db.Exec("DELETE FROM project_links WHERE project_id = ? AND linked_project_id = ?", linkedProjectID, projectID); err != nil {
		return fmt.Errorf("unlink reverse: %w", err)
	}
	return nil
}

func (d *DB) GetLinkedProjects(projectID int64) ([]Project, error) {
	rows, err := d.db.Query(`
		SELECT p.id, p.name, p.path, p.languages, p.branch, p.dirty, p.dirty_files,
			p.last_commit_at, p.last_commit_msg, p.ahead, p.behind, p.status,
			p.discovered_at, p.last_scanned_at
		FROM project_links pl
		JOIN projects p ON p.id = pl.linked_project_id
		WHERE pl.project_id = ?
		ORDER BY p.name`, projectID)
	if err != nil {
		return nil, fmt.Errorf("get linked: %w", err)
	}
	defer rows.Close()

	var projects []Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.Name, &p.Path, &p.Languages, &p.Branch,
			&p.Dirty, &p.DirtyFiles, &p.LastCommitAt, &p.LastCommitMsg,
			&p.Ahead, &p.Behind, &p.Status, &p.DiscoveredAt, &p.LastScannedAt); err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate rows: %w", err)
	}
	return projects, nil
}
