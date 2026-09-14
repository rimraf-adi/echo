package merkle

import (
	"sort"
)

type EntryType int

const (
	EntryTypeBlob EntryType = iota
	EntryTypeTree
)

func (t EntryType) String() string {
	switch t {
	case EntryTypeBlob:
		return "blob"
	case EntryTypeTree:
		return "tree"
	default:
		return "unknown"
	}
}

const (
	ModeFileRegular    = "100644"
	ModeFileExecutable = "100755"
	ModeDirectory      = "040000"
	ModeSymlink        = "120000"
)

// TreeEntry represents an entry inside a TreeNode directory
type TreeEntry struct {
	Mode string    `json:"mode"`
	Name string    `json:"name"`
	Hash string    `json:"hash"`
	Type EntryType `json:"type"`
	Size int64     `json:"size,omitempty"`
}

// TreeNode represents a directory in the Merkle tree
type TreeNode struct {
	Hash    string      `json:"hash"`
	Entries []TreeEntry `json:"entries"`
}

// NewTreeNode creates an empty TreeNode
func NewTreeNode() *TreeNode {
	return &TreeNode{
		Entries: make([]TreeEntry, 0),
	}
}

// AddEntry adds an entry and maintains entries sorted by Name
func (n *TreeNode) AddEntry(entry TreeEntry) {
	// Check if already exists; if so, replace
	for i, e := range n.Entries {
		if e.Name == entry.Name {
			n.Entries[i] = entry
			return
		}
	}
	n.Entries = append(n.Entries, entry)
	sort.Slice(n.Entries, func(i, j int) bool {
		return n.Entries[i].Name < n.Entries[j].Name
	})
}

// FindEntry finds an entry by name
func (n *TreeNode) FindEntry(name string) *TreeEntry {
	for i := range n.Entries {
		if n.Entries[i].Name == name {
			return &n.Entries[i]
		}
	}
	return nil
}
