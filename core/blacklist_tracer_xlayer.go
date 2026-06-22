// XLayer emergency-freeze blacklist — execution-gate tracer.
// Fork-local XLayer extension (kept in a dedicated _xlayer.go file).
//
// The tracer is an observational core/tracing.Hooks consumer: it cannot abort
// execution, it only records committed effects, and the gate decides revert
// after ApplyMessage returns. It must be multiplexed onto evm.Config.Tracer,
// never replace an existing one.
//
// Detection is two committed-effect checks (priority: Transfer-event > balance):
//   - Transfer-event check: a committed Transfer-class event (scanned from the
//     committed logs) with a blacklisted from/to.
//   - balance check: a committed native-ETH balance movement (fees stripped).
//
// An earlier committed-CALL-touch check was removed for cross-client alignment:
// a real asset-moving attack always trips the Transfer-event or balance check,
// and normal-L2 interception is sequencer-only (off the consensus path), so all
// clients judge every tx on these two checks alone.

package core

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
)

// Hit-category labels for the exec_revert_total metric.
const (
	HookLog          = "log"          // committed Transfer-class event
	HookSelfdestruct = "selfdestruct" // balance check via a selfdestruct reason
	HookEthBalance   = "eth_balance"  // balance check via native ETH balance diff
)

// Transfer-class event topic0 signatures.
var (
	topicERC20Transfer    = crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)"))
	topicERC1155Single    = crypto.Keccak256Hash([]byte("TransferSingle(address,address,address,uint256,uint256)"))
	topicERC1155Batch     = crypto.Keccak256Hash([]byte("TransferBatch(address,address,address,uint256[],uint256[])"))
	transferFeeReasonsSet = map[tracing.BalanceChangeReason]struct{}{
		tracing.BalanceIncreaseRewardTransactionFee: {}, // 5
		tracing.BalanceDecreaseGasBuy:               {}, // 6
		tracing.BalanceIncreaseGasReturn:            {}, // 7
	}
)

// BlacklistTracer accumulates committed-effect evidence for a single tx and
// decides, after execution, whether a blacklisted address was hit by a committed
// Transfer-class event or a committed native-ETH balance movement with fees
// stripped.
//
// Balance-check detail: balStart[a] is captured from the first
// OnBalanceChange.prev of a; balEnd is read from committed state after
// ApplyMessage; feeDelta[a] strips reasons {5,6,7}. A hit is
// (balEnd-balStart)-feeDelta != 0. Transient transfers inside reverted frames
// are already cancelled by the EVM's own journaling, so they never enter the
// committed diff. The candidate set is exactly the addresses observed in
// OnBalanceChange — provably complete, including selfdestruct beneficiaries.
type BlacklistTracer struct {
	balStart        map[common.Address]*big.Int
	feeDelta        map[common.Address]*big.Int
	sawSelfdestruct map[common.Address]bool
}

// NewBlacklistTracer returns a tracer with empty per-tx state.
func NewBlacklistTracer() *BlacklistTracer {
	t := &BlacklistTracer{}
	t.reset()
	return t
}

func (t *BlacklistTracer) reset() {
	t.balStart = make(map[common.Address]*big.Int)
	t.feeDelta = make(map[common.Address]*big.Int)
	t.sawSelfdestruct = make(map[common.Address]bool)
}

// Hooks returns the tracing.Hooks to multiplex onto evm.Config.Tracer. Only the
// hooks the balance check needs are consumed: OnTxStart (per-tx reset) and
// OnBalanceChange.
func (t *BlacklistTracer) Hooks() *tracing.Hooks {
	return &tracing.Hooks{
		OnTxStart:       t.onTxStart,
		OnBalanceChange: t.onBalanceChange,
	}
}

func (t *BlacklistTracer) onTxStart(*tracing.VMContext, *types.Transaction, common.Address) {
	t.reset()
}

func (t *BlacklistTracer) onBalanceChange(addr common.Address, prev, newBal *big.Int, reason tracing.BalanceChangeReason) {
	if _, seen := t.balStart[addr]; !seen {
		t.balStart[addr] = new(big.Int).Set(prev)
	}
	if _, ok := transferFeeReasonsSet[reason]; ok {
		delta := new(big.Int).Sub(newBal, prev)
		if cur, ok := t.feeDelta[addr]; ok {
			cur.Add(cur, delta)
		} else {
			t.feeDelta[addr] = delta
		}
	}
	switch reason {
	case tracing.BalanceIncreaseSelfdestruct, tracing.BalanceDecreaseSelfdestruct, tracing.BalanceDecreaseSelfdestructBurn:
		t.sawSelfdestruct[addr] = true
	}
}

// scanTransferLogs reports whether any committed Transfer-class log has a
// blacklisted from/to. receipt.Logs already excludes reverted-frame logs, so no
// frame analysis is needed here.
func scanTransferLogs(snap *Snapshot, logs []*types.Log) bool {
	for _, lg := range logs {
		if len(lg.Topics) == 0 {
			continue
		}
		switch lg.Topics[0] {
		case topicERC20Transfer:
			// Transfer(from indexed, to indexed, value). Require both indexed args.
			if len(lg.Topics) >= 3 {
				if snap.Contains(common.BytesToAddress(lg.Topics[1].Bytes())) ||
					snap.Contains(common.BytesToAddress(lg.Topics[2].Bytes())) {
					return true
				}
			}
		case topicERC1155Single, topicERC1155Batch:
			// Transfer{Single,Batch}(operator indexed, from indexed, to indexed, ...).
			if len(lg.Topics) >= 4 {
				if snap.Contains(common.BytesToAddress(lg.Topics[2].Bytes())) ||
					snap.Contains(common.BytesToAddress(lg.Topics[3].Bytes())) {
					return true
				}
			}
		}
	}
	return false
}

// balanceHit reports whether any candidate address (observed via
// OnBalanceChange) that is on the blacklist had a committed native-ETH balance
// movement once fees (reasons {5,6,7}) are stripped. It returns the metric
// category (selfdestruct vs eth_balance) for the matched address.
func (t *BlacklistTracer) balanceHit(snap *Snapshot, balanceOf func(common.Address) *uint256.Int) (bool, string) {
	for addr, start := range t.balStart {
		if !snap.Contains(addr) {
			continue
		}
		end := balanceOf(addr).ToBig()
		net := new(big.Int).Sub(end, start)
		if fee, ok := t.feeDelta[addr]; ok {
			net.Sub(net, fee)
		}
		if net.Sign() != 0 {
			if t.sawSelfdestruct[addr] {
				return true, HookSelfdestruct
			}
			return true, HookEthBalance
		}
	}
	return false, ""
}

// Evaluate runs the two committed-effect checks against the block-head snapshot.
// It returns whether the tx hit the blacklist and the metric category of the
// first matched check (priority: log > balance). balanceOf must read the
// committed (post-ApplyMessage, pre-revert) state. Used identically for deposit
// and normal L2 txs.
func (t *BlacklistTracer) Evaluate(snap *Snapshot, logs []*types.Log, balanceOf func(common.Address) *uint256.Int) (bool, string) {
	if snap == nil || snap.Size() == 0 {
		return false, ""
	}
	if scanTransferLogs(snap, logs) {
		return true, HookLog
	}
	if hit, category := t.balanceHit(snap, balanceOf); hit {
		return true, category
	}
	return false, ""
}
