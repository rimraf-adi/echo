package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/echo-vcs/echo/internal/merkle"
	"github.com/echo-vcs/echo/internal/scan"
	"github.com/sergi/go-diff/diffmatchpatch"
)

// DiffLine represents a single line in a diff hunk
type DiffLine struct {
	Type    string `json:"type"` // "context", "add", "delete"
	Content string `json:"content"`
}

// DiffHunk represents a unified diff hunk
type DiffHunk struct {
	OldStart int        `json:"old_start"`
	OldCount int        `json:"old_count"`
	NewStart int        `json:"new_start"`
	NewCount int        `json:"new_count"`
	Lines    []DiffLine `json:"lines"`
}

// FileDiff represents line-level diff details for a single file
type FileDiff struct {
	Path         string     `json:"path"`
	Type         string     `json:"type"` // "added", "modified", "deleted"
	OldHash      string     `json:"old_hash,omitempty"`
	NewHash      string     `json:"new_hash,omitempty"`
	LinesAdded   int        `json:"lines_added"`
	LinesRemoved int        `json:"lines_removed"`
	Hunks        []DiffHunk `json:"hunks,omitempty"`
}

// DiffResult holds the complete diff between two states
type DiffResult struct {
	From  string     `json:"from"`
	To    string     `json:"to"`
	Files []FileDiff `json:"files"`
}

// DiffCheckpoints computes line-level diffs between two checkpoints
func DiffCheckpoints(ws *Workspace, fromRef, toRef string) (*DiffResult, error) {
	fromID, err := ResolveRef(ws, fromRef)
	if err != nil {
		return nil, fmt.Errorf("resolving from ref %s: %w", fromRef, err)
	}
	toID, err := ResolveRef(ws, toRef)
	if err != nil {
		return nil, fmt.Errorf("resolving to ref %s: %w", toRef, err)
	}

	fromCP, err := LoadCheckpoint(ws, fromID)
	if err != nil {
		return nil, err
	}
	toCP, err := LoadCheckpoint(ws, toID)
	if err != nil {
		return nil, err
	}

	fromTreeBytes, err := ws.Store.ReadTree(fromCP.TreeHash)
	if err != nil {
		return nil, err
	}
	fromTree, err := merkle.DeserializeTree(fromTreeBytes)
	if err != nil {
		return nil, err
	}

	toTreeBytes, err := ws.Store.ReadTree(toCP.TreeHash)
	if err != nil {
		return nil, err
	}
	toTree, err := merkle.DeserializeTree(toTreeBytes)
	if err != nil {
		return nil, err
	}

	deltas, err := merkle.DiffTrees(fromTree, toTree, ws.Store)
	if err != nil {
		return nil, err
	}

	res := &DiffResult{
		From:  fromID,
		To:    toID,
		Files: make([]FileDiff, 0, len(deltas)),
	}

	for _, d := range deltas {
		fd, err := computeFileDiff(ws, d)
		if err != nil {
			return nil, err
		}
		res.Files = append(res.Files, fd)
	}

	return res, nil
}

// DiffWorking computes differences between working directory and a base checkpoint (default HEAD)
func DiffWorking(ws *Workspace, baseRef string) (*DiffResult, error) {
	if baseRef == "" {
		baseRef = "HEAD"
	}
	baseID, err := ResolveRef(ws, baseRef)
	if err != nil {
		return nil, err
	}

	baseCP, err := LoadCheckpoint(ws, baseID)
	if err != nil {
		return nil, err
	}

	baseTreeBytes, err := ws.Store.ReadTree(baseCP.TreeHash)
	if err != nil {
		return nil, err
	}
	baseTree, err := merkle.DeserializeTree(baseTreeBytes)
	if err != nil {
		return nil, err
	}

	// Scan working directory
	scanner := scan.NewScanner(ws.Matcher)
	scanRes, err := scanner.Scan(ws.RootDir)
	if err != nil {
		return nil, err
	}
	if err := scan.HashFiles(ws.RootDir, scanRes.Files, 0); err != nil {
		return nil, err
	}

	// Build working tree
	merkleFiles := make(map[string]merkle.ScannedFile)
	for p, fi := range scanRes.Files {
		merkleFiles[p] = merkle.ScannedFile{
			Path: p,
			Size: fi.Size,
			Mode: fi.Mode,
			Hash: fi.Hash,
		}
	}

	workingTree, err := merkle.BuildTreeFromFiles(merkleFiles, ws.Store)
	if err != nil {
		return nil, err
	}

	deltas, err := merkle.DiffTrees(baseTree, workingTree, ws.Store)
	if err != nil {
		return nil, err
	}

	res := &DiffResult{
		From:  baseID,
		To:    "working-tree",
		Files: make([]FileDiff, 0, len(deltas)),
	}

	for _, d := range deltas {
		fd, err := computeWorkingFileDiff(ws, d)
		if err != nil {
			return nil, err
		}
		res.Files = append(res.Files, fd)
	}

	return res, nil
}

func computeFileDiff(ws *Workspace, d merkle.Delta) (FileDiff, error) {
	fd := FileDiff{
		Path:    d.Path,
		Type:    d.Type.String(),
		OldHash: d.OldHash,
		NewHash: d.NewHash,
	}

	var oldText, newText string
	if d.OldHash != "" {
		oldBytes, err := ws.Store.ReadBlob(d.OldHash)
		if err == nil {
			oldText = string(oldBytes)
		}
	}
	if d.NewHash != "" {
		newBytes, err := ws.Store.ReadBlob(d.NewHash)
		if err == nil {
			newText = string(newBytes)
		}
	}

	hunks, added, removed := generateHunks(oldText, newText)
	fd.Hunks = hunks
	fd.LinesAdded = added
	fd.LinesRemoved = removed
	return fd, nil
}

func computeWorkingFileDiff(ws *Workspace, d merkle.Delta) (FileDiff, error) {
	fd := FileDiff{
		Path:    d.Path,
		Type:    d.Type.String(),
		OldHash: d.OldHash,
		NewHash: d.NewHash,
	}

	var oldText, newText string
	if d.OldHash != "" {
		oldBytes, err := ws.Store.ReadBlob(d.OldHash)
		if err == nil {
			oldText = string(oldBytes)
		}
	}

	fullPath := filepath.Join(ws.RootDir, filepath.FromSlash(d.Path))
	data, err := os.ReadFile(fullPath)
	if err == nil {
		newText = string(data)
	}

	hunks, added, removed := generateHunks(oldText, newText)
	fd.Hunks = hunks
	fd.LinesAdded = added
	fd.LinesRemoved = removed
	return fd, nil
}

func generateHunks(textA, textB string) ([]DiffHunk, int, int) {
	dmp := diffmatchpatch.New()
	aLines, bLines, lineArray := dmp.DiffLinesToChars(textA, textB)
	diffs := dmp.DiffMain(aLines, bLines, false)
	diffs = dmp.DiffCharsToLines(diffs, lineArray)

	var lines []DiffLine
	added := 0
	removed := 0

	for _, diff := range diffs {
		content := diff.Text
		// Split by newline while preserving individual lines
		splitLines := strings.Split(strings.TrimSuffix(content, "\n"), "\n")
		for _, sl := range splitLines {
			switch diff.Type {
			case diffmatchpatch.DiffEqual:
				lines = append(lines, DiffLine{Type: "context", Content: sl})
			case diffmatchpatch.DiffInsert:
				lines = append(lines, DiffLine{Type: "add", Content: sl})
				added++
			case diffmatchpatch.DiffDelete:
				lines = append(lines, DiffLine{Type: "delete", Content: sl})
				removed++
			}
		}
	}

	if added == 0 && removed == 0 {
		return nil, 0, 0
	}

	hunk := DiffHunk{
		OldStart: 1,
		OldCount: len(strings.Split(textA, "\n")),
		NewStart: 1,
		NewCount: len(strings.Split(textB, "\n")),
		Lines:    lines,
	}

	return []DiffHunk{hunk}, added, removed
}
