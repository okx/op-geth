---
name: "ethdb"
description: "Module design for ethdb: database abstraction layer with KV store and ancient interfaces"
---
# EthDB Module

## Responsibilities

- Defines pure storage interfaces: `KeyValueStore`, `Database`, `AncientStore`
- Interface methods: Has, Get, Put, Delete, DeleteRange, Compact, Stat, Sync
- Ancient/Freezer interface for immutable chain segments
- Backend implementations: Pebble, LevelDB, RocksDB, in-memory

## NOT Responsible For

- Chain-level semantics (handled by `core/rawdb`)
- Trie management (handled by `triedb`)
- Any higher-layer business logic

## Core Entities

| Entity | Key Fields | Description |
|--------|-----------|-------------|
| `KeyValueStore` | interface | Pure KV store (Has/Get/Put/Delete) |
| `Database` | interface | KeyValueStore + AncientStore |
| `AncientStore` | interface | Immutable append-only storage |
| `pebble.Database` | `db *pebble.DB` | Pebble KV backend |

## Dependencies

- Require to reference arch/dependency.md for full dependency details

## Relevant Flows

- Require to reference core-flows/ for flows involving this module

## Module-Specific Pitfalls

[Rule] `ethdb` package must never import any higher-layer package — it defines pure storage interfaces. Only `bytes`, `errors`, `io` imports allowed.

[Rule] `AncientStore` operations must never be called on `nofreezedb` — returns `errNotSupported`.
