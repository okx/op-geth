package cache

import (
	"context"
	"fmt"

	libcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	realtimeTypes "github.com/ethereum/go-ethereum/realtime/types"
)

type StatelessCache struct {
	blockInfoMap *realtimeTypes.BlockInfoMap
	txInfoMap    *realtimeTypes.TxInfoMap
}

func NewStatelessCache(blockCacheSize int, txCacheSize int) *StatelessCache {
	return &StatelessCache{
		blockInfoMap: realtimeTypes.NewBlockInfoMap(blockCacheSize),
		txInfoMap:    realtimeTypes.NewTxInfoMap(blockCacheSize, txCacheSize),
	}
}

func (cache *StatelessCache) Clear() {
	cache.blockInfoMap.Clear()
	cache.txInfoMap.Clear()
}

// -------------- Read operations --------------
func (cache *StatelessCache) GetHeader(blockNum uint64) (*types.Header, int64, libcommon.Hash, bool) {
	return cache.blockInfoMap.Get(blockNum)
}

func (cache *StatelessCache) GetHeaderByHash(blockHash libcommon.Hash) (*types.Header, int64, libcommon.Hash, bool) {
	blockNum, exists := cache.blockInfoMap.GetBlockNumberByHash(blockHash)
	if !exists {
		return nil, 0, libcommon.Hash{}, false
	}
	return cache.blockInfoMap.Get(blockNum)
}

func (cache *StatelessCache) GetBlockNumberByHash(blockHash libcommon.Hash) (uint64, bool) {
	return cache.blockInfoMap.GetBlockNumberByHash(blockHash)
}

func (cache *StatelessCache) GetTxInfo(txHash libcommon.Hash) (*types.Transaction, *types.Receipt, uint64, []*types.InnerTx, bool) {
	return cache.txInfoMap.GetTx(txHash)
}

func (cache *StatelessCache) GetBlockTxs(blockNum uint64) ([]libcommon.Hash, bool) {
	return cache.txInfoMap.GetBlockTxs(blockNum)
}

// -------------- Write operations --------------
func (cache *StatelessCache) PutHeader(blockNum uint64, header *types.Header, prevBlockInfo *realtimeTypes.BlockInfo) {
	cache.blockInfoMap.PutHeader(blockNum, header, prevBlockInfo)
}

func (cache *StatelessCache) PutTxInfo(blockNum uint64, txHash libcommon.Hash, tx *types.Transaction, receipt *types.Receipt, innerTxs []*types.InnerTx) {
	cache.txInfoMap.Put(blockNum, txHash, tx, receipt, innerTxs)
}

func (cache *StatelessCache) DeleteBlock(blockNum uint64) {
	cache.blockInfoMap.Delete(blockNum)
	cache.txInfoMap.Delete(blockNum)
}

// -------------- ReceiptGetter implementation --------------
func (cache *StatelessCache) GetReceipts(ctx context.Context, hash libcommon.Hash) (types.Receipts, error) {
	blockNum, ok := cache.GetBlockNumberByHash(hash)
	if !ok {
		return nil, fmt.Errorf("block header %s not found in cache", hash.Hex())
	}
	txHashes, ok := cache.GetBlockTxs(blockNum)
	if !ok {
		return nil, fmt.Errorf("block tx %s not found in cache", hash.Hex())
	}
	receipts := make(types.Receipts, len(txHashes))
	for i, txHash := range txHashes {
		_, receipt, _, _, ok := cache.GetTxInfo(txHash)
		if !ok {
			return nil, fmt.Errorf("receipt %s not found in cache", txHash.Hex())
		}
		receipts[i] = receipt
	}
	return receipts, nil
}

// -------------- Debug operations --------------
func (cache *StatelessCache) DebugDumpToFile(cacheDumpPath string) error {
	err := cache.blockInfoMap.DebugDumpToFile(cacheDumpPath)
	if err != nil {
		return err
	}
	return cache.txInfoMap.DebugDumpToFile(cacheDumpPath)
}
