---
name: "dependency"
description: "Upstream callers, inter-module deps, storage/middleware, external services"
---
# Dependency Map

## Upstream Callers

| Caller | Protocol | Entry Point |
|--------|----------|------------|
| Beacon/Consensus client (op-node) | HTTP/JSON-RPC (authenticated JWT) | Engine API: `ForkchoiceUpdatedV1/V2/V3/V4`, `NewPayloadV1/V2/V3/V4`, `GetPayloadV1..V6` via `eth/catalyst/api.go` |
| Sequencer RPC (seqRPCService) | HTTP/JSON-RPC | Forwards `eth_sendRawTransaction` / `eth_sendRawTransactionConditional`; configured as `rpc.Client` in `eth/backend.go` |
| Historical RPC service (historicalRPCService) | HTTP/JSON-RPC | Optional external historical node for pre-Bedrock block data; `rpc.Client` in `eth/backend.go` |
| XLayer-Erigon (PP RPC) | HTTP/JSON-RPC | Legacy migration routing for pre-migration blocks; configured via `MigrationConfig.PPRPCUrl` |
| Interop supervisor (interopRPC) | HTTP/gRPC | Cross-chain interop safety checks via `interop.InteropClient` in `core/txpool/ingress_filters.go` |
| P2P devp2p peers | TCP/RLPx (devp2p) | eth and snap protocol peers; `p2p/server.go` and `eth/handler.go` |

## Inter-Module Dependencies

| From | To | Mechanism |
|------|----|-----------|
| `eth` → `core/txpool` | in-process Go interface | handler uses `txPool` interface for tx add/fetch/broadcast |
| `eth` → `eth/downloader` | in-process Go | handler creates `downloader.New`; snap packets via `downloader.DeliverSnapPacket` |
| `eth` → `eth/catalyst` | in-process Go | `catalyst.Register` adds Engine API to node |
| `eth` → `ethdb` | in-process Go interface | `Ethereum.chainDb` typed as `ethdb.Database` |
| `eth` → `core/rawdb` | in-process Go | `ParseStateScheme`, `ReadDatabaseVersion`, `WriteDatabaseVersion` |
| `eth` → `node` | in-process Go | `eth.New` receives `*node.Node`; calls `stack.OpenDatabaseWithOptions` |
| `eth` → `p2p` | in-process Go | `Ethereum` embeds `*p2p.Server` |
| `eth` → `internal/sequencerapi` | in-process Go | `GetSendRawTxConditionalAPI` exposes conditional tx API |
| `eth/catalyst` → `beacon/engine` | in-process Go | `engine.ExecutableData`, `ForkchoiceStateV1`, `PayloadAttributes` types |
| `core/state` → `triedb` | in-process Go | `state.Database.TrieDB()` returns `*triedb.Database` |
| `triedb` → `ethdb` | in-process Go interface | `triedb.Database.disk` is `ethdb.Database` |
| `triedb` → `triedb/pathdb` | in-process Go | pathdb backend for path-based state scheme |
| `triedb` → `triedb/hashdb` | in-process Go | hashdb backend for hash-based state scheme |
| `core/rawdb` → `ethdb` | in-process Go interface | `freezerdb`/`nofreezedb` wrap `ethdb.KeyValueStore` |
| `core/txpool` → `core/txpool/ingress_filters` | in-process Go | `IngressFilter` interface applied on `Add` |
| `node` → `p2p` | in-process Go | Node embeds `*p2p.Server` as networking layer |
| `internal/sequencerapi` → `rpc.Client` (seqRPC) | HTTP/JSON-RPC | Forwards conditional txs to upstream sequencer |
| `ethdb/pebble` → `github.com/cockroachdb/pebble` | external Go module | Pebble KV storage engine |

## Storage and Middleware

| Component | Type | Usage |
|-----------|------|-------|
| LevelDB (`syndtr/goleveldb`) | KV Store | Legacy/compat chain kv storage via `ethdb.KeyValueStore` |
| Pebble (`cockroachdb/pebble`) | KV Store | Primary KV storage backend for chaindata |
| RocksDB (`linxGnu/grocksdb`) | KV Store | Optional KV storage backend |
| Ancient/Freezer (flat files) | Immutable Store | Immutable ancient chain segment storage (blocks, receipts) |
| In-memory DB (`ethdb/memorydb`) | KV Store | Ephemeral test/dev storage |
| Path-based TrieDB (`triedb/pathdb`) | Trie Store | Account/storage trie nodes using path scheme with diff layers |
| Hash-based TrieDB (`triedb/hashdb`) | Trie Store | Account/storage trie nodes using legacy hash scheme |
| State snapshot (`core/state/snapshot`) | Flat State | Flat snapshot of account and storage state for fast access |
| Azure Blob Storage | External Archive | Block/era archive (freezer offload) |
| AWS S3/Route53 | External Storage/DNS | DNS discovery / potential storage backend |

## External Services

| Service | SDK/Client | Purpose |
|---------|-----------|---------|
| InfluxDB v1/v2 | `influxdb-client-go/v2`, `influxdb1-client` | Metrics export (chain, txpool, p2p metrics) |
| OpenTelemetry (OTLP/HTTP) | `go.opentelemetry.io/otel` + exporters | Distributed tracing export |
| Cloudflare DNS | `cloudflare-go` | DNS-based peer discovery (dnsdisc) |
| AWS Route53 | `aws-sdk-go-v2/service/route53` | DNS record management for peer discovery |
| OP Interop Supervisor | `eth/interop.InteropClient` | Cross-chain message safety verification |
| XLayer-Erigon RPC | `rpc.Client` via `MigrationConfig.PPRPCUrl` | Historical/pre-migration block queries |

## Prohibited Patterns

[Rule] `ethdb/database.go`: `AncientStore` operations (`Ancient`, `ModifyAncients`, `TruncateHead`, `TruncateTail`) must never be called on `nofreezedb` — returns `errNotSupported`.

[Rule] `eth/catalyst/api.go`: Engine API endpoints must be registered as `Authenticated: true` — never on public RPC port.

[Rule] `eth/handler_eth.go`: Transaction gossip must be filtered through `txGossipAllowed`; `NoTxGossip`, `TxGossipNetRestrict`, `TxGossipTrustedPeersOnly` flags all gate tx propagation.

[Rule] `p2p/transport_xlayer.go`: ETH69 capability must be stripped for Geth peers before handshake.

[Rule] `core/txpool/ingress_filters.go`: Interop txs must pass supervisor `CheckAccessList` at `CrossUnsafe` safety level; supervisor unavailability = tx rejection.

[Rule] `eth/ethconfig/config_xlayer.go`: Pre-migration blocks must be routed to XLayer-Erigon PP RPC when configured.

[Rule] `internal/sequencerapi/api.go`: When `seqRPC` is set, conditional txs must be forwarded upstream, NOT added locally.
