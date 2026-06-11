// XLayer emergency-freeze blacklist — ingress filter (XLOP-1099, FR-1).
//
// Fork-local XLayer extension (KG naming rule). Implements the existing
// txpool.IngressFilter interface so RPC and P2P admissions both run through it
// via LegacyPool.Add. It holds an in-memory, atomically-swapped block-head
// snapshot refreshed on every pool reset (commit + reorg).

package txpool

import (
	"context"
	"math/big"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
)

// BlacklistFilter rejects a transaction whose top-level sender or recipient is
// on the blacklist snapshot. It only inspects the top-level from/to (O(1)) and
// never reads state per-tx; the snapshot is refreshed once per block via
// Refresh. Committed inner touches are caught by the execution gate (FR-2), so
// an ingress refresh lag is harmless (FR-1 AC3).
type BlacklistFilter struct {
	chainID uint64
	signer  types.Signer
	snap    atomic.Pointer[core.Snapshot]
}

// NewBlacklistFilter creates a BlacklistFilter for the given chain. Until the
// first Refresh the snapshot is empty, so the filter passes everything through.
func NewBlacklistFilter(chainID uint64) *BlacklistFilter {
	f := &BlacklistFilter{
		chainID: chainID,
		signer:  types.LatestSignerForChainID(new(big.Int).SetUint64(chainID)),
	}
	f.snap.Store(core.NewSnapshot(nil))
	return f
}

// NewBlacklistFilterWithSnapshot builds a filter pre-seeded with a snapshot,
// bypassing the on-chain read. Used by tests (and tooling) to construct a filter
// with a known list without deploying a mirror contract.
func NewBlacklistFilterWithSnapshot(chainID uint64, snap *core.Snapshot) *BlacklistFilter {
	f := NewBlacklistFilter(chainID)
	if snap != nil {
		f.snap.Store(snap)
	}
	return f
}

// FilterTx implements txpool.IngressFilter. It returns true (allow) when the
// snapshot is empty (disabled chain or empty list) or neither top-level address
// is blacklisted; false (reject) on a hit.
func (f *BlacklistFilter) FilterTx(ctx context.Context, tx *types.Transaction) bool {
	snap := f.snap.Load()
	if snap.Size() == 0 {
		return true // disabled chain / empty list → pass-through (FR-6 AC2)
	}
	if to := tx.To(); to != nil && snap.Contains(*to) {
		core.MetricBlacklistPoolRejected()
		return false
	}
	// Recover the top-level sender; if recovery fails the tx is invalid and will
	// be rejected by later validation, so we do not reject on `from` here.
	if from, err := types.Sender(f.signer, tx); err == nil && snap.Contains(from) {
		core.MetricBlacklistPoolRejected()
		return false
	}
	return true
}

// Refresh rebuilds the in-memory snapshot from the given block-head state via
// the mirror contract's view ABI. header/config are needed to build the
// read-only EVM for the view call. It is invoked at the end of LegacyPool.reset,
// which fires on both commit and reorg (FR-1 AC4) — so after a reorg the snapshot
// is rebuilt from the new head with no stale-chain residue.
func (f *BlacklistFilter) Refresh(headState vm.StateDB, header *types.Header, config *params.ChainConfig) {
	snap := core.ReadBlacklistSnapshot(headState, header, config, f.chainID)
	f.snap.Store(snap)
	core.MetricBlacklistCacheSize(snap.Size())
}
