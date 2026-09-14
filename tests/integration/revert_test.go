package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/echo-vcs/echo/internal/core"
)

func TestRevertRestoreAndDryRun(t *testing.T) {
	tempDir := t.TempDir()

	file1 := filepath.Join(tempDir, "file1.txt")
	_ = os.WriteFile(file1, []byte("original content"), 0644)

	ws, cp0, err := core.Init(tempDir, "")
	if err != nil {
		t.Fatalf("core.Init failed: %v", err)
	}

	// Change file1 and create file2
	_ = os.WriteFile(file1, []byte("modified content"), 0644)
	file2 := filepath.Join(tempDir, "file2.txt")
	_ = os.WriteFile(file2, []byte("temporary file"), 0644)

	_, err = core.CreateCheckpoint(ws, core.CheckpointOpts{Agent: "agent-1"})
	if err != nil {
		t.Fatalf("CreateCheckpoint failed: %v", err)
	}

	// 1. Dry-run revert to cp0
	dryRes, err := core.Revert(ws, cp0.ID, true, false)
	if err != nil {
		t.Fatalf("Revert dry-run failed: %v", err)
	}
	if len(dryRes.ChangesApplied.Deleted) != 1 || dryRes.ChangesApplied.Deleted[0] != "file2.txt" {
		t.Fatalf("expected file2.txt in dry-run deleted, got %+v", dryRes.ChangesApplied)
	}
	if len(dryRes.ChangesApplied.Modified) != 1 || dryRes.ChangesApplied.Modified[0] != "file1.txt" {
		t.Fatalf("expected file1.txt in dry-run modified, got %+v", dryRes.ChangesApplied)
	}

	// Verify working directory was untouched by dry run
	f1Data, _ := os.ReadFile(file1)
	if string(f1Data) != "modified content" {
		t.Fatalf("dry run modified file1!")
	}
	if _, err := os.Stat(file2); err != nil {
		t.Fatalf("dry run deleted file2!")
	}

	// 2. Real revert to cp0 with revert checkpoint creation
	realRes, err := core.Revert(ws, cp0.ID, false, true)
	if err != nil {
		t.Fatalf("Real revert failed: %v", err)
	}
	if realRes.RevertCheckpoint == "" {
		t.Fatalf("expected revert checkpoint ID to be populated")
	}

	// Verify filesystem is back to cp0 state
	f1Data, _ = os.ReadFile(file1)
	if string(f1Data) != "original content" {
		t.Fatalf("expected file1 content to be 'original content', got '%s'", string(f1Data))
	}
	if _, err := os.Stat(file2); !os.IsNotExist(err) {
		t.Fatalf("expected file2 to be deleted by revert")
	}

	// Verify HEAD is now pointing to the revert checkpoint
	headID, err := core.ResolveRef(ws, "HEAD")
	if err != nil || headID != realRes.RevertCheckpoint {
		t.Fatalf("expected HEAD to point to revert checkpoint %s, got %s", realRes.RevertCheckpoint, headID)
	}
}
