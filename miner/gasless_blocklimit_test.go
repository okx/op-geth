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

package miner

import (
	"math"
	"testing"
)

// TestGaslessGasFits exercises the per-block gasless gas budget decision used by
// commitTransactions to admit or skip gasless transactions.
func TestGaslessGasFits(t *testing.T) {
	tests := []struct {
		name  string
		used  uint64
		txGas uint64
		limit uint64
		want  bool
	}{
		{
			name:  "zero limit disables the check",
			used:  500_000_000,
			txGas: 100_000_000,
			limit: 0,
			want:  true,
		},
		{
			name:  "fits well within budget",
			used:  10_000_000,
			txGas: 21_000,
			limit: 100_000_000,
			want:  true,
		},
		{
			name:  "fits exactly at the budget boundary",
			used:  79_000_000,
			txGas: 21_000_000,
			limit: 100_000_000,
			want:  true,
		},
		{
			name:  "exceeds budget by one gas",
			used:  80_000_001,
			txGas: 20_000_000,
			limit: 100_000_000,
			want:  false,
		},
		{
			name:  "single tx larger than the whole budget",
			used:  0,
			txGas: 100_000_001,
			limit: 100_000_000,
			want:  false,
		},
		{
			name:  "single tx exactly equal to the budget on an empty block",
			used:  0,
			txGas: 100_000_000,
			limit: 100_000_000,
			want:  true,
		},
		{
			name:  "budget already exhausted",
			used:  100_000_000,
			txGas: 1,
			limit: 100_000_000,
			want:  false,
		},
		{
			name:  "no overflow when used+txGas would wrap uint64",
			used:  math.MaxUint64 - 10,
			txGas: 100,
			limit: 100_000_000,
			want:  false,
		},
		{
			name:  "zero-gas tx always fits a non-zero budget",
			used:  100_000_000,
			txGas: 0,
			limit: 100_000_000,
			want:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := gaslessGasFits(tt.used, tt.txGas, tt.limit); got != tt.want {
				t.Fatalf("gaslessGasFits(%d, %d, %d) = %v, want %v",
					tt.used, tt.txGas, tt.limit, got, tt.want)
			}
		})
	}
}

// TestGaslessBudgetSequentialAdmission mirrors the admit/accumulate loop in
// commitTransactions: each candidate gasless tx is reserved against its gas
// limit before inclusion, and the running total is advanced by the actual gas
// used once admitted. It verifies the budget caps the included set while still
// accounting actual (not reserved) usage.
func TestGaslessBudgetSequentialAdmission(t *testing.T) {
	const limit = 100_000_000

	type candidate struct {
		gasLimit uint64 // worst-case reservation used by the pre-check
		gasUsed  uint64 // actual gas consumed, accounted after inclusion
	}
	candidates := []candidate{
		{gasLimit: 40_000_000, gasUsed: 30_000_000}, // admitted, used -> 30M
		{gasLimit: 40_000_000, gasUsed: 35_000_000}, // admitted (30M+40M<=100M), used -> 65M
		{gasLimit: 40_000_000, gasUsed: 10_000_000}, // skipped: 65M reserved +40M > 100M
		{gasLimit: 30_000_000, gasUsed: 20_000_000}, // admitted (65M+30M<=100M), used -> 85M
		{gasLimit: 20_000_000, gasUsed: 5_000_000},  // skipped: 85M +20M > 100M
	}

	var (
		used     uint64
		included int
	)
	for _, c := range candidates {
		if !gaslessGasFits(used, c.gasLimit, limit) {
			continue
		}
		used += c.gasUsed
		included++
	}

	if wantIncluded := 3; included != wantIncluded {
		t.Fatalf("included %d gasless txs, want %d", included, wantIncluded)
	}
	if wantUsed := uint64(85_000_000); used != wantUsed {
		t.Fatalf("accounted gasless gas used = %d, want %d", used, wantUsed)
	}
	if used > limit {
		t.Fatalf("accounted gasless gas used %d exceeded limit %d", used, limit)
	}
}
