---
name: "core"
description: "Module design for core: blockchain, state, txpool, VM, rawdb — the heart of the Ethereum execution layer"
---
# Core Module

## Responsibilities

- Canonical chain management: block import, validation, reorg, state commitment
- EVM execution: opcode interpreter, gas metering, precompiled contracts
- Transaction pool: validation, subpool management (legacy, blob), rollup cost calculation
- State database: account/storage state access, snapshot layer, code caching
- Raw database: block/receipt/header storage, ancient freezer, chain metadata
- Genesis initialization and XLayer first-block commitment
- OP-Stack extensions: optimism metrics, OKX statistics logging, XLayer genesis/fork overrides

## NOT Responsible For

- Serving JSON-RPC (delegated to `internal/ethapi`)
- Managing P2P peers (delegated to `eth/handler`)
- Block building/mining (delegated to `miner`)
- Consensus algorithm implementation (delegated to `consensus`)

## Core Entities

| Entity | Key Fields | Description |
|--------|-----------|-------------|
| `BlockChain` | `chainDb`, `stateCache`, `triedb`, `hc`, `txLookupCache`, `snaps` | Main chain manager |
| `Block` | `Header`, `Transactions`, `Uncles`, `Withdrawals` | Complete block with body |
| `Header` | `ParentHash`, `Number`, `Root`, `TxHash`, `ReceiptHash`, `BaseFee`, `BlobGasUsed`, `SlotNumber` | Block header |
| `Transaction` | `inner TxData`, `hash`, `from`, `rollupCostData`, `conditional` | Transaction wrapper |
| `Receipt` | `Status`, `CumulativeGasUsed`, `Logs`, `L1Fee`, `OperatorFeeScalar`, `DAFootprintGasScalar` | Execution result |
| `StateDB` | account trie, storage tries, journal, snapshot | EVM state access |
| `TxPool` | subpools (legacy, blob), ingress filters, reserver | Transaction pool manager |
| `EVM` | `BlockContext`, `TxContext`, `StateDB`, `interpreter` | Ethereum Virtual Machine |

## Dependencies

- Require to reference arch/dependency.md for full dependency details

## Relevant Flows

- Require to reference core-flows/ for flows involving this module

## Module-Specific Pitfalls

[Pitfall] `SnapshotWait` default=true in `DefaultConfig` is a dirty hack for testing — never set in production. Source: TODO comment at `blockchain.go:196`.

[Pitfall] `TrieDirtyLimit` also controls pathdb `WriteBufferSize` — naming is misleading. Source: TODO at `blockchain.go:276-279`.

[Pitfall] Deleted-log emission order during reorg is forward (oldest first) instead of reverse — acknowledged as "borked" but kept for legacy API compatibility. Source: `blockchain.go:2641-2644`.

[Pitfall] `TotalTxCost` returns `(nil, true)` on `big.Int` overflow without logging — callers must always check the overflow bool. Source: `txpool/rollup.go:38-40`.

[Pitfall] `EnsureXLayerHardcodedForksInDB` uses `log.Error` for non-error conditions (empty DB, non-XLayer chain) — misleading for operators. Source: `genesis_xlayer.go:20,28`.

[Pitfall] `CommitXLayerFirstBlock` re-hashes genesis header with modified block number — resulting hash differs from genesis hash. Source: `util_xlayer.go:30-34,67`.
