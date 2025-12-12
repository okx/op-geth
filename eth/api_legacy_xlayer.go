package eth

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/eth/filters"
	"github.com/ethereum/go-ethereum/internal/ethapi"
	"github.com/ethereum/go-ethereum/internal/ethapi/override"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"
)

// XlayerLegacyRPCService Forward Policy
// LOCAL
// 1. Local first
// 2. If not found, and erigon configured, forward request
// FORWARD
// 1. number, greater than migration block, use local, do not fallback
// 2. number, less or equal than migration block, use proxy, do not fallback
// 3. other cases, use local, fallback to proxy

var (
	errInvalidBlockRange = errors.New("invalid block range params")
)

// XlayerLegacyRPCService holds the configuration for RPC migration
type XlayerLegacyRPCService struct {
	MigrationBlock uint64
	ErigonClient   *rpc.Client
}

// NewXlayerLegacyRPCService creates a new migration configuration
func NewXlayerLegacyRPCService(config *ethconfig.Config) (*XlayerLegacyRPCService, error) {
	if config.XLayer.LegacyPp.MigrationBlock == nil || config.XLayer.LegacyPp.PPRPCUrl == "" {
		return nil, nil // Migration not configured
	}

	timeout := config.XLayer.LegacyPp.PPRPCTimeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	erigonClient, err := rpc.DialContext(ctx, config.XLayer.LegacyPp.PPRPCUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to erigon RPC: %w", err)
	}

	return &XlayerLegacyRPCService{
		MigrationBlock: *config.XLayer.LegacyPp.MigrationBlock,
		ErigonClient:   erigonClient,
	}, nil
}

// Close closes the erigon RPC client
func (mc *XlayerLegacyRPCService) Close() {
	if mc.ErigonClient == nil {
		return
	}
	mc.ErigonClient.Close()
}

// shouldProxy determines if a request should be proxied based on block number
func (mc *XlayerLegacyRPCService) shouldProxyByNumber(blockNumber rpc.BlockNumber) bool {
	if blockNumber < 0 {
		// for the following special block numbers defined in rpc/types.go, do not proxy
		// EarliestBlockNumber  = BlockNumber(-5)
		// SafeBlockNumber      = BlockNumber(-4)
		// FinalizedBlockNumber = BlockNumber(-3)
		// LatestBlockNumber    = BlockNumber(-2)
		// PendingBlockNumber   = BlockNumber(-1)
		return false
	}
	return mc.MigrationBlock > 0 && uint64(blockNumber.Int64()) < mc.MigrationBlock
}

func (api *XlayerHybridBlockChainAPI) shouldProxy(ctx context.Context, bNrOrHash *rpc.BlockNumberOrHash) bool {
	if bNrOrHash == nil {
		return false
	}

	if blockNr, ok := bNrOrHash.Number(); ok {
		// For specific historical blocks, check if we should proxy to Erigon
		if blockNr >= 0 && api.legacyRpc.shouldProxyByNumber(blockNr) {
			return true
		}
		return false
	}

	if hash, ok := bNrOrHash.Hash(); ok {
		header := api.BlockChainAPI.GetHeaderByHash(ctx, hash)
		return header == nil
	}

	return false
}

// XlayerHybridBlockChainAPI wraps the standard BlockChainAPI to add migration routing
type XlayerHybridBlockChainAPI struct {
	*ethapi.BlockChainAPI
	txPreExecAPI *TxPreExecAPI
	legacyRpc    *XlayerLegacyRPCService
}

// NewXlayerHybridBlockChainAPI creates a new migration-aware BlockChainAPI
func NewXlayerHybridBlockChainAPI(original *ethapi.BlockChainAPI, txPreExecAPI *TxPreExecAPI, legacyRPCService *XlayerLegacyRPCService) *XlayerHybridBlockChainAPI {
	return &XlayerHybridBlockChainAPI{
		BlockChainAPI: original,
		legacyRpc:     legacyRPCService,
		txPreExecAPI:  txPreExecAPI,
	}
}

// eth_call
// FORWARD
func (api *XlayerHybridBlockChainAPI) Call(ctx context.Context, args ethapi.TransactionArgs, blockNrOrHash *rpc.BlockNumberOrHash, overrides *override.StateOverride, blockOverrides *override.BlockOverrides) (hexutil.Bytes, error) {
	bNrOrHash := rpc.BlockNumberOrHashWithNumber(rpc.LatestBlockNumber)
	if blockNrOrHash != nil {
		bNrOrHash = *blockNrOrHash
	}

	callRequest := map[string]interface{}{
		"from":                 args.From,
		"to":                   args.To,
		"value":                args.Value,
		"gas":                  args.Gas,
		"gasPrice":             args.GasPrice,
		"data":                 args.Data,
		"nonce":                args.Nonce,
		"input":                args.Input,
		"maxFeePerGas":         args.MaxFeePerGas,
		"maxPriorityFeePerGas": args.MaxPriorityFeePerGas,
		"maxFeePerBlobGas":     args.BlobFeeCap,
		"accessList":           args.AccessList,
		"blobVersionedHashes":  args.BlobHashes,
		"blobs":                args.Blobs,
		"chainId":              args.ChainID,
	}

	shouldProxy := api.shouldProxy(ctx, &bNrOrHash)
	if shouldProxy {
		var result hexutil.Bytes
		err := api.legacyRpc.ErigonClient.CallContext(ctx, &result, "eth_call", callRequest, &bNrOrHash)
		return result, err
	}

	return api.BlockChainAPI.Call(ctx, args, &bNrOrHash, overrides, blockOverrides)
}

// eth_estimateGas
// FORWARD
func (api *XlayerHybridBlockChainAPI) EstimateGas(ctx context.Context, args ethapi.TransactionArgs, blockNrOrHash *rpc.BlockNumberOrHash, overrides *override.StateOverride, blockOverrides *override.BlockOverrides) (hexutil.Uint64, error) {
	bNrOrHash := rpc.BlockNumberOrHashWithNumber(rpc.LatestBlockNumber)
	if blockNrOrHash != nil {
		bNrOrHash = *blockNrOrHash
	}
	estimateGasRequest := map[string]interface{}{
		"from":                 args.From,
		"to":                   args.To,
		"value":                args.Value,
		"gas":                  args.Gas,
		"gasPrice":             args.GasPrice,
		"data":                 args.Data,
		"nonce":                args.Nonce,
		"input":                args.Input,
		"maxFeePerGas":         args.MaxFeePerGas,
		"maxPriorityFeePerGas": args.MaxPriorityFeePerGas,
		"maxFeePerBlobGas":     args.BlobFeeCap,
		"accessList":           args.AccessList,
		"blobVersionedHashes":  args.BlobHashes,
		"blobs":                args.Blobs,
		"chainId":              args.ChainID,
	}

	shouldProxy := api.shouldProxy(ctx, &bNrOrHash)

	if shouldProxy {
		var result hexutil.Uint64
		err := api.legacyRpc.ErigonClient.CallContext(ctx, &result, "eth_estimateGas", estimateGasRequest, &bNrOrHash)
		return result, err
	}

	return api.BlockChainAPI.EstimateGas(ctx, args, &bNrOrHash, overrides, blockOverrides)
}

type accessListResult struct {
	Accesslist *types.AccessList `json:"accessList"`
	Error      string            `json:"error,omitempty"`
	GasUsed    hexutil.Uint64    `json:"gasUsed"`
}

// eth_createAccessList
// FORWARD
func (api *XlayerHybridBlockChainAPI) CreateAccessList(ctx context.Context, args ethapi.TransactionArgs, blockNrOrHash *rpc.BlockNumberOrHash, stateOverrides *override.StateOverride) (*accessListResult, error) {
	bNrOrHash := rpc.BlockNumberOrHashWithNumber(rpc.LatestBlockNumber)
	if blockNrOrHash != nil {
		bNrOrHash = *blockNrOrHash
	}

	createAccessListRequest := map[string]interface{}{
		"from":                 args.From,
		"to":                   args.To,
		"value":                args.Value,
		"gas":                  args.Gas,
		"gasPrice":             args.GasPrice,
		"data":                 args.Data,
		"nonce":                args.Nonce,
		"input":                args.Input,
		"maxFeePerGas":         args.MaxFeePerGas,
		"maxPriorityFeePerGas": args.MaxPriorityFeePerGas,
		"maxFeePerBlobGas":     args.BlobFeeCap,
		"accessList":           args.AccessList,
		"blobVersionedHashes":  args.BlobHashes,
		"blobs":                args.Blobs,
		"chainId":              args.ChainID,
	}

	shouldProxy := api.shouldProxy(ctx, &bNrOrHash)

	if shouldProxy {
		var result *accessListResult
		err := api.legacyRpc.ErigonClient.CallContext(ctx, &result, "eth_createAccessList", createAccessListRequest, &bNrOrHash)
		return result, err
	}

	accessList, err := api.BlockChainAPI.CreateAccessList(ctx, args, &bNrOrHash, stateOverrides)
	return (*accessListResult)(accessList), err
}

// eth_getBlockByNumber
// FORWARD
func (api *XlayerHybridBlockChainAPI) GetBlockByNumber(ctx context.Context, number rpc.BlockNumber, fullTx bool) (map[string]interface{}, error) {
	// Check if we should proxy to erigon
	if api.legacyRpc.shouldProxyByNumber(number) {
		var result map[string]interface{}
		err := api.legacyRpc.ErigonClient.CallContext(ctx, &result, "eth_getBlockByNumber", hexutil.Uint64(number), fullTx)
		return result, err
	}
	// Handle locally
	return api.BlockChainAPI.GetBlockByNumber(ctx, number, fullTx)
}

// eth_getBlockByHash
// LOCAL
func (api *XlayerHybridBlockChainAPI) GetBlockByHash(ctx context.Context, hash common.Hash, fullTx bool) (map[string]interface{}, error) {
	// Try local first
	result, err := api.BlockChainAPI.GetBlockByHash(ctx, hash, fullTx)
	if err == nil && result != nil {
		return result, nil
	}

	// If not found locally and migration is configured, try erigon
	var remoteResult map[string]interface{}
	err = api.legacyRpc.ErigonClient.CallContext(ctx, &remoteResult, "eth_getBlockByHash", hash, fullTx)
	if err == nil && remoteResult != nil {
		return remoteResult, nil
	}

	return result, err
}

// eth_getStorageAt
// FORWARD
func (api *XlayerHybridBlockChainAPI) GetStorageAt(ctx context.Context, address common.Address, hexKey string, blockNrOrHash rpc.BlockNumberOrHash) (hexutil.Bytes, error) {
	shouldProxy := api.shouldProxy(ctx, &blockNrOrHash)

	if shouldProxy {
		var result hexutil.Bytes
		err := api.legacyRpc.ErigonClient.CallContext(ctx, &result, "eth_getStorageAt", address, hexKey, blockNrOrHash)
		return result, err
	}

	return api.BlockChainAPI.GetStorageAt(ctx, address, hexKey, blockNrOrHash)
}

// eth_getHeaderByHash LOCAL
func (api *XlayerHybridBlockChainAPI) GetHeaderByHash(ctx context.Context, hash common.Hash) (map[string]interface{}, error) {
	// Try local first to get the header and determine block number
	localResult := api.BlockChainAPI.GetHeaderByHash(ctx, hash)
	if localResult != nil {
		return localResult, nil
	}

	// If not found locally and migration is configured, try erigon
	var result map[string]interface{}
	err := api.legacyRpc.ErigonClient.CallContext(ctx, &result, "eth_getHeaderByHash", hash)
	return result, err
}

// eth_getHeaderByNumber
// FORWARD
func (api *XlayerHybridBlockChainAPI) GetHeaderByNumber(ctx context.Context, number rpc.BlockNumber) (map[string]interface{}, error) {
	// Check if we should proxy to erigon
	if api.legacyRpc.shouldProxyByNumber(number) {
		var result map[string]interface{}
		err := api.legacyRpc.ErigonClient.CallContext(ctx, &result, "eth_getHeaderByNumber", hexutil.Uint64(number))
		return result, err
	}

	// Handle locally
	return api.BlockChainAPI.GetHeaderByNumber(ctx, number)
}

// eth_getBlockReceipts
// FORWARD
func (api *XlayerHybridBlockChainAPI) GetBlockReceipts(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) ([]map[string]interface{}, error) {
	shouldProxy := api.shouldProxy(ctx, &blockNrOrHash)

	if shouldProxy {
		var result []map[string]interface{}
		err := api.legacyRpc.ErigonClient.CallContext(ctx, &result, "eth_getBlockReceipts", blockNrOrHash)
		return result, err
	}

	return api.BlockChainAPI.GetBlockReceipts(ctx, blockNrOrHash)
}

// eth_getBalance
// FORWARD
func (api *XlayerHybridBlockChainAPI) GetBalance(ctx context.Context, address common.Address, blockNrOrHash rpc.BlockNumberOrHash) (*hexutil.Big, error) {
	shouldProxy := api.shouldProxy(ctx, &blockNrOrHash)
	if shouldProxy {
		var result *hexutil.Big
		err := api.legacyRpc.ErigonClient.CallContext(ctx, &result, "eth_getBalance", address, blockNrOrHash)
		return result, err
	}

	return api.BlockChainAPI.GetBalance(ctx, address, blockNrOrHash)
}

// eth_getCode
// FORWARD
func (api *XlayerHybridBlockChainAPI) GetCode(ctx context.Context, address common.Address, blockNrOrHash rpc.BlockNumberOrHash) (hexutil.Bytes, error) {
	shouldProxy := api.shouldProxy(ctx, &blockNrOrHash)
	if shouldProxy {
		var result hexutil.Bytes
		err := api.legacyRpc.ErigonClient.CallContext(ctx, &result, "eth_getCode", address, blockNrOrHash)
		return result, err
	}

	return api.BlockChainAPI.GetCode(ctx, address, blockNrOrHash)
}

// eth_transactionPreExec FORWARD
func (api *XlayerHybridBlockChainAPI) TransactionPreExec(ctx context.Context, origins []PreArgs, blockNrOrHash *rpc.BlockNumberOrHash, stateOverrides *override.StateOverride) ([]PreResult, error) {
	if api.txPreExecAPI == nil {
		return nil, fmt.Errorf("TxPreExecAPI not available")
	}

	bNrOrHash := rpc.BlockNumberOrHashWithNumber(rpc.LatestBlockNumber)
	if blockNrOrHash != nil {
		bNrOrHash = *blockNrOrHash
	}

	shouldProxy := api.shouldProxy(ctx, &bNrOrHash)
	if shouldProxy {
		var result []PreResult
		err := api.legacyRpc.ErigonClient.CallContext(ctx, &result, "eth_transactionPreExec", origins, &bNrOrHash, stateOverrides)
		return result, err
	}

	return api.txPreExecAPI.TransactionPreExec(ctx, origins, &bNrOrHash, stateOverrides)
}

// XlayerHybridTransactionAPI wraps the standard TransactionAPI to add migration routing
type XlayerHybridTransactionAPI struct {
	*ethapi.TransactionAPI
	TxPreExecAPI *TxPreExecAPI
	legacyRpc    *XlayerLegacyRPCService
}

// NewXlayerHybridTransactionAPI creates a new migration-aware TransactionAPI
func NewXlayerHybridTransactionAPI(original *ethapi.TransactionAPI, config *XlayerLegacyRPCService) *XlayerHybridTransactionAPI {
	return &XlayerHybridTransactionAPI{
		TransactionAPI: original,
		legacyRpc:      config,
	}
}

// eth_getTransactionByHash LOCAL
func (api *XlayerHybridTransactionAPI) GetTransactionByHash(ctx context.Context, hash common.Hash) (*ethapi.RPCTransaction, error) {
	// Try local first
	tx, err := api.TransactionAPI.GetTransactionByHash(ctx, hash)
	if err == nil && tx != nil {
		return tx, nil
	}

	// If not found locally and migration is configured, try erigon
	var remoteTx *ethapi.RPCTransaction
	err = api.legacyRpc.ErigonClient.CallContext(ctx, &remoteTx, "eth_getTransactionByHash", hash)
	return remoteTx, err
}

// eth_getTransactionReceipt LOCAL
func (api *XlayerHybridTransactionAPI) GetTransactionReceipt(ctx context.Context, hash common.Hash) (map[string]interface{}, error) {
	// Try local first
	receipt, err := api.TransactionAPI.GetTransactionReceipt(ctx, hash)
	if err == nil && receipt != nil {
		return receipt, nil
	}

	// If not found locally and migration is configured, try erigon
	var remoteReceipt map[string]interface{}
	err = api.legacyRpc.ErigonClient.CallContext(ctx, &remoteReceipt, "eth_getTransactionReceipt", hash)
	return remoteReceipt, err
}

// eth_getBlockTransactionCountByHash LOCAL
func (api *XlayerHybridTransactionAPI) GetBlockTransactionCountByHash(ctx context.Context, blockHash common.Hash) (*hexutil.Uint, error) {
	// Try local first
	result, err := api.TransactionAPI.GetBlockTransactionCountByHash(ctx, blockHash)
	if err == nil && result != nil {
		return result, nil
	}

	// If not found locally and migration is configured, try erigon
	var remoteResult *hexutil.Uint
	err = api.legacyRpc.ErigonClient.CallContext(ctx, &remoteResult, "eth_getBlockTransactionCountByHash", blockHash)
	return remoteResult, err
}

// eth_getBlockTransactionCountByNumber FORWARD
func (api *XlayerHybridTransactionAPI) GetBlockTransactionCountByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*hexutil.Uint, error) {
	// Check if we should proxy to erigon
	if api.legacyRpc.shouldProxyByNumber(blockNr) {
		var result *hexutil.Uint
		err := api.legacyRpc.ErigonClient.CallContext(ctx, &result, "eth_getBlockTransactionCountByNumber", hexutil.Uint64(blockNr))
		return result, err
	}
	// Handle locally
	return api.TransactionAPI.GetBlockTransactionCountByNumber(ctx, blockNr)
}

// eth_getBlockInternalTransactions FORWARD
func (api *XlayerHybridTransactionAPI) GetBlockInternalTransactions(ctx context.Context, blockNr rpc.BlockNumber) (map[common.Hash][]*types.InnerTx, error) {
	// Check if we should proxy to erigon
	if api.legacyRpc.shouldProxyByNumber(blockNr) {
		var result map[common.Hash][]*types.InnerTx
		err := api.legacyRpc.ErigonClient.CallContext(ctx, &result, "eth_getBlockInternalTransactions", hexutil.Uint64(blockNr))
		return result, err
	}
	// Handle locally
	return api.TransactionAPI.GetBlockInternalTransactions(ctx, blockNr)
}

// eth_getInternalTransactions TransactionAPI LOCAL
func (api *XlayerHybridTransactionAPI) GetInternalTransactions(ctx context.Context, txHash common.Hash) ([]*types.InnerTx, error) {
	// Check if the transaction exists locally
	tx, err := api.TransactionAPI.GetTransactionByHash(ctx, txHash)

	// If transaction doesn't exist locally, try Erigon
	if tx == nil || err != nil {
		var remoteResult []*types.InnerTx
		err := api.legacyRpc.ErigonClient.CallContext(ctx, &remoteResult, "eth_getInternalTransactions", txHash)
		return remoteResult, err
	}

	// Transaction exists locally
	return api.TransactionAPI.GetInternalTransactions(ctx, txHash)
}

// eth_getRawTransactionByBlockHashAndIndex TransactionAPI LOCAL
func (api *XlayerHybridTransactionAPI) GetRawTransactionByBlockHashAndIndex(ctx context.Context, blockHash common.Hash, index hexutil.Uint) hexutil.Bytes {
	// Try local first
	result := api.TransactionAPI.GetRawTransactionByBlockHashAndIndex(ctx, blockHash, index)
	if result != nil {
		return result
	}
	// If not found locally and migration is configured, try erigon
	var remoteResult hexutil.Bytes
	err := api.legacyRpc.ErigonClient.CallContext(ctx, &remoteResult, "eth_getRawTransactionByBlockHashAndIndex", blockHash, index)
	if err != nil {
		return nil
	}
	return remoteResult
}

// eth_getRawTransactionByBlockNumberAndIndex TransactionAPI FORWARD
func (api *XlayerHybridTransactionAPI) GetRawTransactionByBlockNumberAndIndex(ctx context.Context, blockNr rpc.BlockNumber, index hexutil.Uint) hexutil.Bytes {
	// If not found locally and migration is configured, try erigon
	if api.legacyRpc.shouldProxyByNumber(blockNr) {
		var result hexutil.Bytes
		err := api.legacyRpc.ErigonClient.CallContext(ctx, &result, "eth_getRawTransactionByBlockNumberAndIndex", blockNr, index)
		if err == nil && result != nil {
			return result
		}
	}

	return api.TransactionAPI.GetRawTransactionByBlockNumberAndIndex(ctx, blockNr, index)
}

// eth_getRawTransactionByHash TransactionAPI LOCAL
func (api *XlayerHybridTransactionAPI) GetRawTransactionByHash(ctx context.Context, hash common.Hash) (hexutil.Bytes, error) {
	// Try local first
	result, err := api.TransactionAPI.GetRawTransactionByHash(ctx, hash)
	if err == nil && result != nil {
		return result, nil
	}

	// If not found locally and migration is configured, try erigon
	var remoteResult hexutil.Bytes
	err = api.legacyRpc.ErigonClient.CallContext(ctx, &remoteResult, "eth_getRawTransactionByHash", hash)
	return remoteResult, err
}

// eth_getTransactionByBlockHashAndIndex TransactionAPI LOCAL
func (api *XlayerHybridTransactionAPI) GetTransactionByBlockHashAndIndex(ctx context.Context, blockHash common.Hash, index hexutil.Uint) (*ethapi.RPCTransaction, error) {
	// Try local first
	result, err := api.TransactionAPI.GetTransactionByBlockHashAndIndex(ctx, blockHash, index)
	if err == nil && result != nil {
		return result, nil
	}

	// If not found locally and migration is configured, try erigon
	var remoteResult *ethapi.RPCTransaction
	err = api.legacyRpc.ErigonClient.CallContext(ctx, &remoteResult, "eth_getTransactionByBlockHashAndIndex", blockHash, index)
	return remoteResult, err
}

// eth_getTransactionByBlockNumberAndIndex TransactionAPI FORWARD
func (api *XlayerHybridTransactionAPI) GetTransactionByBlockNumberAndIndex(ctx context.Context, blockNr rpc.BlockNumber, index hexutil.Uint) (*ethapi.RPCTransaction, error) {
	if api.legacyRpc.shouldProxyByNumber(blockNr) {
		var result *ethapi.RPCTransaction
		err := api.legacyRpc.ErigonClient.CallContext(ctx, &result, "eth_getTransactionByBlockNumberAndIndex", blockNr, index)
		return result, err
	}
	return api.TransactionAPI.GetTransactionByBlockNumberAndIndex(ctx, blockNr, index)
}

// eth_getTransactionCount TransactionAPI FORWARD
func (api *XlayerHybridTransactionAPI) GetTransactionCount(ctx context.Context, address common.Address, blockNrOrHash rpc.BlockNumberOrHash) (*hexutil.Uint64, error) {
	if blockNr, ok := blockNrOrHash.Number(); ok {
		if api.legacyRpc.shouldProxyByNumber(blockNr) {
			var result *hexutil.Uint64
			err := api.legacyRpc.ErigonClient.CallContext(ctx, &result, "eth_getTransactionCount", address, blockNrOrHash)
			return result, err
		} else {
			return api.TransactionAPI.GetTransactionCount(ctx, address, blockNrOrHash)
		}
	}

	localResult, err := api.TransactionAPI.GetTransactionCount(ctx, address, blockNrOrHash)
	if err == nil && localResult != nil {
		return localResult, nil
	}

	var result *hexutil.Uint64
	err = api.legacyRpc.ErigonClient.CallContext(ctx, &result, "eth_getTransactionCount", address, blockNrOrHash)
	return result, err
}

type XlayerHybridFilterAPI struct {
	*filters.FilterAPI
	legacyRpc *XlayerLegacyRPCService
	// Track which filters are managed by erigon
	erigonFilters map[rpc.ID]bool
	filtersMu     sync.Mutex
}

// NewXlayerHybridFilterAPI creates a new migration-aware FilterAPI
func NewXlayerHybridFilterAPI(original *filters.FilterAPI, config *XlayerLegacyRPCService) *XlayerHybridFilterAPI {
	return &XlayerHybridFilterAPI{
		FilterAPI:     original,
		legacyRpc:     config,
		erigonFilters: make(map[rpc.ID]bool),
	}
}

// eth_newFilter
// eth_uninstallFilter
// eth_getFilterChanges
// eth_getFilterLogs
// If range overlaps the migration_block, return error, else FORWARD
func (api *XlayerHybridFilterAPI) NewFilter(crit filters.FilterCriteria) (rpc.ID, error) {
	// Determine the block range
	begin := rpc.LatestBlockNumber.Int64()
	if crit.FromBlock != nil {
		begin = crit.FromBlock.Int64()
	}
	end := rpc.LatestBlockNumber.Int64()
	if crit.ToBlock != nil {
		end = crit.ToBlock.Int64()
	}

	// Check for invalid range
	if begin > 0 && end > 0 && begin > end {
		return "", errInvalidBlockRange
	}

	migrationBlock := int64(api.legacyRpc.MigrationBlock)

	// Check if range overlaps with migration block
	// Case 1: Both begin and end are before migration block -> forward to Erigon
	if begin >= 0 && end >= 0 && end < migrationBlock {
		var id rpc.ID
		err := api.legacyRpc.ErigonClient.Call(&id, "eth_newFilter", crit)
		if err != nil {
			return "", err
		}
		// Track this filter as managed by Erigon
		api.filtersMu.Lock()
		api.erigonFilters[id] = true
		api.filtersMu.Unlock()
		return id, nil
	}

	// Case 2: Both begin and end are at or after migration block -> use local
	if begin >= migrationBlock {
		return api.FilterAPI.NewFilter(crit)
	}

	// Case 3: Range overlaps migration block -> return error
	if begin < migrationBlock && end >= migrationBlock {
		return "", fmt.Errorf("filter range overlaps migration block %d: fromBlock=%d, toBlock=%d",
			api.legacyRpc.MigrationBlock, begin, end)
	}

	// Handle special block numbers (latest, pending) -> use local
	if begin < 0 || end < 0 {
		return api.FilterAPI.NewFilter(crit)
	}

	// Default to local
	return api.FilterAPI.NewFilter(crit)
}

// eth_uninstallFilter
func (api *XlayerHybridFilterAPI) UninstallFilter(id rpc.ID) bool {
	// Check if this filter is managed by Erigon
	api.filtersMu.Lock()
	isErigon := api.erigonFilters[id]
	if isErigon {
		delete(api.erigonFilters, id)
	}
	api.filtersMu.Unlock()

	// If managed by Erigon, forward the uninstall request
	if isErigon {
		var result bool
		err := api.legacyRpc.ErigonClient.Call(&result, "eth_uninstallFilter", id)
		if err != nil {
			// Log the error but still return false
			return false
		}
		return result
	}

	// Otherwise, use local
	return api.FilterAPI.UninstallFilter(id)
}

// eth_getFilterChanges
func (api *XlayerHybridFilterAPI) GetFilterChanges(id rpc.ID) (interface{}, error) {
	// Check if this filter is managed by Erigon
	api.filtersMu.Lock()
	isErigon := api.erigonFilters[id]
	api.filtersMu.Unlock()

	// If managed by Erigon, forward the request
	if isErigon {
		var result interface{}
		err := api.legacyRpc.ErigonClient.Call(&result, "eth_getFilterChanges", id)
		return result, err
	}

	// Otherwise, use local
	return api.FilterAPI.GetFilterChanges(id)
}

// eth_getFilterLogs
func (api *XlayerHybridFilterAPI) GetFilterLogs(ctx context.Context, id rpc.ID) ([]*types.Log, error) {
	// Check if this filter is managed by Erigon
	api.filtersMu.Lock()
	isErigon := api.erigonFilters[id]
	api.filtersMu.Unlock()

	// If managed by Erigon, forward the request
	if isErigon {
		var result []*types.Log
		err := api.legacyRpc.ErigonClient.CallContext(ctx, &result, "eth_getFilterLogs", id)
		return result, err
	}

	// Otherwise, use local
	return api.FilterAPI.GetFilterLogs(ctx, id)
}

// getLogsForOverlappingRange handles the case where the query range spans the migration block
func (api *XlayerHybridFilterAPI) getLogsForOverlappingRange(ctx context.Context, crit filters.FilterCriteria) ([]*types.Log, error) {
	// Query Erigon for logs up to migration block
	erigonCrit := crit
	erigonCrit.ToBlock = big.NewInt(int64(api.legacyRpc.MigrationBlock) - 1)
	var erigonLogs []*types.Log
	err := api.legacyRpc.ErigonClient.CallContext(ctx, &erigonLogs, "eth_getLogs", erigonCrit)
	if err != nil {
		return nil, err
	}

	// Query local for logs from migration block onwards
	localCrit := crit
	localCrit.FromBlock = big.NewInt(int64(api.legacyRpc.MigrationBlock))
	localLogs, err := api.FilterAPI.GetLogs(ctx, localCrit)
	if err != nil {
		return nil, err
	}

	// Combine results
	if erigonLogs == nil {
		erigonLogs = []*types.Log{}
	}
	if localLogs == nil {
		localLogs = []*types.Log{}
	}
	return append(erigonLogs, localLogs...), nil
}

// eth_getLogs
func (api *XlayerHybridFilterAPI) GetLogs(ctx context.Context, crit filters.FilterCriteria) ([]*types.Log, error) {
	// Handle blockHash parameter (single block query)
	if crit.BlockHash != nil {
		// Try local first - FilterAPI already has complete blockHash handling logic
		result, err := api.FilterAPI.GetLogs(ctx, crit)
		if err == nil {
			return result, nil
		}

		// If local query failed with "unknown block", fallback to Erigon
		if err.Error() == "unknown block" {
			var erigonResult []*types.Log
			err = api.legacyRpc.ErigonClient.CallContext(ctx, &erigonResult, "eth_getLogs", crit)
			return erigonResult, err
		}

		// For other errors, return the error directly
		return nil, err
	}

	// Handle block range queries
	begin := rpc.LatestBlockNumber.Int64()
	if crit.FromBlock != nil {
		begin = crit.FromBlock.Int64()
	}
	end := rpc.LatestBlockNumber.Int64()
	if crit.ToBlock != nil {
		end = crit.ToBlock.Int64()
	}
	if begin > 0 && end > 0 && begin > end {
		return nil, errInvalidBlockRange
	}

	migrationBlock := int64(api.legacyRpc.MigrationBlock)

	// 1. begin and end are both earlier than migration block
	if begin < migrationBlock && end < migrationBlock {
		var result []*types.Log
		err := api.legacyRpc.ErigonClient.CallContext(ctx, &result, "eth_getLogs", crit)
		return result, err
	}

	// 2. begin and end are both later than migration block
	if begin >= migrationBlock && end >= migrationBlock {
		return api.FilterAPI.GetLogs(ctx, crit)
	}

	// 3. begin is earlier than migration block and end is later than migration block
	if begin < migrationBlock && end >= migrationBlock {
		return api.getLogsForOverlappingRange(ctx, crit)
	}

	return api.FilterAPI.GetLogs(ctx, crit)
}

func (api *XlayerHybridFilterAPI) Logs(ctx context.Context, crit filters.FilterCriteria) (*rpc.Subscription, error) {
	migrationBlock := int64(api.legacyRpc.MigrationBlock)
	if crit.FromBlock != nil && crit.FromBlock.Int64() < migrationBlock {
		log.Warn("Parameter fromBlock is earlier than migration block, overwriting fromBlock to migrationBlock", "fromBlock", crit.FromBlock.Int64(), "migrationBlock", migrationBlock)
		crit.FromBlock = big.NewInt(migrationBlock)
	}

	return api.FilterAPI.Logs(ctx, crit)
}

// WrapAPIsForXlayer wraps the standard APIs with migration-aware versions
func WrapAPIsForXlayer(apis []rpc.API, txPreExecAPI *TxPreExecAPI, config *XlayerLegacyRPCService) []rpc.API {
	if config == nil {
		return apis // No migration configured, return original APIs
	}

	// Create a map for easy lookup and replacement
	wrapped := make([]rpc.API, 0, len(apis))

	for _, api := range apis {
		switch api.Namespace {
		case "eth":
			// Check if this is a BlockChainAPI, TransactionAPI, or FilterAPI and wrap it
			switch original := api.Service.(type) {
			case *ethapi.BlockChainAPI:
				wrapped = append(wrapped, rpc.API{
					Namespace:     api.Namespace,
					Version:       api.Version,
					Service:       NewXlayerHybridBlockChainAPI(original, txPreExecAPI, config),
					Public:        api.Public,
					Authenticated: api.Authenticated,
				})
			case *ethapi.TransactionAPI:
				wrapped = append(wrapped, rpc.API{
					Namespace:     api.Namespace,
					Version:       api.Version,
					Service:       NewXlayerHybridTransactionAPI(original, config),
					Public:        api.Public,
					Authenticated: api.Authenticated,
				})
			case *filters.FilterAPI:
				wrapped = append(wrapped, rpc.API{
					Namespace:     api.Namespace,
					Version:       api.Version,
					Service:       NewXlayerHybridFilterAPI(original, config),
					Public:        api.Public,
					Authenticated: api.Authenticated,
				})
			default:
				wrapped = append(wrapped, api)
			}
		default:
			wrapped = append(wrapped, api)
		}
	}

	return wrapped
}
