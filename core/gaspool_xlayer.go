// XLayer emergency-freeze blacklist — GasPool extension.
//
// Fork-local XLayer extension (kept in a dedicated _xlayer.go file): a GasPool
// method kept out of the upstream gaspool.go so that file carries no fork
// changes. ChargeUsed is a method on the upstream core.GasPool defined here
// (same package, cross-file).

package core

// ChargeUsed charges `amount` of gas as consumed: it deducts from the remaining
// pool AND adds to the cumulative usage in one step. It is used by the XLayer
// blacklist gate to account the *full gasLimit* of an intercepted
// (included-as-reverted) deposit, mirroring the canonical failed-deposit
// accounting (state_transition.go: ReturnGas(0, GasLimit) after SubGas(GasLimit)
// at buy time). Unlike SubGas, which only moves `remaining` and leaves
// `cumulativeUsed` stale, ChargeUsed keeps both in sync so the receipt's
// CumulativeGasUsed matches the block-level Used().
func (gp *GasPool) ChargeUsed(amount uint64) error {
	if gp.remaining < amount {
		return ErrGasLimitReached
	}
	gp.remaining -= amount
	gp.cumulativeUsed += amount
	return nil
}
