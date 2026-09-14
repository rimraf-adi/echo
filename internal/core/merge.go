package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/echo-vcs/echo/internal/lock"
	"github.com/echo-vcs/echo/internal/merkle"
	"github.com/echo-vcs/echo/internal/store"
)

func loadTree(hash string, objStore *store.ObjectStore) (*merkle.TreeNode, error) {
	data, err := objStore.ReadTree(hash)
	if err != nil {
		return nil, fmt.Errorf("reading tree %s: %w", hash, err)
	}
	return merkle.DeserializeTree(data)
}

type MergeStrategy int

const (
	StrategyTheirs MergeStrategy = iota
	StrategyOurs
)

func (s MergeStrategy) String() string {
	switch s {
	case StrategyTheirs:
		return "theirs"
	case StrategyOurs:
		return "ours"
	default:
		return "theirs"
	}
}

// Conflict describes a file that was modified in both branches
type Conflict struct {
	Path       string `json:"path"`
	Resolution string `json:"resolution"`
}

// MergeResult describes the result of a branch merge
type MergeResult struct {
	MergedBranch    string     `json:"merged_branch"`
	IntoBranch      string     `json:"into_branch"`
	MergeCheckpoint string     `json:"merge_checkpoint"`
	Parents         []string   `json:"parents"`
	Strategy        string     `json:"strategy"`
	Conflicts       []Conflict `json:"conflicts"`
	Changes         Changeset  `json:"changes"`
}

var (
	ErrNoCommonAncestor = errors.New("no common ancestor found between branches")
	ErrAlreadyUpToDate  = errors.New("already up to date")
)

// FindLCA finds the Lowest Common Ancestor of two checkpoints in the DAG
func FindLCA(ws *Workspace, cpA_ID, cpB_ID string) (string, error) {
	if cpA_ID == cpB_ID {
		return cpA_ID, nil
	}

	ancestorsA := make(map[string]bool)
	queueA := []string{cpA_ID}

	for len(queueA) > 0 {
		curr := queueA[0]
		queueA = queueA[1:]

		if ancestorsA[curr] {
			continue
		}
		ancestorsA[curr] = true

		cp, err := LoadCheckpoint(ws, curr)
		if err == nil {
			queueA = append(queueA, cp.Parents...)
		}
	}

	queueB := []string{cpB_ID}
	visitedB := make(map[string]bool)

	for len(queueB) > 0 {
		curr := queueB[0]
		queueB = queueB[1:]

		if ancestorsA[curr] {
			return curr, nil
		}

		if visitedB[curr] {
			continue
		}
		visitedB[curr] = true

		cp, err := LoadCheckpoint(ws, curr)
		if err == nil {
			queueB = append(queueB, cp.Parents...)
		}
	}

	return "", ErrNoCommonAncestor
}

// Merge merges a branch into the current branch using a deterministic conflict strategy
func Merge(ws *Workspace, branchName string, strategy MergeStrategy, dryRun bool) (*MergeResult, error) {
	l, err := lock.Acquire(ws.LockPath(), 5*time.Second)
	if err != nil {
		return nil, err
	}
	defer l.Release()

	intoBranch, err := ReadHead(ws)
	if err != nil {
		return nil, fmt.Errorf("reading current HEAD branch: %w", err)
	}

	if branchName == intoBranch {
		return nil, fmt.Errorf("cannot merge branch %s into itself", branchName)
	}

	oursID, err := GetRef(ws, intoBranch)
	if err != nil {
		return nil, fmt.Errorf("getting ref for current branch %s: %w", intoBranch, err)
	}

	theirsID, err := GetRef(ws, branchName)
	if err != nil {
		return nil, fmt.Errorf("getting ref for incoming branch %s: %w", branchName, err)
	}

	if oursID == theirsID {
		return nil, ErrAlreadyUpToDate
	}

	lcaID, err := FindLCA(ws, oursID, theirsID)
	if err != nil {
		return nil, err
	}

	if lcaID == theirsID {
		return nil, ErrAlreadyUpToDate
	}

	// Load trees
	oursCP, err := LoadCheckpoint(ws, oursID)
	if err != nil {
		return nil, err
	}
	theirsCP, err := LoadCheckpoint(ws, theirsID)
	if err != nil {
		return nil, err
	}
	baseCP, err := LoadCheckpoint(ws, lcaID)
	if err != nil {
		return nil, err
	}

	oursTree, _ := loadTree(oursCP.TreeHash, ws.Store)
	theirsTree, _ := loadTree(theirsCP.TreeHash, ws.Store)
	baseTree, _ := loadTree(baseCP.TreeHash, ws.Store)

	oursFiles, _ := merkle.FlattenTree(oursTree, ws.Store)
	theirsFiles, _ := merkle.FlattenTree(theirsTree, ws.Store)
	baseFiles, _ := merkle.FlattenTree(baseTree, ws.Store)

	allPaths := make(map[string]bool)
	for p := range oursFiles {
		allPaths[p] = true
	}
	for p := range theirsFiles {
		allPaths[p] = true
	}
	for p := range baseFiles {
		allPaths[p] = true
	}

	mergedFiles := make(map[string]merkle.TreeEntry)
	var conflicts []Conflict
	var added, modified, deleted []string

	for p := range allPaths {
		baseEntry, inBase := baseFiles[p]
		oursEntry, inOurs := oursFiles[p]
		theirsEntry, inTheirs := theirsFiles[p]

		baseHash := ""
		if inBase {
			baseHash = baseEntry.Hash
		}
		oursHash := ""
		if inOurs {
			oursHash = oursEntry.Hash
		}
		theirsHash := ""
		if inTheirs {
			theirsHash = theirsEntry.Hash
		}

		// Check changes
		oursChanged := oursHash != baseHash
		theirsChanged := theirsHash != baseHash

		if !oursChanged && !theirsChanged {
			// Unchanged in both
			if inOurs {
				mergedFiles[p] = oursEntry
			}
		} else if !oursChanged && theirsChanged {
			// Changed only in theirs
			if inTheirs {
				mergedFiles[p] = theirsEntry
				if !inBase {
					added = append(added, p)
				} else {
					modified = append(modified, p)
				}
			} else {
				deleted = append(deleted, p)
			}
		} else if oursChanged && !theirsChanged {
			// Changed only in ours
			if inOurs {
				mergedFiles[p] = oursEntry
			}
		} else {
			// Changed in both!
			if oursHash == theirsHash {
				// Both made the exact same change
				if inOurs {
					mergedFiles[p] = oursEntry
				}
			} else {
				// Conflict!
				resolution := "used theirs"
				if strategy == StrategyOurs {
					resolution = "used ours"
					if inOurs {
						mergedFiles[p] = oursEntry
					}
				} else {
					if inTheirs {
						mergedFiles[p] = theirsEntry
						if !inOurs {
							added = append(added, p)
						} else {
							modified = append(modified, p)
						}
					} else {
						deleted = append(deleted, p)
					}
				}
				conflicts = append(conflicts, Conflict{
					Path:       p,
					Resolution: resolution,
				})
			}
		}
	}

	changes := Changeset{
		Added:    added,
		Modified: modified,
		Deleted:  deleted,
	}

	res := &MergeResult{
		MergedBranch: branchName,
		IntoBranch:   intoBranch,
		Parents:      []string{oursID, theirsID},
		Strategy:     strategy.String(),
		Conflicts:    conflicts,
		Changes:      changes,
	}

	if dryRun {
		return res, nil
	}

	// Apply merged state to working directory
	// 1. Delete files removed in merge
	for _, p := range deleted {
		fullPath := filepath.Join(ws.RootDir, filepath.FromSlash(p))
		_ = os.Remove(fullPath)
	}

	// 2. Write files added/modified from theirs
	for _, p := range append(added, modified...) {
		entry := mergedFiles[p]
		data, err := ws.Store.ReadBlob(entry.Hash)
		if err != nil {
			return nil, fmt.Errorf("reading blob for %s: %w", p, err)
		}
		fullPath := filepath.Join(ws.RootDir, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			return nil, err
		}
		mode := os.FileMode(0644)
		if octalMode, err := strconv.ParseUint(entry.Mode, 8, 32); err == nil {
			mode = os.FileMode(octalMode & 0777)
		}
		if err := os.WriteFile(fullPath, data, mode); err != nil {
			return nil, err
		}
	}

	// Build new Merkle tree for merged state
	scannedMerged := make(map[string]merkle.ScannedFile)
	for p, entry := range mergedFiles {
		scannedMerged[p] = merkle.ScannedFile{
			Path: p,
			Size: entry.Size,
			Hash: entry.Hash,
		}
	}

	newTree, err := merkle.BuildTreeFromFiles(scannedMerged, ws.Store)
	if err != nil {
		return nil, fmt.Errorf("building merged tree: %w", err)
	}

	mergeCP_ID := GenerateCheckpointID()
	mergeCP := &Checkpoint{
		ID:        mergeCP_ID,
		Parents:   []string{oursID, theirsID},
		TreeHash:  newTree.Hash,
		CreatedAt: time.Now().UTC(),
		Metadata: CheckpointMetadata{
			Message: fmt.Sprintf("merge branch %s into %s", branchName, intoBranch),
		},
		Changeset: changes,
		Stats: CheckpointStats{
			TotalFiles: len(mergedFiles),
		},
	}

	cpData, err := json.MarshalIndent(mergeCP, "", "  ")
	if err != nil {
		return nil, err
	}
	cpFile := filepath.Join(ws.CheckpointsDir(), mergeCP_ID+".json")
	if err := os.WriteFile(cpFile, cpData, 0644); err != nil {
		return nil, fmt.Errorf("writing merge checkpoint: %w", err)
	}

	if err := SetRef(ws, intoBranch, mergeCP_ID); err != nil {
		return nil, fmt.Errorf("updating ref %s: %w", intoBranch, err)
	}

	res.MergeCheckpoint = mergeCP_ID
	return res, nil
}
