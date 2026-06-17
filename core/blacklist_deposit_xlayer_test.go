package core

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
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

// canyonDepositTestConfig activates Canyon (and its Shanghai prerequisite) from
// genesis so MakeReceipt writes receipt.DepositReceiptVersion. The three XLayer
// networks are all past Canyon, so this is the production-representative config
// for the deposit receipt-version field.
func canyonDepositTestConfig() *params.ChainConfig {
	c := depositTestConfig()
	c.ShanghaiTime = u64(0)
	c.CanyonTime = u64(0)
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
	gate := NewBlacklistGateFromSnapshot(chainID, NewSnapshot([]common.Address{listed}))
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
	// receipt.DepositNonce encodes the PRE-exec nonce N (0 here), deliberately
	// NOT the account's post-state nonce N+1 (asserted as 1 just above). Both must
	// hold simultaneously or the receipts root diverges from xlayer-reth
	// (PRD FR-3 / I-6 — this field is the reth-alignment trap).
	if receipt.DepositNonce == nil {
		t.Fatal("receipt.DepositNonce = nil, want 0 (pre-exec N must be recorded under Regolith)")
	}
	if *receipt.DepositNonce != 0 {
		t.Fatalf("receipt.DepositNonce = %d, want 0 (pre-exec N, NOT account N+1=%d)", *receipt.DepositNonce, sdb.GetNonce(depositor))
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

// TestDeposit_BlacklistedHit_CanyonReceiptVersion covers the deposit receipt
// fields that enter the receipts root and are the primary xlayer-reth alignment
// trap (PRD FR-3 / I-6): with Canyon active, an intercepted deposit must carry
// receipt.DepositReceiptVersion == CanyonDepositReceiptVersion AND
// receipt.DepositNonce == pre-exec N, alongside status=0 / gasUsed=gasLimit.
// depositTestConfig() does not activate Canyon, so without this test the
// DepositReceiptVersion branch is never executed.
func TestDeposit_BlacklistedHit_CanyonReceiptVersion(t *testing.T) {
	const chainID = params.XLayerMainnetChainID
	config := canyonDepositTestConfig()
	listed := common.HexToAddress("0x00000000000000000000000000000000000000AA")
	depositor := common.HexToAddress("0x00000000000000000000000000000000000000BB")

	sdb := newTestStateDB(t)
	gate := NewBlacklistGateFromSnapshot(chainID, NewSnapshot([]common.Address{listed}))
	if gate == nil {
		t.Fatal("expected active gate")
	}
	evm := newBuildPathEVM(config, sdb, gate)

	tx := depositTx(depositor, listed, 1000, 100, 100000)
	receipt, err := ApplyTransactionGatedForBuild(gate, evm, NewGasPool(30_000_000), sdb, buildPathHeader(), tx)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if receipt.Status != types.ReceiptStatusFailed {
		t.Fatalf("status = %d, want 0 (included-as-reverted)", receipt.Status)
	}
	if receipt.GasUsed != tx.Gas() {
		t.Fatalf("gasUsed = %d, want %d (full gasLimit override)", receipt.GasUsed, tx.Gas())
	}
	if receipt.DepositNonce == nil || *receipt.DepositNonce != 0 {
		t.Fatalf("receipt.DepositNonce = %v, want 0 (pre-exec N)", receipt.DepositNonce)
	}
	if receipt.DepositReceiptVersion == nil {
		t.Fatal("receipt.DepositReceiptVersion = nil, want CanyonDepositReceiptVersion (Canyon active)")
	}
	if *receipt.DepositReceiptVersion != types.CanyonDepositReceiptVersion {
		t.Fatalf("receipt.DepositReceiptVersion = %d, want %d", *receipt.DepositReceiptVersion, types.CanyonDepositReceiptVersion)
	}
}

// TestDeposit_BlacklistedHit_CumulativeGasUsed is the C-1 regression: an
// intercepted deposit must report receipt.CumulativeGasUsed == tx.Gas() (full
// gasLimit), matching gp.Used() and the canonical failed-deposit accounting. A
// plain SubGas would leave CumulativeGasUsed at the natural consumed gas and
// diverge the receipts root from xlayer-reth.
func TestDeposit_BlacklistedHit_CumulativeGasUsed(t *testing.T) {
	const chainID = params.XLayerMainnetChainID
	config := depositTestConfig()
	listed := common.HexToAddress("0x00000000000000000000000000000000000000AA")
	depositor := common.HexToAddress("0x00000000000000000000000000000000000000BB")

	sdb := newTestStateDB(t)
	gate := NewBlacklistGateFromSnapshot(chainID, NewSnapshot([]common.Address{listed}))
	evm := newBuildPathEVM(config, sdb, gate)

	const gas = 100000
	tx := depositTx(depositor, listed, 1000, 100, gas)
	gp := NewGasPool(30_000_000)

	receipt, err := ApplyTransactionGatedForBuild(gate, evm, gp, sdb, buildPathHeader(), tx)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if receipt.CumulativeGasUsed != tx.Gas() {
		t.Fatalf("CumulativeGasUsed = %d, want %d (full gasLimit; C-1)", receipt.CumulativeGasUsed, tx.Gas())
	}
	if receipt.CumulativeGasUsed != gp.Used() {
		t.Fatalf("CumulativeGasUsed (%d) != gp.Used() (%d) — receipt/header gas mismatch", receipt.CumulativeGasUsed, gp.Used())
	}
}

// TestDeposit_BlacklistedHit_CumulativeAcrossTxs: on a shared block gas pool, a
// tx following an intercepted deposit must see CumulativeGasUsed advance by the
// deposit's full gasLimit (not its natural usage). This is the build-path
// (worker) coverage of the cumulative C-1 fix across multiple txs.
func TestDeposit_BlacklistedHit_CumulativeAcrossTxs(t *testing.T) {
	const chainID = params.XLayerMainnetChainID
	config := depositTestConfig()
	listed := common.HexToAddress("0x00000000000000000000000000000000000000AA")
	depositor := common.HexToAddress("0x00000000000000000000000000000000000000BB")
	recipient := common.HexToAddress("0x00000000000000000000000000000000000000CC") // not listed

	sdb := newTestStateDB(t)
	gate := NewBlacklistGateFromSnapshot(chainID, NewSnapshot([]common.Address{listed}))
	evm := newBuildPathEVM(config, sdb, gate)
	gp := NewGasPool(30_000_000) // shared across both txs

	const depGas = 100000
	dep := depositTx(depositor, listed, 1000, 100, depGas)
	r1, err := ApplyTransactionGatedForBuild(gate, evm, gp, sdb, buildPathHeader(), dep)
	if err != nil {
		t.Fatalf("deposit apply err: %v", err)
	}
	if r1.CumulativeGasUsed != dep.Gas() {
		t.Fatalf("deposit CumulativeGasUsed = %d, want %d", r1.CumulativeGasUsed, dep.Gas())
	}

	normalTx, from := signValueTx(t, recipient, 100)
	sdb.AddBalance(from, uint256.NewInt(1e18), 0 /* BalanceChangeUnspecified */)
	r2, err := ApplyTransactionGatedForBuild(gate, evm, gp, sdb, buildPathHeader(), normalTx)
	if err != nil {
		t.Fatalf("normal apply err: %v", err)
	}
	if r2.Status != types.ReceiptStatusSuccessful {
		t.Fatalf("normal tx status = %d, want 1", r2.Status)
	}
	if want := r1.CumulativeGasUsed + r2.GasUsed; r2.CumulativeGasUsed != want {
		t.Fatalf("second CumulativeGasUsed = %d, want %d (= deposit full gasLimit %d + normal %d)",
			r2.CumulativeGasUsed, want, r1.CumulativeGasUsed, r2.GasUsed)
	}
}

// TestDeposit_NaturalSuccess_CumulativeUnchanged: control — a non-intercepted
// (successful) deposit is unaffected by the C-1 fix; its CumulativeGasUsed
// equals its own natural GasUsed (single tx in the pool).
func TestDeposit_NaturalSuccess_CumulativeUnchanged(t *testing.T) {
	const chainID = params.XLayerMainnetChainID
	config := depositTestConfig()
	depositor := common.HexToAddress("0x00000000000000000000000000000000000000BB")
	recipient := common.HexToAddress("0x00000000000000000000000000000000000000CC") // not listed

	sdb := newTestStateDB(t)
	gate := NewBlacklistGateFromSnapshot(chainID, NewSnapshot(nil)) // empty list → nil gate (no hit possible)
	evm := newBuildPathEVM(config, sdb, gate)
	gp := NewGasPool(30_000_000)

	tx := depositTx(depositor, recipient, 1000, 100, 100000)
	receipt, err := ApplyTransactionGatedForBuild(gate, evm, gp, sdb, buildPathHeader(), tx)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if receipt.Status != types.ReceiptStatusSuccessful {
		t.Fatalf("status = %d, want 1 (not intercepted)", receipt.Status)
	}
	if receipt.CumulativeGasUsed != receipt.GasUsed {
		t.Fatalf("CumulativeGasUsed = %d, want == GasUsed %d (single-tx, natural)", receipt.CumulativeGasUsed, receipt.GasUsed)
	}
}

// TestDeposit_BlacklistedHit_ExtraZero exercises the `extra > 0` guard's false
// edge: when the deposit's natural gas usage already equals its gasLimit
// (extra==0), ChargeUsed is skipped, yet CumulativeGasUsed must already equal
// the full gasLimit (no double count). gasLimit is set to the intrinsic gas of a
// bare value transfer (21000).
func TestDeposit_BlacklistedHit_ExtraZero(t *testing.T) {
	const chainID = params.XLayerMainnetChainID
	config := depositTestConfig()
	listed := common.HexToAddress("0x00000000000000000000000000000000000000AA")
	depositor := common.HexToAddress("0x00000000000000000000000000000000000000BB")

	sdb := newTestStateDB(t)
	gate := NewBlacklistGateFromSnapshot(chainID, NewSnapshot([]common.Address{listed}))
	evm := newBuildPathEVM(config, sdb, gate)

	const gas = params.TxGas // 21000 — intrinsic of a value transfer; deposit uses exactly this, so extra==0
	tx := depositTx(depositor, listed, 1000, 100, gas)
	gp := NewGasPool(30_000_000)

	receipt, err := ApplyTransactionGatedForBuild(gate, evm, gp, sdb, buildPathHeader(), tx)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if receipt.Status != types.ReceiptStatusFailed {
		t.Fatalf("status = %d, want 0", receipt.Status)
	}
	if receipt.CumulativeGasUsed != tx.Gas() {
		t.Fatalf("CumulativeGasUsed = %d, want %d (extra==0: cumulative already == gasLimit)", receipt.CumulativeGasUsed, tx.Gas())
	}
	if gp.Used() != tx.Gas() {
		t.Fatalf("gp.Used() = %d, want %d", gp.Used(), tx.Gas())
	}
}

// TestDeposit_PureCallTouch_NotIntercepted locks decision B's coverage edge
// (XLOP-1100): a deposit that only CALL-touches a listed address (value=0,
// mint=0, no Transfer event, no ETH movement) is NO LONGER intercepted now that
// check① is dropped for deposits — it is included normally (status=1). Under the
// old all-three-checks path this same deposit was included-as-reverted (status=0),
// so this test fails if the deposit gate still runs check①.
func TestDeposit_PureCallTouch_NotIntercepted(t *testing.T) {
	const chainID = params.XLayerMainnetChainID
	config := depositTestConfig()
	listed := common.HexToAddress("0x00000000000000000000000000000000000000AA")
	depositor := common.HexToAddress("0x00000000000000000000000000000000000000BB")

	sdb := newTestStateDB(t)
	gate := NewBlacklistGateFromSnapshot(chainID, NewSnapshot([]common.Address{listed}))
	if gate == nil {
		t.Fatal("expected active gate")
	}
	evm := newBuildPathEVM(config, sdb, gate)

	// to=listed, value=0, mint=0 → only a top-level CALL touch of the listed
	// address; no Transfer event, no ETH balance change, so check②/③ do not fire.
	tx := depositTx(depositor, listed, 0, 0, 100000)
	gp := NewGasPool(30_000_000)

	receipt, err := ApplyTransactionGatedForBuild(gate, evm, gp, sdb, buildPathHeader(), tx)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if receipt.Status != types.ReceiptStatusSuccessful {
		t.Fatalf("status = %d, want 1 (pure CALL-touch deposit no longer intercepted)", receipt.Status)
	}
}

// emitTransferToAAARuntime is hand-written runtime bytecode (no PUSH0, so it runs
// on the build-path EVM's pre-Shanghai instruction set) that always emits
// ERC20 Transfer(msg.sender, 0xAA, 1) via LOG3. Used to drive a real check②
// (committed Transfer event) hit on a deposit.
//
//	PUSH1 1; PUSH1 0; MSTORE              ; mem[0:32]=1 (value)
//	PUSH32 0x..AA                         ; topic2 = to = 0xAA (listed)
//	CALLER                                ; topic1 = from = msg.sender
//	PUSH32 <Transfer sig>                 ; topic0
//	PUSH1 0x20; PUSH1 0; LOG3; STOP
const emitTransferToAAARuntime = "60016000527f00000000000000000000000000000000000000000000000000000000000000aa337fddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef60206000a300"

// TestDeposit_EventHit_IncludedAsReverted is the end-to-end deposit check②
// (Transfer event) case: a deposit calls a non-listed contract that emits a real
// committed Transfer(_, 0xAA) event. With check① skipped for deposits, the hit
// comes solely from check② (no value moved → no check③), and the deposit must be
// included-as-reverted (status=0, gasUsed=gasLimit). Pairs the event-path
// judgment with the included-as-reverted processing end to end.
func TestDeposit_EventHit_IncludedAsReverted(t *testing.T) {
	const chainID = params.XLayerMainnetChainID
	config := depositTestConfig()
	listed := common.HexToAddress("0x00000000000000000000000000000000000000AA") // == topic2 in the stub
	depositor := common.HexToAddress("0x00000000000000000000000000000000000000BB")
	emitter := common.HexToAddress("0x00000000000000000000000000000000000000E1") // not listed

	sdb := newTestStateDB(t)
	sdb.SetCode(emitter, common.FromHex(emitTransferToAAARuntime), tracing.CodeChangeGenesis)
	gate := NewBlacklistGateFromSnapshot(chainID, NewSnapshot([]common.Address{listed}))
	if gate == nil {
		t.Fatal("expected active gate")
	}
	evm := newBuildPathEVM(config, sdb, gate)

	// deposit to the emitter, value=0: the only committed effect is the emitted
	// Transfer(_, 0xAA) → check② hit (check① skipped, check③ no balance move).
	tx := depositTx(depositor, emitter, 0, 0, 100000)
	sdb.SetTxContext(tx.Hash(), 0) // logs are indexed by tx hash; gate reads GetLogs(tx.Hash())
	gp := NewGasPool(30_000_000)

	receipt, err := ApplyTransactionGatedForBuild(gate, evm, gp, sdb, buildPathHeader(), tx)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if receipt.Status != types.ReceiptStatusFailed {
		t.Fatalf("status = %d, want 0 (deposit Transfer-event hit → included-as-reverted)", receipt.Status)
	}
	if receipt.GasUsed != tx.Gas() {
		t.Fatalf("gasUsed = %d, want %d (full gasLimit override)", receipt.GasUsed, tx.Gas())
	}
}

// selfdestructToAAARuntime is runtime bytecode `PUSH20 0x..AA; SELFDESTRUCT`
// that sends the contract's whole balance to the listed beneficiary 0xAA. No
// PUSH0, so it runs on the build-path EVM's pre-Shanghai instruction set. Used to
// drive a real check③ selfdestruct hit end to end.
const selfdestructToAAARuntime = "7300000000000000000000000000000000000000aaff"

// TestDeposit_SelfdestructBeneficiary_IncludedAsReverted is the end-to-end check③
// selfdestruct case (AC FR-2 names selfdestruct beneficiary explicitly): a deposit
// funds a non-listed contract that selfdestructs, sending its balance to the listed
// 0xAA. The committed beneficiary balance movement trips check③ (category
// selfdestruct), and the deposit is included-as-reverted (status=0, beneficiary
// balance reverted to 0). Complements the pure-function DM-2.12 coverage with a
// real SELFDESTRUCT opcode through the gated apply path.
func TestDeposit_SelfdestructBeneficiary_IncludedAsReverted(t *testing.T) {
	const chainID = params.XLayerMainnetChainID
	config := depositTestConfig()
	listed := common.HexToAddress("0x00000000000000000000000000000000000000AA")    // selfdestruct beneficiary
	depositor := common.HexToAddress("0x00000000000000000000000000000000000000BB") // not exempt
	victim := common.HexToAddress("0x00000000000000000000000000000000000000E2")    // contract, not listed

	sdb := newTestStateDB(t)
	sdb.SetCode(victim, common.FromHex(selfdestructToAAARuntime), tracing.CodeChangeGenesis)
	gate := NewBlacklistGateFromSnapshot(chainID, NewSnapshot([]common.Address{listed}))
	if gate == nil {
		t.Fatal("expected active gate")
	}
	evm := newBuildPathEVM(config, sdb, gate)

	// deposit value=100 to the contract; on execution it selfdestructs, sending
	// 100 to 0xAA → committed beneficiary balance movement → check③ hit.
	tx := depositTx(depositor, victim, 1000, 100, 100000)
	sdb.SetTxContext(tx.Hash(), 0)
	gp := NewGasPool(30_000_000)

	receipt, err := ApplyTransactionGatedForBuild(gate, evm, gp, sdb, buildPathHeader(), tx)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if receipt.Status != types.ReceiptStatusFailed {
		t.Fatalf("status = %d, want 0 (selfdestruct beneficiary listed → included-as-reverted)", receipt.Status)
	}
	if receipt.GasUsed != tx.Gas() {
		t.Fatalf("gasUsed = %d, want %d (full gasLimit override)", receipt.GasUsed, tx.Gas())
	}
	if !sdb.GetBalance(listed).IsZero() {
		t.Fatalf("listed beneficiary balance = %s, want 0 (selfdestruct transfer reverted)", sdb.GetBalance(listed))
	}
}

// TestImportPath_DepositHit_CumulativeAcrossTxs covers the IMPORT path
// (dropNormalHit=false, the StateProcessor.Process entry) cumulative behaviour:
// an intercepted deposit followed by a normal tx on a shared gas pool advances
// CumulativeGasUsed by the deposit's full gasLimit. Mirrors the build-path
// coverage on the other apply entry.
func TestImportPath_DepositHit_CumulativeAcrossTxs(t *testing.T) {
	const chainID = params.XLayerMainnetChainID
	config := depositTestConfig()
	listed := common.HexToAddress("0x00000000000000000000000000000000000000AA")
	depositor := common.HexToAddress("0x00000000000000000000000000000000000000BB")
	recipient := common.HexToAddress("0x00000000000000000000000000000000000000CC")

	sdb := newTestStateDB(t)
	gate := NewBlacklistGateFromSnapshot(chainID, NewSnapshot([]common.Address{listed}))
	evm := newBuildPathEVM(config, sdb, gate)
	gp := NewGasPool(30_000_000)
	signer := types.MakeSigner(config, big.NewInt(1), 1)

	dep := depositTx(depositor, listed, 1000, 100, 100000)
	dmsg, err := TransactionToMessage(dep, signer, nil)
	if err != nil {
		t.Fatalf("deposit msg: %v", err)
	}
	r1, hit, err := applyTransactionWithBlacklistGate(gate, dmsg, gp, sdb, big.NewInt(1), common.Hash{}, 1, dep, evm, false /* import path */)
	if err != nil {
		t.Fatalf("import-path deposit err: %v", err)
	}
	if !hit || r1.Status != types.ReceiptStatusFailed {
		t.Fatalf("deposit not intercepted on import path (hit=%v status=%d)", hit, r1.Status)
	}
	if r1.CumulativeGasUsed != dep.Gas() {
		t.Fatalf("deposit CumulativeGasUsed = %d, want %d", r1.CumulativeGasUsed, dep.Gas())
	}

	normalTx, from := signValueTx(t, recipient, 100)
	sdb.AddBalance(from, uint256.NewInt(1e18), 0)
	nmsg, err := TransactionToMessage(normalTx, signer, nil)
	if err != nil {
		t.Fatalf("normal msg: %v", err)
	}
	r2, _, err := applyTransactionWithBlacklistGate(gate, nmsg, gp, sdb, big.NewInt(1), common.Hash{}, 1, normalTx, evm, false)
	if err != nil {
		t.Fatalf("import-path normal err: %v", err)
	}
	if r2.Status != types.ReceiptStatusSuccessful {
		t.Fatalf("normal tx status = %d, want 1", r2.Status)
	}
	if want := r1.CumulativeGasUsed + r2.GasUsed; r2.CumulativeGasUsed != want {
		t.Fatalf("second CumulativeGasUsed = %d, want %d (import path)", r2.CumulativeGasUsed, want)
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
			gate := NewBlacklistGateFromSnapshot(chainID, NewSnapshot([]common.Address{listed}))
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

// TestApplyTransactionDispatch_L2HitNotIntercepted drives the actual import
// entry point (applyTransactionDispatch, used by StateProcessor.Process): an L2
// tx touching a listed address is executed normally (status=1, transfer kept) —
// the follower does not intercept L2 txs.
func TestApplyTransactionDispatch_L2HitNotIntercepted(t *testing.T) {
	const chainID = params.XLayerMainnetChainID
	config := buildPathTestConfig()
	listed := common.HexToAddress("0x00000000000000000000000000000000000000AA")

	sdb := newTestStateDB(t)
	tx, from := signValueTx(t, listed, 100)
	sdb.AddBalance(from, uint256.NewInt(1e18), 0)
	gate := NewBlacklistGateFromSnapshot(chainID, NewSnapshot([]common.Address{listed}))
	evm := newBuildPathEVM(config, sdb, gate)
	signer := types.MakeSigner(config, big.NewInt(1), 1)
	msg, err := TransactionToMessage(tx, signer, nil)
	if err != nil {
		t.Fatalf("TransactionToMessage: %v", err)
	}

	receipt, err := applyTransactionDispatch(gate, msg, NewGasPool(30_000_000), sdb, big.NewInt(1), common.Hash{}, 1, tx, evm)
	if err != nil {
		t.Fatalf("dispatch err: %v", err)
	}
	if receipt.Status != types.ReceiptStatusSuccessful {
		t.Fatalf("status = %d, want 1 (import dispatch must not intercept L2 tx)", receipt.Status)
	}
	if got := sdb.GetBalance(listed).Uint64(); got != 100 {
		t.Fatalf("listed balance = %d, want 100 (transfer kept)", got)
	}
}

// TestImportPath_L2NormalHit_NotIntercepted: on the import path
// (dropNormalHit=false), an L2 (non-deposit) tx that touches a listed address is
// NOT intercepted by the follower. L2 interception is the sequencer's job (it
// drops such txs at build time); a follower executes the block as-is and follows
// the sequencer. So the tx executes normally: status=1, value transfer kept,
// sender nonce bumped — and the gate reports hit=false (no interception action).
func TestImportPath_L2NormalHit_NotIntercepted(t *testing.T) {
	const chainID = params.XLayerMainnetChainID
	config := buildPathTestConfig()
	listed := common.HexToAddress("0x00000000000000000000000000000000000000AA")

	sdb := newTestStateDB(t)
	tx, from := signValueTx(t, listed, 100)
	const startBal = uint64(1e18)
	sdb.AddBalance(from, uint256.NewInt(startBal), 0 /* tracing.BalanceChangeUnspecified */)
	gate := NewBlacklistGateFromSnapshot(chainID, NewSnapshot([]common.Address{listed}))
	evm := newBuildPathEVM(config, sdb, gate)
	signer := types.MakeSigner(config, big.NewInt(1), 1)
	msg, err := TransactionToMessage(tx, signer, nil)
	if err != nil {
		t.Fatalf("TransactionToMessage: %v", err)
	}

	receipt, hit, err := applyTransactionWithBlacklistGate(
		gate, msg, NewGasPool(30_000_000), sdb, big.NewInt(1), common.Hash{}, 1, tx, evm, false /* dropNormalHit */)
	if err != nil {
		t.Fatalf("import path err: %v", err)
	}
	if hit {
		t.Fatal("L2 normal tx must NOT be intercepted on the import path (follower follows seq)")
	}
	if receipt.Status != types.ReceiptStatusSuccessful {
		t.Fatalf("status = %d, want 1 (L2 tx executed normally, not intercepted)", receipt.Status)
	}
	if got := sdb.GetBalance(listed).Uint64(); got != 100 {
		t.Fatalf("listed balance = %d, want 100 (transfer kept; not intercepted)", got)
	}
	if got := sdb.GetNonce(from); got != 1 {
		t.Fatalf("sender nonce = %d, want 1 (tx applied normally)", got)
	}
	// Sender paid value + gas fee, not reverted (tx executed normally). With
	// GasPrice=1 (signValueTx) the gas fee equals receipt.GasUsed.
	wantSenderBal := startBal - 100 - receipt.GasUsed
	if got := sdb.GetBalance(from).Uint64(); got != wantSenderBal {
		t.Fatalf("sender balance = %d, want %d (start %d - value 100 - gas %d)", got, wantSenderBal, startBal, receipt.GasUsed)
	}
}
