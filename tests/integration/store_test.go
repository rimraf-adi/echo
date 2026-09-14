package integration

import (
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/echo-vcs/echo/internal/store"
)

func TestStoreRoundTrip(t *testing.T) {
	tempDir := t.TempDir()
	s := store.NewObjectStore(tempDir)

	content := []byte("hello world! This is a test blob that should be compressed nicely because it has repeated patterns and words like hello world and test blob.")
	hash, err := s.WriteBlob(content)
	if err != nil {
		t.Fatalf("WriteBlob failed: %v", err)
	}

	if len(hash) != 64 {
		t.Fatalf("expected 64-char hash, got %d", len(hash))
	}

	if !s.HasObject(hash) {
		t.Fatalf("HasObject returned false for newly written object")
	}

	readBack, err := s.ReadBlob(hash)
	if err != nil {
		t.Fatalf("ReadBlob failed: %v", err)
	}

	if string(readBack) != string(content) {
		t.Fatalf("content mismatch:\nexpected: %s\ngot: %s", string(content), string(readBack))
	}
}

func TestStoreDeduplication(t *testing.T) {
	tempDir := t.TempDir()
	s := store.NewObjectStore(tempDir)

	content := []byte("identical content for dedup test")
	hash1, err := s.WriteBlob(content)
	if err != nil {
		t.Fatalf("first write failed: %v", err)
	}

	hash2, err := s.WriteBlob(content)
	if err != nil {
		t.Fatalf("second write failed: %v", err)
	}

	if hash1 != hash2 {
		t.Fatalf("hashes differ for identical content: %s vs %s", hash1, hash2)
	}

	objects, err := s.ListObjects()
	if err != nil {
		t.Fatalf("ListObjects failed: %v", err)
	}

	if len(objects) != 1 {
		t.Fatalf("expected 1 object file due to dedup, got %d", len(objects))
	}
}

func TestStoreTinyFileUncompressed(t *testing.T) {
	tempDir := t.TempDir()
	s := store.NewObjectStore(tempDir)

	tinyContent := []byte("short")
	hash, err := s.WriteBlob(tinyContent)
	if err != nil {
		t.Fatalf("WriteBlob failed: %v", err)
	}

	path, err := s.ObjectPath(hash)
	if err != nil {
		t.Fatalf("ObjectPath failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading file failed: %v", err)
	}

	header, err := store.DecodeHeader(data)
	if err != nil {
		t.Fatalf("DecodeHeader failed: %v", err)
	}

	if header.IsCompressed() {
		t.Fatalf("expected tiny file (<64 bytes) to be uncompressed")
	}

	readBack, err := s.ReadBlob(hash)
	if err != nil {
		t.Fatalf("ReadBlob failed: %v", err)
	}
	if string(readBack) != string(tinyContent) {
		t.Fatalf("content mismatch")
	}
}

func TestStoreIncompressibleContent(t *testing.T) {
	tempDir := t.TempDir()
	s := store.NewObjectStore(tempDir)

	randomData := make([]byte, 1024)
	if _, err := rand.Read(randomData); err != nil {
		t.Fatalf("rand.Read failed: %v", err)
	}

	hash, err := s.WriteBlob(randomData)
	if err != nil {
		t.Fatalf("WriteBlob failed: %v", err)
	}

	path, err := s.ObjectPath(hash)
	if err != nil {
		t.Fatalf("ObjectPath failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading file failed: %v", err)
	}

	header, err := store.DecodeHeader(data)
	if err != nil {
		t.Fatalf("DecodeHeader failed: %v", err)
	}

	if header.IsCompressed() {
		t.Fatalf("expected random incompressible data to be stored uncompressed")
	}

	readBack, err := s.ReadBlob(hash)
	if err != nil {
		t.Fatalf("ReadBlob failed: %v", err)
	}
	if string(readBack) != string(randomData) {
		t.Fatalf("content mismatch")
	}
}

func TestStoreIntegrityCorruptionDetection(t *testing.T) {
	tempDir := t.TempDir()
	s := store.NewObjectStore(tempDir)

	content := []byte("integrity test content")
	hash, err := s.WriteBlob(content)
	if err != nil {
		t.Fatalf("WriteBlob failed: %v", err)
	}

	path, err := s.ObjectPath(hash)
	if err != nil {
		t.Fatalf("ObjectPath failed: %v", err)
	}

	// Corrupt a byte in the payload
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading object failed: %v", err)
	}

	data[len(data)-1] ^= 0xFF
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("corrupting object failed: %v", err)
	}

	_, err = s.ReadBlob(hash)
	if err == nil {
		t.Fatalf("expected error reading corrupted object, got nil")
	}
}

func TestStoreBulkRoundTrip(t *testing.T) {
	tempDir := t.TempDir()
	s := store.NewObjectStore(tempDir)

	count := 200
	hashes := make([]string, count)
	contents := make([][]byte, count)

	for i := 0; i < count; i++ {
		c := make([]byte, 128)
		_, _ = rand.Read(c)
		contents[i] = c

		h, err := s.WriteBlob(c)
		if err != nil {
			t.Fatalf("write %d failed: %v", i, err)
		}
		hashes[i] = h
	}

	for i := 0; i < count; i++ {
		readBack, err := s.ReadBlob(hashes[i])
		if err != nil {
			t.Fatalf("read %d (%s) failed: %v", i, hashes[i], err)
		}
		if string(readBack) != string(contents[i]) {
			t.Fatalf("content mismatch at %d", i)
		}
	}

	list, err := s.ListObjects()
	if err != nil {
		t.Fatalf("ListObjects failed: %v", err)
	}
	if len(list) != count {
		t.Fatalf("expected %d objects in list, got %d", count, len(list))
	}
}

func TestStoreDeleteAndExistence(t *testing.T) {
	tempDir := t.TempDir()
	s := store.NewObjectStore(tempDir)

	content := []byte("to be deleted")
	hash, err := s.WriteBlob(content)
	if err != nil {
		t.Fatalf("WriteBlob failed: %v", err)
	}

	if !s.HasObject(hash) {
		t.Fatalf("expected object to exist")
	}

	if err := s.DeleteObject(hash); err != nil {
		t.Fatalf("DeleteObject failed: %v", err)
	}

	if s.HasObject(hash) {
		t.Fatalf("expected object to not exist after deletion")
	}

	_, err = s.ReadBlob(hash)
	if err == nil {
		t.Fatalf("expected error reading deleted object")
	}
}

func TestHeaderEncodeDecode(t *testing.T) {
	var hash [32]byte
	copy(hash[:], []byte("0123456789abcdef0123456789abcdef"))

	original := store.Header{
		Magic:      store.MagicEcho,
		Version:    store.CurrentVersion,
		ObjType:    store.ObjectTypeTree,
		Flags:      store.FlagCompressed,
		Reserved:   0,
		OrigSize:   123456,
		StoredSize: 45678,
		Hash:       hash,
	}

	encoded := store.EncodeHeader(original)
	if len(encoded) != store.HeaderSize {
		t.Fatalf("expected header length %d, got %d", store.HeaderSize, len(encoded))
	}

	decoded, err := store.DecodeHeader(encoded)
	if err != nil {
		t.Fatalf("DecodeHeader failed: %v", err)
	}

	if decoded != original {
		t.Fatalf("header roundtrip mismatch:\norig: %+v\ngot:  %+v", original, decoded)
	}
}

func TestTreeObjectRoundTrip(t *testing.T) {
	tempDir := t.TempDir()
	s := store.NewObjectStore(tempDir)

	treeData := []byte("100644 a1b2c3d4e5f6 file1.go\n040000 1234567890ab dir1\n")
	hash, err := s.WriteTree(treeData)
	if err != nil {
		t.Fatalf("WriteTree failed: %v", err)
	}

	readBack, err := s.ReadTree(hash)
	if err != nil {
		t.Fatalf("ReadTree failed: %v", err)
	}

	if string(readBack) != string(treeData) {
		t.Fatalf("tree data mismatch:\nexpected: %s\ngot: %s", string(treeData), string(readBack))
	}

	// Reading tree as blob should fail
	_, err = s.ReadBlob(hash)
	if err == nil {
		t.Fatalf("expected ReadBlob on tree object to fail")
	}
}

func TestInvalidHashHandling(t *testing.T) {
	s := store.NewObjectStore(filepath.Join(os.TempDir(), "echo-test"))
	if _, err := s.ObjectPath("invalid"); err == nil {
		t.Fatalf("expected error for invalid hash length")
	}
	if s.HasObject("short") {
		t.Fatalf("expected HasObject to return false for short hash")
	}
}
