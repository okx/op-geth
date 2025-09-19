package core

import realtimeTypes "github.com/ethereum/go-ethereum/realtime/types"

func (bc *BlockChain) SetRealtimeFinishChan(finishChan chan realtimeTypes.FinishedEntry) {
	bc.realtimeFinishChan = finishChan
}
