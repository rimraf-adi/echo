package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/echo-vcs/echo/internal/core"
)

func TestCheckpointLifecycleAndDeltas(t *testing.T) {
	tempDir := t.TempDir()

	file1 := filepath.Join(tempDir, "file1.txt")
	_ = os.WriteFile(file1, []byte("initial content 1"), 0644)

	ws, cp0, err := core.Init(tempDir, "")
	if err != nil {
		t.Fatalf("core.Init failed: %v", err)
	}

	// 1. Checkpoint with no changes should return nil
	cpNoChange, err := core.CreateCheckpoint(ws, core.CheckpointOpts{Agent: "agent-1"})
	if err != nil {
		t.Fatalf("CreateCheckpoint no change failed: %v", err)
	}
	if cpNoChange != nil {
		t.Fatalf("expected nil checkpoint when no changes made, got %+v", cpNoChange)
	}

	// 2. Add file2.txt and modify file1.txt
	file2 := filepath.Join(tempDir, "file2.txt")
	_ = os.WriteFile(file2, []byte("file 2 content"), 0644)
	_ = os.WriteFile(file1, []byte("modified content 1"), 0644)

	cp1, err := core.CreateCheckpoint(ws, core.CheckpointOpts{
		Agent: "claude-opus-4-20250514",
		Task:  "add file2 and modify file1",
	})
	if err != nil {
		t.Fatalf("CreateCheckpoint 1 failed: %v", err)
	}
	if cp1 == nil {
		t.Fatalf("expected non-nil checkpoint")
	}

	if len(cp1.Parents) != 1 || cp1.Parents[0] != cp0.ID {
		t.Fatalf("parent chain broken: %+v", cp1.Parents)
	}

	if len(cp1.Changeset.Added) != 1 || cp1.Changeset.Added[0] != "file2.txt" {
		t.Fatalf("expected file2.txt added, got %+v", cp1.Changeset.Added)
	}
	if len(cp1.Changeset.Modified) != 1 || cp1.Changeset.Modified[0] != "file1.txt" {
		t.Fatalf("expected file1.txt modified, got %+v", cp1.Changeset.Modified)
	}

	// 3. Delete file2.txt
	_ = os.Remove(file2)
	cp2, err := core.CreateCheckpoint(ws, core.CheckpointOpts{
		Agent: "claude-opus-4-20250514",
		Task:  "delete file2",
	})
	if err != nil {
		t.Fatalf("CreateCheckpoint 2 failed: %v", err)
	}
	if len(cp2.Changeset.Deleted) != 1 || cp2.Changeset.Deleted[0] != "file2.txt" {
		t.Fatalf("expected file2.txt deleted, got %+v", cp2.Changeset.Deleted)
	}

	// 4. Test ref resolution
	headID, err := core.ResolveRef(ws, "HEAD")
	if err != nil || headID != cp2.ID {
		t.Fatalf("failed to resolve HEAD: %v, got %s, expected %s", err, headID, cp2.ID)
	}

	head1ID, err := core.ResolveRef(ws, "HEAD~1")
	if err != nil || head1ID != cp1.ID {
		t.Fatalf("failed to resolve HEAD~1: %v, got %s, expected %s", err, head1ID, cp1.ID)
	}

	head2ID, err := core.ResolveRef(ws, "HEAD~2")
	if err != nil || head2ID != cp0.ID {
		t.Fatalf("failed to resolve HEAD~2: %v, got %s, expected %s", err, head2ID, cp0.ID)
	}

	// Test prefix matching: short prefix is ambiguous
	_, err = core.ResolveRef(ws, cp1.ID[:10])
	if err == nil {
		t.Fatalf("expected error for ambiguous prefix")
	}

	// Unambiguous prefix (everything except last 2 chars) resolves properly
	prefixID, err := core.ResolveRef(ws, cp1.ID[:len(cp1.ID)-2])
	if err != nil || prefixID != cp1.ID {
		t.Fatalf("failed to resolve prefix %s: %v", cp1.ID[:len(cp1.ID)-2], err)
	}

	// Test ListCheckpoints
	allCheckpoints, err := core.ListCheckpoints(ws)
	if err != nil {
		t.Fatalf("ListCheckpoints failed: %v", err)
	}
	if len(allCheckpoints) != 3 {
		t.Fatalf("expected 3 checkpoints, got %d", len(allCheckpoints))
	}
}
