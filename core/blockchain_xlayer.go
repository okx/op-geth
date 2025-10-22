package core

import (
	"github.com/ethereum/go-ethereum/core/types"
	realtimeTypes "github.com/ethereum/go-ethereum/realtime/types"
)

func (bc *BlockChain) SetRealtimeFinishChan(finishChan chan realtimeTypes.FinishedEntry) {
	bc.realtimeFinishChan = finishChan
}

func (bc *BlockChain) RealtimeUpdateExecutionHeight(head *types.Block) {
	if bc.realtimeFinishChan != nil {
		select {
		case bc.realtimeFinishChan <- realtimeTypes.FinishedEntry{
			Height: head.Number().Uint64(),
			Root:   head.Header().Root,
		}:
		default: // realtime consumer could be killed, dispose message
		}
	}
}
