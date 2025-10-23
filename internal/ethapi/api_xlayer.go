package ethapi

import (
	"context"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
)

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
