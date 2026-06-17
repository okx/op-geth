package txpool

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/tracing"
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

	// Exercises the REAL Refresh method (and the ReadBlacklistSnapshot on-chain
	// view read it drives) rather than a manual snap.Store, deploying a mirror
	// stub at the hardcoded address. This is the wiring that legacypool.reset
	// invokes on commit/reorg (FR-1 AC4); without this the Refresh path itself
	// has no coverage. It also asserts the reorg no-residue property: a stale
	// pre-seeded entry is gone after a real refresh from the new head.
	t.Run("real Refresh reads the mirror and drops stale residue", func(t *testing.T) {
		mirror, ok := params.BlacklistMirror(chainID)
		if !ok {
			t.Fatalf("chain %d must be blacklist-enabled", chainID)
		}
		fromMirror := common.HexToAddress("0x00000000000000000000000000000000000000C3")
		stale := common.HexToAddress("0x00000000000000000000000000000000000000C4")

		statedb, _ := state.New(types.EmptyRootHash, state.NewDatabaseForTesting())
		statedb.SetCode(mirror, mirrorReturningBytecode(fromMirror), tracing.CodeChangeGenesis)
		header := &types.Header{Number: big.NewInt(1), Time: 1, Difficulty: big.NewInt(0)}
		config := &params.ChainConfig{ChainID: new(big.Int).SetUint64(chainID)}

		f := filterWith(chainID, stale) // stale old-head snapshot
		f.Refresh(statedb, header, config)

		if f.FilterTx(context.Background(), signedTx(t, chainID, fromMirror)) {
			t.Fatal("address read from the refreshed mirror must be rejected")
		}
		if !f.FilterTx(context.Background(), signedTx(t, chainID, stale)) {
			t.Fatal("stale old-head entry must not linger after a real Refresh")
		}
	})
}

// mirrorReturningBytecode is a minimal getBlacklist(start,limit) stub: it ignores
// the page args and always returns the ABI tuple (total=1, [addr]). Memory layout
// of the return data: [0x00]=1 (total) [0x20]=0x40 (array offset) [0x40]=1 (len)
// [0x60]=addr. No PUSH0, so it runs on any instruction set the read-only EVM picks.
func mirrorReturningBytecode(addr common.Address) []byte {
	code := []byte{
		0x60, 0x01, 0x60, 0x00, 0x52, // PUSH1 1;    PUSH1 0x00; MSTORE  → total=1
		0x60, 0x40, 0x60, 0x20, 0x52, // PUSH1 0x40; PUSH1 0x20; MSTORE  → array offset=0x40
		0x60, 0x01, 0x60, 0x40, 0x52, // PUSH1 1;    PUSH1 0x40; MSTORE  → array len=1
		0x73, // PUSH20 (address follows)
	}
	code = append(code, addr.Bytes()...) // 20-byte address literal
	code = append(code,
		0x60, 0x60, 0x52, // PUSH1 0x60; MSTORE → addr at mem[0x60]
		0x60, 0x80, 0x60, 0x00, 0xf3, // PUSH1 0x80; PUSH1 0x00; RETURN (return mem[0:0x80])
	)
	return code
}
