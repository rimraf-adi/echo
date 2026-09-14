package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/echo-vcs/echo/internal/core"
)

func TestDiffCheckpointsAndWorking(t *testing.T) {
	tempDir := t.TempDir()

	file1 := filepath.Join(tempDir, "file1.txt")
	_ = os.WriteFile(file1, []byte("line 1\nline 2\nline 3\n"), 0644)

	ws, cp0, err := core.Init(tempDir, "")
	if err != nil {
		t.Fatalf("core.Init failed: %v", err)
	}

	// Modify file1.txt and add file2.txt
	_ = os.WriteFile(file1, []byte("line 1\nline 2 modified\nline 3\nline 4\n"), 0644)
	file2 := filepath.Join(tempDir, "file2.txt")
	_ = os.WriteFile(file2, []byte("new file\n"), 0644)

	cp1, err := core.CreateCheckpoint(ws, core.CheckpointOpts{Agent: "agent-1"})
	if err != nil {
		t.Fatalf("CreateCheckpoint failed: %v", err)
	}

	// Diff cp0 vs cp1
	diffRes, err := core.DiffCheckpoints(ws, cp0.ID, cp1.ID)
	if err != nil {
		t.Fatalf("DiffCheckpoints failed: %v", err)
	}

	if len(diffRes.Files) != 2 {
		t.Fatalf("expected 2 changed files in diff, got %d", len(diffRes.Files))
	}

	diffMap := make(map[string]core.FileDiff)
	for _, f := range diffRes.Files {
		diffMap[f.Path] = f
	}

	f1Diff, ok := diffMap["file1.txt"]
	if !ok || f1Diff.Type != "modified" {
		t.Fatalf("unexpected diff for file1.txt: %+v", f1Diff)
	}
	if f1Diff.LinesAdded != 2 || f1Diff.LinesRemoved != 1 {
		t.Fatalf("expected 2 lines added, 1 line removed, got +%d -%d", f1Diff.LinesAdded, f1Diff.LinesRemoved)
	}

	f2Diff, ok := diffMap["file2.txt"]
	if !ok || f2Diff.Type != "added" {
		t.Fatalf("unexpected diff for file2.txt: %+v", f2Diff)
	}

	// Modify working directory without checkpointing
	_ = os.WriteFile(file1, []byte("line 1\nline 2 modified\nline 3\nline 4\nline 5\n"), 0644)
	workingDiff, err := core.DiffWorking(ws, "HEAD")
	if err != nil {
		t.Fatalf("DiffWorking failed: %v", err)
	}

	if len(workingDiff.Files) != 1 || workingDiff.Files[0].Path != "file1.txt" {
		t.Fatalf("expected 1 file in working diff, got %+v", workingDiff.Files)
	}
	if workingDiff.Files[0].LinesAdded != 1 {
		t.Fatalf("expected 1 line added in working diff, got %d", workingDiff.Files[0].LinesAdded)
	}
}
