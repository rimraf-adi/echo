package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/echo-vcs/echo/internal/ignore"
	"github.com/echo-vcs/echo/internal/scan"
	"github.com/echo-vcs/echo/internal/store"
)

func TestScannerAndParallelHashing(t *testing.T) {
	tempDir := t.TempDir()

	// Setup directory structure
	// tempDir/
	//   main.go
	//   cmd/
	//     server/
	//       main.go
	//   test.log (ignored)
	//   .echo/ (ignored)
	//     config.json
	//   node_modules/ (ignored)
	//     pkg/index.js

	_ = os.WriteFile(filepath.Join(tempDir, "main.go"), []byte("package main"), 0644)
	_ = os.MkdirAll(filepath.Join(tempDir, "cmd", "server"), 0755)
	_ = os.WriteFile(filepath.Join(tempDir, "cmd", "server", "main.go"), []byte("package server"), 0644)
	_ = os.WriteFile(filepath.Join(tempDir, "test.log"), []byte("log output"), 0644)

	_ = os.MkdirAll(filepath.Join(tempDir, ".echo"), 0755)
	_ = os.WriteFile(filepath.Join(tempDir, ".echo", "config.json"), []byte("{}"), 0644)

	_ = os.MkdirAll(filepath.Join(tempDir, "node_modules", "pkg"), 0755)
	_ = os.WriteFile(filepath.Join(tempDir, "node_modules", "pkg", "index.js"), []byte("module.exports = {}"), 0644)

	matcher := ignore.NewMatcher([]string{"*.log", "node_modules/"})
	scanner := scan.NewScanner(matcher)

	res, err := scanner.Scan(tempDir)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if len(res.Files) != 2 {
		t.Fatalf("expected 2 files tracked, got %d: %+v", len(res.Files), res.Files)
	}

	if _, ok := res.Files["main.go"]; !ok {
		t.Fatalf("main.go not found in scan")
	}
	if _, ok := res.Files["cmd/server/main.go"]; !ok {
		t.Fatalf("cmd/server/main.go not found in scan")
	}

	// Test parallel hashing with 4 workers
	err = scan.HashFiles(tempDir, res.Files, 4)
	if err != nil {
		t.Fatalf("HashFiles failed: %v", err)
	}

	expectedMainHash := store.HashBytes([]byte("package main"))
	if res.Files["main.go"].Hash != expectedMainHash {
		t.Fatalf("hash mismatch for main.go: expected %s, got %s", expectedMainHash, res.Files["main.go"].Hash)
	}

	expectedServerHash := store.HashBytes([]byte("package server"))
	if res.Files["cmd/server/main.go"].Hash != expectedServerHash {
		t.Fatalf("hash mismatch for cmd/server/main.go: expected %s, got %s", expectedServerHash, res.Files["cmd/server/main.go"].Hash)
	}
}

func TestScanEmptyDirectory(t *testing.T) {
	tempDir := t.TempDir()
	scanner := scan.NewScanner(nil)

	res, err := scanner.Scan(tempDir)
	if err != nil {
		t.Fatalf("Scan on empty directory failed: %v", err)
	}
	if len(res.Files) != 0 {
		t.Fatalf("expected 0 files, got %d", len(res.Files))
	}
}
