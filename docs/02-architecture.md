# Echo — Architecture & Data Model

## System Architecture

```
┌─────────────────────────────────────────────────────┐
│                   CLI / API Layer                    │
│  echo init | checkpoint | log | diff | revert | ... │
├─────────────────────────────────────────────────────┤
│                   Core Engine                        │
│  Checkpoint Manager │ Diff Engine │ Query Engine     │
├─────────────────────────────────────────────────────┤
│                   Object Layer                       │
│  Merkle Tree Builder │ Blob Manager │ DAG Walker     │
├─────────────────────────────────────────────────────┤
│                   Storage Layer                      │
│  Object Store (blobs + trees) │ Ref Store │ Index DB │
├─────────────────────────────────────────────────────┤
│                   Filesystem                         │
│  .echo/ directory structure                          │
└─────────────────────────────────────────────────────┘
```

---

## Directory Layout

```
project/
├── .echo/
│   ├── config.json                # Workspace configuration
│   ├── HEAD                       # Current branch name (plain text)
│   ├── refs/
│   │   ├── main                   # → checkpoint ID (plain text file)
│   │   └── feature-auth           # → checkpoint ID
│   ├── objects/
│   │   ├── ab/
│   │   │   └── cdef1234...        # Object files (first 2 chars = dir)
│   │   ├── 3f/
│   │   │   └── 9a8b7c6d...
│   │   └── ...
│   ├── checkpoints/
│   │   ├── <checkpoint-id>.json   # Checkpoint metadata
│   │   └── ...
│   ├── index.db                   # SQLite index for fast queries
│   └── lock                       # Lock file for concurrent access
└── ... (actual project files)
```

### Why This Layout

- **`objects/`** uses 2-char prefix directories (like git) to avoid filesystem performance degradation with thousands of files in a single directory.
- **`refs/`** are plain text files containing a checkpoint ID — simple, atomic-writable, human-readable.
- **`HEAD`** contains the current branch name (e.g., `main`), which indirects to the checkpoint ID via refs.
- **`checkpoints/`** are separate from objects because they contain metadata that needs to be human-inspectable and are always JSON.
- **`index.db`** is a SQLite database for fast queries — rebuilt from objects if corrupted.

---

## Object Model

Echo has three types of objects, all stored in `.echo/objects/` as zlib-compressed data, keyed by their SHA-256 hash.

### Object Type 1: Blob

A blob stores the compressed contents of a single file.

```
┌──────────────────────────────┐
│ Blob                         │
├──────────────────────────────┤
│ Type: "blob"                 │
│ Hash: SHA-256(raw content)   │
│ Data: zlib(raw content)      │
│ Size: original byte count    │
└──────────────────────────────┘
```

**Storage format on disk:**

```
[header][compressed-data]
```

Header (binary, fixed size):
```
Bytes 0-3:    Magic number "ECHO"
Byte  4:      Object type (0x01 = blob, 0x02 = tree)
Bytes 5-8:    Original size (uint32, big-endian)
Bytes 9-40:   SHA-256 hash of original content (32 bytes)
Bytes 41+:    zlib-compressed content
```

**Key properties:**
- Hash is computed on the **raw (uncompressed)** content, so the same file always produces the same hash regardless of compression settings.
- The hash serves as both the storage key and the integrity check.

### Object Type 2: Tree

A tree represents a directory. It maps entry names to their type and hash.

```
┌────────────────────────────────────────────┐
│ Tree                                       │
├────────────────────────────────────────────┤
│ Type: "tree"                               │
│ Hash: SHA-256(canonical serialization)     │
│ Entries: sorted list of (mode, name, hash) │
└────────────────────────────────────────────┘
```

**Canonical serialization format (before hashing):**

```
Each entry is one line, sorted lexicographically by name:
<mode> <hash> <name>\n

Example:
100644 a1b2c3d4e5f6... README.md
100644 f6e5d4c3b2a1... main.go
040000 1a2b3c4d5e6f... pkg
```

Modes:
- `100644` — regular file (blob reference)
- `100755` — executable file (blob reference)
- `040000` — directory (tree reference)
- `120000` — symlink (blob containing link target path)

**Key properties:**
- Entries are **sorted by name** to ensure deterministic hashing. The same directory contents always produce the same tree hash.
- Tree hash = SHA-256 of the canonical serialization (all entry lines concatenated).
- Trees reference other trees (subdirectories) and blobs (files), forming the Merkle tree.

### Object Type 3: Checkpoint

Checkpoints are **not** stored in the object store. They are stored as JSON files in `.echo/checkpoints/` because:
1. They contain mutable metadata (tags can be added post-creation).
2. They need to be human-readable without decompression.
3. They reference objects by hash but are not themselves content-addressed.

```json
{
  "id": "cp-20260914-093500-a1b2c3d4",
  "parents": ["cp-20260914-093000-x9y8z7w6"],
  "tree_hash": "ab3def78...",
  "created_at": "2026-09-14T09:35:00.123Z",
  "metadata": {
    "agent": "claude-opus-4-20250514",
    "task": "add-authentication",
    "model": "claude-opus-4-20250514",
    "tags": ["auth", "security"],
    "custom": {
      "token_count": 15420,
      "tool_calls": 7
    }
  },
  "changeset": {
    "added": ["internal/auth/handler.go", "internal/auth/middleware.go"],
    "modified": ["cmd/server/main.go", "internal/routes/router.go"],
    "deleted": []
  },
  "stats": {
    "total_files": 847,
    "total_size_bytes": 2341567,
    "blobs_reused": 842,
    "blobs_new": 5,
    "compressed_size_bytes": 891204
  }
}
```

**Checkpoint ID format:** `cp-<YYYYMMDD>-<HHMMSS>-<8-char-random-hex>`

This is human-readable (you can eyeball the date) but unique enough to avoid collisions.

---

## Merkle Tree Structure

### How The Tree Is Built

Given a project directory:
```
project/
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   ├── auth/
│   │   ├── handler.go
│   │   └── middleware.go
│   └── routes/
│       └── router.go
├── go.mod
└── README.md
```

The Merkle tree is:

```
Root Tree (hash: ROOT_HASH)
├── 100644 BLOB_HASH_1  README.md
├── 100644 BLOB_HASH_2  go.mod
├── 040000 TREE_HASH_A  cmd
│   └── cmd/ Tree (hash: TREE_HASH_A)
│       └── 040000 TREE_HASH_B  server
│           └── server/ Tree (hash: TREE_HASH_B)
│               └── 100644 BLOB_HASH_3  main.go
└── 040000 TREE_HASH_C  internal
    └── internal/ Tree (hash: TREE_HASH_C)
        ├── 040000 TREE_HASH_D  auth
        │   └── auth/ Tree (hash: TREE_HASH_D)
        │       ├── 100644 BLOB_HASH_4  handler.go
        │       └── 100644 BLOB_HASH_5  middleware.go
        └── 040000 TREE_HASH_E  routes
            └── routes/ Tree (hash: TREE_HASH_E)
                └── 100644 BLOB_HASH_6  router.go
```

### Merkle-Accelerated Diff Algorithm

To diff Checkpoint A and Checkpoint B:

```
function diff(tree_a, tree_b):
    if tree_a.hash == tree_b.hash:
        return []  // Entire subtree is identical — skip

    changes = []
    entries_a = map of tree_a entries by name
    entries_b = map of tree_b entries by name

    for name in union(entries_a.keys, entries_b.keys):
        if name not in entries_a:
            changes.append(ADDED, name, entries_b[name])
        else if name not in entries_b:
            changes.append(DELETED, name, entries_a[name])
        else if entries_a[name].hash != entries_b[name].hash:
            if both are trees:
                changes.extend(diff(entries_a[name], entries_b[name]))
            else:
                changes.append(MODIFIED, name, entries_a[name], entries_b[name])

    return changes
```

**Complexity:** O(number of changed files + depth of changed paths), NOT O(total files). For a 10,000-file codebase with 5 changed files, this touches ~20 tree nodes instead of 10,000 files.

---

## SQLite Index Schema

The SQLite index (`index.db`) accelerates queries. It is **derived data** — it can be fully rebuilt from the object store and checkpoint files.

```sql
-- All known files across all checkpoints
CREATE TABLE files (
    checkpoint_id TEXT NOT NULL,
    path          TEXT NOT NULL,
    blob_hash     TEXT NOT NULL,
    mode          INTEGER NOT NULL,
    size_bytes    INTEGER NOT NULL,
    PRIMARY KEY (checkpoint_id, path)
);

-- Checkpoint metadata for fast filtering
CREATE TABLE checkpoints (
    id            TEXT PRIMARY KEY,
    parent_ids    TEXT NOT NULL,          -- JSON array of parent checkpoint IDs
    tree_hash     TEXT NOT NULL,
    created_at    TEXT NOT NULL,          -- ISO 8601
    agent         TEXT,
    task          TEXT,
    model         TEXT,
    tags          TEXT,                   -- JSON array
    total_files   INTEGER NOT NULL,
    total_bytes   INTEGER NOT NULL
);

-- Branch refs
CREATE TABLE refs (
    name          TEXT PRIMARY KEY,
    checkpoint_id TEXT NOT NULL
);

-- Full-text search index on file contents
-- Uses SQLite FTS5 for efficient text search
CREATE VIRTUAL TABLE content_fts USING fts5(
    path,
    content,
    checkpoint_id UNINDEXED
);

-- Indexes for common query patterns
CREATE INDEX idx_files_path ON files(path);
CREATE INDEX idx_files_blob ON files(blob_hash);
CREATE INDEX idx_checkpoints_agent ON checkpoints(agent);
CREATE INDEX idx_checkpoints_task ON checkpoints(task);
CREATE INDEX idx_checkpoints_created ON checkpoints(created_at);
```

### Index Rebuild

If `index.db` is corrupted or deleted:

```
echo index rebuild
```

This walks all checkpoint files, loads their tree hashes, traverses the Merkle trees, and repopulates the index. The object store is the source of truth.

---

## Compression Strategy

### Blob Compression

- **Algorithm:** zlib (RFC 1950), using Go's `compress/flate` with `zlib` wrapper.
- **Default level:** 6 (balanced speed/ratio). Configurable via `config.json`.
- **Compression target:** 60-70% reduction for typical source code files.

### When Compression Happens

```
Write path:
  raw content → SHA-256 hash → check if blob exists → if not: zlib compress → write to objects/

Read path:
  objects/<hash> → read header → zlib decompress → return raw content
```

### Compression Is Skipped For

- Files already smaller than 64 bytes (overhead exceeds savings).
- Binary files detected as incompressible (if compressed size ≥ 95% of original, store uncompressed and set a flag in the header).

### Object File Binary Format (detailed)

```
Offset  Size  Field
------  ----  -----
0       4     Magic: 0x4543484F ("ECHO")
4       1     Version: 0x01
5       1     Object type: 0x01 (blob) | 0x02 (tree)
6       1     Flags: bit 0 = compressed (1) or raw (0)
7       1     Reserved (0x00)
8       4     Original size (uint32 big-endian)
12      4     Stored size (uint32 big-endian, = original if uncompressed)
16      32    SHA-256 hash of original content
48      ...   Payload (zlib-compressed or raw, per flags)
```

Total header size: **48 bytes**.

---

## Concurrency & Atomicity

### Write Locking

Echo uses a lockfile (`.echo/lock`) for write operations:

```
1. Attempt to create .echo/lock exclusively (O_EXCL)
2. If lock exists, check if PID inside is still alive
3. If stale, remove and retry. If active, fail with error.
4. Perform write operation
5. Remove lock
```

### Atomic Checkpoint Creation

Checkpoints must be all-or-nothing:

```
1. Acquire write lock
2. Scan workspace for changes (compare against current Merkle tree)
3. Write new blobs to .echo/objects/ (each blob is atomic: write to temp, rename)
4. Build new tree objects, write to .echo/objects/
5. Write checkpoint JSON to .echo/checkpoints/<id>.json (write to temp, rename)
6. Update ref file (write to temp, rename)
7. Update index.db (within a SQLite transaction)
8. Release write lock
```

**Crash safety:** If Echo crashes at any point:
- Steps 3-4: Orphaned objects in the store are harmless (GC cleans them up).
- Step 5: No checkpoint file = checkpoint doesn't exist.
- Step 6: Ref still points to previous checkpoint.
- Step 7: Index is stale but rebuildable.

### Concurrent Reads

Reads do **not** acquire locks. They operate on immutable objects and snapshot-consistent checkpoint files. SQLite handles concurrent read access natively with WAL mode.

---

## Ignore Rules

### Default Ignores

Echo ignores these by default (not configurable):
- `.echo/` (own metadata)
- `.git/` (git metadata)

### User Ignores

File: `.echo/ignore` (same syntax as `.gitignore`)

```
# Example .echo/ignore
node_modules/
*.pyc
__pycache__/
.env
*.log
dist/
build/
.DS_Store
```

The ignore engine supports:
- Glob patterns (`*.log`, `build/**`)
- Directory patterns (trailing `/`)
- Negation (`!important.log`)
- Comments (`#`)

---

## Configuration

File: `.echo/config.json`

```json
{
  "version": 1,
  "workspace_id": "ws-a1b2c3d4",
  "created_at": "2026-09-14T09:30:00Z",
  "compression": {
    "level": 6,
    "min_size_bytes": 64
  },
  "hash_algorithm": "sha256",
  "default_branch": "main",
  "auto_checkpoint": {
    "enabled": false,
    "debounce_ms": 1000
  }
}
```
