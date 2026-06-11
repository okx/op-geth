// XLayer emergency-freeze blacklist — execution-gate STF integration
// (XLOP-1099, FR-2 / FR-3 / FR-4 / FR-5). Fork-local XLayer extension (KG naming
// rule).
//
// The gate runs on the SAME per-tx decision anchor for both block import
// (StateProcessor.Process) and block building (miner), satisfying the TD R-1
// single-anchor consensus requirement: both paths read the block-head snapshot,
// multiplex the same observational tracer, and call gate.evaluate. Detection is
// observational (core/tracing.Hooks, cannot abort); the decision is made AFTER
// ApplyMessage returns, using committed effects only.
//
// Outcome differs by path for a committed NORMAL-tx hit (a tx that should never
// be in an honest block):
//   - import path  (dropNormalHit=false): keep as included-with status=0 so a
//     follower validating an adversarial block produces a deterministic result.
//   - build path   (dropNormalHit=true):  signal the miner to drop the tx from
//     the block entirely (ErrBlacklistDrop) and eject it from the mempool; the
//     miner's outer snapshot fully undoes state + gas.
// A committed DEPOSIT hit is identical on both paths: included-as-reverted with
// status=0, gasUsed=tx.Gas(), full gasLimit charged to the block.
//
// Safety note: while a chain is "blacklist enabled" by chain_id, the gate is a
// strict no-op until a non-empty L2BlacklistMirror is deployed (empty snapshot
// short-circuits). Byte-for-byte cross-client (xlayer-reth) parity of the
// revert + receipt-override path is validated against the shared adversarial
// vectors (TD §11.4 B-2) and the e2e harness before network-wide enablement
// (real mirror addresses, B-1).

package core

import (
	"errors"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
)

// ErrBlacklistDrop is an internal control signal returned by the build-path
// gated apply when a committed NORMAL-tx hit must be dropped from the block. It
// is not user-facing; the miner matches it with errors.Is to drop + reject the
// tx. Deposit hits never produce this (they are kept as included-as-reverted).
var ErrBlacklistDrop = errors.New("xlayer-blacklist: transaction dropped from block (committed blacklist hit)")

// BlacklistGate carries the block-head snapshot and per-block tracer for the
// execution gate. It is nil when the chain is disabled or the list is empty, in
// which case callers use the unmodified apply path (zero hot-path cost).
type BlacklistGate struct {
	chainID uint64
	snap    *Snapshot
	tracer  *BlacklistTracer
}

// NewBlacklistGate reads the block-head snapshot for chainID from the parent
// state and returns a gate, or nil if the chain is disabled or the list empty.
// statedb MUST be the block-head/parent state (read once before the tx loop).
func NewBlacklistGate(statedb vm.StateDB, chainID uint64) *BlacklistGate {
	if !params.IsBlacklistEnabled(chainID) {
		return nil
	}
	start := time.Now()
	snap := ReadBlacklistSnapshot(statedb, chainID)
	MetricBlacklistSnapshotRead(time.Since(start).Nanoseconds())
	MetricBlacklistCacheSize(snap.Size())
	if snap.Size() == 0 {
		return nil
	}
	return &BlacklistGate{chainID: chainID, snap: snap, tracer: NewBlacklistTracer()}
}

// Hooks returns the tracer hooks to multiplex onto the EVM config.
func (g *BlacklistGate) Hooks() *tracing.Hooks { return g.tracer.Hooks() }

// CombineBlacklistHooks merges the blacklist tracer hooks onto an existing
// (possibly nil) tracer so debug/monitor tracing is preserved (TD R-8). Only
// the hooks the blacklist tracer consumes are chained; all other base hooks are
// passed through unchanged.
func CombineBlacklistHooks(base, add *tracing.Hooks) *tracing.Hooks {
	if base == nil {
		return add
	}
	merged := *base // shallow copy preserves all base hooks
	merged.OnTxStart = chainTxStart(base.OnTxStart, add.OnTxStart)
	merged.OnEnter = chainEnter(base.OnEnter, add.OnEnter)
	merged.OnExit = chainExit(base.OnExit, add.OnExit)
	merged.OnBalanceChange = chainBalanceChange(base.OnBalanceChange, add.OnBalanceChange)
	return &merged
}

func chainTxStart(a, b tracing.TxStartHook) tracing.TxStartHook {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	return func(vmctx *tracing.VMContext, tx *types.Transaction, from common.Address) {
		a(vmctx, tx, from)
		b(vmctx, tx, from)
	}
}

func chainEnter(a, b tracing.EnterHook) tracing.EnterHook {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	return func(depth int, typ byte, from, to common.Address, input []byte, gas uint64, value *big.Int) {
		a(depth, typ, from, to, input, gas, value)
		b(depth, typ, from, to, input, gas, value)
	}
}

func chainExit(a, b tracing.ExitHook) tracing.ExitHook {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	return func(depth int, output []byte, gasUsed uint64, err error, reverted bool) {
		a(depth, output, gasUsed, err, reverted)
		b(depth, output, gasUsed, err, reverted)
	}
}

func chainBalanceChange(a, b tracing.BalanceChangeHook) tracing.BalanceChangeHook {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	return func(addr common.Address, prev, newBal *big.Int, reason tracing.BalanceChangeReason) {
		a(addr, prev, newBal, reason)
		b(addr, prev, newBal, reason)
	}
}

// applyTransactionDispatch routes an import-path tx through the blacklist-gated
// apply path when a gate is active, or the unmodified ApplyTransactionWithEVM
// otherwise. Import path keeps normal-tx hits as status=0 (dropNormalHit=false).
func applyTransactionDispatch(gate *BlacklistGate, msg *Message, gp *GasPool, statedb *state.StateDB, blockNumber *big.Int, blockHash common.Hash, blockTime uint64, tx *types.Transaction, evm *vm.EVM) (*types.Receipt, error) {
	if gate == nil {
		return ApplyTransactionWithEVM(msg, gp, statedb, blockNumber, blockHash, blockTime, tx, evm)
	}
	receipt, _, err := applyTransactionWithBlacklistGate(gate, msg, gp, statedb, blockNumber, blockHash, blockTime, tx, evm, false)
	return receipt, err
}

// ApplyTransactionGatedForBuild is the build-path (miner) entry point. It mirrors
// core.ApplyTransaction (header-based message construction) but runs the SAME
// blacklist gate as the import path. A committed NORMAL-tx hit returns
// ErrBlacklistDrop (no Finalise) so the caller's outer snapshot fully undoes the
// tx and drops it from the block; a committed DEPOSIT hit is kept as
// included-as-reverted (status=0, gasUsed=gasLimit). With a nil gate it falls
// back to core.ApplyTransaction.
func ApplyTransactionGatedForBuild(gate *BlacklistGate, evm *vm.EVM, gp *GasPool, statedb *state.StateDB, header *types.Header, tx *types.Transaction) (*types.Receipt, error) {
	if gate == nil {
		return ApplyTransaction(evm, gp, statedb, header, tx)
	}
	msg, err := TransactionToMessage(tx, types.MakeSigner(evm.ChainConfig(), header.Number, header.Time), header.BaseFee)
	if err != nil {
		return nil, err
	}
	receipt, _, err := applyTransactionWithBlacklistGate(gate, msg, gp, statedb, header.Number, header.Hash(), header.Time, tx, evm, true)
	return receipt, err
}

// applyTransactionWithBlacklistGate mirrors ApplyTransactionWithEVM but inserts
// the blacklist execution gate between ApplyMessage and Finalise. See the file
// header for the per-path outcome matrix. System / L1-attributes deposits
// (IsDepositExemptSender) are never gated.
func applyTransactionWithBlacklistGate(gate *BlacklistGate, msg *Message, gp *GasPool, statedb *state.StateDB, blockNumber *big.Int, blockHash common.Hash, blockTime uint64, tx *types.Transaction, evm *vm.EVM, dropNormalHit bool) (receipt *types.Receipt, hit bool, err error) {
	if hooks := evm.Config.Tracer; hooks != nil {
		if hooks.OnTxStart != nil {
			hooks.OnTxStart(evm.GetVMContext(), tx, msg.From)
		}
		if hooks.OnTxEnd != nil {
			defer func() { hooks.OnTxEnd(receipt, err) }()
		}
	}

	nonce := tx.Nonce()
	if msg.IsDepositTx && evm.ChainConfig().IsOptimismRegolith(evm.Context.Time) {
		nonce = statedb.GetNonce(msg.From)
	}

	// Snapshot BEFORE execution so a committed hit can be fully reverted (the
	// pre-tx snapshot is only valid until Finalise clears the journal).
	snapID := statedb.Snapshot()

	result, err := ApplyMessage(evm, msg, gp)
	if err != nil {
		return nil, false, err
	}

	var category string
	hit, category = gate.evaluate(msg, tx, statedb, blockNumber, blockHash, blockTime)
	if hit {
		MetricBlacklistExecRevert(category)
		// Build path: a committed normal-tx hit is dropped from the block. Return
		// before Finalise so the caller's outer snapshot can fully undo state+gas.
		// The miner logs this drop (worker.go), so we do NOT log here to avoid a
		// duplicate; all other hit shapes are logged just below.
		if dropNormalHit && !tx.IsDepositTx() {
			return nil, true, ErrBlacklistDrop
		}
		// M2: per-tx structured WARN for the import path (and every kept hit) so a
		// chain-level security interception is traceable from logs alone (R6.5),
		// closing the build/import log asymmetry.
		log.Warn("xlayer-blacklist: tx intercepted at exec gate",
			"hash", tx.Hash(), "chainID", gate.chainID, "category", category, "deposit", tx.IsDepositTx())
		statedb.RevertToSnapshot(snapID)
		if tx.IsDepositTx() {
			// B1 fix — reproduce the canonical OP-Stack *failed-deposit* post-state
			// (core/state_transition.go:474-512), NOT a blanket pre-ApplyMessage
			// revert. A naturally-failed deposit (a) KEEPS the deposit-mint and
			// (b) ALWAYS increments the depositor nonce. The RevertToSnapshot above
			// undid both, so we re-apply them:
			//   - MINT TREATMENT = KEEP-MINT (natural deposit-failure semantics,
			//     TD §4.4.3 「deposit 既有失败语义保留」). This is the lowest
			//     cross-client-divergence choice: xlayer-reth's existing
			//     failed-deposit path produces the identical post-state for free,
			//     so seq/follower/op-geth/reth all agree byte-for-byte (FR-5). The
			//     re-mint cannot overflow — ApplyMessage already applied it once.
			//   - NONCE: always bump to N+1 (mirrors state_transition.go:494).
			// Net effect == a naturally-failed deposit: mint kept, nonce N+1, call
			// effects reverted, status=0, gasUsed=gasLimit, full gasLimit charged.
			if msg.Mint != nil {
				if mintU256, overflow := uint256.FromBig(msg.Mint); !overflow {
					statedb.AddBalance(msg.From, mintU256, tracing.BalanceMint)
				}
			}
			statedb.SetNonce(msg.From, nonce+1, tracing.NonceChangeEoACall)
			// Count the full gasLimit into the block (DM-3.6): consume the gas not
			// already consumed by the (now-reverted) execution.
			if extra := tx.Gas() - result.UsedGas; extra > 0 {
				_ = gp.SubGas(extra)
			}
		}
	}

	var root []byte
	if evm.ChainConfig().IsByzantium(blockNumber) {
		evm.StateDB.Finalise(true)
	} else {
		root = statedb.IntermediateRoot(evm.ChainConfig().IsEIP158(blockNumber)).Bytes()
	}
	if statedb.Database().TrieDB().IsVerkle() {
		statedb.AccessEvents().Merge(evm.AccessEvents)
	}

	receipt = MakeReceipt(evm, result, statedb, blockNumber, blockHash, blockTime, tx, gp.CumulativeUsed(), root, evm.ChainConfig(), nonce)
	if hit {
		receipt.Status = types.ReceiptStatusFailed
		if tx.IsDepositTx() {
			// Explicit gasUsed override — must not rely on natural Regolith
			// consumed-gas (TD §4.4.3 / PRD I-6), else receipts root diverges.
			receipt.GasUsed = tx.Gas()
		}
		// Logs were reverted, so recompute the (now empty) committed log set.
		receipt.Logs = statedb.GetLogs(tx.Hash(), blockNumber.Uint64(), blockHash, blockTime)
		receipt.Bloom = types.CreateBloom(receipt)
	}
	return receipt, hit, nil
}

// evaluate runs the three committed-effect checks for one tx. Deposits from the
// exempt-sender set (system / L1-attributes) are never intercepted (FR-3 AC2).
func (g *BlacklistGate) evaluate(msg *Message, tx *types.Transaction, statedb *state.StateDB, blockNumber *big.Int, blockHash common.Hash, blockTime uint64) (bool, string) {
	if msg.IsDepositTx && params.IsDepositExemptSender(msg.From) {
		return false, ""
	}
	logs := statedb.GetLogs(tx.Hash(), blockNumber.Uint64(), blockHash, blockTime)
	return g.tracer.Evaluate(g.snap, logs, statedb.GetBalance)
}
