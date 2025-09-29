package core

import (
	"github.com/ethereum/go-ethereum/core/types"
	realtimeTypes "github.com/ethereum/go-ethereum/realtime/types"
)

func (bc *BlockChain) SetRealtimeFinishChan(finishChan chan realtimeTypes.FinishedEntry) {
	bc.realtimeFinishChan = finishChan
}

func (bc *BlockChain) SetRealtimeBlockInfoChan(blockInfoChan chan *realtimeTypes.BlockInfo) {
	bc.realtimeBlockInfoChan = blockInfoChan
}

func (bc *BlockChain) RealtimeSendConfirmedBlock(block *types.Block, changeset *realtimeTypes.Changeset) {
	if bc.realtimeBlockInfoChan != nil {
		bc.realtimeBlockInfoChan <- &realtimeTypes.BlockInfo{
			Header:    block.Header(),
			TxCount:   int64(len(block.Transactions())),
			Hash:      block.Hash(),
			Changeset: changeset,
		}
	}
}

func (bc *BlockChain) RealtimeUpdateExecutionHeight(head *types.Block) {
	if bc.realtimeFinishChan != nil {
		bc.realtimeFinishChan <- realtimeTypes.FinishedEntry{
			Height: head.Number().Uint64(),
			Root:   head.Header().Root,
		}
	}
}
