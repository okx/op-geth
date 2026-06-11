package legacypool

import (
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/txpool"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/params"
)

// TestAddTx_BlacklistFilterMapsToErrBlacklisted (FR-7): a tx rejected by the
// *txpool.BlacklistFilter must surface core.ErrBlacklisted (→ JSON-RPC -32000),
// not the generic ErrTxFilteredOut. The filter's recipient (To) check fires
// regardless of signer chain, so a To==listed tx is rejected.
func TestAddTx_BlacklistFilterMapsToErrBlacklisted(t *testing.T) {
	statedb, _ := state.New(types.EmptyRootHash, state.NewDatabaseForTesting())
	blockchain := newTestBlockChain(params.TestChainConfig, 1000000, statedb, new(event.Feed))
	pool := New(testTxPoolConfig, blockchain)

	const chainID = params.XLayerMainnetChainID
	listed := common.HexToAddress("0x00000000000000000000000000000000000000AA")
	bf := txpool.NewBlacklistFilterWithSnapshot(chainID, core.NewSnapshot([]common.Address{listed}))

	pool.SetIngressFilters([]txpool.IngressFilter{bf})
	pool.Init(testTxPoolConfig.PriceLimit, blockchain.CurrentBlock(), newReserver())
	defer pool.Close()

	key, _ := crypto.GenerateKey()
	testAddBalance(pool, crypto.PubkeyToAddress(key.PublicKey), big.NewInt(1000000))
	signer := types.LatestSigner(params.TestChainConfig)
	tx := types.MustSignNewTx(key, signer, &types.LegacyTx{
		Nonce: 0, To: &listed, Gas: 100000, GasPrice: big.NewInt(1), Value: big.NewInt(0),
	})

	err := pool.addRemoteSync(tx)
	if !errors.Is(err, core.ErrBlacklisted) {
		t.Fatalf("err = %v, want core.ErrBlacklisted (-32000 sentinel)", err)
	}
}

// TestAddTx_OtherFilterMapsToErrTxFilteredOut: a tx rejected by a non-blacklist
// ingress filter keeps the generic core.ErrTxFilteredOut (the type-switch else
// branch), proving the blacklist mapping is scoped to *txpool.BlacklistFilter.
func TestAddTx_OtherFilterMapsToErrTxFilteredOut(t *testing.T) {
	statedb, _ := state.New(types.EmptyRootHash, state.NewDatabaseForTesting())
	blockchain := newTestBlockChain(params.TestChainConfig, 1000000, statedb, new(event.Feed))
	pool := New(testTxPoolConfig, blockchain)

	filter := &dummyFilter{}
	filter.allow.Store(false) // reject everything
	pool.SetIngressFilters([]txpool.IngressFilter{filter})
	pool.Init(testTxPoolConfig.PriceLimit, blockchain.CurrentBlock(), newReserver())
	defer pool.Close()

	key, _ := crypto.GenerateKey()
	testAddBalance(pool, crypto.PubkeyToAddress(key.PublicKey), big.NewInt(1000000))

	err := pool.addRemoteSync(pricedTransaction(0, 100000, big.NewInt(1), key))
	if !errors.Is(err, core.ErrTxFilteredOut) {
		t.Fatalf("err = %v, want core.ErrTxFilteredOut", err)
	}
	if errors.Is(err, core.ErrBlacklisted) {
		t.Fatal("non-blacklist filter must NOT map to ErrBlacklisted")
	}
}
