package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/echo-vcs/echo/internal/lock"
	"github.com/echo-vcs/echo/internal/merkle"
	"github.com/echo-vcs/echo/internal/scan"
)

// RevertResult describes the outcome of a revert operation
type RevertResult struct {
	RevertedTo       string    `json:"reverted_to"`
	RevertCheckpoint string    `json:"revert_checkpoint,omitempty"`
	ChangesApplied   Changeset `json:"changes_applied"`
}

// Revert restores the workspace to match a historical checkpoint
func Revert(ws *Workspace, targetRef string, dryRun bool, createCP bool) (*RevertResult, error) {
	l, err := lock.Acquire(ws.LockPath(), 5*time.Second)
	if err != nil {
		return nil, err
	}
	defer l.Release()

	targetID, err := ResolveRef(ws, targetRef)
	if err != nil {
		return nil, fmt.Errorf("resolving target checkpoint %s: %w", targetRef, err)
	}

	targetCP, err := LoadCheckpoint(ws, targetID)
	if err != nil {
		return nil, fmt.Errorf("loading target checkpoint %s: %w", targetID, err)
	}

	targetTreeBytes, err := ws.Store.ReadTree(targetCP.TreeHash)
	if err != nil {
		return nil, fmt.Errorf("reading target tree: %w", err)
	}
	targetTree, err := merkle.DeserializeTree(targetTreeBytes)
	if err != nil {
		return nil, fmt.Errorf("deserializing target tree: %w", err)
	}

	targetFiles, err := merkle.FlattenTree(targetTree, ws.Store)
	if err != nil {
		return nil, fmt.Errorf("flattening target tree: %w", err)
	}

	// Scan working directory
	scanner := scan.NewScanner(ws.Matcher)
	scanRes, err := scanner.Scan(ws.RootDir)
	if err != nil {
		return nil, fmt.Errorf("scanning workspace: %w", err)
	}
	if err := scan.HashFiles(ws.RootDir, scanRes.Files, 0); err != nil {
		return nil, fmt.Errorf("hashing working files: %w", err)
	}

	var toRestore []string
	var toModify []string
	var toDelete []string

	// Check files to restore or modify
	for path, entry := range targetFiles {
		current, exists := scanRes.Files[path]
		if !exists {
			toRestore = append(toRestore, path)
		} else if current.Hash != entry.Hash {
			toModify = append(toModify, path)
		}
	}

	// Check files to delete
	for path := range scanRes.Files {
		if _, inTarget := targetFiles[path]; !inTarget {
			toDelete = append(toDelete, path)
		}
	}

	changes := Changeset{
		Added:    toRestore,
		Modified: toModify,
		Deleted:  toDelete,
	}

	res := &RevertResult{
		RevertedTo:     targetID,
		ChangesApplied: changes,
	}

	if dryRun {
		return res, nil
	}

	// 1. Delete files not present in target checkpoint
	for _, path := range toDelete {
		fullPath := filepath.Join(ws.RootDir, filepath.FromSlash(path))
		_ = os.Remove(fullPath)
	}

	// 2. Write/restore files from target checkpoint
	for _, path := range append(toRestore, toModify...) {
		entry := targetFiles[path]
		data, err := ws.Store.ReadBlob(entry.Hash)
		if err != nil {
			return nil, fmt.Errorf("reading blob for %s: %w", path, err)
		}

		fullPath := filepath.Join(ws.RootDir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			return nil, fmt.Errorf("creating directory for %s: %w", path, err)
		}

		mode := os.FileMode(0644)
		if octalMode, err := strconv.ParseUint(entry.Mode, 8, 32); err == nil {
			mode = os.FileMode(octalMode & 0777)
		}

		if err := os.WriteFile(fullPath, data, mode); err != nil {
			return nil, fmt.Errorf("writing restored file %s: %w", path, err)
		}
	}

	// 3. Create revert checkpoint if requested
	if createCP {
		headBranch, err := ReadHead(ws)
		if err != nil {
			return nil, err
		}
		headID, _ := GetRef(ws, headBranch)

		var parents []string
		if headID != "" {
			parents = []string{headID}
		}

		newCP_ID := GenerateCheckpointID()
		newCP := &Checkpoint{
			ID:        newCP_ID,
			Parents:   parents,
			TreeHash:  targetCP.TreeHash,
			CreatedAt: time.Now().UTC(),
			Metadata: CheckpointMetadata{
				Message: fmt.Sprintf("revert to %s", targetID),
			},
			Changeset: changes,
			Stats: CheckpointStats{
				TotalFiles: len(targetFiles),
			},
		}

		cpData, err := json.MarshalIndent(newCP, "", "  ")
		if err != nil {
			return nil, err
		}
		cpFile := filepath.Join(ws.CheckpointsDir(), newCP_ID+".json")
		if err := os.WriteFile(cpFile, cpData, 0644); err != nil {
			return nil, fmt.Errorf("writing revert checkpoint: %w", err)
		}

		if err := SetRef(ws, headBranch, newCP_ID); err != nil {
			return nil, fmt.Errorf("updating ref %s: %w", headBranch, err)
		}

		res.RevertCheckpoint = newCP_ID
	}

	return res, nil
}
