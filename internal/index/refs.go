package index

import (
	"database/sql"
	"errors"
)

// SetRef indexes or updates a branch ref in SQLite
func (idx *Index) SetRef(name string, checkpointID string) error {
	_, err := idx.db.Exec(`
		INSERT OR REPLACE INTO refs (name, checkpoint_id)
		VALUES (?, ?)
	`, name, checkpointID)
	return err
}

// GetRef retrieves a branch ref from SQLite
func (idx *Index) GetRef(name string) (string, error) {
	row := idx.db.QueryRow(`SELECT checkpoint_id FROM refs WHERE name = ?`, name)
	var cpID string
	err := row.Scan(&cpID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return cpID, nil
}

// ListRefs returns all indexed refs
func (idx *Index) ListRefs() (map[string]string, error) {
	rows, err := idx.db.Query(`SELECT name, checkpoint_id FROM refs ORDER BY name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	refs := make(map[string]string)
	for rows.Next() {
		var name, cpID string
		if err := rows.Scan(&name, &cpID); err != nil {
			return nil, err
		}
		refs[name] = cpID
	}
	return refs, rows.Err()
}

// DeleteRef removes a branch ref from SQLite
func (idx *Index) DeleteRef(name string) error {
	_, err := idx.db.Exec(`DELETE FROM refs WHERE name = ?`, name)
	return err
}
