package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/echo-vcs/echo/internal/core"
)

func TestVerifyAndGarbageCollection(t *testing.T) {
	tempDir := t.TempDir()

	file1 := filepath.Join(tempDir, "file1.txt")
	_ = os.WriteFile(file1, []byte("file 1 content"), 0644)

	ws, cp0, err := core.Init(tempDir, "")
	if err != nil {
		t.Fatalf("core.Init failed: %v", err)
	}

	// 1. Verify clean repository
	stdout, _, code := runEchoCLI(t, tempDir, "verify", "--json")
	if code != 0 {
		t.Fatalf("verify failed on clean repo: %s", stdout)
	}

	// 2. Create an orphaned blob directly in store
	orphanHash, err := ws.Store.WriteBlob([]byte("this blob is not referenced anywhere"))
	if err != nil {
		t.Fatalf("WriteBlob failed: %v", err)
	}

	if !ws.Store.HasObject(orphanHash) {
		t.Fatalf("orphan blob should exist in store")
	}

	// 3. Dry-run GC should detect 1 unreferenced object
	stdout, _, code = runEchoCLI(t, tempDir, "gc", "--dry-run", "--json")
	if code != 0 {
		t.Fatalf("echo gc --dry-run failed: %d", code)
	}
	if !ws.Store.HasObject(orphanHash) {
		t.Fatalf("dry run gc should not delete the object")
	}

	// 4. Real GC should remove the orphaned object
	stdout, _, code = runEchoCLI(t, tempDir, "gc", "--json")
	if code != 0 {
		t.Fatalf("echo gc failed: %d", code)
	}

	if ws.Store.HasObject(orphanHash) {
		t.Fatalf("orphan blob should have been removed by gc")
	}

	// Checkpoint 0's tree and blobs should still exist!
	cp0TreeBytes, err := ws.Store.ReadTree(cp0.TreeHash)
	if err != nil || len(cp0TreeBytes) == 0 {
		t.Fatalf("referenced tree was incorrectly removed by gc: %v", err)
	}

	// 5. Corrupt an object on disk and test verify
	path, _ := ws.Store.ObjectPath(cp0.TreeHash)
	data, _ := os.ReadFile(path)
	data[len(data)-1] ^= 0xFF
	_ = os.WriteFile(path, data, 0644)

	// Verify should report 1 corrupted object
	stdout, _, code = runEchoCLI(t, tempDir, "verify", "--json")
	if code != 0 {
		t.Fatalf("verify failed: %d", code)
	}
}
