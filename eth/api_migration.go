package eth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/eth/filters"
	"github.com/ethereum/go-ethereum/internal/ethapi"
	"github.com/ethereum/go-ethereum/rpc"
)

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
	if config.MigrationBlock == nil || config.PPRPCUrl == "" {
		return nil, nil // Migration not configured
	}

	timeout := config.PPRPCTimeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	erigonClient, err := rpc.DialContext(ctx, config.PPRPCUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to erigon RPC: %w", err)
	}

	return &MigrationConfig{
		MigrationBlock: *config.MigrationBlock,
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

	// For hash-based queries or recent blocks, use local
	return api.BlockChainAPI.GetStorageAt(ctx, address, hexKey, blockNrOrHash)
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
}

// NewMigrationFilterAPI creates a new migration-aware FilterAPI
func NewMigrationFilterAPI(original *filters.FilterAPI, config *MigrationConfig) *MigrationFilterAPI {
	return &MigrationFilterAPI{
		FilterAPI: original,
		config:    config,
	}
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
		crit.ToBlock = big.NewInt(int64(api.config.MigrationBlock))
		var result []*types.Log
		err := api.config.ErigonClient.CallContext(ctx, &result, "eth_getLogs", crit)
		if err != nil || result == nil {
			return nil, err
		}

		localResult, err := api.FilterAPI.GetLogs(ctx, crit)
		if err != nil || localResult == nil {
			return nil, err
		}
		return append(result, localResult...), nil
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
			default:
				wrapped = append(wrapped, api)
			}
		default:
			wrapped = append(wrapped, api)
		}
	}

	return wrapped
}

// RegisterFallbackMethods adds a fallback service for unimplemented methods
// This allows forwarding of unregistered RPC calls to Erigon
func RegisterFallbackMethods(apis []rpc.API, config *MigrationConfig) []rpc.API {
	if config == nil || config.ErigonClient == nil {
		return apis
	}

	// Create the fallback service
	fallback := &FallbackService{
		config: config,
	}

	// Add a special API that can handle fallback calls
	// Note: This is a simplified implementation. A full implementation would
	// require modifying the RPC server to support dynamic method routing.
	apis = append(apis, rpc.API{
		Namespace: "fallback",
		Service:   fallback,
	})

	return apis
}

// FallbackService handles forwarding of unregistered methods to Erigon
type FallbackService struct {
	config *MigrationConfig
}

// ForwardCall is a generic method that forwards calls to Erigon
// This can be called explicitly for methods not implemented in op-geth
func (s *FallbackService) ForwardCall(ctx context.Context, method string, params []interface{}) (json.RawMessage, error) {
	if s.config == nil || s.config.ErigonClient == nil {
		return nil, fmt.Errorf("fallback not configured")
	}

	var result json.RawMessage
	err := s.config.ErigonClient.CallContext(ctx, &result, method, params...)
	if err != nil {
		return nil, fmt.Errorf("fallback call failed: %w", err)
	}

	return result, nil
}
