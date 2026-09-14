# Echo Agent Integration Specification (AGENTS.md)

This specification defines instructions and operational contracts for autonomous AI coding agents (e.g. Claude Code, OpenAI Operator, Devin, Cursor, Gemini, and custom agent harnesses) to install, configure, and operate Echo programmatically within any execution environment.

---

## 1. System Overview

Echo is a headless, content-addressable codebase state tracker and history DAG. It operates independently of Git and provides:
- Atomic sub-second checkpoints (< 15ms) across bulk multi-file edits.
- Cryptographic Merkle tree verification and zlib blob deduplication.
- Pure Go embedded SQLite database with Full-Text Search (FTS5).
- Deterministic single-step rollbacks with append-only DAG tracking.
- Pre-mutation safety auto-saves guarding branch switches, reverts, and merges.
- Full machine-readable `--json` output across all commands.

---

## 2. Non-Interactive Machine Installation

Agents can install Echo non-interactively in standard container, virtual machine, or bare-metal environments.

### 2.1 Linux & macOS (POSIX Shell)

Run the automated installer script:
```bash
curl -fsSL https://raw.githubusercontent.com/rimraf-adi/echo/main/install.sh | sh
```

- **Target Binary Path**: `/usr/local/bin/echo` (if root/writable) or `~/.local/bin/echo`.
- **Environment Update**: Automatically appends the binary directory to `~/.bashrc`, `~/.zshrc`, or fish configuration.
- **Immediate Shell Availability**: If the installer placed the binary in `~/.local/bin`, ensure it is present in the current process `PATH`:
  ```bash
  export PATH="$HOME/.local/bin:$PATH"
  ```

### 2.2 Windows (PowerShell)

Run the automated PowerShell installer:
```powershell
irm https://raw.githubusercontent.com/rimraf-adi/echo/main/install.ps1 | iex
```

- **Target Binary Path**: `%LOCALAPPDATA%\echo\bin\echo.exe`.
- **Environment Update**: Persisted to the Windows Registry User `PATH` and exported to the active PowerShell process.

### 2.3 Go Toolchain Fallback

If Go 1.24+ is installed:
```bash
go install github.com/rimraf-adi/echo/cmd/echo@latest
```

### 2.4 Installation Verification

Verify successful installation and executable access:
```bash
echo init --help
```
Exit code must be `0`.

---

## 3. Standard Agent Execution Lifecycle

When an agent enters a target codebase, it should adhere to the following 5-phase lifecycle:

### Phase 1: Initialize Tracking

Check if `.echo/` exists in the repository root. If not, initialize tracking:
```bash
echo init --json
```

**Response Format:**
```json
{
  "workspace_id": "ws-a1b2c3d4",
  "initial_checkpoint": "cp-20260914-043250-eed5e8b8",
  "files_tracked": 128,
  "total_size_bytes": 1048576,
  "compressed_size_bytes": 419430,
  "tree_hash": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
}
```

### Phase 2: Establish State Tracking

Choose between **Automated Mode** (recommended for bulk write loops) or **Explicit Mode** (recommended for targeted milestones):

#### Mode A: Automated Filesystem Watcher (Background Daemon)
Run the watcher in the background before initiating multi-file modifications. The watcher aggregates rapid writes and automatically creates an atomic checkpoint when disk I/O settles:
```bash
echo watch --debounce 500 --agent "my-agent-id" --json &
WATCHER_PID=$!
```
- `--debounce <ms>`: Delay after last write event before committing checkpoint (default: `1000`).
- Termination: Kill `$WATCHER_PID` when agent finishes the task.

#### Mode B: Explicit Checkpointing
Run a manual checkpoint after completing a cohesive logical change:
```bash
echo checkpoint --agent "my-agent-id" --task "refactor authentication" --tag "auth" --json
```

**Response Format:**
```json
{
  "checkpoint_id": "cp-20260914-043347-09f2f4a2",
  "parents": ["cp-20260914-043250-eed5e8b8"],
  "tree_hash": "173fdb6fe01894fa65baedc2fe20e5ce915ea456a1b0ed397a19b5a6cac8a28b",
  "changeset": {
    "added": ["src/api/auth.py"],
    "modified": ["main.py"],
    "deleted": []
  },
  "stats": {
    "total_files": 130,
    "total_size_bytes": 1052300,
    "blobs_reused": 128,
    "blobs_new": 2,
    "duration_ms": 11
  }
}
```
*Note: If the working tree has no uncommitted changes, `echo checkpoint` returns exit code `0` with `{"status": "clean", "message": "nothing to checkpoint, working tree clean"}`.*

### Phase 3: Code Inspection & Semantic Search

Agents can query the tracked state without external grep tools:

#### 1. Fast Full-Text Search (FTS5)
Search tracked files at current HEAD with surrounding context lines:
```bash
echo search "def authenticate_user" --context 2 --json
```
**Response Format:**
```json
{
  "query": "def authenticate_user",
  "checkpoint_id": "cp-20260914-043347-09f2f4a2",
  "matches": [
    {
      "path": "src/api/auth.py",
      "line_number": 42,
      "content": "def authenticate_user(token: str) -> User:",
      "context_before": ["class SecurityManager:", "    # Token verification"],
      "context_after": ["    payload = decode_jwt(token)", "    return User.from_payload(payload)"]
    }
  ],
  "total_matches": 1
}
```

#### 2. Inspect File List & Metadata
List all files tracked at any checkpoint:
```bash
echo files --at HEAD --json
```

#### 3. Read Historical File Contents
Fetch raw content of any file from any previous checkpoint:
```bash
echo cat src/api/auth.py --at HEAD~1
```

### Phase 4: Speculative Branching & Parallel Exploration

When testing hypotheses, agents should isolate changes in dedicated branches:

```bash
# 1. Create and switch to exploratory branch
echo branch experiment-approach-b --json
echo switch experiment-approach-b --json

# 2. Modify files and verify execution...

# 3. If successful, switch back to main and merge with automated conflict policy
echo switch main --json
echo merge experiment-approach-b --strategy theirs --json
```

**Conflict Policies:**
- `--strategy theirs`: In the event of conflicting modifications to the same file, changes from the incoming branch overwrite the target branch.
- `--strategy ours`: In the event of conflicting modifications to the same file, changes on the target branch are preserved.

### Phase 5: Deterministic Rollback (Error Recovery)

If mutations cause syntax errors, failed test suites, or unintended side effects, execute an immediate rollback:
```bash
echo revert HEAD~1 --json
```
**Response Format:**
```json
{
  "reverted_to": "cp-20260914-043250-eed5e8b8",
  "revert_checkpoint": "cp-20260914-044831-7e0c0057",
  "changes_applied": {
    "added": [],
    "modified": ["main.py"],
    "deleted": ["src/api/auth.py"]
  }
}
```

**Safety Guarantee**: If the agent had uncommitted changes on disk before issuing `echo revert`, Echo automatically creates an intermediary checkpoint (`agent: safety-guard`) before modifying the filesystem. Uncommitted work is never lost.

---

## 4. Programmatic Command Reference

| Command | Recommended Arguments | Description |
|---|---|---|
| `echo init` | `--json` | Initialize tracking in project root |
| `echo status` | `--json` | Return active branch, HEAD ID, and uncheckpointed deltas |
| `echo checkpoint` | `--agent <name> --task <task> --json` | Commit working tree changes |
| `echo watch` | `--debounce <ms> --agent <name> --json` | Run background auto-checkpoint daemon |
| `echo revert <ref>` | `--json` | Rollback working tree to target checkpoint |
| `echo diff [ref1] [ref2]` | `--stat --json` | Line-level Myers diff between checkpoints or working tree |
| `echo log` | `-n <limit> --json` | History DAG and changeset records |
| `echo show <cp-id>` | `--json` | Detailed metadata and file list for a checkpoint |
| `echo files` | `--at <ref> --json` | Tracked files at specified reference |
| `echo cat <path>` | `--at <ref>` | Stream raw file contents to stdout |
| `echo search <query>` | `--context <lines> --json` | Full-text FTS5 index search |
| `echo branch <name>` | `--from <ref> --json` | Create new branch |
| `echo switch <name>` | `--json` | Switch active branch (auto-saves dirty tree first) |
| `echo branches` | `--json` | List existing branches |
| `echo merge <branch>` | `--strategy theirs --json` | Three-way LCA merge (auto-saves dirty tree first) |
| `echo verify` | `--json` | Cryptographic SHA-256 audit of all stored objects |
| `echo gc` | `--json` | Purge unreferenced blobs and trees |

---

## 5. Exit Codes & Error Protocols

All Echo commands return standard exit codes:
- `0`: Execution succeeded. Valid JSON output written to `stdout`.
- `1`: Operational error (e.g. unresolved reference, lock collision, invalid state). Error description written to `stderr`.
- `2`: Command line usage error (e.g. missing required argument, invalid flag). Usage text written to `stderr`.

### Error Handling Rules for Agents:
1. **Lock Contention**: If another process holds the lock, Echo waits up to 5 seconds. If timeout occurs, check for stale lockfiles in `.echo/lock`.
2. **Untracked / Ignored Files**: Files matching `.echo/ignore` (or default ignores `.git`, `.echo`) are excluded from checkpoints and will not be reverted.
3. **Reference Shorthands**: The following references are always valid: `HEAD`, `HEAD~1`, `HEAD~N`, branch names (e.g. `main`), and checkpoint IDs (or unambiguous 8+ character prefixes).
