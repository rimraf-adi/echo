# Echo 🏛️

**Fast, lightweight, content-verified codebase state tracker for AI agents.**

[![Go Version](https://img.shields.io/badge/go-1.24+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Pure Go](https://img.shields.io/badge/Pure%20Go-Zero%20CGo-blue)](#architecture)

Echo is a dedicated codebase state machine and history DAG purpose-built for AI agent workflows. It provides zero-friction automatic checkpointing, Merkle tree content verification, zlib-compressed blob deduplication, and deterministic rollback — completely independent of git.

---

## Why Echo?

Agents do not use version control the way humans do. While humans stage individual hunks and write prose commit messages, **agents bulk-write 10 to 50 files simultaneously**. 

When an agent breaks code in a bulk mutation, recovery with traditional tools is painful:
- Git requires manual staging, commit ceremony, and human conflict resolution.
- Git diffs become slow and noisy on massive multi-file modifications.
- Standard tools lack structured JSON outputs and indexed full-text content querying out of the box.

Echo solves this by acting as an immutable state capture layer running alongside git (or standalone) with sub-second checkpointing, Merkle-accelerated diffing, and true one-command rollbacks.

---

## ⚡ 1-Line Installation

Echo automatically detects your OS and CPU architecture, installs the static binary, and permanently configures your `PATH` environment variable.

### macOS & Linux
```bash
curl -fsSL https://raw.githubusercontent.com/rimraf-adi/echo/main/install.sh | sh
```
*Installs to `/usr/local/bin` (if writable) or `~/.local/bin`, and automatically adds it to `~/.zshrc`, `~/.bashrc`, or `~/.config/fish/config.fish`.*

### Windows (PowerShell)
```powershell
irm https://raw.githubusercontent.com/rimraf-adi/echo/main/install.ps1 | iex
```
*Installs to `%LOCALAPPDATA%\echo\bin`, permanently registers it in the Windows Registry User `PATH`, and makes it available immediately in the current session.*

### Via Go
```bash
go install github.com/rimraf-adi/echo/cmd/echo@latest
```

### Build from Source
```bash
git clone https://github.com/rimraf-adi/echo.git
cd echo
make build
# Binary is built at bin/echo
```

---

## Key Capabilities

- ⚡ **Sub-Second Checkpoints**: Captures full-codebase state in $< 15\text{ms}$ with atomic file writes and automatic duplicate pruning.
- 👁 **Automated Checkpoints & File Watcher**: Never lose uncommitted work. `echo watch` monitors the filesystem with configurable debouncing (default 1s), auto-saving agent write bursts into atomic checkpoints. Safety auto-checkpoints also guard branch switches, reverts, and merges.
- 🌳 **Merkle Tree Directory Structure**: $O(\text{changes})$ diff algorithm prunes unchanged subtrees instantly without walking untouched files.
- 📦 **Content-Addressable Storage**: SHA-256 blobs compressed with zlib level 6. Files smaller than 64 bytes or incompressible binaries are automatically kept raw.
- 🔄 **Deterministic Rollbacks**: `echo revert <cp-id>` restores the exact file tree while keeping history append-only.
- 🌿 **Branch Isolation & 3-Way Merge**: Isolated branches for agents exploring alternative approaches, with Lowest Common Ancestor (LCA) detection and automated conflict resolution (`theirs` / `ours`).
- 🔍 **Pure Go SQLite Index & FTS5**: Built with `modernc.org/sqlite` (no CGo compiler required) with indexed full-text search across all tracked code.
- 🤖 **Agent First (`--json`)**: Every CLI command supports `--json` (`-j`) for zero-parsing programmatic orchestration.

---

## Quick Start Walkthrough

```bash
# 1. Initialize tracking in any project
echo init

# 2. Check current branch, HEAD, and uncommitted changes
echo status

# 3. Capture an agent bulk write with rich metadata
echo checkpoint --agent "claude" --task "implement auth endpoints" --tag "auth" --tag "v1"

# 4. Or let Echo watch and auto-checkpoint whenever files change
echo watch --debounce 1000 --agent "auto-agent"

# 5. Search file contents instantly across the codebase
echo search "def login" --context 2

# 6. List all tracked files at any checkpoint
echo files --at HEAD

# 7. View the visual directory tree
echo tree

# 8. Print content of any file at any historical checkpoint
echo cat src/services/auth.py --at HEAD~1

# 9. Inspect unified Myers line diffs
echo diff HEAD~1 HEAD --stat

# 10. Try an alternative approach in a branch
echo branch experiment-oauth
echo switch experiment-oauth

# 11. Revert bad changes instantly
echo revert HEAD

# 12. Merge parallel branches
echo merge experiment-oauth --strategy theirs

# 13. Verify store integrity & clean unreferenced objects
echo verify
echo gc
```

---

## Machine-Readable JSON Mode

Every command outputs structured JSON when invoked with `-j` or `--json`:

```bash
# Example: programmatic checkpoint
echo checkpoint --agent "agent-1" --task "add routes" --json
```

```json
{
  "checkpoint_id": "cp-20260914-043347-09f2f4a2",
  "parents": ["cp-20260914-043250-eed5e8b8"],
  "tree_hash": "173fdb6fe01894fa65baedc2fe20e5ce915ea456a1b0ed397a19b5a6cac8a28b",
  "changeset": {
    "added": ["src/api/routes.py", "src/models/user.py"],
    "modified": ["main.py"],
    "deleted": []
  },
  "stats": {
    "total_files": 15,
    "total_size_bytes": 45120,
    "blobs_reused": 12,
    "blobs_new": 3,
    "duration_ms": 8
  }
}
```

```bash
# Example: full-text search output for agents
echo search "def register" --json
```

```json
{
  "query": "def register",
  "checkpoint_id": "cp-20260914-043347-09f2f4a2",
  "matches": [
    {
      "path": "src/services/auth.py",
      "line_number": 10,
      "content": "    def register(self, username: str, email: str):",
      "context_before": ["class AuthService:", "    def __init__(self): ..."],
      "context_after": ["        user_id = f'usr_{len(self._users) + 1}'"]
    }
  ],
  "total_matches": 1
}
```

---

## Architecture & Storage Model

```
project/
├── .echo/
│   ├── config.json                # Workspace configuration
│   ├── HEAD                       # Current branch reference
│   ├── refs/                      # Branch pointers
│   │   ├── main                   # -> checkpoint ID
│   │   └── experiment-a           # -> checkpoint ID
│   ├── objects/                   # 2-character sharded object store
│   │   ├── ab/
│   │   │   └── cdef1234...        # 48-byte header + zlib compressed payload
│   │   └── ...
│   ├── checkpoints/               # Immutable JSON checkpoint manifests
│   │   ├── cp-20260914-...json
│   │   └── ...
│   ├── index.db                   # Pure Go SQLite database (WAL mode, FTS5)
│   ├── ignore                     # Custom ignore rules (default: .echo, .git)
│   └── lock                       # Atomic write lockfile with stale PID detection
└── ... (tracked project files)
```

---

## Command Reference

| Command | Description |
|---|---|
| `echo init` | Initialize Echo tracking in the directory |
| `echo status` | Display branch, HEAD checkpoint, and uncheckpointed deltas |
| `echo checkpoint` | Atomically snapshot working directory with agent metadata |
| `echo log` | Show chronological history DAG and changesets |
| `echo diff [a] [b]` | Compute line-level unified diffs with hunks |
| `echo show <cp-id>` | Display detailed metadata and file breakdown for a checkpoint |
| `echo files` | List tracked files, file sizes, and modes |
| `echo cat <path>` | Output raw content of any file at any historical state |
| `echo search <query>` | FTS5 full-text search with line numbers and context |
| `echo tree` | Render directory hierarchy with file counts and byte sizes |
| `echo revert <cp-id>` | Rollback files to match any historical checkpoint state |
| `echo branch <name>` | Create a new isolated branch |
| `echo switch <name>` | Switch active branch and update working directory |
| `echo branches` | List all branches and active branch indicator |
| `echo merge <branch>` | Perform 3-way merge with LCA detection and conflict policies |
| `echo verify` | Cryptographically verify all blobs and trees against SHA-256 |
| `echo gc` | Reclaim storage by purging unreferenced dangling objects |
| `echo watch` | Watch workspace and automatically checkpoint when files change |

---

## Comprehensive Specifications

Detailed architectural designs, formal PRDs, and testing documents are located in [`docs/`](docs/):
- [`01-prd.md`](docs/01-prd.md): Product Requirements Document
- [`02-architecture.md`](docs/02-architecture.md): Object Model, Binary Format, & SQLite Schema
- [`03-cli-spec.md`](docs/03-cli-spec.md): CLI Command Specifications & Flags
- [`04-project-structure.md`](docs/04-project-structure.md): Package Layout & Responsibilities
- [`05-implementation-guide.md`](docs/05-implementation-guide.md): Step-by-step phased engineering guide
- [`06-testing-strategy.md`](docs/06-testing-strategy.md): Test Matrix & Validation Strategy

---

## Contributing & Testing

Run all unit and integration tests with Go's race detector:

```bash
make test
```

## License

[MIT](LICENSE)
