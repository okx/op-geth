package monitor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/log"
)

const (
	// DepositTxType is the transaction type for deposit transactions (system transactions)
	DepositTxType = 0x7E
)

// TraceLogEntry represents a single trace log entry
type TraceLogEntry struct {
	Timestamp       int64  `json:"timestamp"`
	Hash            string `json:"hash"`
	ServiceName     string `json:"serviceName"`
	ProcessID       uint64 `json:"processId"`
	ProcessWord     string `json:"processWord"`
	Phase           string `json:"phase"`
	Step            string `json:"step"`
	BlockHeight     uint64 `json:"blockHeight"`
	BlockHash       string `json:"blockHash"`
	BlockTime       uint64 `json:"blockTime"`
	TransactionType int8   `json:"transactionType"`
	Status          string `json:"status"`
	Error           string `json:"error,omitempty"`
	GasUsed         uint64 `json:"gasUsed,omitempty"`
	GasPrice        string `json:"gasPrice,omitempty"`
	From            string `json:"from,omitempty"`
	To              string `json:"to,omitempty"`
	Value           string `json:"value,omitempty"`
	Nonce           uint64 `json:"nonce,omitempty"`
	Duration        int64  `json:"duration,omitempty"`
}

// TraceLogger handles transaction trace logging
type TraceLogger struct {
	enabled bool
	logPath string
	file    *os.File
	mutex   sync.Mutex
}

var (
	globalLogger *TraceLogger
	once         sync.Once
)

func getPhaseFromServiceName(serviceName string) string {
	switch serviceName {
	case ServiceNameRPC:
		return "rpc"
	case ServiceNameTxPool:
		return "txpool"
	case ServiceNameMiner:
		return "miner"
	case ServiceNameState:
		return "state"
	case ServiceNameBlockchain:
		return "blockchain"
	default:
		return "unknown"
	}
}

func getStepFromProcessWord(processWord string) string {
	if len(processWord) > 11 && processWord[:11] == "op_geth_" {
		return processWord[11:]
	}
	return processWord
}

// InitTraceLogger initializes the global trace logger
func InitTraceLogger(enabled bool, logPath string) {
	once.Do(func() {
		globalLogger = &TraceLogger{
			enabled: enabled,
			logPath: logPath,
		}

		if enabled {
			if err := globalLogger.initFile(); err != nil {
				log.Error("Failed to initialize trace logger", "error", err)
				globalLogger.enabled = false
			}
		}
	})
}

// initFile initializes the log file
func (tl *TraceLogger) initFile() error {
	if tl.logPath == "" {
		return fmt.Errorf("log path is empty")
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(tl.logPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %v", err)
	}

	// Open or create log file
	file, err := os.OpenFile(tl.logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %v", err)
	}

	tl.file = file
	return nil
}

// LogTrace logs a transaction trace entry
func LogTrace(txHash, serviceName string, processID uint64, processWord string,
	blockHeight uint64, blockHash string, blockTime uint64, txType int8,
	status string, errMsg string, gasUsed uint64, gasPrice, from, to, value string, nonce uint64) {
	if globalLogger == nil || !globalLogger.enabled {
		return
	}

	// Filter out deposit transactions (system transactions)
	if txType == DepositTxType {
		return
	}

	timestamp := time.Now().UnixNano() / 1000000 // milliseconds
	step := getStepFromProcessWord(processWord)

	entry := TraceLogEntry{
		Timestamp:       timestamp,
		Hash:            txHash,
		ServiceName:     serviceName,
		ProcessID:       processID,
		ProcessWord:     processWord,
		Phase:           getPhaseFromServiceName(serviceName),
		Step:            step,
		BlockHeight:     blockHeight,
		BlockHash:       blockHash,
		BlockTime:       blockTime,
		TransactionType: txType,
		Status:          status,
		Error:           errMsg,
		GasUsed:         gasUsed,
		GasPrice:        gasPrice,
		From:            from,
		To:              to,
		Value:           value,
		Nonce:           nonce,
	}

	globalLogger.logEntry(entry)
}

// logEntry writes a log entry to file
func (tl *TraceLogger) logEntry(entry TraceLogEntry) {
	tl.mutex.Lock()
	defer tl.mutex.Unlock()

	if tl.file == nil {
		return
	}

	// Convert to JSON
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}

	// Write to file with newline
	tl.file.WriteString(string(data) + "\n")
	tl.file.Sync()
}

// Close closes the trace logger
func CloseTraceLogger() {
	if globalLogger != nil && globalLogger.file != nil {
		globalLogger.mutex.Lock()
		defer globalLogger.mutex.Unlock()
		globalLogger.file.Close()
	}
}

// LogTransactionStart logs the start of transaction processing
func LogTransactionStart(txHash, serviceName string, processID uint64, processWord string,
	blockHeight uint64, txType int8, from, to, value string, nonce uint64) {
	LogTrace(txHash, serviceName, processID, processWord, blockHeight, "", 0, txType,
		"start", "", 0, "", from, to, value, nonce)
}

// LogTransactionEnd logs the end of transaction processing
func LogTransactionEnd(txHash, serviceName string, processID uint64, processWord string,
	blockHeight uint64, blockHash string, blockTime uint64, txType int8,
	status string, errMsg string, gasUsed uint64) {
	LogTrace(txHash, serviceName, processID, processWord, blockHeight, blockHash, blockTime,
		txType, status, errMsg, gasUsed, "", "", "", "", 0)
}

// LogTransactionProgress logs transaction progress
func LogTransactionProgress(txHash, serviceName string, processID uint64, processWord string,
	blockHeight uint64, txType int8, status string, gasUsed uint64) {
	LogTrace(txHash, serviceName, processID, processWord, blockHeight, "", 0, txType,
		status, "", gasUsed, "", "", "", "", 0)
}
