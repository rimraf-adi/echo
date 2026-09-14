# Echo

Fast, lightweight, content-verified codebase state tracker for AI agents.

Echo provides zero-friction automatic checkpointing, Merkle tree content verification, content-addressable zlib-compressed blob storage, DAG history traversal, and deterministic rollback — completely independent of git.

## Features

- **Agent-First Checkpoints**: Atomic snapshots capturing bulk writes in milliseconds.
- **Merkle Tree Directory Structure**: $O(\text{changes})$ accelerated diffing and instant subtree verification.
- **Content-Addressable Deduplication**: SHA-256 blobs compressed with zlib.
- **Pure Go**: Zero-CGo SQLite index using `modernc.org/sqlite` with FTS5 full-text search.
- **VCS Capabilities**: Branching, isolated branch switching, 3-way merge with LCA detection, and append-only rollback/revert.
- **Machine-Readable**: Full `--json` flag support across all commands for autonomous agent orchestration.

## Installation & Build

```bash
git clone https://github.com/rimraf-adi/echo.git
cd echo
make build
```

The compiled binary will be placed at `bin/echo`.

## Quick Start

```bash
# Initialize Echo tracking in a project
echo init

# Check status
echo status

# Capture a checkpoint
echo checkpoint --agent "claude" --task "add user auth" --tag "api"

# Query the codebase
echo files
echo search "func Login"
echo tree
echo cat internal/auth/handler.go

# Branch & Switch
echo branch experiment-a
echo switch experiment-a

# Review changes & History
echo log
echo diff HEAD~1 HEAD

# Revert back to any checkpoint safely
echo revert HEAD~1

# Merge branches
echo merge experiment-a

# Integrity check & GC
echo verify
echo gc
```

## Running Tests

```bash
make test
```

## Documentation

Comprehensive design specifications and architecture guides are available in [`docs/`](docs/):
- `01-prd.md`: Product Requirements Document
- `02-architecture.md`: Architecture, Object Model & Binary Format
- `03-cli-spec.md`: Full CLI Specification
- `04-project-structure.md`: Go Project Structure
- `05-implementation-guide.md`: Implementation Guide
- `06-testing-strategy.md`: Test Scenarios & Strategy

## License

MIT
