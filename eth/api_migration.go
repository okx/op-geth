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
	"github.com/ethereum/go-ethereum/rpc"
)

// Policy
// LOCAL
// 1. Local first
// 2. If not found, and erigon configured, forward request
// FORWARD
// 1. If block number is earlier than configured, forward request
// 2. Otherwise, use local

var (
	errInvalidBlockRange = errors.New("invalid block range params")
)

// MigrationConfig holds the configuration for RPC migration
type MigrationConfig struct {
	MigrationBlock uint64
	ErigonClient   *rpc.Client
}

// NewMigrationConfig creates a new migration configuration
func NewMigrationConfig(config *ethconfig.Config) (*MigrationConfig, error) {
	if config.XLayer.RpcMigration.MigrationBlock == nil || config.XLayer.RpcMigration.PPRPCUrl == "" {
		return nil, nil // Migration not configured
	}

	timeout := config.XLayer.RpcMigration.PPRPCTimeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	erigonClient, err := rpc.DialContext(ctx, config.XLayer.RpcMigration.PPRPCUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to erigon RPC: %w", err)
	}

	return &MigrationConfig{
		MigrationBlock: *config.XLayer.RpcMigration.MigrationBlock,
		ErigonClient:   erigonClient,
	}, nil
}

// Close closes the erigon RPC client
func (mc *MigrationConfig) Close() {
	if mc != nil && mc.ErigonClient != nil {
		mc.ErigonClient.Close()
	}
}

// shouldProxy determines if a request should be proxied based on block number
func (mc *MigrationConfig) shouldProxy(blockNumber uint64) bool {
	return mc != nil && mc.MigrationBlock > 0 && blockNumber < mc.MigrationBlock
}

// MigrationBlockChainAPI wraps the standard BlockChainAPI to add migration routing
type MigrationBlockChainAPI struct {
	*ethapi.BlockChainAPI
	config *MigrationConfig
}

// NewMigrationBlockChainAPI creates a new migration-aware BlockChainAPI
func NewMigrationBlockChainAPI(original *ethapi.BlockChainAPI, config *MigrationConfig) *MigrationBlockChainAPI {
	return &MigrationBlockChainAPI{
		BlockChainAPI: original,
		config:        config,
	}
}

// GetBlockByNumber returns the block for the given block number
func (api *MigrationBlockChainAPI) GetBlockByNumber(ctx context.Context, number rpc.BlockNumber, fullTx bool) (map[string]interface{}, error) {
	// Handle special block numbers (latest, pending, etc.)
	if number < 0 {
		return api.BlockChainAPI.GetBlockByNumber(ctx, number, fullTx)
	}

	// Check if we should proxy to erigon
	if api.config != nil && api.config.shouldProxy(uint64(number)) {
		var result map[string]interface{}
		err := api.config.ErigonClient.CallContext(ctx, &result, "eth_getBlockByNumber", hexutil.Uint64(number), fullTx)
		return result, err
	}

	// Handle locally
	return api.BlockChainAPI.GetBlockByNumber(ctx, number, fullTx)
}

// GetBlockByHash returns the block for the given block hash
func (api *MigrationBlockChainAPI) GetBlockByHash(ctx context.Context, hash common.Hash, fullTx bool) (map[string]interface{}, error) {
	// Try local first
	result, err := api.BlockChainAPI.GetBlockByHash(ctx, hash, fullTx)
	if err == nil && result != nil {
		return result, nil
	}

	// If not found locally and migration is configured, try erigon
	if api.config != nil && api.config.ErigonClient != nil {
		var remoteResult map[string]interface{}
		err := api.config.ErigonClient.CallContext(ctx, &remoteResult, "eth_getBlockByHash", hash, fullTx)
		if err == nil && remoteResult != nil {
			return remoteResult, nil
		}
	}

	return result, err
}

// GetStorageAt returns the storage value at the given address and key
func (api *MigrationBlockChainAPI) GetStorageAt(ctx context.Context, address common.Address, hexKey string, blockNrOrHash rpc.BlockNumberOrHash) (hexutil.Bytes, error) {
	// Try to determine the block number
	if blockNr, ok := blockNrOrHash.Number(); ok && blockNr >= 0 {
		if api.config != nil && api.config.shouldProxy(uint64(blockNr)) {
			var result hexutil.Bytes
			err := api.config.ErigonClient.CallContext(ctx, &result, "eth_getStorageAt", address, hexKey, blockNrOrHash)
			return result, err
		}
	}

	// For hash-based queries or recent blocks, try local first, if not found attempt to proxy to erigon
	result, err := api.BlockChainAPI.GetStorageAt(ctx, address, hexKey, blockNrOrHash)
	if err == nil && result != nil {
		return result, nil
	}

	if api.config != nil && api.config.ErigonClient != nil {
		var remoteResult hexutil.Bytes
		err := api.config.ErigonClient.CallContext(ctx, &remoteResult, "eth_getStorageAt", address, hexKey, blockNrOrHash)
		return remoteResult, err
	}

	return result, err
}

// MigrationTransactionAPI wraps the standard TransactionAPI to add migration routing
type MigrationTransactionAPI struct {
	*ethapi.TransactionAPI
	config *MigrationConfig
}

// NewMigrationTransactionAPI creates a new migration-aware TransactionAPI
func NewMigrationTransactionAPI(original *ethapi.TransactionAPI, config *MigrationConfig) *MigrationTransactionAPI {
	return &MigrationTransactionAPI{
		TransactionAPI: original,
		config:         config,
	}
}

// GetTransactionByHash returns the transaction for the given hash
func (api *MigrationTransactionAPI) GetTransactionByHash(ctx context.Context, hash common.Hash) (*ethapi.RPCTransaction, error) {
	// Try local first
	tx, err := api.TransactionAPI.GetTransactionByHash(ctx, hash)
	if err == nil && tx != nil {
		return tx, nil
	}

	// If not found locally and migration is configured, try erigon
	if api.config != nil && api.config.ErigonClient != nil {
		var result *ethapi.RPCTransaction
		err := api.config.ErigonClient.CallContext(ctx, &result, "eth_getTransactionByHash", hash)
		if err == nil && result != nil {
			return result, nil
		}
	}

	return tx, err
}

// GetTransactionReceipt returns the receipt for the given transaction hash
func (api *MigrationTransactionAPI) GetTransactionReceipt(ctx context.Context, hash common.Hash) (map[string]interface{}, error) {
	// Try local first
	receipt, err := api.TransactionAPI.GetTransactionReceipt(ctx, hash)
	if err == nil && receipt != nil {
		return receipt, nil
	}

	// If not found locally and migration is configured, try erigon
	if api.config != nil && api.config.ErigonClient != nil {
		var result map[string]interface{}
		err := api.config.ErigonClient.CallContext(ctx, &result, "eth_getTransactionReceipt", hash)
		if err == nil && result != nil {
			return result, nil
		}
	}

	return receipt, err
}

type MigrationFilterAPI struct {
	*filters.FilterAPI
	config *MigrationConfig
	// Track which filters are managed by erigon
	erigonFilters map[rpc.ID]bool
	filtersMu     sync.Mutex
}

// NewMigrationFilterAPI creates a new migration-aware FilterAPI
func NewMigrationFilterAPI(original *filters.FilterAPI, config *MigrationConfig) *MigrationFilterAPI {
	return &MigrationFilterAPI{
		FilterAPI:     original,
		config:        config,
		erigonFilters: make(map[rpc.ID]bool),
	}
}

// eth_newFilter
// eth_uninstallFilter
// eth_getFilterChanges
// eth_getFilterLogs
// If range overlaps the migration_block, return error, else FORWARD

func (api *MigrationFilterAPI) NewFilter(crit filters.FilterCriteria) (rpc.ID, error) {
	// If migration is not configured, use local
	if api.config == nil || api.config.ErigonClient == nil {
		return api.FilterAPI.NewFilter(crit)
	}

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

	migrationBlock := int64(api.config.MigrationBlock)

	// Check if range overlaps with migration block
	// Case 1: Both begin and end are before migration block -> forward to Erigon
	if begin >= 0 && end >= 0 && end < migrationBlock {
		var id rpc.ID
		err := api.config.ErigonClient.Call(&id, "eth_newFilter", crit)
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
			api.config.MigrationBlock, begin, end)
	}

	// Handle special block numbers (latest, pending) -> use local
	if begin < 0 || end < 0 {
		return api.FilterAPI.NewFilter(crit)
	}

	// Default to local
	return api.FilterAPI.NewFilter(crit)
}

func (api *MigrationFilterAPI) UninstallFilter(id rpc.ID) bool {
	// Check if this filter is managed by Erigon
	api.filtersMu.Lock()
	isErigon := api.erigonFilters[id]
	if isErigon {
		delete(api.erigonFilters, id)
	}
	api.filtersMu.Unlock()

	// If managed by Erigon, forward the uninstall request
	if isErigon && api.config != nil && api.config.ErigonClient != nil {
		var result bool
		err := api.config.ErigonClient.Call(&result, "eth_uninstallFilter", id)
		if err != nil {
			// Log the error but still return false
			return false
		}
		return result
	}

	// Otherwise, use local
	return api.FilterAPI.UninstallFilter(id)
}

func (api *MigrationFilterAPI) GetFilterChanges(id rpc.ID) (interface{}, error) {
	// Check if this filter is managed by Erigon
	api.filtersMu.Lock()
	isErigon := api.erigonFilters[id]
	api.filtersMu.Unlock()

	// If managed by Erigon, forward the request
	if isErigon && api.config != nil && api.config.ErigonClient != nil {
		var result interface{}
		err := api.config.ErigonClient.Call(&result, "eth_getFilterChanges", id)
		return result, err
	}

	// Otherwise, use local
	return api.FilterAPI.GetFilterChanges(id)
}

func (api *MigrationFilterAPI) GetFilterLogs(ctx context.Context, id rpc.ID) ([]*types.Log, error) {
	// Check if this filter is managed by Erigon
	api.filtersMu.Lock()
	isErigon := api.erigonFilters[id]
	api.filtersMu.Unlock()

	// If managed by Erigon, forward the request
	if isErigon && api.config != nil && api.config.ErigonClient != nil {
		var result []*types.Log
		err := api.config.ErigonClient.CallContext(ctx, &result, "eth_getFilterLogs", id)
		return result, err
	}

	// Otherwise, use local
	return api.FilterAPI.GetFilterLogs(ctx, id)
}

func (api *MigrationFilterAPI) GetLogs(ctx context.Context, crit filters.FilterCriteria) ([]*types.Log, error) {
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

	// 1. begin and end are both earlier than migration block
	if begin < int64(api.config.MigrationBlock) && end < int64(api.config.MigrationBlock) {
		var result []*types.Log
		err := api.config.ErigonClient.CallContext(ctx, &result, "eth_getLogs", crit)
		return result, err
	}

	// 2. begin and end are both later than migration block
	if begin >= int64(api.config.MigrationBlock) && end >= int64(api.config.MigrationBlock) {
		return api.FilterAPI.GetLogs(ctx, crit)
	}

	// 3. begin is earlier than migration block and end is later than migration block
	if begin < int64(api.config.MigrationBlock) && end >= int64(api.config.MigrationBlock) {
		// Query Erigon for logs up to migration block
		erigonCrit := crit
		erigonCrit.ToBlock = big.NewInt(int64(api.config.MigrationBlock) - 1)
		var erigonLogs []*types.Log
		err := api.config.ErigonClient.CallContext(ctx, &erigonLogs, "eth_getLogs", erigonCrit)
		if err != nil {
			return nil, err
		}

		// Query local for logs from migration block onwards
		localCrit := crit
		localCrit.FromBlock = big.NewInt(int64(api.config.MigrationBlock))
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

	return api.FilterAPI.GetLogs(ctx, crit)
}

// WrapAPIsForMigration wraps the standard APIs with migration-aware versions
func WrapAPIsForMigration(apis []rpc.API, config *MigrationConfig) []rpc.API {
	if config == nil {
		return apis // No migration configured, return original APIs
	}

	// Create a map for easy lookup and replacement
	wrapped := make([]rpc.API, 0, len(apis))

	for _, api := range apis {
		switch api.Namespace {
		case "eth":
			// Check if this is a BlockChainAPI, TransactionAPI or FilterAPI and wrap it
			switch original := api.Service.(type) {
			case *ethapi.BlockChainAPI:
				wrapped = append(wrapped, rpc.API{
					Namespace:     api.Namespace,
					Version:       api.Version,
					Service:       NewMigrationBlockChainAPI(original, config),
					Public:        api.Public,
					Authenticated: api.Authenticated,
				})
			case *ethapi.TransactionAPI:
				wrapped = append(wrapped, rpc.API{
					Namespace:     api.Namespace,
					Version:       api.Version,
					Service:       NewMigrationTransactionAPI(original, config),
					Public:        api.Public,
					Authenticated: api.Authenticated,
				})
			case *filters.FilterAPI:
				wrapped = append(wrapped, rpc.API{
					Namespace:     api.Namespace,
					Version:       api.Version,
					Service:       NewMigrationFilterAPI(original, config),
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
