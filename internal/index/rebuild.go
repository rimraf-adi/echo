package index

import (
	"fmt"

	"github.com/echo-vcs/echo/internal/core"
	"github.com/echo-vcs/echo/internal/merkle"
)

// Rebuild drops and reconstructs the SQLite index entirely from checkpoint files and the object store
func (idx *Index) Rebuild(ws *core.Workspace) error {
	dropQuery := `
		DROP TABLE IF EXISTS files;
		DROP TABLE IF EXISTS checkpoints;
		DROP TABLE IF EXISTS refs;
		DROP TABLE IF EXISTS content_fts;
	`
	if _, err := idx.db.Exec(dropQuery); err != nil {
		return fmt.Errorf("dropping tables for rebuild: %w", err)
	}

	if err := idx.migrate(); err != nil {
		return fmt.Errorf("re-creating schema: %w", err)
	}

	checkpoints, err := core.ListCheckpoints(ws)
	if err != nil {
		return fmt.Errorf("listing checkpoints: %w", err)
	}

	for _, cp := range checkpoints {
		if err := idx.IndexCheckpoint(cp); err != nil {
			return fmt.Errorf("indexing checkpoint %s: %w", cp.ID, err)
		}

		treeData, err := ws.Store.ReadTree(cp.TreeHash)
		if err != nil {
			return fmt.Errorf("reading tree for %s: %w", cp.ID, err)
		}

		treeNode, err := merkle.DeserializeTree(treeData)
		if err != nil {
			return fmt.Errorf("deserializing tree for %s: %w", cp.ID, err)
		}

		files, err := merkle.FlattenTree(treeNode, ws.Store)
		if err != nil {
			return fmt.Errorf("flattening tree for %s: %w", cp.ID, err)
		}

		if err := idx.IndexFiles(cp.ID, files); err != nil {
			return fmt.Errorf("indexing files for %s: %w", cp.ID, err)
		}
	}

	// Index content for current HEAD for full-text search
	headID, err := core.ResolveRef(ws, "HEAD")
	if err == nil && headID != "" {
		headCP, err := core.LoadCheckpoint(ws, headID)
		if err == nil {
			treeData, err := ws.Store.ReadTree(headCP.TreeHash)
			if err == nil {
				treeNode, err := merkle.DeserializeTree(treeData)
				if err == nil {
					files, err := merkle.FlattenTree(treeNode, ws.Store)
					if err == nil {
						for path, entry := range files {
							blob, err := ws.Store.ReadBlob(entry.Hash)
							if err == nil {
								_ = idx.IndexContent(headID, path, string(blob))
							}
						}
					}
				}
			}
		}
	}

	// Index all refs
	refs, err := core.ListRefs(ws)
	if err == nil {
		for name, cpID := range refs {
			_ = idx.SetRef(name, cpID)
		}
	}

	return nil
}
