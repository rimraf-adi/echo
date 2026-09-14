package store

import (
	"bytes"
	"compress/zlib"
	"io"
)

const (
	// DefaultCompressionLevel is standard zlib level 6
	DefaultCompressionLevel = 6
	// MinCompressionSize is the minimum byte size to attempt compression
	MinCompressionSize = 64
	// IncompressibleRatio is the threshold above which content is stored raw (95%)
	IncompressibleRatio = 0.95
)

// Compress attempts to compress the given data using zlib.
// Returns compressed data and true if compressed, or original data and false if skipped.
func Compress(data []byte, level int) ([]byte, bool, error) {
	if len(data) < MinCompressionSize {
		return data, false, nil
	}

	if level <= 0 || level > zlib.BestCompression {
		level = DefaultCompressionLevel
	}

	var buf bytes.Buffer
	w, err := zlib.NewWriterLevel(&buf, level)
	if err != nil {
		return nil, false, err
	}

	if _, err := w.Write(data); err != nil {
		return nil, false, err
	}
	if err := w.Close(); err != nil {
		return nil, false, err
	}

	compressed := buf.Bytes()
	if float64(len(compressed)) >= float64(len(data))*IncompressibleRatio {
		return data, false, nil
	}

	return compressed, true, nil
}

// Decompress decompresses zlib-compressed data.
func Decompress(data []byte) ([]byte, error) {
	r, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer r.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
