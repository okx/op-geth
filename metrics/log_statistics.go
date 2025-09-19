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

	// Timings (per-block)
	TotalBuildMs

	// Execution/validation/write phases
	ExecuteMs
	ValidateMs
	CrossValidateMs
	WriteBlockMs
	EvmExecPureMs
	ValidationPureMs

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

	// Propose-stage timings
	ProposeTotalMs
	ProposePrepareMs
	ProposeSelectBuildMs
	ProposeExecTxMs
	ProposePragueMs
	ProposeAssembleMs
	ProposeForcedInclusionMs
	ProposeFetchTxMs
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
	ProposeCheckpoint() string
	ResetStatistics()
}

var (
	instance *statisticsInstance
	once     sync.Once
)

// GetLogStatistics returns a singleton Statistics collector.
func GetLogStatistics() Statistics {
	once.Do(func() {
		instance = &statisticsInstance{
			durations: make(map[LogTag]time.Duration),
			counters:  make(map[LogTag]int64),
			tags:      make(map[LogTag]string),
		}
	})
	return instance
}

// NewLogStatistics returns a fresh, independent statistics collector.
// Useful for measuring separate phases (e.g., propose stage) without
// interfering with the singleton used for block insertion.
func NewLogStatistics() Statistics {
	return &statisticsInstance{
		durations: make(map[LogTag]time.Duration),
		counters:  make(map[LogTag]int64),
		tags:      make(map[LogTag]string),
	}
}

type statisticsInstance struct {
	durations map[LogTag]time.Duration // per-block durations
	counters  map[LogTag]int64         // per-block counters
	tags      map[LogTag]string
}

func (l *statisticsInstance) CumulativeCounting(tag LogTag) {
	if l.counters == nil {
		l.counters = make(map[LogTag]int64)
	}
	l.counters[tag]++
}

func (l *statisticsInstance) CumulativeValue(tag LogTag, value int64) {
	if l.counters == nil {
		l.counters = make(map[LogTag]int64)
	}
	l.counters[tag] += value
}

func (l *statisticsInstance) CumulativeTiming(tag LogTag, duration time.Duration) {
	if l.durations == nil {
		l.durations = make(map[LogTag]time.Duration)
	}
	l.durations[tag] += duration
}

func (l *statisticsInstance) CumulativeMicroTiming(tag LogTag, duration time.Duration) {
	l.CumulativeTiming(tag, duration)
}

func (l *statisticsInstance) SetTag(tag LogTag, value string) {
	if l.tags == nil {
		l.tags = make(map[LogTag]string)
	}
	l.tags[tag] = value
}

func (l *statisticsInstance) GetTag(tag LogTag) string {
	return l.tags[tag]
}

func (l *statisticsInstance) GetStatistics(tag LogTag) int64 {
	return l.counters[tag]
}

func (l *statisticsInstance) ResetStatistics() {
	if l.durations != nil {
		clear(l.durations)
	}
	if l.counters != nil {
		clear(l.counters)
	}
	if l.tags != nil {
		clear(l.tags)
	}
}

// SummaryCheckpoint computes per-block stats and logs a single-line summary.
func (l *statisticsInstance) SummaryCheckpoint() string {
	block := l.counters[BlockNumberTag]
	blockDuration := l.durations[TotalBuildMs]

	// Current block values
	tx := l.counters[TxCounter]
	gasUsed := l.counters[GasUsedCounter]

	exec := l.durations[ExecuteMs]
	validate := l.durations[ValidateMs]
	xvalidate := l.durations[CrossValidateMs]
	writeBlk := l.durations[WriteBlockMs]
	evmPure := l.durations[EvmExecPureMs]
	valPure := l.durations[ValidationPureMs]
	accRead := l.durations[AccountReadMs]
	storRead := l.durations[StorageReadMs]
	accUpdate := l.durations[AccountUpdateMs]
	storUpdate := l.durations[StorageUpdateMs]
	accHash := l.durations[AccountHashMs]
	trieUpd := l.durations[TrieUpdateMs]
	accCommit := l.durations[AccountCommitMs]
	storCommit := l.durations[StorageCommitMs]
	snapCommit := l.durations[SnapshotCommitMs]
	triedbCommit := l.durations[TrieDBCommitMs]

	line := fmt.Sprintf(
		"Block<%d>, Txs<%d> GasUsed<%d>, BlockTime<%s> { Exec { execute[%s], validate[%s], crossValidate[%s], evmExecPure[%s], validatePure[%s] }, Write { writeBlock[%s] }, State { accRead[%s], storRead[%s], accUpdate[%s], storUpdate[%s], accHash[%s], trieUpdate[%s] }, Commits { accCommit[%s], storCommit[%s], snapCommit[%s], trieDBCommit[%s] } }",
		block,
		tx,
		gasUsed,
		common.PrettyDuration(blockDuration),
		common.PrettyDuration(exec), common.PrettyDuration(validate), common.PrettyDuration(xvalidate), common.PrettyDuration(evmPure), common.PrettyDuration(valPure),
		common.PrettyDuration(writeBlk),
		common.PrettyDuration(accRead), common.PrettyDuration(storRead), common.PrettyDuration(accUpdate), common.PrettyDuration(storUpdate), common.PrettyDuration(accHash), common.PrettyDuration(trieUpd),
		common.PrettyDuration(accCommit), common.PrettyDuration(storCommit), common.PrettyDuration(snapCommit), common.PrettyDuration(triedbCommit),
	)
	log.Info(line)

	return line
}

// ProposeCheckpoint prints a compact summary for the propose stage using
// the same tags, intended for an independently created collector
// (via NewLogStatistics) so it does not interfere with block insertion stats.
func (l *statisticsInstance) ProposeCheckpoint() string {
	total := l.durations[ProposeTotalMs]
	prep := l.durations[ProposePrepareMs]
	execTx := l.durations[ProposeExecTxMs]
	pragueMs := l.durations[ProposePragueMs]
	assemble := l.durations[ProposeAssembleMs]

	fetchTx := l.durations[ProposeFetchTxMs]

	block := l.counters[BlockNumberTag]
	tx := l.counters[TxCounter]
	gasUsed := l.counters[GasUsedCounter]

	accRead := l.durations[AccountReadMs]
	storRead := l.durations[StorageReadMs]
	accUpdate := l.durations[AccountUpdateMs]
	storUpdate := l.durations[StorageUpdateMs]
	accHash := l.durations[AccountHashMs]
	accCommit := l.durations[AccountCommitMs]
	storCommit := l.durations[StorageCommitMs]
	snapCommit := l.durations[SnapshotCommitMs]
	triedbCommit := l.durations[TrieDBCommitMs]

	line := fmt.Sprintf(
		"ProposeBlock<%d>, Txs<%d>, GasUsed<%d> BlockTime<[%s]> { Exec { Prepare[%s], FetchTx[%s], execute[%s], Prague[%s], assemble[%s] } , State { accRead[%s], storRead[%s], accUpdate[%s], storUpdate[%s], accHash[%s] }, Commits { accCommit[%s], storCommit[%s], snapCommit[%s], trieDBCommit[%s] } }",
		block,
		tx,
		gasUsed,
		common.PrettyDuration(total),
		common.PrettyDuration(prep),
		common.PrettyDuration(fetchTx),
		common.PrettyDuration(execTx),
		common.PrettyDuration(pragueMs),
		common.PrettyDuration(assemble),
		common.PrettyDuration(accRead),
		common.PrettyDuration(storRead),
		common.PrettyDuration(accUpdate),
		common.PrettyDuration(storUpdate),
		common.PrettyDuration(accHash),
		common.PrettyDuration(accCommit),
		common.PrettyDuration(storCommit),
		common.PrettyDuration(snapCommit),
		common.PrettyDuration(triedbCommit),
	)
	log.Info(line)
	return line
}
