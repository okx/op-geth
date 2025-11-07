package operations

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
)

func GetBlockNumber() (uint64, error) {
	var result string
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := clientRPC.Client().CallContext(ctx, &result, "eth_blockNumber")
	if err != nil {
		return 0, err
	}

	return transHexStringToUint64(result)
}

func GetEthSyncing(url string) (bool, error) {
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

func GetNetVersion(url string) (uint64, error) {
	var result string
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := clientRPC.Client().CallContext(ctx, &result, "net_version")
	if err != nil {
		return 0, err
	}

	num, err := strconv.ParseUint(result, 10, 64)
	if err != nil {
		return 0, err
	}
	return num, nil
}

func GetBlockByHash(hash common.Hash) (*types.Block, error) {
	var result types.Block
	err := clientRPC.Client().CallContext(context.Background(), &result, "eth_getBlockByHash", hash, true)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func toBlockNumArg(number *big.Int) string {
	if number == nil {
		return "latest"
	}
	if number.Sign() >= 0 {
		return hexutil.EncodeBig(number)
	}
	// It's negative.
	if number.IsInt64() {
		return rpc.BlockNumber(number.Int64()).String()
	}
	// It's negative and large, which is invalid.
	panic(fmt.Sprintf("<invalid block number %d>", number))
}

// GetBlockByNumber retrieves a block by its number
func GetBlockByNumber(blockNumber *big.Int) (*types.Block, error) {
	var result types.Block
	err := clientRPC.Client().CallContext(context.Background(), &result, "eth_getBlockByNumber", toBlockNumArg(blockNumber), true)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func GetBlockHashByNumber(block uint64) (string, error) {
	blockData, err := GetBlockByNumber(big.NewInt(int64(block)))
	if err != nil {
		return "", err
	}
	return blockData.Hash().String(), nil
}

func GetInternalTransactions(hash common.Hash) ([]interface{}, error) {
	var result []interface{}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := clientRPC.Client().CallContext(ctx, &result, "eth_getInternalTransactions", hash)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func GetBlockInternalTransactions(block *big.Int) (map[common.Hash][]interface{}, error) {
	var result map[common.Hash][]interface{}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := clientRPC.Client().CallContext(ctx, &result, "eth_getBlockInternalTransactions", hexutil.EncodeBig(block))
	if err != nil {
		return nil, err
	}

	return result, nil
}

func GetTransactionByHash(hash common.Hash) (*types.Transaction, error) {
	var result types.Transaction
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := clientRPC.Client().CallContext(ctx, &result, "eth_getTransactionByHash", hash)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func GetGasPrice() (uint64, error) {
	var result string
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := clientRPC.Client().CallContext(ctx, &result, "eth_gasPrice")
	if err != nil {
		return 0, err
	}

	return transHexStringToUint64(result)
}

func GetMinGasPrice() (uint64, error) {
	var result string
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := clientRPC.Client().CallContext(ctx, &result, "eth_minGasPrice")
	if err != nil {
		return 0, err
	}

	return transHexStringToUint64(result)
}

func GetMetricsPrometheus() (string, error) {
	client := http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Get(DefaultL2MetricsPrometheusURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func GetMetrics() (string, error) {
	client := http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Get(DefaultL2MetricsURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func GetBlockGasLimit() (uint64, error) {
	var result string
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := clientRPC.Client().CallContext(ctx, &result, "eth_getBlockGasLimit")
	if err != nil {
		return 0, err
	}

	return transHexStringToUint64(result)
}
