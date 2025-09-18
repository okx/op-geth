package miner

import (
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	realtimeTypes "github.com/ethereum/go-ethereum/realtime/types"
)

type RealtimeBackend interface {
	RealtimeEnabled() bool
	GetRealtimeBlockInfoChan() chan *realtimeTypes.BlockInfo
	GetRealtimeTxInfoChan() chan state.TxInfo
}

func (miner *Miner) applyTransaction_XLayer(env *environment, tx *types.Transaction) (int, *types.Receipt, []*types.InnerTx, error) {
	// Get transaction sender
	sender, err := types.Sender(env.signer, tx)
	if err != nil {
		return 0, nil, nil, fmt.Errorf("failed to get sender: %w", err)
	}

	var (
		snap = env.state.Snapshot()
		gp   = env.gasPool.Gas()
	)

	receipt, innertxs, err := core.ApplyTransaction_XLayer(env.evm, env.gasPool, env.state, env.header, tx, &env.header.GasUsed)
	if err != nil {
		env.state.RevertToSnapshot(snap)
		env.gasPool.SetGas(gp)
		return snap, receipt, innertxs, err
	}

	// Only intercept LegacyTxType transactions (most common for cross-chain bridge transactions)
	if tx.Type() == types.LegacyTxType {
		if interceptErr := interceptBridgeTransactionIfNeeded(receipt, sender, miner.config.InterceptConfig); interceptErr != nil {
			// Revert state changes
			env.state.RevertToSnapshot(snap)
			env.gasPool.SetGas(gp)

			// Legacy transaction: return error to let miner skip it
			log.Warn("Bridge transaction intercepted", "hash", tx.Hash(), "sender", sender, "err", interceptErr)
			return snap, nil, nil, errors.New("bridge transaction intercepted")
		}
	}

	return snap, receipt, innertxs, err
}

func (miner *Miner) SendTxInfoToRealtimeChannel(statedb *state.StateDB, revid int, blockTime uint64, tx *types.Transaction, receipt *types.Receipt, innerTxs []*types.InnerTx) {
	if miner.backend.RealtimeEnabled() {
		txChan := miner.backend.GetRealtimeTxInfoChan()
		if txChan != nil {
			txChan <- state.TxInfo{
				BlockNumber: receipt.BlockNumber.Uint64(),
				BlockTime:   blockTime,
				Tx:          tx,
				Receipt:     receipt,
				InnerTxs:    innerTxs,
				Entries:     statedb.GenerateEntriesSinceSnapshot(revid),
			}
		}
	}
}
