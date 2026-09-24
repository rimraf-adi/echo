package index

import (
	"context"
	"fmt"
	"time"

	"github.com/echo-vcs/echo/internal/indexer"
)

// IndexSymbols processes and stores the parsed symbols and edges for a given file and checkpoint.
func (idx *Index) IndexSymbols(ctx context.Context, cpID, path string, symbols []indexer.Symbol, edges []indexer.Edge) error {
	tx, err := idx.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	symStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO symbols (checkpoint_id, path, name, type, start_line, end_line)
		VALUES (?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("preparing symbol insert: %w", err)
	}
	defer symStmt.Close()

	for _, sym := range symbols {
		_, err := symStmt.ExecContext(ctx, cpID, path, sym.Name, string(sym.Type), sym.StartLine, sym.EndLine)
		if err != nil {
			return fmt.Errorf("inserting symbol %s: %w", sym.Name, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing symbols transaction: %w", err)
	}
	return nil
}

type SymbolRecord struct {
	CheckpointID string
	Path         string
	Name         string
	Type         string
	StartLine    int
	EndLine      int
	CreatedAt    time.Time
}

// FindSymbol queries the most recent instances of a symbol by name.
func (idx *Index) FindSymbol(name string, limit int) ([]SymbolRecord, error) {
	query := `
		SELECT s.checkpoint_id, s.path, s.name, s.type, s.start_line, s.end_line, c.created_at
		FROM symbols s
		JOIN checkpoints c ON s.checkpoint_id = c.id
		WHERE s.name = ?
		ORDER BY c.created_at DESC
		LIMIT ?
	`
	rows, err := idx.db.Query(query, name, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []SymbolRecord
	for rows.Next() {
		var rec SymbolRecord
		var createdStr string
		if err := rows.Scan(&rec.CheckpointID, &rec.Path, &rec.Name, &rec.Type, &rec.StartLine, &rec.EndLine, &createdStr); err != nil {
			return nil, err
		}
		rec.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdStr)
		records = append(records, rec)
	}
	return records, nil
}
