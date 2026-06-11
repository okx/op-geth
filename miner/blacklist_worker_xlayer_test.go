package miner

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
)

// blacklistWorkerConfig is a Berlin-era config on the XLayer mainnet chain_id
// (196 → blacklist enabled), legacy-tx friendly.
func blacklistWorkerConfig() *params.ChainConfig {
	return &params.ChainConfig{
		ChainID:             big.NewInt(int64(params.XLayerMainnetChainID)),
		HomesteadBlock:      big.NewInt(0),
		EIP150Block:         big.NewInt(0),
		EIP155Block:         big.NewInt(0),
		EIP158Block:         big.NewInt(0),
		ByzantiumBlock:      big.NewInt(0),
		ConstantinopleBlock: big.NewInt(0),
		PetersburgBlock:     big.NewInt(0),
		IstanbulBlock:       big.NewInt(0),
		BerlinBlock:         big.NewInt(0),
	}
}

// TestCommitTransaction_BlacklistHitMarksRejected guards the build-path handling
// of ErrBlacklistDrop in commitTransaction (worker.go): when the execution gate
// drops a committed normal-tx hit, the miner must mark the tx Rejected() — the
// flag the pool's demoteUnexecutables later uses to eject it — and surface
// ErrBlacklistDrop. An injected snapshot gate stands in for the mirror read, so
// this isolates the worker step (no mirror deploy / merge-Random dependency).
func TestCommitTransaction_BlacklistHitMarksRejected(t *testing.T) {
	const chainID = params.XLayerMainnetChainID
	listed := common.HexToAddress("0x00000000000000000000000000000000000000AA")
	config := blacklistWorkerConfig()

	sdb, err := state.New(types.EmptyRootHash, state.NewDatabaseForTesting())
	if err != nil {
		t.Fatalf("state.New: %v", err)
	}
	key, _ := crypto.GenerateKey()
	from := crypto.PubkeyToAddress(key.PublicKey)
	sdb.AddBalance(from, uint256.NewInt(1e18), tracing.BalanceChangeUnspecified)

	signer := types.LatestSignerForChainID(big.NewInt(int64(chainID)))
	tx := types.MustSignNewTx(key, signer, &types.LegacyTx{
		Nonce: 0, To: &listed, Gas: 21000, GasPrice: big.NewInt(1), Value: big.NewInt(100),
	})

	gate := core.NewBlacklistGateFromSnapshot(chainID, core.NewSnapshot([]common.Address{listed}))
	if gate == nil {
		t.Fatal("expected active gate")
	}

	header := &types.Header{Number: big.NewInt(1), Time: 1}
	blockCtx := vm.BlockContext{
		CanTransfer: core.CanTransfer,
		Transfer:    core.Transfer,
		GetHash:     func(uint64) common.Hash { return common.Hash{} },
		GasLimit:    30_000_000,
		BlockNumber: big.NewInt(1),
		Time:        1,
		Difficulty:  big.NewInt(0),
		BaseFee:     big.NewInt(0),
	}
	// EVM observes the gate via hooked state + tracer; env.state stays the raw
	// StateDB so the outer snapshot/revert mirrors miner.applyTransaction.
	evm := vm.NewEVM(blockCtx, state.NewHookedState(sdb, gate.Hooks()), config, vm.Config{Tracer: gate.Hooks()})

	env := &environment{
		signer:  signer,
		state:   sdb,
		gasPool: core.NewGasPool(30_000_000),
		evm:     evm,
		blGate:  gate,
		header:  header,
	}

	miner := &Miner{}
	err = miner.commitTransaction(context.Background(), env, tx)
	if !errors.Is(err, core.ErrBlacklistDrop) {
		t.Fatalf("err = %v, want core.ErrBlacklistDrop", err)
	}
	if !tx.Rejected() {
		t.Fatal("blacklisted normal-tx hit must be marked Rejected() so the pool ejects it")
	}
}
