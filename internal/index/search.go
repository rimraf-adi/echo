package index

import (
	"strings"
)

// SearchResult represents a text match in a file
type SearchResult struct {
	Path          string   `json:"path"`
	LineNumber    int      `json:"line_number"`
	Content       string   `json:"content"`
	ContextBefore []string `json:"context_before"`
	ContextAfter  []string `json:"context_after"`
}

// IndexContent indexes raw file content for full-text search
func (idx *Index) IndexContent(checkpointID string, path string, content string) error {
	_, err := idx.db.Exec(`
		INSERT INTO content_fts (path, content, checkpoint_id)
		VALUES (?, ?, ?)
	`, path, content, checkpointID)
	return err
}

// ClearSearchIndex removes all indexed content for a checkpoint
func (idx *Index) ClearSearchIndex(checkpointID string) error {
	_, err := idx.db.Exec(`DELETE FROM content_fts WHERE checkpoint_id = ?`, checkpointID)
	return err
}

// Search searches for query string in the files of a checkpoint and returns matching lines with context
func (idx *Index) Search(checkpointID string, query string, limit int, contextLines int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 50
	}
	if contextLines <= 0 {
		contextLines = 2
	}

	// Query matching records from content_fts
	rows, err := idx.db.Query(`
		SELECT path, content
		FROM content_fts
		WHERE checkpoint_id = ? AND content_fts MATCH ?
		LIMIT ?
	`, checkpointID, query, limit*2)
	if err != nil {
		// Fallback to LIKE if FTS query syntax error occurs
		rows, err = idx.db.Query(`
			SELECT path, content
			FROM content_fts
			WHERE checkpoint_id = ? AND content LIKE ?
			LIMIT ?
		`, checkpointID, "%"+query+"%", limit*2)
		if err != nil {
			return nil, err
		}
	}
	defer rows.Close()

	var results []SearchResult
	lowerQuery := strings.ToLower(query)

	for rows.Next() {
		var path, fullContent string
		if err := rows.Scan(&path, &fullContent); err != nil {
			return nil, err
		}

		lines := strings.Split(fullContent, "\n")
		for i, line := range lines {
			if strings.Contains(strings.ToLower(line), lowerQuery) {
				var before, after []string

				startBefore := i - contextLines
				if startBefore < 0 {
					startBefore = 0
				}
				for b := startBefore; b < i; b++ {
					before = append(before, lines[b])
				}

				endAfter := i + 1 + contextLines
				if endAfter > len(lines) {
					endAfter = len(lines)
				}
				for a := i + 1; a < endAfter; a++ {
					after = append(after, lines[a])
				}

				results = append(results, SearchResult{
					Path:          path,
					LineNumber:    i + 1,
					Content:       line,
					ContextBefore: before,
					ContextAfter:  after,
				})

				if len(results) >= limit {
					return results, nil
				}
			}
		}
	}

	return results, rows.Err()
}
