package metrics

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/log"
)

// LogTag enumerates metric keys used by the block statistics logger.
type LogTag int

const (
	// Identifiers / tags
	BlockNumberTag LogTag = iota

	// Counters (cumulative, diffed per block)
	TxCounter
	GasUsedCounter
	InvalidTxCounter
	GasOverTxCounter

	// Timings in milliseconds (cumulative, diffed per block)
	TotalBuildMs
	PrepareWorkMs
	ForcedTxMs
	GetTxMs
	CommitTxMs
	FinalizeBlockMs

	// DB timings (ms)
	DBBatchWriteMs
	DBStateCommitMs
	DBTrieCommitMs
	DBInsertTotalMs
)

// Statistics exposes accumulation helpers and summary output.
type Statistics interface {
	CumulativeCounting(tag LogTag)
	CumulativeValue(tag LogTag, value int64)
	CumulativeTiming(tag LogTag, duration time.Duration)
	CumulativeMicroTiming(tag LogTag, duration time.Duration)
	SetTag(tag LogTag, value string)
	GetTag(tag LogTag) string
	GetStatistics(tag LogTag) int64
	SummaryCheckpoint() string
}

var (
	instance *statisticsInstance
	once     sync.Once
)

// GetLogStatistics returns a singleton Statistics collector.
func GetLogStatistics() Statistics {
	once.Do(func() {
		instance = &statisticsInstance{}
		instance.resetStatistics()
	})
	return instance
}

type statisticsInstance struct {
	mu            sync.RWMutex
	newBlockTime  time.Time
	statistics    map[LogTag]int64 // value is a counter or time (ms)
	statisticsOld map[LogTag]int64
	tags          map[LogTag]string
}

func (l *statisticsInstance) CumulativeCounting(tag LogTag) {
	l.mu.Lock()
	l.statistics[tag]++
	l.mu.Unlock()
}

func (l *statisticsInstance) CumulativeValue(tag LogTag, value int64) {
	l.mu.Lock()
	l.statistics[tag] += value
	l.mu.Unlock()
}

func (l *statisticsInstance) CumulativeTiming(tag LogTag, duration time.Duration) {
	l.mu.Lock()
	l.statistics[tag] += duration.Milliseconds()
	l.mu.Unlock()
}

func (l *statisticsInstance) CumulativeMicroTiming(tag LogTag, duration time.Duration) {
	l.mu.Lock()
	l.statistics[tag] += duration.Microseconds()
	l.mu.Unlock()
}

func (l *statisticsInstance) SetTag(tag LogTag, value string) {
	l.mu.Lock()
	l.tags[tag] = value
	l.mu.Unlock()
}

func (l *statisticsInstance) GetTag(tag LogTag) string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.tags[tag]
}

func (l *statisticsInstance) GetStatistics(tag LogTag) int64 {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.statistics[tag]
}

func (l *statisticsInstance) resetStatistics() {
	l.mu.Lock()
	l.newBlockTime = time.Now()
	l.statistics = make(map[LogTag]int64)
	l.statisticsOld = make(map[LogTag]int64)
	l.tags = make(map[LogTag]string)
	l.mu.Unlock()
}

// SummaryCheckpoint computes the per-block delta and logs a single-line summary.
func (l *statisticsInstance) SummaryCheckpoint() string {
	l.mu.RLock()
	block := l.tags[BlockNumberTag]

	// Deltas
	blockDuration := time.Since(l.newBlockTime).Milliseconds()
	tx := l.statistics[TxCounter] - l.statisticsOld[TxCounter]
	gasUsed := l.statistics[GasUsedCounter] - l.statisticsOld[GasUsedCounter]
	invalidTx := l.statistics[InvalidTxCounter] - l.statisticsOld[InvalidTxCounter]
	gasOverTx := l.statistics[GasOverTxCounter] - l.statisticsOld[GasOverTxCounter]

	// total build time (not currently printed separately, but kept for potential use)
	_ = l.statistics[TotalBuildMs] - l.statisticsOld[TotalBuildMs]
	prepare := l.statistics[PrepareWorkMs] - l.statisticsOld[PrepareWorkMs]
	forced := l.statistics[ForcedTxMs] - l.statisticsOld[ForcedTxMs]
	gettx := l.statistics[GetTxMs] - l.statisticsOld[GetTxMs]
	commit := l.statistics[CommitTxMs] - l.statisticsOld[CommitTxMs]
	finalize := l.statistics[FinalizeBlockMs] - l.statisticsOld[FinalizeBlockMs]

	dbBatch := l.statistics[DBBatchWriteMs] - l.statisticsOld[DBBatchWriteMs]
	dbState := l.statistics[DBStateCommitMs] - l.statisticsOld[DBStateCommitMs]
	dbTrie := l.statistics[DBTrieCommitMs] - l.statisticsOld[DBTrieCommitMs]
	dbTotal := l.statistics[DBInsertTotalMs] - l.statisticsOld[DBInsertTotalMs]
	l.mu.RUnlock()

	// Compose subsections conditionally
	var processParts []string
	if gettx > 0 {
		processParts = append(processParts, fmt.Sprintf("getTx[%dms]", gettx))
	}
	if commit > 0 {
		processParts = append(processParts, fmt.Sprintf("commitTx[%dms]", commit))
	}
	processSection := ""
	if len(processParts) > 0 {
		processTotal := prepare + forced + gettx + commit
		processSection = fmt.Sprintf("ProcessTxTiming<%dms> { %s }, ", processTotal, strings.Join(processParts, ", "))
	}

	var dbParts []string
	if dbBatch > 0 {
		dbParts = append(dbParts, fmt.Sprintf("blockBatchWrite[%dms]", dbBatch))
	}
	if dbState > 0 {
		dbParts = append(dbParts, fmt.Sprintf("stateCommit[%dms]", dbState))
	}
	if dbTrie > 0 {
		dbParts = append(dbParts, fmt.Sprintf("trieCommit[%dms]", dbTrie))
	}
	if dbTotal > 0 {
		dbParts = append(dbParts, fmt.Sprintf("insertTotal[%dms]", dbTotal))
	}
	dbSection := ""
	if len(dbParts) > 0 {
		dbSection = fmt.Sprintf("DB { %s }, ", strings.Join(dbParts, ", "))
	}

	// Final line
	line := fmt.Sprintf(
		"Block<%s>, Txs<%d>, TotalDuration-block<%dms> { %sFinalizeBlockTiming<%dms>, %s}, GasUsed<%d>",
		block, tx, blockDuration, processSection, finalize, dbSection, gasUsed,
	)
	// Optional tails
	if gasOverTx > 0 {
		line += fmt.Sprintf(", GasOverTx<%d>", gasOverTx)
	}
	if invalidTx > 0 {
		line += fmt.Sprintf(", InvalidTx<%d>", invalidTx)
	}

	log.Info(line)

	// roll window
	l.mu.Lock()
	for k, v := range l.statistics {
		l.statisticsOld[k] = v
	}
	l.newBlockTime = time.Now()
	l.mu.Unlock()
	return line
}
