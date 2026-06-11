---
name: "tracing-committed-effect"
description: "Pitfall: detecting consensus-critical committed value effects via core/tracing.Hooks event summation instead of a final committed balance diff"
---
# Tracing Committed-Effect Detection Pitfalls

[Pitfall] **`core/tracing.Hooks` cannot attribute balance changes to frames, and summing its events over-intercepts reverted/transient transfers**: `OnBalanceChange` carries no call-depth/frame parameter (`core/tracing/hooks.go:225`), and the reverse-value reason (reason=15) is only emitted under `WrapWithJournal` (`core/tracing/hooks.go:331-333`). A detector that sums `OnBalanceChange` deltas therefore counts value transfers that happened inside a sub-frame which later reverted — yielding a "hit" for a transfer that the EVM journaling already cancelled (final committed effect = 0). For a consensus-critical decision (e.g. native-ETH blacklist interception) this over-interception diverges from clients that key off committed state (xlayer-reth), causing a cross-client fork.

**Trigger**: Building a consensus-affecting decision (interception, gating, accounting) on top of `tracing.Hooks` by summing per-event balance deltas, expecting them to reflect the transaction's committed effect.

**Correct**: Compute the **final committed balance diff** from the StateDB before/after `ApplyMessage`, then strip fee reasons so only genuine value transfers remain. A non-zero result is the committed hit; reverted sub-frame transfers are already netted out by the EVM's own journaling.

```go
// balStart captured before ApplyMessage, balEnd after (before any RevertToSnapshot).
// feeDelta sums only fee BalanceChangeReasons {5,6,7} (gas buy/refund, tip/fee recipient).
hit := new(uint256.Int).Sub(balEnd, balStart) // (balEnd − balStart)
hit.Sub(hit, feeDelta)                          // − feeDelta
intercept := !hit.IsZero()                      // committed value moved ⇔ non-zero
// OnBalanceChange is used ONLY to accumulate feeDelta, never to decide the value hit.
```

[Rule] For consensus-critical committed-effect detection, decide on a final committed balance diff (`GetBalance` start/end, fee reasons {5,6,7} stripped). Never sum `tracing.Hooks` balance events to infer committed transfers, and never depend on reason=15 (it requires `WrapWithJournal`).

**Module**: core (`core/blacklist_tracer_xlayer.go`), core/tracing.
**Source**: review-finding F-01 — A-15 Adversarial Review (Context-KG Impact Analysis); A-03 TD §4.4.2; A-08 Code Review R2. Anchors: `core/tracing/hooks.go:225,331-333`, `core/vm/interface.go:36` (`GetBalance`).
**Date**: 2026-06-11
**Hit count**: 1
