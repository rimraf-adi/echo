package merkle

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/echo-vcs/echo/internal/store"
)

// ScannedFile holds metadata and blob hash for a file on disk
type ScannedFile struct {
	Path string
	Size int64
	Mode os.FileMode
	Hash string
}

type dirNode struct {
	files   map[string]ScannedFile
	subdirs map[string]*dirNode
}

func newDirNode() *dirNode {
	return &dirNode{
		files:   make(map[string]ScannedFile),
		subdirs: make(map[string]*dirNode),
	}
}

// BuildTreeFromFiles builds a Merkle tree from a map of relative file paths to ScannedFile,
// stores all intermediate and root tree objects in the ObjectStore, and returns the root TreeNode.
func BuildTreeFromFiles(files map[string]ScannedFile, objStore *store.ObjectStore) (*TreeNode, error) {
	rootDir := newDirNode()

	for rawPath, file := range files {
		cleanPath := filepath.ToSlash(filepath.Clean(rawPath))
		if cleanPath == "." || cleanPath == "" {
			continue
		}

		parts := strings.Split(cleanPath, "/")
		curr := rootDir
		for i := 0; i < len(parts)-1; i++ {
			part := parts[i]
			if _, exists := curr.subdirs[part]; !exists {
				curr.subdirs[part] = newDirNode()
			}
			curr = curr.subdirs[part]
		}
		fileName := parts[len(parts)-1]
		curr.files[fileName] = file
	}

	return buildSubtree(rootDir, objStore)
}

func buildSubtree(dir *dirNode, objStore *store.ObjectStore) (*TreeNode, error) {
	node := NewTreeNode()

	// Add file entries
	for name, file := range dir.files {
		mode := ModeFileRegular
		if file.Mode&os.ModeSymlink != 0 {
			mode = ModeSymlink
		} else if file.Mode&0111 != 0 {
			mode = ModeFileExecutable
		}

		node.AddEntry(TreeEntry{
			Mode: mode,
			Name: name,
			Hash: file.Hash,
			Type: EntryTypeBlob,
			Size: file.Size,
		})
	}

	// Recursively process subdirectories
	for name, subdir := range dir.subdirs {
		childNode, err := buildSubtree(subdir, objStore)
		if err != nil {
			return nil, err
		}

		node.AddEntry(TreeEntry{
			Mode: ModeDirectory,
			Name: name,
			Hash: childNode.Hash,
			Type: EntryTypeTree,
		})
	}

	// Serialize and store tree object
	serialized := SerializeTree(node)
	treeHash, err := objStore.WriteTree(serialized)
	if err != nil {
		return nil, fmt.Errorf("storing tree object: %w", err)
	}
	node.Hash = treeHash

	return node, nil
}
