---
name: "miner"
description: "Module design for miner: block building, payload assembly, OP-Stack sequencer integration"
---
# Miner Module

## Responsibilities

- Block building: assembles pending blocks from txpool
- Payload assembly for Engine API: `BuildPayload`, `generateWork`
- Transaction ordering by effective tip (via `transactionsByPriceAndNonce` heap)
- OP-Stack sequencer support: `NoTxPool` mode (forced tx only), deposit tx ordering
- Interop failsafe detection (background polling for `BackendWithInterop`)
- Block sealing via `consensus.Engine.Seal`

## NOT Responsible For

- Validating imported blocks (done by `core.BlockChain`)
- Managing P2P peers
- Serving RPC (Engine API is in `eth/catalyst`)

## Core Entities

| Entity | Key Fields | Description |
|--------|-----------|-------------|
| `Miner` | `txpool`, `chain`, `engine`, `pendingMu`, `lifeCtx` | Block producer |
| `Payload` | `empty`, `full`, `stop`, `cond` | In-progress payload being built |
| `BuildPayloadArgs` | `Parent`, `Timestamp`, `FeeRecipient`, `GasLimit`, `Transactions`, `NoTxPool`, `EIP1559Params`, `MinBaseFee` | Payload build parameters |
| `environment` | `signer`, `state`, `header`, `txs`, `receipts`, `gasPool` | Block building workspace |

## Dependencies

- Require to reference arch/dependency.md for full dependency details

## Relevant Flows

- Require to reference core-flows/block-building.md

## Module-Specific Pitfalls

[Pitfall] Setting `NoTxPool=false` on OP-Stack sequencer launches a background goroutine; if tx pool is empty, empty block is never set and `Resolve` blocks. OP-Stack must always use `NoTxPool=true`. Source: `miner/payload_building.go:418`.

[Pitfall] Miner must not start interop failsafe if backend doesn't implement `BackendWithInterop` — logs warning but continues silently. Source: `miner/miner.go:141-145`.

[Pitfall] Not providing `MinBaseFee` on Jovian+ blocks causes `prepareWork` to return error. Source: `miner/worker.go:404-406`.

[Pitfall] Not providing `EIP1559Params` on Holocene+ blocks or providing them pre-Holocene both cause errors. Source: `miner/worker.go:419-421`.
