---
name: "knowledge-base"
description: "Highest-authority rules — all Skills defer to this on conflicts"
---
# Knowledge Base

> This is the highest-weight file in the knowledge base. All Skills that support
> context-kg defer to this file when conflicts arise with general AI knowledge.

## Data Type Constraints

[Rule] `beacon/engine/types.go`: All numeric fields in JSON-RPC wire format must use `hexutil.Big`, `hexutil.Uint64`, or `hexutil.Uint` wrapper types — never raw integers. Reason: JSON-RPC spec requires 0x-prefixed hex encoding for all numeric values.

[Rule] `common/hexutil/hexutil.go`: All hex strings in JSON-RPC wire format must carry the "0x" prefix; encode with `hexutil.Encode`, decode with `hexutil.Decode`. Reason: Ethereum JSON-RPC specification mandates 0x prefix.

[Rule] `common/hexutil/hexutil.go`: `hexutil.MustDecode` must never be used outside of package-level var/const initialisation — it panics on malformed input. Reason: runtime panics from untrusted input.

[Rule] `common/lru/basiclru.go`: The zero value of `BasicLRU` must never be used — always construct with `NewBasicLRU(capacity)`. Reason: zero-capacity LRU panics on insert.

[Rule] `common/lru/blob_lru.go`: Values must never be mutated after passing to `SizeConstrainedCache.Add` — the cache holds the original slice without copying. Reason: cache corruption from shared references.

[Rule] `core/txpool/rollup.go`: The overflow bool returned by `TotalTxCost` must always be checked — an overflow means the transaction cost exceeds uint256 and must be rejected. Reason: nil pointer dereference on unchecked overflow.

[Rule] `core/types/block_config.go`: `BlockConfig.IsIsthmusEnabled` must be checked before accessing Isthmus-specific receipt or withdrawal fields. Reason: fields are nil pre-Isthmus.

## Naming Constraints

[Rule] `core/blockchain_okx.go`: XLayer-specific additions to core must be in a separate `_okx.go` file. Reason: maintain clean upstream merge boundary.

[Rule] `core/blockchain_optimism.go`: OP-Stack blockchain additions must be in a separate `_optimism.go` file. Reason: maintain clean upstream merge boundary.

[Rule] `eth/api_backend_op.go`: OP-Stack additions to `EthAPIBackend` must be in a separate `_op.go` file. Reason: maintain clean upstream merge boundary.

[Rule] `params/config_xlayer.go`: XLayer-specific chain parameters must be in `config_xlayer.go` files. Reason: maintain clean upstream merge boundary.

[Rule] File naming convention: OP Stack extensions use `_op`, `_optimism`, `_okx`, or `_xlayer` suffix — must never mix upstream logic into these files or vice versa. Reason: simplifies rebasing against upstream go-ethereum and op-geth.

## Dependency Constraints

[Rule] `consensus/consensus.go`: Consensus engine implementations must never call `eth`, `miner`, or `rpc` packages. Reason: consensus is a pure algorithm layer; importing service packages creates circular dependencies.

[Rule] `core/state_prefetcher.go`: `statePrefetcher` must never commit state changes; execution results are discarded. Reason: prefetcher runs speculatively in parallel; committing would corrupt canonical state.

[Rule] `eth/catalyst/api.go`: Engine API endpoints must be registered as `Authenticated: true` on the node RPC server (authenticated JWT port only). Reason: Engine API controls block production; unauthenticated access enables chain manipulation.

[Rule] `eth/handler.go`: OP-Stack tx gossip restrictions (`NoTxGossip`, `TxGossipNetRestrict`, `TxGossipTrustedPeersOnly`) must always be enforced at the handler layer, not in txpool. Reason: gossip filtering is a network-level concern.

[Rule] `ethdb/database.go`: `ethdb` package must never import any higher-layer package; it defines pure storage interfaces. Reason: `ethdb` is the lowest abstraction layer; importing higher packages creates circular dependencies.

[Rule] `internal/ethapi/backend.go`: JSON-RPC API layer must only call chain/txpool through the `Backend` interface, never concrete `eth.Ethereum`. Reason: decouples API from service implementation; enables light client and test backends.

[Rule] `core/txpool/ingress_filters.go`: Transactions with interop access list entries must pass interop supervisor `CheckAccessList` at `CrossUnsafe` safety level; if supervisor unavailable, tx must be rejected. Reason: interop safety requires external validation.

[Rule] `core/vm/interface.go`: EVM state must be accessed exclusively through the `vm.StateDB` interface — never use a concrete state type in VM/opcode code. Reason: enables tracing, stateless execution, and test backends.

[Rule] `p2p/transport_xlayer.go`: When `--p2p.eth69-compat=true` (default), ETH69 capability is stripped for Geth peers before handshake via `trimETH69Counted`. When set to false, no stripping occurs and eth/69 is negotiated normally. Counter metric `p2p/handshake/eth69/trimmed` tracks connections where trim fired. Reason: ETH69 protocol compatibility issue with standard Geth nodes; flag allows safe rollout of eth/69 once upstream interop is verified.

[Rule] `triedb/database.go`: TrieDB backend selection (hashdb vs pathdb) is determined at init; must never switch scheme on existing database. Reason: schema mismatch causes data corruption.

## Security Constraints

[Rule] `eth/catalyst/api.go`: Engine API must always be registered on the authenticated RPC endpoint only — never expose on public HTTP/WS. Reason: Engine API controls block production and chain head.

[Rule] `internal/sequencerapi/api.go`: `eth_sendRawTransactionConditional` must validate conditional cost <= `TransactionConditionalMaxCost` before state checks. Reason: rate-limiting prevents DoS via expensive state lookups.

[Rule] `internal/sequencerapi/api.go`: When `seqRPC` is set, conditional txs must be forwarded to upstream sequencer and NOT added to local txpool. Reason: prevents duplicate inclusion and MEV exploitation.

[Rule] `eth/ethconfig/config_xlayer.go`: Pre-migration blocks (block height < `MigrationBlock`) must be routed to XLayer-Erigon PP RPC when `MigrationBlock` and `PPRPCUrl` are configured. Reason: local node does not have pre-migration state.

[Rule] `core/txpool/validation.go`: `DepositTxType` must never be accepted through the public txpool — only through Engine API forced transactions. Reason: deposit spam protection; deposits are L1-originated.

[Rule] `eth/handler_eth.go`: Transaction gossip must be filtered through `txGossipAllowed`; peers failing the check receive a `NilPool` that drops all transactions silently. Reason: OP Stack sequencer controls tx propagation.

[Rule] No credentials, API keys, or internal service addresses must appear in generated knowledge base files — reference by type only (e.g., "DB password configured via environment variable"). Reason: security policy.
