// Copyright 2024 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package legacypool

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/txpool"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
)

func mkDynFee(t *testing.T, feeCap, tipCap int64) *types.Transaction {
	t.Helper()
	return types.NewTx(&types.DynamicFeeTx{
		ChainID:   big.NewInt(1),
		Nonce:     0,
		To:        &common.Address{},
		Gas:       21000,
		GasFeeCap: big.NewInt(feeCap),
		GasTipCap: big.NewInt(tipCap),
	})
}

func mkLegacy(t *testing.T, gasPrice int64) *types.Transaction {
	t.Helper()
	return types.NewTx(&types.LegacyTx{
		Nonce:    0,
		To:       &common.Address{},
		Gas:      21000,
		GasPrice: big.NewInt(gasPrice),
	})
}

func TestComputeMockGasPrice_BasicPercentile(t *testing.T) {
	// Effective tips when baseFee=10: min(tipCap, feeCap-10).
	// Resulting effective gas prices = effective tip + baseFee.
	// We pick tips so that effective tip == tipCap (feeCap-10 ≥ tipCap).
	txs := types.Transactions{
		mkDynFee(t, 100, 1),  // effective price = 11
		mkDynFee(t, 100, 3),  // effective price = 13
		mkDynFee(t, 100, 5),  // effective price = 15
		mkDynFee(t, 100, 7),  // effective price = 17
		mkDynFee(t, 100, 9),  // effective price = 19
		mkDynFee(t, 100, 11), // effective price = 21
		mkDynFee(t, 100, 13), // effective price = 23
		mkDynFee(t, 100, 15), // effective price = 25
		mkDynFee(t, 100, 17), // effective price = 27
		mkDynFee(t, 100, 19), // effective price = 29
	}
	baseFee := big.NewInt(10)

	cases := []struct {
		name string
		bps  uint16
		want int64
	}{
		// idx = (10-1) * bps / 10000
		{"p0_smallest", 0, 11},
		{"p10_default", 1000, 11},
		{"p50_median", 5000, 19},
		{"p90_high", 9000, 27},
		{"p100_largest", 10000, 29},
		// out-of-range bps clamped to 100%
		{"clamped_to_100", 20000, 29},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := computeMockGasPrice(txs, baseFee, tc.bps)
			if got == nil {
				t.Fatalf("nil result for bps=%d", tc.bps)
			}
			if got.Int64() != tc.want {
				t.Fatalf("bps=%d: got %d want %d", tc.bps, got.Int64(), tc.want)
			}
		})
	}
}

func TestComputeMockGasPrice_ExcludesZeroFee(t *testing.T) {
	baseFee := big.NewInt(5)
	// 3 zero-fee gasless txs + 2 paying txs (effective price 6 and 100).
	txs := types.Transactions{
		mkDynFee(t, 0, 0),
		mkDynFee(t, 0, 0),
		mkDynFee(t, 0, 0),
		mkDynFee(t, 100, 1),  // effective = 6
		mkDynFee(t, 100, 95), // effective = 100
	}
	got := computeMockGasPrice(txs, baseFee, 5000) // median of [6, 100]
	if got == nil {
		t.Fatal("expected non-nil result")
	}
	// idx = (2-1)*5000/10000 = 0 → first element
	if got.Int64() != 6 {
		t.Fatalf("got %d, want 6 (median of the two non-zero-fee samples)", got.Int64())
	}
}

func TestComputeMockGasPrice_AllZeroFeeReturnsNil(t *testing.T) {
	baseFee := big.NewInt(5)
	txs := types.Transactions{
		mkDynFee(t, 0, 0),
		mkDynFee(t, 0, 0),
	}
	if got := computeMockGasPrice(txs, baseFee, 5000); got != nil {
		t.Fatalf("got %v, want nil for all-gasless block (caller must keep prior mock)", got)
	}
}

func TestComputeMockGasPrice_EmptyBlockReturnsNil(t *testing.T) {
	if got := computeMockGasPrice(types.Transactions{}, big.NewInt(5), 5000); got != nil {
		t.Fatalf("got %v, want nil for empty block", got)
	}
}

func TestComputeMockGasPrice_NilBaseFeeLegacyTxs(t *testing.T) {
	// Pre-London style: baseFee=nil means effective price collapses to gasPrice.
	txs := types.Transactions{
		mkLegacy(t, 7),
		mkLegacy(t, 11),
		mkLegacy(t, 17),
	}
	got := computeMockGasPrice(txs, nil, 5000)
	if got == nil {
		t.Fatal("expected non-nil")
	}
	// idx = (3-1)*5000/10000 = 1 → median 11
	if got.Int64() != 11 {
		t.Fatalf("got %d, want 11", got.Int64())
	}
}

func TestComputeMockGasPrice_FeeCapBelowBaseFeeExcluded(t *testing.T) {
	// txs whose feeCap < baseFee make EffectiveGasTip return ErrGasFeeCapTooLow
	// and are dropped from the sample. Only the tx with feeCap >= baseFee
	// contributes to the percentile.
	baseFee := big.NewInt(10)
	txs := types.Transactions{
		mkDynFee(t, 5, 1),   // feeCap=5 < baseFee → excluded
		mkDynFee(t, 3, 1),   // feeCap=3 < baseFee → excluded
		mkDynFee(t, 100, 5), // feeCap=100 ≥ baseFee → effective tip 5, price 15
	}
	got := computeMockGasPrice(txs, baseFee, 5000)
	if got == nil {
		t.Fatal("expected non-nil result (one qualifying sample remains)")
	}
	if got.Int64() != 15 {
		t.Fatalf("got %d, want 15 (only feeCap>=baseFee tx contributes)", got.Int64())
	}
}

// mkGasless builds a zero-fee dynamic-fee tx with a non-nil `to` (the shape an
// IsGaslessTxFor check requires). The nonce lets callers create distinct hashes.
func mkGasless(t *testing.T, nonce uint64) *types.Transaction {
	t.Helper()
	to := common.HexToAddress("0x00000000000000000000000000000000000ca511")
	return types.NewTx(&types.DynamicFeeTx{
		ChainID:   big.NewInt(1),
		Nonce:     nonce,
		To:        &to,
		Gas:       21000,
		GasFeeCap: big.NewInt(0),
		GasTipCap: big.NewInt(0),
	})
}

// whitelistChecker returns a GaslessChecker that reports allowed=true (with an
// ample gas limit) for exactly the given transaction hashes and false otherwise.
func whitelistChecker(txs ...*types.Transaction) types.GaslessChecker {
	allowed := make(map[common.Hash]struct{}, len(txs))
	for _, tx := range txs {
		allowed[tx.Hash()] = struct{}{}
	}
	return func(tx *types.Transaction) (types.GaslessAllowance, error) {
		if _, ok := allowed[tx.Hash()]; ok {
			return types.GaslessAllowance{Allowed: true, GasLimit: 1_000_000}, nil
		}
		return types.GaslessAllowance{Allowed: false}, nil
	}
}

// newTestPricedList builds a standalone pricedList over a lookup pre-populated
// with the given transactions, mirroring how the pool tracks them
// (lookup.Add + priced.Put). The returned list has baseFee=nil so the heaps sort
// by gasFeeCap, which keeps zero-fee gasless txs at the bottom of the heap.
func newTestPricedList(txs ...*types.Transaction) *pricedList {
	all := newLookup()
	priced := newPricedList(all)
	for _, tx := range txs {
		all.Add(tx)
		priced.Put(tx)
	}
	return priced
}

// inHeaps reports whether the tx is present in either the urgent or floating heap.
func inHeaps(l *pricedList, tx *types.Transaction) bool {
	for _, h := range []*priceHeap{&l.urgent, &l.floating} {
		for _, t := range h.list {
			if t.Hash() == tx.Hash() {
				return true
			}
		}
	}
	return false
}

// gaslessPredeployCode is minimal EVM bytecode for the Gasless predeploy that
// ignores its calldata and returns the ABI tuple (bool allowed=true, uint64
// gasLimit=0xffffffff) — i.e. it whitelists every probed transaction with an
// ample gas limit. Layout returned:
//
//	mem[0:32]  = 0x..01  (canonical ABI true)
//	mem[32:64] = 0x..ffffffff (gas limit, right-aligned)
var gaslessPredeployCode = []byte{
	0x60, 0x01, 0x60, 0x00, 0x52, // PUSH1 1 PUSH1 0 MSTORE
	0x63, 0xff, 0xff, 0xff, 0xff, 0x60, 0x20, 0x52, // PUSH4 0xffffffff PUSH1 0x20 MSTORE
	0x60, 0x40, 0x60, 0x00, 0xf3, // PUSH1 0x40 PUSH1 0x00 RETURN
}

// TestSetGasTip_GaslessNotEvicted verifies: raising the pool's minimum
// gas tip evicts sub-threshold normal txs but spares gasless (zero-tip) txs,
// because the SetGasTip eviction path consults the gasless checker.
func TestSetGasTip_GaslessNotEvicted(t *testing.T) {
	cfg := testTxPoolConfig
	cfg.AllowGasless = true
	pool, _ := setupPoolWithTxPoolConfig(params.TestChainConfig, cfg)
	defer pool.Close()

	// Deploy the whitelist predeploy into the head state so pool.gaslessChecker()
	// classifies the zero-fee txs below as gasless.
	predeploy := types.GaslessAddressFor(params.TestChainConfig.ChainID)
	pool.mu.Lock()
	pool.currentState.SetCode(predeploy, gaslessPredeployCode, tracing.CodeChangeUnspecified)
	pool.mu.Unlock()

	signer := types.LatestSignerForChainID(params.TestChainConfig.ChainID)

	// Two senders: one for a gasless (zero-tip) tx, one for a normal low-tip tx.
	gaslessKey, _ := crypto.GenerateKey()
	gaslessAddr := crypto.PubkeyToAddress(gaslessKey.PublicKey)
	normalKey, _ := crypto.GenerateKey()
	normalAddr := crypto.PubkeyToAddress(normalKey.PublicKey)

	to := common.HexToAddress("0x00000000000000000000000000000000000ca511")
	gaslessTx, _ := types.SignTx(types.NewTx(&types.DynamicFeeTx{
		ChainID:   params.TestChainConfig.ChainID,
		Nonce:     0,
		To:        &to,
		Gas:       21000,
		GasFeeCap: big.NewInt(0),
		GasTipCap: big.NewInt(0),
	}), signer, gaslessKey)
	// Normal tx with a small tip, below the threshold we will set.
	normalTx, _ := types.SignTx(types.NewTx(&types.DynamicFeeTx{
		ChainID:   params.TestChainConfig.ChainID,
		Nonce:     0,
		To:        &to,
		Gas:       21000,
		GasFeeCap: big.NewInt(1_000_000_000),
		GasTipCap: big.NewInt(1),
	}), signer, normalKey)

	// Sanity-check the deployed predeploy actually whitelists the gasless tx.
	if checker := pool.gaslessChecker(); checker == nil || !types.IsGaslessTxFor(gaslessTx, checker) {
		t.Fatal("predeploy did not classify the zero-fee tx as gasless")
	}

	// Place both txs into the pending set via the low-level promote path (as
	// TestDropping does), so we control the pool contents precisely.
	testAddBalance(pool, gaslessAddr, big.NewInt(1_000_000_000_000_000_000))
	testAddBalance(pool, normalAddr, big.NewInt(1_000_000_000_000_000_000))

	pool.mu.Lock()
	// Reserve the senders so the SetGasTip eviction path (removeTx with
	// unreserve=true) can release them without the reserver panicking.
	_ = pool.reserver.Hold(gaslessAddr)
	_ = pool.reserver.Hold(normalAddr)
	pool.all.Add(gaslessTx)
	pool.priced.Put(gaslessTx)
	pool.promoteTx(gaslessAddr, gaslessTx.Hash(), gaslessTx)

	pool.all.Add(normalTx)
	pool.priced.Put(normalTx)
	pool.promoteTx(normalAddr, normalTx.Hash(), normalTx)
	pool.mu.Unlock()

	if pool.Get(gaslessTx.Hash()) == nil || pool.Get(normalTx.Hash()) == nil {
		t.Fatal("setup: both txs should be present before SetGasTip")
	}

	// Raise the min tip above the normal tx's tip (1) but the gasless tx carries a
	// zero tip and must be exempt.
	pool.SetGasTip(big.NewInt(1_000))

	if pool.Get(gaslessTx.Hash()) == nil {
		t.Fatal("gasless tx was evicted by SetGasTip, but must be spared")
	}
	if pool.Get(normalTx.Hash()) != nil {
		t.Fatal("sub-threshold normal tx should have been evicted by SetGasTip")
	}
}

// TestSetGasTip_GaslessEvictedWhenDisabled is the guard for FIX 2(a): when
// AllowGasless is false (checker nil), a zero-tip tx is NOT exempt and is evicted
// by a min-tip raise like any other sub-threshold tx.
func TestSetGasTip_GaslessEvictedWhenDisabled(t *testing.T) {
	pool, _ := setupPoolWithConfig(params.TestChainConfig) // AllowGasless defaults to false
	defer pool.Close()

	signer := types.LatestSignerForChainID(params.TestChainConfig.ChainID)
	key, _ := crypto.GenerateKey()
	addr := crypto.PubkeyToAddress(key.PublicKey)
	to := common.HexToAddress("0x00000000000000000000000000000000000ca511")
	zeroTip, _ := types.SignTx(types.NewTx(&types.DynamicFeeTx{
		ChainID:   params.TestChainConfig.ChainID,
		Nonce:     0,
		To:        &to,
		Gas:       21000,
		GasFeeCap: big.NewInt(0),
		GasTipCap: big.NewInt(0),
	}), signer, key)

	if pool.gaslessChecker() != nil {
		t.Fatal("checker must be nil when AllowGasless is disabled")
	}

	testAddBalance(pool, addr, big.NewInt(1_000_000_000_000_000_000))
	pool.mu.Lock()
	_ = pool.reserver.Hold(addr)
	pool.all.Add(zeroTip)
	pool.priced.Put(zeroTip)
	pool.promoteTx(addr, zeroTip.Hash(), zeroTip)
	pool.mu.Unlock()

	if pool.Get(zeroTip.Hash()) == nil {
		t.Fatal("setup: zero-tip tx should be present before SetGasTip")
	}
	pool.SetGasTip(big.NewInt(1_000))
	if pool.Get(zeroTip.Hash()) != nil {
		t.Fatal("with gasless disabled, a zero-tip tx must be evicted by SetGasTip")
	}
}

// TestPricedList_GaslessNotUnderpriced verifies FIX 2(b): with a gasless checker
// installed, pricedList.Underpriced returns false for a gasless tx even though it
// is the cheapest possible (zero fee), while a normal cheap tx is still flagged
// underpriced. When the checker is nil (gasless disabled) the gasless-looking tx
// IS underpriced — the exemption is fully gated.
func TestPricedList_GaslessNotUnderpriced(t *testing.T) {
	// Populate the list with a couple of paying txs so the heaps are non-empty.
	paying1 := mkDynFee(t, 100, 5)
	paying2 := mkDynFee(t, 100, 9)
	// Two zero-fee gasless txs not yet tracked — the admission candidates.
	gasless := mkGasless(t, 0)
	// A normal cheap tx (zero fee but not whitelisted): must be underpriced.
	cheap := mkGasless(t, 1) // same zero-fee shape, but not in the whitelist

	build := func() *pricedList { return newTestPricedList(paying1, paying2) }

	t.Run("checker_set_gasless_not_underpriced", func(t *testing.T) {
		l := build()
		l.setGaslessChecker(whitelistChecker(gasless))
		l.Reheap() // distribute into urgent/floating like the pool does
		if l.Underpriced(gasless) {
			t.Fatal("gasless tx must not be underpriced when checker allows it")
		}
		if !l.Underpriced(cheap) {
			t.Fatal("normal zero-fee tx must be underpriced for contrast")
		}
	})

	t.Run("nil_checker_gasless_is_underpriced", func(t *testing.T) {
		l := build()
		l.setGaslessChecker(nil) // gasless disabled
		l.Reheap()
		if !l.Underpriced(gasless) {
			t.Fatal("with no checker, a zero-fee tx must be underpriced (exemption gated)")
		}
	})
}

// TestPricedList_GaslessSparedByDiscard verifies FIX 2(c): pricedList.Discard
// never returns gasless txs as victims even though they rank lowest, and the
// gasless txs survive in the heaps afterwards. With a nil checker the same
// zero-fee txs ARE discarded.
func TestPricedList_GaslessSparedByDiscard(t *testing.T) {
	// Gasless (zero-fee) txs rank lowest, so they would be evicted first.
	gasless1 := mkGasless(t, 0)
	gasless2 := mkGasless(t, 1)
	// Paying txs that should be the actual eviction victims.
	paying1 := mkDynFee(t, 100, 3)
	paying2 := mkDynFee(t, 100, 7)

	t.Run("checker_set_gasless_spared", func(t *testing.T) {
		l := newTestPricedList(gasless1, gasless2, paying1, paying2)
		l.setGaslessChecker(whitelistChecker(gasless1, gasless2))
		l.Reheap()

		drop, ok := l.Discard(1)
		if !ok {
			t.Fatal("expected Discard to succeed evicting a paying tx")
		}
		for _, tx := range drop {
			if l.isGasless(tx) {
				t.Fatalf("Discard returned a gasless tx as a victim: %s", tx.Hash())
			}
		}
		// Both gasless txs must remain present in the heaps.
		if !inHeaps(l, gasless1) || !inHeaps(l, gasless2) {
			t.Fatal("gasless txs must survive Discard and remain in the heaps")
		}
	})

	t.Run("nil_checker_gasless_discarded", func(t *testing.T) {
		l := newTestPricedList(gasless1, gasless2, paying1, paying2)
		l.setGaslessChecker(nil) // gasless disabled
		l.Reheap()

		drop, ok := l.Discard(1)
		if !ok {
			t.Fatal("expected Discard to succeed")
		}
		// With no exemption, the cheapest (a zero-fee tx) is a valid victim.
		var droppedGaslessShaped bool
		for _, tx := range drop {
			if tx.Hash() == gasless1.Hash() || tx.Hash() == gasless2.Hash() {
				droppedGaslessShaped = true
			}
		}
		if !droppedGaslessShaped {
			t.Fatal("with no checker, a zero-fee tx must be eligible for eviction (exemption gated)")
		}
	})
}

func TestComputeMockGasPrice_AllBelowBaseFeeReturnsNil(t *testing.T) {
	// Pathological: every tx has feeCap < baseFee, so EffectiveGasTip rejects
	// all of them and the sample is empty. Callers preserve the prior mock.
	baseFee := big.NewInt(100)
	txs := types.Transactions{
		mkDynFee(t, 5, 1),  // feeCap=5 < 100 → excluded
		mkDynFee(t, 50, 0), // feeCap=50 < 100 → excluded
	}
	got := computeMockGasPrice(txs, baseFee, 0)
	if got != nil {
		t.Fatalf("got %v, want nil (no qualifying samples)", got)
	}
}

// fixedCostProvider implements rollupCostFuncProvider for list-level tests: it
// adds a flat rollup (L1/operator) cost to every non-gasless tx and exposes a
// configurable gasless checker.
type fixedCostProvider struct {
	rollupCost *uint256.Int
	checker    types.GaslessChecker
}

func (p fixedCostProvider) RollupCostFunc() txpool.RollupCostFunc {
	return func(types.RollupTransaction) *uint256.Int { return p.rollupCost }
}

func (p fixedCostProvider) GaslessChecker() types.GaslessChecker { return p.checker }

func mkGaslessValue(value uint64) *types.Transaction {
	to := common.HexToAddress("0x00000000000000000000000000000000000ca511")
	return types.NewTx(&types.DynamicFeeTx{
		ChainID:   big.NewInt(1),
		Nonce:     0,
		To:        &to,
		Gas:       21000,
		GasFeeCap: big.NewInt(0),
		GasTipCap: big.NewInt(0),
		Value:     new(big.Int).SetUint64(value),
	})
}

// TestListGaslessExemptFromBalanceFilter verifies that the per-account list's
// balance accounting charges a gasless tx only its value, not the fee-inclusive
// TotalTxCost (value + L1 data fee + operator fee).
func TestListGaslessExemptFromBalanceFilter(t *testing.T) {
	const value = uint64(1_000)
	l1Cost := uint256.NewInt(500) // nonzero rollup/L1 cost charged to non-gasless txs
	balance := uint256.NewInt(value)

	t.Run("gasless_kept_while_underfunded_normal_dropped", func(t *testing.T) {
		gasless := mkGaslessValue(value)
		// A pricey normal tx at the next nonce so costcap exceeds the balance and
		// Filter actually walks the per-tx loop (rather than short-circuiting).
		normal := types.NewTx(&types.DynamicFeeTx{
			ChainID:   big.NewInt(1),
			Nonce:     1,
			To:        &common.Address{},
			Gas:       21000,
			GasFeeCap: big.NewInt(1_000_000_000),
			GasTipCap: big.NewInt(1_000_000_000),
		})
		l := newRollupList(true, fixedCostProvider{rollupCost: l1Cost, checker: whitelistChecker(gasless)})
		if ok, _ := l.Add(gasless, 0); !ok {
			t.Fatal("Add rejected gasless tx")
		}
		// Gasless tx contributes only its value to totalcost, not value+L1Cost.
		if l.totalcost.Cmp(uint256.NewInt(value)) != 0 {
			t.Fatalf("totalcost after gasless Add = %v, want value-only %d", l.totalcost, value)
		}
		if ok, _ := l.Add(normal, 0); !ok {
			t.Fatal("Add rejected normal tx")
		}

		l.Filter(balance, 30_000_000)
		if !l.Contains(0) {
			t.Fatal("gasless tx wrongly evicted by balance filter")
		}
		if l.Contains(1) {
			t.Fatal("underfunded normal tx should have been evicted")
		}
	})

	t.Run("gasless_shaped_tx_evicted_when_not_whitelisted", func(t *testing.T) {
		// Same zero-fee tx, but the checker does not whitelist it (nil checker =>
		// gasless disabled), so it is charged the full value+L1Cost and evicted
		// because the account cannot cover the L1 data fee.
		tx := mkGaslessValue(value)
		l := newRollupList(true, fixedCostProvider{rollupCost: l1Cost, checker: nil})
		if ok, _ := l.Add(tx, 0); !ok {
			t.Fatal("Add rejected tx")
		}
		want := new(uint256.Int).Add(uint256.NewInt(value), l1Cost)
		if l.totalcost.Cmp(want) != 0 {
			t.Fatalf("totalcost = %v, want full cost %v", l.totalcost, want)
		}
		drops, _ := l.Filter(balance, 30_000_000)
		if len(drops) != 1 {
			t.Fatalf("non-gasless underfunded tx should be evicted: got %d drops", len(drops))
		}
	})
}
