package core

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/echo-vcs/echo/internal/lock"
	"github.com/echo-vcs/echo/internal/merkle"
	"github.com/echo-vcs/echo/internal/scan"
)

// GenerateCheckpointID creates a standard formatted checkpoint ID: cp-YYYYMMDD-HHMMSS-<8-hex>
func GenerateCheckpointID() string {
	now := time.Now().UTC()
	randBytes := make([]byte, 4)
	_, _ = rand.Read(randBytes)
	return fmt.Sprintf("cp-%s-%s-%s",
		now.Format("20060102"),
		now.Format("150405"),
		hex.EncodeToString(randBytes),
	)
}

// CheckpointMetadata stores agent and task metadata
type CheckpointMetadata struct {
	Agent   string         `json:"agent,omitempty"`
	Task    string         `json:"task,omitempty"`
	Model   string         `json:"model,omitempty"`
	Tags    []string       `json:"tags,omitempty"`
	Custom  map[string]any `json:"custom,omitempty"`
	Message string         `json:"message,omitempty"`
}

// CheckpointStats stores metrics for a checkpoint
type CheckpointStats struct {
	TotalFiles     int   `json:"total_files"`
	TotalSizeBytes int64 `json:"total_size_bytes"`
	BlobsReused    int   `json:"blobs_reused"`
	BlobsNew       int   `json:"blobs_new"`
	DurationMs     int64 `json:"duration_ms"`
}

// Checkpoint represents an immutable snapshot of the codebase
type Checkpoint struct {
	ID        string             `json:"id"`
	Parents   []string           `json:"parents"`
	TreeHash  string             `json:"tree_hash"`
	CreatedAt time.Time          `json:"created_at"`
	Metadata  CheckpointMetadata `json:"metadata"`
	Changeset Changeset          `json:"changeset"`
	Stats     CheckpointStats    `json:"stats"`
}

// CheckpointOpts options for creating a new checkpoint
type CheckpointOpts struct {
	Agent   string
	Task    string
	Model   string
	Tags    []string
	Custom  map[string]any
	Message string
}

// LoadCheckpoint loads a checkpoint by exact or shorthand ID
func LoadCheckpoint(ws *Workspace, refOrID string) (*Checkpoint, error) {
	id, err := ResolveRef(ws, refOrID)
	if err != nil {
		return nil, err
	}

	path := filepath.Join(ws.CheckpointsDir(), id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading checkpoint %s: %w", id, err)
	}

	var cp Checkpoint
	if err := json.Unmarshal(data, &cp); err != nil {
		return nil, fmt.Errorf("parsing checkpoint %s: %w", id, err)
	}

	return &cp, nil
}

// ListCheckpoints returns all checkpoints in the workspace sorted by CreatedAt descending
func ListCheckpoints(ws *Workspace) ([]*Checkpoint, error) {
	entries, err := os.ReadDir(ws.CheckpointsDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var checkpoints []*Checkpoint
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		cpID := strings.TrimSuffix(entry.Name(), ".json")
		cp, err := LoadCheckpoint(ws, cpID)
		if err == nil {
			checkpoints = append(checkpoints, cp)
		}
	}

	sort.Slice(checkpoints, func(i, j int) bool {
		return checkpoints[i].CreatedAt.After(checkpoints[j].CreatedAt)
	})

	return checkpoints, nil
}

// CreateCheckpoint captures the current workspace state and creates an immutable checkpoint.
// If no changes are detected against HEAD, it returns nil, nil.
func CreateCheckpoint(ws *Workspace, opts CheckpointOpts) (*Checkpoint, error) {
	l, err := lock.Acquire(ws.LockPath(), 5*time.Second)
	if err != nil {
		return nil, err
	}
	defer l.Release()

	startTime := time.Now()

	// Scan working directory
	scanner := scan.NewScanner(ws.Matcher)
	scanRes, err := scanner.Scan(ws.RootDir)
	if err != nil {
		return nil, fmt.Errorf("scanning workspace: %w", err)
	}

	if err := scan.HashFiles(ws.RootDir, scanRes.Files, 0); err != nil {
		return nil, fmt.Errorf("hashing workspace files: %w", err)
	}

	// Read HEAD branch and resolve parent checkpoint
	headBranch, err := ReadHead(ws)
	if err != nil {
		return nil, fmt.Errorf("reading HEAD branch: %w", err)
	}

	parentID, err := GetRef(ws, headBranch)
	var parentCP *Checkpoint
	var parentTree *merkle.TreeNode

	if err == nil && parentID != "" {
		parentCP, err = LoadCheckpoint(ws, parentID)
		if err != nil {
			return nil, fmt.Errorf("loading parent checkpoint %s: %w", parentID, err)
		}
		treeBytes, err := ws.Store.ReadTree(parentCP.TreeHash)
		if err != nil {
			return nil, fmt.Errorf("reading parent tree: %w", err)
		}
		parentTree, err = merkle.DeserializeTree(treeBytes)
		if err != nil {
			return nil, fmt.Errorf("deserializing parent tree: %w", err)
		}
	}

	// Check if changes exist against parent tree
	var headFiles map[string]merkle.TreeEntry
	if parentTree != nil {
		headFiles, err = merkle.FlattenTree(parentTree, ws.Store)
		if err != nil {
			return nil, fmt.Errorf("flattening parent tree: %w", err)
		}

		// Quick comparison: if file count matches, check each file's hash
		if len(headFiles) == len(scanRes.Files) {
			identical := true
			for p, fi := range scanRes.Files {
				hf, exists := headFiles[p]
				if !exists || hf.Hash != fi.Hash {
					identical = false
					break
				}
			}
			if identical {
				return nil, nil // No changes to checkpoint
			}
		}
	}

	// Store new blobs and prepare files for Merkle tree construction
	merkleFiles := make(map[string]merkle.ScannedFile)
	var totalSize int64
	var blobsReused, blobsNew int

	for path, fi := range scanRes.Files {
		totalSize += fi.Size

		// Check if blob already exists in parent tree
		if headFiles != nil {
			if hf, exists := headFiles[path]; exists && hf.Hash == fi.Hash {
				blobsReused++
				merkleFiles[path] = merkle.ScannedFile{
					Path: path,
					Size: fi.Size,
					Mode: fi.Mode,
					Hash: fi.Hash,
				}
				continue
			}
		}

		// New or changed file: read content and write blob
		fullPath := filepath.Join(ws.RootDir, filepath.FromSlash(path))
		var content []byte
		if fi.Mode&os.ModeSymlink != 0 {
			target, rerr := os.Readlink(fullPath)
			if rerr != nil {
				return nil, rerr
			}
			content = []byte(target)
		} else {
			data, rerr := os.ReadFile(fullPath)
			if rerr != nil {
				return nil, rerr
			}
			content = data
		}

		hash, err := ws.Store.WriteBlob(content)
		if err != nil {
			return nil, fmt.Errorf("storing blob for %s: %w", path, err)
		}

		blobsNew++
		merkleFiles[path] = merkle.ScannedFile{
			Path: path,
			Size: fi.Size,
			Mode: fi.Mode,
			Hash: hash,
		}
	}

	// Build new Merkle tree
	newTree, err := merkle.BuildTreeFromFiles(merkleFiles, ws.Store)
	if err != nil {
		return nil, fmt.Errorf("building Merkle tree: %w", err)
	}

	// Compute changeset deltas
	deltas, err := merkle.DiffTrees(parentTree, newTree, ws.Store)
	if err != nil {
		return nil, fmt.Errorf("computing changeset deltas: %w", err)
	}

	changeset := GroupDeltas(deltas)

	var parents []string
	if parentCP != nil {
		parents = []string{parentCP.ID}
	}

	cpID := GenerateCheckpointID()
	cp := &Checkpoint{
		ID:        cpID,
		Parents:   parents,
		TreeHash:  newTree.Hash,
		CreatedAt: time.Now().UTC(),
		Metadata: CheckpointMetadata{
			Agent:   opts.Agent,
			Task:    opts.Task,
			Model:   opts.Model,
			Tags:    opts.Tags,
			Custom:  opts.Custom,
			Message: opts.Message,
		},
		Changeset: changeset,
		Stats: CheckpointStats{
			TotalFiles:     len(merkleFiles),
			TotalSizeBytes: totalSize,
			BlobsReused:    blobsReused,
			BlobsNew:       blobsNew,
			DurationMs:     time.Since(startTime).Milliseconds(),
		},
	}

	// Write checkpoint JSON
	cpData, err := json.MarshalIndent(cp, "", "  ")
	if err != nil {
		return nil, err
	}
	cpFile := filepath.Join(ws.CheckpointsDir(), cpID+".json")
	if err := os.WriteFile(cpFile, cpData, 0644); err != nil {
		return nil, fmt.Errorf("writing checkpoint JSON: %w", err)
	}

	// Update branch ref
	if err := SetRef(ws, headBranch, cpID); err != nil {
		return nil, fmt.Errorf("updating ref %s: %w", headBranch, err)
	}

	return cp, nil
}
