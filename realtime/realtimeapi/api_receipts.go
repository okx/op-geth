package realtimeapi

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/internal/ethapi"
	"github.com/ethereum/go-ethereum/rpc"
)

// GetTransactionReceipt implements the realtime eth_getTransactionReceipt.
// Returns the receipt of a transaction given the transaction's hash.
func (api *RealtimeAPIImpl) GetTransactionReceipt(ctx context.Context, hash common.Hash) (map[string]interface{}, error) {
	if api.cacheDB == nil || !api.cacheDB.ReadyFlag.Load() {
		return api.transactionApi.GetTransactionReceipt(ctx, hash)
	}

	txn, receipt, _, _, ok := api.cacheDB.Stateless.GetTxInfo(hash)
	if !ok {
		return api.transactionApi.GetTransactionReceipt(ctx, hash)
	}
	header, _, _, _, ok := api.cacheDB.Stateless.GetBlockInfo(receipt.BlockNumber.Uint64())
	if !ok {
		return api.transactionApi.GetTransactionReceipt(ctx, hash)
	}
	signer := types.MakeSigner(api.b.ChainConfig(), header.Number, header.Time)
	return ethapi.MarshalReceipt(receipt, header.Number.Uint64(), signer, txn, api.b.ChainConfig()), nil
}

func (api *RealtimeAPIImpl) GetBlockReceipts(ctx context.Context, number rpc.BlockNumberOrHash) ([]map[string]interface{}, error) {
	if api.cacheDB == nil || !api.cacheDB.ReadyFlag.Load() {
		return api.blockchainApi.GetBlockReceipts(ctx, number)
	}

	blockNum, _, isPending, err := api.getBlockNumberOrHash(number)
	if err != nil {
		return api.blockchainApi.GetBlockReceipts(ctx, number)
	}

	header, _, _, _, ok := api.cacheDB.Stateless.GetBlockInfo(blockNum)
	if !ok {
		if isPending {
			// Pending block not open yet. Default to latest block
			blockNum = api.cacheDB.GetHighestConfirmHeight()
			header, _, _, _, ok = api.cacheDB.Stateless.GetBlockInfo(blockNum)
			if !ok {
				return nil, fmt.Errorf("header not found for block %d", blockNum)
			}
		} else {
			return api.blockchainApi.GetBlockReceipts(ctx, number)
		}
	}

	txHashes, ok := api.cacheDB.Stateless.GetBlockTxs(blockNum)
	if !ok {
		return api.blockchainApi.GetBlockReceipts(ctx, number)
	}
	signer := types.MakeSigner(api.b.ChainConfig(), header.Number, header.Time)
	result := make([]map[string]interface{}, 0, len(txHashes))
	for _, txHash := range txHashes {
		txn, receipt, _, _, exists := api.cacheDB.Stateless.GetTxInfo(txHash)
		if !exists {
			return api.blockchainApi.GetBlockReceipts(ctx, number)
		}
		result = append(result, ethapi.MarshalReceipt(receipt, header.Number.Uint64(), signer, txn, api.b.ChainConfig()))
	}
	return result, nil
}
