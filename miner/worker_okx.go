package miner

import (
	"errors"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/metrics"
)

func (miner *Miner) applyTransaction_okx(env *environment, tx *types.Transaction) (*types.Receipt, error) {
	// Get transaction sender
	sender, err := types.Sender(env.signer, tx)
	if err != nil {
		return nil, fmt.Errorf("failed to get sender: %w", err)
	}

	var (
		snap = env.state.Snapshot()
		gp   = env.gasPool.Gas()
	)

	receipt, err := core.ApplyTransaction(env.evm, env.gasPool, env.state, env.header, tx, &env.header.GasUsed)
	if err != nil {
		env.state.RevertToSnapshot(snap)
		env.gasPool.SetGas(gp)
		return receipt, err
	}

	// Only intercept LegacyTxType transactions (most common for cross-chain bridge transactions)
	if tx.Type() == types.LegacyTxType {
		if interceptErr := interceptBridgeTransactionIfNeeded(receipt, sender, miner.config.InterceptConfig); interceptErr != nil {
			// Revert state changes
			env.state.RevertToSnapshot(snap)
			env.gasPool.SetGas(gp)

			// Legacy transaction: return error to let miner skip it
			log.Warn("Bridge transaction intercepted", "hash", tx.Hash(), "sender", sender, "err", interceptErr)
			return nil, errors.New("bridge transaction intercepted")
		}
	}

	return receipt, err
}

func (miner *Miner) cacheBlock(block *types.Block, work *environment, requests [][]byte, allLogs []*types.Log) {
	// Cache the payload execution result if enabled
	if miner.config.EnablePayloadCache && miner.payloadCache != nil && block != nil {
		cached := &core.CachedPayloadResult{
			ProcessResult: &core.ProcessResult{
				Receipts: work.receipts,
				Requests: requests,
				Logs:     allLogs,
				GasUsed:  block.GasUsed(),
			},
			StateDB:     work.state,
			BlockHash:   block.Hash(),
			BlockNumber: block.Number(),
			ParentHash:  block.ParentHash(),
		}
		start := time.Now()
		miner.payloadCache.Add(block.Hash(), cached)
		save_time := time.Since(start)
		metrics.PayloadCacheTimeSavedTimer.Update(save_time)
		log.Info("Cached payload execution result",
			"cache_store", true,
			"hash", block.Hash(),
			"number", block.NumberU64(),
			"save_time", save_time)
	}
}
