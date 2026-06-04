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
	"github.com/ethereum/go-ethereum/core/types"
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
		mkDynFee(t, 100, 1),   // effective = 6
		mkDynFee(t, 100, 95),  // effective = 100
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
