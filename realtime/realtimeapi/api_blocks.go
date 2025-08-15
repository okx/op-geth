package realtimeapi

import (
	"context"

	libcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/internal/ethapi"
	"github.com/ethereum/go-ethereum/rpc"
)

func (api *RealtimeAPIImpl) BlockNumber(ctx context.Context, tag *RealtimeTag) (hexutil.Uint64, error) {
	if api.cacheDB == nil || !api.cacheDB.ReadyFlag.Load() {
		backend := ethapi.NewBlockChainAPI(api.b)
		return backend.BlockNumber(), nil
	}

	if tag == nil {
		// Default to latest block number if no tag is provided
		latestTag := Latest
		tag = &latestTag
	}

	blockNumber, _, err := api.getBlockNumber(rpc.BlockNumber(*tag))
	if err != nil {
		// Do not redirect to default eth api as block number with tag is custom for realtime
		return hexutil.Uint64(0), err
	}
	return hexutil.Uint64(blockNumber), nil
}

func (api *RealtimeAPIImpl) GetBlockTransactionCountByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*hexutil.Uint, error) {
	if api.cacheDB == nil || !api.cacheDB.ReadyFlag.Load() {
		backend := ethapi.NewTransactionAPI(api.b, nil)
		return backend.GetBlockTransactionCountByNumber(ctx, blockNr)
	}

	blockNum, _, err := api.getBlockNumber(blockNr)
	if err != nil {
		backend := ethapi.NewTransactionAPI(api.b, nil)
		return backend.GetBlockTransactionCountByNumber(ctx, blockNr)
	}

	_, _, _, ok := api.cacheDB.Stateless.GetHeader(blockNum)
	if !ok {
		backend := ethapi.NewTransactionAPI(api.b, nil)
		return backend.GetBlockTransactionCountByNumber(ctx, blockNr)
	}

	txs, ok := api.cacheDB.Stateless.GetBlockTxs(blockNum)
	if !ok {
		backend := ethapi.NewTransactionAPI(api.b, nil)
		return backend.GetBlockTransactionCountByNumber(ctx, blockNr)
	}
	numOfTx := hexutil.Uint(len(txs))
	return &numOfTx, nil
}

func (api *RealtimeAPIImpl) GetBlockTransactionCountByHash(ctx context.Context, blockHash libcommon.Hash) (*hexutil.Uint, error) {
	if api.cacheDB == nil || !api.cacheDB.ReadyFlag.Load() {
		backend := ethapi.NewTransactionAPI(api.b, nil)
		return backend.GetBlockTransactionCountByHash(ctx, blockHash)
	}

	blockNum, found := api.cacheDB.Stateless.GetBlockNumberByHash(blockHash)
	if !found {
		backend := ethapi.NewTransactionAPI(api.b, nil)
		return backend.GetBlockTransactionCountByHash(ctx, blockHash)
	}

	txHashes, ok := api.cacheDB.Stateless.GetBlockTxs(blockNum)
	if !ok {
		backend := ethapi.NewTransactionAPI(api.b, nil)
		return backend.GetBlockTransactionCountByHash(ctx, blockHash)
	}

	numOfTx := hexutil.Uint(len(txHashes))
	return &numOfTx, nil
}

func (api *RealtimeAPIImpl) GetBlockByNumber(ctx context.Context, blockNr rpc.BlockNumber, fullTx bool) (map[string]interface{}, error) {
	if api.cacheDB == nil || !api.cacheDB.ReadyFlag.Load() {
		backend := ethapi.NewBlockChainAPI(api.b)
		return backend.GetBlockByNumber(ctx, blockNr, fullTx)
	}

	blockNum, _, err := api.getBlockNumber(blockNr)
	if err != nil {
		backend := ethapi.NewBlockChainAPI(api.b)
		return backend.GetBlockByNumber(ctx, blockNr, fullTx)
	}

	response, err := api.tryGetBlockResponseFromNumber(ctx, blockNum, fullTx)
	if err != nil {
		backend := ethapi.NewBlockChainAPI(api.b)
		return backend.GetBlockByNumber(ctx, blockNr, fullTx)
	}

	if blockNr == rpc.PendingBlockNumber {
		for _, field := range []string{"hash", "nonce", "miner"} {
			response[field] = nil
		}
	}

	return response, nil
}

func (api *RealtimeAPIImpl) GetBlockByHash(ctx context.Context, hash libcommon.Hash, fullTx bool) (map[string]interface{}, error) {
	if api.cacheDB == nil || !api.cacheDB.ReadyFlag.Load() {
		backend := ethapi.NewBlockChainAPI(api.b)
		return backend.GetBlockByHash(ctx, hash, fullTx)
	}

	blockNum, found := api.cacheDB.Stateless.GetBlockNumberByHash(hash)
	if !found {
		backend := ethapi.NewBlockChainAPI(api.b)
		return backend.GetBlockByHash(ctx, hash, fullTx)
	}

	response, err := api.tryGetBlockResponseFromNumber(ctx, blockNum, fullTx)
	if err != nil {
		backend := ethapi.NewBlockChainAPI(api.b)
		return backend.GetBlockByHash(ctx, hash, fullTx)
	}

	return response, nil
}

func (api *RealtimeAPIImpl) GetBlockInternalTransactions(ctx context.Context, blockNr rpc.BlockNumber) (map[libcommon.Hash][]*types.InnerTx, error) {
	if api.cacheDB == nil || !api.cacheDB.ReadyFlag.Load() {
		backend := ethapi.NewBlockChainAPI(api.b)
		return backend.GetBlockInternalTransactions(ctx, blockNr)
	}

	blockNum, _, err := api.getBlockNumber(blockNr)
	if err != nil {
		backend := ethapi.NewBlockChainAPI(api.b)
		return backend.GetBlockInternalTransactions(ctx, blockNr)
	}

	_, _, _, ok := api.cacheDB.Stateless.GetHeader(blockNum)
	if !ok {
		backend := ethapi.NewBlockChainAPI(api.b)
		return backend.GetBlockInternalTransactions(ctx, blockNr)
	}

	txHashes, ok := api.cacheDB.Stateless.GetBlockTxs(blockNum)
	if !ok {
		backend := ethapi.NewBlockChainAPI(api.b)
		return backend.GetBlockInternalTransactions(ctx, blockNr)
	}

	result := make(map[libcommon.Hash][]*types.InnerTx)

	for _, txHash := range txHashes {
		_, _, _, innerTxs, exists := api.cacheDB.Stateless.GetTxInfo(txHash)
		if !exists {
			backend := ethapi.NewBlockChainAPI(api.b)
			return backend.GetBlockInternalTransactions(ctx, blockNr)
		}
		result[txHash] = innerTxs
	}

	return result, nil
}
