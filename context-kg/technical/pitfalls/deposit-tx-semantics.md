---
name: "deposit-tx-semantics"
description: "Pitfall: reverting/intercepting an OP-Stack deposit transaction must reproduce the canonical failed-deposit post-state (keep-mint + nonce+1), not a blanket snapshot revert"
---
# OP-Stack Deposit Transaction Semantics Pitfalls

[Pitfall] **A blanket `RevertToSnapshot` on an intercepted deposit forks consensus by undoing the mandatory nonce bump and the mint**: OP-Stack defines a specific *failed-deposit* post-state. In `core/state_transition.go:474-512` the mint is applied to `msg.From`, a **post-mint snapshot** is taken, and on failure only the post-mint call effects are rewound — the **mint is kept** and the depositor **nonce is always incremented** (`:494`, "always increment the nonce for the next deposit transaction"). If a feature intercepts a deposit and simply snapshots before `ApplyMessage` then reverts the whole thing, it wipes the mint (balance → 0) and the nonce bump (nonce stays N), producing a state root + receipt that diverge from canonical OP-Stack / other clients. This is consensus-critical and is NOT caught by the compiler, vet, or a passing build — only by a cross-client / canonical-path state-root assertion.

**Trigger**: Intercepting, gating, or otherwise force-failing a deposit transaction (`tx.IsDepositTx()`) and reverting its effects.

**Correct**: Reproduce the canonical failed-deposit post-state explicitly: re-apply the mint (KEEP-MINT), re-increment the nonce to N+1, force `status=0`, `gasUsed=tx.Gas()`, and charge the full gas limit. Capture the pre-`ApplyMessage` nonce so the bump is `nonce+1` regardless of the revert.

```go
nonce := statedb.GetNonce(msg.From)        // capture BEFORE ApplyMessage
// ... ApplyMessage runs, then a committed blacklist hit is detected ...
statedb.RevertToSnapshot(snapID)           // undo mint + nonce + call effects
if tx.IsDepositTx() {
    if msg.Mint != nil {
        statedb.AddBalance(msg.From, mintU256, tracing.BalanceMint) // KEEP-MINT (matches state_transition.go:479)
    }
    statedb.SetNonce(msg.From, nonce+1, tracing.NonceChangeEoACall) // always +1 (matches :494)
    if extra := tx.Gas() - result.UsedGas; extra > 0 { _ = gp.SubGas(extra) } // full gasLimit
}
receipt.Status = types.ReceiptStatusFailed
if tx.IsDepositTx() { receipt.GasUsed = tx.Gas() } // explicit override
```

[Rule] When force-failing/reverting a deposit transaction, KEEP the mint and SET nonce to N+1 (canonical OP-Stack failed-deposit post-state); never let a blanket pre-`ApplyMessage` revert erase the mint or nonce bump. Any client replicating the behaviour must replicate the exact same choice — assert state-root equality with shared cross-client vectors.

**Module**: core (`core/blacklist_gate_xlayer.go:209-269`), state-transition (`core/state_transition.go:474-512`).
**Source**: regression test `core/blacklist_deposit_xlayer_test.go::TestDeposit_BlacklistedHit`.
**Date**: 2026-06-11
**Hit count**: 1
