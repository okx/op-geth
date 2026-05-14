---
name: "internal-ethapi"
description: "Module design for internal/ethapi: public Ethereum JSON-RPC API implementation"
---
# Internal EthAPI Module

## Responsibilities

- Implements `eth_*` JSON-RPC API methods: `eth_call`, `eth_estimateGas`, `eth_sendRawTransaction`, `eth_getBlockByNumber`, `eth_getLogs`, etc.
- Defines `Backend` interface contract between API layer and concrete backends
- Transaction argument construction and validation (`TransactionArgs`)
- Block simulation (`eth_simulateV1`)
- Address locking for nonce serialization (`AddrLocker`)
- OP-Stack additions: `HistoricalRPCService()`, `Genesis()`, header helpers
- Sequencer API: `eth_sendRawTransactionConditional` (in `internal/sequencerapi`)

## NOT Responsible For

- P2P networking or peer management
- Block building or mining
- Core chain logic or EVM execution
- Database I/O (all through `Backend` interface)

## Core Entities

| Entity | Key Fields | Description |
|--------|-----------|-------------|
| `Backend` | interface | Chain/pool access contract for API layer |
| `TransactionAPI` | `b Backend`, `nonceLock *AddrLocker` | eth_sendRawTransaction, eth_call, etc. |
| `BlockChainAPI` | `b Backend` | eth_getBlockByNumber, etc. |
| `TransactionArgs` | `From`, `To`, `Gas`, `GasPrice`, `Value`, `Data` | TX construction args |
| `AddrLocker` | `mu sync.Mutex`, `locks map[common.Address]*sync.Mutex` | Per-address nonce lock |
| `sendRawTxCond` | `b Backend`, `seqRPC *rpc.Client` | Conditional TX submission |

## Dependencies

- Require to reference arch/dependency.md for full dependency details

## Relevant Flows

- Require to reference core-flows/tx-submission.md

## Module-Specific Pitfalls

[Rule] JSON-RPC API layer must only call chain/txpool through the `Backend` interface — never concrete `eth.Ethereum`.

[Rule] Hold `AddrLocker.LockAddr` around entire nonce-read → sign → submit sequence to prevent duplicate-nonce race.

[Pitfall] `TransactionConditional` cost rate-limiter burst hardcoded at 3x max — not configurable. Source: `sequencerapi/api.go:33-34`.

[Pitfall] `checkOptimismPayloadAttributes` will panic if `payloadAttributes` is nil — callers must guarantee non-nil. Source: `eth/catalyst/api_optimism.go:39`.

[Pitfall] EIP1559Params error message has typo: "eip155Params" instead of "eip1559Params". Source: `api_optimism.go:59`.
