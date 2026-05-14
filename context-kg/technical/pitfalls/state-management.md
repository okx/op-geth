---
name: "state-management"
description: "Pitfalls related to state/trie management, snapshots, and chain database operations"
---
# State Management Pitfalls

[Pitfall] **SnapshotWait default=true is a dirty hack**: `DefaultConfig` sets `SnapshotWait=true` which blocks startup synchronously for snapshot construction. Explicitly noted as testing-only hack. **Correct approach**: Never set `SnapshotWait=true` in production; use `SnapshotNoBuild` or `AsyncBuild`. Source: TODO at `core/blockchain.go:196`. Affected module: core.

[Pitfall] **TrieDirtyLimit / WriteBufferSize naming confusion**: pathdb `WriteBufferSize` is set from `cfg.TrieDirtyLimit`, not a dedicated config. Operators tuning "TrieDirtyLimit" also silently control pathdb write buffer. **Correct approach**: Document the dual role or rename. Source: TODO at `core/blockchain.go:276-279`. Affected module: core, triedb.

[Pitfall] **Deleted-log emission order on reorg is wrong**: During reorg, deleted log events are emitted in forward order (oldest first) instead of reverse. Acknowledged as "borked" but kept for legacy API compatibility. **Correct approach**: Do not build features relying on deleted-log ordering. Source: `core/blockchain.go:2641-2644`. Affected module: core.

[Pitfall] **pathdb state rollback broken in dev mode**: When `stateFreezer` is nil (dev mode), `Recoverable()` always returns false. Deep reorgs are silently unsupported. **Correct approach**: Document limitation; implement in-memory ancient store. Source: TODO at `triedb/pathdb/database.go:503`. Affected module: triedb/pathdb.

[Pitfall] **Background state history indexing in read-only mode**: `setHistoryIndexer` does not check read-only mode before starting background indexing, causing unexpected write attempts. Source: TODO at `triedb/pathdb/database.go:210`. Affected module: triedb/pathdb.

[Pitfall] **Skeleton downloader never reclaims trimmed header DB space**: After reorg/trim, skeleton headers outside subchain index are never deleted. DB space leaks over long-running nodes. Source: TODO at `eth/downloader/skeleton.go:630,725`. Affected module: eth/downloader.

[Pitfall] **Missing reorg hook for reverse log emission**: During reorg, reversal logs are collected and reversed in slice but the tracing hook for reverse emission is not wired. Tracers/monitors consuming reversal logs via hook will miss reorg-related log reversals. Source: TODO at `core/blockchain.go:2681`. Affected module: core.

[Warning] **Header validation not skipped for pre-cutoff blocks**: `InsertHeadersBeforeCutoff` still runs full `ValidateHeaderChain` even though headers before cutoff are guaranteed. Unnecessary CPU overhead during historical sync. Source: TODO at `core/blockchain.go:2935-2937`. Affected module: core.
