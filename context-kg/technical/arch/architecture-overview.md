---
name: "architecture-overview"
description: "Layer definitions, allowed/prohibited call directions, service responsibilities"
---
# Architecture Overview

## Layer Definitions

| Layer | Responsibilities | Allowed Calls | Prohibited Calls |
|-------|-----------------|---------------|-----------------|
| CLI/Entry (`cmd/geth`) | Flag definitions, node initialization, OP-Stack hardfork override flags | `node.New`, registers backends | Must not contain business logic |
| Node/Infrastructure (`node`) | Lifecycle management, RPC servers (HTTP/WS/IPC/Auth), p2p.Server, account manager | Calls `p2p`, opens databases | Must not call `core` directly |
| Ethereum Service (`eth`) | Wires BlockChain, TxPool, Miner, handler, APIBackend together | Calls `core`, `miner`, `p2p` | Must not bypass `Backend` interface for RPC |
| P2P Network (`p2p`) | Peer connections, discovery (discv4/v5), dial scheduling, RLPx transport | Internal only | Must not call `core` or `eth` |
| Consensus Engine (`consensus`) | Defines `Engine` interface (VerifyHeader, Finalize, Seal, etc.) | Called by `core` | Must not call `eth`, `miner`, or `rpc` |
| Core Chain (`core`) | BlockChain, state, txpool, VM execution, state prefetcher, rawdb | Calls `ethdb`, `consensus`, `trie` | Must not call `eth` or `rpc` |
| Storage (`ethdb`) | Defines `KeyValueStore`/`Database` interfaces (pure I/O) | Internal only | Must not call any higher-layer package |
| RPC Framework (`rpc`) | Service registry, reflection-based dispatch | Internal only | Must not contain chain logic |
| Public JSON-RPC API (`internal/ethapi`) | Implements `eth_*` API methods via `Backend` interface | Must only call `Backend` interface | Must not call concrete `eth.Ethereum` |
| Engine/Catalyst API (`eth/catalyst`) | Implements `engine_*` namespace (OP-Stack Engine API) | Calls `eth.Ethereum` and `miner` directly | Must register on authenticated RPC port only |
| GraphQL API (`graphql`) | HTTP handler wrapping graphql-go schema | Calls `ethapi.Backend` and `eth/filters` | Must not implement chain logic |
| Superchain Config (`superchain`) | Loads per-network superchain TOML configs | Read-only config data | Must not call `core` or `eth` |
| OP-Stack Extensions | `*_op.go`, `*_optimism.go`, `*_okx.go`, `*_xlayer.go` files | Layered on standard types | Must not break upstream interfaces |

## Service Responsibilities

| Module | Responsibility | NOT Responsible For |
|--------|---------------|---------------------|
| `eth.Ethereum` | Full node service: owns BlockChain, TxPool, BlobPool, Miner, handler, APIBackend, p2p.Server, consensus.Engine, OP-Stack seqRPC/historicalRPC/interopRPC clients, XLayer legacy RPC routing | Does not expose JSON-RPC directly; does not implement consensus algorithm |
| `eth.handler` | P2P protocol handler: peers set, downloader, txFetcher, block/tx broadcast, OP-Stack tx gossip restrictions | Does not serve JSON-RPC; does not write to chain directly |
| `eth.EthAPIBackend` | Bridges JSON-RPC layer to `eth.Ethereum` fields; adds `HistoricalRPCService()` and `Genesis()` for OP-Stack | Does not contain P2P or mining logic |
| `core.BlockChain` | Canonical chain management: block import/validation/reorg, state commitment, snapshot, history pruning | Does not serve RPC; does not manage peers |
| `core.blockchain_reader` | Read-only accessors (CurrentBlock, CurrentFinalBlock, GetHeader*, etc.) | Does not write state |
| `core.blockchain_optimism` | OP-Stack metrics (baseFee, gasUsed, blobGasUsed gauges/histograms) updated on new head | Does not add chain logic; metrics only |
| `core.blockchain_okx` | XLayer-specific block statistics logging using `metrics.LogStatistics` | Does not add chain logic; logging/metrics only |
| `core.statePrefetcher` | Pre-warms state cache by executing block txs speculatively in parallel before import | Discards all state changes; must not commit state |
| `node.Node` | Service container: lifecycle, RPC server setup, account manager, database opener | Does not implement chain or consensus |
| `miner.Miner` | Block building: assembles pending blocks from txpool, drives `consensus.Engine.Seal`, OP-Stack interop failsafe | Does not validate imported blocks; does not manage peers |
| `consensus.Engine` | Interface only: VerifyHeader, VerifyUncles, Prepare, Finalize, Seal | Does not access database directly |
| `rpc.serviceRegistry` | Reflection-based RPC service registration and dispatch | Does not perform authentication; does not know Ethereum types |
| `internal/ethapi.Backend` | Interface contract between JSON-RPC API and concrete node backends | Not a concrete implementation |
| `graphql.handler` | HTTP handler for GraphQL queries; depth-limited to 20 | Does not implement chain logic |
| `superchain.Superchain` | Loads/caches per-network superchain config from embedded TOML | Does not call chain; read-only |
| `p2p.Server` | TCP/UDP peer connections, RLPx handshake, protocol multiplexing, discovery | Does not know about Ethereum block/tx types |
| `ethdb.KeyValueStore` | Pure storage interface (Has/Get/Put/Delete/Compact/Stat) | Must not call any higher-layer package |
| `eth/catalyst.ConsensusAPI` | Engine API for CL-EL communication; OP-Stack payload/attribute validation | Registered on authenticated port only |
| `cmd/geth.main` | CLI entry point: all node, txpool, sync, OP-Stack hardfork override flags | Does not implement any service logic |
