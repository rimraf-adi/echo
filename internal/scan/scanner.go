package scan

import (
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/echo-vcs/echo/internal/ignore"
)

// FileInfo holds metadata about a scanned file
type FileInfo struct {
	Path    string      `json:"path"`
	Size    int64       `json:"size"`
	Mode    os.FileMode `json:"mode"`
	ModTime time.Time   `json:"mod_time"`
	Hash    string      `json:"hash"`
}

// ScanResult holds the collected files from a workspace scan
type ScanResult struct {
	Root  string               `json:"root"`
	Files map[string]*FileInfo `json:"files"`
}

// Scanner scans directories respecting ignore rules
type Scanner struct {
	matcher *ignore.Matcher
}

// NewScanner creates a Scanner with the given ignore matcher
func NewScanner(matcher *ignore.Matcher) *Scanner {
	if matcher == nil {
		matcher = ignore.NewMatcher(nil)
	}
	return &Scanner{matcher: matcher}
}

// Scan traverses rootDir and returns all non-ignored files
func (s *Scanner) Scan(rootDir string) (*ScanResult, error) {
	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, err
	}

	result := &ScanResult{
		Root:  absRoot,
		Files: make(map[string]*FileInfo),
	}

	err = filepath.WalkDir(absRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(absRoot, path)
		if err != nil {
			return err
		}

		if relPath == "." {
			return nil
		}

		cleanRel := filepath.ToSlash(relPath)
		isDir := d.IsDir()

		if s.matcher.Match(cleanRel, isDir) {
			if isDir {
				return fs.SkipDir
			}
			return nil
		}

		if isDir {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		result.Files[cleanRel] = &FileInfo{
			Path:    cleanRel,
			Size:    info.Size(),
			Mode:    info.Mode(),
			ModTime: info.ModTime(),
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}
