package ignore

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Pattern represents a single ignore pattern rule
type Pattern struct {
	Raw        string
	Pattern    string
	IsNegation bool
	IsDirOnly  bool
	IsRootOnly bool
	regex      *regexp.Regexp
}

// Matcher evaluates file paths against a set of ignore patterns
type Matcher struct {
	patterns []Pattern
}

// NewMatcher creates a Matcher from raw string lines, prepending DefaultPatterns
func NewMatcher(lines []string) *Matcher {
	m := &Matcher{
		patterns: make([]Pattern, 0, len(DefaultPatterns)+len(lines)),
	}

	for _, p := range DefaultPatterns {
		m.addPattern(p)
	}

	for _, line := range lines {
		m.addPattern(line)
	}

	return m
}

func (m *Matcher) addPattern(raw string) {
	line := strings.TrimSpace(raw)
	if line == "" || strings.HasPrefix(line, "#") {
		return
	}

	p := Pattern{Raw: line}

	if strings.HasPrefix(line, "!") {
		p.IsNegation = true
		line = line[1:]
	}

	if strings.HasSuffix(line, "/") {
		p.IsDirOnly = true
		line = line[:len(line)-1]
	}

	if strings.HasPrefix(line, "/") {
		p.IsRootOnly = true
		line = line[1:]
	} else if !strings.Contains(line, "/") {
		p.IsRootOnly = false
	} else {
		p.IsRootOnly = true
	}

	p.Pattern = line
	p.regex = compilePattern(p)
	m.patterns = append(m.patterns, p)
}

func compilePattern(p Pattern) *regexp.Regexp {
	pat := p.Pattern

	var sb strings.Builder
	sb.WriteString("^")

	if !p.IsRootOnly {
		sb.WriteString("(?:.*/)?")
	}

	i := 0
	for i < len(pat) {
		if i+1 < len(pat) && pat[i:i+2] == "**" {
			// Handle /**/ or **
			if i+2 < len(pat) && pat[i+2] == '/' {
				sb.WriteString("(?:.*/)?")
				i += 3
				continue
			}
			sb.WriteString(".*")
			i += 2
			continue
		}

		c := pat[i]
		switch c {
		case '*':
			sb.WriteString("[^/]*")
		case '?':
			sb.WriteString("[^/]")
		case '.', '+', '(', ')', '[', ']', '{', '}', '^', '$', '|', '\\':
			sb.WriteByte('\\')
			sb.WriteByte(c)
		default:
			sb.WriteByte(c)
		}
		i++
	}

	sb.WriteString("(?:/.*)?$")

	re, err := regexp.Compile(sb.String())
	if err != nil {
		return nil
	}
	return re
}

// LoadIgnoreFile loads patterns from an ignore file on disk
func LoadIgnoreFile(path string) (*Matcher, error) {
	var lines []string

	data, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return NewMatcher(nil), nil
		}
		return nil, err
	}
	defer data.Close()

	scanner := bufio.NewScanner(data)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return NewMatcher(lines), nil
}

// Match checks if relPath is ignored.
// relPath must be relative to the workspace root, formatted with forward slashes.
func (m *Matcher) Match(relPath string, isDir bool) bool {
	cleanPath := filepath.ToSlash(filepath.Clean(relPath))
	if cleanPath == "." || cleanPath == "" {
		return false
	}
	cleanPath = strings.TrimPrefix(cleanPath, "/")

	ignored := false

	for _, p := range m.patterns {
		if p.regex == nil {
			continue
		}

		if p.IsDirOnly && !isDir {
			// Check if any directory component matches
			if p.regex.MatchString(cleanPath + "/") {
				if p.IsNegation {
					ignored = false
				} else {
					ignored = true
				}
			}
			continue
		}

		checkPath := cleanPath
		if isDir {
			checkPath += "/"
		}

		if p.regex.MatchString(checkPath) || p.regex.MatchString(cleanPath) {
			if p.IsNegation {
				ignored = false
			} else {
				ignored = true
			}
		}
	}

	return ignored
}
