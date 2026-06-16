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

package tracers

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
)

// bcGaslessAllowAll is EVM bytecode for a stub Gasless predeploy whose
// getGaslessAllowance always returns (allowed=true, gasLimit=0xFFFFFFFF). The
// memory layout matches types.DecodeGaslessAllowance.
var bcGaslessAllowAll = []byte{
	0x60, 0x01, 0x60, 0x00, 0x52, // MSTORE(0, 1)           -> allowed = true
	0x63, 0xFF, 0xFF, 0xFF, 0xFF, // PUSH4 0xFFFFFFFF
	0x60, 0x20, 0x52, // MSTORE(32, 0xFFFFFFFF) -> gasLimit
	0x60, 0x40, 0x60, 0x00, 0xF3, // RETURN(0, 64)
}

// TestMarkGaslessReplayClassification verifies that the replay helper used by
// every on-chain tx replay/trace path classifies a zero-fee gasless tx as
// gasless (so it is replayed through the fee-exempt path, matching canonical
// execution) while leaving fee-paying txs untouched.
func TestMarkGaslessReplayClassification(t *testing.T) {
	t.Parallel()

	accounts := newAccounts(1)
	predeploy := types.GaslessAddressFor(params.TestChainConfig.ChainID)
	genesis := &core.Genesis{
		Config: params.TestChainConfig,
		Alloc: types.GenesisAlloc{
			predeploy:        {Code: bcGaslessAllowAll, Balance: big.NewInt(0)},
			accounts[0].addr: {Balance: big.NewInt(params.Ether)},
		},
	}
	backend := newTestBackend(t, 1, genesis, func(i int, b *core.BlockGen) {})
	defer backend.teardown()
	api := NewAPI(backend)

	statedb, err := backend.chain.State()
	if err != nil {
		t.Fatalf("failed to get state: %v", err)
	}
	header := backend.chain.CurrentBlock()
	signer := types.LatestSigner(params.TestChainConfig)
	to := accounts[0].addr

	// A zero-fee tx allowed by the predeploy must be classified gasless.
	gaslessTx := types.MustSignNewTx(accounts[0].key, signer, &types.DynamicFeeTx{
		ChainID:   params.TestChainConfig.ChainID,
		Nonce:     0,
		To:        &to,
		Gas:       21000,
		GasFeeCap: big.NewInt(0),
		GasTipCap: big.NewInt(0),
	})
	msg, err := core.TransactionToMessage(gaslessTx, signer, header.BaseFee)
	if err != nil {
		t.Fatalf("to message: %v", err)
	}
	if msg.IsGaslessTx {
		t.Fatal("precondition: TransactionToMessage must not pre-set IsGaslessTx")
	}
	api.markGasless(msg, gaslessTx, header, statedb)
	if !msg.IsGaslessTx {
		t.Fatal("zero-fee tx should be classified gasless on replay")
	}

	// A fee-paying tx must never be classified gasless.
	feeTx := types.MustSignNewTx(accounts[0].key, signer, &types.DynamicFeeTx{
		ChainID:   params.TestChainConfig.ChainID,
		Nonce:     1,
		To:        &to,
		Gas:       21000,
		GasFeeCap: big.NewInt(1e9),
		GasTipCap: big.NewInt(1),
	})
	msg2, err := core.TransactionToMessage(feeTx, signer, header.BaseFee)
	if err != nil {
		t.Fatalf("to message: %v", err)
	}
	api.markGasless(msg2, feeTx, header, statedb)
	if msg2.IsGaslessTx {
		t.Fatal("fee-paying tx must not be classified gasless on replay")
	}
}
