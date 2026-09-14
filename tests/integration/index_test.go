package integration

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/echo-vcs/echo/internal/core"
	"github.com/echo-vcs/echo/internal/index"
	"github.com/echo-vcs/echo/internal/merkle"
)

func TestIndexFilesAndSearch(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "index.db")

	idx, err := index.OpenIndex(dbPath)
	if err != nil {
		t.Fatalf("OpenIndex failed: %v", err)
	}
	defer idx.Close()

	cpID := "cp-test-01"
	files := map[string]merkle.TreeEntry{
		"cmd/main.go": {
			Mode: merkle.ModeFileRegular,
			Name: "main.go",
			Hash: "1111111111111111111111111111111111111111111111111111111111111111",
			Type: merkle.EntryTypeBlob,
			Size: 100,
		},
		"pkg/auth/jwt.go": {
			Mode: merkle.ModeFileRegular,
			Name: "jwt.go",
			Hash: "2222222222222222222222222222222222222222222222222222222222222222",
			Type: merkle.EntryTypeBlob,
			Size: 200,
		},
	}

	if err := idx.IndexFiles(cpID, files); err != nil {
		t.Fatalf("IndexFiles failed: %v", err)
	}

	// Query all
	records, err := idx.QueryFiles(cpID, "")
	if err != nil {
		t.Fatalf("QueryFiles failed: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 files, got %d", len(records))
	}

	// Query with glob
	filtered, err := idx.QueryFiles(cpID, "*.go")
	if err != nil {
		t.Fatalf("QueryFiles with glob failed: %v", err)
	}
	if len(filtered) != 2 {
		t.Fatalf("expected 2 .go files, got %d", len(filtered))
	}

	// Get specific file
	rec, err := idx.GetFile(cpID, "cmd/main.go")
	if err != nil {
		t.Fatalf("GetFile failed: %v", err)
	}
	if rec == nil || rec.BlobHash != "1111111111111111111111111111111111111111111111111111111111111111" {
		t.Fatalf("GetFile returned unexpected record: %+v", rec)
	}

	// Test FTS search
	mainContent := `package main

import "fmt"

func main() {
    fmt.Println("Starting server...")
}
`
	if err := idx.IndexContent(cpID, "cmd/main.go", mainContent); err != nil {
		t.Fatalf("IndexContent failed: %v", err)
	}

	results, err := idx.Search(cpID, "Starting server", 10, 2)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 search result, got %d", len(results))
	}
	if results[0].LineNumber != 6 {
		t.Fatalf("expected match at line 6, got %d", results[0].LineNumber)
	}
}

func TestIndexCheckpointsAndRebuild(t *testing.T) {
	tempDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tempDir, "hello.txt"), []byte("Hello world!"), 0644)
	ws, cp0, err := core.Init(tempDir, "")
	if err != nil {
		t.Fatalf("core.Init failed: %v", err)
	}

	dbPath := filepath.Join(ws.EchoDir, "index.db")
	idx, err := index.OpenIndex(dbPath)
	if err != nil {
		t.Fatalf("OpenIndex failed: %v", err)
	}
	defer idx.Close()

	// Rebuild index
	if err := idx.Rebuild(ws); err != nil {
		t.Fatalf("Rebuild failed: %v", err)
	}

	// Query checkpoints
	cps, err := idx.QueryCheckpoints(index.CheckpointFilters{})
	if err != nil {
		t.Fatalf("QueryCheckpoints failed: %v", err)
	}
	if len(cps) != 1 || cps[0].ID != cp0.ID {
		t.Fatalf("expected 1 indexed checkpoint (%s), got %+v", cp0.ID, cps)
	}

	// Query files from index
	files, err := idx.QueryFiles(cp0.ID, "")
	if err != nil {
		t.Fatalf("QueryFiles failed: %v", err)
	}
	if len(files) != 1 || files[0].Path != "hello.txt" {
		t.Fatalf("expected hello.txt, got %+v", files)
	}

	// Search text
	results, err := idx.Search(cp0.ID, "world", 10, 1)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 search result from rebuilt index, got %d", len(results))
	}

	// Checkpoint filtering by date
	since := time.Now().Add(-1 * time.Hour)
	cpsSince, err := idx.QueryCheckpoints(index.CheckpointFilters{Since: since})
	if err != nil || len(cpsSince) != 1 {
		t.Fatalf("date filter failed: %v, count=%d", err, len(cpsSince))
	}
}
