// XLayer emergency-freeze blacklist — on-chain data source (XLOP-1099, FR-4).
//
// This file is a fork-local XLayer extension (see KG naming rule: XLayer
// additions to core live in a dedicated _xlayer.go file). It must not contain
// upstream go-ethereum logic.

package core

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
)

// blacklistArraySlot is the storage slot of the enumerable address array in the
// L2BlacklistMirror contract. PRD I-4 forbids a bare mapping(address=>bool);
// the layout is an enumerable `address[]` so a node can read the whole list at
// block head. For a Solidity dynamic array declared first, the length lives at
// slot 0 and element i lives at keccak256(slot0)+i. The concrete layout is an
// ABI contract shared with xlayer-reth (blocking open item B-3, TD §5); any
// change must be synchronized across both clients and the contracts repo.
var blacklistArraySlot = common.Hash{}

// maxSnapshotEntries bounds how many entries ReadBlacklistSnapshot will read in
// a single block, defending against a corrupt/pathological length word causing
// unbounded work. The cap is deterministic (identical on every client) and far
// above the PRD engineering upper bound of 300k addresses, so it never affects
// well-formed lists. If a list legitimately exceeds this, it must be raised in
// lockstep across op-geth and xlayer-reth.
const maxSnapshotEntries = 1 << 20

// Snapshot is an immutable, block-head view of the blacklist address set. It is
// safe for concurrent reads (never mutated after construction).
type Snapshot struct {
	set       map[common.Address]struct{}
	blockHash common.Hash
}

// NewSnapshot builds a Snapshot from the given addresses. Used by the ingress
// filter refresh path and by tests; the execution gate uses
// ReadBlacklistSnapshot.
func NewSnapshot(addrs []common.Address) *Snapshot {
	set := make(map[common.Address]struct{}, len(addrs))
	for _, a := range addrs {
		set[a] = struct{}{}
	}
	return &Snapshot{set: set}
}

// Contains reports whether the address is on the blacklist snapshot. A nil
// snapshot contains nothing (treated as empty / no-op).
func (s *Snapshot) Contains(a common.Address) bool {
	if s == nil {
		return false
	}
	_, ok := s.set[a]
	return ok
}

// Size returns the number of blacklisted addresses in the snapshot. A nil
// snapshot has size 0.
func (s *Snapshot) Size() int {
	if s == nil {
		return 0
	}
	return len(s.set)
}

// BlockHash returns the block hash this snapshot was read at (zero if unset).
func (s *Snapshot) BlockHash() common.Hash {
	if s == nil {
		return common.Hash{}
	}
	return s.blockHash
}

// ReadBlacklistSnapshot reads the blacklist address set from the parent /
// block-head state of the L2BlacklistMirror contract for the given chain_id.
//
// statedb MUST be the parent / block-head state (never mid-block live state),
// so that an add landing in block N is only visible from block N+1 — there is
// no in-block delta (FR-4, DM-4.2 / DM-4.7). The whole list is read once per
// block and reused for every tx in that block.
//
// Behavior:
//   - chain not enabled (BlacklistMirror !ok)          → empty snapshot (no read)
//   - mirror not deployed / empty (length slot == 0)    → empty snapshot, no-op
//   - non-empty enumerable list                         → populated snapshot
func ReadBlacklistSnapshot(statedb vm.StateDB, chainID uint64) *Snapshot {
	mirror, ok := params.BlacklistMirror(chainID)
	if !ok {
		return &Snapshot{set: map[common.Address]struct{}{}}
	}

	lengthWord := statedb.GetState(mirror, blacklistArraySlot)
	length := new(big.Int).SetBytes(lengthWord.Bytes())
	if length.Sign() == 0 {
		// Not deployed or empty list → no-op, state root identical to baseline.
		return &Snapshot{set: map[common.Address]struct{}{}}
	}

	n := length.Uint64()
	if !length.IsUint64() || n > maxSnapshotEntries {
		log.Warn("XLayer blacklist: mirror length exceeds cap, truncating",
			"chainID", chainID, "length", length.String(), "cap", maxSnapshotEntries)
		n = maxSnapshotEntries
	}

	set := make(map[common.Address]struct{}, n)
	base := new(big.Int).SetBytes(crypto.Keccak256(blacklistArraySlot.Bytes()))
	for i := uint64(0); i < n; i++ {
		slot := common.BigToHash(new(big.Int).Add(base, new(big.Int).SetUint64(i)))
		word := statedb.GetState(mirror, slot)
		addr := common.BytesToAddress(word.Bytes())
		if addr == (common.Address{}) {
			// Zero entries are not valid blacklist members; skip (defensive).
			continue
		}
		set[addr] = struct{}{}
	}
	return &Snapshot{set: set}
}
