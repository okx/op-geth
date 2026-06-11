package core

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
)

// TestCombineBlacklistHooks_BothCalled: when both base and add provide a hook,
// the merged hook invokes both, base first then add (deterministic order).
func TestCombineBlacklistHooks_BothCalled(t *testing.T) {
	var order []string
	base := &tracing.Hooks{
		OnEnter: func(depth int, typ byte, from, to common.Address, input []byte, gas uint64, value *big.Int) {
			order = append(order, "base")
		},
		OnExit: func(depth int, output []byte, gasUsed uint64, err error, reverted bool) {
			order = append(order, "base-exit")
		},
	}
	add := &tracing.Hooks{
		OnEnter: func(depth int, typ byte, from, to common.Address, input []byte, gas uint64, value *big.Int) {
			order = append(order, "add")
		},
		OnExit: func(depth int, output []byte, gasUsed uint64, err error, reverted bool) {
			order = append(order, "add-exit")
		},
	}

	merged := CombineBlacklistHooks(base, add)
	if merged == nil || merged.OnEnter == nil || merged.OnExit == nil {
		t.Fatal("merged hooks must be non-nil with both callbacks set")
	}
	merged.OnEnter(0, 0, common.Address{}, common.Address{}, nil, 0, nil)
	merged.OnExit(0, nil, 0, nil, false)

	want := []string{"base", "add", "base-exit", "add-exit"}
	if len(order) != len(want) {
		t.Fatalf("call order = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("call order = %v, want %v", order, want)
		}
	}
}

// TestCombineBlacklistHooks_BaseNil: base==nil returns add unchanged (add's hook
// is the only one and is called).
func TestCombineBlacklistHooks_BaseNil(t *testing.T) {
	called := false
	add := &tracing.Hooks{
		OnEnter: func(depth int, typ byte, from, to common.Address, input []byte, gas uint64, value *big.Int) {
			called = true
		},
	}
	merged := CombineBlacklistHooks(nil, add)
	if merged == nil || merged.OnEnter == nil {
		t.Fatal("merged must carry add's hook when base is nil")
	}
	merged.OnEnter(0, 0, common.Address{}, common.Address{}, nil, 0, nil)
	if !called {
		t.Fatal("add hook was not called")
	}
}

// TestCombineBlacklistHooks_AddNil: when add has no callback for a slot, the
// merged hook calls base only and does not panic.
func TestCombineBlacklistHooks_AddNil(t *testing.T) {
	called := false
	base := &tracing.Hooks{
		OnEnter: func(depth int, typ byte, from, to common.Address, input []byte, gas uint64, value *big.Int) {
			called = true
		},
	}
	add := &tracing.Hooks{} // all callbacks nil
	merged := CombineBlacklistHooks(base, add)
	if merged == nil || merged.OnEnter == nil {
		t.Fatal("merged must carry base's hook when add slot is nil")
	}
	merged.OnEnter(0, 0, common.Address{}, common.Address{}, nil, 0, nil) // must not panic
	if !called {
		t.Fatal("base hook was not called")
	}
}

// TestCombineBlacklistHooks_BothNil: base==nil and add==nil returns nil (callers
// guard nil hooks). Documents the actual contract.
func TestCombineBlacklistHooks_BothNil(t *testing.T) {
	if merged := CombineBlacklistHooks(nil, nil); merged != nil {
		t.Fatalf("Combine(nil,nil) = %v, want nil", merged)
	}
}
