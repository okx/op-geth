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

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
)

// gaslessProbeGasLimit caps the gas spent on a single getGaslessAllowance call.
const gaslessProbeGasLimit uint64 = 30_000_000

// CallGaslessAllowance invokes getGaslessAllowance(to, dataPrefix) on the
// Gasless predeploy. The call runs inside a statedb snapshot which is reverted
// before this function returns, so the world state is never modified.
//
// Failures (no code at the predeploy, call revert, malformed return) yield
// (GaslessAllowance{}, err).
func CallGaslessAllowance(evm *vm.EVM, to common.Address, dataPrefix []byte) (types.GaslessAllowance, error) {
	predeploy := types.GaslessAddressFor(evm.ChainConfig().ChainID)
	if len(evm.StateDB.GetCode(predeploy)) == 0 {
		return types.GaslessAllowance{}, errors.New("gasless: predeploy has no code")
	}

	prevTxCtx := evm.TxContext
	snap := evm.StateDB.Snapshot()
	defer func() {
		evm.StateDB.RevertToSnapshot(snap)
		evm.SetTxContext(prevTxCtx)
	}()

	probe := &Message{
		From:      params.SystemAddress,
		GasLimit:  gaslessProbeGasLimit,
		GasPrice:  common.Big0,
		GasFeeCap: common.Big0,
		GasTipCap: common.Big0,
		To:        &predeploy,
	}
	evm.SetTxContext(NewEVMTxContext(probe))

	ret, _, err := evm.Call(params.SystemAddress, predeploy, types.EncodeGetGaslessAllowanceCalldata(to, dataPrefix), gaslessProbeGasLimit, common.U2560)
	if err != nil {
		return types.GaslessAllowance{}, err
	}
	return types.DecodeGaslessAllowance(ret)
}

// MakeGaslessChecker returns a GaslessChecker bound to the supplied EVM. The
// checker is safe to invoke many times against the same evm — every call
// snapshots and reverts the statedb so successive transactions see a clean
// world state.
func MakeGaslessChecker(evm *vm.EVM) types.GaslessChecker {
	if evm == nil {
		return nil
	}
	predeploy := types.GaslessAddressFor(evm.ChainConfig().ChainID)
	// Cache the "no code" decision once per checker so we don't snapshot the
	// statedb on every tx when the predeploy simply isn't deployed.
	if len(evm.StateDB.GetCode(predeploy)) == 0 {
		noCode := errors.New("gasless: predeploy has no code")
		return func(*types.Transaction) (types.GaslessAllowance, error) {
			return types.GaslessAllowance{}, noCode
		}
	}
	return func(tx *types.Transaction) (types.GaslessAllowance, error) {
		to := tx.To()
		if to == nil {
			return types.GaslessAllowance{}, errors.New("gasless: contract creation has no `to`")
		}
		return CallGaslessAllowance(evm, *to, types.GaslessDataPrefix(tx))
	}
}

// gaslessCheckerForState builds a one-off checker against an arbitrary statedb
// + header. Used by the txpool which lacks a long-lived EVM bound to the head
// state.
func gaslessCheckerForState(chainConfig *params.ChainConfig, header *types.Header, statedb vm.StateDB) types.GaslessChecker {
	if header == nil || statedb == nil {
		return nil
	}
	blockCtx := vm.BlockContext{
		CanTransfer: CanTransfer,
		Transfer:    Transfer,
		GetHash:     func(uint64) common.Hash { return common.Hash{} },
		BlockNumber: new(big.Int).Set(header.Number),
		Time:        header.Time,
		Difficulty:  new(big.Int),
		GasLimit:    header.GasLimit,
	}
	if header.BaseFee != nil {
		blockCtx.BaseFee = new(big.Int).Set(header.BaseFee)
	}
	evm := vm.NewEVM(blockCtx, statedb, chainConfig, vm.Config{})
	return MakeGaslessChecker(evm)
}

// NewGaslessCheckerForState exposes gaslessCheckerForState to other packages.
func NewGaslessCheckerForState(chainConfig *params.ChainConfig, header *types.Header, statedb vm.StateDB) types.GaslessChecker {
	return gaslessCheckerForState(chainConfig, header, statedb)
}
