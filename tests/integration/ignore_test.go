package integration

import (
	"testing"

	"github.com/echo-vcs/echo/internal/ignore"
)

func TestIgnoreDefaults(t *testing.T) {
	m := ignore.NewMatcher(nil)

	if !m.Match(".echo/config.json", false) {
		t.Fatalf("expected .echo/config.json to be ignored")
	}
	if !m.Match(".echo", true) {
		t.Fatalf("expected .echo dir to be ignored")
	}
	if !m.Match(".git/HEAD", false) {
		t.Fatalf("expected .git/HEAD to be ignored")
	}
	if m.Match("main.go", false) {
		t.Fatalf("expected main.go to not be ignored")
	}
}

func TestIgnorePatterns(t *testing.T) {
	patterns := []string{
		"*.log",
		"# comment line",
		"",
		"node_modules/",
		"!important.log",
		"build/**",
		"/root_only.txt",
	}

	m := ignore.NewMatcher(patterns)

	// *.log ignored
	if !m.Match("app.log", false) {
		t.Fatalf("expected app.log to be ignored")
	}
	if !m.Match("sub/dir/test.log", false) {
		t.Fatalf("expected sub/dir/test.log to be ignored")
	}

	// Negation !important.log
	if m.Match("important.log", false) {
		t.Fatalf("expected !important.log to NOT be ignored")
	}

	// Directory node_modules/
	if !m.Match("node_modules", true) {
		t.Fatalf("expected node_modules dir to be ignored")
	}
	if !m.Match("node_modules/express/index.js", false) {
		t.Fatalf("expected file inside node_modules to be ignored")
	}

	// build/**
	if !m.Match("build/bin/app", false) {
		t.Fatalf("expected build/bin/app to be ignored")
	}

	// /root_only.txt
	if !m.Match("root_only.txt", false) {
		t.Fatalf("expected root_only.txt at root to be ignored")
	}
	if m.Match("sub/root_only.txt", false) {
		t.Fatalf("expected sub/root_only.txt to NOT be ignored")
	}
}
