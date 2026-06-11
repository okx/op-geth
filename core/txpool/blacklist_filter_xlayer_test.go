package txpool

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
)

// blacklistArraySlot mirrors core's enumerable-array layout (length at slot 0,
// element i at keccak256(slot0)+i). Kept local to avoid exporting internals.
var blacklistArraySlot = common.Hash{}

func writeMirror(t *testing.T, sdb *state.StateDB, chainID uint64, addrs []common.Address) {
	t.Helper()
	mirror, ok := params.BlacklistMirror(chainID)
	if !ok {
		t.Fatalf("chain %d not enabled", chainID)
	}
	sdb.SetState(mirror, blacklistArraySlot, common.BigToHash(big.NewInt(int64(len(addrs)))))
	base := new(big.Int).SetBytes(crypto.Keccak256(blacklistArraySlot.Bytes()))
	for i, a := range addrs {
		slot := common.BigToHash(new(big.Int).Add(base, big.NewInt(int64(i))))
		sdb.SetState(mirror, slot, common.BytesToHash(a.Bytes()))
	}
}

func newStateDB(t *testing.T) *state.StateDB {
	t.Helper()
	sdb, err := state.New(types.EmptyRootHash, state.NewDatabaseForTesting())
	if err != nil {
		t.Fatalf("state.New: %v", err)
	}
	return sdb
}

func signedTx(t *testing.T, chainID uint64, to common.Address) *types.Transaction {
	t.Helper()
	key, _ := crypto.GenerateKey()
	signer := types.LatestSignerForChainID(new(big.Int).SetUint64(chainID))
	return types.MustSignNewTx(key, signer, &types.LegacyTx{
		Nonce: 0, To: &to, Gas: 21000, GasPrice: big.NewInt(1), Value: big.NewInt(0),
	})
}

// signedTxFromListed signs a tx whose sender is `listed`, returning the tx and the sender.
func signedTxFromListed(t *testing.T, chainID uint64, to common.Address) (*types.Transaction, common.Address) {
	t.Helper()
	key, _ := crypto.GenerateKey()
	from := crypto.PubkeyToAddress(key.PublicKey)
	signer := types.LatestSignerForChainID(new(big.Int).SetUint64(chainID))
	tx := types.MustSignNewTx(key, signer, &types.LegacyTx{
		Nonce: 0, To: &to, Gas: 21000, GasPrice: big.NewInt(1), Value: big.NewInt(0),
	})
	return tx, from
}

func TestBlacklistFilter_FilterTx(t *testing.T) {
	const chainID = params.XLayerMainnetChainID
	other := common.HexToAddress("0x00000000000000000000000000000000000000BB")

	t.Run("none hit → allow (DM-1.4)", func(t *testing.T) {
		sdb := newStateDB(t)
		listed := common.HexToAddress("0x00000000000000000000000000000000000000AA")
		writeMirror(t, sdb, chainID, []common.Address{listed})
		f := NewBlacklistFilter(chainID)
		f.Refresh(sdb)
		if !f.FilterTx(context.Background(), signedTx(t, chainID, other)) {
			t.Fatalf("non-hit tx must be allowed")
		}
	})

	t.Run("to hit → reject (DM-1.2)", func(t *testing.T) {
		sdb := newStateDB(t)
		listed := common.HexToAddress("0x00000000000000000000000000000000000000AA")
		writeMirror(t, sdb, chainID, []common.Address{listed})
		f := NewBlacklistFilter(chainID)
		f.Refresh(sdb)
		if f.FilterTx(context.Background(), signedTx(t, chainID, listed)) {
			t.Fatalf("tx to a listed recipient must be rejected")
		}
	})

	t.Run("from hit → reject (DM-1.1)", func(t *testing.T) {
		sdb := newStateDB(t)
		tx, from := signedTxFromListed(t, chainID, other)
		writeMirror(t, sdb, chainID, []common.Address{from})
		f := NewBlacklistFilter(chainID)
		f.Refresh(sdb)
		if f.FilterTx(context.Background(), tx) {
			t.Fatalf("tx from a listed sender must be rejected")
		}
	})

	t.Run("disabled chain → pass-through (DM-1.8)", func(t *testing.T) {
		sdb := newStateDB(t)
		f := NewBlacklistFilter(1) // not enabled
		f.Refresh(sdb)
		if !f.FilterTx(context.Background(), signedTx(t, 1, common.HexToAddress("0x00000000000000000000000000000000000000AA"))) {
			t.Fatalf("disabled chain must pass everything through")
		}
	})

	t.Run("empty list → pass-through before refresh", func(t *testing.T) {
		f := NewBlacklistFilter(chainID)
		if !f.FilterTx(context.Background(), signedTx(t, chainID, other)) {
			t.Fatalf("empty (pre-refresh) snapshot must pass-through")
		}
	})

	t.Run("reorg refresh rebuilds from new head (DM-1.6)", func(t *testing.T) {
		listedOld := common.HexToAddress("0x00000000000000000000000000000000000000A1")
		listedNew := common.HexToAddress("0x00000000000000000000000000000000000000A2")
		f := NewBlacklistFilter(chainID)

		oldHead := newStateDB(t)
		writeMirror(t, oldHead, chainID, []common.Address{listedOld})
		f.Refresh(oldHead)
		if f.FilterTx(context.Background(), signedTx(t, chainID, listedOld)) {
			t.Fatalf("old-head listed addr should be rejected")
		}

		newHead := newStateDB(t)
		writeMirror(t, newHead, chainID, []common.Address{listedNew})
		f.Refresh(newHead) // simulate reorg reset to a new head
		if !f.FilterTx(context.Background(), signedTx(t, chainID, listedOld)) {
			t.Fatalf("stale old-head entry must not linger after refresh")
		}
		if f.FilterTx(context.Background(), signedTx(t, chainID, listedNew)) {
			t.Fatalf("new-head listed addr should be rejected")
		}
	})
}
