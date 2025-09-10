package metrics

import (
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
)

// LogTag enumerates metric keys used by the block statistics logger.
type LogTag int

const (
	// Identifiers / tags
	BlockNumberTag LogTag = iota

	// Counters (per-block)
	TxCounter
	GasUsedCounter
	InvalidTxCounter
	GasOverTxCounter

	// Timings (per-block)
	TotalBuildMs

	// Execution/validation/write phases
	ExecuteMs
	ValidateMs
	CrossValidateMs
	WriteBlockMs
	BlockWriteAdjustedMs
	EvmExecPureMs
	ValidationPureMs

	// Trie/Snapshot pressure indicators
	TrieDiffNodes
	TrieBufNodes
	SnapDiffItems
	SnapBufItems

	// State-level timings (per-block)
	AccountReadMs
	StorageReadMs
	AccountUpdateMs
	StorageUpdateMs
	AccountHashMs
	TrieHashMs
	TrieUpdateMs
	AccountCommitMs
	StorageCommitMs
	SnapshotCommitMs
	TrieDBCommitMs
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
	ResetStatistics()
}

var (
	instance *statisticsInstance
	once     sync.Once
)

// GetLogStatistics returns a singleton Statistics collector.
func GetLogStatistics() Statistics {
	once.Do(func() {
		instance = &statisticsInstance{}
	})
	return instance
}

type statisticsInstance struct {
	mu        sync.RWMutex
	durations map[LogTag]time.Duration // per-block durations
	counters  map[LogTag]int64         // per-block counters
	tags      map[LogTag]string
}

func (l *statisticsInstance) CumulativeCounting(tag LogTag) {
	l.mu.Lock()
	if l.counters == nil {
		l.counters = make(map[LogTag]int64)
	}
	l.counters[tag]++
	l.mu.Unlock()
}

func (l *statisticsInstance) CumulativeValue(tag LogTag, value int64) {
	l.mu.Lock()
	if l.counters == nil {
		l.counters = make(map[LogTag]int64)
	}
	l.counters[tag] += value
	l.mu.Unlock()
}

func (l *statisticsInstance) CumulativeTiming(tag LogTag, duration time.Duration) {
	l.mu.Lock()
	if l.durations == nil {
		l.durations = make(map[LogTag]time.Duration)
	}
	l.durations[tag] += duration
	l.mu.Unlock()
}

func (l *statisticsInstance) CumulativeMicroTiming(tag LogTag, duration time.Duration) {
	l.CumulativeTiming(tag, duration)
}

func (l *statisticsInstance) SetTag(tag LogTag, value string) {
	l.mu.Lock()
	if l.tags == nil {
		l.tags = make(map[LogTag]string)
	}
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
	return l.counters[tag]
}

func (l *statisticsInstance) ResetStatistics() {
	l.mu.Lock()
	l.durations = make(map[LogTag]time.Duration)
	l.counters = make(map[LogTag]int64)
	l.tags = make(map[LogTag]string)
	l.mu.Unlock()
}

// SummaryCheckpoint computes per-block stats and logs a single-line summary.
func (l *statisticsInstance) SummaryCheckpoint() string {
	l.mu.RLock()
	block := l.counters[BlockNumberTag]
	blockDuration := l.durations[TotalBuildMs]

	// Current block values
	tx := l.counters[TxCounter]
	gasUsed := l.counters[GasUsedCounter]
	invalidTx := l.counters[InvalidTxCounter]
	gasOverTx := l.counters[GasOverTxCounter]

	exec := l.durations[ExecuteMs]
	validate := l.durations[ValidateMs]
	xvalidate := l.durations[CrossValidateMs]
	writeBlk := l.durations[WriteBlockMs]
	writeBlkAdj := l.durations[BlockWriteAdjustedMs]
	evmPure := l.durations[EvmExecPureMs]
	valPure := l.durations[ValidationPureMs]

	trieDiff := l.counters[TrieDiffNodes]
	trieBuf := l.counters[TrieBufNodes]
	snapDiff := l.counters[SnapDiffItems]
	snapBuf := l.counters[SnapBufItems]

	accRead := l.durations[AccountReadMs]
	storRead := l.durations[StorageReadMs]
	accUpdate := l.durations[AccountUpdateMs]
	storUpdate := l.durations[StorageUpdateMs]
	accHash := l.durations[AccountHashMs]
	trieHash := l.durations[TrieHashMs]
	trieUpd := l.durations[TrieUpdateMs]
	accCommit := l.durations[AccountCommitMs]
	storCommit := l.durations[StorageCommitMs]
	snapCommit := l.durations[SnapshotCommitMs]
	triedbCommit := l.durations[TrieDBCommitMs]
	l.mu.RUnlock()

	line := fmt.Sprintf(
		"Block<%d>, Txs<%d>, BlockTime<%s> { Exec { execute[%s], validate[%s], crossValidate[%s], evmExecPure[%s], validatePure[%s] }, Write { writeBlock[%s], writeAdjusted[%s] }, Trie<diff:%d, buf:%d>, Snap<diff:%d, buf:%d>, State { accRead[%s], storRead[%s], accUpdate[%s], storUpdate[%s], accHash[%s], trieHash[%s], trieUpdate[%s] }, Commits { accCommit[%s], storCommit[%s], snapCommit[%s], trieDBCommit[%s] } }, GasUsed<%d>, GasOverTx<%d>, InvalidTx<%d>",
		block,
		tx,
		common.PrettyDuration(blockDuration),
		common.PrettyDuration(exec), common.PrettyDuration(validate), common.PrettyDuration(xvalidate), common.PrettyDuration(evmPure), common.PrettyDuration(valPure),
		common.PrettyDuration(writeBlk), common.PrettyDuration(writeBlkAdj),
		trieDiff, trieBuf, snapDiff, snapBuf,
		common.PrettyDuration(accRead), common.PrettyDuration(storRead), common.PrettyDuration(accUpdate), common.PrettyDuration(storUpdate), common.PrettyDuration(accHash), common.PrettyDuration(trieHash), common.PrettyDuration(trieUpd),
		common.PrettyDuration(accCommit), common.PrettyDuration(storCommit), common.PrettyDuration(snapCommit), common.PrettyDuration(triedbCommit),
		gasUsed, gasOverTx, invalidTx,
	)
	log.Info(line)

	return line
}
