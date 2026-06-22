package core

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
)

// CombineBlacklistHooks chains only the hooks the blacklist tracer consumes —
// OnTxStart and OnBalanceChange (the balance check); all other base hooks pass
// through unchanged (the CALL-touch check was removed, so OnEnter/OnExit are no
// longer chained).

// TestCombineBlacklistHooks_BothCalled: when both base and add provide a chained
// hook (OnBalanceChange), the merged hook invokes both, base first then add.
func TestCombineBlacklistHooks_BothCalled(t *testing.T) {
	var order []string
	base := &tracing.Hooks{
		OnBalanceChange: func(addr common.Address, prev, newBal *big.Int, reason tracing.BalanceChangeReason) {
			order = append(order, "base")
		},
	}
	add := &tracing.Hooks{
		OnBalanceChange: func(addr common.Address, prev, newBal *big.Int, reason tracing.BalanceChangeReason) {
			order = append(order, "add")
		},
	}

	merged := CombineBlacklistHooks(base, add)
	if merged == nil || merged.OnBalanceChange == nil {
		t.Fatal("merged hooks must be non-nil with OnBalanceChange set")
	}
	merged.OnBalanceChange(common.Address{}, nil, nil, 0)

	want := []string{"base", "add"}
	if len(order) != len(want) || order[0] != want[0] || order[1] != want[1] {
		t.Fatalf("call order = %v, want %v", order, want)
	}
}

// TestCombineBlacklistHooks_BaseNil: base==nil returns add unchanged.
func TestCombineBlacklistHooks_BaseNil(t *testing.T) {
	called := false
	add := &tracing.Hooks{
		OnBalanceChange: func(addr common.Address, prev, newBal *big.Int, reason tracing.BalanceChangeReason) {
			called = true
		},
	}
	merged := CombineBlacklistHooks(nil, add)
	if merged == nil || merged.OnBalanceChange == nil {
		t.Fatal("merged must carry add's hook when base is nil")
	}
	merged.OnBalanceChange(common.Address{}, nil, nil, 0)
	if !called {
		t.Fatal("add hook was not called")
	}
}

// TestCombineBlacklistHooks_AddNil: when add has no callback for a slot, the
// merged hook calls base only and does not panic.
func TestCombineBlacklistHooks_AddNil(t *testing.T) {
	called := false
	base := &tracing.Hooks{
		OnBalanceChange: func(addr common.Address, prev, newBal *big.Int, reason tracing.BalanceChangeReason) {
			called = true
		},
	}
	add := &tracing.Hooks{} // all callbacks nil
	merged := CombineBlacklistHooks(base, add)
	if merged == nil || merged.OnBalanceChange == nil {
		t.Fatal("merged must carry base's hook when add slot is nil")
	}
	merged.OnBalanceChange(common.Address{}, nil, nil, 0) // must not panic
	if !called {
		t.Fatal("base hook was not called")
	}
}

// TestCombineBlacklistHooks_PassThrough: a base hook the blacklist tracer does
// NOT consume (e.g. OnEnter) must survive on the merged hooks unchanged, so a
// debug/monitor tracer keeps working.
func TestCombineBlacklistHooks_PassThrough(t *testing.T) {
	called := false
	base := &tracing.Hooks{
		OnEnter: func(depth int, typ byte, from, to common.Address, input []byte, gas uint64, value *big.Int) {
			called = true
		},
	}
	add := &tracing.Hooks{
		OnBalanceChange: func(addr common.Address, prev, newBal *big.Int, reason tracing.BalanceChangeReason) {},
	}
	merged := CombineBlacklistHooks(base, add)
	if merged == nil || merged.OnEnter == nil {
		t.Fatal("non-consumed base hook (OnEnter) must pass through")
	}
	merged.OnEnter(0, 0, common.Address{}, common.Address{}, nil, 0, nil)
	if !called {
		t.Fatal("passed-through base OnEnter was not called")
	}
}

// TestCombineBlacklistHooks_BothNil: base==nil and add==nil returns nil (callers
// guard nil hooks). Documents the actual contract.
func TestCombineBlacklistHooks_BothNil(t *testing.T) {
	if merged := CombineBlacklistHooks(nil, nil); merged != nil {
		t.Fatalf("Combine(nil,nil) = %v, want nil", merged)
	}
}
