package txpool

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
)

// filterWith builds a BlacklistFilter pre-seeded with the given list (bypassing
// the on-chain view read, which is covered by core's read tests).
func filterWith(chainID uint64, addrs ...common.Address) *BlacklistFilter {
	return NewBlacklistFilterWithSnapshot(chainID, core.NewSnapshot(addrs))
}

func signedTx(t *testing.T, chainID uint64, to common.Address) *types.Transaction {
	t.Helper()
	key, _ := crypto.GenerateKey()
	signer := types.LatestSignerForChainID(new(big.Int).SetUint64(chainID))
	return types.MustSignNewTx(key, signer, &types.LegacyTx{
		Nonce: 0, To: &to, Gas: 21000, GasPrice: big.NewInt(1), Value: big.NewInt(0),
	})
}

// signedTxFromListed signs a tx whose sender is `from`, returning the tx and the sender.
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
	listed := common.HexToAddress("0x00000000000000000000000000000000000000AA")

	t.Run("none hit → allow (DM-1.4)", func(t *testing.T) {
		f := filterWith(chainID, listed)
		if !f.FilterTx(context.Background(), signedTx(t, chainID, other)) {
			t.Fatal("non-hit tx must be allowed")
		}
	})

	t.Run("to hit → reject (DM-1.2)", func(t *testing.T) {
		f := filterWith(chainID, listed)
		if f.FilterTx(context.Background(), signedTx(t, chainID, listed)) {
			t.Fatal("tx to a listed recipient must be rejected")
		}
	})

	t.Run("from hit → reject (DM-1.1)", func(t *testing.T) {
		tx, from := signedTxFromListed(t, chainID, other)
		f := filterWith(chainID, from)
		if f.FilterTx(context.Background(), tx) {
			t.Fatal("tx from a listed sender must be rejected")
		}
	})

	t.Run("disabled chain → pass-through (DM-1.8)", func(t *testing.T) {
		f := NewBlacklistFilter(1) // not enabled → snapshot stays empty
		if !f.FilterTx(context.Background(), signedTx(t, 1, listed)) {
			t.Fatal("disabled chain must pass everything through")
		}
	})

	t.Run("empty list → pass-through", func(t *testing.T) {
		f := NewBlacklistFilter(chainID)
		if !f.FilterTx(context.Background(), signedTx(t, chainID, other)) {
			t.Fatal("empty snapshot must pass-through")
		}
	})

	t.Run("refresh rebuilds from new head (DM-1.6)", func(t *testing.T) {
		listedOld := common.HexToAddress("0x00000000000000000000000000000000000000A1")
		listedNew := common.HexToAddress("0x00000000000000000000000000000000000000A2")
		f := filterWith(chainID, listedOld)
		if f.FilterTx(context.Background(), signedTx(t, chainID, listedOld)) {
			t.Fatal("old-head listed addr should be rejected")
		}
		// Simulate a reset/reorg replacing the snapshot with a new-head list.
		f.snap.Store(core.NewSnapshot([]common.Address{listedNew}))
		if !f.FilterTx(context.Background(), signedTx(t, chainID, listedOld)) {
			t.Fatal("stale old-head entry must not linger after refresh")
		}
		if f.FilterTx(context.Background(), signedTx(t, chainID, listedNew)) {
			t.Fatal("new-head listed addr should be rejected")
		}
	})
}
