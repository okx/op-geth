---
name: "common"
description: "Module design for common: shared types, utilities, data structures used across all packages"
---
# Common Module

## Responsibilities

- Fundamental types: `Hash` (32-byte), `Address` (20-byte)
- Hex utilities: `hexutil.Bytes`, `hexutil.Big`, `hexutil.Uint`, `hexutil.Uint64` for JSON-RPC wire format
- Byte manipulation: `CopyBytes`, `LeftPadBytes`, `RightPadBytes`, `TrimLeftZeroes`
- LRU caches: `lru.Cache[K,V]` (thread-safe), `lru.BasicLRU[K,V]` (non-thread-safe), `SizeConstrainedCache`
- Priority queue: `prque.Prque[P,V]`
- Monotonic clock: `mclock.Clock` interface for testability
- Display formatting: `StorageSize`, `PrettyDuration`, `PrettyAge`
- Big integer constants: `Big0`, `Big1`, `Big2`, `Big3`, `Big32`, `Big256`, `Big257`
- Bitutil: compression and bit operations

## NOT Responsible For

- Any chain or consensus logic
- RPC serving or P2P networking
- State management

## Core Entities

| Entity | Key Fields | Description |
|--------|-----------|-------------|
| `Hash` | [32]byte | Keccak256 hash |
| `Address` | [20]byte | Ethereum address |
| `lru.Cache[K,V]` | `BasicLRU` + `sync.Mutex` | Thread-safe LRU cache |
| `lru.SizeConstrainedCache` | `maxSize`, `size` | Byte-capacity-bounded LRU |
| `mclock.Clock` | interface | Injectable monotonic clock |

## Dependencies

- Require to reference arch/dependency.md for full dependency details

## Relevant Flows

- Require to reference core-flows/ for flows involving this module

## Module-Specific Pitfalls

[Rule] All hex strings in JSON-RPC wire format must use `hexutil` functions with "0x" prefix — never raw encoding.

[Rule] `hexutil.MustDecode` must never be used outside package-level initialisation — it panics.

[Rule] `BasicLRU` zero value is invalid — always use `NewBasicLRU(capacity)`.

[Rule] `BasicLRU` must never be used from concurrent goroutines without external locking — use `lru.Cache[K,V]` instead.

[Rule] Values must not be mutated after passing to `SizeConstrainedCache.Add`.
