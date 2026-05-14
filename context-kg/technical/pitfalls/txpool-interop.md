---
name: "txpool-interop"
description: "Pitfalls related to transaction pool, interop filtering, and conditional transactions"
---
# Transaction Pool and Interop Pitfalls

[Pitfall] **InteropFilter rejects all txs when supervisor unavailable**: If the interop supervisor API returns an error from `CurrentInteropBlockTime`, `FilterTx` returns false, silently dropping all interop transactions. **Correct approach**: Implement circuit-breaker or emit metric/log when rejecting due to unavailability. Source: `core/txpool/ingress_filters.go:53-55`. Affected module: core/txpool.

[Pitfall] **TransactionConditional cost rate-limiter burst hardcoded at 3x max**: Burst for cost rate limiter is hardcoded as `3*params.TransactionConditionalMaxCost` — not configurable. **Correct approach**: Make burst multiplier configurable via flag. Source: `internal/sequencerapi/api.go:33-34`. Affected module: internal/sequencerapi.

[Pitfall] **TransactionConditional double state-check**: Both latest AND parent block state are checked to remove MEV incentive. A conditional tx valid at latest may be rejected because parent state differs — submitters must satisfy both simultaneously. Source: `internal/sequencerapi/api.go:61-89`. Affected module: internal/sequencerapi.

[Pitfall] **seqRPC nil check controls divergent paths**: When `seqRPC` is set (sequencerapi), tx is proxied upstream and local path is NOT called. When `seqRPC` is nil, tx goes through `SubmitTransaction` which checks `seqRPCService` — two different RPC client fields must both be absent for fully local add. Source: `internal/sequencerapi/api.go:105-128`. Affected module: internal/sequencerapi, eth.

[Pitfall] **TotalTxCost returns (nil, true) on overflow**: If `tx.Cost()` exceeds uint256, returns nil without logging. Callers that don't check overflow bool will dereference nil pointer. **Correct approach**: Always check overflow bool. Source: `core/txpool/rollup.go:38-40`. Affected module: core/txpool.

[Pitfall] **rollupCostFn nil means L1 fee excluded from balance check**: If `RollupCostFunc` is not wired, `TotalTxCost` returns only EVM cost. A tx that looks affordable may fail at block inclusion due to L1 data fee. Source: `core/txpool/rollup.go:37-50`. Affected module: core/txpool.

[Warning] **Interop filter hardcodes 86400s (1 day) preverifier window**: Timeout is not configurable; if preverifier window changes, code change required. Source: `core/txpool/ingress_filters.go`. Affected module: core/txpool.

[Warning] **TransactionConditional checks against parent block state**: Intentional design to remove MEV incentives; conditionals may be accepted on slightly stale state. Source: `internal/sequencerapi/api.go`. Affected module: internal/sequencerapi.
