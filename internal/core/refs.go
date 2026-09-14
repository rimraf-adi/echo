package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var (
	ErrRefNotFound       = errors.New("reference not found")
	ErrAmbiguousPrefix   = errors.New("ambiguous checkpoint prefix")
	ErrInvalidRefFormat  = errors.New("invalid reference format")
	ErrNoParentAvailable = errors.New("checkpoint has no parent (reached root)")
)

// ReadHead reads the current branch name from .echo/HEAD
func ReadHead(ws *Workspace) (string, error) {
	headPath := filepath.Join(ws.EchoDir, HeadFileName)
	data, err := os.ReadFile(headPath)
	if err != nil {
		return "", fmt.Errorf("reading HEAD: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

// WriteHead updates .echo/HEAD to point to a branch name
func WriteHead(ws *Workspace, branchName string) error {
	headPath := filepath.Join(ws.EchoDir, HeadFileName)
	return os.WriteFile(headPath, []byte(branchName+"\n"), 0644)
}

// GetRef reads the checkpoint ID that a branch points to
func GetRef(ws *Workspace, refName string) (string, error) {
	refPath := filepath.Join(ws.RefsDir(), refName)
	data, err := os.ReadFile(refPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("%w: %s", ErrRefNotFound, refName)
		}
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// SetRef updates a branch ref to point to a checkpoint ID
func SetRef(ws *Workspace, refName string, checkpointID string) error {
	refPath := filepath.Join(ws.RefsDir(), refName)
	if err := os.MkdirAll(filepath.Dir(refPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(refPath, []byte(checkpointID+"\n"), 0644)
}

// DeleteRef removes a branch ref
func DeleteRef(ws *Workspace, refName string) error {
	refPath := filepath.Join(ws.RefsDir(), refName)
	if err := os.Remove(refPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// ListRefs returns a map of all branch names to their checkpoint IDs
func ListRefs(ws *Workspace) (map[string]string, error) {
	refs := make(map[string]string)
	entries, err := os.ReadDir(ws.RefsDir())
	if err != nil {
		if os.IsNotExist(err) {
			return refs, nil
		}
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		id, err := GetRef(ws, entry.Name())
		if err == nil {
			refs[entry.Name()] = id
		}
	}
	return refs, nil
}

// ResolveRef resolves any reference (HEAD, HEAD~n, branch name, checkpoint ID, prefix) to a full checkpoint ID
func ResolveRef(ws *Workspace, ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", ErrRefNotFound
	}

	// Case 1: "HEAD"
	if ref == "HEAD" {
		branch, err := ReadHead(ws)
		if err != nil {
			return "", err
		}
		return GetRef(ws, branch)
	}

	// Case 2: "HEAD~n"
	if strings.HasPrefix(ref, "HEAD~") {
		nStr := strings.TrimPrefix(ref, "HEAD~")
		n, err := strconv.Atoi(nStr)
		if err != nil || n < 1 {
			return "", fmt.Errorf("%w: invalid ancestor index %s", ErrInvalidRefFormat, nStr)
		}

		currentID, err := ResolveRef(ws, "HEAD")
		if err != nil {
			return "", err
		}

		for i := 0; i < n; i++ {
			cp, err := LoadCheckpoint(ws, currentID)
			if err != nil {
				return "", err
			}
			if len(cp.Parents) == 0 {
				return "", fmt.Errorf("%w at %s", ErrNoParentAvailable, currentID)
			}
			currentID = cp.Parents[0]
		}
		return currentID, nil
	}

	// Case 3: Branch name in .echo/refs/
	if id, err := GetRef(ws, ref); err == nil {
		return id, nil
	}

	// Case 4: Exact checkpoint file exists
	exactPath := filepath.Join(ws.CheckpointsDir(), ref+".json")
	if _, err := os.Stat(exactPath); err == nil {
		return ref, nil
	}

	// Case 5: Prefix matching
	entries, err := os.ReadDir(ws.CheckpointsDir())
	if err != nil {
		return "", err
	}

	var matches []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		cpID := strings.TrimSuffix(entry.Name(), ".json")
		if strings.HasPrefix(cpID, ref) {
			matches = append(matches, cpID)
		}
	}

	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("%w: %s matches %v", ErrAmbiguousPrefix, ref, matches)
	}

	return "", fmt.Errorf("%w: %s", ErrRefNotFound, ref)
}

var (
	ErrBranchAlreadyExists = errors.New("branch already exists")
	ErrCannotDeleteCurrent = errors.New("cannot delete current active branch")
)

// BranchInfo holds information about a named branch
type BranchInfo struct {
	Name         string    `json:"name"`
	CheckpointID string    `json:"checkpoint_id"`
	IsCurrent    bool      `json:"is_current"`
	CreatedAt    time.Time `json:"created_at"`
}

// CreateBranch creates a new named branch pointing to fromRef (or HEAD if empty)
func CreateBranch(ws *Workspace, name string, fromRef string) error {
	if fromRef == "" {
		fromRef = "HEAD"
	}
	targetID, err := ResolveRef(ws, fromRef)
	if err != nil {
		return err
	}

	if _, err := GetRef(ws, name); err == nil {
		return fmt.Errorf("%w: %s", ErrBranchAlreadyExists, name)
	}

	return SetRef(ws, name, targetID)
}

// SwitchBranch switches the current branch and updates the working directory to that branch's HEAD
func SwitchBranch(ws *Workspace, name string) error {
	targetID, err := GetRef(ws, name)
	if err != nil {
		return fmt.Errorf("branch %s not found: %w", name, err)
	}

	// Update working directory to match branch state without creating a new checkpoint
	if _, err := Revert(ws, targetID, false, false); err != nil {
		return fmt.Errorf("updating working directory for branch switch: %w", err)
	}

	return WriteHead(ws, name)
}

// DeleteBranch removes a branch reference
func DeleteBranch(ws *Workspace, name string) error {
	current, err := ReadHead(ws)
	if err == nil && current == name {
		return fmt.Errorf("%w: %s", ErrCannotDeleteCurrent, name)
	}

	return DeleteRef(ws, name)
}

// ListBranches returns all branches with metadata
func ListBranches(ws *Workspace) ([]BranchInfo, error) {
	currentBranch, _ := ReadHead(ws)
	refs, err := ListRefs(ws)
	if err != nil {
		return nil, err
	}

	var branches []BranchInfo
	for name, cpID := range refs {
		var createdAt time.Time
		cp, err := LoadCheckpoint(ws, cpID)
		if err == nil {
			createdAt = cp.CreatedAt
		}

		branches = append(branches, BranchInfo{
			Name:         name,
			CheckpointID: cpID,
			IsCurrent:    name == currentBranch,
			CreatedAt:    createdAt,
		})
	}

	return branches, nil
}

