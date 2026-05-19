---
name: "forkchoice-update"
description: "Core flow: forkchoice update — Engine API head/safe/finalized block management"
---
# Forkchoice Update Flow

## Entry Point

`POST engine_forkchoiceUpdatedV1..V4` | `ConsensusAPI.ForkchoiceUpdatedV{1,2,3,4}` | `ForkchoiceStateV1{HeadBlockHash, SafeBlockHash, FinalizedBlockHash}`, `PayloadAttributes` (optional)

## Primary Entities

`ForkchoiceStateV1`, `PayloadAttributes`, `BlockChain`, `PayloadID`

## State Transitions

| Current State | Trigger | Target State |
|--------------|---------|-------------|
| `BlockChain.currentBlock` old_head | `SetCanonical()` after reorg | HeadBlockHash |
| `BlockChain.currentFinalBlock` nil/old | `SetFinalized()` | FinalizedBlockHash header |
| `BlockChain.currentSafeBlock` nil/old | `SetSafe()` | SafeBlockHash header |
| PayloadQueue empty | `miner.BuildPayload()` when attributes present | has PayloadID |
| Downloader idle | `BeaconSync()` when head unknown | beacon_syncing |

**[Rule] Terminal states must never be reversed**: finalized block hash, once set, must not go backward

## Normal Flow Steps

| Step | Action | Module |
|------|--------|--------|
| 1 | Hardfork-gated attribute validation (withdrawals, BeaconRoot, SlotNumber) | `eth/catalyst/api.go` |
| 2 | Acquire `forkchoiceLock`; validate HeadBlockHash non-zero | `eth/catalyst/api.go` |
| 3 | OP-Stack `checkOptimismPayloadAttributes`: GasLimit, Canyon, Holocene, Isthmus checks | `eth/catalyst/api_optimism.go` |
| 4 | Lookup HeadBlockHash locally; if missing check invalidAncestor/remoteBlocks caches, then peer fetch | `eth/catalyst/api.go` |
| 5 | If block not local: `Downloader.BeaconSync(header, finalized)` → return STATUS_SYNCING | `eth/catalyst/api.go` |
| 6 | Pre-merge reorg guard: reject if block.Difficulty > 0 and parent is post-TTD | `eth/catalyst/api.go` |
| 7 | Check canonical hash vs HeadBlockHash; call `SetCanonical(block)` if needed | `eth/catalyst/api.go` |
| 8 | `SetCanonical`: if state missing → `recoverAncestors()`; reorg if parent != currentBlock; `writeHeadBlock()` | `core/blockchain.go` |
| 9 | OP-specific: allow proposer self-reorg (skip "ignore old head" short-circuit) | `eth/catalyst/api.go` |
| 10 | Call `eth.SetSynced()` — marks node as post-merge synced | `eth/catalyst/api.go` |
| 11 | If FinalizedBlockHash != zero: verify in DB + canonical, `SetFinalized(header)` + `rawdb.WriteFinalizedBlockHash` | `eth/catalyst/api.go` |
| 12 | If SafeBlockHash != zero: verify in DB + canonical, `SetSafe(header)` (in-memory only) | `eth/catalyst/api.go` |
| 13 | If payloadAttributes != nil: extract params, build `BuildPayloadArgs`, call `miner.BuildPayload()` | `eth/catalyst/api.go` |
| 14 | Return `ForkChoiceResponse{VALID, LatestValidHash=HeadBlockHash, PayloadID}` | `eth/catalyst/api.go` |

## Exception Branches

| Trigger | State Change | Compensation |
|---------|-------------|-------------|
| HeadBlockHash unknown + not in remoteBlocks | No state change | Peer fetch + STATUS_SYNCING |
| Known invalid ancestor | No state change | Return cached invalid status |
| Pre-merge block (PoW) | No state change | Return INVALID_TERMINAL_BLOCK |
| FinalizedBlockHash not in DB/canonical | Head may already be updated | Return STATUS_INVALID |
| SafeBlockHash not in DB/canonical | Head+finalized updated | Return STATUS_INVALID |
| `SetCanonical` fails (state recovery fails) | Returns INVALID | Chain stays at previous head |
| `miner.BuildPayload` fails | Head/safe/finalized still updated | Return valid(nil) + InvalidPayloadAttributes |
| Duplicate PayloadID in localBlocks | No new build | Return valid(&id) immediately |

## Flow-Specific Pitfalls

[Pitfall] OP-Stack proposer reorg: `cfg.Optimism != nil` bypasses "ignore old head" — Optimism always calls `SetCanonical` even for already-canonical blocks. Source: `eth/catalyst/api.go:333-338`.

[Pitfall] Safe block not persisted to rawdb: `SetSafe` only in-memory; unlike `SetFinalized` which writes to rawdb. Lost on restart. Source: `core/blockchain.go:828-835`.

[Pitfall] Head updated before finalized/safe validation: `SetCanonical` executes before FinalizedBlockHash/SafeBlockHash validated. Bad safe/finalized causes INVALID but head already moved. Source: `eth/catalyst/api.go:326-369`.

[Pitfall] `forkchoiceLock` does NOT guard payload construction race: `BuildPayload` called inside lock but miner runs async; `localBlocks.has(id)` dedup is the only guard. Source: `eth/catalyst/api.go:406-408`.
