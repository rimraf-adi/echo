package index

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Index wraps the SQLite database used for accelerating queries and full-text search
type Index struct {
	db *sql.DB
}

// OpenIndex opens or creates the SQLite index database with WAL mode and runs migrations
func OpenIndex(dbPath string) (*Index, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("creating index directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening index db: %w", err)
	}

	// Configure SQLite pragmas for performance and concurrency
	pragmas := []string{
		"PRAGMA journal_mode = WAL;",
		"PRAGMA busy_timeout = 5000;",
		"PRAGMA synchronous = NORMAL;",
	}
	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("executing pragma %s: %w", pragma, err)
		}
	}

	idx := &Index{db: db}
	if err := idx.migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("running index migrations: %w", err)
	}

	return idx, nil
}

// Close closes the database connection
func (idx *Index) Close() error {
	if idx.db != nil {
		return idx.db.Close()
	}
	return nil
}

func (idx *Index) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS files (
		checkpoint_id TEXT NOT NULL,
		path          TEXT NOT NULL,
		blob_hash     TEXT NOT NULL,
		mode          TEXT NOT NULL,
		size_bytes    INTEGER NOT NULL,
		PRIMARY KEY (checkpoint_id, path)
	);

	CREATE TABLE IF NOT EXISTS checkpoints (
		id            TEXT PRIMARY KEY,
		parent_ids    TEXT NOT NULL,
		tree_hash     TEXT NOT NULL,
		created_at    TEXT NOT NULL,
		agent         TEXT,
		task          TEXT,
		model         TEXT,
		tags          TEXT,
		total_files   INTEGER NOT NULL,
		total_bytes   INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS refs (
		name          TEXT PRIMARY KEY,
		checkpoint_id TEXT NOT NULL
	);

	CREATE VIRTUAL TABLE IF NOT EXISTS content_fts USING fts5(
		path,
		content,
		checkpoint_id UNINDEXED
	);

	CREATE INDEX IF NOT EXISTS idx_files_path ON files(path);
	CREATE INDEX IF NOT EXISTS idx_files_blob ON files(blob_hash);
	CREATE INDEX IF NOT EXISTS idx_checkpoints_agent ON checkpoints(agent);
	CREATE INDEX IF NOT EXISTS idx_checkpoints_task ON checkpoints(task);
	CREATE INDEX IF NOT EXISTS idx_checkpoints_created ON checkpoints(created_at);
	`

	_, err := idx.db.Exec(schema)
	return err
}
