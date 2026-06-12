package core

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"
)

var (
	addrAAA = common.HexToAddress("0x00000000000000000000000000000000000000AA")
	addrBBB = common.HexToAddress("0x00000000000000000000000000000000000000BB")
	addrCCC = common.HexToAddress("0x00000000000000000000000000000000000000CC")
)

func snapOf(addrs ...common.Address) *Snapshot { return NewSnapshot(addrs) }

// balances returns a balanceOf closure (committed final balances) for check③.
func balances(m map[common.Address]int64) func(common.Address) *uint256.Int {
	return func(a common.Address) *uint256.Int {
		return uint256.NewInt(uint64(m[a]))
	}
}

func zeroBalance(common.Address) *uint256.Int { return uint256.NewInt(0) }

// --- check① committed CALL touch (frame tree) ---

func TestEvaluate_CommittedCallTouch(t *testing.T) {
	// DM-2.8: EOA → Proxy → 0xAAA committed inner CALL → hit, category call.
	tr := NewBlacklistTracer()
	h := tr.Hooks()
	h.OnTxStart(nil, nil, common.Address{})
	h.OnEnter(0, 0, addrBBB, addrCCC, nil, 0, nil) // root: EOA→Proxy
	h.OnEnter(1, 0, addrCCC, addrAAA, nil, 0, nil) // inner: Proxy→0xAAA
	h.OnExit(1, nil, 0, nil, false)                // inner commits
	h.OnExit(0, nil, 0, nil, false)                // root commits

	hit, cat := tr.Evaluate(snapOf(addrAAA), nil, zeroBalance)
	if !hit || cat != HookCall {
		t.Fatalf("got hit=%v cat=%q, want true/call", hit, cat)
	}
}

func TestEvaluate_RevertedSubcallTouch(t *testing.T) {
	// DM-2.9: inner subcall touches 0xAAA but reverts; outer succeeds → NOT hit.
	tr := NewBlacklistTracer()
	h := tr.Hooks()
	h.OnTxStart(nil, nil, common.Address{})
	h.OnEnter(0, 0, addrBBB, addrCCC, nil, 0, nil)
	h.OnEnter(1, 0, addrCCC, addrAAA, nil, 0, nil)
	h.OnExit(1, nil, 0, nil, true) // inner reverts → touch voided
	h.OnExit(0, nil, 0, nil, false)

	if hit, _ := tr.Evaluate(snapOf(addrAAA), nil, zeroBalance); hit {
		t.Fatalf("reverted subcall touch must not hit")
	}
}

func TestEvaluate_AncestorRevertPropagation(t *testing.T) {
	// A child commits but a LATER-reverting ancestor voids the touch.
	tr := NewBlacklistTracer()
	h := tr.Hooks()
	h.OnTxStart(nil, nil, common.Address{})
	h.OnEnter(0, 0, addrBBB, addrCCC, nil, 0, nil) // root
	h.OnEnter(1, 0, addrCCC, addrCCC, nil, 0, nil) // mid
	h.OnEnter(2, 0, addrCCC, addrAAA, nil, 0, nil) // deep: touches 0xAAA
	h.OnExit(2, nil, 0, nil, false)                // deep commits
	h.OnExit(1, nil, 0, nil, true)                 // mid (ancestor) reverts
	h.OnExit(0, nil, 0, nil, false)

	if hit, _ := tr.Evaluate(snapOf(addrAAA), nil, zeroBalance); hit {
		t.Fatalf("touch under a reverted ancestor must be voided")
	}
}

// --- check② committed Transfer-class events ---

func erc20Transfer(from, to common.Address) *types.Log {
	return &types.Log{Topics: []common.Hash{
		topicERC20Transfer,
		common.BytesToHash(from.Bytes()),
		common.BytesToHash(to.Bytes()),
	}}
}

func erc1155Single(operator, from, to common.Address) *types.Log {
	return &types.Log{Topics: []common.Hash{
		topicERC1155Single,
		common.BytesToHash(operator.Bytes()),
		common.BytesToHash(from.Bytes()),
		common.BytesToHash(to.Bytes()),
	}}
}

func TestEvaluate_TransferEvents(t *testing.T) {
	cases := []struct {
		name string
		logs []*types.Log
		snap *Snapshot
		want bool
		cat  string
	}{
		{"ERC20 from-hit (DM-2.1)", []*types.Log{erc20Transfer(addrAAA, addrBBB)}, snapOf(addrAAA), true, HookLog},
		{"ERC20 to-hit", []*types.Log{erc20Transfer(addrBBB, addrAAA)}, snapOf(addrAAA), true, HookLog},
		{"ERC20 miss (DM-2.5)", []*types.Log{erc20Transfer(addrBBB, addrCCC)}, snapOf(addrAAA), false, ""},
		{"ERC1155 single hit (DM-2.3)", []*types.Log{erc1155Single(addrCCC, addrAAA, addrBBB)}, snapOf(addrAAA), true, HookLog},
		{"non-Transfer topic (DM-2.7)", []*types.Log{{Topics: []common.Hash{common.HexToHash("0xdeadbeef"), common.BytesToHash(addrAAA.Bytes())}}}, snapOf(addrAAA), false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tr := NewBlacklistTracer()
			tr.Hooks().OnTxStart(nil, nil, common.Address{})
			hit, cat := tr.Evaluate(tc.snap, tc.logs, zeroBalance)
			if hit != tc.want {
				t.Fatalf("hit=%v, want %v", hit, tc.want)
			}
			if hit && cat != tc.cat {
				t.Fatalf("cat=%q, want %q", cat, tc.cat)
			}
		})
	}
}

// --- check③ committed native-ETH balance diff (fee-stripped) ---

func bc(h *tracing.Hooks, addr common.Address, prev, newBal int64, reason tracing.BalanceChangeReason) {
	h.OnBalanceChange(addr, big.NewInt(prev), big.NewInt(newBal), reason)
}

func TestEvaluate_BalanceChecks(t *testing.T) {
	t.Run("value transfer to 0xAAA hits eth_balance (DM-2.11)", func(t *testing.T) {
		tr := NewBlacklistTracer()
		h := tr.Hooks()
		h.OnTxStart(nil, nil, common.Address{})
		bc(h, addrAAA, 0, 100, tracing.BalanceChangeTransfer)
		hit, cat := tr.Evaluate(snapOf(addrAAA), nil, balances(map[common.Address]int64{addrAAA: 100}))
		if !hit || cat != HookEthBalance {
			t.Fatalf("got hit=%v cat=%q, want true/eth_balance", hit, cat)
		}
	})

	t.Run("gas-only payer not intercepted (DM-2.15/2.16)", func(t *testing.T) {
		tr := NewBlacklistTracer()
		h := tr.Hooks()
		h.OnTxStart(nil, nil, common.Address{})
		bc(h, addrAAA, 1000, 900, tracing.BalanceDecreaseGasBuy)   // -100
		bc(h, addrAAA, 900, 910, tracing.BalanceIncreaseGasReturn) // +10
		if hit, _ := tr.Evaluate(snapOf(addrAAA), nil, balances(map[common.Address]int64{addrAAA: 910})); hit {
			t.Fatalf("gas-only balance change must not hit")
		}
	})

	t.Run("coinbase/fee recipient not intercepted (DM-2.17)", func(t *testing.T) {
		tr := NewBlacklistTracer()
		h := tr.Hooks()
		h.OnTxStart(nil, nil, common.Address{})
		bc(h, addrAAA, 0, 50, tracing.BalanceIncreaseRewardTransactionFee) // +50 fee
		if hit, _ := tr.Evaluate(snapOf(addrAAA), nil, balances(map[common.Address]int64{addrAAA: 50})); hit {
			t.Fatalf("fee reward must not hit")
		}
	})

	t.Run("selfdestruct beneficiary hits selfdestruct (DM-2.12)", func(t *testing.T) {
		tr := NewBlacklistTracer()
		h := tr.Hooks()
		h.OnTxStart(nil, nil, common.Address{})
		bc(h, addrAAA, 0, 100, tracing.BalanceIncreaseSelfdestruct)
		hit, cat := tr.Evaluate(snapOf(addrAAA), nil, balances(map[common.Address]int64{addrAAA: 100}))
		if !hit || cat != HookSelfdestruct {
			t.Fatalf("got hit=%v cat=%q, want true/selfdestruct", hit, cat)
		}
	})

	t.Run("transient transfer in reverted frame not intercepted (DM-2.18)", func(t *testing.T) {
		tr := NewBlacklistTracer()
		h := tr.Hooks()
		h.OnTxStart(nil, nil, common.Address{})
		// transient transfer observed, but EVM journaling restores committed balance to 0
		bc(h, addrAAA, 0, 100, tracing.BalanceChangeTransfer)
		if hit, _ := tr.Evaluate(snapOf(addrAAA), nil, balances(map[common.Address]int64{addrAAA: 0})); hit {
			t.Fatalf("transient (reverted) transfer must not hit")
		}
	})
}

// --- short-circuit when disabled/empty (DM-2.20) ---

func TestEvaluate_EmptySnapshotNoOp(t *testing.T) {
	tr := NewBlacklistTracer()
	h := tr.Hooks()
	h.OnTxStart(nil, nil, common.Address{})
	h.OnEnter(0, 0, addrAAA, addrAAA, nil, 0, nil)
	h.OnExit(0, nil, 0, nil, false)
	if hit, _ := tr.Evaluate(NewSnapshot(nil), []*types.Log{erc20Transfer(addrAAA, addrBBB)}, zeroBalance); hit {
		t.Fatalf("empty snapshot must short-circuit to no-op")
	}
	if hit, _ := tr.Evaluate(nil, nil, zeroBalance); hit {
		t.Fatalf("nil snapshot must short-circuit to no-op")
	}
}

// --- deposit variant: skips check① (decision B, XLOP-1100) ---

func TestEvaluateDeposit_SkipsCallTouch(t *testing.T) {
	// A committed CALL touch of 0xAAA with no event and no ETH movement: the
	// deposit path (EvaluateDeposit) must NOT hit (check① skipped); the same trace
	// via Evaluate (L2) still hits via check①, proving only deposits skip it.
	tr := NewBlacklistTracer()
	h := tr.Hooks()
	h.OnTxStart(nil, nil, common.Address{})
	h.OnEnter(0, 0, addrBBB, addrCCC, nil, 0, nil) // root
	h.OnEnter(1, 0, addrCCC, addrAAA, nil, 0, nil) // inner committed touch of 0xAAA
	h.OnExit(1, nil, 0, nil, false)
	h.OnExit(0, nil, 0, nil, false)

	if hit, _ := tr.EvaluateDeposit(snapOf(addrAAA), nil, zeroBalance); hit {
		t.Fatal("deposit must skip check① (pure CALL touch must not hit)")
	}
	if hit, cat := tr.Evaluate(snapOf(addrAAA), nil, zeroBalance); !hit || cat != HookCall {
		t.Fatalf("L2 Evaluate must still hit via check①, got hit=%v cat=%q", hit, cat)
	}
}

func TestEvaluateDeposit_StillHitsLogAndBalance(t *testing.T) {
	t.Run("event hit", func(t *testing.T) {
		tr := NewBlacklistTracer()
		tr.Hooks().OnTxStart(nil, nil, common.Address{})
		hit, cat := tr.EvaluateDeposit(snapOf(addrAAA), []*types.Log{erc20Transfer(addrBBB, addrAAA)}, zeroBalance)
		if !hit || cat != HookLog {
			t.Fatalf("deposit event hit: got hit=%v cat=%q, want true/log", hit, cat)
		}
	})
	t.Run("balance hit", func(t *testing.T) {
		tr := NewBlacklistTracer()
		h := tr.Hooks()
		h.OnTxStart(nil, nil, common.Address{})
		bc(h, addrAAA, 0, 100, tracing.BalanceChangeTransfer)
		hit, cat := tr.EvaluateDeposit(snapOf(addrAAA), nil, balances(map[common.Address]int64{addrAAA: 100}))
		if !hit || cat != HookEthBalance {
			t.Fatalf("deposit balance hit: got hit=%v cat=%q, want true/eth_balance", hit, cat)
		}
	})
}
