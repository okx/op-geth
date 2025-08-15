package realtimeapi

import (
	"context"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/internal/ethapi"
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
	header, _, blockhash, ok := api.cacheDB.Stateless.GetHeader(receipt.BlockNumber.Uint64())
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
