package realtimeapi

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/internal/ethapi"
	"github.com/ethereum/go-ethereum/rpc"
)

func (api *RealtimeAPIImpl) BlockNumber(ctx context.Context, tag *RealtimeTag) (hexutil.Uint64, error) {
	if api.cacheDB == nil || !api.cacheDB.ReadyFlag.Load() {
		return api.blockchainApi.BlockNumber(), nil
	}

	if tag == nil {
		// Default to latest block number if no tag is provided
		latestTag := Latest
		tag = &latestTag
	}

	blockNumber, _, _, err := api.getBlockNumber(rpc.BlockNumber(*tag))
	if err != nil {
		// Do not redirect to default eth api as block number with tag is custom for realtime
		return hexutil.Uint64(0), err
	}
	return hexutil.Uint64(blockNumber), nil
}

func (api *RealtimeAPIImpl) GetBlockTransactionCountByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*hexutil.Uint, error) {
	if api.cacheDB == nil || !api.cacheDB.ReadyFlag.Load() {
		return api.transactionApi.GetBlockTransactionCountByNumber(ctx, blockNr)
	}

	blockNum, _, isPending, err := api.getBlockNumber(blockNr)
	if err != nil {
		return api.transactionApi.GetBlockTransactionCountByNumber(ctx, blockNr)
	}

	_, _, _, _, ok := api.cacheDB.Stateless.GetBlockInfo(blockNum)
	if !ok {
		if isPending {
			numOfTx := hexutil.Uint(0)
			return &numOfTx, nil
		} else {
			return api.transactionApi.GetBlockTransactionCountByNumber(ctx, blockNr)
		}
	}

	txs, ok := api.cacheDB.Stateless.GetBlockTxs(blockNum)
	if !ok {
		return api.transactionApi.GetBlockTransactionCountByNumber(ctx, blockNr)
	}
	numOfTx := hexutil.Uint(len(txs))
	return &numOfTx, nil
}

func (api *RealtimeAPIImpl) GetBlockTransactionCountByHash(ctx context.Context, blockHash common.Hash) (*hexutil.Uint, error) {
	if api.cacheDB == nil || !api.cacheDB.ReadyFlag.Load() {
		return api.transactionApi.GetBlockTransactionCountByHash(ctx, blockHash)
	}

	blockNum, found := api.cacheDB.Stateless.GetBlockNumberByHash(blockHash)
	if !found {
		return api.transactionApi.GetBlockTransactionCountByHash(ctx, blockHash)
	}

	txHashes, ok := api.cacheDB.Stateless.GetBlockTxs(blockNum)
	if !ok {
		return api.transactionApi.GetBlockTransactionCountByHash(ctx, blockHash)
	}
	numOfTx := hexutil.Uint(len(txHashes))
	return &numOfTx, nil
}

func (api *RealtimeAPIImpl) GetBlockByNumber(ctx context.Context, blockNr rpc.BlockNumber, fullTx bool) (map[string]interface{}, error) {
	if api.cacheDB == nil || !api.cacheDB.ReadyFlag.Load() {
		return api.blockchainApi.GetBlockByNumber(ctx, blockNr, fullTx)
	}

	blockNum, _, isPending, err := api.getBlockNumber(blockNr)
	if err != nil {
		return api.blockchainApi.GetBlockByNumber(ctx, blockNr, fullTx)
	}
	response, err := api.tryGetBlockResponseFromNumber(ctx, blockNum, fullTx, isPending)
	if err != nil {
		return api.blockchainApi.GetBlockByNumber(ctx, blockNr, fullTx)
	}
	return response, nil
}

func (api *RealtimeAPIImpl) GetBlockByHash(ctx context.Context, hash common.Hash, fullTx bool) (map[string]interface{}, error) {
	if api.cacheDB == nil || !api.cacheDB.ReadyFlag.Load() {
		return api.blockchainApi.GetBlockByHash(ctx, hash, fullTx)
	}

	blockNum, found := api.cacheDB.Stateless.GetBlockNumberByHash(hash)
	if !found {
		return api.blockchainApi.GetBlockByHash(ctx, hash, fullTx)
	}
	response, err := api.tryGetBlockResponseFromNumber(ctx, blockNum, fullTx, false)
	if err != nil {
		return api.blockchainApi.GetBlockByHash(ctx, hash, fullTx)
	}
	return response, nil
}

func (api *RealtimeAPIImpl) GetBlockInternalTransactions(ctx context.Context, blockNr rpc.BlockNumber) (map[common.Hash][]*types.InnerTx, error) {
	if api.cacheDB == nil || !api.cacheDB.ReadyFlag.Load() {
		return api.transactionApi.GetBlockInternalTransactions(ctx, blockNr)
	}

	blockNum, _, isPending, err := api.getBlockNumber(blockNr)
	if err != nil {
		return api.transactionApi.GetBlockInternalTransactions(ctx, blockNr)
	}

	_, _, _, _, ok := api.cacheDB.Stateless.GetBlockInfo(blockNum)
	if !ok {
		if isPending {
			// Pending block not open yet. Default to latest block
			blockNum = api.cacheDB.GetHighestConfirmHeight()
			_, _, _, _, ok = api.cacheDB.Stateless.GetBlockInfo(blockNum)
			if !ok {
				return nil, fmt.Errorf("header not found for block %d", blockNum)
			}
		} else {
			return api.transactionApi.GetBlockInternalTransactions(ctx, blockNr)
		}
	}

	txHashes, ok := api.cacheDB.Stateless.GetBlockTxs(blockNum)
	if !ok {
		return api.transactionApi.GetBlockInternalTransactions(ctx, blockNr)
	}

	result := make(map[common.Hash][]*types.InnerTx)

	for _, txHash := range txHashes {
		_, _, _, innerTxs, exists := api.cacheDB.Stateless.GetTxInfo(txHash)
		if !exists {
			return api.transactionApi.GetBlockInternalTransactions(ctx, blockNr)
		}
		result[txHash] = innerTxs
	}
	return result, nil
}

func (api *RealtimeAPIImpl) GetTransactionByBlockNumberAndIndex(ctx context.Context, blockNr rpc.BlockNumber, index hexutil.Uint) (*ethapi.RPCTransaction, error) {
	if api.cacheDB == nil || !api.cacheDB.ReadyFlag.Load() {
		return api.transactionApi.GetTransactionByBlockNumberAndIndex(ctx, blockNr, index)
	}

	blockNum, _, _, err := api.getBlockNumber(blockNr)
	if err != nil {
		return api.transactionApi.GetTransactionByBlockNumberAndIndex(ctx, blockNr, index)
	}
	header, _, _, blockhash, ok := api.cacheDB.Stateless.GetBlockInfo(blockNum)
	if !ok {
		return api.transactionApi.GetTransactionByBlockNumberAndIndex(ctx, blockNr, index)
	}
	txHashes, ok := api.cacheDB.Stateless.GetBlockTxs(blockNum)
	if !ok {
		return api.transactionApi.GetTransactionByBlockNumberAndIndex(ctx, blockNr, index)
	}
	txHash := txHashes[index]
	txn, receipt, _, _, exists := api.cacheDB.Stateless.GetTxInfo(txHash)
	if !exists {
		return nil, nil
	}
	return newRPCTransaction_realtime(txn, blockhash, blockNum, header.Time, uint64(receipt.TransactionIndex), header.BaseFee, api.b.ChainConfig(), receipt), nil
}

func (api *RealtimeAPIImpl) GetTransactionByBlockHashAndIndex(ctx context.Context, blockHash common.Hash, index hexutil.Uint) (*ethapi.RPCTransaction, error) {
	if api.cacheDB == nil || !api.cacheDB.ReadyFlag.Load() {
		return api.transactionApi.GetTransactionByBlockHashAndIndex(ctx, blockHash, index)
	}

	blockNum, found := api.cacheDB.Stateless.GetBlockNumberByHash(blockHash)
	if !found {
		return api.transactionApi.GetTransactionByBlockHashAndIndex(ctx, blockHash, index)
	}
	return api.GetTransactionByBlockNumberAndIndex(ctx, rpc.BlockNumber(blockNum), index)
}
