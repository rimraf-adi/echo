package index

import (
	"database/sql"
	"errors"
	"path/filepath"

	"github.com/echo-vcs/echo/internal/merkle"
)

// FileRecord represents a tracked file at a specific checkpoint
type FileRecord struct {
	CheckpointID string `json:"checkpoint_id"`
	Path         string `json:"path"`
	BlobHash     string `json:"blob_hash"`
	Mode         string `json:"mode"`
	SizeBytes    int64  `json:"size_bytes"`
}

// IndexFiles indexes all files for a checkpoint inside a single transaction
func (idx *Index) IndexFiles(checkpointID string, files map[string]merkle.TreeEntry) error {
	tx, err := idx.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	stmt, err := tx.Prepare(`
		INSERT OR REPLACE INTO files (checkpoint_id, path, blob_hash, mode, size_bytes)
		VALUES (?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for path, entry := range files {
		if _, err := stmt.Exec(checkpointID, path, entry.Hash, entry.Mode, entry.Size); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// QueryFiles retrieves files for a checkpoint, optionally filtered by a glob pattern
func (idx *Index) QueryFiles(checkpointID string, pattern string) ([]FileRecord, error) {
	query := `SELECT checkpoint_id, path, blob_hash, mode, size_bytes FROM files WHERE checkpoint_id = ? ORDER BY path ASC`
	rows, err := idx.db.Query(query, checkpointID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []FileRecord
	for rows.Next() {
		var rec FileRecord
		if err := rows.Scan(&rec.CheckpointID, &rec.Path, &rec.BlobHash, &rec.Mode, &rec.SizeBytes); err != nil {
			return nil, err
		}

		if pattern != "" && pattern != "*" {
			matched, err := filepath.Match(pattern, rec.Path)
			if err != nil || !matched {
				base := filepath.Base(rec.Path)
				baseMatched, berr := filepath.Match(pattern, base)
				if berr != nil || !baseMatched {
					continue
				}
			}
		}

		records = append(records, rec)
	}

	return records, rows.Err()
}

// GetFile retrieves a specific file record for a checkpoint
func (idx *Index) GetFile(checkpointID string, path string) (*FileRecord, error) {
	row := idx.db.QueryRow(`
		SELECT checkpoint_id, path, blob_hash, mode, size_bytes
		FROM files
		WHERE checkpoint_id = ? AND path = ?
	`, checkpointID, path)

	var rec FileRecord
	err := row.Scan(&rec.CheckpointID, &rec.Path, &rec.BlobHash, &rec.Mode, &rec.SizeBytes)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &rec, nil
}
