---
name: "eth"
description: "Module design for eth: full node service, P2P handler, Engine API catalyst, downloader, filters, tracers"
---
# Eth Module

## Responsibilities

- Full node service (`Ethereum`): wires BlockChain, TxPool, BlobPool, Miner, handler, APIBackend
- P2P protocol handler: peer management, downloader, txFetcher, block/tx broadcast
- Engine API (`catalyst`): `engine_*` namespace for CL-EL communication, OP-Stack payload validation
- API backend (`EthAPIBackend`): bridges JSON-RPC layer to `Ethereum` fields
- Downloader: block/state sync (full, snap)
- Filters: log/event filtering, subscription management
- Tracers: EVM execution tracing (call/prestate/flatcall tracers)
- Gas price oracle and estimator
- OP-Stack extensions: seqRPC forwarding, historicalRPC, interopRPC, XLayer legacy migration routing

## NOT Responsible For

- Core blockchain logic (delegated to `core.BlockChain`)
- EVM execution (delegated to `core/vm`)
- Consensus algorithm (delegated to `consensus`)
- JSON-RPC method implementation (delegated to `internal/ethapi`)

## Core Entities

| Entity | Key Fields | Description |
|--------|-----------|-------------|
| `Ethereum` | `blockchain`, `txPool`, `miner`, `handler`, `chainDb`, `config` | Full node service |
| `handler` | `peers`, `downloader`, `txFetcher`, `noTxGossip`, `txGossipNetRestrict` | P2P handler |
| `EthAPIBackend` | `eth`, `allowUnprotectedTxs` | Backend interface implementation |
| `ConsensusAPI` | `eth`, `localBlocks`, `forkchoiceLock`, `newPayloadLock` | Engine API |
| `XlayerLegacyRPCService` | `migrationBlock`, `ppRPCClient` | XLayer migration RPC routing |

## Dependencies

- Require to reference arch/dependency.md for full dependency details

## Relevant Flows

- Require to reference core-flows/ for flows involving this module

## Module-Specific Pitfalls

[Pitfall] `RegisterXlayerHybridFilterAPI` panics on Erigon connection failure at startup — should return error instead. Source: `cmd/utils/flags_xlayer.go:88`.

[Pitfall] Migration routing `eth_getLogs` error fallback uses string comparison (`err.Error() == "unknown block"`) — fragile against upstream error message changes. Source: `api_legacy_xlayer.go:687`.

[Pitfall] Migration routing: `EarliestBlockNumber` (-5) never proxied to Erigon — returns local first block (migration block) instead of true genesis. Source: `api_legacy_xlayer.go:76-85`.

[Pitfall] In-memory `erigonFilters` map lost on restart — subsequent filter calls for those IDs silently fail. Source: `api_legacy_xlayer.go:514-516`.

[Warning] `MigrationBlock=0` silently disables routing — `shouldProxyByNumber` returns false when `MigrationBlock==0`. Source: `api_legacy_xlayer.go`.
