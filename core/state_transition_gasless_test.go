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

package core

import (
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
)

// newGaslessPreCheckEVM builds a fresh EVM whose chain config has Osaka enabled
// or disabled (via OsakaTime) so the EIP-7825 per-tx gas cap is/ isn't active.
// The block context uses BlockNumber=1, Time=1, BaseFee=0 (matching the zero-fee
// gasless model). The sender EOA exists in state with nonce 0 and zero balance.
func newGaslessPreCheckEVM(t *testing.T, osaka bool) (*vm.EVM, common.Address) {
	t.Helper()
	sdb, err := state.New(types.EmptyRootHash, state.NewDatabaseForTesting())
	if err != nil {
		t.Fatalf("statedb: %v", err)
	}
	sender := common.HexToAddress("0x71562b71999873DB5b286dF957af199Ec94617F7")
	// Create the sender account (EOA, nonce 0, zero balance). No code => EOA.
	sdb.CreateAccount(sender)

	cfg := *params.TestChainConfig
	cfg.ChainID = big.NewInt(1)
	if osaka {
		cfg.OsakaTime = new(uint64) // 0 => active at any block/time
	} else {
		cfg.OsakaTime = nil // never active
	}
	blockCtx := vm.BlockContext{
		CanTransfer: CanTransfer,
		Transfer:    Transfer,
		GetHash:     func(uint64) common.Hash { return common.Hash{} },
		BlockNumber: big.NewInt(1),
		Time:        1,
		Difficulty:  new(big.Int),
		GasLimit:    10_000_000,
		BaseFee:     big.NewInt(0),
	}
	evm := vm.NewEVM(blockCtx, sdb, &cfg, vm.Config{})
	// Sanity check that we set Osaka the way we intended.
	if got := evm.ChainConfig().IsOsaka(blockCtx.BlockNumber, blockCtx.Time); got != osaka {
		t.Fatalf("IsOsaka = %v, want %v", got, osaka)
	}
	return evm, sender
}

// newGaslessMessage builds a zero-fee gasless Message for `sender` with the
// given gas limit. All other fields are kept valid so the only check under test
// is the EIP-7825 cap.
func newGaslessMessage(sender common.Address, gasLimit uint64) *Message {
	to := common.HexToAddress("0x00000000000000000000000000000000000000ff")
	return &Message{
		To:          &to,
		From:        sender,
		Nonce:       0,
		Value:       big.NewInt(0),
		GasLimit:    gasLimit,
		GasPrice:    big.NewInt(0),
		GasFeeCap:   big.NewInt(0),
		GasTipCap:   big.NewInt(0),
		Data:        nil,
		IsGaslessTx: true,
	}
}

// TestGaslessTxEnforcesMaxTxGasCap verifies that the gasless branch of
// preCheck() enforces the EIP-7825 per-tx gas cap (issue #6 fix): previously
// the gasless branch early-returned and skipped the cap check entirely.
func TestGaslessTxEnforcesMaxTxGasCap(t *testing.T) {
	t.Run("over_cap_osaka_rejected", func(t *testing.T) {
		evm, sender := newGaslessPreCheckEVM(t, true)
		msg := newGaslessMessage(sender, params.MaxTxGas+1)
		// Pool is large enough that SubGas would succeed; the cap check must
		// fire first and reject the tx.
		gp := new(GasPool).AddGas(params.MaxTxGas * 2)
		st := newStateTransition(evm, msg, gp)

		err := st.preCheck()
		if !errors.Is(err, ErrGasLimitTooHigh) {
			t.Fatalf("gasless over-cap tx: got err %v, want ErrGasLimitTooHigh", err)
		}
	})

	t.Run("at_cap_osaka_not_rejected_by_cap", func(t *testing.T) {
		evm, sender := newGaslessPreCheckEVM(t, true)
		msg := newGaslessMessage(sender, params.MaxTxGas) // exactly at cap, allowed
		gp := new(GasPool).AddGas(params.MaxTxGas * 2)
		st := newStateTransition(evm, msg, gp)

		// preCheck must NOT reject this for the cap reason. It may return nil
		// (it should here) or some unrelated error, but never ErrGasLimitTooHigh.
		if err := st.preCheck(); errors.Is(err, ErrGasLimitTooHigh) {
			t.Fatalf("gasless at-cap tx wrongly rejected by cap check: %v", err)
		}
	})

	t.Run("over_cap_non_osaka_not_rejected", func(t *testing.T) {
		evm, sender := newGaslessPreCheckEVM(t, false)
		// Over the cap, but Osaka is inactive so the cap is not enforced.
		msg := newGaslessMessage(sender, params.MaxTxGas+1)
		gp := new(GasPool).AddGas(params.MaxTxGas * 2)
		st := newStateTransition(evm, msg, gp)

		if err := st.preCheck(); errors.Is(err, ErrGasLimitTooHigh) {
			t.Fatalf("gasless over-cap tx on non-Osaka config wrongly rejected by cap check: %v", err)
		}
	})
}

// TestGaslessTxEnforcesAuthList verifies that the gasless branch of preCheck()
// enforces EIP-7702 authorization-list well-formedness:
// gasless only waives fees, not type-level validity, so an empty auth list must
// still be rejected (matching reth/revm, which only relaxes the base-fee check),
// while a well-formed auth list passes the auth-list check.
func TestGaslessTxEnforcesAuthList(t *testing.T) {
	t.Run("empty_auth_list_rejected", func(t *testing.T) {
		evm, sender := newGaslessPreCheckEVM(t, true)
		msg := newGaslessMessage(sender, 100_000)
		// Non-nil but empty authorization list => ErrEmptyAuthList.
		msg.SetCodeAuthorizations = []types.SetCodeAuthorization{}
		gp := new(GasPool).AddGas(params.MaxTxGas * 2)
		st := newStateTransition(evm, msg, gp)

		if err := st.preCheck(); !errors.Is(err, ErrEmptyAuthList) {
			t.Fatalf("gasless empty-auth-list tx: got err %v, want ErrEmptyAuthList", err)
		}
	})

	t.Run("create_with_auth_list_rejected", func(t *testing.T) {
		evm, sender := newGaslessPreCheckEVM(t, true)
		msg := newGaslessMessage(sender, 100_000)
		msg.To = nil // SetCode tx must have a `to`
		msg.SetCodeAuthorizations = []types.SetCodeAuthorization{{Address: sender}}
		gp := new(GasPool).AddGas(params.MaxTxGas * 2)
		st := newStateTransition(evm, msg, gp)

		if err := st.preCheck(); !errors.Is(err, ErrSetCodeTxCreate) {
			t.Fatalf("gasless setcode-create tx: got err %v, want ErrSetCodeTxCreate", err)
		}
	})

	t.Run("valid_auth_list_passes", func(t *testing.T) {
		evm, sender := newGaslessPreCheckEVM(t, true)
		msg := newGaslessMessage(sender, 100_000)
		// Well-formed (non-empty) authorization list on a tx with a `to`.
		msg.SetCodeAuthorizations = []types.SetCodeAuthorization{{Address: sender}}
		gp := new(GasPool).AddGas(params.MaxTxGas * 2)
		st := newStateTransition(evm, msg, gp)

		// Must not be rejected by the auth-list checks; the zero-fee gasless tx
		// should clear preCheck entirely here.
		if err := st.preCheck(); err != nil {
			t.Fatalf("gasless tx with valid auth list wrongly rejected: %v", err)
		}
	})
}
