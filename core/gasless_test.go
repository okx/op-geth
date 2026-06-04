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
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
)

// Minimal EVM bytecode stubs used by these tests. They ignore calldata and
// return a fixed payload that the gasless helpers parse.
var (
	// REVERTs immediately with empty data.
	bcRevert = []byte{0x60, 0x00, 0x60, 0x00, 0xfd}
	// returns 64 zero bytes — ABI-encoded (false, 0) for getGaslessAllowance.
	bcReturnDisallowed = []byte{0x60, 0x40, 0x60, 0x00, 0xf3}
)

// bcReturnAllowance returns bytecode that yields (true, uint64(gasLimit)) ABI-
// encoded. gasLimit must fit in a single byte (PUSH1) for this minimal stub.
func bcReturnAllowance(gasLimit byte) []byte {
	return []byte{
		// memory[0..32] = 0x...01 (allowed=true)
		0x60, 0x01, 0x60, 0x00, 0x52,
		// memory[32..64] = uint256(gasLimit) — single PUSH1 sits at byte 63
		0x60, gasLimit, 0x60, 0x20, 0x52,
		// RETURN memory[0..64]
		0x60, 0x40, 0x60, 0x00, 0xf3,
	}
}

// setupGaslessEVM builds an EVM bound to a fresh statedb that has `code`
// installed at the gasless predeploy address resolved from chainID. If
// code is nil, the predeploy is left empty.
func setupGaslessEVM(t *testing.T, chainID *big.Int, code []byte) *vm.EVM {
	t.Helper()
	sdb, err := state.New(types.EmptyRootHash, state.NewDatabaseForTesting())
	if err != nil {
		t.Fatalf("statedb: %v", err)
	}
	if len(code) > 0 {
		sdb.SetCode(types.GaslessAddressFor(chainID), code, tracing.CodeChangeGenesis)
	}
	cfg := *params.TestChainConfig
	cfg.ChainID = new(big.Int).Set(chainID)
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
	return vm.NewEVM(blockCtx, sdb, &cfg, vm.Config{})
}

func TestCallGaslessAllowance(t *testing.T) {
	to := common.HexToAddress("0xabcd000000000000000000000000000000000001")
	data := []byte{0xde, 0xad, 0xbe, 0xef}

	t.Run("no_code", func(t *testing.T) {
		evm := setupGaslessEVM(t, big.NewInt(1), nil)
		if _, err := CallGaslessAllowance(evm, to, data); err == nil {
			t.Fatal("expected error when predeploy has no code")
		}
	})

	t.Run("allowed_with_limit", func(t *testing.T) {
		evm := setupGaslessEVM(t, big.NewInt(1), bcReturnAllowance(0x80))
		out, err := CallGaslessAllowance(evm, to, data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.Allowed {
			t.Fatal("expected Allowed=true")
		}
		if out.GasLimit != 0x80 {
			t.Fatalf("got gasLimit %d want %d", out.GasLimit, 0x80)
		}
	})

	t.Run("disallowed", func(t *testing.T) {
		evm := setupGaslessEVM(t, big.NewInt(1), bcReturnDisallowed)
		out, err := CallGaslessAllowance(evm, to, data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.Allowed {
			t.Fatalf("expected Allowed=false, got %+v", out)
		}
		if out.GasLimit != 0 {
			t.Fatalf("expected gasLimit 0, got %d", out.GasLimit)
		}
	})

	t.Run("reverts", func(t *testing.T) {
		evm := setupGaslessEVM(t, big.NewInt(1), bcRevert)
		if _, err := CallGaslessAllowance(evm, to, data); err == nil {
			t.Fatal("expected error on revert")
		}
	})
}

func TestCallGaslessAllowance_ChainIDSelectsAddress(t *testing.T) {
	// Deploy the allowance stub only at the chain-196 predeploy address;
	// confirm CallGaslessAllowance routed to that address (via the chain id
	// embedded in evm.ChainConfig) and decoded the response.
	sdb, err := state.New(types.EmptyRootHash, state.NewDatabaseForTesting())
	if err != nil {
		t.Fatal(err)
	}
	chain196Addr := types.GaslessAddressFor(big.NewInt(196))
	defaultAddr := types.GaslessAddressFor(big.NewInt(1))
	if chain196Addr == defaultAddr {
		t.Fatalf("chain-id routing collapse: 196=%s default=%s", chain196Addr.Hex(), defaultAddr.Hex())
	}
	sdb.SetCode(chain196Addr, bcReturnAllowance(0x10), tracing.CodeChangeGenesis)

	cfg := *params.TestChainConfig
	cfg.ChainID = big.NewInt(196)
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

	out, err := CallGaslessAllowance(evm, common.Address{}, nil)
	if err != nil {
		t.Fatalf("expected no error (chain-196 address has code), got %v", err)
	}
	if !out.Allowed || out.GasLimit != 0x10 {
		t.Fatalf("got %+v", out)
	}
}

func TestMakeGaslessChecker(t *testing.T) {
	to := common.HexToAddress("0xabcd000000000000000000000000000000000002")

	t.Run("nil_evm", func(t *testing.T) {
		if MakeGaslessChecker(nil) != nil {
			t.Fatal("expected nil checker for nil evm")
		}
	})

	t.Run("no_code_returns_failing_checker", func(t *testing.T) {
		evm := setupGaslessEVM(t, big.NewInt(1), nil)
		chk := MakeGaslessChecker(evm)
		if chk == nil {
			t.Fatal("expected non-nil checker")
		}
		tx := types.NewTx(&types.DynamicFeeTx{To: &to})
		_, err := chk(tx)
		if err == nil {
			t.Fatal("expected error from no-code checker")
		}
	})

	t.Run("with_code_returns_working_checker", func(t *testing.T) {
		evm := setupGaslessEVM(t, big.NewInt(1), bcReturnAllowance(0x42))
		chk := MakeGaslessChecker(evm)
		tx := types.NewTx(&types.DynamicFeeTx{To: &to, Data: []byte{1, 2, 3}})
		out, err := chk(tx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.Allowed || out.GasLimit != 0x42 {
			t.Fatalf("got %+v", out)
		}
	})

	t.Run("contract_creation_tx_errors", func(t *testing.T) {
		evm := setupGaslessEVM(t, big.NewInt(1), bcReturnAllowance(0x42))
		chk := MakeGaslessChecker(evm)
		tx := types.NewTx(&types.DynamicFeeTx{To: nil})
		if _, err := chk(tx); err == nil {
			t.Fatal("expected error for contract-creation tx (nil to)")
		}
	})
}

func TestNewGaslessCheckerForState(t *testing.T) {
	to := common.HexToAddress("0xabcd000000000000000000000000000000000003")

	t.Run("nil_header_returns_nil", func(t *testing.T) {
		sdb, _ := state.New(types.EmptyRootHash, state.NewDatabaseForTesting())
		if NewGaslessCheckerForState(params.TestChainConfig, nil, sdb) != nil {
			t.Fatal("expected nil checker for nil header")
		}
	})

	t.Run("nil_statedb_returns_nil", func(t *testing.T) {
		hdr := &types.Header{Number: big.NewInt(1), Time: 1, GasLimit: 10_000_000, Difficulty: new(big.Int)}
		if NewGaslessCheckerForState(params.TestChainConfig, hdr, nil) != nil {
			t.Fatal("expected nil checker for nil statedb")
		}
	})

	t.Run("delegates_to_contract", func(t *testing.T) {
		sdb, _ := state.New(types.EmptyRootHash, state.NewDatabaseForTesting())
		sdb.SetCode(types.GaslessAddressFor(big.NewInt(1)), bcReturnAllowance(0x33), tracing.CodeChangeGenesis)
		hdr := &types.Header{
			Number:     big.NewInt(1),
			Time:       1,
			GasLimit:   10_000_000,
			Difficulty: new(big.Int),
			BaseFee:    big.NewInt(0),
		}
		chk := NewGaslessCheckerForState(params.TestChainConfig, hdr, sdb)
		if chk == nil {
			t.Fatal("expected non-nil checker")
		}
		tx := types.NewTx(&types.DynamicFeeTx{To: &to})
		out, err := chk(tx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.Allowed || out.GasLimit != 0x33 {
			t.Fatalf("got %+v", out)
		}
	})
}
