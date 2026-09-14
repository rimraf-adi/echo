package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/echo-vcs/echo/internal/core"
)

func TestThreeWayMergeWithStrategies(t *testing.T) {
	tempDir := t.TempDir()

	file1 := filepath.Join(tempDir, "shared.txt")
	_ = os.WriteFile(file1, []byte("shared base"), 0644)

	ws, cpBase, err := core.Init(tempDir, "")
	if err != nil {
		t.Fatalf("core.Init failed: %v", err)
	}

	// Create feature branch
	if err := core.CreateBranch(ws, "feature", "HEAD"); err != nil {
		t.Fatalf("CreateBranch failed: %v", err)
	}

	// Make changes on main branch
	_ = os.WriteFile(file1, []byte("shared modified by main"), 0644)
	mainOnly := filepath.Join(tempDir, "main_only.txt")
	_ = os.WriteFile(mainOnly, []byte("main file"), 0644)

	cpMain, err := core.CreateCheckpoint(ws, core.CheckpointOpts{Agent: "main-agent"})
	if err != nil {
		t.Fatalf("CreateCheckpoint on main failed: %v", err)
	}

	// Switch to feature branch
	if err := core.SwitchBranch(ws, "feature"); err != nil {
		t.Fatalf("SwitchBranch to feature failed: %v", err)
	}

	// Make conflicting change on shared.txt and add feature_only.txt
	_ = os.WriteFile(file1, []byte("shared modified by feature"), 0644)
	featOnly := filepath.Join(tempDir, "feat_only.txt")
	_ = os.WriteFile(featOnly, []byte("feature file"), 0644)

	cpFeat, err := core.CreateCheckpoint(ws, core.CheckpointOpts{Agent: "feat-agent"})
	if err != nil {
		t.Fatalf("CreateCheckpoint on feature failed: %v", err)
	}

	// Switch back to main
	if err := core.SwitchBranch(ws, "main"); err != nil {
		t.Fatalf("SwitchBranch to main failed: %v", err)
	}

	// Verify LCA is cpBase
	lca, err := core.FindLCA(ws, cpMain.ID, cpFeat.ID)
	if err != nil || lca != cpBase.ID {
		t.Fatalf("expected LCA %s, got %s (err: %v)", cpBase.ID, lca, err)
	}

	// Merge feature into main with StrategyTheirs (default)
	mergeRes, err := core.Merge(ws, "feature", core.StrategyTheirs, false)
	if err != nil {
		t.Fatalf("Merge failed: %v", err)
	}

	if len(mergeRes.Parents) != 2 || mergeRes.Parents[0] != cpMain.ID || mergeRes.Parents[1] != cpFeat.ID {
		t.Fatalf("merge parents mismatch: %+v", mergeRes.Parents)
	}

	if len(mergeRes.Conflicts) != 1 || mergeRes.Conflicts[0].Path != "shared.txt" {
		t.Fatalf("expected 1 conflict on shared.txt, got %+v", mergeRes.Conflicts)
	}

	// Because StrategyTheirs was used, shared.txt should have feature's content
	sharedData, _ := os.ReadFile(file1)
	if string(sharedData) != "shared modified by feature" {
		t.Fatalf("expected feature content in shared.txt, got %s", string(sharedData))
	}

	// Both main_only.txt and feat_only.txt should exist!
	if _, err := os.Stat(mainOnly); err != nil {
		t.Fatalf("main_only.txt missing after merge: %v", err)
	}
	if _, err := os.Stat(featOnly); err != nil {
		t.Fatalf("feat_only.txt missing after merge: %v", err)
	}
}
