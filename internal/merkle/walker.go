package merkle

import (
	"path"

	"github.com/echo-vcs/echo/internal/store"
)

// WalkFunc is the callback type for WalkTree
type WalkFunc func(entryPath string, entry TreeEntry, isDir bool) error

// WalkTree traverses the Merkle tree rooted at node in pre-order
func WalkTree(root *TreeNode, objStore *store.ObjectStore, fn WalkFunc) error {
	if root == nil {
		return nil
	}
	return walkSubtree("", root, objStore, fn)
}

func walkSubtree(currentPath string, node *TreeNode, objStore *store.ObjectStore, fn WalkFunc) error {
	for _, entry := range node.Entries {
		entryPath := path.Join(currentPath, entry.Name)
		isDir := entry.Type == EntryTypeTree

		if err := fn(entryPath, entry, isDir); err != nil {
			return err
		}

		if isDir {
			data, err := objStore.ReadTree(entry.Hash)
			if err != nil {
				return err
			}
			childNode, err := DeserializeTree(data)
			if err != nil {
				return err
			}
			if err := walkSubtree(entryPath, childNode, objStore, fn); err != nil {
				return err
			}
		}
	}
	return nil
}

// FlattenTree extracts all file (blob) entries from the tree as a map of relative path to TreeEntry
func FlattenTree(root *TreeNode, objStore *store.ObjectStore) (map[string]TreeEntry, error) {
	files := make(map[string]TreeEntry)
	err := WalkTree(root, objStore, func(entryPath string, entry TreeEntry, isDir bool) error {
		if !isDir {
			files[entryPath] = entry
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}
