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

## Ingress Filtering

[Convention] `txpool.IngressFilter.FilterTx` returns a **bool only** — every rejection from any registered filter collapses to the single generic error `core.ErrTxFilteredOut`. The interface cannot carry a filter-specific reason. To surface a filter-specific RPC error/message to the submitter, do NOT widen the interface: EXTEND the legacypool add loop with a **concrete-type special-case** (e.g. type-assert the filter or check a sentinel and map to a dedicated error like `core.ErrBlacklisted` → RPC `-32000` + text). `legacypool` already imports `core`/`core/txpool`, so this adds no new import edge and stays fork-local. Match the surfaced error with `errors.Is` against a sentinel, never string equality.

[Convention] Register an `IngressFilter` only on the chains/conditions where it applies, and refresh any snapshot it depends on inside the pool `reset` path (commit + reorg) so the filter view tracks the canonical head. Keep the per-tx pass-through allocation-free — it is a high-frequency hot path; do not log per accepted tx.

## OP-Stack Gossip Control

[Convention] Three independent gossip gates: `NoTxGossip` (disable all), `TxGossipTrustedPeersOnly` (only trusted peers), `TxGossipNetRestrict` (IP allowlist). All enforced at handler layer via `txGossipAllowed`. Peers failing checks receive `NilPool`.
