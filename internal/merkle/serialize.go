package merkle

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/echo-vcs/echo/internal/store"
)

var (
	ErrInvalidTreeSerialization = errors.New("invalid tree serialization format")
)

// SerializeTree produces the canonical byte serialization for a TreeNode and updates its Hash
func SerializeTree(node *TreeNode) []byte {
	// Ensure entries are sorted lexicographically by name
	sort.Slice(node.Entries, func(i, j int) bool {
		return node.Entries[i].Name < node.Entries[j].Name
	})

	var buf bytes.Buffer
	for _, entry := range node.Entries {
		buf.WriteString(entry.Mode)
		buf.WriteByte(' ')
		buf.WriteString(entry.Hash)
		buf.WriteByte(' ')
		buf.WriteString(entry.Name)
		buf.WriteByte('\n')
	}

	data := buf.Bytes()
	node.Hash = store.HashBytes(data)
	return data
}

// DeserializeTree parses canonical bytes into a TreeNode and verifies its hash
func DeserializeTree(data []byte) (*TreeNode, error) {
	node := NewTreeNode()
	if len(data) == 0 {
		node.Hash = store.HashBytes(nil)
		return node, nil
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 3)
		if len(parts) != 3 {
			return nil, fmt.Errorf("%w: invalid line %q", ErrInvalidTreeSerialization, line)
		}

		mode := parts[0]
		hash := parts[1]
		name := parts[2]

		entryType := EntryTypeBlob
		if mode == ModeDirectory {
			entryType = EntryTypeTree
		}

		node.Entries = append(node.Entries, TreeEntry{
			Mode: mode,
			Name: name,
			Hash: hash,
			Type: entryType,
		})
	}

	sort.Slice(node.Entries, func(i, j int) bool {
		return node.Entries[i].Name < node.Entries[j].Name
	})

	node.Hash = store.HashBytes(data)
	return node, nil
}
