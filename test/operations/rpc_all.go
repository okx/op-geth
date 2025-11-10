package operations

import (
	"context"
	"fmt"
	"math/big"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"
)

// DebugTraceBlockByHash traces all transactions in the block given by block hash
func DebugTraceBlockByHash(blockHash common.Hash) (interface{}, error) {
	var result interface{}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	err := clientRPC.Client().CallContext(ctx, &result, "debug_traceBlockByHash", blockHash, map[string]interface{}{})
	if err != nil {
		return nil, err
	}

	return result, nil
}

// DebugTraceBlockByNumber traces a block with specified options given by block number
func DebugTraceBlockByNumber(blockNumber uint64) (interface{}, error) {
	blockNumberHex := fmt.Sprintf("0x%x", blockNumber)
	var result interface{}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	err := clientRPC.Client().CallContext(ctx, &result, "debug_traceBlockByNumber", blockNumberHex, map[string]interface{}{})
	if err != nil {
		return nil, err
	}

	return result, nil
}

// DebugTraceBatchByNumber traces all transactions in a batch given by batch number
func DebugTraceBatchByNumber(batchNumber uint64) (interface{}, error) {
	batchNumberHex := fmt.Sprintf("0x%x", batchNumber)
	var result interface{}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	err := clientRPC.Client().CallContext(ctx, &result, "debug_traceBatchByNumber", batchNumberHex, map[string]interface{}{})
	if err != nil {
		return nil, err
	}

	return result, nil
}

// DebugTraceTransaction traces a transaction with specified options
func DebugTraceTransaction(txHash common.Hash) (interface{}, error) {
	var result interface{}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	err := clientRPC.Client().CallContext(ctx, &result, "debug_traceTransaction", txHash, map[string]interface{}{})
	if err != nil {
		return nil, err
	}

	return result, nil
}

// ZKEVMGetExitRootTable returns the exit root table
// func ZKEVMGetExitRootTable() (interface{}, error) {
// 	response, err := client.JSONRPCCall(DefaultL2NetworkURL, "zkevm_getExitRootTable")
// 	if err != nil {
// 		return nil, err
// 	}
// 	if response.Error != nil {
// 		return nil, fmt.Errorf("%d - %s", response.Error.Code, response.Error.Message)
// 	}

// 	var result interface{}
// 	err = json.Unmarshal(response.Result, &result)
// 	if err != nil {
// 		return nil, err
// 	}

// 	return result, nil
// }

// EthChainID returns the chain ID of the current network
func EthChainID() (uint64, error) {
	var result string
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := clientRPC.Client().CallContext(ctx, &result, "eth_chainId")
	if err != nil {
		return 0, err
	}

	return transHexStringToUint64(result)
}

// EthEstimateGas estimates the gas required for a transaction
func EthEstimateGas(from, to common.Address, gas string, gasPrice string, value string, data string) (uint64, error) {
	txParams := map[string]interface{}{
		"from":     from,
		"to":       to,
		"gas":      gas,
		"gasPrice": gasPrice,
		"value":    value,
		"data":     data,
	}

	var result string
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := clientRPC.Client().CallContext(ctx, &result, "eth_estimateGas", txParams)
	if err != nil {
		return 0, err
	}

	return transHexStringToUint64(result)
}

// EthGetBalance returns the balance of an account
func EthGetBalance(address common.Address, block string) (*big.Int, error) {
	var result string
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := clientRPC.Client().CallContext(ctx, &result, "eth_getBalance", address, block)
	if err != nil {
		return nil, err
	}

	return transHexStringToBigInt(result)
}

// EthGetBlockByHash returns information about a block by hash
func EthGetBlockByHash(blockHash common.Hash, fullTx bool) (interface{}, error) {
	var result interface{}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := clientRPC.Client().CallContext(ctx, &result, "eth_getBlockByHash", blockHash, fullTx)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// EthGetBlockByNumber returns information about a block by number
func EthGetBlockByNumber(blockNumber string, fullTx bool) (interface{}, error) {
	var result interface{}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := clientRPC.Client().CallContext(ctx, &result, "eth_getBlockByNumber", blockNumber, fullTx)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// EthGetBlockTransactionCountByHash returns the number of transactions in a block by hash
func EthGetBlockTransactionCountByHash(blockHash common.Hash) (uint64, error) {
	var result string
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := clientRPC.Client().CallContext(ctx, &result, "eth_getBlockTransactionCountByHash", blockHash)
	if err != nil {
		return 0, err
	}

	return transHexStringToUint64(result)
}

// EthGetBlockTransactionCountByNumber returns the number of transactions in a block by number
func EthGetBlockTransactionCountByNumber(blockNumber string) (uint64, error) {
	var result string
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := clientRPC.Client().CallContext(ctx, &result, "eth_getBlockTransactionCountByNumber", blockNumber)
	if err != nil {
		return 0, err
	}

	return transHexStringToUint64(result)
}

// EthGetCode returns the code at a given address
func EthGetCode(address common.Address, block string) (string, error) {
	if clientRPC == nil {
		return "", fmt.Errorf("RPC client not initialized")
	}

	var result string
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := clientRPC.Client().CallContext(ctx, &result, "eth_getCode", address, block)
	if err != nil {
		return "", err
	}

	return result, nil
}

// EthGetTransactionCount returns the number of transactions sent from an address
func EthGetTransactionCount(address common.Address, block string) (uint64, error) {
	var result string
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := clientRPC.Client().CallContext(ctx, &result, "eth_getTransactionCount", address, block)
	if err != nil {
		return 0, err
	}

	return transHexStringToUint64(result)
}

// EthSyncing returns an object with data about the sync status
func EthSyncing() (bool, error) {
	if clientRPC == nil {
		return false, fmt.Errorf("RPC client not initialized")
	}

	var result interface{}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := clientRPC.Client().CallContext(ctx, &result, "eth_syncing")
	if err != nil {
		return false, err
	}

	// If result is false, node is not syncing
	if result == false {
		return false, nil
	}

	// If result is not false (could be a sync object), node is syncing
	return true, nil
}

// TxPoolContent returns the transactions that are in the transaction pool
func TxPoolContent() (interface{}, error) {
	var result interface{}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := clientRPC.Client().CallContext(ctx, &result, "txpool_content")
	if err != nil {
		return nil, err
	}

	return result, nil
}

// TxPoolStatus returns the number of transactions in the pool
func TxPoolStatus() (map[string]any, error) {
	var result map[string]any
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := clientRPC.Client().CallContext(ctx, &result, "txpool_status")
	if err != nil {
		return nil, err
	}

	return result, nil
}

func RemoveTransaction(networkUrl string, txHash common.Hash) error {
	var result interface{}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := clientRPC.Client().CallContext(ctx, &result, "txpool_removeTransaction", txHash)
	if err != nil {
		return err
	}
	log.Info("Removed transaction result: ", result)
	return nil
}

// EthBlockNumber returns the number of the most recent block
func EthBlockNumber() (uint64, error) {
	var result string
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := clientRPC.Client().CallContext(ctx, &result, "eth_blockNumber")
	if err != nil {
		return 0, err
	}

	return transHexStringToUint64(result)
}

// EthCall executes a new message call immediately without creating a transaction
func EthCall(from, to common.Address, gas string, gasPrice string, value string, data string, block string) (string, error) {
	txParams := map[string]interface{}{
		"from":     from,
		"to":       to,
		"gas":      gas,
		"gasPrice": gasPrice,
		"value":    value,
		"data":     data,
	}

	var result string
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := clientRPC.Client().CallContext(ctx, &result, "eth_call", txParams, block)
	if err != nil {
		return "", err
	}

	return result, nil
}

// EthGasPrice returns the current price per gas in wei
func EthGasPrice() (*big.Int, error) {
	var result string
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := clientRPC.Client().CallContext(ctx, &result, "eth_gasPrice")
	if err != nil {
		return nil, err
	}

	return transHexStringToBigInt(result)
}

// EthGetLogs returns logs matching the given filter
func EthGetLogs(fromBlock, toBlock string, address common.Address) (interface{}, error) {
	filter := map[string]interface{}{
		"fromBlock": fromBlock,
		"toBlock":   toBlock,
		"address":   address,
	}

	var result interface{}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := clientRPC.Client().CallContext(ctx, &result, "eth_getLogs", filter)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// EthGetStorageAt returns the value from a storage position at a given address
func EthGetStorageAt(address common.Address, position string, block string) (string, error) {
	if clientRPC == nil {
		return "", fmt.Errorf("RPC client not initialized")
	}

	var result string
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := clientRPC.Client().CallContext(ctx, &result, "eth_getStorageAt", address, position, block)
	if err != nil {
		return "", err
	}

	return result, nil
}

// EthTransactionPreExec executes a transaction and returns the result without creating a transaction on chain
func EthTransactionPreExec(txRequest interface{}, blockParameter string, stateOverride interface{}) (interface{}, error) {
	if clientRPC == nil {
		return nil, fmt.Errorf("RPC client not initialized")
	}

	var result interface{}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var args []interface{}
	args = append(args, []interface{}{txRequest}) // eth_transactionPreExec expects array of transactions
	args = append(args, blockParameter)
	if stateOverride != nil {
		args = append(args, stateOverride)
	}

	err := clientRPC.Client().CallContext(ctx, &result, "eth_transactionPreExec", args...)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// EthGetTransactionByBlockHashAndIndex returns information about a transaction by block hash and transaction index position
func EthGetTransactionByBlockHashAndIndex(blockHash common.Hash, index string) (interface{}, error) {
	var result interface{}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := clientRPC.Client().CallContext(ctx, &result, "eth_getTransactionByBlockHashAndIndex", blockHash, index)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// EthGetTransactionByBlockNumberAndIndex returns information about a transaction by block number and transaction index position
func EthGetTransactionByBlockNumberAndIndex(blockNumber string, index string) (interface{}, error) {
	var result interface{}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := clientRPC.Client().CallContext(ctx, &result, "eth_getTransactionByBlockNumberAndIndex", blockNumber, index)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// EthGetTransactionByHash returns the information about a transaction requested by transaction hash
func EthGetTransactionByHash(txHash common.Hash) (interface{}, error) {
	var result interface{}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := clientRPC.Client().CallContext(ctx, &result, "eth_getTransactionByHash", txHash)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// EthGetInternalTransactions returns the internal transactions for a given transaction hash
func EthGetInternalTransactions(txHash common.Hash) ([]*types.InnerTx, error) {
	var result []*types.InnerTx
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := clientRPC.Client().CallContext(ctx, &result, "eth_getInternalTransactions", txHash)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// EthGetBlockInternalTransactions returns the internal transactions for a given block number
func EthGetBlockInternalTransactions(blockNumber rpc.BlockNumber) (map[common.Hash][]*types.InnerTx, error) {
	var result map[common.Hash][]*types.InnerTx
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := clientRPC.Client().CallContext(ctx, &result, "eth_getBlockInternalTransactions", blockNumber)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// EthGetTransactionReceipt returns the receipt of a transaction by transaction hash
func EthGetTransactionReceipt(txHash common.Hash) (interface{}, error) {
	var result interface{}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := clientRPC.Client().CallContext(ctx, &result, "eth_getTransactionReceipt", txHash)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// TxPoolLimbo returns the transactions that are in the limbo state
func TxPoolLimbo() (interface{}, error) {
	var result interface{}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := clientRPC.Client().CallContext(ctx, &result, "txpool_limbo")
	if err != nil {
		return nil, err
	}

	return result, nil
}

// ZKEVMBatchNumber returns the current batch number
// func ZKEVMBatchNumber() (uint64, error) {
// 	response, err := client.JSONRPCCall(DefaultL2NetworkURL, "zkevm_batchNumber")
// 	if err != nil {
// 		return 0, err
// 	}
// 	if response.Error != nil {
// 		return 0, fmt.Errorf("%d - %s", response.Error.Code, response.Error.Message)
// 	}

// 	return transHexToUint64(response.Result)
// }

// // ZKEVMGetLatestDataStreamBlock returns the latest data stream block
// func ZKEVMGetLatestDataStreamBlock() (interface{}, error) {
// 	response, err := client.JSONRPCCall(DefaultL2NetworkURL, "zkevm_getLatestDataStreamBlock")
// 	if err != nil {
// 		return nil, err
// 	}
// 	if response.Error != nil {
// 		return nil, fmt.Errorf("%d - %s", response.Error.Code, response.Error.Message)
// 	}

// 	var result interface{}
// 	err = json.Unmarshal(response.Result, &result)
// 	if err != nil {
// 		return nil, err
// 	}

// 	return result, nil
// }

// // ZKEVMEstimateCounters estimates the counters for a given transaction
// func ZKEVMEstimateCounters(from, to common.Address, gas, gasPrice, value, data string) (interface{}, error) {
// 	txParams := map[string]interface{}{
// 		"from":     from,
// 		"to":       to,
// 		"gas":      gas,
// 		"gasPrice": gasPrice,
// 		"value":    value,
// 		"input":    data,
// 	}

// 	response, err := client.JSONRPCCall(DefaultL2NetworkURL, "zkevm_estimateCounters", txParams)
// 	if err != nil {
// 		return nil, err
// 	}
// 	if response.Error != nil {
// 		return nil, fmt.Errorf("%d - %s", response.Error.Code, response.Error.Message)
// 	}

// 	var result interface{}
// 	err = json.Unmarshal(response.Result, &result)
// 	if err != nil {
// 		return nil, err
// 	}

// 	return result, nil
// }

// SyncGetOffChainData returns off-chain data for a given hash
func SyncGetOffChainData(hash common.Hash) (interface{}, error) {
	var result interface{}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := clientRPC.Client().CallContext(ctx, &result, "sync_getOffChainData", hash)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// transHexStringToUint64 converts a hex string to uint64
func transHexStringToUint64(hexStr string) (uint64, error) {
	if len(hexStr) > 2 && (hexStr[:2] == "0x" || hexStr[:2] == "0X") {
		hexStr = hexStr[2:]
	}
	res, err := strconv.ParseUint(hexStr, 16, 64)
	if err != nil {
		return 0, err
	}
	return res, nil
}

// transHexStringToBigInt converts a hex string to a big.Int
func transHexStringToBigInt(hexStr string) (*big.Int, error) {
	if len(hexStr) > 2 && (hexStr[:2] == "0x" || hexStr[:2] == "0X") {
		hexStr = hexStr[2:]
	}

	value := new(big.Int)
	value, ok := value.SetString(hexStr, 16)
	if !ok {
		return nil, fmt.Errorf("failed to convert hex to big.Int: %s", hexStr)
	}

	return value, nil
}
