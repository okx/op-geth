---
name: "xlayer-migration"
description: "Pitfalls related to XLayer migration routing between local node and legacy XLayer-Erigon RPC"
---
# XLayer Migration Routing Pitfalls

[Pitfall] **eth_getLogs error fallback uses string comparison**: The fallback from local to Erigon for blockHash queries relies on `err.Error() == "unknown block"` (literal string equality). Any upstream go-ethereum change that rewrites this error string silently breaks the fallback. **Correct approach**: Use `errors.Is()` with a sentinel error variable. Source: `eth/api_legacy_xlayer.go:687`. Affected module: eth.

[Pitfall] **eth_subscribe/Logs silently drops historical pre-migration events**: The Logs subscription handler overwrites `fromBlock` to `migrationBlock` if earlier, emitting only a Warn. Callers subscribing to ranges spanning both chains silently lose pre-migration events. **Correct approach**: Return an error or document that pre-migration subscription windows are not supported. Source: `eth/api_legacy_xlayer.go:735-736`. Affected module: eth.

[Pitfall] **EarliestBlockNumber never proxied to Erigon**: `shouldProxyByNumber` returns false for all negative block numbers including `EarliestBlockNumber` (-5). If local node synced only from migration block, `eth_getBlockByNumber("earliest")` returns the migration block, not true genesis. **Correct approach**: Explicitly handle `EarliestBlockNumber` and proxy to Erigon. Source: `eth/api_legacy_xlayer.go:76-85`. Affected module: eth.

[Pitfall] **In-memory erigonFilters map lost on restart**: `XlayerHybridFilterAPI.erigonFilters` is in-memory; on restart, Erigon-tracked filter IDs are forgotten. Subsequent calls for those IDs silently fail or return incorrect data. **Correct approach**: Document ephemeral nature; filters are inherently per-session per RPC spec. Source: `eth/api_legacy_xlayer.go:514-516`. Affected module: eth.

[Pitfall] **RegisterXlayerHybridFilterAPI panics on Erigon connection failure**: If `PPRPCUrl` is set and Erigon dial fails at startup, the function calls `panic(err)` rather than returning a graceful error. **Correct approach**: Replace panic with startup error return. Source: `cmd/utils/flags_xlayer.go:88`. Affected module: cmd.

[Warning] **MigrationBlock=0 silently disables routing**: `shouldProxyByNumber` returns false when `MigrationBlock==0` because condition is `MigrationBlock > 0`. A misconfigured `MigrationBlock=0` disables routing silently. Source: `eth/api_legacy_xlayer.go`. Affected module: eth.

[Warning] **CheckWriteChainConfig hash mismatch**: `CommitXLayerFirstBlock` calls `WriteChainConfig` with synthetic block hash (not genesis hash). On subsequent restarts, `ReadChainConfig(genesisHash)` will not find this config entry. Source: `core/util_xlayer.go`. Affected module: core.
