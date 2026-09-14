# Echo

**Fast, lightweight, content-verified codebase state tracker for AI agents.**

[![Go Version](https://img.shields.io/badge/go-1.24+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Pure Go](https://img.shields.io/badge/Pure%20Go-Zero%20CGo-blue)](#architecture--storage-model)

Echo is a dedicated codebase state tracker and history DAG designed specifically for autonomous AI agent workflows. It provides automatic checkpointing, Merkle tree content verification, content-addressable zlib-compressed blob storage, full-text indexing, and deterministic rollbacks — completely independent of Git.

---

## Motivation

Autonomous coding agents interact with codebases fundamentally differently than human developers. While humans incrementally stage changes and write prose commit messages, agents execute rapid bulk modifications spanning dozens of files simultaneously.

When an automated modification introduces regressions, traditional version control presents several friction points:
- Manual staging and commit ceremony disrupt autonomous agent execution loops.
- Standard diff utilities incur performance overhead on large-scale file modifications.
- Existing tools lack structured, machine-readable JSON outputs and integrated content indexing out of the box.

Echo addresses these limitations by providing an immutable state capture layer that operates alongside Git (or standalone) with sub-second checkpoint creation, Merkle-accelerated diffing, and single-command rollbacks.

---

## Installation

### Automated Installer

Echo provides automated installation scripts that detect the host operating system and CPU architecture, place the static binary in a standard binary path, and configure the user's `PATH` environment variable.

#### macOS and Linux
```bash
curl -fsSL https://raw.githubusercontent.com/rimraf-adi/echo/main/install.sh | sh
```
*Installs to `/usr/local/bin` (or `~/.local/bin`), configuring `~/.zshrc`, `~/.bashrc`, or `~/.config/fish/config.fish` automatically.*

#### Windows (PowerShell)
```powershell
irm https://raw.githubusercontent.com/rimraf-adi/echo/main/install.ps1 | iex
```
*Installs to `%LOCALAPPDATA%\echo\bin`, persists to the Windows Registry User `PATH`, and activates in the current session.*

### Via Go Toolchain
```bash
go install github.com/rimraf-adi/echo/cmd/echo@latest
```

### Build from Source
```bash
git clone https://github.com/rimraf-adi/echo.git
cd echo
make build
# Binary output is placed at bin/echo
```

---

## Key Capabilities

- **Sub-Second Checkpoints**: Captures full-codebase state in less than 15ms with atomic file operations and automatic deduplication.
- **Automated Checkpoints and File Watcher**: `echo watch` monitors the filesystem with configurable debouncing (default: 1000ms), committing agent write bursts into atomic checkpoints. Safety auto-checkpoints safeguard branch switches, reverts, and merges against uncommitted data loss.
- **Merkle Tree Directory Structure**: Merkle diffing prunes unchanged subtrees in O(changes) time without inspecting untouched directories.
- **Content-Addressable Storage**: SHA-256 blobs compressed with zlib (level 6). Files smaller than 64 bytes or incompressible binary streams are retained uncompressed.
- **Deterministic Rollbacks**: `echo revert <cp-id>` restores the working tree to any historical checkpoint state while keeping history append-only.
- **Branch Isolation and Three-Way Merge**: Independent branches for parallel tasks, featuring Lowest Common Ancestor (LCA) graph traversal and deterministic conflict resolution policies (`theirs` and `ours`).
- **Pure Go SQLite Index (FTS5)**: Embedded database powered by `modernc.org/sqlite` (no CGo required), providing full-text search across all tracked code.
- **Agent-First Programmatic Interface**: Every command supports `--json` (`-j`) for zero-parsing programmatic orchestration.

---

## Quick Start

```bash
# 1. Initialize tracking in a project directory
echo init

# 2. Inspect active branch, HEAD checkpoint, and modified files
echo status

# 3. Snapshot state manually with agent metadata
echo checkpoint --agent "claude" --task "implement auth endpoints" --tag "auth"

# 4. Alternatively, launch the automated watcher to capture changes in background
echo watch --debounce 1000 --agent "auto-worker"

# 5. Search file contents across the codebase via FTS5
echo search "def login" --context 2

# 6. List tracked files at the current or historical checkpoint
echo files --at HEAD

# 7. Render hierarchical directory structure
echo tree

# 8. Print file contents at a specific historical checkpoint
echo cat src/services/auth.py --at HEAD~1

# 9. Compute line-level unified diffs
echo diff HEAD~1 HEAD --stat

# 10. Create and switch to an isolated branch
echo branch feature-oauth
echo switch feature-oauth

# 11. Revert workspace to an earlier checkpoint state
echo revert HEAD~1

# 12. Merge a feature branch with a specified conflict strategy
echo merge feature-oauth --strategy theirs

# 13. Verify cryptographic store integrity and purge unreferenced objects
echo verify
echo gc
```

---

## Programmatic Usage (JSON Mode)

Echo commands accept `--json` (or `-j`) to produce structured output on `stdout`:

```bash
# Example: creating a checkpoint programmatically
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
# Example: full-text search
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
│   ├── config.json                # Workspace configuration and defaults
│   ├── HEAD                       # Current branch reference
│   ├── refs/                      # Branch pointers
│   │   ├── main                   # Latest checkpoint ID on main
│   │   └── feature-a              # Latest checkpoint ID on feature-a
│   ├── objects/                   # Sharded content-addressable storage
│   │   ├── ab/
│   │   │   └── cdef1234...        # 48-byte binary header + zlib payload
│   │   └── ...
│   ├── checkpoints/               # Immutable JSON checkpoint manifests
│   │   ├── cp-20260914-...json
│   │   └── ...
│   ├── index.db                   # Embedded SQLite index (WAL mode, FTS5)
│   ├── ignore                     # Workspace ignore patterns (default: .echo, .git)
│   └── lock                       # Atomic write lockfile with stale PID detection
└── ... (tracked project files)
```

---

## Command Reference

| Command | Description |
|---|---|
| `echo init` | Initialize Echo tracking within the target directory |
| `echo status` | Display active branch, HEAD checkpoint, and uncommitted modifications |
| `echo checkpoint` | Atomically capture the working directory with agent metadata |
| `echo log` | Display chronological checkpoint history DAG and changesets |
| `echo diff [a] [b]` | Compute line-level unified diffs with context hunks |
| `echo show <cp-id>` | Display metadata and file breakdowns for a specific checkpoint |
| `echo files` | List tracked files, sizes, modes, and cryptographic hashes |
| `echo cat <path>` | Output raw contents of a file at any historical checkpoint |
| `echo search <query>` | Execute full-text FTS5 search with matching lines and context |
| `echo tree` | Render directory hierarchy with file counts and storage metrics |
| `echo revert <cp-id>` | Rollback working directory to match any historical checkpoint state |
| `echo branch <name>` | Create a new isolated branch |
| `echo switch <name>` | Switch active branch and synchronize the working tree |
| `echo branches` | List all existing branches with active branch indicator |
| `echo merge <branch>` | Perform three-way merge with LCA detection and conflict resolution |
| `echo verify` | Cryptographically verify all stored objects against their SHA-256 digests |
| `echo gc` | Reclaim disk space by purging unreferenced dangling objects |
| `echo watch` | Monitor workspace continuously and auto-checkpoint on settled file activity |

---

## Documentation

Detailed architectural designs, formal specifications, and testing strategies are maintained in the [`docs/`](docs/) directory:

- [`01-prd.md`](docs/01-prd.md): Product Requirements Document
- [`02-architecture.md`](docs/02-architecture.md): Object Model, Binary Format, and SQLite Schema
- [`03-cli-spec.md`](docs/03-cli-spec.md): CLI Command Specifications and Flag Definitions
- [`04-project-structure.md`](docs/04-project-structure.md): Package Layout and Architecture Boundaries
- [`05-implementation-guide.md`](docs/05-implementation-guide.md): Step-by-Step Implementation Guide
- [`06-testing-strategy.md`](docs/06-testing-strategy.md): Test Matrix and Verification Methodology

---

## Development & Testing

Execute unit and integration tests with Go's race detector enabled:

```bash
make test
```

---

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.
