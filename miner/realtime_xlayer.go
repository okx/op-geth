package miner

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	realtimeTypes "github.com/ethereum/go-ethereum/realtime/types"
)

func (miner *Miner) RealtimeSendNewPendingBlock(statedb *state.StateDB, header *types.Header) {
	headerInfoChan := miner.backend.GetRealtimeBlockInfoChan()
	if headerInfoChan != nil {
		select {
		case headerInfoChan <- &realtimeTypes.BlockInfo{
			Header:      header,
			Withdrawals: nil,
			TxCount:     -1,
			Hash:        common.Hash{},
			Changeset:   statedb.GenerateChangeset(),
		}:
		default:
			log.Warn(fmt.Sprintf("[Realtime] Send blockInfo channel is full, dropping header info. header: %s", header.Hash().Hex()))
			miner.backend.SendRealtimeErrorTrigger(header.Number.Uint64())
		}
	}
}

func (miner *Miner) RealtimeSendTxInfo(txInfo state.TxInfo) {
	txInfoChan := miner.backend.GetRealtimeTxInfoChan()
	if txInfoChan != nil {
		select {
		case txInfoChan <- txInfo:
		default:
			log.Warn(fmt.Sprintf("[Realtime] Send txInfo channel is full, dropping tx info. txInfo: %s", txInfo.Tx.Hash().Hex()))
			miner.backend.SendRealtimeErrorTrigger(txInfo.BlockNumber)
		}
	}
}
