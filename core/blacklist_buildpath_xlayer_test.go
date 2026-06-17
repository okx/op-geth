package core

import (
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
)

// buildPathTestConfig is a Berlin-era (pre-London, legacy-tx friendly) config on
// the XLayer mainnet chain_id (196 → blacklist enabled). Byzantium active so the
// gated apply takes the Finalise(true) branch, mirroring the production path.
func buildPathTestConfig() *params.ChainConfig {
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

func newBuildPathEVM(config *params.ChainConfig, statedb *state.StateDB, gate *BlacklistGate) *vm.EVM {
	blockCtx := vm.BlockContext{
		CanTransfer: CanTransfer,
		Transfer:    Transfer,
		GetHash:     func(uint64) common.Hash { return common.Hash{} },
		Coinbase:    common.Address{},
		GasLimit:    30_000_000,
		BlockNumber: big.NewInt(1),
		Time:        1,
		Difficulty:  big.NewInt(0),
		BaseFee:     big.NewInt(0),
	}
	var sdb vm.StateDB = statedb
	var vmcfg vm.Config
	if gate != nil {
		sdb = state.NewHookedState(statedb, gate.Hooks())
		vmcfg = vm.Config{Tracer: gate.Hooks()}
	}
	return vm.NewEVM(blockCtx, sdb, config, vmcfg)
}

func buildPathHeader() *types.Header {
	return &types.Header{Number: big.NewInt(1), Time: 1}
}

// signValueTx signs a legacy value-transfer tx from a fresh key, returning the
// tx and the sender address.
func signValueTx(t *testing.T, to common.Address, value int64) (*types.Transaction, common.Address) {
	t.Helper()
	key, _ := crypto.GenerateKey()
	from := crypto.PubkeyToAddress(key.PublicKey)
	signer := types.LatestSignerForChainID(big.NewInt(int64(params.XLayerMainnetChainID)))
	tx := types.MustSignNewTx(key, signer, &types.LegacyTx{
		Nonce: 0, To: &to, Gas: 21000, GasPrice: big.NewInt(1), Value: big.NewInt(value),
	})
	return tx, from
}

// TestBuildPath_NormalHitDropped: a build-path value transfer to a blacklisted
// recipient is signalled for drop (ErrBlacklistDrop); the miner's outer-snapshot
// revert then fully undoes it (recipient balance + sender nonce unchanged).
func TestBuildPath_NormalHitDropped(t *testing.T) {
	const chainID = params.XLayerMainnetChainID
	config := buildPathTestConfig()
	listed := common.HexToAddress("0x00000000000000000000000000000000000000AA")

	sdb := newTestStateDB(t)
	tx, from := signValueTx(t, listed, 100)
	sdb.AddBalance(from, uint256.NewInt(1e18), tracing.BalanceChangeUnspecified)
	gate := NewBlacklistGateFromSnapshot(chainID, NewSnapshot([]common.Address{listed}), true)
	if gate == nil {
		t.Fatal("expected active gate")
	}
	evm := newBuildPathEVM(config, sdb, gate)

	snap := sdb.Snapshot() // mimic miner.applyTransaction outer snapshot
	_, err := ApplyTransaction(gate, evm, NewGasPool(30_000_000), sdb, buildPathHeader(), tx)
	if !errors.Is(err, ErrBlacklistDrop) {
		t.Fatalf("err = %v, want ErrBlacklistDrop", err)
	}
	sdb.RevertToSnapshot(snap) // miner fully undoes the dropped tx
	if !sdb.GetBalance(listed).IsZero() {
		t.Fatalf("recipient balance = %s, want 0 (dropped)", sdb.GetBalance(listed))
	}
	if sdb.GetNonce(from) != 0 {
		t.Fatalf("sender nonce = %d, want 0 (dropped)", sdb.GetNonce(from))
	}
}

// TestBuildPath_NonHitKept: a value transfer to a non-listed recipient is kept
// with a normal successful receipt and the value moved.
func TestBuildPath_NonHitKept(t *testing.T) {
	const chainID = params.XLayerMainnetChainID
	config := buildPathTestConfig()
	listed := common.HexToAddress("0x00000000000000000000000000000000000000AA")
	recipient := common.HexToAddress("0x00000000000000000000000000000000000000CC")

	sdb := newTestStateDB(t)
	tx, from := signValueTx(t, recipient, 100)
	sdb.AddBalance(from, uint256.NewInt(1e18), tracing.BalanceChangeUnspecified)
	gate := NewBlacklistGateFromSnapshot(chainID, NewSnapshot([]common.Address{listed}), true)
	evm := newBuildPathEVM(config, sdb, gate)

	receipt, err := ApplyTransaction(gate, evm, NewGasPool(30_000_000), sdb, buildPathHeader(), tx)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if receipt.Status != types.ReceiptStatusSuccessful {
		t.Fatalf("status = %d, want 1", receipt.Status)
	}
	if sdb.GetBalance(recipient).Uint64() != 100 {
		t.Fatalf("recipient balance = %s, want 100", sdb.GetBalance(recipient))
	}
}

// TestBuildPath_NilGateFallback: with a nil gate, the build entrypoint falls back
// to core.ApplyTransaction (no drop, normal receipt) — the non-XLayer hot path.
func TestBuildPath_NilGateFallback(t *testing.T) {
	config := buildPathTestConfig()
	recipient := common.HexToAddress("0x00000000000000000000000000000000000000CC")

	sdb := newTestStateDB(t)
	tx, from := signValueTx(t, recipient, 100)
	sdb.AddBalance(from, uint256.NewInt(1e18), tracing.BalanceChangeUnspecified)

	evm := newBuildPathEVM(config, sdb, nil)
	receipt, err := ApplyTransaction(nil, evm, NewGasPool(30_000_000), sdb, buildPathHeader(), tx)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if receipt.Status != types.ReceiptStatusSuccessful {
		t.Fatalf("status = %d, want 1", receipt.Status)
	}
	if sdb.GetBalance(recipient).Uint64() != 100 {
		t.Fatalf("recipient balance = %s, want 100", sdb.GetBalance(recipient))
	}
}
