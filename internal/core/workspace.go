package core

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/echo-vcs/echo/internal/ignore"
	"github.com/echo-vcs/echo/internal/merkle"
	"github.com/echo-vcs/echo/internal/scan"
	"github.com/echo-vcs/echo/internal/store"
)

const (
	EchoDirName       = ".echo"
	ConfigFileName    = "config.json"
	HeadFileName      = "HEAD"
	RefsDirName       = "refs"
	ObjectsDirName    = "objects"
	CheckpointsDirName = "checkpoints"
	IgnoreFileName    = "ignore"
	LockFileName      = "lock"
	DefaultBranch     = "main"
)

var (
	ErrWorkspaceAlreadyExists = errors.New("workspace already initialized")
	ErrWorkspaceNotFound      = errors.New("not an Echo workspace (no .echo directory found)")
)

// Config represents the workspace configuration in .echo/config.json
type Config struct {
	Version       int       `json:"version"`
	WorkspaceID   string    `json:"workspace_id"`
	CreatedAt     time.Time `json:"created_at"`
	DefaultBranch string    `json:"default_branch"`
}

// Workspace represents an active Echo workspace
type Workspace struct {
	RootDir string
	EchoDir string
	Config  Config
	Store   *store.ObjectStore
	Matcher *ignore.Matcher
}

// LockPath returns the path to the workspace lockfile
func (w *Workspace) LockPath() string {
	return filepath.Join(w.EchoDir, LockFileName)
}

// RefsDir returns the path to the refs directory
func (w *Workspace) RefsDir() string {
	return filepath.Join(w.EchoDir, RefsDirName)
}

// CheckpointsDir returns the path to the checkpoints directory
func (w *Workspace) CheckpointsDir() string {
	return filepath.Join(w.EchoDir, CheckpointsDirName)
}

// FindRoot walks upwards from dir to find the .echo directory
func FindRoot(startDir string) (string, error) {
	curr, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}

	for {
		candidate := filepath.Join(curr, EchoDirName)
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return curr, nil
		}

		parent := filepath.Dir(curr)
		if parent == curr {
			return "", ErrWorkspaceNotFound
		}
		curr = parent
	}
}

// Open opens an existing Echo workspace from or above startDir
func Open(startDir string) (*Workspace, error) {
	root, err := FindRoot(startDir)
	if err != nil {
		return nil, err
	}

	echoDir := filepath.Join(root, EchoDirName)
	configPath := filepath.Join(echoDir, ConfigFileName)
	configBytes, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("reading workspace config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(configBytes, &cfg); err != nil {
		return nil, fmt.Errorf("parsing workspace config: %w", err)
	}

	objStore := store.NewObjectStore(filepath.Join(echoDir, ObjectsDirName))

	ignorePath := filepath.Join(echoDir, IgnoreFileName)
	matcher, err := ignore.LoadIgnoreFile(ignorePath)
	if err != nil {
		matcher = ignore.NewMatcher(nil)
	}

	return &Workspace{
		RootDir: root,
		EchoDir: echoDir,
		Config:  cfg,
		Store:   objStore,
		Matcher: matcher,
	}, nil
}

// Init initializes a new Echo workspace in rootDir and creates the initial checkpoint
func Init(rootDir string, customIgnorePath string) (*Workspace, *Checkpoint, error) {
	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, nil, err
	}

	echoDir := filepath.Join(absRoot, EchoDirName)
	if _, err := os.Stat(echoDir); err == nil {
		return nil, nil, ErrWorkspaceAlreadyExists
	}

	// Create directory structure
	for _, sub := range []string{RefsDirName, ObjectsDirName, CheckpointsDirName} {
		if err := os.MkdirAll(filepath.Join(echoDir, sub), 0755); err != nil {
			return nil, nil, fmt.Errorf("creating directory %s: %w", sub, err)
		}
	}

	// Generate workspace ID
	randBytes := make([]byte, 4)
	_, _ = rand.Read(randBytes)
	wsID := fmt.Sprintf("ws-%x", randBytes)

	cfg := Config{
		Version:       1,
		WorkspaceID:   wsID,
		CreatedAt:     time.Now().UTC(),
		DefaultBranch: DefaultBranch,
	}

	cfgData, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, nil, err
	}
	if err := os.WriteFile(filepath.Join(echoDir, ConfigFileName), cfgData, 0644); err != nil {
		return nil, nil, fmt.Errorf("writing config: %w", err)
	}

	// Setup HEAD pointing to default branch
	if err := os.WriteFile(filepath.Join(echoDir, HeadFileName), []byte(DefaultBranch+"\n"), 0644); err != nil {
		return nil, nil, fmt.Errorf("writing HEAD: %w", err)
	}

	// Setup ignore file
	destIgnore := filepath.Join(echoDir, IgnoreFileName)
	if customIgnorePath != "" {
		content, err := os.ReadFile(customIgnorePath)
		if err == nil {
			_ = os.WriteFile(destIgnore, content, 0644)
		}
	} else {
		// Write standard starter ignore
		starter := "# Echo ignore patterns\nnode_modules/\n*.log\n.DS_Store\nbuild/\ndist/\n"
		_ = os.WriteFile(destIgnore, []byte(starter), 0644)
	}

	matcher, err := ignore.LoadIgnoreFile(destIgnore)
	if err != nil {
		matcher = ignore.NewMatcher(nil)
	}

	objStore := store.NewObjectStore(filepath.Join(echoDir, ObjectsDirName))
	ws := &Workspace{
		RootDir: absRoot,
		EchoDir: echoDir,
		Config:  cfg,
		Store:   objStore,
		Matcher: matcher,
	}

	// Scan workspace for initial state
	scanner := scan.NewScanner(matcher)
	scanRes, err := scanner.Scan(absRoot)
	if err != nil {
		return nil, nil, fmt.Errorf("scanning workspace: %w", err)
	}

	if err := scan.HashFiles(absRoot, scanRes.Files, 0); err != nil {
		return nil, nil, fmt.Errorf("hashing files: %w", err)
	}

	// Store blobs
	merkleFiles := make(map[string]merkle.ScannedFile)
	var totalSize int64
	var addedFiles []string

	for path, fi := range scanRes.Files {
		fullPath := filepath.Join(absRoot, filepath.FromSlash(path))
		var content []byte
		if fi.Mode&os.ModeSymlink != 0 {
			target, rerr := os.Readlink(fullPath)
			if rerr != nil {
				return nil, nil, rerr
			}
			content = []byte(target)
		} else {
			data, rerr := os.ReadFile(fullPath)
			if rerr != nil {
				return nil, nil, rerr
			}
			content = data
		}

		hash, err := objStore.WriteBlob(content)
		if err != nil {
			return nil, nil, fmt.Errorf("writing blob for %s: %w", path, err)
		}

		merkleFiles[path] = merkle.ScannedFile{
			Path: path,
			Size: fi.Size,
			Mode: fi.Mode,
			Hash: hash,
		}
		totalSize += fi.Size
		addedFiles = append(addedFiles, path)
	}

	// Build Merkle tree
	rootTree, err := merkle.BuildTreeFromFiles(merkleFiles, objStore)
	if err != nil {
		return nil, nil, fmt.Errorf("building Merkle tree: %w", err)
	}

	// Create initial checkpoint
	cpID := GenerateCheckpointID()
	cp := &Checkpoint{
		ID:        cpID,
		Parents:   []string{},
		TreeHash:  rootTree.Hash,
		CreatedAt: time.Now().UTC(),
		Metadata: CheckpointMetadata{
			Message: "initial checkpoint",
		},
		Changeset: Changeset{
			Added:    addedFiles,
			Modified: []string{},
			Deleted:  []string{},
		},
		Stats: CheckpointStats{
			TotalFiles:     len(addedFiles),
			TotalSizeBytes: totalSize,
			BlobsReused:    0,
			BlobsNew:       len(addedFiles),
		},
	}

	// Save checkpoint JSON
	cpData, err := json.MarshalIndent(cp, "", "  ")
	if err != nil {
		return nil, nil, err
	}
	cpFile := filepath.Join(ws.CheckpointsDir(), cpID+".json")
	if err := os.WriteFile(cpFile, cpData, 0644); err != nil {
		return nil, nil, fmt.Errorf("writing initial checkpoint: %w", err)
	}

	// Point default branch ref to this checkpoint
	if err := SetRef(ws, DefaultBranch, cpID); err != nil {
		return nil, nil, fmt.Errorf("setting %s ref: %w", DefaultBranch, err)
	}

	return ws, cp, nil
}
