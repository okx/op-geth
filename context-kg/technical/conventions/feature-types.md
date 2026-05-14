---
name: "feature-types"
description: "Base patterns, feature type conventions, and reusable architectural patterns"
---
# Feature Types and Base Patterns

## Base Patterns

| Pattern | Base Class/Interface | Description |
|---------|---------------------|-------------|
| Generic-LRU-with-mutex | `lru.Cache[K,V]` wrapping `BasicLRU[K,V]` + `sync.Mutex` | Thread-safe caching: outer Cache holds mutex, all methods lock before delegating to BasicLRU |
| Size-Constrained-Content-Addressed-Cache | `lru.SizeConstrainedCache[K,V blobType]` | Byte-capacity cache that evicts oldest; assumes content-addressing; value not copied on Add |
| Clock-Interface-for-Testability | `mclock.Clock` / `mclock.System{}` | Injectable clock: production structs accept `mclock.Clock`; tests inject simulated clock |
| Event-Feed-Subscription | `event.Feed` + buffered channel + `Subscription.Unsubscribe` | Pub-sub: producer calls `Feed.Send(value)`; consumers subscribe with buffered channels |
| Lazy-Resolution | `txpool.LazyTransaction.Resolve()` | Deferred fetch: store hash + pool ref in hot paths; call Resolve() when full tx needed |
| Cross-SubPool-Reservation | `txpool.ReservationTracker` + `ReservationHandle` | Shared ownership: each subpool gets unique ID; `Hold`/`Release` prevent double-reservation |
| RPC-Error-Interface | `rpc.Error` + `rpc.DataError` | Custom errors implement `Error()` + `ErrorCode()`; extra data via `ErrorData()` |
| Hex-JSON-Type-Wrapper | `hexutil.Bytes`/`Big`/`Uint`/`Uint64` | All binary/numeric RPC fields use hexutil wrappers for 0x-prefixed hex JSON |
| StateDB-Interface | `core/vm.StateDB` | EVM state via interface only — enables tracing, stateless, test backends |
| TTL-Store | `metrics.StatsStore` | Mutex-protected map with TTL; `Cleanup()` is caller-driven; nil receiver safe |

## Pagination / Import / Export Patterns

[Convention] `eth_getLogs` uses block range `[fromBlock, toBlock]` with `RangeLimit` enforcement. No cursor-based pagination. XLayer hybrid splits at `MigrationBlock`.

[Convention] `eth_simulateV1` uses bounded block array (`maxSimulateBlocks=256`, `maxSimulateTotalCalls=10000`). Single request, no continuation.

[Convention] `engine_getPayloadBodiesByRange` uses `(start, count)` range parameters with internal count cap.

## Async / Background Patterns

[Convention] Block building uses conditional broadcast (`sync.Cond`) for payload availability signaling between `buildPayload` goroutine and `Resolve()` caller.

[Convention] `statePrefetcher.Prefetch` runs in background goroutine with atomic interrupt flag; parent sets `interrupt.Store(true)` on completion to terminate prefetch workers.

[Convention] `event.Feed` subscription pattern for chain events: `ChainHeadEvent`, `ChainSideEvent`, `NewTxsEvent`, `LogsEvent`. All subscribers must use buffered channels and call `Unsubscribe()`.
