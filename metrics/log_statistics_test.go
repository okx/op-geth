package metrics

import (
	"regexp"
	"testing"
	"time"
)

func TestStatisticsTimingAccumulation(t *testing.T) {
	s := NewLogStatistics()

	// Simulate timings
	s.CumulativeTiming(ExecuteMs, 10*time.Millisecond)
	s.CumulativeTiming(ExecuteMs, 15*time.Millisecond)
	s.CumulativeTiming(ValidateMs, 7*time.Millisecond)
	s.CumulativeTiming(TotalBuildMs, 50*time.Millisecond)

	// Simulate counters
	s.CumulativeValue(TxCounter, 10)
	s.CumulativeValue(GasUsedCounter, 1000)
	s.CumulativeValue(BlockNumberTag, 123)

	if got := s.GetDuration(ExecuteMs); got != 25*time.Millisecond {
		t.Fatalf("ExecuteMs expected 25ms, got %v", got)
	}
	if got := s.GetDuration(ValidateMs); got != 7*time.Millisecond {
		t.Fatalf("ValidateMs expected 7ms, got %v", got)
	}
	if got := s.GetDuration(TotalBuildMs); got != 50*time.Millisecond {
		t.Fatalf("TotalBuildMs expected 50ms, got %v", got)
	}
	if got := s.GetStatistics(TxCounter); got != 10 {
		t.Fatalf("TxCounter expected 10, got %d", got)
	}
	if got := s.GetStatistics(GasUsedCounter); got != 1000 {
		t.Fatalf("GasUsedCounter expected 1000, got %d", got)
	}
	if got := s.GetStatistics(BlockNumberTag); got != 123 {
		t.Fatalf("BlockNumberTag expected 123, got %d", got)
	}
}

func TestCombinedSummaryFormatting(t *testing.T) {
	// Insert-side stats
	insert := NewLogStatistics()
	insert.CumulativeValue(BlockNumberTag, 101)
	insert.CumulativeValue(TxCounter, 2)
	insert.CumulativeValue(GasUsedCounter, 21000)
	insert.CumulativeTiming(TotalBuildMs, 20*time.Millisecond)
	insert.CumulativeTiming(ExecuteMs, 5*time.Millisecond)
	insert.CumulativeTiming(ValidateMs, 3*time.Millisecond)
	insert.CumulativeTiming(CrossValidateMs, 1*time.Millisecond)
	insert.CumulativeTiming(WriteBlockMs, 2*time.Millisecond)
	insert.CumulativeTiming(EvmExecPureMs, 1*time.Millisecond)
	insert.CumulativeTiming(ValidationPureMs, 1*time.Millisecond)

	// Propose-side stats
	propose := NewLogStatistics()
	propose.CumulativeTiming(ProposeTotalMs, 30*time.Millisecond)
	propose.CumulativeTiming(ProposePrepareMs, 4*time.Millisecond)
	propose.CumulativeTiming(ProposeExecTxMs, 20*time.Millisecond)
	propose.CumulativeTiming(ProposePragueMs, 3*time.Millisecond)
	propose.CumulativeTiming(ProposeAssembleMs, 2*time.Millisecond)
	propose.CumulativeTiming(AccountReadMs, 1*time.Millisecond)
	propose.CumulativeTiming(StorageReadMs, 1*time.Millisecond)
	propose.CumulativeTiming(AccountUpdateMs, 1*time.Millisecond)
	propose.CumulativeTiming(StorageUpdateMs, 1*time.Millisecond)
	propose.CumulativeTiming(AccountHashMs, 1*time.Millisecond)

	line := insert.CombinedSummary(propose)

	// Basic checks: contains Block<101>, Txs<2>, GasUsed<21000>
	if matched, _ := regexp.MatchString(`Block<101>`, line); !matched {
		t.Fatalf("expected Block<101> in line: %s", line)
	}
	if matched, _ := regexp.MatchString(`Txs<2>`, line); !matched {
		t.Fatalf("expected Txs<2> in line: %s", line)
	}
	if matched, _ := regexp.MatchString(`GasUsed<21000>`, line); !matched {
		t.Fatalf("expected GasUsed<21000> in line: %s", line)
	}

	// BlockTime should reflect insert+propose in pretty string; just ensure presence of both sections
	if matched, _ := regexp.MatchString(`Propose\[`, line); !matched {
		t.Fatalf("expected Propose section in line: %s", line)
	}
	if matched, _ := regexp.MatchString(`Insert\[`, line); !matched {
		t.Fatalf("expected Insert section in line: %s", line)
	}
}
