---
name: "block-building"
description: "Core flow: block building — sequencer payload assembly via Engine API ForkchoiceUpdated + GetPayload"
---
# Block Building Flow

## Entry Point

`POST /engine` | `ConsensusAPI.ForkchoiceUpdatedV3` | `ForkchoiceStateV1{HeadBlockHash, SafeBlockHash, FinalizedBlockHash}`, `PayloadAttributes{Timestamp, Random, SuggestedFeeRecipient, Withdrawals, BeaconRoot, Transactions, NoTxPool, GasLimit, EIP1559Params, MinBaseFee}`

## Primary Entities

`PayloadAttributes`, `BuildPayloadArgs`, `PayloadID`, `Payload`, `ExecutionPayloadEnvelope`, `Block`

## State Transitions

| Current State | Trigger | Target State |
|--------------|---------|-------------|
| PayloadAttributes nil | ForkchoiceUpdated with non-nil payloadAttributes | BuildPayloadArgs constructed |
| BuildPayloadArgs constructed | `args.Id()` hashes all fields | PayloadID (8-byte SHA256) |
| Payload initializing | NoTxPool=true → `generateWork` sync | empty=full block (OP-Stack sequencer normal path) |
| Payload initializing | NoTxPool=false → background goroutine | building state |
| Payload building | `runBuildIteration` → fees > previous | updated |
| Payload building | `Resolve()` or `endTimer` | stopped |
| Payload stopped | `GetPayloadV3/V4` | resolved → `ExecutionPayloadEnvelope` returned |

**[Rule] Terminal states must never be reversed**: stopped, resolved

## Normal Flow Steps

| Step | Action | Module |
|------|--------|--------|
| 1 | `ForkchoiceUpdatedV3` validates PayloadAttributes fields (fork version gating) | `eth/catalyst/api.go` |
| 2 | `checkOptimismPayloadAttributes`: GasLimit required, Canyon empty-withdrawals, Holocene EIP1559Params | `eth/catalyst/api_optimism.go` |
| 3 | Lookup HeadBlockHash in BlockChain; if missing attempt peer fetch → STATUS_SYNCING | `eth/catalyst/api.go` |
| 4 | `SetCanonical(block)` if not already canonical head; OP-Stack allows proposer reorg | `eth/catalyst/api.go` |
| 5 | `SetFinalized` / `SetSafe` from ForkchoiceState | `eth/catalyst/api.go` |
| 6 | Decode `payloadAttributes.Transactions` (RLP), build `BuildPayloadArgs`, compute PayloadID, dedup via `localBlocks.has(id)` | `eth/catalyst/api.go` |
| 7 | Call `miner.Miner.BuildPayload(ctx, args)` | `eth/catalyst/api.go` |
| 8 | NoTxPool=true: `generateWork` once synchronously with `noTxs=true` and forced txs; store as both empty and full | `miner/payload_building.go` |
| 9 | `prepareWork`: resolves parent, constructs header (Number, GasLimit, BaseFee, EIP-1559 ExtraData for Holocene, MinBaseFee for Jovian) | `miner/worker.go` |
| 10 | Commits forced txs (deposits) first, then pool txs ordered by effective tip if !noTxs | `miner/worker.go` |
| 11 | Computes Prague EIP-6110/7002/7251 requests unless Isthmus (empty requests) | `miner/worker.go` |
| 12 | `engine.FinalizeAndAssemble` seals the block | `miner/worker.go` |
| 13 | `payload.update()` accepts only if fees higher than previous (monotonic) | `miner/payload_building.go` |
| 14 | `GetPayloadV3/V4`: `Resolve()` → waits for full block or falls back to empty; returns `ExecutionPayloadEnvelope` | `eth/catalyst/api.go` |

## Exception Branches

| Trigger | State Change | Compensation |
|---------|-------------|-------------|
| HeadBlockHash unknown locally | No state change | Return STATUS_SYNCING; CL must retry |
| HeadBlockHash previously invalidated | No state change | Return INVALID with LatestValidHash |
| FinalizedBlockHash not canonical | No state change | Return INVALID + InvalidForkChoiceState |
| `checkOptimismPayloadAttributes` fails | No state change | Return STATUS_INVALID + InvalidPayloadAttributes |
| `generateWork` fails | `Payload.err` set | Return valid(nil) with no PayloadID |
| Background build interrupted by `Resolve()` | Ongoing work stops | Block sealed with current state |
| `errSupervisorInFailsafe` | `fillTransactions` aborts interop txs | Block built with non-interop txs only |

## Flow-Specific Pitfalls

[Pitfall] Setting `NoTxPool=false` on OP-Stack sequencer: background goroutine launched; if pool empty, empty block never set and `Resolve` blocks. Always use `NoTxPool=true`. Source: `miner/payload_building.go:418`.

[Pitfall] Passing non-zero `EIP1559Params` before Holocene activation: both `checkOptimismPayloadAttributes` and `prepareWork` return error. Source: `eth/catalyst/api_optimism.go:58-59`.

[Pitfall] Not providing `MinBaseFee` on Jovian+ blocks: `prepareWork` returns "missing minBaseFee" error. Source: `miner/worker.go:404-406`.

[Pitfall] Reusing PayloadID: dedup check short-circuits to `valid(&id)` immediately without new build — correct only if attributes truly identical. Source: `eth/catalyst/api.go:406-408`.

[Pitfall] Calling `GetPayloadV3` with PayloadID from FCU V4: `payloadID.Is(PayloadV3)` fails, returns UnsupportedFork. Source: `eth/catalyst/api.go:532-534`.
