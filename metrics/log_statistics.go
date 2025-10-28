package metrics

import (
	"fmt"
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
	ProposeExecTxMs
	ProposePragueMs
	ProposeAssembleMs
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
	GetDuration(tag LogTag) time.Duration
	ResetStatistics()
	CombinedSummary(Statistics) string
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

func (l *statisticsInstance) GetDuration(tag LogTag) time.Duration {
	return l.durations[tag]
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

// CombinedSummary prints a combined line that shows Propose (from snapshot)
// and Insert (from this instance) sections, while BlockTime is TotalBuildMs (already combined).
func (l *statisticsInstance) CombinedSummary(pstat Statistics) string {
	block := l.counters[BlockNumberTag]
	blockDuration := l.durations[TotalBuildMs]

	// Insert (this instance)
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

	// Propose (access propose stats directly, no copy)
	var pTotal, pPrepare, pExec, pPrague, pAssemble time.Duration
	var pAccRead, pStorRead, pAccUpdate, pStorUpdate, pAccHash time.Duration
	if pstat != nil {
		pTotal = pstat.GetDuration(ProposeTotalMs)
		pPrepare = pstat.GetDuration(ProposePrepareMs)
		pExec = pstat.GetDuration(ProposeExecTxMs)
		pPrague = pstat.GetDuration(ProposePragueMs)
		pAssemble = pstat.GetDuration(ProposeAssembleMs)
		pAccRead = pstat.GetDuration(AccountReadMs)
		pStorRead = pstat.GetDuration(StorageReadMs)
		pAccUpdate = pstat.GetDuration(AccountUpdateMs)
		pStorUpdate = pstat.GetDuration(StorageUpdateMs)
		pAccHash = pstat.GetDuration(AccountHashMs)
	}

	line := fmt.Sprintf(
		"Block<%d>, Txs<%d> GasUsed<%d>, BlockTime<%s> { Mine[%s] { Prepare[%s], execute[%s], Prague[%s], assemble[%s] , State { accRead[%s], storRead[%s], accUpdate[%s], storUpdate[%s], accHash[%s] } }, Insert[%s] { execute[%s], validate[%s], crossValidate[%s], evmExecPure[%s], validatePure[%s] , Write { writeBlock[%s] }, State { accRead[%s], storRead[%s], accUpdate[%s], storUpdate[%s], accHash[%s], trieUpdate[%s] }, Commits { accCommit[%s], storCommit[%s], snapCommit[%s], trieDBCommit[%s] } }",
		block,
		tx,
		gasUsed,
		common.PrettyDuration(blockDuration+pTotal),
		common.PrettyDuration(pTotal),
		common.PrettyDuration(pPrepare),
		common.PrettyDuration(pExec),
		common.PrettyDuration(pPrague),
		common.PrettyDuration(pAssemble),
		common.PrettyDuration(pAccRead), common.PrettyDuration(pStorRead), common.PrettyDuration(pAccUpdate), common.PrettyDuration(pStorUpdate), common.PrettyDuration(pAccHash),
		common.PrettyDuration(blockDuration),
		common.PrettyDuration(exec), common.PrettyDuration(validate), common.PrettyDuration(xvalidate), common.PrettyDuration(evmPure), common.PrettyDuration(valPure),
		common.PrettyDuration(writeBlk),
		common.PrettyDuration(accRead), common.PrettyDuration(storRead), common.PrettyDuration(accUpdate), common.PrettyDuration(storUpdate), common.PrettyDuration(accHash), common.PrettyDuration(trieUpd),
		common.PrettyDuration(accCommit), common.PrettyDuration(storCommit), common.PrettyDuration(snapCommit), common.PrettyDuration(triedbCommit),
	)
	log.Info(line)
	return line
}
