// XLayer emergency-freeze blacklist — execution-gate decision logic
// (XLOP-1099, FR-2 / FR-3 / FR-4 / FR-5). Fork-local XLayer extension (KG naming
// rule).
//
// The gate is injected IN-PLACE into the shared apply path (ApplyTransactionWithEVM)
// rather than via a duplicated apply function: both block import
// (StateProcessor.Process) and block building (miner, via ApplyTransaction) pass
// a *BlacklistGate into ApplyTransactionWithEVM, which calls gate.decide between
// ApplyMessage and Finalise and gate.overrideReceipt after MakeReceipt. A nil
// gate (disabled chain / empty list) is the unmodified upstream path. Detection
// is observational (core/tracing.Hooks, cannot abort); the decision uses
// committed effects only, AFTER ApplyMessage returns.
//
// Outcome differs by path for a committed NORMAL (L2, non-deposit) tx hit, keyed
// off gate.dropNormalHit (set at construction):
//   - build path (dropNormalHit=true): decide returns ErrBlacklistDrop so
//     ApplyTransactionWithEVM returns before Finalise; the miner's outer snapshot
//     fully undoes state + gas and drops the tx from the block + ejects it from
//     the mempool. The sequencer is the sole enforcement point for L2 txs.
//   - import path (dropNormalHit=false): NOT intercepted — the follower executes
//     the L2 tx as-is and follows the sequencer (force-failing an already-included
//     L2 tx would leave an inconsistent post-state and risk cross-client fork).
// A committed DEPOSIT hit is identical on both paths: included-as-reverted with
// status=0, gasUsed=tx.Gas(), full gasLimit charged — L1->L2 deposits bypass the
// sequencer (forced inclusion) and must be gated by consensus.

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

// ErrBlacklistDrop is an internal control signal returned by the gated apply path
// when a committed NORMAL-tx hit must be dropped from the block (build path
// only). It is not user-facing; the miner matches it with errors.Is to drop +
// reject the tx. Deposit hits never produce this (they are kept as
// included-as-reverted).
var ErrBlacklistDrop = errors.New("xlayer-blacklist: transaction dropped from block (committed blacklist hit)")

// BlacklistGate carries the block-head snapshot and per-block tracer for the
// execution gate. It is nil when the chain is disabled or the list is empty, in
// which case ApplyTransactionWithEVM uses the unmodified upstream path (zero
// hot-path cost). dropNormalHit selects the per-path outcome for a committed
// NORMAL-tx hit (true = build path drops it; false = import path follows seq).
type BlacklistGate struct {
	chainID       uint64
	snap          *Snapshot
	tracer        *BlacklistTracer
	dropNormalHit bool
}

// NewBlacklistGate reads the block-head snapshot for chainID via the mirror
// contract's view ABI and returns a gate, or nil if the chain is disabled or the
// list empty. statedb MUST be the block-head/parent state; header/config are
// needed to build the read-only EVM for the view call (read once before the tx
// loop). dropNormalHit selects the build (true) vs import (false) outcome.
func NewBlacklistGate(statedb vm.StateDB, header *types.Header, config *params.ChainConfig, chainID uint64, dropNormalHit bool) *BlacklistGate {
	if !params.IsBlacklistEnabled(chainID) {
		return nil
	}
	start := time.Now()
	snap := ReadBlacklistSnapshot(statedb, header, config, chainID)
	MetricBlacklistSnapshotRead(time.Since(start).Nanoseconds())
	MetricBlacklistCacheSize(snap.Size())
	if snap.Size() == 0 {
		return nil
	}
	return &BlacklistGate{chainID: chainID, snap: snap, tracer: NewBlacklistTracer(), dropNormalHit: dropNormalHit}
}

// NewBlacklistGateFromSnapshot builds a gate from an already-built snapshot,
// bypassing the on-chain read. It is a test seam: gate/deposit behaviour tests
// inject a list via NewSnapshot without deploying a mirror contract. Returns nil
// for an empty snapshot (matching NewBlacklistGate's "no list = no gate").
func NewBlacklistGateFromSnapshot(chainID uint64, snap *Snapshot, dropNormalHit bool) *BlacklistGate {
	if snap == nil || snap.Size() == 0 {
		return nil
	}
	return &BlacklistGate{chainID: chainID, snap: snap, tracer: NewBlacklistTracer(), dropNormalHit: dropNormalHit}
}

// Hooks returns the tracer hooks to multiplex onto the EVM config.
func (g *BlacklistGate) Hooks() *tracing.Hooks { return g.tracer.Hooks() }

// CombineBlacklistHooks merges the blacklist tracer hooks onto an existing
// (possibly nil) tracer so debug/monitor tracing is preserved (TD R-8). Only the
// hooks the blacklist tracer consumes (OnTxStart, OnBalanceChange) are chained;
// all other base hooks are passed through unchanged.
func CombineBlacklistHooks(base, add *tracing.Hooks) *tracing.Hooks {
	if base == nil {
		return add
	}
	merged := *base // shallow copy preserves all base hooks
	merged.OnTxStart = chainTxStart(base.OnTxStart, add.OnTxStart)
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

// decide runs the committed-effect checks after ApplyMessage and applies the
// per-path interception, all between ApplyMessage and Finalise (so reverted
// effects never reach the state root). It returns:
//   - hit=true: a KEPT hit (deposit included-as-reverted, OR a normal tx on the
//     import path that is... never — import normal hits are waived). In practice
//     hit=true only for deposits here; the caller then rewrites the receipt via
//     overrideReceipt. On hit the pre-execution snapshot (blSnap) is reverted and
//     the canonical failed-deposit post-state is reproduced (keep-mint, nonce
//     N+1, full gasLimit charged).
//   - dropErr=ErrBlacklistDrop: a committed NORMAL-tx hit on the build path; the
//     caller returns before Finalise so the miner's outer snapshot drops the tx.
//   - hit=false, dropErr=nil: no interception (no hit, or an import-path normal
//     hit that followers must not intercept).
func (g *BlacklistGate) decide(msg *Message, tx *types.Transaction, statedb *state.StateDB, gp *GasPool, result *ExecutionResult, nonce uint64, blSnap int, blockNumber *big.Int, blockHash common.Hash, blockTime uint64) (hit bool, dropErr error) {
	matched, category := g.evaluate(msg, tx, statedb, blockNumber, blockHash, blockTime)
	if !matched {
		return false, nil
	}
	// Import path: followers do NOT intercept L2 (non-deposit) txs — L2
	// interception is solely the sequencer's responsibility (it drops blacklisted
	// L2 txs at build time, so an honest block never contains one). Only L1->L2
	// deposits — which bypass the sequencer via forced inclusion — are gated on
	// every path.
	if !tx.IsDepositTx() && !g.dropNormalHit {
		return false, nil
	}
	MetricBlacklistExecRevert(category)
	// Build path: a committed normal-tx hit is dropped from the block. Signal the
	// caller to return before Finalise so its outer snapshot fully undoes
	// state+gas. The miner logs this drop (worker.go), so we do NOT log here.
	if g.dropNormalHit && !tx.IsDepositTx() {
		return false, ErrBlacklistDrop
	}
	// A kept hit (deposit, any path). Log it so a chain-level security
	// interception is traceable from logs alone (R6.5).
	log.Warn("xlayer-blacklist: tx intercepted at exec gate",
		"hash", tx.Hash(), "chainID", g.chainID, "category", category, "deposit", tx.IsDepositTx())
	statedb.RevertToSnapshot(blSnap)
	if tx.IsDepositTx() {
		// B1 fix — reproduce the canonical OP-Stack *failed-deposit* post-state
		// (core/state_transition.go:474-512), NOT a blanket pre-ApplyMessage
		// revert. A naturally-failed deposit (a) KEEPS the deposit-mint and (b)
		// ALWAYS increments the depositor nonce; RevertToSnapshot undid both:
		//   - MINT: re-add (keep-mint, TD §4.4.3). xlayer-reth's existing
		//     failed-deposit path produces the identical post-state, so both
		//     clients agree byte-for-byte (FR-5). Re-mint cannot overflow —
		//     ApplyMessage already applied it once.
		//   - NONCE: always bump to N+1 (mirrors state_transition.go:494).
		//   - GAS: count the full gasLimit into the block (DM-3.6). ChargeUsed
		//     deducts the not-yet-consumed gas from remaining AND adds it to
		//     cumulativeUsed, so header.GasUsed and receipt.CumulativeGasUsed both
		//     reflect the full gasLimit (C-1 fix; a plain SubGas would leave
		//     cumulativeUsed stale and diverge the receipts root).
		if msg.Mint != nil {
			if mintU256, overflow := uint256.FromBig(msg.Mint); !overflow {
				statedb.AddBalance(msg.From, mintU256, tracing.BalanceMint)
			}
		}
		statedb.SetNonce(msg.From, nonce+1, tracing.NonceChangeEoACall)
		if extra := tx.Gas() - result.UsedGas; extra > 0 {
			_ = gp.ChargeUsed(extra)
		}
	}
	return true, nil
}

// overrideReceipt rewrites the receipt of a kept hit (called after MakeReceipt):
// status=0; deposit gasUsed=tx.Gas() (explicit override — must not rely on
// natural Regolith consumed-gas, TD §4.4.3 / PRD I-6); logs recomputed to the
// (now-reverted, empty) committed set + bloom rebuilt.
func (g *BlacklistGate) overrideReceipt(receipt *types.Receipt, tx *types.Transaction, statedb *state.StateDB, blockNumber *big.Int, blockHash common.Hash, blockTime uint64) {
	receipt.Status = types.ReceiptStatusFailed
	if tx.IsDepositTx() {
		receipt.GasUsed = tx.Gas()
	}
	receipt.Logs = statedb.GetLogs(tx.Hash(), blockNumber.Uint64(), blockHash, blockTime)
	receipt.Bloom = types.CreateBloom(receipt)
}

// evaluate runs the committed-effect checks for one tx (check② + check③; check①
// dropped, see tracer file header). Deposits from the exempt-sender set
// (system / L1-attributes) are never intercepted (FR-3 AC2).
func (g *BlacklistGate) evaluate(msg *Message, tx *types.Transaction, statedb *state.StateDB, blockNumber *big.Int, blockHash common.Hash, blockTime uint64) (bool, string) {
	if msg.IsDepositTx && params.IsDepositExemptSender(msg.From) {
		return false, ""
	}
	logs := statedb.GetLogs(tx.Hash(), blockNumber.Uint64(), blockHash, blockTime)
	return g.tracer.Evaluate(g.snap, logs, statedb.GetBalance)
}
