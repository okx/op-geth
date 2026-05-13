---
name: "common-tools"
description: "Must-reuse common components — duplication prohibited"
---
# Common Tools

## Hex and Encoding

[Reuse] `hexutil.Bytes` / `hexutil.Big` / `hexutil.Uint` / `hexutil.Uint64`: Use in all JSON-RPC request/response structs for binary/numeric data. Package: `common/hexutil/json.go`.

[Reuse] `hexutil.Encode` / `hexutil.Decode`: All RPC wire hex must go through hexutil with 0x prefix. Package: `common/hexutil/hexutil.go`.

[Reuse] `hexutil.UnmarshalFixedJSON`: Implement `UnmarshalJSON` for fixed-size byte types by delegating to this helper. Package: `common/hexutil/json.go`.

[Reuse] `common.FromHex` / `Hex2Bytes` / `Bytes2Hex`: Internal binary/hex work without 0x prefix. Package: `common/bytes.go`.

## Byte Operations

[Reuse] `common.CopyBytes`: Nil-safe byte slice copy — use instead of manual `make+copy`. Package: `common/bytes.go`.

[Reuse] `common.LeftPadBytes` / `RightPadBytes`: Byte-padding to fixed length for ABI/RLP encoding. Package: `common/bytes.go`.

[Reuse] `common.TrimLeftZeroes` / `TrimRightZeroes`: Stripping leading/trailing zero bytes for canonical encoding. Package: `common/bytes.go`.

## Caching

[Reuse] `lru.Cache[K,V]`: Thread-safe generic LRU cache. Package: `common/lru/lru.go`. Usage: any concurrent caching need.

[Reuse] `lru.BasicLRU[K,V]`: Non-thread-safe LRU — use only inside already-locked structs. Package: `common/lru/basiclru.go`.

[Reuse] `lru.SizeConstrainedCache[K,V]`: Byte-size-bounded LRU for blob/code caches. Package: `common/lru/blob_lru.go`.

## Time and Scheduling

[Reuse] `mclock.Clock` / `mclock.System{}`: Injectable monotonic clock for testability. Package: `common/mclock/mclock.go`.

[Reuse] `mclock.AbsTime`: Monotonic timestamp type for performance-sensitive timing. Package: `common/mclock/mclock.go`.

## Data Structures

[Reuse] `prque.Prque[P,V]`: Generic max-priority queue. Negate priority for min-queue. Package: `common/prque/prque.go`.

[Reuse] `event.Feed`: One-to-many typed event broadcast. Preferred over `TypeMux`. Package: `event/feed.go`.

## Constants

[Reuse] `common.Big0` / `Big1` / `Big2` / `Big3` / `Big32` / `Big256` / `Big257`: Shared constant `big.Int` values — never call `big.NewInt()` for these. Package: `common/big.go`.

[Reuse] `common.U2560`: Shared constant `uint256` zero — use instead of `uint256.NewInt(0)`. Package: `common/big.go`.

## Display

[Reuse] `common.StorageSize`: Float64 wrapper with human-readable IEC formatting for byte counts. Package: `common/size.go`.

[Reuse] `common.PrettyDuration` / `common.PrettyAge`: Human-readable duration/age wrappers for logs. Package: `common/format.go`.

## Transaction Pool

[Reuse] `txpool.LazyTransaction.Resolve()`: Always call to get full tx — never assume `Tx` field is pre-populated. Package: `core/txpool/subpool.go`.

[Reuse] `txpool.ReservationTracker` / `ReservationHandle`: Cross-subpool address ownership. Package: `core/txpool/reserver.go`.

[Reuse] `txpool.TotalTxCost`: Rollup cost calculation — always check overflow bool. Package: `core/txpool/rollup.go`.

## API Utilities

[Reuse] `ethapi.AddrLocker`: Per-address mutex for nonce serialization. Package: `internal/ethapi/addrlock.go`.

[Reuse] `rpc.BlockNumber` with sentinels (`LatestBlockNumber`, `PendingBlockNumber`, `SafeBlockNumber`, `FinalizedBlockNumber`, `EarliestBlockNumber`). Package: `rpc/types.go`.

## Metrics

[Reuse] `metrics.StatsStore` / `metrics.GlobalStatsStore`: TTL-keyed statistics snapshot store. Call `Cleanup()` periodically. Package: `metrics/stats_store.go`.
