package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/echo-vcs/echo/internal/core"
	"github.com/echo-vcs/echo/internal/index"
	"github.com/echo-vcs/echo/internal/watcher"
)

func TestAutoCheckpointIfDirty(t *testing.T) {
	tempDir := t.TempDir()

	file1 := filepath.Join(tempDir, "file1.txt")
	if err := os.WriteFile(file1, []byte("hello initial"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	ws, _, err := core.Init(tempDir, "")
	if err != nil {
		t.Fatalf("core.Init failed: %v", err)
	}

	// 1. Clean workspace -> AutoCheckpointIfDirty returns nil
	cp, err := core.AutoCheckpointIfDirty(ws, "should not save", "test-agent")
	if err != nil {
		t.Fatalf("AutoCheckpointIfDirty error: %v", err)
	}
	if cp != nil {
		t.Fatalf("expected nil checkpoint for clean workspace, got %v", cp)
	}

	// 2. Modify file1.txt and add file2.txt
	if err := os.WriteFile(file1, []byte("hello modified"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	file2 := filepath.Join(tempDir, "file2.txt")
	if err := os.WriteFile(file2, []byte("second file"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	cp, err = core.AutoCheckpointIfDirty(ws, "safety test", "safety-guard")
	if err != nil {
		t.Fatalf("AutoCheckpointIfDirty error: %v", err)
	}
	if cp == nil {
		t.Fatalf("expected checkpoint created for dirty workspace, got nil")
	}

	if cp.Metadata.Agent != "safety-guard" {
		t.Errorf("expected agent safety-guard, got %s", cp.Metadata.Agent)
	}
	if len(cp.Changeset.Added) != 1 || cp.Changeset.Added[0] != "file2.txt" {
		t.Errorf("expected file2.txt added, got %v", cp.Changeset.Added)
	}
	if len(cp.Changeset.Modified) != 1 || cp.Changeset.Modified[0] != "file1.txt" {
		t.Errorf("expected file1.txt modified, got %v", cp.Changeset.Modified)
	}

	// Test SQLite indexing of auto-checkpoint
	if err := index.IndexWorkspaceCheckpoint(ws, cp); err != nil {
		t.Fatalf("IndexWorkspaceCheckpoint failed: %v", err)
	}

	dbPath := filepath.Join(ws.EchoDir, "index.db")
	idx, err := index.OpenIndex(dbPath)
	if err != nil {
		t.Fatalf("OpenIndex failed: %v", err)
	}
	defer idx.Close()

	records, err := idx.QueryCheckpoints(index.CheckpointFilters{Agent: "safety-guard"})
	if err != nil {
		t.Fatalf("QueryCheckpoints failed: %v", err)
	}
	if len(records) == 0 {
		t.Fatalf("expected indexed checkpoint record, got 0")
	}
	if records[0].ID != cp.ID {
		t.Errorf("expected checkpoint record %s, got %s", cp.ID, records[0].ID)
	}
}

func TestWatcherDebouncedAutoCheckpoint(t *testing.T) {
	tempDir := t.TempDir()

	file1 := filepath.Join(tempDir, "initial.txt")
	if err := os.WriteFile(file1, []byte("hello"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	ws, _, err := core.Init(tempDir, "")
	if err != nil {
		t.Fatalf("core.Init failed: %v", err)
	}

	cpChan := make(chan *core.Checkpoint, 10)
	w, err := watcher.NewWatcher(ws, 150*time.Millisecond, "watcher-agent", func(cp *core.Checkpoint) {
		cpChan <- cp
	})
	if err != nil {
		t.Fatalf("NewWatcher failed: %v", err)
	}
	defer w.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = w.Start(ctx)
	}()

	// Allow watcher to establish inotify/kqueue hooks
	time.Sleep(50 * time.Millisecond)

	// Burst-write multiple files within debounce window (150ms)
	fA := filepath.Join(tempDir, "fileA.txt")
	fB := filepath.Join(tempDir, "fileB.txt")
	if err := os.WriteFile(fA, []byte("content A"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	time.Sleep(20 * time.Millisecond)
	if err := os.WriteFile(fB, []byte("content B"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Wait for auto-checkpoint after debounce
	select {
	case cp := <-cpChan:
		if cp == nil {
			t.Fatalf("received nil checkpoint")
		}
		if cp.Metadata.Agent != "watcher-agent" {
			t.Errorf("expected watcher-agent, got %s", cp.Metadata.Agent)
		}
		summary := cp.Changeset.Summary()
		if summary["added"] != 2 {
			t.Errorf("expected 2 added files grouped in debounced checkpoint, got added: %d, summary: %+v", summary["added"], summary)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for auto-checkpoint from watcher")
	}
}
