// Copyright 2025 The go-ethereum Authors
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

package txpool

import (
	"crypto/ecdsa"
	"errors"
	"math"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
)

func TestValidateTransactionEIP2681(t *testing.T) {
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	head := &types.Header{
		Number:     big.NewInt(1),
		GasLimit:   5000000,
		Time:       1,
		Difficulty: big.NewInt(1),
	}

	signer := types.LatestSigner(params.TestChainConfig)

	// Create validation options
	opts := &ValidationOptions{
		Config:       params.TestChainConfig,
		Accept:       0xFF, // Accept all transaction types
		MaxSize:      32 * 1024,
		MaxBlobCount: 6,
		MinTip:       big.NewInt(0),
	}

	tests := []struct {
		name    string
		nonce   uint64
		wantErr error
	}{
		{
			name:    "normal nonce",
			nonce:   42,
			wantErr: nil,
		},
		{
			name:    "max allowed nonce (2^64-2)",
			nonce:   math.MaxUint64 - 1,
			wantErr: nil,
		},
		{
			name:    "EIP-2681 nonce overflow (2^64-1)",
			nonce:   math.MaxUint64,
			wantErr: core.ErrNonceMax,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx := createTestTransaction(key, tt.nonce)
			err := ValidateTransaction(tx, head, signer, opts)

			if tt.wantErr == nil {
				if err != nil {
					t.Errorf("ValidateTransaction() error = %v, wantErr nil", err)
				}
			} else {
				if err == nil {
					t.Errorf("ValidateTransaction() error = nil, wantErr %v", tt.wantErr)
				} else if !errors.Is(err, tt.wantErr) {
					t.Errorf("ValidateTransaction() error = %v, wantErr %v", err, tt.wantErr)
				}
			}
		})
	}
}

// createTestTransaction creates a basic transaction for testing
func createTestTransaction(key *ecdsa.PrivateKey, nonce uint64) *types.Transaction {
	to := common.HexToAddress("0x0000000000000000000000000000000000000001")

	txdata := &types.LegacyTx{
		Nonce:    nonce,
		To:       &to,
		Value:    big.NewInt(1000),
		Gas:      21000,
		GasPrice: big.NewInt(1),
		Data:     nil,
	}

	tx := types.NewTx(txdata)
	signedTx, _ := types.SignTx(tx, types.HomesteadSigner{}, key)
	return signedTx
}

// TestValidateTransaction_GaslessMinTipBypass verifies that the MinTip floor
// in ValidateTransaction is short-circuited when a GaslessChecker reports
// allowed=true for the transaction, and enforced as normal otherwise.
func TestValidateTransaction_GaslessMinTipBypass(t *testing.T) {
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	to := common.HexToAddress("0xabcdef0000000000000000000000000000001234")
	head := &types.Header{
		Number:     big.NewInt(1),
		GasLimit:   5_000_000,
		Time:       1,
		Difficulty: big.NewInt(1),
	}
	signer := types.LatestSigner(params.TestChainConfig)

	mkDynFeeZeroTipped := func() *types.Transaction {
		tx := types.NewTx(&types.DynamicFeeTx{
			ChainID:   params.TestChainConfig.ChainID,
			Nonce:     0,
			To:        &to,
			Gas:       100_000,
			GasFeeCap: big.NewInt(0),
			GasTipCap: big.NewInt(0),
		})
		signed, _ := types.SignTx(tx, signer, key)
		return signed
	}
	mkDynFeePayingTip := func() *types.Transaction {
		tx := types.NewTx(&types.DynamicFeeTx{
			ChainID:   params.TestChainConfig.ChainID,
			Nonce:     0,
			To:        &to,
			Gas:       100_000,
			GasFeeCap: big.NewInt(1_000_000_000),
			GasTipCap: big.NewInt(2),
		})
		signed, _ := types.SignTx(tx, signer, key)
		return signed
	}
	// IsGaslessTxFor requires tx.Gas() <= allowance.GasLimit unconditionally,
	// so the stub must report enough gas to cover the 100_000-gas test tx.
	allowedChecker := types.GaslessChecker(func(*types.Transaction) (types.GaslessAllowance, error) {
		return types.GaslessAllowance{Allowed: true, GasLimit: 1_000_000}, nil
	})

	cases := []struct {
		name    string
		tx      *types.Transaction
		checker types.GaslessChecker
		minTip  *big.Int
		wantErr bool
	}{
		{
			name:    "zero_tip_no_checker_rejected",
			tx:      mkDynFeeZeroTipped(),
			checker: nil,
			minTip:  big.NewInt(1),
			wantErr: true,
		},
		{
			// With the gasless checker allowing, the single price check is
			// short-circuited entirely, so a zero-tip tx is admitted.
			name:    "zero_tip_checker_allows_admitted",
			tx:      mkDynFeeZeroTipped(),
			checker: allowedChecker,
			minTip:  big.NewInt(0),
			wantErr: false,
		},
		{
			// When the checker allows the tx, the MinTip floor is bypassed
			// together with the rest of the price check — a gasless tx carries
			// no tip by design, so even a non-zero MinTip admits it.
			name:    "zero_tip_checker_allows_bypasses_mintip",
			tx:      mkDynFeeZeroTipped(),
			checker: allowedChecker,
			minTip:  big.NewInt(1),
			wantErr: false,
		},
		{
			// No checker and MinTip=0: the only price gate is MinTip, and a
			// zero tip does not fall below a zero floor, so the tx is admitted.
			name:    "zero_tip_zero_mintip_admitted",
			tx:      mkDynFeeZeroTipped(),
			checker: nil,
			minTip:  big.NewInt(0),
			wantErr: false,
		},
		{
			name:    "paying_tip_unaffected_by_checker",
			tx:      mkDynFeePayingTip(),
			checker: nil,
			minTip:  big.NewInt(1),
			wantErr: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts := &ValidationOptions{
				Config:         params.TestChainConfig,
				Accept:         0xFF,
				MaxSize:        32 * 1024,
				MaxBlobCount:   6,
				MinTip:         tc.minTip,
				GaslessChecker: tc.checker,
			}
			err := ValidateTransaction(tc.tx, head, signer, opts)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if !errors.Is(err, ErrTxGasPriceTooLow) {
					t.Fatalf("expected ErrTxGasPriceTooLow, got %v", err)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// TestValidateTransactionWithState_GaslessBalanceCheck verifies that the stateful
// admission balance check only requires balance >= tx.Value() for a gasless tx
// (the consensus gasless branch skips buyGas), while non-gasless txs (and gasless
// txs with balance below their value) are still rejected for insufficient funds.
func TestValidateTransactionWithState_GaslessBalanceCheck(t *testing.T) {
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	from := crypto.PubkeyToAddress(key.PublicKey)
	to := common.HexToAddress("0xabcdef0000000000000000000000000000001234")
	signer := types.LatestSigner(params.TestChainConfig)

	// A nonzero rollup/L1 cost, added on top of the regular tx cost by TotalTxCost.
	rollupCost := uint256.NewInt(500_000)
	rollupCostFn := func(types.RollupTransaction) *uint256.Int {
		return new(uint256.Int).Set(rollupCost)
	}

	// The transferred value of the gasless tx. The full cost (with paying fees)
	// is value + gas*price + rollupCost; the gasless admission cost is just value.
	txValue := big.NewInt(1_000)

	// A zero-fee gasless-shaped tx (no gas fees). IsGaslessTxFor still requires
	// the checker to allow it and gas <= allowance.GasLimit.
	mkGaslessTx := func() *types.Transaction {
		tx := types.NewTx(&types.DynamicFeeTx{
			ChainID:   params.TestChainConfig.ChainID,
			Nonce:     0,
			To:        &to,
			Gas:       100_000,
			GasFeeCap: big.NewInt(0),
			GasTipCap: big.NewInt(0),
			Value:     txValue,
		})
		signed, _ := types.SignTx(tx, signer, key)
		return signed
	}
	// A normal fee-paying tx whose full cost (fees + rollup + value) exceeds a
	// value-only balance. It must never be treated as gasless.
	mkNormalTx := func() *types.Transaction {
		tx := types.NewTx(&types.DynamicFeeTx{
			ChainID:   params.TestChainConfig.ChainID,
			Nonce:     0,
			To:        &to,
			Gas:       100_000,
			GasFeeCap: big.NewInt(1_000_000_000),
			GasTipCap: big.NewInt(1),
			Value:     txValue,
		})
		signed, _ := types.SignTx(tx, signer, key)
		return signed
	}

	allowedChecker := types.GaslessChecker(func(*types.Transaction) (types.GaslessAllowance, error) {
		return types.GaslessAllowance{Allowed: true, GasLimit: 1_000_000}, nil
	})

	// newStateWithBalance returns a fresh statedb with `from` funded by `bal`.
	newStateWithBalance := func(t *testing.T, bal *big.Int) *state.StateDB {
		t.Helper()
		statedb, err := state.New(types.EmptyRootHash, state.NewDatabaseForTesting())
		if err != nil {
			t.Fatalf("failed to create statedb: %v", err)
		}
		statedb.SetBalance(from, uint256.MustFromBig(bal), tracing.BalanceChangeUnspecified)
		return statedb
	}

	cases := []struct {
		name    string
		tx      *types.Transaction
		checker types.GaslessChecker
		balance *big.Int
		wantErr bool
	}{
		{
			// Gasless tx, balance >= value but far below the full cost (could not
			// pay gas/L1/operator fees): accepted because only value is required.
			name:    "gasless_balance_covers_value_only_accepted",
			tx:      mkGaslessTx(),
			checker: allowedChecker,
			balance: txValue, // exactly the value, nothing for fees
			wantErr: false,
		},
		{
			// Gasless tx but balance below the transferred value: still rejected.
			name:    "gasless_balance_below_value_rejected",
			tx:      mkGaslessTx(),
			checker: allowedChecker,
			balance: new(big.Int).Sub(txValue, big.NewInt(1)),
			wantErr: true,
		},
		{
			// Same zero-fee tx but no checker (gasless disabled): falls back to the
			// full-cost check. value alone is not enough, so it is rejected.
			name:    "gasless_shaped_no_checker_value_only_rejected",
			tx:      mkGaslessTx(),
			checker: nil,
			balance: txValue,
			wantErr: true,
		},
		{
			// Non-gasless fee-paying tx with balance below the full cost: rejected
			// (no regression — the value-only relaxation must not leak to it).
			name:    "normal_tx_below_full_cost_rejected",
			tx:      mkNormalTx(),
			checker: allowedChecker, // checker present but tx is not gasless (pays fees)
			balance: txValue,
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			statedb := newStateWithBalance(t, tc.balance)
			opts := &ValidationOptionsWithState{
				State:               statedb,
				RollupCostFn:        rollupCostFn,
				ExistingExpenditure: func(common.Address) *big.Int { return new(big.Int) },
				ExistingCost:        func(common.Address, uint64) *big.Int { return nil },
				GaslessChecker:      tc.checker,
			}
			err := ValidateTransactionWithState(tc.tx, signer, opts)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if !errors.Is(err, core.ErrInsufficientFunds) {
					t.Fatalf("expected ErrInsufficientFunds, got %v", err)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
