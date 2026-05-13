---
name: "service-patterns"
description: "Service-level patterns: event broadcasting, caching, locking, rollup cost calculation"
---
# Service Patterns

## Event Broadcasting

[Convention] Use `event.Feed` (not deprecated `TypeMux`) for all new event broadcasting. Producer calls `Feed.Send(value)`, consumers subscribe with buffered channels. Always call `Subscription.Unsubscribe()` to prevent goroutine/memory leaks.

[Convention] Key chain events: `ChainHeadEvent` (new canonical head), `ChainSideEvent` (reorged blocks), `NewTxsEvent` (new pool transactions), `LogsEvent` (new block logs), `RemovedLogsEvent` (reorg removed logs).

## Caching

[Convention] Use `lru.Cache[K,V]` for thread-safe concurrent caching. Use `lru.BasicLRU[K,V]` only inside already-locked structs.

[Convention] Use `lru.SizeConstrainedCache` for byte-capacity-bounded caches (code cache, blob cache). Never mutate values after insertion.

[Convention] Use `mclock.Clock` interface in structs for injectable monotonic time — enables test simulated clocks.

## Locking

[Convention] Use `ethapi.AddrLocker.LockAddr`/`UnlockAddr` around nonce-read → sign → submit to prevent duplicate-nonce race conditions.

[Convention] `forkchoiceLock` serializes concurrent `ForkchoiceUpdated` calls. `newPayloadLock` serializes concurrent `NewPayload` calls. Both are `sync.Mutex` in `ConsensusAPI`.

[Convention] `txpool.ReservationTracker`: each subpool gets unique ID; `Hold(addr)` reserves address for that subpool; `Release(addr)` frees it. Double-Hold is logged as error and silently ignored for recovery.

## Rollup Cost Calculation

[Convention] `txpool.TotalTxCost(tx, rollupCostFn)` computes total = EVM cost + L1 data fee + operator fee. Always check the overflow bool return. Pass nil `rollupCostFn` when not on rollup.

[Convention] `RollupCostFunc` is wired per-subpool from the `RollupCostFuncProvider` interface. Missing wiring means L1 fees are excluded from balance checks — txs may pass pool validation but fail at execution.

## OP-Stack Gossip Control

[Convention] Three independent gossip gates: `NoTxGossip` (disable all), `TxGossipTrustedPeersOnly` (only trusted peers), `TxGossipNetRestrict` (IP allowlist). All enforced at handler layer via `txGossipAllowed`. Peers failing checks receive `NilPool`.
