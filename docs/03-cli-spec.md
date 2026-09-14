# Echo — CLI Specification

Every command supports `--json` for machine-readable output. When `--json` is set, all output is valid JSON written to stdout. Human-readable output is the default.

Error output always goes to stderr. Exit code 0 = success, 1 = error, 2 = usage error.

---

## Global Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--json` | `-j` | Output as JSON |
| `--quiet` | `-q` | Suppress non-essential output |
| `--verbose` | `-v` | Verbose output (debug info) |
| `--dir <path>` | `-C` | Run as if echo was started in `<path>` |

---

## `echo init`

Initialize Echo tracking in the current directory.

```
echo init [--ignore <path>]
```

| Flag | Description |
|------|-------------|
| `--ignore <path>` | Path to an ignore file to copy as `.echo/ignore` |

**What it does:**
1. Creates `.echo/` directory structure.
2. Creates `config.json` with workspace ID and defaults.
3. Creates `HEAD` pointing to `main`.
4. Creates the initial checkpoint capturing current codebase state.
5. Builds full Merkle tree and stores all blobs.
6. Initializes `index.db`.

**JSON output:**
```json
{
  "workspace_id": "ws-a1b2c3d4",
  "initial_checkpoint": "cp-20260914-093000-a1b2c3d4",
  "files_tracked": 847,
  "total_size_bytes": 2341567,
  "compressed_size_bytes": 891204,
  "tree_hash": "ab3def78..."
}
```

**Error cases:**
- `.echo/` already exists → error (use `--force` to reinitialize)
- Directory is empty → warning, creates checkpoint with empty tree

---

## `echo checkpoint`

Create a new checkpoint capturing the current codebase state.

```
echo checkpoint [--agent <name>] [--task <desc>] [--model <name>] [--tag <tag>]... [--meta <key>=<value>]... [--message <msg>]
```

| Flag | Description |
|------|-------------|
| `--agent <name>` | Agent identifier (e.g., "claude-opus-4-20250514", "cursor") |
| `--task <desc>` | Task description |
| `--model <name>` | Model name |
| `--tag <tag>` | Tag (repeatable) |
| `--meta <key>=<value>` | Custom metadata key-value pair (repeatable) |
| `--message <msg>` | Human-readable message (optional) |

**What it does:**
1. Scans workspace for changes since last checkpoint.
2. If no changes detected, prints "nothing to checkpoint" and exits (code 0).
3. Hashes all changed/new files, stores new blobs.
4. Builds updated Merkle tree (reuses unchanged subtrees).
5. Creates checkpoint JSON with changeset and metadata.
6. Updates branch ref and HEAD.
7. Updates index.db.

**JSON output:**
```json
{
  "checkpoint_id": "cp-20260914-093500-b2c3d4e5",
  "parent": "cp-20260914-093000-a1b2c3d4",
  "tree_hash": "cd4ef890...",
  "changeset": {
    "added": ["internal/auth/handler.go"],
    "modified": ["cmd/server/main.go"],
    "deleted": []
  },
  "stats": {
    "files_changed": 2,
    "blobs_new": 2,
    "blobs_reused": 845,
    "duration_ms": 127
  }
}
```

---

## `echo status`

Show current workspace status: branch, HEAD checkpoint, and uncommitted changes.

```
echo status
```

**Human output:**
```
Branch: main
HEAD:   cp-20260914-093500-b2c3d4e5 (2 minutes ago, agent: claude-opus-4-20250514)

Changes since last checkpoint:
  modified: internal/routes/router.go
  added:    internal/auth/jwt.go
  deleted:  internal/auth/basic.go

3 files changed
```

**JSON output:**
```json
{
  "branch": "main",
  "head": "cp-20260914-093500-b2c3d4e5",
  "head_created_at": "2026-09-14T09:35:00Z",
  "changes": {
    "added": ["internal/auth/jwt.go"],
    "modified": ["internal/routes/router.go"],
    "deleted": ["internal/auth/basic.go"]
  },
  "total_changes": 3
}
```

---

## `echo log`

Show checkpoint history.

```
echo log [--limit <n>] [--agent <name>] [--task <desc>] [--since <date>] [--until <date>] [--branch <name>] [--graph]
```

| Flag | Description |
|------|-------------|
| `--limit <n>` | Show at most n checkpoints (default: 20) |
| `--agent <name>` | Filter by agent name |
| `--task <desc>` | Filter by task (substring match) |
| `--since <date>` | Show checkpoints after this date (ISO 8601 or relative: "1h", "2d") |
| `--until <date>` | Show checkpoints before this date |
| `--branch <name>` | Show history for a specific branch |
| `--graph` | Show ASCII DAG visualization |

**Human output:**
```
cp-20260914-093500-b2c3d4e5  2 min ago   agent:claude-opus-4-20250514  task:add-auth     +2 ~1 -0
cp-20260914-093000-a1b2c3d4  7 min ago   agent:gemini-flash  task:init-project  +847 ~0 -0
```

**Human output with `--graph`:**
```
* cp-20260914-094000-c3d4e5f6  (HEAD → main) merge: approach-b
|\
| * cp-20260914-093800-f6e5d4c3  (approach-b) try alternative auth
| * cp-20260914-093600-e5d4c3b2  start approach b
|/
* cp-20260914-093500-b2c3d4e5  add auth handlers
* cp-20260914-093000-a1b2c3d4  initial checkpoint
```

**JSON output:**
```json
{
  "checkpoints": [
    {
      "id": "cp-20260914-093500-b2c3d4e5",
      "parents": ["cp-20260914-093000-a1b2c3d4"],
      "created_at": "2026-09-14T09:35:00Z",
      "agent": "claude-opus-4-20250514",
      "task": "add-auth",
      "changeset_summary": { "added": 2, "modified": 1, "deleted": 0 }
    }
  ]
}
```

---

## `echo diff`

Show differences between two checkpoints, or between a checkpoint and the working directory.

```
echo diff [<cp-a>] [<cp-b>] [--stat] [--name-only]
```

| Argument | Description |
|----------|-------------|
| (no args) | Diff working directory vs HEAD |
| `<cp-a>` | Diff working directory vs checkpoint `<cp-a>` |
| `<cp-a> <cp-b>` | Diff between two checkpoints |

| Flag | Description |
|------|-------------|
| `--stat` | Show diffstat summary only (files + lines changed) |
| `--name-only` | Show only file names that changed |

**Human output:**
```
diff internal/auth/handler.go (added)
+++ b/internal/auth/handler.go
@@ -0,0 +1,45 @@
+package auth
+
+import "net/http"
+...

diff cmd/server/main.go (modified)
--- a/cmd/server/main.go
+++ b/cmd/server/main.go
@@ -12,6 +12,8 @@
 import (
     "fmt"
+    "myapp/internal/auth"
 )
```

**JSON output:**
```json
{
  "from": "cp-20260914-093000-a1b2c3d4",
  "to": "cp-20260914-093500-b2c3d4e5",
  "deltas": [
    {
      "path": "internal/auth/handler.go",
      "type": "added",
      "new_hash": "a1b2c3...",
      "new_size": 1234,
      "lines_added": 45,
      "lines_removed": 0
    },
    {
      "path": "cmd/server/main.go",
      "type": "modified",
      "old_hash": "x9y8z7...",
      "new_hash": "d4e5f6...",
      "lines_added": 2,
      "lines_removed": 0,
      "hunks": [
        {
          "old_start": 12, "old_count": 6,
          "new_start": 12, "new_count": 8,
          "lines": [
            { "type": "context", "content": "import (" },
            { "type": "context", "content": "    \"fmt\"" },
            { "type": "add", "content": "    \"myapp/internal/auth\"" },
            { "type": "context", "content": ")" }
          ]
        }
      ]
    }
  ]
}
```

---

## `echo show`

Show details of a specific checkpoint.

```
echo show <cp-id>
```

**JSON output:**
```json
{
  "id": "cp-20260914-093500-b2c3d4e5",
  "parents": ["cp-20260914-093000-a1b2c3d4"],
  "tree_hash": "cd4ef890...",
  "created_at": "2026-09-14T09:35:00Z",
  "metadata": {
    "agent": "claude-opus-4-20250514",
    "task": "add-auth",
    "tags": ["auth"]
  },
  "changeset": {
    "added": ["internal/auth/handler.go", "internal/auth/middleware.go"],
    "modified": ["cmd/server/main.go"],
    "deleted": []
  },
  "stats": {
    "total_files": 849,
    "total_size_bytes": 2345678,
    "blobs_reused": 845,
    "blobs_new": 4
  }
}
```

---

## `echo files`

List tracked files.

```
echo files [--at <cp-id>] [--pattern <glob>] [--sort <field>]
```

| Flag | Description |
|------|-------------|
| `--at <cp-id>` | List files at a specific checkpoint (default: HEAD) |
| `--pattern <glob>` | Filter by glob pattern (e.g., `"*.go"`, `"internal/**"`) |
| `--sort <field>` | Sort by: `name` (default), `size`, `modified` |

**JSON output:**
```json
{
  "checkpoint_id": "cp-20260914-093500-b2c3d4e5",
  "files": [
    { "path": "cmd/server/main.go", "hash": "d4e5f6...", "size": 2340, "mode": "100644" },
    { "path": "go.mod", "hash": "a1b2c3...", "size": 156, "mode": "100644" }
  ],
  "total": 849
}
```

---

## `echo cat`

Show file contents.

```
echo cat <file-path> [--at <cp-id>]
```

| Flag | Description |
|------|-------------|
| `--at <cp-id>` | Show contents at a specific checkpoint (default: HEAD) |

Outputs raw file contents to stdout. With `--json`, outputs:

```json
{
  "path": "internal/auth/handler.go",
  "checkpoint_id": "cp-20260914-093500-b2c3d4e5",
  "hash": "a1b2c3...",
  "size": 1234,
  "content": "package auth\n\nimport \"net/http\"\n..."
}
```

---

## `echo search`

Full-text search across tracked files.

```
echo search <query> [--at <cp-id>] [--pattern <glob>] [--limit <n>] [--context <lines>]
```

| Flag | Description |
|------|-------------|
| `--at <cp-id>` | Search at a specific checkpoint (default: HEAD) |
| `--pattern <glob>` | Filter files by glob before searching |
| `--limit <n>` | Maximum results (default: 50) |
| `--context <lines>` | Lines of context around matches (default: 2) |

**JSON output:**
```json
{
  "query": "func Login",
  "checkpoint_id": "cp-20260914-093500-b2c3d4e5",
  "matches": [
    {
      "path": "internal/auth/handler.go",
      "line": 15,
      "content": "func Login(w http.ResponseWriter, r *http.Request) {",
      "context_before": ["", "// Login handles user authentication"],
      "context_after": ["    email := r.FormValue(\"email\")", "    password := r.FormValue(\"password\")"]
    }
  ],
  "total_matches": 1
}
```

---

## `echo tree`

Show directory tree structure.

```
echo tree [--at <cp-id>] [--depth <n>] [--path <subdir>]
```

| Flag | Description |
|------|-------------|
| `--at <cp-id>` | Tree at a specific checkpoint |
| `--depth <n>` | Maximum depth (default: unlimited) |
| `--path <subdir>` | Show subtree rooted at this path |

**Human output:**
```
.
├── cmd/
│   └── server/
│       └── main.go (2.3 KB)
├── internal/
│   ├── auth/
│   │   ├── handler.go (1.2 KB)
│   │   └── middleware.go (890 B)
│   └── routes/
│       └── router.go (3.1 KB)
├── go.mod (156 B)
└── README.md (2.1 KB)

4 directories, 5 files, 9.7 KB total
```

---

## `echo revert`

Revert workspace to a previous checkpoint.

```
echo revert <cp-id> [--dry-run] [--no-checkpoint]
```

| Flag | Description |
|------|-------------|
| `--dry-run` | Show what would change without applying |
| `--no-checkpoint` | Don't create a revert checkpoint (dangerous, not recommended) |

**What it does:**
1. Loads the target checkpoint's Merkle tree.
2. Diffs against current working directory.
3. Restores all files to the target checkpoint's state (add, modify, delete).
4. Creates a new checkpoint recording the revert (unless `--no-checkpoint`).

**JSON output:**
```json
{
  "reverted_to": "cp-20260914-093000-a1b2c3d4",
  "revert_checkpoint": "cp-20260914-094500-d4e5f6a7",
  "changes_applied": {
    "restored": ["internal/auth/handler.go"],
    "deleted": ["internal/auth/jwt.go"],
    "modified": ["cmd/server/main.go"]
  }
}
```

---

## `echo branch`

Create a new branch.

```
echo branch <name> [--from <cp-id>]
```

| Flag | Description |
|------|-------------|
| `--from <cp-id>` | Branch from a specific checkpoint (default: HEAD) |

---

## `echo switch`

Switch to a different branch.

```
echo switch <name>
```

Updates working directory to match the branch's HEAD checkpoint.

---

## `echo branches`

List all branches.

```
echo branches
```

**Human output:**
```
* main          cp-20260914-093500-b2c3d4e5  (2 min ago)
  approach-b    cp-20260914-093800-f6e5d4c3  (1 min ago)
```

---

## `echo merge`

Merge a branch into the current branch.

```
echo merge <branch> [--strategy <ours|theirs>] [--dry-run]
```

| Flag | Description |
|------|-------------|
| `--strategy <s>` | Conflict resolution: `theirs` (default) or `ours` |
| `--dry-run` | Show what would change |

**JSON output:**
```json
{
  "merged": "approach-b",
  "into": "main",
  "merge_checkpoint": "cp-20260914-094000-c3d4e5f6",
  "parents": ["cp-20260914-093500-b2c3d4e5", "cp-20260914-093800-f6e5d4c3"],
  "strategy": "theirs",
  "conflicts": [
    {
      "path": "internal/routes/router.go",
      "resolution": "used theirs"
    }
  ],
  "changes": {
    "added": 1,
    "modified": 2,
    "deleted": 0
  }
}
```

---

## `echo verify`

Verify integrity of the object store.

```
echo verify [--fix]
```

| Flag | Description |
|------|-------------|
| `--fix` | Remove corrupted objects (they will be missing, not wrong) |

**JSON output:**
```json
{
  "total_objects": 1523,
  "verified": 1523,
  "corrupted": 0,
  "missing": 0,
  "duration_ms": 340
}
```

---

## `echo gc`

Garbage collect unreferenced objects.

```
echo gc [--dry-run]
```

Walks all checkpoint trees, identifies blobs and tree objects not referenced by any checkpoint, and removes them.

**JSON output:**
```json
{
  "objects_scanned": 1523,
  "objects_removed": 47,
  "bytes_freed": 234567,
  "duration_ms": 890
}
```

---

## `echo watch`

Continuously monitor the workspace for filesystem changes and automatically create atomic checkpoints.

```
echo watch [--debounce <ms>] [--agent <name>]
```

| Flag | Description | Default |
|------|-------------|---------|
| `--debounce <ms>` | Quiet duration in milliseconds after file activity before checkpointing | `1000` |
| `--agent <name>` | Agent identifier tag attached to auto-checkpoints | `"auto-watcher"` |

**What it does:**
1. Establishes recursive filesystem watchers on all workspace directories respecting `.echo/ignore`.
2. Groups bursts of file creations, edits, and deletions (e.g. from agent bulk writes) using debouncing.
3. Automatically triggers `AutoCheckpointIfDirty` and indexes the checkpoint into `index.db` once disk activity settles.
4. Auto-saves any existing uncommitted changes immediately upon startup.

**JSON output:**
Streamed JSON event on each automated checkpoint:
```json
{
  "event": "auto_checkpoint",
  "checkpoint_id": "cp-20260914-101500-a1b2c3d4",
  "tree_hash": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
  "changeset": {
    "added": ["src/service.py"],
    "modified": ["README.md"],
    "deleted": []
  },
  "stats": {
    "total_files": 12,
    "total_size_bytes": 10420,
    "blobs_reused": 10,
    "blobs_new": 2,
    "duration_ms": 11
  }
}
```

---

## Shorthand Checkpoint References

Throughout the CLI, checkpoint IDs can be referenced using shorthands:

| Shorthand | Meaning |
|-----------|---------|
| `HEAD` | Current branch's latest checkpoint |
| `HEAD~1` | Parent of HEAD |
| `HEAD~n` | n-th ancestor of HEAD |
| `main` | Latest checkpoint on branch `main` |
| `cp-2026` | Prefix match (if unambiguous) |
| `@{-1}` | Previous branch's HEAD |
