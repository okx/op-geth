package cache

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/misc/eip4844"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	realtimeTypes "github.com/ethereum/go-ethereum/realtime/types"
)

type StatelessCache struct {
	config       *params.ChainConfig
	blockInfoMap *realtimeTypes.BlockInfoMap
	txInfoMap    *realtimeTypes.TxInfoMap
}

func NewStatelessCache(config *params.ChainConfig, blockCacheSize int, txCacheSize int) *StatelessCache {
	return &StatelessCache{
		config:       config,
		blockInfoMap: realtimeTypes.NewBlockInfoMap(blockCacheSize),
		txInfoMap:    realtimeTypes.NewTxInfoMap(blockCacheSize, txCacheSize),
	}
}

func (cache *StatelessCache) Clear() {
	cache.blockInfoMap.Clear()
	cache.txInfoMap.Clear()
}

// -------------- Read operations --------------
func (cache *StatelessCache) GetBlockInfo(blockNum uint64) (*types.Header, *types.Withdrawals, int64, common.Hash, bool) {
	return cache.blockInfoMap.Get(blockNum)
}

func (cache *StatelessCache) GetBlockInfoByHash(blockHash common.Hash) (*types.Header, *types.Withdrawals, int64, common.Hash, bool) {
	blockNum, exists := cache.blockInfoMap.GetBlockNumberByHash(blockHash)
	if !exists {
		return nil, nil, 0, common.Hash{}, false
	}
	return cache.blockInfoMap.Get(blockNum)
}

func (cache *StatelessCache) GetBlockNumberByHash(blockHash common.Hash) (uint64, bool) {
	return cache.blockInfoMap.GetBlockNumberByHash(blockHash)
}

func (cache *StatelessCache) GetTxInfo(txHash common.Hash) (*types.Transaction, *types.Receipt, uint64, []*types.InnerTx, bool) {
	return cache.txInfoMap.GetTx(txHash)
}

func (cache *StatelessCache) GetBlockTxs(blockNum uint64) ([]common.Hash, bool) {
	if _, _, _, _, ok := cache.blockInfoMap.Get(blockNum); !ok {
		return nil, false
	}
	return cache.txInfoMap.GetBlockTxs(blockNum), true
}

// -------------- Write operations --------------
func (cache *StatelessCache) PutNewBlockInfo(blockNum uint64, blockInfo *realtimeTypes.BlockInfo) {
	log.Debug(fmt.Sprintf("Putting new block info for block %d\n", blockNum))
	cache.blockInfoMap.PutNewBlockInfo(blockNum, blockInfo)
}

func (cache *StatelessCache) PutConfirmedBlockInfo(blockNum uint64, blockInfo *realtimeTypes.BlockInfo) {
	log.Debug(fmt.Sprintf("Putting confirmed block info for block %d\n", blockNum))
	cache.blockInfoMap.PutConfirmedBlockInfo(blockNum, blockInfo)
}

func (cache *StatelessCache) PutTxInfo(blockNum uint64, txHash common.Hash, tx *types.Transaction, receipt *types.Receipt, innerTxs []*types.InnerTx) {
	log.Debug(fmt.Sprintf("Putting tx info for block %d, tx hash %s\n", blockNum, txHash.Hex()))
	cache.txInfoMap.Put(blockNum, txHash, tx, receipt, innerTxs)
}

func (cache *StatelessCache) DeleteBlock(blockNum uint64) {
	cache.blockInfoMap.Delete(blockNum)
	cache.txInfoMap.Delete(blockNum)
}

// UpdateConfirmedBlock updates the stateless cache with the confirmed block info.
// Note that this function should be called only after all tx and block data of
// that height has been updated.
func (cache *StatelessCache) UpdateConfirmedBlock(ctx context.Context, blockNum uint64) error {
	header, _, _, blockhash, ok := cache.blockInfoMap.Get(blockNum)
	if !ok {
		return fmt.Errorf("block header %s not found in cache", blockhash.Hex())
	}
	txHashes, ok := cache.GetBlockTxs(blockNum)
	if !ok {
		return fmt.Errorf("block tx %s not found in cache", blockhash.Hex())
	}
	txs := make(types.Transactions, 0, len(txHashes))
	receipts := make(types.Receipts, 0, len(txHashes))
	for _, txHash := range txHashes {
		tx, receipt, _, _, ok := cache.GetTxInfo(txHash)
		if !ok {
			return fmt.Errorf("receipt %s not found in cache", txHash.Hex())
		}
		txs = append(txs, tx)
		receipts = append(receipts, receipt)
	}
	var blobGasPrice *big.Int
	if header.ExcessBlobGas != nil {
		blobGasPrice = eip4844.CalcBlobFee(cache.config, header)
	}
	if err := receipts.DeriveFields(cache.config, blockhash, header.Number.Uint64(), header.Time, header.BaseFee, blobGasPrice, txs); err != nil {
		return fmt.Errorf("failed to derive receipt fields for block %d. Error: %v", blockNum, err)
	}
	return nil
}

// -------------- ReceiptGetter implementation --------------
func (cache *StatelessCache) GetReceipts(ctx context.Context, hash common.Hash) (types.Receipts, error) {
	blockNum, ok := cache.GetBlockNumberByHash(hash)
	if !ok {
		return nil, fmt.Errorf("block header %s not found in cache", hash.Hex())
	}
	txHashes, ok := cache.GetBlockTxs(blockNum)
	if !ok {
		return nil, fmt.Errorf("block tx %s not found in cache", hash.Hex())
	}
	receipts := make(types.Receipts, 0, len(txHashes))
	for _, txHash := range txHashes {
		_, receipt, _, _, ok := cache.GetTxInfo(txHash)
		if !ok {
			return nil, fmt.Errorf("receipt %s not found in cache", txHash.Hex())
		}
		receipts = append(receipts, receipt)
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
