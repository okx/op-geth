package monitor

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestTraceLogger(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test_trace.log")

	// Initialize trace logger
	InitTraceLogger(true, logPath)
	defer CloseTraceLogger()

	// Test logging a transaction (matching reth implementation)
	txHash := "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	blockNumber := uint64(100)

	LogTransaction(txHash, RpcReceiveTxEnd, blockNumber)
	LogTransaction(txHash, SeqReceiveTxEnd, blockNumber)
	LogTransaction(txHash, SeqTxExecutionEnd, blockNumber)

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
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) < 3 {
		t.Fatalf("Expected at least 3 log entries, got %d", len(lines))
	}

	// Verify CSV format (23 fields)
	for i, line := range lines {
		fields := strings.Split(line, ",")
		if len(fields) < 23 {
			t.Errorf("Line %d: Expected 23 CSV fields, got %d", i+1, len(fields))
		}
		// Verify transaction hash is present
		if !strings.Contains(strings.ToLower(line), strings.ToLower(txHash)) {
			t.Errorf("Line %d: Transaction hash not found", i+1)
		}
	}

	t.Logf("Successfully logged %d entries to %s", len(lines), logPath)
}

func TestTraceLoggerDisabled(t *testing.T) {
	// Test with disabled logger
	InitTraceLogger(false, "")

	// This should not create any files or cause errors
	LogTransaction("0xtest", RpcReceiveTxEnd, 100)

	// Should not panic or create files
	t.Log("Disabled logger test passed")
}

func TestLogBlock(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "test_block_trace.log")

	// Close previous logger if any
	CloseTraceLogger()

	// Reset once to allow re-initialization for testing
	// Note: In production, InitTraceLogger should only be called once
	// For testing, we need to reset the global state
	globalLogger = nil
	once = sync.Once{}

	// Initialize trace logger
	InitTraceLogger(true, logPath)
	defer CloseTraceLogger()

	blockHash := "0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	blockNumber := uint64(200)

	LogBlock(blockHash, blockNumber, RpcBlockInsertEnd)

	// Flush to ensure data is written
	Flush()

	// Verify log file was created and contains data
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	if len(content) == 0 {
		t.Fatal("Log file is empty")
	}

	// Verify block hash is present
	contentStr := strings.ToLower(string(content))
	if !strings.Contains(contentStr, strings.ToLower(blockHash)) {
		t.Error("Block hash not found in log")
	}

	t.Log("Block logging test passed")
}

func TestTransactionProcessId(t *testing.T) {
	// Test that all process IDs are properly defined
	testCases := []struct {
		id   TransactionProcessId
		name string
	}{
		{RpcReceiveTxEnd, "xlayer_rpc_receive_tx"},
		{SeqReceiveTxEnd, "xlayer_seq_receive_tx"},
		{SeqBlockBuildStart, "xlayer_seq_begin_block"},
		{SeqTxExecutionEnd, "xlayer_seq_package_tx"},
		{SeqBlockBuildEnd, "xlayer_seq_end_block"},
		{SeqBlockSendStart, "xlayer_seq_ds_sent"},
		{RpcBlockReceiveEnd, "xlayer_rpc_receive_block"},
		{RpcBlockInsertEnd, "xlayer_rpc_finish_block"},
	}

	for _, tc := range testCases {
		if tc.id.String() != tc.name {
			t.Errorf("Process ID %d: Expected name %s, got %s", tc.id, tc.name, tc.id.String())
		}
		serviceName := tc.id.ServiceName()
		if serviceName != RPC_SERVICE_NAME && serviceName != SEQ_SERVICE_NAME {
			t.Errorf("Process ID %d: Invalid service name %s", tc.id, serviceName)
		}
	}

	t.Logf("All %d process IDs are properly defined", len(testCases))
}
