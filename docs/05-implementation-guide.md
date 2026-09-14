# Echo — Implementation Guide

## Build Phases

This document breaks the implementation into sequential phases. Each phase produces a working, testable increment. An agent should complete one phase fully (including tests) before starting the next.

---

## Phase 1: Foundation — Object Store & Blob Storage

**Goal:** Store and retrieve content-addressable, zlib-compressed blobs.

**Why first:** Everything else depends on being able to write and read objects. Get this right and the rest is plumbing.

### Files to Implement

1. **`internal/store/hash.go`**
   - Implement `HashBytes(data []byte) string` — returns hex-encoded SHA-256.
   - Implement `HashReader(r io.Reader) (string, error)` — streaming hash for large files.
   - Hash is always computed on **raw (uncompressed)** content.

2. **`internal/store/compress.go`**
   - Implement `Compress(data []byte, level int) ([]byte, error)` — zlib compress.
   - Implement `Decompress(data []byte) ([]byte, error)` — zlib decompress.
   - If compressed size ≥ 95% of original size, return original data and set uncompressed flag.
   - If original size < 64 bytes, skip compression.

3. **`internal/store/header.go`**
   - Define the 48-byte binary header struct.
   - Implement `EncodeHeader(h Header) []byte`.
   - Implement `DecodeHeader(data []byte) (Header, error)`.
   - Validate magic number (`ECHO` = `0x4543484F`), version, and hash on decode.
   - Header fields:
     ```
     Magic [4]byte     // "ECHO"
     Version uint8     // 0x01
     ObjType uint8     // 0x01=blob, 0x02=tree
     Flags   uint8     // bit 0: compressed
     Reserved uint8
     OrigSize uint32   // big-endian
     StoredSize uint32 // big-endian
     Hash [32]byte     // SHA-256 of original content
     ```

4. **`internal/store/object_store.go`**
   - Define `ObjectStore` struct with base path (`.echo/objects/`).
   - Implement `NewObjectStore(basePath string) *ObjectStore`.
   - Implement `WriteBlob(content []byte) (hash string, err error)`:
     1. Compute SHA-256 of raw content.
     2. Check if object already exists (by hash path) — if yes, return hash (dedup).
     3. Compress content (respecting skip rules).
     4. Encode header + compressed data.
     5. Write to temp file in `.echo/objects/tmp-<random>`.
     6. Rename to `.echo/objects/<hash[0:2]>/<hash[2:]>`.
   - Implement `ReadBlob(hash string) ([]byte, error)`:
     1. Read file from `.echo/objects/<hash[0:2]>/<hash[2:]>`.
     2. Decode header, validate magic and hash.
     3. Decompress if flag set.
     4. Verify SHA-256 of result matches header hash.
     5. Return raw content.
   - Implement `HasObject(hash string) bool`.
   - Implement `DeleteObject(hash string) error`.
   - Implement `ListObjects() ([]string, error)` — walk objects dir, return all hashes.

### Tests for Phase 1

```
tests/integration/store_test.go
```

- Write a blob, read it back, verify content matches.
- Write the same content twice, verify only one object file exists (dedup).
- Write a tiny file (< 64 bytes), verify it's stored uncompressed.
- Write an incompressible file, verify stored uncompressed.
- Corrupt an object file, verify `ReadBlob` returns an integrity error.
- Write and read 1000 blobs, verify all round-trip correctly.
- Verify header encoding/decoding round-trips for all field combinations.

### Acceptance Criteria
- [x] All blob operations work correctly
- [x] Deduplication verified
- [x] Compression works and is skippable
- [x] Integrity checking catches corruption
- [x] All tests pass with `-race` flag

---

## Phase 2: Merkle Tree

**Goal:** Build, serialize, hash, and diff Merkle trees representing directory structures.

### Files to Implement

1. **`internal/merkle/blob.go`**
   - Define `BlobNode` struct: `Hash string`, `Size int64`, `Mode os.FileMode`.
   - Constructor that takes content hash, size, and mode.

2. **`internal/merkle/tree.go`**
   - Define `TreeNode` struct:
     ```go
     type TreeNode struct {
         Hash     string
         Entries  []TreeEntry  // sorted by Name
     }
     type TreeEntry struct {
         Name string
         Mode string       // "100644", "100755", "040000", "120000"
         Hash string        // blob hash or child tree hash
         Type EntryType     // Blob or Tree
     }
     ```
   - Implement `AddEntry(entry TreeEntry)` — inserts in sorted order.
   - Implement `ComputeHash()` — canonical serialization → SHA-256.

3. **`internal/merkle/serialize.go`**
   - Implement `SerializeTree(node *TreeNode) []byte` — canonical format:
     ```
     <mode> <hash> <name>\n
     ```
     Entries sorted lexicographically by name. This byte sequence is what gets hashed.
   - Implement `DeserializeTree(data []byte) (*TreeNode, error)`.

4. **`internal/merkle/builder.go`**
   - Implement `BuildTree(scanResult *ScanResult, store *ObjectStore) (*TreeNode, error)`:
     1. Take a flat map of `path → content hash` from the scanner.
     2. Group by directory.
     3. Build leaf tree nodes (directories containing only files).
     4. Build upward — each directory's hash depends on its children's hashes.
     5. Store each tree node as a tree object in the object store.
     6. Return the root `TreeNode`.
   - Tree objects are stored with type `0x02` (tree) in the same object store.

5. **`internal/merkle/diff.go`**
   - Implement `DiffTrees(a, b *TreeNode, store *ObjectStore) ([]Delta, error)`:
     ```go
     type DeltaType int
     const (
         DeltaAdded DeltaType = iota
         DeltaModified
         DeltaDeleted
     )
     type Delta struct {
         Path    string
         Type    DeltaType
         OldHash string  // empty for Added
         NewHash string  // empty for Deleted
     }
     ```
   - Algorithm:
     1. If `a.Hash == b.Hash`, return empty (subtree identical).
     2. Build maps of entries by name for both trees.
     3. For each name in the union:
        - Only in b → Added (recurse if tree to list all files).
        - Only in a → Deleted (recurse if tree to list all files).
        - In both but different hash → if both trees, recurse. If blobs, Modified.
        - In both, same hash → skip.

6. **`internal/merkle/walker.go`**
   - Implement `Walk(root *TreeNode, store *ObjectStore, fn WalkFunc) error` — pre-order traversal.
   - `WalkFunc = func(path string, entry TreeEntry, depth int) error`.
   - Support early termination (return sentinel error to stop).

### Tests for Phase 2

```
tests/integration/merkle_test.go
```

- Build a tree from a known directory structure, verify root hash is deterministic.
- Build the same tree twice, verify same root hash.
- Change one file, rebuild tree, verify root hash changes.
- Change one file, verify only the leaf blob and ancestor tree nodes have new hashes (unchanged subtrees keep same hash).
- Diff two identical trees → empty diff.
- Diff trees with one added file → correct delta.
- Diff trees with one modified file → correct delta.
- Diff trees with one deleted file → correct delta.
- Diff trees with changes in nested subdirectories → correct deltas with full paths.
- Serialize/deserialize tree round-trip.
- Walk tree, verify all entries visited in correct order.

### Acceptance Criteria
- [x] Merkle tree builds correctly from flat file list
- [x] Root hash is deterministic (same content → same hash)
- [x] Diff only traverses changed subtrees
- [x] All tree objects stored in object store
- [x] Tests pass with `-race`

---

## Phase 3: Filesystem Scanner & Ignore Patterns

**Goal:** Scan a directory tree efficiently, respect ignore patterns, hash files in parallel.

### Files to Implement

1. **`internal/ignore/defaults.go`**
   - Define `DefaultPatterns` — always ignored: `.echo/`, `.git/`.

2. **`internal/ignore/matcher.go`**
   - Implement `Matcher` struct.
   - Implement `LoadIgnoreFile(path string) (*Matcher, error)` — parse `.echo/ignore`.
   - Implement `Match(path string, isDir bool) bool` — returns true if path should be ignored.
   - Support: glob patterns, directory patterns (trailing `/`), negation (`!`), comments (`#`).
   - Pattern matching uses `filepath.Match` semantics plus `**` for recursive matching.

3. **`internal/scan/scanner.go`**
   - Implement `Scanner` struct with ignore matcher.
   - Implement `Scan(rootDir string) (*ScanResult, error)`:
     ```go
     type ScanResult struct {
         Files map[string]FileInfo  // relative path → info
         Root  string
     }
     type FileInfo struct {
         Path    string
         Size    int64
         Mode    os.FileMode
         ModTime time.Time
         Hash    string         // filled by parallel hasher
     }
     ```
   - Walk directory using `filepath.WalkDir` (uses `fs.WalkDirFunc`, doesn't stat twice).
   - Skip ignored paths immediately (don't descend into ignored directories).
   - Collect all non-ignored file paths.

4. **`internal/scan/parallel.go`**
   - Implement `HashFiles(files map[string]FileInfo, rootDir string, numWorkers int) error`:
   - Fan out file hashing to `numWorkers` goroutines (default: `runtime.NumCPU()`).
   - Each worker: read file → SHA-256 → store hash in `FileInfo.Hash`.
   - Use buffered channel as work queue, `sync.WaitGroup` for completion.
   - Handle errors: if any file fails to hash, return error with file path context.

### Tests for Phase 3

```
tests/integration/scan_test.go
tests/integration/ignore_test.go
```

- Scan a directory, verify all files found.
- Scan with `.echo/ignore`, verify ignored files excluded.
- Verify `.echo/` and `.git/` are always ignored.
- Negation patterns work (`!important.log` overrides `*.log`).
- Directory patterns work (`node_modules/` ignores the entire directory).
- Parallel hashing produces correct hashes for all files.
- Parallel hashing with 1 worker produces same results as with 8 workers.
- Scan empty directory → empty result (not error).
- Scan with symlinks → symlinks stored as link targets.

### Acceptance Criteria
- [x] Scanner finds all non-ignored files
- [x] Ignore patterns match gitignore behavior
- [x] Parallel hashing is correct and race-free
- [x] Tests pass with `-race`

---

## Phase 4: Checkpointing & Core Workspace Operations

**Goal:** Initialize workspaces, create checkpoints, manage HEAD and refs.

### Files to Implement

1. **`internal/lock/lockfile.go`**
   - Implement `Acquire(lockPath string, timeout time.Duration) (*Lock, error)`.
   - Implement `Release(lock *Lock) error`.
   - Create lock file with `O_CREATE|O_EXCL` for atomicity.
   - Write PID to lock file for stale detection.
   - On failure to acquire: check if PID in lockfile is alive. If dead, remove and retry.

2. **`internal/core/workspace.go`**
   - Implement `Init(rootDir string) (*Workspace, error)`:
     1. Create `.echo/` directory structure.
     2. Write `config.json` with defaults.
     3. Create `HEAD` file pointing to `main`.
     4. Run full scan → build Merkle tree → store objects.
     5. Create initial checkpoint.
     6. Create `main` ref pointing to initial checkpoint.
     7. Initialize `index.db`.
   - Implement `Open(dir string) (*Workspace, error)` — find `.echo/` (walk up from dir), load config, open store and index.
   - Implement `FindRoot(dir string) (string, error)` — walk up directory tree looking for `.echo/`.

3. **`internal/core/checkpoint.go`**
   - Implement `CreateCheckpoint(ws *Workspace, opts CheckpointOpts) (*Checkpoint, error)`:
     ```go
     type CheckpointOpts struct {
         Agent   string
         Task    string
         Model   string
         Tags    []string
         Meta    map[string]string
         Message string
     }
     ```
     1. Acquire write lock.
     2. Scan workspace for current state.
     3. Load current HEAD checkpoint's Merkle tree.
     4. Diff current scan against HEAD tree to detect changes.
     5. If no changes → return nil (nothing to checkpoint).
     6. Store new/changed blobs.
     7. Build new Merkle tree (reuse unchanged subtree hashes).
     8. Generate checkpoint ID: `cp-<YYYYMMDD>-<HHMMSS>-<8-hex>`.
     9. Write checkpoint JSON to `.echo/checkpoints/<id>.json`.
     10. Update branch ref.
     11. Update index.db.
     12. Release lock.
   - Implement `LoadCheckpoint(ws *Workspace, id string) (*Checkpoint, error)`.
   - Implement `ListCheckpoints(ws *Workspace, filters CheckpointFilters) ([]*Checkpoint, error)`.

4. **`internal/core/refs.go`**
   - Implement `ResolveRef(ws *Workspace, ref string) (string, error)`:
     - `HEAD` → read branch name from HEAD file → read checkpoint ID from ref file.
     - Branch name → read checkpoint ID from `.echo/refs/<name>`.
     - `HEAD~n` → resolve HEAD, then walk n parents.
     - Checkpoint ID prefix → scan checkpoints for unique prefix match.
   - Implement `UpdateRef(ws *Workspace, name string, checkpointID string) error`.
   - Implement `ReadHead(ws *Workspace) (string, error)` — returns current branch name.

5. **`internal/core/changeset.go`**
   - Implement `ComputeChangeset(oldTree, newTree *TreeNode, store *ObjectStore) (*Changeset, error)`:
     - Uses `merkle.DiffTrees` to get deltas.
     - Groups deltas into added/modified/deleted lists.
     - Returns structured changeset for checkpoint JSON.

### Tests for Phase 4

```
tests/integration/workspace_test.go
tests/integration/checkpoint_test.go
```

- `Init` creates all expected directory structure and files.
- `Init` on a project with files creates initial checkpoint with correct file count.
- `Open` finds `.echo/` when run from a subdirectory.
- `CreateCheckpoint` with no changes returns nil.
- `CreateCheckpoint` after adding a file records it as added.
- `CreateCheckpoint` after modifying a file records it as modified.
- `CreateCheckpoint` after deleting a file records it as deleted.
- Checkpoint metadata is correctly stored and loadable.
- Ref resolution works for HEAD, branch names, `HEAD~n`, and prefixes.
- Two checkpoints in sequence have correct parent chain.
- Lock prevents concurrent checkpoint creation.

### Acceptance Criteria
- [x] Init creates valid workspace
- [x] Checkpoints are atomic and correct
- [x] Changesets accurately reflect filesystem changes
- [x] Ref resolution handles all shorthand formats
- [x] Tests pass with `-race`

---

## Phase 5: SQLite Index & Query Layer

**Goal:** Fast querying of files, checkpoints, and full-text search via SQLite.

### Files to Implement

1. **`internal/index/db.go`**
   - Implement `OpenIndex(path string) (*Index, error)`:
     - Open SQLite database with WAL mode, busy timeout 5s.
     - Run migrations (create tables if not exist).
   - Implement `Close()`.
   - Schema as defined in `02-architecture.md`.

2. **`internal/index/files.go`**
   - Implement `IndexFiles(checkpointID string, files map[string]FileInfo) error` — batch insert within transaction.
   - Implement `QueryFiles(checkpointID string, pattern string) ([]FileRecord, error)`.
   - Implement `GetFile(checkpointID string, path string) (*FileRecord, error)`.

3. **`internal/index/checkpoints.go`**
   - Implement `IndexCheckpoint(cp *Checkpoint) error`.
   - Implement `QueryCheckpoints(filters CheckpointFilters) ([]*CheckpointRecord, error)`.
   - Filtering by: agent, task, date range, tags.

4. **`internal/index/search.go`**
   - Implement `IndexContent(checkpointID string, path string, content string) error`.
   - Implement `Search(checkpointID string, query string, limit int) ([]SearchResult, error)`.
   - Uses FTS5 `MATCH` for full-text search.
   - Index only the HEAD checkpoint's content by default (not all historical states).

5. **`internal/index/refs.go`**
   - Implement `SetRef(name string, checkpointID string) error`.
   - Implement `GetRef(name string) (string, error)`.
   - Implement `ListRefs() (map[string]string, error)`.

6. **`internal/index/rebuild.go`**
   - Implement `Rebuild(ws *Workspace) error`:
     1. Drop all tables.
     2. Re-create schema.
     3. Walk all checkpoint files in `.echo/checkpoints/`.
     4. For each checkpoint: load tree, walk tree, index files.
     5. Index current HEAD content for FTS.
     6. Re-index all refs.

### Tests for Phase 5

```
tests/integration/index_test.go
```

- Index files for a checkpoint, query them back.
- Query files with glob pattern filtering.
- Index multiple checkpoints, query files at specific checkpoint.
- Full-text search returns correct results with line numbers.
- Checkpoint query filtering by agent, task, date range.
- Full index rebuild produces identical results to incremental indexing.
- Concurrent reads don't block or error.

### Acceptance Criteria
- [x] All index operations are correct
- [x] FTS5 search works for common queries
- [x] Rebuild produces identical results
- [x] Tests pass with `-race`

---

## Phase 6: Diff, Revert, Branch, Merge

**Goal:** Complete the core VCS operations.

### Files to Implement

1. **`internal/core/diff.go`**
   - Implement `DiffCheckpoints(ws *Workspace, cpA, cpB string) (*DiffResult, error)`:
     - Load both checkpoint trees.
     - Use Merkle diff to find changed files.
     - For each changed file: compute line-level unified diff using Myers algorithm.
     - Return structured diff with hunks.
   - Implement `DiffWorking(ws *Workspace) (*DiffResult, error)` — diff working directory against HEAD.

2. **`internal/core/revert.go`**
   - Implement `Revert(ws *Workspace, targetCpID string, dryRun bool) (*RevertResult, error)`:
     1. Load target checkpoint's tree.
     2. Scan current working directory.
     3. Compute changes needed (files to restore, modify, delete).
     4. If `dryRun`, return changes without applying.
     5. Apply changes to working directory:
        - Read blobs from store → write to filesystem.
        - Delete files that don't exist in target checkpoint.
        - Create directories as needed.
     6. Create a revert checkpoint (parent = current HEAD, tree = target's tree).
   - Implement `RevertChangeset(ws *Workspace, cpID string) error` — inverse-apply a single changeset.

3. **`internal/core/merge.go`**
   - Implement `Merge(ws *Workspace, branchName string, strategy MergeStrategy) (*MergeResult, error)`:
     ```go
     type MergeStrategy int
     const (
         StrategyTheirs MergeStrategy = iota
         StrategyOurs
     )
     ```
     1. Resolve branch name to checkpoint ID.
     2. Find common ancestor (LCA in the DAG):
        - BFS from both checkpoints backward through parents.
        - First checkpoint reachable from both = LCA.
     3. Three-way diff: (base vs ours) and (base vs theirs).
     4. For non-conflicting changes: apply directly.
     5. For conflicts (same file changed in both): apply strategy.
     6. Write merged files to working directory.
     7. Create merge checkpoint with two parents.

4. **Branch commands in `internal/core/refs.go`** (extend from Phase 4):
   - Implement `CreateBranch(ws *Workspace, name string, fromCP string) error`.
   - Implement `SwitchBranch(ws *Workspace, name string) error`:
     1. Load target branch's HEAD checkpoint tree.
     2. Diff against current working directory.
     3. Apply changes to match target state.
     4. Update HEAD to point to new branch.
   - Implement `DeleteBranch(ws *Workspace, name string) error` — delete ref only.
   - Implement `ListBranches(ws *Workspace) ([]BranchInfo, error)`.

5. **DAG utilities** (add to `internal/core/checkpoint.go`):
   - Implement `FindLCA(ws *Workspace, cpA, cpB string) (string, error)` — lowest common ancestor.
   - Implement `WalkHistory(ws *Workspace, from string, fn func(*Checkpoint) error) error` — walk DAG backward.
   - Implement `GraphLog(ws *Workspace, limit int) ([]GraphEntry, error)` — build DAG for visualization.

### Tests for Phase 6

```
tests/integration/diff_test.go
tests/integration/revert_test.go
tests/integration/branch_test.go
tests/integration/merge_test.go
```

- Diff between two checkpoints shows correct changes per file.
- Line-level diff shows correct hunks.
- Revert restores workspace to exact checkpoint state.
- Revert creates a new checkpoint (history preserved).
- Dry-run revert shows changes without applying.
- Create branch, switch to it, verify HEAD updated.
- Switch branch updates working directory.
- Merge with no conflicts applies cleanly.
- Merge with conflicts uses correct strategy.
- Merge checkpoint has two parents.
- LCA correctly found in various DAG shapes.

### Acceptance Criteria
- [x] Diff produces correct, structured output
- [x] Revert restores exact state
- [x] Branch operations are correct
- [x] Merge with strategies works
- [x] Tests pass with `-race`

---

## Phase 7: CLI Layer

**Goal:** Wire everything up with Cobra CLI, implement all commands with both JSON and human output.

### Files to Implement

All files in `internal/cli/` and `cmd/echo/main.go`.

1. **`internal/output/json.go`** — JSON output helper: marshal any struct to stdout with consistent formatting.
2. **`internal/output/human.go`** — Human output: colors (detect TTY, respect `NO_COLOR` env var), relative timestamps.
3. **`internal/output/table.go`** — Aligned column output for list commands.
4. **`cmd/echo/main.go`** — Cobra root command, subcommand registration, global flags.
5. **Each `internal/cli/<command>.go`** — One file per command, delegates to `internal/core/`.

### Implementation Notes for CLI

- Use `cobra.Command` for each command. Register all in `root.go`.
- Global `--json` flag: set `outputMode` in context, all commands check it.
- Error handling pattern:
  ```go
  if err != nil {
      if jsonMode {
          json.NewEncoder(os.Stderr).Encode(map[string]string{"error": err.Error()})
      } else {
          fmt.Fprintf(os.Stderr, "error: %s\n", err)
      }
      os.Exit(1)
  }
  ```
- Working directory: default to `os.Getwd()`, override with `--dir` / `-C`.

### Tests for Phase 7

```
tests/integration/cli_test.go
```

- End-to-end: `echo init` → `echo checkpoint` → `echo log` → `echo diff` → `echo revert`.
- All commands with `--json` produce valid JSON.
- Error cases produce stderr output and exit code 1.
- `--dir` flag works for remote directory.
- `--quiet` suppresses output.

### Acceptance Criteria
- [x] All CLI commands functional
- [x] JSON output is valid and matches spec
- [x] Human output is readable
- [x] Error handling is consistent
- [x] Tests pass with `-race`

---

## Phase 8: Verify, GC, and Hardening

**Goal:** Integrity verification, garbage collection, and edge case hardening.

### Files to Implement

1. **`echo verify`** — Walk all objects, verify hashes, report corrupted.
2. **`echo gc`** — Walk all checkpoint trees, find unreferenced objects, delete them.
3. **Edge case hardening:**
   - Handle empty directories gracefully.
   - Handle binary files (large files, non-UTF8).
   - Handle symlinks (store target path as blob content).
   - Handle file permission changes (mode changes tracked as modifications).
   - Handle concurrent `echo` commands (lockfile correctly prevents corruption).
   - Handle very long file paths (Windows 260 char limit consideration).

### Tests for Phase 8

- Verify on clean store → all pass.
- Manually corrupt a blob → verify reports it.
- Verify with `--fix` removes corrupted blob.
- GC removes orphaned blobs.
- GC preserves all referenced blobs.
- Concurrent checkpoint attempts → one succeeds, other gets lock error.

### Acceptance Criteria
- [x] Verify catches all corruption
- [x] GC is safe (never deletes referenced objects)
- [x] Edge cases handled gracefully
- [x] All tests pass with `-race`
- [x] `golangci-lint` passes with zero warnings
