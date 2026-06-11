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

// mirrorStubRuntimeHex: getBlacklist(start,limit)->(total=2,[0xAA,0xBB]) — same
// MirrorStub as core's read E2E (solc 0.8.30, shanghai).
const mirrorStubRuntimeHex = "608060405234801561000f575f5ffd5b5060043610610029575f3560e01c8063f1c0c3731461002d575b5f5ffd5b61004061003b366004610162565b610057565b60405161004e929190610182565b60405180910390f35b6040805180820190915260aa815260bb602082015260029060609082851061008e575050604080515f81526020810190915261015b565b5f6100998587610200565b905060028111156100a8575060025b5f6100b38783610219565b90508067ffffffffffffffff8111156100ce576100ce6101d8565b6040519080825280602002602001820160405280156100f7578160200160208202803683370190505b5093505f5b81811015610156578361010f828a610200565b6002811061011f5761011f61022c565b60200201518582815181106101365761013661022c565b6001600160a01b03909216602092830291909101909101526001016100fc565b505050505b9250929050565b5f5f60408385031215610173575f5ffd5b50508035926020909101359150565b5f60408201848352604060208401528084518083526060850191506020860192505f5b818110156101cc5783516001600160a01b03168352602093840193909201916001016101a5565b50909695505050505050565b634e487b7160e01b5f52604160045260245ffd5b634e487b7160e01b5f52601160045260245ffd5b80820180821115610213576102136101ec565b92915050565b81810381811115610213576102136101ec565b634e487b7160e01b5f52603260045260245ffdfea2646970667358221220d0ef18377e6092f1e40ad9bba5331ff0fa61679d6304509867922df8ebab8b7b64736f6c634300081e0033"

// emitterStubRuntimeHex: poke(address to) emits ERC20 Transfer(msg.sender, to, 1)
// (solc 0.8.30, shanghai). poke selector = 0xb1a997ac.
const emitterStubRuntimeHex = "6080604052348015600e575f5ffd5b50600436106026575f3560e01c8063b1a997ac14602a575b5f5ffd5b60396035366004607f565b603b565b005b604051600181526001600160a01b0382169033907fddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef9060200160405180910390a350565b5f60208284031215608e575f5ffd5b81356001600160a01b038116811460a3575f5ffd5b939250505056fea264697066735822122074d87bd7902e6f10840828271600f75e146c41aff74402ebeb1a3f43c7f05f1864736f6c634300081e0033"

// TestCommitTransaction_BlacklistEventHitEndToEnd is the end-to-end build-path
// case (closes gap 2): a real MirrorStub is deployed so the gate reads the live
// list (0xAA, 0xBB) via getBlacklist; a tx then calls a real Emitter that emits
// ERC20 Transfer(_, 0xAA, _). The execution gate must detect the committed
// Transfer-event hit, return ErrBlacklistDrop, and the miner must mark the tx
// Rejected() (the pool then ejects it — see legacypool TestRejectedDropping).
// Difficulty==0 + ShanghaiTime==0 give the merge/Shanghai instruction set both
// the mirror and emitter (PUSH0) require.
func TestCommitTransaction_BlacklistEventHitEndToEnd(t *testing.T) {
	const chainID = params.XLayerMainnetChainID
	listed := common.BigToAddress(big.NewInt(0xAA)) // in the mirror's list
	config := &params.ChainConfig{
		ChainID:             big.NewInt(int64(chainID)),
		HomesteadBlock:      big.NewInt(0),
		EIP150Block:         big.NewInt(0),
		EIP155Block:         big.NewInt(0),
		EIP158Block:         big.NewInt(0),
		ByzantiumBlock:      big.NewInt(0),
		ConstantinopleBlock: big.NewInt(0),
		PetersburgBlock:     big.NewInt(0),
		IstanbulBlock:       big.NewInt(0),
		BerlinBlock:         big.NewInt(0),
		LondonBlock:         big.NewInt(0),
		ShanghaiTime:        new(uint64),
	}

	sdb, err := state.New(types.EmptyRootHash, state.NewDatabaseForTesting())
	if err != nil {
		t.Fatalf("state.New: %v", err)
	}
	mirror, ok := params.BlacklistMirror(chainID)
	if !ok {
		t.Fatal("chain not blacklist-enabled")
	}
	sdb.SetCode(mirror, common.FromHex(mirrorStubRuntimeHex), tracing.CodeChangeGenesis)
	emitter := common.HexToAddress("0x000000000000000000000000000000000000E111")
	sdb.SetCode(emitter, common.FromHex(emitterStubRuntimeHex), tracing.CodeChangeGenesis)

	key, _ := crypto.GenerateKey()
	from := crypto.PubkeyToAddress(key.PublicKey)
	sdb.AddBalance(from, uint256.NewInt(1e18), tracing.BalanceChangeUnspecified)

	// poke(listed): selector b1a997ac + left-padded address.
	calldata := append(common.FromHex("0xb1a997ac"), common.LeftPadBytes(listed.Bytes(), 32)...)
	signer := types.LatestSignerForChainID(big.NewInt(int64(chainID)))
	tx := types.MustSignNewTx(key, signer, &types.LegacyTx{
		Nonce: 0, To: &emitter, Gas: 100000, GasPrice: big.NewInt(1), Value: big.NewInt(0), Data: calldata,
	})

	header := &types.Header{Number: big.NewInt(1), Time: 1, Difficulty: big.NewInt(0), BaseFee: big.NewInt(0)}

	// Real gate: reads the list from the deployed mirror at block-head.
	gate := core.NewBlacklistGate(sdb, header, config, chainID)
	if gate == nil {
		t.Fatal("gate nil — mirror read returned empty (expected list {0xAA,0xBB})")
	}

	random := common.Hash{0x01} // Random set → merge/Shanghai instruction set
	blockCtx := vm.BlockContext{
		CanTransfer: core.CanTransfer,
		Transfer:    core.Transfer,
		GetHash:     func(uint64) common.Hash { return common.Hash{} },
		GasLimit:    30_000_000,
		BlockNumber: big.NewInt(1),
		Time:        1,
		Random:      &random,
		BaseFee:     big.NewInt(0),
	}
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
	// Mirror commitTransactions' per-tx setup (worker.go:804): attribute emitted
	// logs to this tx hash so the gate's statedb.GetLogs(tx.Hash()) sees them.
	sdb.SetTxContext(tx.Hash(), 0)
	err = miner.commitTransaction(context.Background(), env, tx)
	if !errors.Is(err, core.ErrBlacklistDrop) {
		t.Fatalf("err = %v, want core.ErrBlacklistDrop (Transfer-event hit must drop)", err)
	}
	if !tx.Rejected() {
		t.Fatal("event-hit tx must be marked Rejected() so the pool ejects it")
	}
}
