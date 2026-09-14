# Echo — Testing Strategy

## Testing Philosophy

Every phase must have tests before moving to the next phase. Tests are the contract — if the tests pass, the implementation is correct. An agent implementing a phase should write tests first (or alongside) the implementation.

---

## Test Structure

```
tests/
├── integration/
│   ├── helpers_test.go          # Shared test utilities
│   ├── store_test.go            # Phase 1: Object store & blobs
│   ├── merkle_test.go           # Phase 2: Merkle trees
│   ├── scan_test.go             # Phase 3: Scanner
│   ├── ignore_test.go           # Phase 3: Ignore patterns
│   ├── workspace_test.go        # Phase 4: Workspace init
│   ├── checkpoint_test.go       # Phase 4: Checkpointing
│   ├── index_test.go            # Phase 5: SQLite index
│   ├── diff_test.go             # Phase 6: Diffing
│   ├── revert_test.go           # Phase 6: Revert
│   ├── branch_test.go           # Phase 6: Branching
│   ├── merge_test.go            # Phase 6: Merge
│   └── cli_test.go              # Phase 7: End-to-end CLI
│
└── fixtures/
    ├── small_project/            # ~10 files, known content
    │   ├── main.go
    │   ├── utils.go
    │   ├── go.mod
    │   └── README.md
    └── ignore_patterns/          # Files for testing ignore rules
        ├── .echo-ignore-test
        ├── keep.go
        ├── skip.log
        └── node_modules/
            └── package.json
```

---

## Test Helpers (`helpers_test.go`)

Every test that needs a workspace should use these helpers:

```
SetupTestWorkspace(t *testing.T) (rootDir string, cleanup func())
```
- Creates a temp directory.
- Populates with test files.
- Returns path and cleanup function.
- `t.Cleanup(cleanup)` to auto-clean.

```
WriteTestFile(t *testing.T, root string, relPath string, content string)
```
- Creates parent directories and writes file.

```
RunEcho(t *testing.T, root string, args ...string) (stdout, stderr string, exitCode int)
```
- Executes the `echo` binary with given args in the given directory.
- Captures stdout, stderr, exit code.
- For CLI tests only (Phase 7+).

```
MustParseJSON[T any](t *testing.T, data string) T
```
- Parse JSON output into typed struct. Fails test on error.

---

## Test Scenarios by Phase

### Phase 1: Object Store

| # | Test | Validates |
|---|------|-----------|
| 1 | Write blob, read back, compare content | Basic round-trip |
| 2 | Write same content twice, count object files | Deduplication |
| 3 | Write 10-byte file, check flags in header | Compression skip for tiny files |
| 4 | Write random binary blob, check if stored uncompressed | Incompressible detection |
| 5 | Read blob, corrupt file on disk, read again | Integrity checking |
| 6 | Write 1000 diverse blobs, read all back | Bulk correctness |
| 7 | HasObject for existing and non-existing hash | Existence check |
| 8 | DeleteObject, verify HasObject returns false | Deletion |
| 9 | ListObjects returns all stored hashes | Listing |
| 10 | Header encode/decode round-trip with edge values | Binary format |

### Phase 2: Merkle Tree

| # | Test | Validates |
|---|------|-----------|
| 1 | Build tree from 5 files, verify root hash | Basic construction |
| 2 | Build same tree twice, compare root hashes | Determinism |
| 3 | Change one file content, rebuild, compare root | Hash propagation |
| 4 | Add a file, rebuild, compare root | Tree growth |
| 5 | Remove a file, rebuild, compare root | Tree shrinkage |
| 6 | Diff two identical trees → empty | No-op diff |
| 7 | Diff with one added file → correct delta | Add detection |
| 8 | Diff with one modified file → correct delta | Modify detection |
| 9 | Diff with one deleted file → correct delta | Delete detection |
| 10 | Diff with nested directory change → full path in delta | Deep path handling |
| 11 | Diff with unchanged subtree → subtree not traversed | Performance (count store reads) |
| 12 | Serialize + deserialize tree → identical | Serialization round-trip |
| 13 | Walk tree → all entries visited, correct order | Traversal |

### Phase 3: Scanner & Ignore

| # | Test | Validates |
|---|------|-----------|
| 1 | Scan directory with 10 files → all found | Basic scan |
| 2 | Scan with .echo/ present → excluded | Default ignore |
| 3 | Scan with .git/ present → excluded | Default ignore |
| 4 | Scan with .echo/ignore `*.log` → .log files excluded | Glob pattern |
| 5 | Scan with `node_modules/` ignore → entire dir skipped | Directory pattern |
| 6 | Scan with `!important.log` negation → file included | Negation |
| 7 | Scan with `#comment` lines → comments ignored | Comment handling |
| 8 | Parallel hash with 1 worker vs 8 workers → same results | Correctness |
| 9 | Scan empty directory → empty result, no error | Edge case |
| 10 | Scan with nested ignore patterns → correct behavior | Depth handling |

### Phase 4: Workspace & Checkpointing

| # | Test | Validates |
|---|------|-----------|
| 1 | Init creates .echo/ with all expected files | Directory structure |
| 2 | Init creates initial checkpoint with correct file count | Initial state |
| 3 | Open from subdirectory finds .echo/ | Root discovery |
| 4 | Checkpoint with no changes → returns nil | No-op detection |
| 5 | Add file, checkpoint → changeset shows added | Add tracking |
| 6 | Modify file, checkpoint → changeset shows modified | Modify tracking |
| 7 | Delete file, checkpoint → changeset shows deleted | Delete tracking |
| 8 | Checkpoint metadata correctly stored | Metadata persistence |
| 9 | Two sequential checkpoints → correct parent chain | DAG linearity |
| 10 | Resolve HEAD → correct checkpoint ID | Ref resolution |
| 11 | Resolve HEAD~1 → parent checkpoint ID | Ancestor resolution |
| 12 | Resolve branch name → correct checkpoint ID | Branch resolution |
| 13 | Checkpoint with concurrent lock → error | Lock safety |

### Phase 5: SQLite Index

| # | Test | Validates |
|---|------|-----------|
| 1 | Index files, query all → correct count | Basic indexing |
| 2 | Index files, query with glob → filtered results | Glob filtering |
| 3 | Index two checkpoints, query each → correct files | Multi-checkpoint |
| 4 | Full-text search → correct results | FTS5 |
| 5 | Checkpoint query by agent → filtered | Metadata filtering |
| 6 | Checkpoint query by date range → filtered | Date filtering |
| 7 | Rebuild index → identical to incremental | Rebuild correctness |
| 8 | Concurrent reads during write → no errors | Concurrency |

### Phase 6: Diff, Revert, Branch, Merge

| # | Test | Validates |
|---|------|-----------|
| 1 | Diff two checkpoints → correct file-level changes | Diff accuracy |
| 2 | Diff includes line-level hunks | Line-level diff |
| 3 | Revert to checkpoint → workspace matches exactly | Full revert |
| 4 | Revert creates new checkpoint | History preservation |
| 5 | Dry-run revert → no changes applied | Dry-run safety |
| 6 | Create branch, verify ref exists | Branch creation |
| 7 | Switch branch → workspace changes | Branch switching |
| 8 | List branches → all shown | Branch listing |
| 9 | Delete branch → ref removed, checkpoints remain | Branch deletion |
| 10 | Merge non-conflicting → clean merge | Basic merge |
| 11 | Merge conflicting, strategy=theirs → theirs wins | Strategy: theirs |
| 12 | Merge conflicting, strategy=ours → ours wins | Strategy: ours |
| 13 | Merge checkpoint has two parents | Merge DAG |
| 14 | LCA found correctly in diamond DAG | Ancestor finding |

### Phase 7: CLI End-to-End

| # | Test | Validates |
|---|------|-----------|
| 1 | `echo init` → exit 0, .echo/ created | Init command |
| 2 | `echo checkpoint --json` → valid JSON output | JSON mode |
| 3 | `echo log --limit 5 --json` → array of checkpoints | Log command |
| 4 | `echo diff HEAD~1 HEAD --json` → diff structure | Diff command |
| 5 | `echo files --json` → file list | Files command |
| 6 | `echo cat main.go` → file content to stdout | Cat command |
| 7 | `echo search "func main" --json` → search results | Search command |
| 8 | `echo revert HEAD~1 --json` → revert result | Revert command |
| 9 | `echo status --json` → status structure | Status command |
| 10 | `echo branch test && echo switch test` → works | Branch commands |
| 11 | Unknown command → exit 2, stderr message | Error handling |
| 12 | `echo init` in non-empty tracked dir → error | Double init |
| 13 | `echo verify --json` → verification result | Verify command |

---

## Running Tests

```bash
# All tests with race detection
go test ./... -v -race -count=1

# Specific phase
go test ./tests/integration/ -v -race -run TestStore
go test ./tests/integration/ -v -race -run TestMerkle
go test ./tests/integration/ -v -race -run TestScan
go test ./tests/integration/ -v -race -run TestCheckpoint
go test ./tests/integration/ -v -race -run TestIndex
go test ./tests/integration/ -v -race -run TestDiff
go test ./tests/integration/ -v -race -run TestRevert
go test ./tests/integration/ -v -race -run TestBranch
go test ./tests/integration/ -v -race -run TestMerge
go test ./tests/integration/ -v -race -run TestCLI

# With coverage
go test ./... -v -race -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

---

## Coverage Targets

| Package | Target |
|---------|--------|
| `internal/store/` | ≥ 95% |
| `internal/merkle/` | ≥ 95% |
| `internal/scan/` | ≥ 90% |
| `internal/ignore/` | ≥ 95% |
| `internal/core/` | ≥ 90% |
| `internal/index/` | ≥ 85% |
| `internal/cli/` | ≥ 80% |
| **Overall** | **≥ 90%** |
