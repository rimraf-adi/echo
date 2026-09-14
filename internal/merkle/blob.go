package merkle

import "os"

// BlobNode represents a leaf node in the Merkle tree
type BlobNode struct {
	Hash string
	Size int64
	Mode os.FileMode
}

// NewBlobNode creates a new BlobNode
func NewBlobNode(hash string, size int64, mode os.FileMode) *BlobNode {
	return &BlobNode{
		Hash: hash,
		Size: size,
		Mode: mode,
	}
}
