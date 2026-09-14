package integration

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func runEchoCLI(t *testing.T, workDir string, args ...string) (string, string, int) {
	t.Helper()

	// Find echo binary in bin/echo
	root, err := filepath.Abs("../../bin/echo")
	if err != nil {
		t.Fatalf("locating binary failed: %v", err)
	}

	cmd := exec.Command(root, args...)
	cmd.Dir = workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	return stdout.String(), stderr.String(), exitCode
}

func TestCLIWorkflowEndToEnd(t *testing.T) {
	tempDir := t.TempDir()

	// 1. Write initial files
	_ = os.WriteFile(filepath.Join(tempDir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0644)
	_ = os.WriteFile(filepath.Join(tempDir, "README.md"), []byte("# Echo Test\n"), 0644)

	// 2. echo init --json
	stdout, stderr, code := runEchoCLI(t, tempDir, "init", "--json")
	if code != 0 {
		t.Fatalf("echo init failed (%d): %s", code, stderr)
	}

	var initResp map[string]any
	if err := json.Unmarshal([]byte(stdout), &initResp); err != nil {
		t.Fatalf("invalid json from init: %v\noutput: %s", err, stdout)
	}
	if initResp["files_tracked"].(float64) != 2 {
		t.Fatalf("expected 2 files tracked, got %v", initResp["files_tracked"])
	}

	// 3. echo status --json (clean)
	stdout, _, code = runEchoCLI(t, tempDir, "status", "--json")
	if code != 0 {
		t.Fatalf("echo status failed: %d", code)
	}
	var statusResp map[string]any
	_ = json.Unmarshal([]byte(stdout), &statusResp)
	if statusResp["total_changes"].(float64) != 0 {
		t.Fatalf("expected 0 changes in clean status, got %v", statusResp["total_changes"])
	}

	// 4. Modify main.go and add helper.go
	_ = os.WriteFile(filepath.Join(tempDir, "main.go"), []byte("package main\n\nimport \"fmt\"\n\nfunc main() {\n    fmt.Println(\"Hello\")\n}\n"), 0644)
	_ = os.WriteFile(filepath.Join(tempDir, "helper.go"), []byte("package main\n\nfunc Helper() string { return \"ok\" }\n"), 0644)

	// 5. echo checkpoint --json
	stdout, _, code = runEchoCLI(t, tempDir, "checkpoint", "--agent", "claude-test", "--task", "add helper", "--json")
	if code != 0 {
		t.Fatalf("echo checkpoint failed: %d", code)
	}
	var cpResp map[string]any
	if err := json.Unmarshal([]byte(stdout), &cpResp); err != nil {
		t.Fatalf("invalid json from checkpoint: %v\noutput: %s", err, stdout)
	}
	cpID := cpResp["checkpoint_id"].(string)

	// 6. echo files --json
	stdout, _, code = runEchoCLI(t, tempDir, "files", "--json")
	if code != 0 {
		t.Fatalf("echo files failed: %d", code)
	}
	var filesResp map[string]any
	_ = json.Unmarshal([]byte(stdout), &filesResp)
	if filesResp["total"].(float64) != 3 {
		t.Fatalf("expected 3 files, got %v", filesResp["total"])
	}

	// 7. echo cat helper.go
	stdout, _, code = runEchoCLI(t, tempDir, "cat", "helper.go")
	if code != 0 {
		t.Fatalf("echo cat failed: %d", code)
	}
	if !strings.Contains(stdout, "Helper() string") {
		t.Fatalf("unexpected cat output: %s", stdout)
	}

	// 8. echo search "Helper" --json
	stdout, _, code = runEchoCLI(t, tempDir, "search", "Helper", "--json")
	if code != 0 {
		t.Fatalf("echo search failed: %d", code)
	}
	var searchResp map[string]any
	_ = json.Unmarshal([]byte(stdout), &searchResp)
	if searchResp["total_matches"].(float64) < 1 {
		t.Fatalf("expected matches for Helper, got %v", searchResp["total_matches"])
	}

	// 9. echo diff HEAD~1 HEAD --json
	stdout, _, code = runEchoCLI(t, tempDir, "diff", "HEAD~1", "HEAD", "--json")
	if code != 0 {
		t.Fatalf("echo diff failed: %d", code)
	}
	var diffResp map[string]any
	_ = json.Unmarshal([]byte(stdout), &diffResp)
	files := diffResp["files"].([]any)
	if len(files) != 2 {
		t.Fatalf("expected 2 files in diff, got %d", len(files))
	}

	// 10. echo show <cp-id> --json
	stdout, _, code = runEchoCLI(t, tempDir, "show", cpID, "--json")
	if code != 0 {
		t.Fatalf("echo show failed: %d", code)
	}
	var showResp map[string]any
	_ = json.Unmarshal([]byte(stdout), &showResp)
	if showResp["id"].(string) != cpID {
		t.Fatalf("show ID mismatch: %v vs %s", showResp["id"], cpID)
	}

	// 11. echo log --json
	stdout, _, code = runEchoCLI(t, tempDir, "log", "--json")
	if code != 0 {
		t.Fatalf("echo log failed: %d", code)
	}
	var logResp map[string]any
	_ = json.Unmarshal([]byte(stdout), &logResp)
	cps := logResp["checkpoints"].([]any)
	if len(cps) != 2 {
		t.Fatalf("expected 2 checkpoints in log, got %d", len(cps))
	}

	// 12. echo branch feature-b
	stdout, stderr, code = runEchoCLI(t, tempDir, "branch", "feature-b")
	if code != 0 {
		t.Fatalf("echo branch failed: %s", stderr)
	}

	// 13. echo switch feature-b
	stdout, stderr, code = runEchoCLI(t, tempDir, "switch", "feature-b")
	if code != 0 {
		t.Fatalf("echo switch failed: %s", stderr)
	}

	// 14. echo verify --json
	stdout, _, code = runEchoCLI(t, tempDir, "verify", "--json")
	if code != 0 {
		t.Fatalf("echo verify failed: %d", code)
	}
	var verifyResp map[string]any
	_ = json.Unmarshal([]byte(stdout), &verifyResp)
	if verifyResp["corrupted"].(float64) != 0 {
		t.Fatalf("expected 0 corrupted objects, got %v", verifyResp["corrupted"])
	}

	// 15. echo revert HEAD~1 --json
	stdout, _, code = runEchoCLI(t, tempDir, "revert", "HEAD~1", "--json")
	if code != 0 {
		t.Fatalf("echo revert failed: %d", code)
	}
	var revertResp map[string]any
	_ = json.Unmarshal([]byte(stdout), &revertResp)
	if revertResp["revert_checkpoint"] == "" {
		t.Fatalf("expected revert checkpoint to be created")
	}

	// helper.go should now be deleted!
	if _, err := os.Stat(filepath.Join(tempDir, "helper.go")); !os.IsNotExist(err) {
		t.Fatalf("helper.go should have been deleted by revert")
	}

	// 16. echo gc --json
	stdout, _, code = runEchoCLI(t, tempDir, "gc", "--json")
	if code != 0 {
		t.Fatalf("echo gc failed: %d", code)
	}
	var gcResp map[string]any
	_ = json.Unmarshal([]byte(stdout), &gcResp)
	if gcResp["objects_scanned"].(float64) == 0 {
		t.Fatalf("expected objects scanned in GC")
	}
}
