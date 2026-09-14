package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/echo-vcs/echo/internal/core"
)

func TestWorkspaceInitAndOpen(t *testing.T) {
	tempDir := t.TempDir()

	// Add a test file
	_ = os.WriteFile(filepath.Join(tempDir, "README.md"), []byte("# Hello Echo"), 0644)
	_ = os.MkdirAll(filepath.Join(tempDir, "src"), 0755)
	_ = os.WriteFile(filepath.Join(tempDir, "src", "main.go"), []byte("package main"), 0644)

	ws, cp, err := core.Init(tempDir, "")
	if err != nil {
		t.Fatalf("core.Init failed: %v", err)
	}

	if ws == nil || cp == nil {
		t.Fatalf("expected non-nil workspace and checkpoint")
	}

	if cp.Stats.TotalFiles != 2 {
		t.Fatalf("expected 2 files in initial checkpoint, got %d", cp.Stats.TotalFiles)
	}

	// Verify opening from root
	openedWs, err := core.Open(tempDir)
	if err != nil {
		t.Fatalf("core.Open failed: %v", err)
	}
	if openedWs.Config.WorkspaceID != ws.Config.WorkspaceID {
		t.Fatalf("workspace ID mismatch: %s vs %s", ws.Config.WorkspaceID, openedWs.Config.WorkspaceID)
	}

	// Verify opening from a subdirectory
	subDir := filepath.Join(tempDir, "src")
	subWs, err := core.Open(subDir)
	if err != nil {
		t.Fatalf("core.Open from subdirectory failed: %v", err)
	}
	if subWs.RootDir != ws.RootDir {
		t.Fatalf("root dir mismatch from subdir open: %s vs %s", ws.RootDir, subWs.RootDir)
	}

	// Initializing again should fail
	_, _, err = core.Init(tempDir, "")
	if err != core.ErrWorkspaceAlreadyExists {
		t.Fatalf("expected ErrWorkspaceAlreadyExists, got %v", err)
	}
}
