package core

import (
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/metrics"
)

func logStatistic(block *types.Block, statedb *state.StateDB, start time.Time, cacheHit bool, ptime time.Duration, vtime time.Duration, triehash time.Duration, trieUpdate time.Duration, xvtime time.Duration, wstart time.Time, proctime time.Duration) {
	// Export to a fresh LogStatistics instance (no global singleton)
	ls := metrics.NewLogStatistics()
	// Record cache hit status
	if cacheHit {
		ls.CumulativeValue(metrics.PayloadCacheHitCounter, 1)
	}
	ls.CumulativeValue(metrics.BlockNumberTag, int64(block.NumberU64()))
	ls.CumulativeValue(metrics.TxCounter, int64(block.Transactions().Len()))
	ls.CumulativeValue(metrics.GasUsedCounter, int64(block.GasUsed()))
	ls.CumulativeTiming(metrics.AccountReadMs, statedb.AccountReads)
	ls.CumulativeTiming(metrics.StorageReadMs, statedb.StorageReads)
	ls.CumulativeTiming(metrics.AccountUpdateMs, statedb.AccountUpdates)
	ls.CumulativeTiming(metrics.StorageUpdateMs, statedb.StorageUpdates)
	ls.CumulativeTiming(metrics.AccountHashMs, statedb.AccountHashes)
	ls.CumulativeTiming(metrics.TrieUpdateMs, statedb.AccountUpdates+statedb.StorageUpdates)
	ls.CumulativeTiming(metrics.EvmExecPureMs, ptime-(statedb.AccountReads+statedb.StorageReads))
	ls.CumulativeTiming(metrics.ValidationPureMs, vtime-(triehash+trieUpdate))
	ls.CumulativeTiming(metrics.CrossValidateMs, xvtime)
	ls.CumulativeTiming(metrics.WriteBlockMs, time.Since(wstart))
	ls.CumulativeTiming(metrics.AccountCommitMs, statedb.AccountCommits)
	ls.CumulativeTiming(metrics.StorageCommitMs, statedb.StorageCommits)
	ls.CumulativeTiming(metrics.SnapshotCommitMs, statedb.SnapshotCommits)
	ls.CumulativeTiming(metrics.TrieDBCommitMs, statedb.TrieDBCommits)
	ls.CumulativeTiming(metrics.TotalBuildMs, time.Since(start))
	ls.CumulativeTiming(metrics.ExecuteMs, proctime)
	ls.CumulativeTiming(metrics.ValidateMs, vtime-(triehash+trieUpdate))
	// Try merge propose stats snapshot if exists (and add propose time into final block time)
	if pstat, ok := metrics.GlobalStatsStore.GetAndDelete(block.Hash()); ok {
		_ = ls.CombinedSummary(pstat)
	} else {
		ls.CombinedSummary(nil)
	}
}

func (bc *BlockChain) fetchCachedBlock(hash common.Hash) (*ProcessResult, *state.StateDB, bool) {
	if bc.payloadCache != nil {
		if cached, ok := bc.payloadCache.Get(hash); ok {
			// Use cached result
			res := cached.ProcessResult
			copyStart := time.Now()
			cachedState := CopyStateDB(cached.StateDB)
			copyTime := time.Since(copyStart)
			cacheHit := true

			// Update metrics
			metrics.PayloadCacheCopyTimeTimer.Update(copyTime)
			log.Info("Using cached payload result",
				"cache", true,
				"hash", hash,
				"copy_time", copyTime)

			return res, cachedState, cacheHit
		}
	}
	return nil, nil, false
}
