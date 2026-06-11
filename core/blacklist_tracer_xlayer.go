// XLayer emergency-freeze blacklist — execution-gate tracer (XLOP-1099, FR-2).
//
// Fork-local XLayer extension (KG naming rule). The tracer is an observational
// core/tracing.Hooks consumer: it cannot abort execution, it only records
// committed effects, and the gate decides revert AFTER ApplyMessage returns.
// It must be multiplexed onto evm.Config.Tracer, never replace an existing one.

package core

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
)

// Hit-category labels for the exec_revert_total metric (TD §4.5 / DM-7.4..7.7).
const (
	HookCall         = "call"         // check① committed CALL touch
	HookLog          = "log"          // check② committed Transfer-class event
	HookSelfdestruct = "selfdestruct" // check③ via selfdestruct balance reason
	HookEthBalance   = "eth_balance"  // check③ via native ETH balance diff
)

// Transfer-class event topic0 signatures (FR-2 check②, A-04 §4.4).
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

// frame is one node in the per-tx call-frame tree (check①).
type frame struct {
	parent   int              // index of parent frame, -1 for root
	reverted bool             // this frame (or its sub-call) reverted
	touched  []common.Address // addresses touched as caller/callee of this frame
}

// BlacklistTracer accumulates committed-effect evidence for a single tx and
// decides, after execution, whether a blacklisted address was touched in a
// committed (non-reverted) execution sub-tree, a committed Transfer-class event,
// or a committed native-ETH balance movement (fees stripped).
//
// Detection algorithms (TD §4.4.2):
//   - check①: a frame tree with ancestor-revert propagation. A touch is
//     committed iff its frame and ALL ancestors did not revert.
//   - check③: final committed balance diff. balStart[a] is captured from the
//     first OnBalanceChange.prev of a; balEnd is read from committed state after
//     ApplyMessage; feeDelta[a] strips reasons {5,6,7}. A hit is
//     (balEnd-balStart)-feeDelta != 0. Transient transfers inside reverted
//     frames are already cancelled by the EVM's own journaling, so they never
//     enter the committed diff (DM-2.18). The candidate set is exactly the
//     addresses observed in OnBalanceChange — provably complete, including
//     selfdestruct beneficiaries (A-15 Minor-1).
type BlacklistTracer struct {
	frames []frame
	stack  []int // indices of currently-open frames

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
	t.frames = t.frames[:0]
	t.stack = t.stack[:0]
	t.balStart = make(map[common.Address]*big.Int)
	t.feeDelta = make(map[common.Address]*big.Int)
	t.sawSelfdestruct = make(map[common.Address]bool)
}

// Hooks returns the tracing.Hooks to multiplex onto evm.Config.Tracer.
func (t *BlacklistTracer) Hooks() *tracing.Hooks {
	return &tracing.Hooks{
		OnTxStart:       t.onTxStart,
		OnEnter:         t.onEnter,
		OnExit:          t.onExit,
		OnBalanceChange: t.onBalanceChange,
	}
}

func (t *BlacklistTracer) onTxStart(*tracing.VMContext, *types.Transaction, common.Address) {
	t.reset()
}

func (t *BlacklistTracer) onEnter(depth int, typ byte, from common.Address, to common.Address, input []byte, gas uint64, value *big.Int) {
	parent := -1
	if len(t.stack) > 0 {
		parent = t.stack[len(t.stack)-1]
	}
	idx := len(t.frames)
	t.frames = append(t.frames, frame{parent: parent, touched: []common.Address{from, to}})
	t.stack = append(t.stack, idx)
}

func (t *BlacklistTracer) onExit(depth int, output []byte, gasUsed uint64, err error, reverted bool) {
	if len(t.stack) == 0 {
		return
	}
	idx := t.stack[len(t.stack)-1]
	t.stack = t.stack[:len(t.stack)-1]
	if reverted {
		t.frames[idx].reverted = true
	}
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

// committedTouch reports whether addr is touched by any committed frame, i.e. a
// frame whose entire ancestor chain (including itself) did not revert (check①).
func (t *BlacklistTracer) committedTouch(snap *Snapshot) (bool, common.Address) {
	for i := range t.frames {
		if t.frameReverted(i) {
			continue
		}
		for _, a := range t.frames[i].touched {
			if snap.Contains(a) {
				return true, a
			}
		}
	}
	return false, common.Address{}
}

// frameReverted reports whether frame i or any of its ancestors reverted.
func (t *BlacklistTracer) frameReverted(i int) bool {
	for i != -1 {
		if t.frames[i].reverted {
			return true
		}
		i = t.frames[i].parent
	}
	return false
}

// scanTransferLogs reports whether any committed Transfer-class log has a
// blacklisted from/to (check②). receipt.Logs already excludes reverted-frame
// logs, so no frame analysis is needed here (DM-2.10).
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
// movement once fees (reasons {5,6,7}) are stripped (check③). It returns the
// metric category (selfdestruct vs eth_balance) for the matched address.
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

// Evaluate runs all three committed-effect checks against the block-head
// snapshot. It returns whether the tx hit the blacklist and the metric category
// of the first matched check (priority: call > log > balance). balanceOf must
// read the committed (post-ApplyMessage, pre-revert) state.
func (t *BlacklistTracer) Evaluate(snap *Snapshot, logs []*types.Log, balanceOf func(common.Address) *uint256.Int) (bool, string) {
	if snap == nil || snap.Size() == 0 {
		return false, ""
	}
	if hit, _ := t.committedTouch(snap); hit {
		return true, HookCall
	}
	if scanTransferLogs(snap, logs) {
		return true, HookLog
	}
	if hit, category := t.balanceHit(snap, balanceOf); hit {
		return true, category
	}
	return false, ""
}
