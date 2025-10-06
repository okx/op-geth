package types

import (
	"path/filepath"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

const DefaultBlockTxsListSize = 2000

type TxInfo struct {
	BlockNumber uint64
	Tx          *types.Transaction
	Receipt     *types.Receipt
	InnerTxs    []*types.InnerTx
	Changeset   *Changeset
}

type BlockTx struct {
	TxHash  common.Hash
	TxIndex uint
}

func NewOrderedBlockTxsList() *OrderedList[BlockTx] {
	return NewOrderedList(DefaultBlockTxsListSize, func(a, b BlockTx) int {
		return int(a.TxIndex) - int(b.TxIndex)
	})
}

type TxInfoMap struct {
	txInfos  map[common.Hash]TxInfo
	blockTxs map[uint64]*OrderedList[BlockTx]
	mu       sync.RWMutex
}

func NewTxInfoMap(blockCacheSize int, txCacheSize int) *TxInfoMap {
	return &TxInfoMap{
		txInfos:  make(map[common.Hash]TxInfo, txCacheSize),
		blockTxs: make(map[uint64]*OrderedList[BlockTx], blockCacheSize),
	}
}

func (rm *TxInfoMap) Put(blockNumber uint64, txHash common.Hash, tx *types.Transaction, receipt *types.Receipt, innerTxs []*types.InnerTx) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	txInfo := TxInfo{
		BlockNumber: blockNumber,
		Tx:          tx,
		Receipt:     receipt,
		InnerTxs:    innerTxs,
	}

	rm.txInfos[txHash] = txInfo
	if _, exists := rm.blockTxs[blockNumber]; !exists {
		rm.blockTxs[blockNumber] = NewOrderedBlockTxsList()
	}
	rm.blockTxs[blockNumber].Add(BlockTx{
		TxHash:  txHash,
		TxIndex: receipt.TransactionIndex,
	})
	rm.blockTxs[blockNumber].Sort()
}

func (rm *TxInfoMap) Delete(blockNumber uint64) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	txHashes, exists := rm.blockTxs[blockNumber]
	if !exists {
		return
	}
	for _, blockTx := range txHashes.Items() {
		delete(rm.txInfos, blockTx.TxHash)
	}
	delete(rm.blockTxs, blockNumber)
}

func (rm *TxInfoMap) GetTx(txHash common.Hash) (*types.Transaction, *types.Receipt, uint64, []*types.InnerTx, bool) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	txInfo, exists := rm.txInfos[txHash]
	return txInfo.Tx, txInfo.Receipt, txInfo.BlockNumber, txInfo.InnerTxs, exists
}

func (rm *TxInfoMap) GetBlockTxs(blockNumber uint64) []common.Hash {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	hashes := make([]common.Hash, 0)
	hashSet, exists := rm.blockTxs[blockNumber]
	if !exists {
		return hashes
	}

	for _, blockTx := range hashSet.Items() {
		hashes = append(hashes, blockTx.TxHash)
	}
	return hashes
}

func (rm *TxInfoMap) Clear() {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	for k := range rm.txInfos {
		delete(rm.txInfos, k)
	}

	for k := range rm.blockTxs {
		delete(rm.blockTxs, k)
	}
}

// -------------- Debug operations --------------
func (rm *TxInfoMap) DebugDumpToFile(cacheDumpPath string) error {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	return WriteToJSON(filepath.Join(cacheDumpPath, "tx_info_map.json"), rm.txInfos)
}
