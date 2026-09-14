# Echo — Go Project Structure

## Module & Directory Layout

```
echo/
├── go.mod
├── go.sum
├── Makefile
├── README.md
├── .echo-ignore                     # Default ignore patterns shipped with binary
├── docs/                            # These spec documents
│
├── cmd/
│   └── echo/
│       └── main.go                  # Entry point: parse args, dispatch to commands
│
├── internal/
│   ├── cli/                         # CLI command handlers
│   │   ├── root.go                  # Root command setup, global flags
│   │   ├── init.go                  # echo init
│   │   ├── checkpoint.go            # echo checkpoint
│   │   ├── status.go                # echo status
│   │   ├── log.go                   # echo log
│   │   ├── diff.go                  # echo diff
│   │   ├── show.go                  # echo show
│   │   ├── files.go                 # echo files
│   │   ├── cat.go                   # echo cat
│   │   ├── search.go               # echo search
│   │   ├── tree.go                  # echo tree
│   │   ├── revert.go               # echo revert
│   │   ├── branch.go               # echo branch
│   │   ├── switch.go               # echo switch
│   │   ├── branches.go             # echo branches
│   │   ├── merge.go                # echo merge
│   │   ├── verify.go               # echo verify
│   │   └── gc.go                   # echo gc
│   │
│   ├── core/                        # Core business logic (no I/O dependencies)
│   │   ├── workspace.go             # Workspace initialization and management
│   │   ├── checkpoint.go            # Checkpoint creation, loading, validation
│   │   ├── changeset.go             # Changeset computation (detect added/modified/deleted)
│   │   ├── diff.go                  # Line-level diff algorithm (Myers diff)
│   │   ├── merge.go                 # Merge logic and conflict resolution
│   │   ├── revert.go               # Revert logic
│   │   └── refs.go                 # Branch ref management, HEAD resolution
│   │
│   ├── merkle/                      # Merkle tree implementation
│   │   ├── tree.go                  # Tree node type, construction, serialization
│   │   ├── blob.go                  # Blob type, hashing
│   │   ├── builder.go              # Build tree from filesystem scan
│   │   ├── diff.go                 # Merkle-accelerated diff (walk two trees)
│   │   ├── walker.go               # Tree traversal utilities
│   │   └── serialize.go            # Canonical serialization for hashing
│   │
│   ├── store/                       # Object storage layer
│   │   ├── object_store.go         # Read/write objects to .echo/objects/
│   │   ├── compress.go             # zlib compression/decompression
│   │   ├── header.go               # Binary header encoding/decoding (48-byte format)
│   │   └── hash.go                 # SHA-256 hashing utilities
│   │
│   ├── index/                       # SQLite index layer
│   │   ├── db.go                   # Database connection, migrations, WAL setup
│   │   ├── files.go                # File index queries
│   │   ├── checkpoints.go         # Checkpoint index queries
│   │   ├── search.go              # FTS5 full-text search
│   │   ├── refs.go                # Ref queries
│   │   └── rebuild.go             # Full index rebuild from object store
│   │
│   ├── ignore/                      # Ignore pattern matching
│   │   ├── matcher.go              # Glob/pattern matching engine
│   │   └── defaults.go            # Default ignore patterns
│   │
│   ├── scan/                        # Filesystem scanning
│   │   ├── scanner.go              # Walk directory, respect ignores, compute hashes
│   │   └── parallel.go            # Parallel file hashing for large codebases
│   │
│   ├── lock/                        # Write locking
│   │   └── lockfile.go             # Exclusive lockfile with stale detection
│   │
│   └── output/                      # Output formatting
│       ├── json.go                 # JSON output helpers
│       ├── human.go                # Human-readable formatters
│       └── table.go               # Table formatting for terminal
│
└── tests/
    ├── integration/                 # End-to-end CLI tests
    │   ├── init_test.go
    │   ├── checkpoint_test.go
    │   ├── diff_test.go
    │   ├── revert_test.go
    │   ├── branch_test.go
    │   ├── merge_test.go
    │   └── helpers_test.go          # Test utilities (temp dirs, fixture creation)
    │
    └── fixtures/                    # Test fixture files
        ├── small_project/           # ~10 files
        └── medium_project/          # ~1000 files
```

---

## Dependency List

Minimal external dependencies. Prefer stdlib where possible.

| Dependency | Purpose | Why not stdlib |
|------------|---------|----------------|
| `github.com/spf13/cobra` | CLI framework | Subcommand routing, flag parsing, help generation |
| `github.com/mattn/go-sqlite3` | SQLite driver | CGo-based, production-proven SQLite binding |
| `github.com/sergi/go-diff` | Myers diff | Line-level unified diff generation |

**Stdlib packages used heavily:**
- `compress/zlib` — blob compression
- `crypto/sha256` — content hashing
- `database/sql` — SQLite interface
- `encoding/binary` — object header encoding
- `encoding/json` — checkpoint serialization, JSON output
- `io/fs` — filesystem walking
- `os` — file I/O
- `path/filepath` — path manipulation
- `sort` — deterministic tree entry ordering
- `sync` — concurrent file hashing

### CGo Note

`go-sqlite3` requires CGo. For fully static builds:
- Linux: `CGO_ENABLED=1 CC=musl-gcc go build -ldflags '-linkmode external -extldflags -static'`
- macOS: CGo works natively with system SQLite
- For CGo-free alternative: consider `modernc.org/sqlite` (pure Go SQLite) — slower but zero CGo dependency

---

## Build & Cross-Compile

### Makefile targets

```makefile
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

.PHONY: build test clean

build:
	go build $(LDFLAGS) -o bin/echo ./cmd/echo

test:
	go test ./... -v -race -count=1

test-integration:
	go test ./tests/integration/... -v -race -count=1 -tags=integration

lint:
	golangci-lint run ./...

clean:
	rm -rf bin/

# Cross-compilation
build-all: build-linux-amd64 build-linux-arm64 build-darwin-amd64 build-darwin-arm64 build-windows-amd64

build-linux-amd64:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=1 go build $(LDFLAGS) -o bin/echo-linux-amd64 ./cmd/echo

build-linux-arm64:
	GOOS=linux GOARCH=arm64 CGO_ENABLED=1 CC=aarch64-linux-gnu-gcc go build $(LDFLAGS) -o bin/echo-linux-arm64 ./cmd/echo

build-darwin-amd64:
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=1 go build $(LDFLAGS) -o bin/echo-darwin-amd64 ./cmd/echo

build-darwin-arm64:
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=1 go build $(LDFLAGS) -o bin/echo-darwin-arm64 ./cmd/echo

build-windows-amd64:
	GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc go build $(LDFLAGS) -o bin/echo-windows-amd64.exe ./cmd/echo
```

---

## Package Responsibilities

### `cmd/echo` — Entry Point
- Parses CLI arguments via Cobra
- Sets up global flags (`--json`, `--quiet`, `--verbose`, `--dir`)
- Dispatches to the appropriate command handler in `internal/cli/`
- Handles top-level error formatting and exit codes

### `internal/cli/` — Command Handlers
- Each file corresponds to one CLI command
- Responsible for: flag parsing, input validation, calling core logic, formatting output
- No business logic here — delegates to `internal/core/`
- Handles both JSON and human-readable output modes

### `internal/core/` — Business Logic
- Pure logic, no CLI concerns
- `workspace.go`: Initialize workspace, locate `.echo/` directory, load config
- `checkpoint.go`: Create checkpoints (orchestrates scanner → merkle builder → store → index)
- `changeset.go`: Compare two Merkle trees to produce a changeset
- `diff.go`: Line-level diff between two file contents
- `merge.go`: Three-way merge with strategy selection
- `revert.go`: Apply a checkpoint's state to the working directory
- `refs.go`: Resolve HEAD, branch names, shorthand references to checkpoint IDs

### `internal/merkle/` — Merkle Tree
- `tree.go`: Tree node struct, child management
- `blob.go`: Blob node struct, content hashing
- `builder.go`: Given a filesystem scan result, build the full Merkle tree bottom-up
- `diff.go`: Walk two Merkle trees simultaneously, find differences in O(changes)
- `walker.go`: Pre-order and post-order tree traversal
- `serialize.go`: Canonical serialization format for deterministic hashing

### `internal/store/` — Object Storage
- `object_store.go`: Write/read objects to `.echo/objects/<xx>/<hash>`. Handles temp-write + rename atomicity.
- `compress.go`: zlib compress/decompress with configurable level. Skip compression for tiny or incompressible files.
- `header.go`: Encode/decode the 48-byte binary header (magic, type, flags, sizes, hash).
- `hash.go`: SHA-256 wrapper, streaming hash computation for large files.

### `internal/index/` — SQLite Index
- `db.go`: Open/create database, run migrations, configure WAL mode and busy timeout.
- `files.go`: Insert/query file records per checkpoint.
- `checkpoints.go`: Insert/query checkpoint metadata.
- `search.go`: FTS5 full-text search operations.
- `refs.go`: Branch ref CRUD.
- `rebuild.go`: Full index rebuild by walking all checkpoints and their trees.

### `internal/scan/` — Filesystem Scanner
- `scanner.go`: Walk directory tree, respect ignore patterns, collect file paths + stats.
- `parallel.go`: Hash files in parallel using a worker pool (runtime.NumCPU workers). Returns map of path → hash.

### `internal/ignore/` — Ignore Patterns
- `matcher.go`: Parse `.echo/ignore` file, compile patterns, match paths. Supports globs, directory markers, negation.
- `defaults.go`: Built-in default ignores (`.echo/`, `.git/`).

### `internal/lock/` — Write Locking
- `lockfile.go`: Create/release exclusive lockfile. Detect and clean stale locks (check PID liveness). Configurable timeout for lock acquisition.

### `internal/output/` — Output Formatting
- `json.go`: Marshal structs to JSON with consistent formatting.
- `human.go`: Color-coded, human-readable output (detect TTY, respect `NO_COLOR`).
- `table.go`: Aligned table output for lists.
