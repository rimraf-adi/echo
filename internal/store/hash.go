package store

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
)

// HashBytes computes the SHA-256 hash of data and returns it as a hex string.
func HashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// HashReader computes the SHA-256 hash of an io.Reader and returns it as a hex string.
func HashReader(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
