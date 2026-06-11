package core

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
)

// newTestStateDB builds an empty in-memory StateDB for module tests.
func newTestStateDB(t *testing.T) *state.StateDB {
	t.Helper()
	sdb, err := state.New(types.EmptyRootHash, state.NewDatabaseForTesting())
	if err != nil {
		t.Fatalf("state.New: %v", err)
	}
	return sdb
}

// writeMirrorList writes an enumerable address[] (length at slot 0, element i at
// keccak256(slot0)+i) into the L2BlacklistMirror storage for the given chain.
func writeMirrorList(t *testing.T, sdb *state.StateDB, chainID uint64, addrs []common.Address) {
	t.Helper()
	mirror, ok := params.BlacklistMirror(chainID)
	if !ok {
		t.Fatalf("chain %d not blacklist-enabled", chainID)
	}
	sdb.SetState(mirror, blacklistArraySlot, common.BigToHash(big.NewInt(int64(len(addrs)))))
	base := new(big.Int).SetBytes(crypto.Keccak256(blacklistArraySlot.Bytes()))
	for i, a := range addrs {
		slot := common.BigToHash(new(big.Int).Add(base, big.NewInt(int64(i))))
		sdb.SetState(mirror, slot, common.BytesToHash(a.Bytes()))
	}
}

func TestReadBlacklistSnapshot_NonEmpty(t *testing.T) {
	// DM-4.1: non-empty enumerable list → snapshot contains all entries.
	sdb := newTestStateDB(t)
	aaa := common.HexToAddress("0x00000000000000000000000000000000000000AA")
	bbb := common.HexToAddress("0x00000000000000000000000000000000000000BB")
	writeMirrorList(t, sdb, params.XLayerMainnetChainID, []common.Address{aaa, bbb})

	snap := ReadBlacklistSnapshot(sdb, params.XLayerMainnetChainID)
	if snap.Size() != 2 {
		t.Fatalf("Size = %d, want 2", snap.Size())
	}
	if !snap.Contains(aaa) || !snap.Contains(bbb) {
		t.Fatalf("snapshot missing expected entries")
	}
	if snap.Contains(common.HexToAddress("0x00000000000000000000000000000000000000CC")) {
		t.Fatalf("snapshot contains unexpected entry")
	}
}

func TestReadBlacklistSnapshot_EmptyAndNotDeployed(t *testing.T) {
	// DM-4.4 / DM-4.5: length slot 0 (not deployed or empty) → empty snapshot.
	sdb := newTestStateDB(t)
	snap := ReadBlacklistSnapshot(sdb, params.XLayerMainnetChainID)
	if snap.Size() != 0 {
		t.Fatalf("not-deployed Size = %d, want 0", snap.Size())
	}

	writeMirrorList(t, sdb, params.XLayerMainnetChainID, nil) // explicit length 0
	if got := ReadBlacklistSnapshot(sdb, params.XLayerMainnetChainID).Size(); got != 0 {
		t.Fatalf("empty-list Size = %d, want 0", got)
	}
}

func TestReadBlacklistSnapshot_DisabledChain(t *testing.T) {
	// DM-4.8: BlacklistMirror !ok (disabled chain) → empty snapshot, no read.
	sdb := newTestStateDB(t)
	snap := ReadBlacklistSnapshot(sdb, 1 /* eth mainnet, not enabled */)
	if snap.Size() != 0 {
		t.Fatalf("disabled-chain Size = %d, want 0", snap.Size())
	}
}

func TestSnapshotNilSafety(t *testing.T) {
	var s *Snapshot
	if s.Size() != 0 || s.Contains(common.Address{}) {
		t.Fatalf("nil snapshot must be empty and contain nothing")
	}
}
