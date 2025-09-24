package miner

import (
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
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

func (miner *Miner) applyTransaction_XLayer(env *environment, tx *types.Transaction) (*types.Receipt, []*types.InnerTx, *state.Entries, error) {
	// Get transaction sender
	sender, err := types.Sender(env.signer, tx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get sender: %w", err)
	}

	var (
		snap = env.state.Snapshot()
		gp   = env.gasPool.Gas()
	)

	// Do not finalize statedb to generate changeset
	receipt, innertxs, entries, err := core.ApplyTransaction_XLayer(env.evm, env.gasPool, env.state, env.header, tx, &env.header.GasUsed, false)
	if err != nil {
		env.state.RevertToSnapshot(snap)
		env.gasPool.SetGas(gp)
		return receipt, innertxs, entries, err
	}

	// Only intercept LegacyTxType transactions (most common for cross-chain bridge transactions)
	if tx.Type() == types.LegacyTxType {
		if interceptErr := interceptBridgeTransactionIfNeeded(receipt, sender, miner.config.InterceptConfig); interceptErr != nil {
			// Revert state changes
			env.state.RevertToSnapshot(snap)
			env.gasPool.SetGas(gp)

			// Legacy transaction: return error to let miner skip it
			log.Warn("Bridge transaction intercepted", "hash", tx.Hash(), "sender", sender, "err", interceptErr)
			return nil, nil, nil, errors.New("bridge transaction intercepted")
		}
	}

	return receipt, innertxs, entries, err
}

func (miner *Miner) RealtimeSendNewPendingBlock(statedb *state.StateDB, header *types.Header) {
	if miner.backend.RealtimeEnabled() {
		blockInfoChan := miner.backend.GetRealtimeBlockInfoChan()
		if blockInfoChan != nil {
			blockInfoChan <- &realtimeTypes.BlockInfo{
				Header:    header,
				TxCount:   -1,
				Hash:      common.Hash{},
				Changeset: statedb.GenerateChangeset(),
			}
		}
	}
}

func (miner *Miner) RealtimeSendTxInfo(blockTime uint64, tx *types.Transaction, receipt *types.Receipt, innerTxs []*types.InnerTx, entries *state.Entries) {
	if miner.backend.RealtimeEnabled() {
		txInfoChan := miner.backend.GetRealtimeTxInfoChan()
		if txInfoChan != nil {
			txInfoChan <- state.TxInfo{
				BlockNumber: receipt.BlockNumber.Uint64(),
				BlockTime:   blockTime,
				Tx:          tx,
				Receipt:     receipt,
				InnerTxs:    innerTxs,
				Entries:     *entries,
			}
		}
	}
}
