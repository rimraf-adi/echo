package merkle

import (
	"fmt"
	"path"

	"github.com/echo-vcs/echo/internal/store"
)

type DeltaType int

const (
	DeltaAdded DeltaType = iota
	DeltaModified
	DeltaDeleted
)

func (d DeltaType) String() string {
	switch d {
	case DeltaAdded:
		return "added"
	case DeltaModified:
		return "modified"
	case DeltaDeleted:
		return "deleted"
	default:
		return "unknown"
	}
}

// Delta represents a file-level change between two trees
type Delta struct {
	Path    string    `json:"path"`
	Type    DeltaType `json:"type"`
	OldHash string    `json:"old_hash,omitempty"`
	NewHash string    `json:"new_hash,omitempty"`
	OldMode string    `json:"old_mode,omitempty"`
	NewMode string    `json:"new_mode,omitempty"`
}

// DiffTrees compares two Merkle trees and returns file-level deltas.
// It leverages Merkle tree root and subtree hashes to prune unchanged branches in O(changes).
func DiffTrees(a, b *TreeNode, objStore *store.ObjectStore) ([]Delta, error) {
	if a == nil && b == nil {
		return nil, nil
	}

	if a != nil && b != nil && a.Hash == b.Hash {
		return nil, nil
	}

	var deltas []Delta
	if err := diffSubtree("", a, b, objStore, &deltas); err != nil {
		return nil, err
	}
	return deltas, nil
}

func loadTree(hash string, objStore *store.ObjectStore) (*TreeNode, error) {
	data, err := objStore.ReadTree(hash)
	if err != nil {
		return nil, fmt.Errorf("reading tree %s: %w", hash, err)
	}
	return DeserializeTree(data)
}

func diffSubtree(currentPath string, a, b *TreeNode, objStore *store.ObjectStore, deltas *[]Delta) error {
	// If both non-nil and identical hash, entire subtree matches - prune immediately
	if a != nil && b != nil && a.Hash != "" && a.Hash == b.Hash {
		return nil
	}

	entriesA := make(map[string]TreeEntry)
	if a != nil {
		for _, e := range a.Entries {
			entriesA[e.Name] = e
		}
	}

	entriesB := make(map[string]TreeEntry)
	if b != nil {
		for _, e := range b.Entries {
			entriesB[e.Name] = e
		}
	}

	// Union of entry names
	names := make(map[string]struct{})
	for name := range entriesA {
		names[name] = struct{}{}
	}
	for name := range entriesB {
		names[name] = struct{}{}
	}

	for name := range names {
		entryPath := path.Join(currentPath, name)
		ea, inA := entriesA[name]
		eb, inB := entriesB[name]

		if !inA {
			// Added in B
			if eb.Type == EntryTypeTree {
				childTree, err := loadTree(eb.Hash, objStore)
				if err != nil {
					return err
				}
				if err := emitAll(entryPath, childTree, DeltaAdded, objStore, deltas); err != nil {
					return err
				}
			} else {
				*deltas = append(*deltas, Delta{
					Path:    entryPath,
					Type:    DeltaAdded,
					NewHash: eb.Hash,
					NewMode: eb.Mode,
				})
			}
		} else if !inB {
			// Deleted from A
			if ea.Type == EntryTypeTree {
				childTree, err := loadTree(ea.Hash, objStore)
				if err != nil {
					return err
				}
				if err := emitAll(entryPath, childTree, DeltaDeleted, objStore, deltas); err != nil {
					return err
				}
			} else {
				*deltas = append(*deltas, Delta{
					Path:    entryPath,
					Type:    DeltaDeleted,
					OldHash: ea.Hash,
					OldMode: ea.Mode,
				})
			}
		} else {
			// Present in both
			if ea.Hash == eb.Hash && ea.Mode == eb.Mode {
				continue
			}

			if ea.Type == EntryTypeTree && eb.Type == EntryTypeTree {
				// Both directories, descend
				treeA, err := loadTree(ea.Hash, objStore)
				if err != nil {
					return err
				}
				treeB, err := loadTree(eb.Hash, objStore)
				if err != nil {
					return err
				}
				if err := diffSubtree(entryPath, treeA, treeB, objStore, deltas); err != nil {
					return err
				}
			} else if ea.Type == EntryTypeBlob && eb.Type == EntryTypeBlob {
				// Both files, changed content or mode
				*deltas = append(*deltas, Delta{
					Path:    entryPath,
					Type:    DeltaModified,
					OldHash: ea.Hash,
					NewHash: eb.Hash,
					OldMode: ea.Mode,
					NewMode: eb.Mode,
				})
			} else if ea.Type == EntryTypeBlob && eb.Type == EntryTypeTree {
				// File replaced by directory
				*deltas = append(*deltas, Delta{
					Path:    entryPath,
					Type:    DeltaDeleted,
					OldHash: ea.Hash,
					OldMode: ea.Mode,
				})
				childTree, err := loadTree(eb.Hash, objStore)
				if err != nil {
					return err
				}
				if err := emitAll(entryPath, childTree, DeltaAdded, objStore, deltas); err != nil {
					return err
				}
			} else if ea.Type == EntryTypeTree && eb.Type == EntryTypeBlob {
				// Directory replaced by file
				childTree, err := loadTree(ea.Hash, objStore)
				if err != nil {
					return err
				}
				if err := emitAll(entryPath, childTree, DeltaDeleted, objStore, deltas); err != nil {
					return err
				}
				*deltas = append(*deltas, Delta{
					Path:    entryPath,
					Type:    DeltaAdded,
					NewHash: eb.Hash,
					NewMode: eb.Mode,
				})
			}
		}
	}

	return nil
}

func emitAll(currentPath string, node *TreeNode, deltaType DeltaType, objStore *store.ObjectStore, deltas *[]Delta) error {
	for _, entry := range node.Entries {
		entryPath := path.Join(currentPath, entry.Name)
		if entry.Type == EntryTypeTree {
			childTree, err := loadTree(entry.Hash, objStore)
			if err != nil {
				return err
			}
			if err := emitAll(entryPath, childTree, deltaType, objStore, deltas); err != nil {
				return err
			}
		} else {
			delta := Delta{
				Path: entryPath,
				Type: deltaType,
			}
			if deltaType == DeltaAdded {
				delta.NewHash = entry.Hash
				delta.NewMode = entry.Mode
			} else {
				delta.OldHash = entry.Hash
				delta.OldMode = entry.Mode
			}
			*deltas = append(*deltas, delta)
		}
	}
	return nil
}
