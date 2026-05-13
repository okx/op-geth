---
name: "block-import"
description: "Core flow: block import — NewPayload Engine API through state execution and commitment"
---
# Block Import Flow

## Entry Point

`POST engine_newPayloadV3` | `ConsensusAPI.NewPayloadV3` | `ExecutableData`, `[]common.Hash` (versionedHashes), `*common.Hash` (beaconRoot)
`POST engine_newPayloadV4` | `ConsensusAPI.NewPayloadV4` | `ExecutableData`, `[]common.Hash`, `*common.Hash`, `[]hexutil.Bytes` (executionRequests)

## Primary Entities

`ExecutableData`, `types.Block`, `StateDB`, `PayloadStatusV1`

## State Transitions

| Current State | Trigger | Target State |
|--------------|---------|-------------|
| ExecutableData raw | `ExecutableDataToBlock()` | types.Block (verified hash) |
| Block unverified | `GetBlockByHash()` cache hit | already_known (VALID) |
| Block unverified | `checkInvalidAncestor()` hit | invalid_ancestor (INVALID) |
| Block unverified | parent present + state present | pending_import |
| Block unverified | parent/state missing or snap syncing | delayed (SYNCING/ACCEPTED) |
| Block pending_import | `InsertBlockWithoutSetHead()` success | inserted_no_head (VALID) |
| Block pending_import | insert fails | insert_failed (INVALID) |
| statedb open | `processor.Process()` | executed |
| statedb executed | `validator.ValidateState()` | validated |
| statedb validated | `writeBlockWithState()` (setHead=false) | committed |

**[Rule] Terminal states must never be reversed**: already_known, invalid_ancestor, delayed, insert_failed, committed

## Normal Flow Steps

| Step | Action | Module |
|------|--------|--------|
| 1 | NewPayloadVx version guard: check required fields per hardfork | `eth/catalyst/api.go` |
| 2 | OP-Stack validation via `checkOptimismPayload` (Canyon withdrawals, ExtraData, Isthmus withdrawalsRoot) | `eth/catalyst/api_optimism.go` |
| 3 | Acquire `newPayloadLock` (serializes concurrent NewPayload) | `eth/catalyst/api.go` |
| 4 | `ExecutableDataToBlock`: decode txs, verify hash, validate hardfork rules | `beacon/engine/types.go` |
| 5 | Early exits: block in DB (VALID), invalidAncestor (INVALID), parent nil (SYNCING), snap-sync (SYNCING) | `eth/catalyst/api.go` |
| 6 | `InsertBlockWithoutSetHead`: acquire `chainmu`, call `insertChain` with setHead=false | `core/blockchain.go` |
| 7 | Parallel sender recovery + async `VerifyHeaders` | `core/blockchain.go` |
| 8 | `ValidateBody`: uncle hash, tx root, withdrawals root, blob gas, Jovian DA footprint | `core/block_validator.go` |
| 9 | Launch `statePrefetcher.Prefetch` in goroutine (up to 4*NumCPU/5 parallel) | `core/blockchain.go` |
| 10 | `processor.Process()`: apply all txs via EVM, collect receipts | `core/blockchain.go` |
| 11 | `validator.ValidateState()`: gasUsed, bloom, receiptHash, stateRoot, Isthmus withdrawalsHash | `core/block_validator.go` |
| 12 | `writeBlockWithState()`: commit trie nodes + receipts to disk (setHead=false) | `core/blockchain.go` |
| 13 | Return `PayloadStatusV1{Status: VALID, LatestValidHash: &hash}` | `eth/catalyst/api.go` |

## Exception Branches

| Trigger | State Change | Compensation |
|---------|-------------|-------------|
| BlockHash mismatch in `ExecutableDataToBlock` | No state change | Return INVALID immediately |
| OP-Stack rule violation | No state change | `api.invalid()` with error |
| Parent block not found | Block stored in `remoteBlocks` | `delayPayloadImport` → SYNCING |
| Parent state not available | Block stored in `remoteBlocks` | Return ACCEPTED |
| Snap sync active | No state change | Return SYNCING |
| `InsertBlockWithoutSetHead` error | Block hash added to `invalidBlocksHits`/`invalidTipsets` | Return INVALID |
| `ValidateState` root mismatch | `reportBadBlock` logged | Return INVALID |
| `ErrKnownBlock` in `ValidateBody` | Skip re-execution | `writeKnownBlock` if snapshot exists |
| checkInvalidAncestor >= 128 hits | Entry evicted from cache | Block may be re-processed |

## Flow-Specific Pitfalls

[Pitfall] `ForkchoiceUpdated` must follow `NewPayload` to advance canonical head — `InsertBlockWithoutSetHead` does NOT update head. Source: `core/blockchain.go:2753,2780`.

[Pitfall] OP-Stack allows proposer self-reorg (Optimism config skips "ignore old head" guard) — vanilla geth would ignore FCU to already-canonical non-current head. Source: `eth/catalyst/api.go:333-337`.

[Pitfall] Isthmus withdrawalsHash verified against storage root (`L2ToL1MessagePasser`), not `DeriveSha(withdrawals)`. Source: `core/block_validator.go:190-198`.

[Pitfall] Safe block not persisted to rawdb — `SetSafe` only writes in-memory atomic; lost on restart, must be re-sent by op-node. Source: `core/blockchain.go:828-835`.
