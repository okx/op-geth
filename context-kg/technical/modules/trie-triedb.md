---
name: "trie-triedb"
description: "Module design for trie/triedb: Merkle Patricia Trie implementation and database backends"
---
# Trie / TrieDB Module

## Responsibilities

- Merkle Patricia Trie implementation (secure trie, stack trie)
- TrieDB: trie node database abstraction with backend selection
- Path-based state scheme (`pathdb`): diff layers, disk layer, state history, flush
- Hash-based state scheme (`hashdb`): legacy content-addressed trie node storage
- Binary trie (experimental `bintrie`)
- Transition trie for migration between schemes

## NOT Responsible For

- Account/storage state semantics (handled by `core/state`)
- Block storage (handled by `core/rawdb`)
- Database I/O (handled by `ethdb`)

## Core Entities

| Entity | Key Fields | Description |
|--------|-----------|-------------|
| `triedb.Database` | `disk ethdb.Database`, `config`, backend (hashdb/pathdb) | Trie database manager |
| `pathdb.Database` | `diskdb`, `tree` (layerTree), `freezer` | Path-based trie backend |
| `hashdb.Database` | `diskdb`, `nodes`, `dirties` | Hash-based trie backend |
| `trie.Trie` | `root`, `reader`, `tracer` | In-memory trie instance |

## Dependencies

- Require to reference arch/dependency.md for full dependency details

## Relevant Flows

- Require to reference core-flows/ for flows involving this module

## Module-Specific Pitfalls

[Pitfall] pathdb state rollback is broken in dev mode — when `stateFreezer` is nil, `Recoverable()` always returns false and state rollback returns error. Deep reorgs unsupported in dev mode. Source: TODO at `pathdb/database.go:503`.

[Pitfall] Background state history indexing runs without checking read-only mode — may cause unexpected write attempts. Source: TODO at `pathdb/database.go:210`.

[Rule] TrieDB backend selection (hashdb vs pathdb) is determined at init by `Config.HashDB` / `Config.PathDB` — must never switch scheme on existing database. Reason: schema mismatch causes data corruption.
