package monitor

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/log"
)

const (
	// DepositTxType is the transaction type for deposit transactions (system transactions)
	DepositTxType = 0x7E

	// Number of log entries to write before forcing a flush
	FLUSH_INTERVAL_WRITES = 100

	// Time interval between flushes (in seconds)
	FLUSH_INTERVAL_SECONDS = 1
)

// TraceLogger handles transaction trace logging
type TraceLogger struct {
	enabled       bool
	logPath       string
	file          *os.File
	mutex         sync.Mutex
	writeCount    uint64
	lastFlushTime time.Time
}

var (
	globalLogger *TraceLogger
	once         sync.Once
)

// InitTraceLogger initializes the global trace logger
// When enabled is true, logPath should be provided (default path is set if not specified).
// All trace logs are written to file only, not to console.
func InitTraceLogger(enabled bool, logPath string) {
	once.Do(func() {
		globalLogger = &TraceLogger{
			enabled:       enabled,
			logPath:       logPath,
			lastFlushTime: time.Now(),
		}

		if enabled {
			// logPath should not be empty at this point (default path should have been set),
			// but check for safety
			if logPath == "" {
				log.Error("Transaction tracing enabled but log path is empty. Tracing disabled.")
				globalLogger.enabled = false
				return
			}
			if err := globalLogger.initFile(); err != nil {
				log.Error("Failed to initialize trace logger", "error", err)
				globalLogger.enabled = false
			}
		}
	})
}

// initFile initializes the log file
// This matches reth's path handling: if path ends with directory separator or has no extension,
// trace.log will be appended
func (tl *TraceLogger) initFile() error {
	if tl.logPath == "" {
		return fmt.Errorf("log path is empty")
	}

	// Determine the actual file path based on reth's logic
	filePath := tl.logPath

	// Check if path ends with directory separator
	if strings.HasSuffix(filePath, string(filepath.Separator)) || strings.HasSuffix(filePath, "/") || strings.HasSuffix(filePath, "\\") {
		filePath = filepath.Join(filePath, "trace.log")
	} else if filepath.Ext(filePath) == "" {
		// If path has no extension and file doesn't exist, append trace.log
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			filePath = filepath.Join(filePath, "trace.log")
		}
		// If file exists, use it as is
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %v", err)
	}

	// Open or create log file
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %v", err)
	}

	tl.file = file
	return nil
}

// escapeCSV escapes CSV special characters
func escapeCSV(s string) string {
	if s == "" {
		return ""
	}
	if strings.Contains(s, ",") || strings.Contains(s, "\"") || strings.Contains(s, "\n") {
		return fmt.Sprintf("\"%s\"", strings.ReplaceAll(s, "\"", "\"\""))
	}
	return s
}

// formatCSVLine formats a CSV line with 23 fields matching reth implementation
// Format: chain,trace,status,serviceName,business,client,chainld,process,processWord,index,innerIndex,currentTime,referld,contractAddress,blockHeight,blockHash,blockTime,depositConfirmHeight,tokenID,mevSupplier,businessHash,transactionType,extJson
func formatCSVLine(
	traceHash string,
	processID TransactionProcessId,
	currentTime uint64,
	blockHash string,
	blockNumber uint64,
) string {
	chain := CHAIN_NAME
	trace := strings.ToLower(traceHash)
	status := ""
	serviceName := processID.ServiceName()
	business := BUSINESS_NAME
	client := ""
	chainld := CHAIN_ID
	processStr := strconv.FormatUint(uint64(processID), 10)
	processWordStr := processID.String()
	index := ""
	innerIndex := ""
	currentTimeStr := strconv.FormatUint(currentTime, 10)
	referld := ""
	contractAddress := ""
	blockHeight := ""
	if blockNumber > 0 {
		blockHeight = strconv.FormatUint(blockNumber, 10)
	}
	blockHashStr := strings.ToLower(blockHash)
	blockTime := ""
	depositConfirmHeight := ""
	tokenID := ""
	mevSupplier := ""
	businessHash := ""
	transactionType := ""
	extJson := ""

	return fmt.Sprintf(
		"%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s",
		escapeCSV(chain), escapeCSV(trace), escapeCSV(status), escapeCSV(serviceName),
		escapeCSV(business), escapeCSV(client), escapeCSV(chainld), escapeCSV(processStr),
		escapeCSV(processWordStr), escapeCSV(index), escapeCSV(innerIndex), escapeCSV(currentTimeStr),
		escapeCSV(referld), escapeCSV(contractAddress), escapeCSV(blockHeight), escapeCSV(blockHashStr),
		escapeCSV(blockTime), escapeCSV(depositConfirmHeight), escapeCSV(tokenID), escapeCSV(mevSupplier),
		escapeCSV(businessHash), escapeCSV(transactionType), escapeCSV(extJson),
	)
}

// writeToFile writes a CSV line to the trace file with periodic flush
func (tl *TraceLogger) writeToFile(csvLine string) {
	tl.mutex.Lock()
	defer tl.mutex.Unlock()

	if tl.file == nil {
		return
	}

	if _, err := tl.file.WriteString(csvLine + "\n"); err != nil {
		log.Warn("Failed to write to transaction trace file", "error", err)
		return
	}

	count := atomic.AddUint64(&tl.writeCount, 1)
	now := time.Now()
	timeSinceFlush := now.Sub(tl.lastFlushTime)

	shouldFlush := count%FLUSH_INTERVAL_WRITES == 0 || timeSinceFlush.Seconds() >= FLUSH_INTERVAL_SECONDS

	if shouldFlush {
		tl.lastFlushTime = now
		if err := tl.file.Sync(); err != nil {
			log.Warn("Failed to flush transaction trace file", "error", err)
		}
	}
}

// LogTransaction logs a transaction event at current time point
// This matches the reth implementation: log_transaction(tx_hash, process_id, block_number)
func LogTransaction(txHash string, processID TransactionProcessId, blockNumber uint64) {
	if globalLogger == nil || !globalLogger.enabled {
		return
	}

	// Get current time in milliseconds since Unix epoch
	timestampMs := uint64(time.Now().UnixNano() / 1000000)

	// Format hash to match reth format (0x prefix, lowercase)
	traceHash := strings.ToLower(txHash)
	if !strings.HasPrefix(traceHash, "0x") {
		traceHash = "0x" + traceHash
	}

	csvLine := formatCSVLine(traceHash, processID, timestampMs, "", blockNumber)
	globalLogger.writeToFile(csvLine)
}

// LogBlock logs a block event at current time point
// This matches the reth implementation: log_block(block_hash, block_number, process_id)
func LogBlock(blockHash string, blockNumber uint64, processID TransactionProcessId) {
	if globalLogger == nil || !globalLogger.enabled {
		return
	}

	// Get current time in milliseconds since Unix epoch
	timestampMs := uint64(time.Now().UnixNano() / 1000000)

	// Format hash to match reth format (0x prefix, lowercase)
	traceHash := strings.ToLower(blockHash)
	if !strings.HasPrefix(traceHash, "0x") {
		traceHash = "0x" + traceHash
	}

	csvLine := formatCSVLine(traceHash, processID, timestampMs, traceHash, blockNumber)
	globalLogger.writeToFile(csvLine)
}

// LogBlockWithTimestamp logs a block event with a specific timestamp
// This matches the reth implementation: log_block_with_timestamp(block_hash, block_number, process_id, timestamp_ms)
func LogBlockWithTimestamp(blockHash string, blockNumber uint64, processID TransactionProcessId, timestampMs uint64) {
	if globalLogger == nil || !globalLogger.enabled {
		return
	}

	// Format hash to match reth format (0x prefix, lowercase)
	traceHash := strings.ToLower(blockHash)
	if !strings.HasPrefix(traceHash, "0x") {
		traceHash = "0x" + traceHash
	}

	csvLine := formatCSVLine(traceHash, processID, timestampMs, traceHash, blockNumber)
	globalLogger.writeToFile(csvLine)
}

// Flush forces a flush of the trace file
func Flush() {
	if globalLogger != nil && globalLogger.enabled {
		globalLogger.mutex.Lock()
		defer globalLogger.mutex.Unlock()

		if globalLogger.file != nil {
			if err := globalLogger.file.Sync(); err != nil {
				log.Warn("Failed to flush transaction trace file", "error", err)
			}
		}
	}
}

// CloseTraceLogger closes the trace logger
func CloseTraceLogger() {
	if globalLogger != nil && globalLogger.file != nil {
		globalLogger.mutex.Lock()
		defer globalLogger.mutex.Unlock()
		globalLogger.file.Close()
	}
}
