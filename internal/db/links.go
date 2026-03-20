// internal/db/links.go
package db

// GetLinkedProjects returns projects linked to the given project.
// This is a v2 feature — returns empty slice gracefully if the table doesn't exist yet.
func (d *DB) GetLinkedProjects(projectID int64) ([]Project, error) {
	rows, err := d.db.Query(`
		SELECT p.id, p.name, p.path, p.languages, p.branch, p.dirty, p.dirty_files,
			p.last_commit_at, p.last_commit_msg, p.ahead, p.behind, p.status,
			p.discovered_at, p.last_scanned_at
		FROM project_links pl
		JOIN projects p ON (pl.project_b_id = p.id AND pl.project_a_id = ?)
			OR (pl.project_a_id = p.id AND pl.project_b_id = ?)
	`, projectID, projectID)
	if err != nil {
		// Table doesn't exist yet — return empty slice gracefully
		return nil, nil
	}
	defer rows.Close()

	var projects []Project
	for rows.Next() {
		var p Project
		err := rows.Scan(
			&p.ID, &p.Name, &p.Path, &p.Languages, &p.Branch, &p.Dirty, &p.DirtyFiles,
			&p.LastCommitAt, &p.LastCommitMsg, &p.Ahead, &p.Behind, &p.Status,
			&p.DiscoveredAt, &p.LastScannedAt,
		)
		if err != nil {
			continue
		}
		projects = append(projects, p)
	}
	return projects, nil
}
