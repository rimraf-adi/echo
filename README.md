# Agent-Native Temporal Code Graph (formerly Echo)

**Fast, lightweight, content-verified codebase state tracker and temporal graph for AI agents.**

[![Go Version](https://img.shields.io/badge/go-1.24+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Pure Go](https://img.shields.io/badge/Pure%20Go-Zero%20CGo-blue)](#architecture--storage-model)

The Agent-Native Temporal Code Graph (ATCG) is a local-first, headless service that gives coding agents fast, structured answers about code structure **and** history. It is designed to capture history at the agent's own write granularity (independent of git) and record intent at write time.

> **AI Agent Specification**: Refer to [AGENTS.md](AGENTS.md) for machine installation instructions, non-interactive execution loops, and programmatic JSON contracts for autonomous agents.

---

## The Problem

Coding agents start every task from zero. They re-read files, guess at dependencies, and can't see why code looks the way it does. Static indexes show only the present, and git is built for humans: coarse commits, bulk agent writes, no semantics.

## Our Solution

ATCG provides:
1. Fast structured answers about code structure **and** history.
2. History captured at the agent's own write granularity, **independent of git**.
3. Intent recorded at write time, not mined afterward.
4. **Fully deterministic:** no LLM in the service. The calling agent supplies meaning; the service stores and computes facts.
5. **Agent-agnostic:** works with any agent with zero changes to the agent.
6. Always fresh and incremental, with no manual reindexing.

While human queries are possible via the CLI, the system is fundamentally optimized for agentic queries via MCP and JSON outputs.

---

## Core Architecture

ATCG builds upon the proven architecture of Echo:
- **Local-first SQLite + Blob Store**: Pure Go embedded database powered by `modernc.org/sqlite` and zlib-compressed content-addressed blobs.
- **Append-only Step Ledger**: A cryptographically verifiable DAG tracking File, Symbol, Module, and Test edges.
- **Multi-resolution History**: Epoch, Task, and Step resolutions.
- **Tree-sitter Indexer**: Incremental AST-based parsing (starting with TS/JS and Python) to power semantic symbol search and dependency mapping.
- **Tiered Capture System**: 
  - *Tier 0*: Automated background filesystem watcher (`echo watch`).
  - *Tier 1*: MCP tools and direct CLI commands.
  - *Tier 2*: Native hook adapters.

---

## Usage

*Documentation is currently being updated for the Temporal Code Graph pivot.*

### Quick Start (Tier 0 File Watcher & Checkpoints)

```bash
# 1. Initialize tracking in a project directory
echo init

# 2. Launch the automated watcher to capture changes in background (Tier 0)
echo watch --debounce 1000 --agent "auto-worker"

# 3. Snapshot state manually with agent metadata
echo checkpoint --agent "claude" --task "implement auth endpoints" --tag "auth"

# 4. Search file contents across the codebase via FTS5
echo search "def login" --context 2
```

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.
