package realtimeapi

import (
	"context"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/eth/gasestimator"
	"github.com/ethereum/go-ethereum/internal/ethapi"
	"github.com/ethereum/go-ethereum/internal/ethapi/override"
	"github.com/ethereum/go-ethereum/rpc"
)

// Call implements the realtime eth_call.
// Executes a new message call immediately without creating a transaction on the block chain.
// Note that realtime API only supports execution on the latest block.
func (api *RealtimeAPIImpl) Call(ctx context.Context, args ethapi.TransactionArgs, blockNrOrHash *rpc.BlockNumberOrHash, overrides *override.StateOverride, blockOverrides *override.BlockOverrides) (hexutil.Bytes, error) {
	if api.cacheDB == nil || !api.cacheDB.ReadyFlag.Load() {
		backend := ethapi.NewBlockChainAPI(api.b)
		return backend.Call(ctx, args, blockNrOrHash, overrides, blockOverrides)
	}

	if blockNrOrHash == nil {
		latest := rpc.BlockNumberOrHashWithNumber(rpc.LatestBlockNumber)
		blockNrOrHash = &latest
	}

	reader, blockNum, err := api.createStateReader(*blockNrOrHash)
	if err != nil || reader == nil {
		backend := ethapi.NewBlockChainAPI(api.b)
		return backend.Call(ctx, args, blockNrOrHash, overrides, blockOverrides)
	}

	return api.doRealtimeCall(ctx, args, reader, blockNum, overrides, blockOverrides)
}

func (api *RealtimeAPIImpl) doRealtimeCall(ctx context.Context, args ethapi.TransactionArgs, reader state.Reader, blockNum uint64, overrides *override.StateOverride, blockOverrides *override.BlockOverrides) (hexutil.Bytes, error) {
	header, _, _, ok := api.cacheDB.Stateless.GetBlockInfo(blockNum)
	if !ok {
		return nil, fmt.Errorf("header not found for block number %d", blockNum)
	}

	statedb, err := api.GetStateDbWithCacheReader(ctx, reader)
	if err != nil {
		return nil, err
	}

	result, err := ethapi.DoCallRealtime(ctx, api.b, args, statedb, header, overrides, blockOverrides, api.b.RPCEVMTimeout(), api.b.RPCGasCap())
	if err != nil {
		return nil, err
	}
	if errors.Is(result.Err, vm.ErrExecutionReverted) {
		return nil, ethapi.NewRevertError(result.Revert())
	}
	return result.Return(), result.Err
}

// EstimateGas implements the realtime eth_estimateGas.
// Returns an estimate of how much gas is necessary to allow the transaction to complete.
func (api *RealtimeAPIImpl) EstimateGas(ctx context.Context, args ethapi.TransactionArgs, blockNrOrHash *rpc.BlockNumberOrHash, overrides *override.StateOverride, blockOverrides *override.BlockOverrides) (hexutil.Uint64, error) {
	if api.cacheDB == nil || !api.cacheDB.ReadyFlag.Load() {
		backend := ethapi.NewBlockChainAPI(api.b)
		return backend.EstimateGas(ctx, args, blockNrOrHash, overrides, blockOverrides)
	}

	if blockNrOrHash == nil {
		latest := rpc.BlockNumberOrHashWithNumber(rpc.LatestBlockNumber)
		blockNrOrHash = &latest
	}

	reader, blockNum, err := api.createStateReader(*blockNrOrHash)
	if err != nil || reader == nil {
		backend := ethapi.NewBlockChainAPI(api.b)
		return backend.EstimateGas(ctx, args, blockNrOrHash, overrides, blockOverrides)
	}
	header, _, _, ok := api.cacheDB.Stateless.GetBlockInfo(blockNum)
	if !ok {
		return 0, fmt.Errorf("header not found for block number %d", blockNum)
	}

	return api.doRealtimeEstimateGas(ctx, args, reader, header, overrides, blockOverrides)
}

func (api *RealtimeAPIImpl) doRealtimeEstimateGas(ctx context.Context, args ethapi.TransactionArgs, reader state.Reader, header *types.Header, overrides *override.StateOverride, blockOverrides *override.BlockOverrides) (hexutil.Uint64, error) {
	statedb, err := api.GetStateDbWithCacheReader(ctx, reader)
	if err != nil {
		return 0, err
	}

	gasCap := api.b.RPCGasCap()
	if err := overrides.Apply(statedb, nil); err != nil {
		return 0, err
	}
	// Construct the gas estimator option from the user input
	opts := &gasestimator.Options{
		Config:         api.b.ChainConfig(),
		Chain:          ethapi.NewChainContext(ctx, api.b),
		Header:         header,
		BlockOverrides: blockOverrides,
		State:          statedb,
		ErrorRatio:     ethapi.GetEstimateGasErrorRatio(),
	}
	// Set any required transaction default, but make sure the gas cap itself is not messed with
	// if it was not specified in the original argument list.
	if args.Gas == nil {
		args.Gas = new(hexutil.Uint64)
	}
	if err := args.CallDefaults(gasCap, header.BaseFee, api.b.ChainConfig().ChainID); err != nil {
		return 0, err
	}
	call := args.ToMessage(header.BaseFee, true, true)

	// Run the gas estimation and wrap any revertals into a custom return
	estimate, revert, err := gasestimator.Estimate(ctx, call, opts, gasCap)
	if err != nil {
		if errors.Is(err, vm.ErrExecutionReverted) {
			return 0, ethapi.NewRevertError(revert)
		}
		return 0, err
	}
	return hexutil.Uint64(estimate), nil
}
