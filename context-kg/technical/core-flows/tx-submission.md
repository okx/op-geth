---
name: "tx-submission"
description: "Core flow: transaction submission — from JSON-RPC entry through pool addition and P2P gossip"
---
# Transaction Submission Flow

## Entry Point

`POST /rpc eth_sendRawTransaction` | `TransactionAPI.SendRawTransaction` | `hexutil.Bytes` (RLP tx)
`POST /rpc eth_sendRawTransactionConditional` | `sendRawTxCond.SendRawTransactionConditional` | `types.Transaction` + `types.TransactionConditional`

## Primary Entities

`Transaction`, `TransactionConditional`, `TxPool`, `SubPool`, `IngressFilter`

## State Transitions

| Current State | Trigger | Target State |
|--------------|---------|-------------|
| received | `tx.UnmarshalBinary()` | decoded |
| decoded | `checkTxFee` + EIP-155 guard | fee_checked |
| fee_checked (conditional) | `cond.Validate` + state checks | conditional_validated |
| conditional_validated | `seqRPC != nil` | forwarded_to_upstream (terminal) |
| fee_checked | `seqRPCService != nil` | forwarded_to_sequencer |
| forwarded_to_sequencer | `txPool.Add` | pool_added |
| pool_added | `IngressFilter.FilterTx` | ingress_filtered |
| ingress_filtered | `ValidateTransaction` | validated |
| validated | subpool queue | queued |
| queued | nonce gap resolved | pending |
| pending | `NewTxsEvent` → `BroadcastTransactions` | gossiped |
| pending | block producer picks | included (terminal) |

**[Rule] Terminal states must never be reversed**: forwarded_to_upstream, included

## Normal Flow Steps

| Step | Action | Module |
|------|--------|--------|
| 1 | Decode RLP bytes; convert blob sidecar V0→V1 | `internal/ethapi/api.go` |
| 2 | Log via monitor; check `RPCTxFeeCap`; check EIP-155 protection | `internal/ethapi/api.go` |
| 3 | If `seqRPCService != nil`, forward via `eth_sendRawTransaction` to upstream sequencer; blob txs rejected on Optimism | `eth/api_backend.go` |
| 4 | Call `txPool.Add([tx], sync=false)`; if `localTxTracker` configured, track for resubmission | `eth/api_backend.go` |
| 5 | Fan-out: iterate subpools, first matching `Filter(tx)` wins | `core/txpool/txpool.go` |
| 6 | Interop filter: txs with interop access list checked via supervisor at `CrossUnsafe` safety | `core/txpool/ingress_filters.go` |
| 7 | `ValidateTransaction`: type, size, fork rules, initcode, gas vs intrinsic, sig recovery. OP-Stack subtracts `l1InfoGasOverhead` (70,000) from effective gas limit | `core/txpool/validation.go` |
| 8 | `TotalTxCost`: EVM cost + `RollupCostFunc(tx)` (L1 data fee + operator fee) | `core/txpool/rollup.go` |
| 9 | Subpool add: nonce, balance checks; dedup; underpricing eviction; enqueue or promote | `core/txpool/legacypool` |
| 10 | `txBroadcastLoop`: subscribes to `NewTxsEvent`; calls `BroadcastTransactions` | `eth/handler.go` |
| 11 | Plain txs: direct send to sqrt-of-peers; announce to remaining. Blob/large txs: announce only | `eth/handler.go` |
| 12 | Gossip gated by `txGossipAllowed`: `noTxGossip`, `txGossipTrustedPeersOnly`, `txGossipNetRestrict` | `eth/handler_eth.go` |

## Exception Branches

| Trigger | State Change | Compensation |
|---------|-------------|-------------|
| Conditional cost > max | Rejected pre-pool | `TransactionConditionalCostExceededMaxErrCode` |
| Conditional validation fails | Rejected pre-pool | `TransactionConditionalRejectedErrCode` |
| `seqRPC != nil` (sequencerapi) | Forwarded, not local | Local pool untouched |
| `disableTxPool=true` | Only forward | Local mempool bypassed |
| Blob tx on Optimism | Immediate reject | `ErrTxTypeNotSupported` |
| `DepositTxType` in pool | Immediate reject | Only valid via Engine API |
| Interop supervisor unavailable | `FilterTx` returns false | Tx silently dropped |
| `ErrAlreadyKnown` | Dedup | Error returned |
| `ErrUnderpriced` | Eviction | Error + tracked in underpricedSet |
| Local tx temporary reject | `localTxTracker.Track(tx)` | Resubmitted later; nil returned to caller |
| `noTxGossip=true` | `NilPool` to peer | Inbound/outbound gossip blocked |

## Flow-Specific Pitfalls

[Pitfall] `seqRPCService` forward + local pool add both run: tx forwarded then added locally; local failure is logged not returned — caller may assume local pool has tx when it doesn't. Source: `eth/api_backend.go:332-358`.

[Pitfall] `EffectiveGasLimit` subtracts `l1InfoGasOverhead` unconditionally on Optimism: tx with gas just below block limit but above (limit - 70000) rejected. Source: `core/txpool/validation.go:45-57`.

[Pitfall] `rollupCostFn` nil means L1 fee excluded from balance check — tx may look affordable but fail at inclusion. Source: `core/txpool/rollup.go:37-50`.

[Pitfall] `txGossipTrustedPeersOnly` + `NilPool`: untrusted peers get `NilPool` returning nil for every `Get`; silent divergence in peer tx sets. Source: `eth/handler_eth.go:52-57`.
