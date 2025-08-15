package rtclient

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
)

const (
	PendingTag = "pending"
	LatestTag  = "latest"
)

type RealtimeClient struct {
	*ethclient.Client
	c *rpc.Client
}

// NewClient creates a client that uses the given RPC client.
func NewRealtimeClient(ctx context.Context, ethClient *ethclient.Client, rawurl string) (*RealtimeClient, error) {
	c, err := rpc.DialContext(ctx, rawurl)
	if err != nil {
		return nil, err
	}

	return &RealtimeClient{
		Client: ethClient,
		c:      c,
	}, nil
}

// RealtimeBlockNumber returns the number of the most recent block in real-time
func (rc *RealtimeClient) RealtimeBlockNumber(ctx context.Context) (uint64, error) {
	var result hexutil.Uint64
	err := rc.c.CallContext(ctx, &result, "eth_blockNumber", LatestTag)
	return uint64(result), err
}

// RealtimePendingBlockNumber returns the number of the most recent block in real-time
func (rc *RealtimeClient) RealtimePendingBlockNumber(ctx context.Context) (uint64, error) {
	var result hexutil.Uint64
	err := rc.c.CallContext(ctx, &result, "eth_blockNumber", PendingTag)
	return uint64(result), err
}

// RealtimeCall executes a new message call immediately without creating a transaction in real-time
func (rc *RealtimeClient) RealtimeCall(ctx context.Context, from, to common.Address, gas string, gasPrice string, value string, data string) (string, error) {
	txParams := map[string]any{
		"from":     from,
		"to":       to,
		"gas":      gas,
		"gasPrice": gasPrice,
		"value":    value,
		"data":     data,
	}

	var hex hexutil.Bytes
	err := rc.c.CallContext(ctx, &hex, "eth_call", txParams, PendingTag)
	if err != nil {
		return "", err
	}
	return hexutil.Encode(hex), nil
}

// RealtimeEstimateGas estimates gas for a transaction in real-time
func (rc *RealtimeClient) RealtimeEstimateGas(ctx context.Context, args map[string]any) (uint64, error) {
	var result string
	err := rc.c.CallContext(ctx, &result, "eth_estimateGas", args, PendingTag)
	if err != nil {
		return 0, err
	}
	// Convert hex string to uint64
	gasEstimate, err := hexutil.DecodeUint64(result)
	if err != nil {
		return 0, err
	}

	return gasEstimate, nil
}

// RealtimeGetBalance returns the balance of an account in real-time
func (rc *RealtimeClient) RealtimeGetBalance(ctx context.Context, address common.Address) (*big.Int, error) {
	var result hexutil.Big
	err := rc.c.CallContext(ctx, &result, "eth_getBalance", address, PendingTag)
	return (*big.Int)(&result), err
}

// RealtimeGetTransactionCount returns the number of transactions sent from an address in real-time
func (rc *RealtimeClient) RealtimeGetTransactionCount(ctx context.Context, address common.Address) (uint64, error) {
	var result hexutil.Uint64
	err := rc.c.CallContext(ctx, &result, "eth_getTransactionCount", address, PendingTag)
	return uint64(result), err
}

// RealtimeGetCode returns the code at a given address in real-time
func (rc *RealtimeClient) RealtimeGetCode(ctx context.Context, address common.Address) (string, error) {
	var result hexutil.Bytes
	err := rc.c.CallContext(ctx, &result, "eth_getCode", address, PendingTag)
	return hexutil.Encode(result), err
}

// RealtimeGetStorageAt returns the value from a storage position at a given address in real-time
func (rc *RealtimeClient) RealtimeGetStorageAt(ctx context.Context, address common.Address, position string) (string, error) {
	var result hexutil.Bytes
	err := rc.c.CallContext(ctx, &result, "eth_getStorageAt", address, position, PendingTag)
	return hexutil.Encode(result), err
}

// RealtimeGetLatestBlockTransactionCount returns the number of transactions in a block by number in real-time
func (rc *RealtimeClient) RealtimeGetLatestBlockTransactionCount(ctx context.Context) (uint64, error) {
	var result hexutil.Uint64
	err := rc.c.CallContext(ctx, &result, "eth_getBlockTransactionCountByNumber", LatestTag)
	return uint64(result), err
}

// RealtimeGetPendingBlockTransactionCount returns the number of transactions in a block by number in real-time
func (rc *RealtimeClient) RealtimeGetPendingBlockTransactionCount(ctx context.Context) (uint64, error) {
	var result hexutil.Uint64
	err := rc.c.CallContext(ctx, &result, "eth_getBlockTransactionCountByNumber", PendingTag)
	return uint64(result), err
}

// RealtimeGetBlockTransactionCountByNumber returns the number of transactions in a block by number in real-time
func (rc *RealtimeClient) RealtimeGetBlockTransactionCountByNumber(ctx context.Context, blockNumber uint64) (uint64, error) {
	var result hexutil.Uint64
	err := rc.c.CallContext(ctx, &result, "eth_getBlockTransactionCountByNumber", blockNumber)
	return uint64(result), err
}

// RealtimeGetTransactionByHash returns the information about a transaction requested by transaction hash in real-time
func (rc *RealtimeClient) RealtimeGetTransactionByHash(ctx context.Context, txHash common.Hash) (*types.Transaction, error) {
	var json *rpcTransaction
	err := rc.c.CallContext(ctx, &json, "eth_getTransactionByHash", txHash)
	if err != nil {
		return nil, err
	} else if json == nil {
		return nil, ethereum.NotFound
	} else if _, r, _ := json.tx.RawSignatureValues(); r == nil {
		return nil, errors.New("server returned transaction without signature")
	}
	if json.From != nil && json.BlockHash != nil {
		setSenderFromServer(json.tx, *json.From, *json.BlockHash)
	}
	return json.tx, nil
}

// RealtimeGetTransactionByHash returns raw information about a transaction requested by transaction hash in real-time
func (rc *RealtimeClient) RealtimeGetRawTransactionByHash(ctx context.Context, txHash common.Hash) ([]byte, error) {
	var result hexutil.Bytes
	err := rc.c.CallContext(ctx, &result, "eth_getRawTransactionByHash", txHash)
	return result, err
}

// RealtimeGetTransactionReceipt returns the receipt of a transaction by transaction hash in real-time
func (rc *RealtimeClient) RealtimeGetTransactionReceipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error) {
	var r *types.Receipt
	err := rc.c.CallContext(ctx, &r, "eth_getTransactionReceipt", txHash)
	if err == nil && r == nil {
		return nil, ethereum.NotFound
	}
	return r, err
}

// RealtimeGetInternalTransactions returns the internal transactions for a given transaction hash in real-time
func (rc *RealtimeClient) RealtimeGetInternalTransactions(ctx context.Context, txHash common.Hash) ([]types.InnerTx, error) {
	var innerTxs []types.InnerTx
	err := rc.c.CallContext(ctx, &innerTxs, "eth_getInternalTransactions", txHash)
	if err != nil {
		return nil, err
	}
	return innerTxs, err
}

func (rc *RealtimeClient) RealtimeGetTokenBalance(
	ctx context.Context,
	fromAddress common.Address,
	toAddress common.Address,
	erc20Addr common.Address,
) (*big.Int, error) {
	// Pack the balanceOf function call
	data, err := erc20ABI.Pack("balanceOf", toAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to pack balanceOf call: %v", err)
	}

	// Make the realtime eth_call
	result, err := rc.RealtimeCall(ctx, fromAddress, erc20Addr, "0x100000", "0x1", "0x0", fmt.Sprintf("0x%x", data))
	if err != nil {
		return nil, fmt.Errorf("failed to call contract: %v", err)
	}

	// Parse the hex result
	if len(result) > 2 && (result[:2] == "0x" || result[:2] == "0X") {
		result = result[2:]
	}

	balance := new(big.Int)
	balance, ok := balance.SetString(result, 16)
	if !ok {
		return nil, fmt.Errorf("failed to convert hex to big.Int: %s", result)
	}

	return balance, nil
}

func (rc *RealtimeClient) EthGetTokenBalance(
	ctx context.Context,
	addr common.Address,
	erc20Addr common.Address,
) (*big.Int, error) {
	// Pack the balanceOf function call
	data, err := erc20ABI.Pack("balanceOf", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to pack balanceOf call: %v", err)
	}

	// Make the eth_call
	result, err := rc.CallContract(ctx, ethereum.CallMsg{
		To:   &erc20Addr,
		Data: data,
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to call contract: %v", err)
	}

	// Unpack the result
	var balance *big.Int
	err = erc20ABI.UnpackIntoInterface(&balance, "balanceOf", result)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack result: %v", err)
	}

	return balance, nil
}

func (rc *RealtimeClient) RealtimeGetBlockByNumber(ctx context.Context, blockNumber uint64) (map[string]interface{}, error) {
	// Call eth_getBlockByNumber with fullTx=true to get full transaction details
	fullTx := true
	var result map[string]interface{}
	err := rc.c.CallContext(ctx, &result, "eth_getBlockByNumber", blockNumber, fullTx)
	return result, err
}

// RealtimeGetBlockByHash returns the information about a block requested by block hash in real-time
func (rc *RealtimeClient) RealtimeGetBlockByHash(ctx context.Context, blockHash common.Hash, fullTx bool) (map[string]interface{}, error) {
	var result map[string]interface{}
	err := rc.c.CallContext(ctx, &result, "eth_getBlockByHash", blockHash, fullTx)
	return result, err
}

// RealtimeGetBlockTransactionCountByHash returns the number of transactions in a block requested by block hash in real-time
func (rc *RealtimeClient) RealtimeGetBlockTransactionCountByHash(ctx context.Context, blockHash common.Hash) (uint64, error) {
	var result hexutil.Uint64
	err := rc.c.CallContext(ctx, &result, "eth_getBlockTransactionCountByHash", blockHash)
	return uint64(result), err
}

func (rc *RealtimeClient) RealtimeGetBlockInternalTransactions(ctx context.Context, blockNumber uint64) (map[common.Hash][]*types.InnerTx, error) {
	var result map[common.Hash][]*types.InnerTx
	err := rc.c.CallContext(ctx, &result, "eth_getBlockInternalTransactions", blockNumber)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (rc *RealtimeClient) RealtimeEnabled(ctx context.Context) (bool, error) {
	var result bool
	err := rc.c.CallContext(ctx, &result, "eth_realtimeEnabled")
	return result, err
}
