---
name: "rest-api-conventions"
description: "JSON-RPC response format, versioning strategy, idempotency, and pagination conventions"
---
# REST API Conventions

## Response Format

| Wrapper | Fields | Success Convention | Error Convention |
|---------|--------|-------------------|-----------------|
| `rpc.jsonError` | `code` int, `message` string, `data` interface{} | Method return value serialized as JSON `result` field | `jsonError` in `error` field; `DataError` adds `data` sub-field |
| `RPCTransaction` | `blockHash`, `blockNumber`, `from`, `gas`, `hash`, `nonce`, `to`, `value`, `type`, `depositNonce` (OP), `depositReceiptVersion` (OP) | Pointer returned; nil = not found | Non-nil error |
| `feeHistoryResult` | `oldestBlock`, `reward`, `baseFeePerGas`, `gasUsedRatio`, `baseFeePerBlobGas`, `blobGasUsedRatio` | Struct pointer | Nil result + error |
| `simCallResult` | `returnData`, `logs`, `gasUsed`, `status`, `error` | Array of `simBlockResult` | Per-call `callError` embedded inline, not as RPC error |
| `accessListResult` | `accessList`, `error` string, `gasUsed` | Struct pointer | Error string inline + top-level error |
| `engine.PayloadStatusV1` | `status` (VALID/INVALID/SYNCING/ACCEPTED), `latestValidHash`, `validationError` | status=VALID | status=INVALID + validationError |
| `engine.ForkChoiceResponse` | `payloadStatus`, `payloadId` | Authenticated endpoint only | Non-nil Go error |
| GraphQL | `data`, `errors` per GraphQL spec | `data` populated | `errors` array |

[Rule] All numeric fields must use `hexutil.Big`, `hexutil.Uint64`, or `hexutil.Uint` — never raw integers in JSON-RPC.

## Versioning

| Strategy | Pattern | Breaking Change Rule |
|----------|---------|---------------------|
| Method name suffix | `engine_forkchoiceUpdatedV1..V4`, `engine_newPayloadV1..V5`, `engine_getPayloadV1..V6` | New version creates new V(n+1) method; old versions remain registered |
| None (eth namespace) | `eth_call`, `eth_getBlockByNumber` — no version suffix | Methods evolve via optional new fields |
| Authenticated | `engine_*` requires `Authenticated: true` on `rpc.API` | All non-engine namespaces unauthenticated |
| Version field (deprecated) | `rpc.API.Version` exists but marked deprecated | No active enforcement |

## Idempotency

| Key | Scope | Behavior |
|-----|-------|----------|
| Transaction hash | `eth_sendRawTransaction` | Resubmitting same tx returns same hash; txpool deduplicates by hash |
| `PayloadID` | `engine_getPayload*` | Build jobs keyed by PayloadID; identical ID returns cached payload |
| Filter ID (`rpc.ID`) | `eth_newFilter` / `eth_newBlockFilter` | Filters expire via timeout; `eth_uninstallFilter` removes explicitly |
| None | `eth_call`, `eth_estimateGas`, `eth_simulateV1` | Stateless reads, safe to retry |
| `TransactionConditional` | `eth_sendRawTransactionConditional` | Cost-rate-limited; not idempotent but conditional prevents MEV |

## Pagination

| Endpoint | Parameters | Limits |
|----------|-----------|--------|
| `eth_getLogs` | `fromBlock`, `toBlock` (block range) | `RangeLimit` enforced; `logQueryLimit` per address/topic; `maxTopics=4`, `maxSubTopics=1000`; XLayer splits at `MigrationBlock` |
| `eth_getFilterChanges` | polling by filter ID | Returns changes since last poll; filter expires after timeout |
| `engine_getPayloadBodiesByRangeV1/V2` | `start`, `count` (`hexutil.Uint64`) | Range-based; count capped internally |
| `eth_simulateV1` | block array | `maxSimulateBlocks=256`, `maxSimulateCallsPerBlock=5000`, `maxSimulateTotalCalls=10000` |
| `eth_getStorageValues` | address→slots map | `maxGetStorageSlots=1024` total |
| `eth_getProof` | storage keys array | `maxGetProofKeys=1024` |
| `txpool_content` / `txpool_inspect` | none | Flat dump of entire pool; no pagination |
