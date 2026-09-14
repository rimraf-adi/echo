package index

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/echo-vcs/echo/internal/core"
)

// CheckpointRecord represents indexed checkpoint metadata
type CheckpointRecord struct {
	ID         string    `json:"id"`
	ParentIDs  []string  `json:"parent_ids"`
	TreeHash   string    `json:"tree_hash"`
	CreatedAt  time.Time `json:"created_at"`
	Agent      string    `json:"agent,omitempty"`
	Task       string    `json:"task,omitempty"`
	Model      string    `json:"model,omitempty"`
	Tags       []string  `json:"tags,omitempty"`
	TotalFiles int       `json:"total_files"`
	TotalBytes int64     `json:"total_bytes"`
}

// IndexCheckpoint indexes a checkpoint's metadata in SQLite
func (idx *Index) IndexCheckpoint(cp *core.Checkpoint) error {
	parentJSON, err := json.Marshal(cp.Parents)
	if err != nil {
		return err
	}

	tagsJSON, err := json.Marshal(cp.Metadata.Tags)
	if err != nil {
		return err
	}

	query := `
		INSERT OR REPLACE INTO checkpoints (
			id, parent_ids, tree_hash, created_at, agent, task, model, tags, total_files, total_bytes
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err = idx.db.Exec(query,
		cp.ID,
		string(parentJSON),
		cp.TreeHash,
		cp.CreatedAt.UTC().Format(time.RFC3339Nano),
		cp.Metadata.Agent,
		cp.Metadata.Task,
		cp.Metadata.Model,
		string(tagsJSON),
		cp.Stats.TotalFiles,
		cp.Stats.TotalSizeBytes,
	)
	return err
}

// CheckpointFilters specifies search filters for checkpoints
type CheckpointFilters struct {
	Agent string
	Task  string
	Since time.Time
	Until time.Time
	Limit int
}

// QueryCheckpoints searches checkpoints matching the given filters
func (idx *Index) QueryCheckpoints(filters CheckpointFilters) ([]CheckpointRecord, error) {
	var whereClauses []string
	var args []any

	if filters.Agent != "" {
		whereClauses = append(whereClauses, "agent = ?")
		args = append(args, filters.Agent)
	}

	if filters.Task != "" {
		whereClauses = append(whereClauses, "task LIKE ?")
		args = append(args, "%"+filters.Task+"%")
	}

	if !filters.Since.IsZero() {
		whereClauses = append(whereClauses, "created_at >= ?")
		args = append(args, filters.Since.UTC().Format(time.RFC3339Nano))
	}

	if !filters.Until.IsZero() {
		whereClauses = append(whereClauses, "created_at <= ?")
		args = append(args, filters.Until.UTC().Format(time.RFC3339Nano))
	}

	query := `SELECT id, parent_ids, tree_hash, created_at, agent, task, model, tags, total_files, total_bytes FROM checkpoints`
	if len(whereClauses) > 0 {
		query += " WHERE " + strings.Join(whereClauses, " AND ")
	}
	query += " ORDER BY created_at DESC"

	if filters.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", filters.Limit)
	}

	rows, err := idx.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []CheckpointRecord
	for rows.Next() {
		var rec CheckpointRecord
		var parentStr, tagsStr, createdStr string
		var agent, task, model sqlNullString

		if err := rows.Scan(
			&rec.ID,
			&parentStr,
			&rec.TreeHash,
			&createdStr,
			&agent,
			&task,
			&model,
			&tagsStr,
			&rec.TotalFiles,
			&rec.TotalBytes,
		); err != nil {
			return nil, err
		}

		_ = json.Unmarshal([]byte(parentStr), &rec.ParentIDs)
		_ = json.Unmarshal([]byte(tagsStr), &rec.Tags)
		rec.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdStr)
		rec.Agent = agent.String
		rec.Task = task.String
		rec.Model = model.String

		records = append(records, rec)
	}

	return records, rows.Err()
}

type sqlNullString struct {
	String string
	Valid  bool
}

func (s *sqlNullString) Scan(value any) error {
	if value == nil {
		s.String = ""
		s.Valid = false
		return nil
	}
	switch v := value.(type) {
	case string:
		s.String = v
		s.Valid = true
	case []byte:
		s.String = string(v)
		s.Valid = true
	}
	return nil
}
