package ethapi

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	libcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/internal/ethapi/override"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"
)

func NewRPCTransaction(tx *types.Transaction, txblockhash libcommon.Hash, blockNumber uint64, blockTime uint64, index uint64, baseFee *big.Int, config *params.ChainConfig, receipt *types.Receipt) *RPCTransaction {
	return newRPCTransaction(tx, txblockhash, blockNumber, blockTime, index, baseFee, config, receipt)
}

func DoCallRealtime(ctx context.Context, b Backend, args TransactionArgs, statedb *state.StateDB, header *types.Header, overrides *override.StateOverride, blockOverrides *override.BlockOverrides, timeout time.Duration, globalGasCap uint64) (*core.ExecutionResult, error) {
	defer func(start time.Time) { log.Debug("Executing EVM call finished", "runtime", time.Since(start)) }(time.Now())
	return doCall(ctx, b, args, statedb, header, overrides, blockOverrides, timeout, globalGasCap)
}

func MarshalReceipt(receipt *types.Receipt, blockNumber uint64, signer types.Signer, tx *types.Transaction, chainConfig *params.ChainConfig) map[string]interface{} {
	return marshalReceipt(receipt, receipt.BlockHash, blockNumber, signer, tx, int(receipt.TransactionIndex), chainConfig)
}

func GetEstimateGasErrorRatio() float64 {
	return estimateGasErrorRatio
}

func NewRevertError(revert []byte) *revertError {
	return newRevertError(revert)
}

// RealtimeEnabled returns the status on whether the RT feature is enabled (when tag provided)
func (*BlockChainAPI) RealtimeEnabled(ctx context.Context) (bool, error) {
	return false, nil
}

// GetInternalTransactions returns the inner transactions for a given transaction hash
func (api *TransactionAPI) GetInternalTransactions(ctx context.Context, txHash common.Hash) ([]*types.InnerTx, error) {
	// Check if inner transaction feature is enabled
	if xlayerBackend, ok := api.b.(XLayerBackend); ok && !xlayerBackend.IsInnerTxEnabled() {
		return nil, errors.New("unsupported internal transaction method")
	}

	innerTxs, err := rawdb.ReadInnerTxsByTxHash(api.b.ChainDb(), txHash)

	if err != nil {
		return nil, fmt.Errorf("failed to read inner transactions: %w", err)
	}

	if innerTxs == nil {
		return []*types.InnerTx{}, nil
	}

	return innerTxs, nil
}

// GetBlockInternalTransactions returns all inner transactions for all transactions in a block
func (api *TransactionAPI) GetBlockInternalTransactions(ctx context.Context, blockNr rpc.BlockNumber) (map[common.Hash][]*types.InnerTx, error) {
	// Check if inner transaction feature is enabled
	if xlayerBackend, ok := api.b.(XLayerBackend); ok && !xlayerBackend.IsInnerTxEnabled() {
		return nil, errors.New("unsupported internal transaction method")
	}

	block, err := api.b.BlockByNumber(ctx, blockNr)
	if err != nil {
		return nil, fmt.Errorf("failed to get block: %w", err)
	}
	if block == nil {
		return nil, fmt.Errorf("block not found")
	}

	blockNum := block.NumberU64()
	transactions := block.Transactions()
	result := make(map[common.Hash][]*types.InnerTx)

	// Retrieve inner transactions for each transaction in the block
	for i, tx := range transactions {
		innerTxs, err := rawdb.ReadInnerTxs(api.b.ChainDb(), blockNum, uint32(i))
		if err != nil {
			continue
		}
		if len(innerTxs) > 0 {
			// Use transaction hash as key
			result[tx.Hash()] = innerTxs
		}
	}

	return result, nil
}
