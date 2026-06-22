package core

import "testing"

// TestGasPool_ChargeUsed verifies the X Layer blacklist gas-accounting
// primitive: ChargeUsed must move `remaining` DOWN and `cumulativeUsed` UP by
// the same amount in one step, so Used() and CumulativeUsed() stay in sync —
// unlike SubGas, which leaves cumulativeUsed stale.
func TestGasPool_ChargeUsed(t *testing.T) {
	gp := NewGasPool(1000)
	// Simulate a natural execution that consumed 200 (remaining 800, cumulative 200),
	// mirroring ReturnGas(0, 200).
	if err := gp.ReturnGas(0, 200); err != nil {
		t.Fatalf("ReturnGas: %v", err)
	}
	// SubGas would only touch remaining; here we want remaining=800 first.
	if err := gp.SubGas(200); err != nil { // remaining: 1000-200=800
		t.Fatalf("SubGas: %v", err)
	}
	if gp.Gas() != 800 || gp.CumulativeUsed() != 200 || gp.Used() != 200 {
		t.Fatalf("precondition: remaining=%d cumulative=%d used=%d, want 800/200/200", gp.Gas(), gp.CumulativeUsed(), gp.Used())
	}

	// Charge an extra 300 (e.g. the gasLimit-minus-natural for an intercepted deposit).
	if err := gp.ChargeUsed(300); err != nil {
		t.Fatalf("ChargeUsed: %v", err)
	}
	if gp.Gas() != 500 {
		t.Fatalf("remaining = %d, want 500 (800-300)", gp.Gas())
	}
	if gp.CumulativeUsed() != 500 {
		t.Fatalf("cumulativeUsed = %d, want 500 (200+300)", gp.CumulativeUsed())
	}
	if gp.Used() != 500 {
		t.Fatalf("Used() = %d, want 500 (must equal CumulativeUsed contribution)", gp.Used())
	}
	// The invariant that matters for receipts root: Used() == CumulativeUsed()
	// when the whole pool is attributed to consumed gas.
	if gp.Used() != gp.CumulativeUsed() {
		t.Fatalf("Used()=%d != CumulativeUsed()=%d (gas-accounting divergence)", gp.Used(), gp.CumulativeUsed())
	}
}

// TestGasPool_ChargeUsed_OverLimit: charging more than remaining returns
// ErrGasLimitReached and mutates nothing.
func TestGasPool_ChargeUsed_OverLimit(t *testing.T) {
	gp := NewGasPool(100)
	if err := gp.ChargeUsed(101); err != ErrGasLimitReached {
		t.Fatalf("err = %v, want ErrGasLimitReached", err)
	}
	if gp.Gas() != 100 || gp.CumulativeUsed() != 0 {
		t.Fatalf("state mutated on failed charge: remaining=%d cumulative=%d, want 100/0", gp.Gas(), gp.CumulativeUsed())
	}
}
