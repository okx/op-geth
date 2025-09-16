package monitor

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTraceLogger(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test_trace.log")

	// Initialize trace logger
	InitTraceLogger(true, logPath)
	defer CloseTraceLogger()

	// Test logging a transaction start
	txHash := "0x1234567890abcdef"
	LogTransactionStart(
		txHash,
		ServiceNameRPC,
		StepRPCReceiveTx.ID,
		StepRPCReceiveTx.Key,
		100,
		int8(0), // Legacy transaction
		"0xfrom",
		"0xto",
		"1000000000000000000", // 1 ETH
		42,
	)

	// Test logging transaction progress
	LogTransactionProgress(
		txHash,
		ServiceNameRPC,
		StepRPCSendTx.ID,
		StepRPCSendTx.Key,
		100,
		int8(0),
		"processing",
		21000,
	)

	// Test logging transaction end
	LogTransactionEnd(
		txHash,
		ServiceNameRPC,
		StepRPCSendTx.ID,
		StepRPCSendTx.Key,
		100,
		"0xblockhash",
		uint64(time.Now().Unix()),
		int8(0),
		"success",
		"",
		21000,
	)

	// Verify log file was created and contains data
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Fatalf("Log file was not created: %v", err)
	}

	// Read and verify log content
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	if len(content) == 0 {
		t.Fatal("Log file is empty")
	}

	// Check that we have multiple log entries (should have 3 entries)
	lines := 0
	for _, b := range content {
		if b == '\n' {
			lines++
		}
	}

	if lines < 3 {
		t.Fatalf("Expected at least 3 log entries, got %d", lines)
	}

	t.Logf("Successfully logged %d entries to %s", lines, logPath)
}

func TestTraceLoggerDisabled(t *testing.T) {
	// Test with disabled logger
	InitTraceLogger(false, "")

	// This should not create any files or cause errors
	LogTransactionStart(
		"0xtest",
		ServiceNameRPC,
		StepRPCReceiveTx.ID,
		StepRPCReceiveTx.Key,
		100,
		int8(0),
		"0xfrom",
		"0xto",
		"1000000000000000000",
		42,
	)

	// Should not panic or create files
	t.Log("Disabled logger test passed")
}

func TestProcessSteps(t *testing.T) {
	// Test that all process steps are properly defined
	steps := []ProcessStep{
		StepRPCReceiveTx,
		StepRPCSendTx,
		StepTxPoolAdd,
		StepMinerSelectTx,
		StepMinerExecuteTx,
		StepStateProcessTx,
		StepStateApplyTx,
		StepBlockchainInsert,
	}

	for _, step := range steps {
		if step.ID == 0 {
			t.Errorf("Process step %s has invalid ID", step.Key)
		}
		if step.Key == "" {
			t.Errorf("Process step with ID %d has empty key", step.ID)
		}
	}

	t.Logf("All %d process steps are properly defined", len(steps))
}
