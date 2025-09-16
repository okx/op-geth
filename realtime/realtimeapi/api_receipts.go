package realtimeapi

import (
	"context"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/internal/ethapi"
	"github.com/ethereum/go-ethereum/rpc"
)

// GetTransactionReceipt implements the realtime eth_getTransactionReceipt.
// Returns the receipt of a transaction given the transaction's hash.
func (api *RealtimeAPIImpl) GetTransactionReceipt(ctx context.Context, hash common.Hash) (map[string]interface{}, error) {
	if api.cacheDB == nil || !api.cacheDB.ReadyFlag.Load() {
		backend := ethapi.NewTransactionAPI(api.b, nil)
		return backend.GetTransactionReceipt(ctx, hash)
	}

	txn, receipt, _, _, ok := api.cacheDB.Stateless.GetTxInfo(hash)
	if !ok {
		backend := ethapi.NewTransactionAPI(api.b, nil)
		return backend.GetTransactionReceipt(ctx, hash)
	}
	header, _, blockhash, ok := api.cacheDB.Stateless.GetBlockInfo(receipt.BlockNumber.Uint64())
	if !ok {
		backend := ethapi.NewTransactionAPI(api.b, nil)
		return backend.GetTransactionReceipt(ctx, hash)
	}
	if blockhash != EmptyBlockHash {
		receipt.BlockHash = blockhash
		for _, log := range receipt.Logs {
			log.BlockHash = blockhash
		}
	}

	signer := types.MakeSigner(api.b.ChainConfig(), header.Number, header.Time)
	return ethapi.MarshalReceipt(receipt, header.Number.Uint64(), signer, txn, api.b.ChainConfig()), nil
}

// GetInternalTransactions implements the realtime eth_getInternalTransactions.
// Returns the internal transactions of a transaction given the transaction's hash.
func (api *RealtimeAPIImpl) GetInternalTransactions(ctx context.Context, hash common.Hash) ([]*types.InnerTx, error) {
	if api.cacheDB == nil || !api.cacheDB.ReadyFlag.Load() {
		backend := ethapi.NewTransactionAPI(api.b, nil)
		return backend.GetInternalTransactions(ctx, hash)
	}

	_, _, _, innerTxs, ok := api.cacheDB.Stateless.GetTxInfo(hash)
	if !ok {
		backend := ethapi.NewTransactionAPI(api.b, nil)
		return backend.GetInternalTransactions(ctx, hash)
	}

	return innerTxs, nil
}

func (api *RealtimeAPIImpl) GetBlockReceipts(ctx context.Context, number rpc.BlockNumberOrHash) ([]map[string]interface{}, error) {
	if api.cacheDB == nil || !api.cacheDB.ReadyFlag.Load() {
		backend := ethapi.NewBlockChainAPI(api.b)
		return backend.GetBlockReceipts(ctx, number)
	}

	blockNum, _, _, err := api.getBlockNumberOrHash(number)
	if err != nil {
		backend := ethapi.NewBlockChainAPI(api.b)
		return backend.GetBlockReceipts(ctx, number)
	}

	header, _, blockhash, ok := api.cacheDB.Stateless.GetBlockInfo(blockNum)
	if !ok {
		backend := ethapi.NewBlockChainAPI(api.b)
		return backend.GetBlockReceipts(ctx, number)
	}

	txHashes, ok := api.cacheDB.Stateless.GetBlockTxs(blockNum)
	if !ok {
		backend := ethapi.NewBlockChainAPI(api.b)
		return backend.GetBlockReceipts(ctx, number)
	}

	signer := types.MakeSigner(api.b.ChainConfig(), header.Number, header.Time)
	result := make([]map[string]interface{}, 0, len(txHashes))
	for _, txHash := range txHashes {
		txn, receipt, _, _, exists := api.cacheDB.Stateless.GetTxInfo(txHash)
		if !exists {
			backend := ethapi.NewBlockChainAPI(api.b)
			return backend.GetBlockReceipts(ctx, number)
		}
		if blockhash != EmptyBlockHash {
			receipt.BlockHash = blockhash
			for _, log := range receipt.Logs {
				log.BlockHash = blockhash
			}
		}
		result = append(result, ethapi.MarshalReceipt(receipt, header.Number.Uint64(), signer, txn, api.b.ChainConfig()))
	}

	return result, nil
}
