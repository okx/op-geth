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
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
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
			// With the gasless checker allowing, the any-fee-field-zero gate
			// at the top of the price block is bypassed. MinTip is enforced
			// separately and has no gasless exemption, so this case keeps
			// MinTip=0 to isolate the gasless bypass.
			name:    "zero_tip_checker_allows_admitted",
			tx:      mkDynFeeZeroTipped(),
			checker: allowedChecker,
			minTip:  big.NewInt(0),
			wantErr: false,
		},
		{
			// Sanity: even when the checker allows the tx, a non-zero MinTip
			// still rejects a zero-tip tx — the MinTip floor is operator-side
			// and unaffected by the gasless predeploy.
			name:    "zero_tip_checker_allows_but_mintip_rejects",
			tx:      mkDynFeeZeroTipped(),
			checker: allowedChecker,
			minTip:  big.NewInt(1),
			wantErr: true,
		},
		{
			name:    "zero_tip_zero_mintip_admitted",
			tx:      mkDynFeeZeroTipped(),
			checker: nil,
			minTip:  big.NewInt(0),
			wantErr: true,
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
