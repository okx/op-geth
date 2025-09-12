package types

import (
	"path/filepath"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type BlockInfoMap struct {
	blockInfos        map[uint64]*BlockInfo
	blockHashToHeight map[common.Hash]uint64
	mu                sync.RWMutex
}

func NewBlockInfoMap(size int) *BlockInfoMap {
	return &BlockInfoMap{
		blockInfos:        make(map[uint64]*BlockInfo, size),
		blockHashToHeight: make(map[common.Hash]uint64, size),
	}
}

func (bm *BlockInfoMap) Get(blockNum uint64) (*types.Header, int64, common.Hash, bool) {
	bm.mu.RLock()
	defer bm.mu.RUnlock()
	blockInfo, exists := bm.blockInfos[blockNum]
	if exists {
		return blockInfo.Header, blockInfo.TxCount, blockInfo.Hash, true
	}
	return nil, 0, common.Hash{}, exists
}

func (bm *BlockInfoMap) GetBlockNumberByHash(blockHash common.Hash) (uint64, bool) {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	blockNum, exists := bm.blockHashToHeight[blockHash]
	return blockNum, exists
}

func (bm *BlockInfoMap) PutNewBlockInfo(blockNum uint64, blockInfo *BlockInfo) {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	bm.blockInfos[blockNum] = blockInfo
}

func (bm *BlockInfoMap) PutConfirmedBlockInfo(blockNum uint64, blockInfo *BlockInfo) {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	bm.blockInfos[blockNum] = blockInfo
	bm.blockHashToHeight[blockInfo.Hash] = blockNum
}

func (bm *BlockInfoMap) Delete(blockNum uint64) {
	_, _, blockhash, exists := bm.Get(blockNum)
	bm.mu.Lock()
	defer bm.mu.Unlock()
	if exists {
		delete(bm.blockHashToHeight, blockhash)
		delete(bm.blockInfos, blockNum)
	}
}

func (bm *BlockInfoMap) Clear() {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	for k := range bm.blockInfos {
		delete(bm.blockInfos, k)
	}
	for k := range bm.blockHashToHeight {
		delete(bm.blockHashToHeight, k)
	}
}

// -------------- Debug operations --------------
func (bm *BlockInfoMap) DebugDumpToFile(cacheDumpPath string) error {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	return WriteToJSON(filepath.Join(cacheDumpPath, "block_info_map.json"), bm.blockInfos)
}
