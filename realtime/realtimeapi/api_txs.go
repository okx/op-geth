package realtimeapi

import (
	"context"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/internal/ethapi"
)

// GetTransactionByHash implements realtime_getTransactionByHash.
// Returns information about a transaction given the transaction's hash.
func (api *RealtimeAPIImpl) GetTransactionByHash(ctx context.Context, txnHash common.Hash) (interface{}, error) {
	if api.cacheDB == nil || !api.cacheDB.ReadyFlag.Load() {
		backend := ethapi.NewTransactionAPI(api.b, nil)
		return backend.GetTransactionByHash(ctx, txnHash)
	}

	txn, receipt, blockNum, _, ok := api.cacheDB.Stateless.GetTxInfo(txnHash)
	if !ok {
		backend := ethapi.NewTransactionAPI(api.b, nil)
		return backend.GetTransactionByHash(ctx, txnHash)
	}

	header, _, blockhash, ok := api.cacheDB.Stateless.GetBlockInfo(blockNum)
	if !ok {
		backend := ethapi.NewTransactionAPI(api.b, nil)
		return backend.GetTransactionByHash(ctx, txnHash)
	}

	return newRPCTransaction_realtime(txn, blockhash, blockNum, header.Time, uint64(receipt.TransactionIndex), header.BaseFee, api.b.ChainConfig(), receipt), nil
}

// GetRawTransactionByHash implements the realtime eth_getRawTransactionByHash.
// Returns the bytes of the transaction for the given hash.
func (api *RealtimeAPIImpl) GetRawTransactionByHash(ctx context.Context, hash common.Hash) (hexutil.Bytes, error) {
	if api.cacheDB == nil || !api.cacheDB.ReadyFlag.Load() {
		backend := ethapi.NewTransactionAPI(api.b, nil)
		return backend.GetRawTransactionByHash(ctx, hash)
	}

	txn, _, _, _, ok := api.cacheDB.Stateless.GetTxInfo(hash)
	if !ok || txn == nil {
		backend := ethapi.NewTransactionAPI(api.b, nil)
		return backend.GetRawTransactionByHash(ctx, hash)
	}

	return txn.MarshalBinary()
}
