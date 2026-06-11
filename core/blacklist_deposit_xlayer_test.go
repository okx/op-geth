package core

import (
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
)

// depositTestConfig extends the build-path config with Optimism + Regolith (from
// genesis) so deposit txs and IsOptimismRegolith(time) are active.
func depositTestConfig() *params.ChainConfig {
	c := buildPathTestConfig()
	c.LondonBlock = big.NewInt(0)
	c.BedrockBlock = big.NewInt(0)
	c.RegolithTime = u64(0)
	c.Optimism = &params.OptimismConfig{EIP1559Elasticity: 6, EIP1559Denominator: 50}
	return c
}

func depositTx(from common.Address, to common.Address, mint, value int64, gas uint64) *types.Transaction {
	return types.NewTx(&types.DepositTx{
		SourceHash: common.Hash{0x01},
		From:       from,
		To:         &to,
		Mint:       big.NewInt(mint),
		Value:      big.NewInt(value),
		Gas:        gas,
	})
}

// TestDeposit_BlacklistedHit is the B1 regression anchor. A blacklisted deposit
// (value transfer to a listed address) must reproduce the canonical OP-Stack
// failed-deposit post-state: status=0, gasUsed=tx.Gas(), depositor nonce==N+1,
// mint kept, full gasLimit charged. This FAILS against the pre-fix blanket
// pre-ApplyMessage revert (which left nonce=N and reverted the mint).
func TestDeposit_BlacklistedHit(t *testing.T) {
	const chainID = params.XLayerMainnetChainID
	config := depositTestConfig()
	listed := common.HexToAddress("0x00000000000000000000000000000000000000AA")
	depositor := common.HexToAddress("0x00000000000000000000000000000000000000BB") // not exempt

	sdb := newTestStateDB(t)
	writeMirrorList(t, sdb, chainID, []common.Address{listed})
	gate := NewBlacklistGate(sdb, chainID)
	if gate == nil {
		t.Fatal("expected active gate")
	}
	evm := newBuildPathEVM(config, sdb, gate)

	const mint, gas = 1000, 100000
	tx := depositTx(depositor, listed, mint, 100, gas)
	gp := NewGasPool(30_000_000)

	receipt, err := ApplyTransactionGatedForBuild(gate, evm, gp, sdb, buildPathHeader(), tx)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if receipt.Status != types.ReceiptStatusFailed {
		t.Fatalf("status = %d, want 0 (included-as-reverted)", receipt.Status)
	}
	if receipt.GasUsed != tx.Gas() {
		t.Fatalf("gasUsed = %d, want %d (full gasLimit override)", receipt.GasUsed, tx.Gas())
	}
	if got := sdb.GetNonce(depositor); got != 1 {
		t.Fatalf("depositor nonce = %d, want 1 (deposit always increments, N+1)", got)
	}
	if !sdb.GetBalance(listed).IsZero() {
		t.Fatalf("listed balance = %s, want 0 (value transfer reverted)", sdb.GetBalance(listed))
	}
	if got := sdb.GetBalance(depositor).Uint64(); got != mint {
		t.Fatalf("depositor balance = %d, want %d (mint kept, natural failed-deposit semantics)", got, mint)
	}
	if got := gp.Used(); got != tx.Gas() {
		t.Fatalf("gas pool used = %d, want %d (full gasLimit charged)", got, tx.Gas())
	}
}

// TestDeposit_ExemptSenderNotIntercepted: a deposit from a deposit-exempt sender
// (system / L1-attributes) carrying a listed address is NEVER intercepted, even
// though it transfers value to the listed address (DM-3.2, IsDepositExemptSender
// short-circuit in evaluate).
func TestDeposit_ExemptSenderNotIntercepted(t *testing.T) {
	const chainID = params.XLayerMainnetChainID
	config := depositTestConfig()
	listed := common.HexToAddress("0x00000000000000000000000000000000000000AA")

	exemptSenders := []struct {
		name         string
		from         common.Address
		checkBalance bool // assert the value transfer landed (proves a real committed touch was waived)
	}{
		{"SystemAddress", params.SystemAddress, false},
		{"L1AttributesDepositor", common.HexToAddress("0xDeaDDEaDDeAdDeAdDEAdDEaddeAddEAdDEAd0001"), true},
	}
	for _, es := range exemptSenders {
		t.Run(es.name, func(t *testing.T) {
			sdb := newTestStateDB(t)
			writeMirrorList(t, sdb, chainID, []common.Address{listed})
			gate := NewBlacklistGate(sdb, chainID)
			evm := newBuildPathEVM(config, sdb, gate)

			tx := depositTx(es.from, listed, 1000, 100, 100000)
			receipt, err := ApplyTransactionGatedForBuild(gate, evm, NewGasPool(30_000_000), sdb, buildPathHeader(), tx)
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			// Core requirement: exempt sender is NOT intercepted (status=1). The
			// contrast with TestDeposit_BlacklistedHit (same listed target, a
			// NON-exempt depositor → status=0) proves the IsDepositExemptSender
			// short-circuit in evaluate is what waived interception here.
			if receipt.Status != types.ReceiptStatusSuccessful {
				t.Fatalf("status = %d, want 1 (exempt sender not intercepted)", receipt.Status)
			}
			// Strong positive control: where the deposit's value transfer to the
			// listed address actually commits, it is kept (not reverted) because
			// the sender is exempt.
			if es.checkBalance {
				if got := sdb.GetBalance(listed).Uint64(); got != 100 {
					t.Fatalf("listed balance = %d, want 100 (transfer kept; exempt, not intercepted)", got)
				}
			}
		})
	}
}

// TestImportPath_NormalHitStatusZero: on the import path (dropNormalHit=false), a
// committed normal-tx hit is NOT dropped — it is kept with a deterministic
// status=0 receipt (an adversarial block is validated identically across clients).
func TestImportPath_NormalHitStatusZero(t *testing.T) {
	const chainID = params.XLayerMainnetChainID
	config := buildPathTestConfig()
	listed := common.HexToAddress("0x00000000000000000000000000000000000000AA")

	sdb := newTestStateDB(t)
	tx, from := signValueTx(t, listed, 100)
	sdb.AddBalance(from, uint256.NewInt(1e18), 0 /* tracing.BalanceChangeUnspecified */)
	writeMirrorList(t, sdb, chainID, []common.Address{listed})

	gate := NewBlacklistGate(sdb, chainID)
	evm := newBuildPathEVM(config, sdb, gate)
	signer := types.MakeSigner(config, big.NewInt(1), 1)
	msg, err := TransactionToMessage(tx, signer, nil)
	if err != nil {
		t.Fatalf("TransactionToMessage: %v", err)
	}

	receipt, hit, err := applyTransactionWithBlacklistGate(
		gate, msg, NewGasPool(30_000_000), sdb, big.NewInt(1), common.Hash{}, 1, tx, evm, false /* dropNormalHit */)
	if err != nil {
		t.Fatalf("import path must not return an error for a normal hit, got %v", err)
	}
	if errors.Is(err, ErrBlacklistDrop) {
		t.Fatal("import path must NOT drop (dropNormalHit=false)")
	}
	if !hit {
		t.Fatal("expected hit")
	}
	if receipt.Status != types.ReceiptStatusFailed {
		t.Fatalf("status = %d, want 0 (import-path normal hit kept as status=0)", receipt.Status)
	}
	if !sdb.GetBalance(listed).IsZero() {
		t.Fatalf("listed balance = %s, want 0 (effects reverted)", sdb.GetBalance(listed))
	}
}
