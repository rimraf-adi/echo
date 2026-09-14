package integration

import (
	"os"
	"testing"

	"github.com/echo-vcs/echo/internal/merkle"
	"github.com/echo-vcs/echo/internal/store"
)

func TestMerkleTreeDeterministicAndHash(t *testing.T) {
	tempDir := t.TempDir()
	s := store.NewObjectStore(tempDir)

	blobHash1, _ := s.WriteBlob([]byte("content of main.go"))
	blobHash2, _ := s.WriteBlob([]byte("package auth\nfunc Login() {}"))
	blobHash3, _ := s.WriteBlob([]byte("# Readme"))

	files := map[string]merkle.ScannedFile{
		"cmd/server/main.go": {
			Path: "cmd/server/main.go",
			Size: 18,
			Mode: 0644,
			Hash: blobHash1,
		},
		"internal/auth/auth.go": {
			Path: "internal/auth/auth.go",
			Size: 28,
			Mode: 0644,
			Hash: blobHash2,
		},
		"README.md": {
			Path: "README.md",
			Size: 8,
			Mode: 0644,
			Hash: blobHash3,
		},
	}

	tree1, err := merkle.BuildTreeFromFiles(files, s)
	if err != nil {
		t.Fatalf("BuildTreeFromFiles failed: %v", err)
	}

	tree2, err := merkle.BuildTreeFromFiles(files, s)
	if err != nil {
		t.Fatalf("BuildTreeFromFiles second time failed: %v", err)
	}

	if tree1.Hash != tree2.Hash {
		t.Fatalf("expected deterministic root hash: %s vs %s", tree1.Hash, tree2.Hash)
	}

	// Change only README.md
	newBlobHash, _ := s.WriteBlob([]byte("# Readme Modified"))
	filesChanged := map[string]merkle.ScannedFile{
		"cmd/server/main.go":    files["cmd/server/main.go"],
		"internal/auth/auth.go": files["internal/auth/auth.go"],
		"README.md": {
			Path: "README.md",
			Size: 17,
			Mode: 0644,
			Hash: newBlobHash,
		},
	}

	tree3, err := merkle.BuildTreeFromFiles(filesChanged, s)
	if err != nil {
		t.Fatalf("BuildTreeFromFiles on modified files failed: %v", err)
	}

	if tree3.Hash == tree1.Hash {
		t.Fatalf("expected root hash to change after modifying a file")
	}

	// But cmd and internal subtrees must have the exact same hash!
	cmd1 := tree1.FindEntry("cmd")
	cmd3 := tree3.FindEntry("cmd")
	if cmd1 == nil || cmd3 == nil || cmd1.Hash != cmd3.Hash {
		t.Fatalf("cmd subtree hash should have remained identical")
	}

	internal1 := tree1.FindEntry("internal")
	internal3 := tree3.FindEntry("internal")
	if internal1 == nil || internal3 == nil || internal1.Hash != internal3.Hash {
		t.Fatalf("internal subtree hash should have remained identical")
	}
}

func TestMerkleDiffOperations(t *testing.T) {
	tempDir := t.TempDir()
	s := store.NewObjectStore(tempDir)

	h1, _ := s.WriteBlob([]byte("file 1"))
	h2, _ := s.WriteBlob([]byte("file 2"))
	h3, _ := s.WriteBlob([]byte("file 3"))

	filesA := map[string]merkle.ScannedFile{
		"pkg/a.go": {Path: "pkg/a.go", Size: 6, Mode: 0644, Hash: h1},
		"pkg/b.go": {Path: "pkg/b.go", Size: 6, Mode: 0644, Hash: h2},
	}

	treeA, err := merkle.BuildTreeFromFiles(filesA, s)
	if err != nil {
		t.Fatalf("BuildTreeFromFiles A failed: %v", err)
	}

	// Diff identical trees
	deltasIdentical, err := merkle.DiffTrees(treeA, treeA, s)
	if err != nil {
		t.Fatalf("DiffTrees identical failed: %v", err)
	}
	if len(deltasIdentical) != 0 {
		t.Fatalf("expected 0 deltas for identical trees, got %d", len(deltasIdentical))
	}

	// In Tree B:
	// a.go modified
	// b.go deleted
	// c.go added in new dir 'sub'
	h1Mod, _ := s.WriteBlob([]byte("file 1 modified"))
	filesB := map[string]merkle.ScannedFile{
		"pkg/a.go":     {Path: "pkg/a.go", Size: 15, Mode: 0644, Hash: h1Mod},
		"sub/dir/c.go": {Path: "sub/dir/c.go", Size: 6, Mode: 0644, Hash: h3},
	}

	treeB, err := merkle.BuildTreeFromFiles(filesB, s)
	if err != nil {
		t.Fatalf("BuildTreeFromFiles B failed: %v", err)
	}

	deltas, err := merkle.DiffTrees(treeA, treeB, s)
	if err != nil {
		t.Fatalf("DiffTrees failed: %v", err)
	}

	deltaMap := make(map[string]merkle.Delta)
	for _, d := range deltas {
		deltaMap[d.Path] = d
	}

	if len(deltaMap) != 3 {
		t.Fatalf("expected 3 deltas, got %d: %+v", len(deltaMap), deltas)
	}

	da, ok := deltaMap["pkg/a.go"]
	if !ok || da.Type != merkle.DeltaModified || da.OldHash != h1 || da.NewHash != h1Mod {
		t.Fatalf("unexpected delta for pkg/a.go: %+v", da)
	}

	db, ok := deltaMap["pkg/b.go"]
	if !ok || db.Type != merkle.DeltaDeleted || db.OldHash != h2 {
		t.Fatalf("unexpected delta for pkg/b.go: %+v", db)
	}

	dc, ok := deltaMap["sub/dir/c.go"]
	if !ok || dc.Type != merkle.DeltaAdded || dc.NewHash != h3 {
		t.Fatalf("unexpected delta for sub/dir/c.go: %+v", dc)
	}
}

func TestMerkleSerializeRoundTrip(t *testing.T) {
	node := merkle.NewTreeNode()
	node.AddEntry(merkle.TreeEntry{
		Mode: merkle.ModeFileRegular,
		Name: "main.go",
		Hash: "1111111111111111111111111111111111111111111111111111111111111111",
		Type: merkle.EntryTypeBlob,
	})
	node.AddEntry(merkle.TreeEntry{
		Mode: merkle.ModeDirectory,
		Name: "cmd",
		Hash: "2222222222222222222222222222222222222222222222222222222222222222",
		Type: merkle.EntryTypeTree,
	})

	data := merkle.SerializeTree(node)
	deserialized, err := merkle.DeserializeTree(data)
	if err != nil {
		t.Fatalf("DeserializeTree failed: %v", err)
	}

	if deserialized.Hash != node.Hash {
		t.Fatalf("hash mismatch: %s vs %s", deserialized.Hash, node.Hash)
	}
	if len(deserialized.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(deserialized.Entries))
	}
	if deserialized.Entries[0].Name != "cmd" || deserialized.Entries[1].Name != "main.go" {
		t.Fatalf("entries not correctly sorted: %+v", deserialized.Entries)
	}
}

func TestMerkleFlattenTree(t *testing.T) {
	tempDir := t.TempDir()
	s := store.NewObjectStore(tempDir)

	h1, _ := s.WriteBlob([]byte("1"))
	h2, _ := s.WriteBlob([]byte("2"))

	files := map[string]merkle.ScannedFile{
		"a/b/c.go": {Path: "a/b/c.go", Size: 1, Mode: os.ModePerm, Hash: h1},
		"x.txt":    {Path: "x.txt", Size: 1, Mode: 0644, Hash: h2},
	}

	tree, err := merkle.BuildTreeFromFiles(files, s)
	if err != nil {
		t.Fatalf("BuildTreeFromFiles failed: %v", err)
	}

	flattened, err := merkle.FlattenTree(tree, s)
	if err != nil {
		t.Fatalf("FlattenTree failed: %v", err)
	}

	if len(flattened) != 2 {
		t.Fatalf("expected 2 flattened files, got %d", len(flattened))
	}
	if _, ok := flattened["a/b/c.go"]; !ok {
		t.Fatalf("missing a/b/c.go in flattened tree")
	}
	if _, ok := flattened["x.txt"]; !ok {
		t.Fatalf("missing x.txt in flattened tree")
	}
}
