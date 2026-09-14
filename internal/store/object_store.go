package store

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	ErrObjectNotFound       = errors.New("object not found")
	ErrIntegrityCheckFailed = errors.New("object integrity check failed: hash mismatch")
	ErrInvalidHash          = errors.New("invalid object hash length")
)

// ObjectStore manages content-addressable storage for blobs and trees under .echo/objects/
type ObjectStore struct {
	basePath string
	mu       sync.RWMutex
}

// NewObjectStore initializes a new ObjectStore with the given root directory
func NewObjectStore(basePath string) *ObjectStore {
	return &ObjectStore{
		basePath: basePath,
	}
}

// BasePath returns the root directory for objects
func (s *ObjectStore) BasePath() string {
	return s.basePath
}

// ObjectPath returns the absolute on-disk path for an object hash
func (s *ObjectStore) ObjectPath(hash string) (string, error) {
	if len(hash) != 64 {
		return "", fmt.Errorf("%w: %s", ErrInvalidHash, hash)
	}
	return filepath.Join(s.basePath, hash[0:2], hash[2:]), nil
}

// HasObject checks if an object with the given hash exists
func (s *ObjectStore) HasObject(hash string) bool {
	path, err := s.ObjectPath(hash)
	if err != nil {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// WriteBlob stores raw file content as a blob object
func (s *ObjectStore) WriteBlob(content []byte) (string, error) {
	return s.WriteObject(ObjectTypeBlob, content)
}

// WriteTree stores canonical tree data as a tree object
func (s *ObjectStore) WriteTree(content []byte) (string, error) {
	return s.WriteObject(ObjectTypeTree, content)
}

// WriteObject writes an object with the specified type, compresses it if eligible, and atomically saves it
func (s *ObjectStore) WriteObject(objType ObjectType, content []byte) (string, error) {
	hash := HashBytes(content)

	// Deduplication: if object already exists, return hash immediately
	if s.HasObject(hash) {
		return hash, nil
	}

	payload, compressed, err := Compress(content, DefaultCompressionLevel)
	if err != nil {
		return "", fmt.Errorf("compressing object payload: %w", err)
	}

	var flags uint8
	if compressed {
		flags |= FlagCompressed
	}

	var rawHash [32]byte
	hashBytes, err := hex.DecodeString(hash)
	if err != nil {
		return "", fmt.Errorf("decoding hash: %w", err)
	}
	copy(rawHash[:], hashBytes)

	h := Header{
		Magic:      MagicEcho,
		Version:    CurrentVersion,
		ObjType:    objType,
		Flags:      flags,
		Reserved:   0,
		OrigSize:   uint32(len(content)),
		StoredSize: uint32(len(payload)),
		Hash:       rawHash,
	}

	headerBytes := EncodeHeader(h)
	fullData := make([]byte, len(headerBytes)+len(payload))
	copy(fullData[0:len(headerBytes)], headerBytes)
	copy(fullData[len(headerBytes):], payload)

	targetPath, err := s.ObjectPath(hash)
	if err != nil {
		return "", err
	}

	// Ensure target directory exists
	targetDir := filepath.Dir(targetPath)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return "", fmt.Errorf("creating object directory: %w", err)
	}

	// Ensure temp directory exists
	tmpDir := filepath.Join(s.basePath, "tmp")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return "", fmt.Errorf("creating temp directory: %w", err)
	}

	// Create random temp file
	randBytes := make([]byte, 8)
	_, _ = rand.Read(randBytes)
	tmpFile := filepath.Join(tmpDir, fmt.Sprintf("tmp-%d-%x", time.Now().UnixNano(), randBytes))

	if err := os.WriteFile(tmpFile, fullData, 0644); err != nil {
		return "", fmt.Errorf("writing temp object: %w", err)
	}

	// Atomic rename to final path
	if err := os.Rename(tmpFile, targetPath); err != nil {
		_ = os.Remove(tmpFile)
		return "", fmt.Errorf("renaming temp object: %w", err)
	}

	return hash, nil
}

// ReadBlob reads a blob object and returns its raw uncompressed content
func (s *ObjectStore) ReadBlob(hash string) ([]byte, error) {
	objType, content, err := s.ReadObject(hash)
	if err != nil {
		return nil, err
	}
	if objType != ObjectTypeBlob {
		return nil, fmt.Errorf("expected blob object, got %s", objType)
	}
	return content, nil
}

// ReadTree reads a tree object and returns its raw uncompressed content
func (s *ObjectStore) ReadTree(hash string) ([]byte, error) {
	objType, content, err := s.ReadObject(hash)
	if err != nil {
		return nil, err
	}
	if objType != ObjectTypeTree {
		return nil, fmt.Errorf("expected tree object, got %s", objType)
	}
	return content, nil
}

// ReadObject reads any object from disk, verifies integrity, and returns its raw content
func (s *ObjectStore) ReadObject(hash string) (ObjectType, []byte, error) {
	path, err := s.ObjectPath(hash)
	if err != nil {
		return 0, nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil, fmt.Errorf("%w: %s", ErrObjectNotFound, hash)
		}
		return 0, nil, err
	}

	header, err := DecodeHeader(data)
	if err != nil {
		return 0, nil, fmt.Errorf("decoding object header for %s: %w", hash, err)
	}

	payload := data[HeaderSize:]
	var rawContent []byte

	if header.IsCompressed() {
		rawContent, err = Decompress(payload)
		if err != nil {
			return 0, nil, fmt.Errorf("decompressing object %s: %w", hash, err)
		}
	} else {
		rawContent = payload
	}

	// Verify integrity
	actualHash := HashBytes(rawContent)
	if actualHash != header.HashHex() || actualHash != hash {
		return 0, nil, fmt.Errorf("%w: expected %s, got %s", ErrIntegrityCheckFailed, hash, actualHash)
	}

	return header.ObjType, rawContent, nil
}

// DeleteObject removes an object file
func (s *ObjectStore) DeleteObject(hash string) error {
	path, err := s.ObjectPath(hash)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// ListObjects returns all object hashes stored in the object store
func (s *ObjectStore) ListObjects() ([]string, error) {
	var hashes []string

	entries, err := os.ReadDir(s.basePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() || len(entry.Name()) != 2 || entry.Name() == "tmp" {
			continue
		}
		subDir := filepath.Join(s.basePath, entry.Name())
		subEntries, err := os.ReadDir(subDir)
		if err != nil {
			continue
		}
		for _, fileEntry := range subEntries {
			if fileEntry.IsDir() {
				continue
			}
			if len(fileEntry.Name()) == 62 {
				hash := entry.Name() + fileEntry.Name()
				hashes = append(hashes, strings.ToLower(hash))
			}
		}
	}

	return hashes, nil
}
