package core

import (
	"time"

	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/metrics"
)

func logStatistic(block *types.Block, statedb *state.StateDB, start time.Time, ptime time.Duration, vtime time.Duration, triehash time.Duration, trieUpdate time.Duration, xvtime time.Duration, wstart time.Time, proctime time.Duration) {
	// Export to a fresh LogStatistics instance (no global singleton)
	ls := metrics.NewLogStatistics()
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
