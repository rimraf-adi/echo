package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/echo-vcs/echo/internal/core"
)

func TestBranchLifecycle(t *testing.T) {
	tempDir := t.TempDir()

	file1 := filepath.Join(tempDir, "main.txt")
	_ = os.WriteFile(file1, []byte("main branch v1"), 0644)

	ws, cp0, err := core.Init(tempDir, "")
	if err != nil {
		t.Fatalf("core.Init failed: %v", err)
	}

	// Create feature branch
	if err := core.CreateBranch(ws, "feature-x", "HEAD"); err != nil {
		t.Fatalf("CreateBranch failed: %v", err)
	}

	// Switch to feature branch
	if err := core.SwitchBranch(ws, "feature-x"); err != nil {
		t.Fatalf("SwitchBranch failed: %v", err)
	}

	currentBranch, err := core.ReadHead(ws)
	if err != nil || currentBranch != "feature-x" {
		t.Fatalf("expected current branch feature-x, got %s", currentBranch)
	}

	// Make changes on feature branch
	featFile := filepath.Join(tempDir, "feature.txt")
	_ = os.WriteFile(featFile, []byte("feature content"), 0644)

	cpFeat, err := core.CreateCheckpoint(ws, core.CheckpointOpts{Agent: "agent-feat"})
	if err != nil {
		t.Fatalf("CreateCheckpoint on branch failed: %v", err)
	}

	// Switch back to main
	if err := core.SwitchBranch(ws, "main"); err != nil {
		t.Fatalf("SwitchBranch back to main failed: %v", err)
	}

	// Verify feature.txt is NOT in main working directory!
	if _, err := os.Stat(featFile); !os.IsNotExist(err) {
		t.Fatalf("feature.txt should not exist on main branch")
	}

	// Verify main HEAD is still cp0
	mainHead, _ := core.GetRef(ws, "main")
	if mainHead != cp0.ID {
		t.Fatalf("main HEAD was modified unexpectedly: %s vs %s", mainHead, cp0.ID)
	}

	// Switch back to feature-x
	if err := core.SwitchBranch(ws, "feature-x"); err != nil {
		t.Fatalf("SwitchBranch to feature-x failed: %v", err)
	}

	// Verify feature.txt is restored!
	featData, err := os.ReadFile(featFile)
	if err != nil || string(featData) != "feature content" {
		t.Fatalf("feature.txt not restored correctly on switch back: %v", err)
	}

	// Verify ListBranches
	branches, err := core.ListBranches(ws)
	if err != nil {
		t.Fatalf("ListBranches failed: %v", err)
	}
	if len(branches) != 2 {
		t.Fatalf("expected 2 branches, got %d", len(branches))
	}

	// Cannot delete current branch
	err = core.DeleteBranch(ws, "feature-x")
	if err == nil {
		t.Fatalf("expected error deleting current branch")
	}

	// Switch to main and delete feature-x
	_ = core.SwitchBranch(ws, "main")
	if err := core.DeleteBranch(ws, "feature-x"); err != nil {
		t.Fatalf("DeleteBranch failed: %v", err)
	}

	// Checkpoints from feature branch still exist!
	loadedCp, err := core.LoadCheckpoint(ws, cpFeat.ID)
	if err != nil || loadedCp == nil {
		t.Fatalf("checkpoint from deleted branch should remain intact: %v", err)
	}
}
