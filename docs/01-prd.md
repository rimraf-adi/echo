# Echo — Product Requirements Document

## Overview

**Echo** is a lightweight, Go-based codebase state tracker purpose-built for AI agent workflows. It provides automatic checkpointing, full history traversal, structured querying, and deterministic rollback — completely independent of git.

Agents don't commit. They bulk-write. Echo captures every mutation as an atomic changeset, builds a content-verified Merkle tree of the codebase at each checkpoint, and maintains a DAG of history that any agent (or human) can walk, query, diff, and revert through.

---

## Problem Statement

### How Agents Interact With Code Today

1. An agent receives a task (e.g., "add authentication to this API").
2. The agent reads relevant files, reasons about changes, then **writes 10–50 files at once**.
3. If something breaks, the only recovery options are:
   - Hope git has a recent commit to revert to (it usually doesn't — agents don't commit).
   - Manually inspect and undo changes (impractical at scale).
   - Re-run the agent from scratch (expensive, non-deterministic).

### The Gaps

| Gap | Impact |
|-----|--------|
| No automatic state capture | Agent writes are fire-and-forget. State is lost. |
| No atomic grouping | A 40-file bulk write has no boundary — you can't undo "just that action" |
| No structured history | No way to ask "what did agent X change for task Y?" |
| No efficient diffing | Comparing two states requires full file-by-file scan |
| No integrity verification | No way to verify codebase hasn't been corrupted between actions |
| No branching for parallel agents | Two agents can't safely explore different approaches |

### Why Not Git?

Git solves a different problem — collaborative human development with explicit staging, commits, and merge workflows. For agents:

- **Too ceremonial**: `add → commit → push` is unnecessary friction for automated workflows.
- **No auto-capture**: Git only records what you explicitly commit.
- **Human-oriented metadata**: Commit messages are free-text strings, not structured data.
- **Expensive diffing for bulk operations**: Git's diff is optimized for human-sized changesets, not 50-file bulk writes.
- **No built-in query layer**: Searching file contents requires shelling out to `git grep`.

Echo is not a git replacement. It sits alongside git (or without it) as a parallel tracking system optimized for agent behavior.

---

## Target Users

1. **AI Agent Frameworks** — Any system that orchestrates agents writing code (Antigravity, Cursor, Aider, custom frameworks).
2. **Agent Developers** — Engineers building agents who need reliable state management and rollback.
3. **Multi-Agent Systems** — Environments where multiple agents work on the same codebase, potentially in parallel branches.

---

## Product Goals

### G1: Zero-Friction State Capture
Every agent action that mutates files is automatically captured as an atomic checkpoint. No explicit "commit" required.

### G2: Full History DAG
Maintain a complete, branching history of every codebase state. Support linear history, branching (parallel agents), and merging.

### G3: Structured Queryability
Agents can programmatically query: file listings, content search, history traversal, diffs between any two states — all via a structured API returning machine-readable output.

### G4: Deterministic Rollback
Revert to any checkpoint in history with a single operation. Revert an individual changeset without affecting other changes.

### G5: Data Integrity
Every checkpoint is verified via Merkle tree. Corruption is detectable. Content is deduplicated and compressed.

### G6: Performance
Must handle codebases of 10,000+ files with sub-second checkpoint creation. Storage overhead must be minimal through content-addressable deduplication and zlib compression.

---

## Functional Requirements

### FR1: Workspace Initialization

| ID | Requirement |
|----|-------------|
| FR1.1 | Initialize tracking on any directory, creating a `.echo/` metadata directory |
| FR1.2 | Create an initial checkpoint capturing the full codebase state |
| FR1.3 | Build the initial Merkle tree and blob store |
| FR1.4 | Generate a workspace configuration file (`meta.json`) |

### FR2: Checkpointing

| ID | Requirement |
|----|-------------|
| FR2.1 | Create a checkpoint capturing the full codebase state at a point in time |
| FR2.2 | Each checkpoint contains: unique ID, parent checkpoint ID(s), timestamp, Merkle root hash, metadata (agent, task, tags) |
| FR2.3 | Checkpoints are immutable once created |
| FR2.4 | Each checkpoint records a changeset: the list of file-level deltas (add/modify/delete) relative to its parent |
| FR2.5 | Checkpoint creation must be atomic — partial checkpoints must not exist |
| FR2.6 | Support for explicit metadata attachment: agent ID, model name, task description, custom key-value pairs |

### FR3: Blob Storage

| ID | Requirement |
|----|-------------|
| FR3.1 | File contents are stored as content-addressable blobs keyed by SHA-256 hash |
| FR3.2 | Blobs are compressed using zlib (compression level 6 by default, configurable) |
| FR3.3 | Identical file contents across any number of checkpoints are stored exactly once |
| FR3.4 | Blob integrity is verifiable by recomputing the SHA-256 hash |

### FR4: Merkle Tree

| ID | Requirement |
|----|-------------|
| FR4.1 | Each checkpoint has a Merkle tree representing the full directory structure |
| FR4.2 | Leaf nodes represent files (hash = SHA-256 of file content) |
| FR4.3 | Internal nodes represent directories (hash = SHA-256 of sorted concatenation of child hashes) |
| FR4.4 | The root hash uniquely identifies the entire codebase state |
| FR4.5 | Diffing two checkpoints walks both Merkle trees, only descending into subtrees where hashes differ — O(changes) not O(files) |
| FR4.6 | Tree nodes are serialized and stored in the object store alongside blobs |

### FR5: History DAG

| ID | Requirement |
|----|-------------|
| FR5.1 | Checkpoints form a directed acyclic graph (DAG) via parent references |
| FR5.2 | Linear history: each checkpoint has exactly one parent (except the root) |
| FR5.3 | Branching: a checkpoint can have multiple children (divergent history) |
| FR5.4 | Merging: a checkpoint can have multiple parents (convergent history) |
| FR5.5 | Named branches (refs) point to checkpoint IDs and advance on new checkpoints |
| FR5.6 | HEAD ref tracks the current active checkpoint |

### FR6: Querying

| ID | Requirement |
|----|-------------|
| FR6.1 | List all tracked files at any checkpoint (current or historical) |
| FR6.2 | Read file contents at any checkpoint |
| FR6.3 | Full-text content search across tracked files |
| FR6.4 | List history (linear log or full DAG) with filtering by agent, task, date range |
| FR6.5 | Diff between any two checkpoints: list of added/modified/deleted files with content diffs |
| FR6.6 | Show changeset details for any checkpoint |
| FR6.7 | Tree structure visualization at any checkpoint |
| FR6.8 | All query outputs must be available in JSON format for agent consumption |

### FR7: Revert / Rollback

| ID | Requirement |
|----|-------------|
| FR7.1 | Revert workspace to any checkpoint in history (restores all files to that state) |
| FR7.2 | Revert creates a new checkpoint (history is append-only, never destructive) |
| FR7.3 | Selective revert: undo a specific changeset without affecting other changes |
| FR7.4 | Dry-run mode: show what a revert would change without applying it |

### FR8: Branching

| ID | Requirement |
|----|-------------|
| FR8.1 | Create a named branch from any checkpoint |
| FR8.2 | Switch between branches (updates working directory to branch HEAD state) |
| FR8.3 | List all branches with their HEAD checkpoint |
| FR8.4 | Delete a branch (ref only — checkpoints and blobs are retained) |

### FR9: Merge

| ID | Requirement |
|----|-------------|
| FR9.1 | Merge two branches using a deterministic strategy (no interactive conflict resolution) |
| FR9.2 | Default strategy: "theirs wins" — the incoming branch's version takes precedence |
| FR9.3 | Alternative strategy: "ours wins" — the current branch's version takes precedence |
| FR9.4 | Merge creates a new checkpoint with two parent references |
| FR9.5 | Report conflicts (files modified in both branches) in structured output |

---

## Non-Functional Requirements

### NFR1: Performance

| ID | Requirement | Target |
|----|-------------|--------|
| NFR1.1 | Checkpoint creation (1,000 files, 50 changed) | < 500ms |
| NFR1.2 | Checkpoint creation (10,000 files, 200 changed) | < 2s |
| NFR1.3 | Diff between two checkpoints (10,000 files) | < 200ms (Merkle-accelerated) |
| NFR1.4 | File content query at historical checkpoint | < 50ms |
| NFR1.5 | Full-text search across 10,000 files | < 1s |

### NFR2: Storage Efficiency

| ID | Requirement |
|----|-------------|
| NFR2.1 | Content-addressable deduplication: identical files stored once regardless of checkpoint count |
| NFR2.2 | zlib compression on all blobs: target 60-70% size reduction for source code |
| NFR2.3 | Merkle tree nodes are compact: path + hash only, no content duplication |

### NFR3: Reliability

| ID | Requirement |
|----|-------------|
| NFR3.1 | All write operations (checkpoint, revert) are atomic — crash-safe |
| NFR3.2 | Merkle root hash verifies entire codebase integrity in O(1) |
| NFR3.3 | `echo verify` command validates all blobs against their hashes |
| NFR3.4 | Corrupted blobs are detectable and reportable |

### NFR4: Portability

| ID | Requirement |
|----|-------------|
| NFR4.1 | Single static binary, no runtime dependencies |
| NFR4.2 | Cross-platform: Linux (amd64, arm64), macOS (amd64, arm64), Windows (amd64) |
| NFR4.3 | `.echo/` directory is self-contained and portable |

---

## CLI Command Summary

| Command | Description |
|---------|-------------|
| `echo init` | Initialize tracking in current directory |
| `echo checkpoint` | Create a new checkpoint |
| `echo log` | Show checkpoint history |
| `echo diff <cp-a> <cp-b>` | Diff between two checkpoints |
| `echo show <cp-id>` | Show changeset details for a checkpoint |
| `echo files [--at <cp-id>]` | List tracked files |
| `echo cat <file> [--at <cp-id>]` | Show file contents |
| `echo search <query>` | Full-text search |
| `echo tree [--at <cp-id>]` | Directory tree |
| `echo revert <cp-id>` | Revert to a checkpoint |
| `echo branch <name>` | Create a branch |
| `echo switch <name>` | Switch to a branch |
| `echo branches` | List branches |
| `echo merge <branch>` | Merge a branch |
| `echo status` | Show current state |
| `echo verify` | Verify integrity |
| `echo gc` | Garbage collect unreferenced blobs |

All commands support `--json` flag for machine-readable output.

---

## Out of Scope (v1)

- Git interoperability (reading/writing `.git` repos)
- Remote push/pull (network sync)
- File-level locking
- Large file support (LFS equivalent)
- GUI / web interface
- Filesystem watcher (auto-checkpoint on file change) — may add in v2
- Partial/sparse checkpoints (always full codebase state)

---

## Success Metrics

| Metric | Target |
|--------|--------|
| Checkpoint creation p99 latency (1K files) | < 500ms |
| Storage overhead vs raw codebase | < 40% (with dedup + compression) |
| History query latency | < 100ms |
| Diff accuracy | 100% (verified via Merkle root) |
| Single binary size | < 15MB |
| Zero external dependencies at runtime | Yes |
