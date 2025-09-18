// Copyright 2019 The go-ethereum Authors
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

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/internal/monitor"
)

// ApplyTransaction_XLayer attempts to apply a transaction to the given state
// database and uses the input parameters for its environment. It returns the
// receipt for the transaction, gas used and an error if the transaction failed,
// indicating the block was invalid.
func ApplyTransaction_XLayer(evm *vm.EVM, gp *GasPool, statedb *state.StateDB, header *types.Header, tx *types.Transaction, usedGas *uint64, shouldFinalize bool) (*types.Receipt, []*types.InnerTx, error) {
	msg, err := TransactionToMessage(tx, types.MakeSigner(evm.ChainConfig(), header.Number, header.Time), header.BaseFee)
	if err != nil {
		return nil, nil, err
	}
	// Create a new context to be used in the EVM environment
	return ApplyTransactionWithEVM_XLayer(msg, gp, statedb, header.Number, header.Hash(), tx, usedGas, evm, shouldFinalize)
}

func ApplyTransactionWithEVM_XLayer(msg *Message, gp *GasPool, statedb *state.StateDB, blockNumber *big.Int, blockHash common.Hash, tx *types.Transaction, usedGas *uint64, evm *vm.EVM, shouldFinalize bool) (receipt *types.Receipt, innerTxs []*types.InnerTx, err error) {
	txHash := tx.Hash().Hex()

	// For X Layer, log transaction application start
	monitor.LogTransactionProgress(txHash, monitor.ServiceNameState, monitor.StepStateApplyTx.ID,
		monitor.StepStateApplyTx.Key, blockNumber.Uint64(), int8(tx.Type()), "applying", 0)

	if hooks := evm.Config.Tracer; hooks != nil {
		if hooks.OnTxStart != nil {
			hooks.OnTxStart(evm.GetVMContext(), tx, msg.From)
		}
		if hooks.OnTxEnd != nil {
			defer func() { hooks.OnTxEnd(receipt, err) }()
		}
	}

	nonce := tx.Nonce()
	if msg.IsDepositTx && evm.ChainConfig().IsOptimismRegolith(evm.Context.Time) {
		nonce = statedb.GetNonce(msg.From)
	}

	// Apply the transaction to the current state (included in the env).
	result, err := ApplyMessage(evm, msg, gp)
	if err != nil {
		return nil, nil, err
	}
	// Update the state with pending changes.
	var root []byte
	if evm.ChainConfig().IsByzantium(blockNumber) {
		if shouldFinalize {
			evm.StateDB.Finalise(true)
		}
	} else {
		root = statedb.IntermediateRoot(evm.ChainConfig().IsEIP158(blockNumber)).Bytes()
	}
	*usedGas += result.UsedGas

	// Merge the tx-local access event into the "block-local" one, in order to collect
	// all values, so that the witness can be built.
	if statedb.GetTrie().IsVerkle() {
		statedb.AccessEvents().Merge(evm.AccessEvents)
	}

	// For X Layer, log receipt generation
	monitor.LogTransactionProgress(txHash, monitor.ServiceNameState, monitor.StepStateGenerateReceipt.ID,
		monitor.StepStateGenerateReceipt.Key, blockNumber.Uint64(), int8(tx.Type()), "generating_receipt", result.UsedGas)

	// For X Layer
	if evm.Config.EnableInnerTxs {
		innerTxs = afterApplyTransaction(evm, result.Failed())
	}

	return MakeReceipt(evm, result, statedb, blockNumber, blockHash, tx, *usedGas, root, evm.ChainConfig(), nonce), innerTxs, nil
}

func afterApplyTransaction(env *vm.EVM, failed bool) []*types.InnerTx {
	innerTxs := env.GetInnerTxMeta().InnerTxs
	if failed {
		for _, innerTx := range innerTxs {
			innerTx.IsError = true
		}
	}
	return innerTxs
}
