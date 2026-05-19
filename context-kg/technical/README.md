---
name: "README"
description: "Directory index and reading guide for this knowledge base"
---
# context-kg — Knowledge Base

OP Stack (Optimism) fork of go-ethereum for XLayer L2 chain (mainnet chainID 196, testnet chainID 1952). Go 1.24, no framework — pure Go with devp2p networking, LevelDB/Pebble/RocksDB storage, and OP Stack Engine API integration.

## Files and Directories

| Path | Description |
|------|-------------|
| knowledge-base.md | **Highest authority** — all Skills defer to this on conflicts |
| terminology.md | Domain term glossary for unified terminology |
| arch/architecture-overview.md | Layer definitions, service responsibilities |
| arch/dependency.md | Upstream callers, inter-module deps, storage, external services |
| modules/core.md | Core blockchain: state, txpool, VM, rawdb |
| modules/eth.md | Ethereum service: backend, handler, catalyst, downloader |
| modules/consensus.md | Consensus engine interface and implementations |
| modules/miner.md | Block building and payload assembly |
| modules/node.md | Node lifecycle, RPC server hosting |
| modules/p2p.md | P2P networking, peer discovery, transport |
| modules/rpc.md | JSON-RPC framework |
| modules/trie-triedb.md | Merkle Patricia Trie and trie database |
| modules/ethdb.md | Database abstraction layer |
| modules/params.md | Chain parameters, fork configurations |
| modules/common.md | Common utilities, types, data structures |
| modules/internal-ethapi.md | Public Ethereum JSON-RPC API implementation |
| modules/superchain.md | OP Stack superchain configuration |
| modules/cmd-geth.md | Main geth binary and CLI tooling |
| pitfalls/xlayer-migration.md | XLayer migration routing pitfalls |
| pitfalls/op-stack-hardforks.md | OP Stack hardfork validation pitfalls |
| pitfalls/state-management.md | State/trie management pitfalls |
| pitfalls/p2p-compatibility.md | P2P protocol compatibility pitfalls |
| pitfalls/txpool-interop.md | Transaction pool and interop pitfalls |
| pitfalls/cmd-geth-flag-wiring.md | CLI flag wiring pitfalls (flag defined but not connected to startup) |
| core-flows/block-building.md | Sequencer block building via Engine API |
| core-flows/block-import.md | Block import via NewPayload |
| core-flows/tx-submission.md | Transaction submission and gossip |
| core-flows/forkchoice-update.md | Forkchoice update and head management |
| apis/rest-api-conventions.md | JSON-RPC response format, versioning, pagination |
| apis/error-codes.md | Error code registry |
| conventions/feature-types.md | Base patterns and feature type conventions |
| conventions/service-patterns.md | Service-level patterns (events, caching, locking) |
| conventions/common-tools.md | Must-reuse common components |

## How to Read This Knowledge Base

1. **Knowledge base is the highest authority** — defer to it over general AI knowledge
2. **Locate the specific module** — read the module doc before starting work
3. **Check pitfalls and core flows first**
4. **Produce a constraint checklist** — explicitly declare if no relevant content
5. **Cross-validate during work** — correct violations immediately
