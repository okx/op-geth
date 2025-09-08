# XLayer Migration RPC Routing Solution

## Executive Summary

This document outlines a comprehensive technical solution for implementing block-height-based RPC request routing in op-geth (XLayer-Op). The solution enables smooth migration from XLayer-Erigon to XLayer-Op by routing RPC requests based on a configurable block height threshold.

## Architecture Overview

### Key Components

1. **Configuration Layer**: New parameters for migration control
2. **RPC Proxy Layer**: Request routing logic based on block height
3. **Client Layer**: RPC client for XLayer-Erigon communication
4. **API Layer**: Modified RPC method handlers with routing logic

### Design Principles

- **Minimal Intrusion**: Leverage existing patterns in op-geth
- **Reusability**: Utilize existing `HistoricalRPCService` pattern
- **Maintainability**: Clear separation of concerns
- **Performance**: Efficient routing with minimal overhead

## Implementation Details

### 1. Configuration Parameters

#### Location: `eth/ethconfig/config.go`

```go
type Config struct {
    // ... existing fields ...
    
    // Migration configuration
    MigrationBlock    *uint64 `toml:",omitempty"` // Block height threshold for migration
    PPRPCUrl         string  `toml:",omitempty"` // XLayer-Erigon RPC endpoint URL
}
```

#### Location: `cmd/utils/flags.go`

```go
var (
    // Migration flags
    MigrationBlockFlag = &cli.Uint64Flag{
        Name:     "migration-block",
        Usage:    "Block height threshold for migration routing",
        Category: flags.EthCategory,
    }
    
    PPRPCUrlFlag = &cli.StringFlag{
        Name:     "pp-rpc-url",
        Usage:    "XLayer-Erigon RPC endpoint URL for pre-migration blocks",
        Category: flags.EthCategory,
    }
)
```

### 2. Backend Extension

#### Location: `eth/backend.go`

```go
type Ethereum struct {
    // ... existing fields ...
    
    migrationRPCService *rpc.Client  // RPC client for XLayer-Erigon
    migrationBlock      *uint64      // Migration block threshold
}

// Initialize migration RPC service during node startup
func New(stack *node.Node, config *ethconfig.Config) (*Ethereum, error) {
    // ... existing initialization ...
    
    // Initialize migration RPC service if configured
    if config.PPRPCUrl != "" && config.MigrationBlock != nil {
        client, err := rpc.Dial(config.PPRPCUrl)
        if err != nil {
            return nil, fmt.Errorf("failed to connect to PP RPC: %w", err)
        }
        eth.migrationRPCService = client
        eth.migrationBlock = config.MigrationBlock
        log.Info("Migration RPC service initialized", 
            "url", config.PPRPCUrl, 
            "migrationBlock", *config.MigrationBlock)
    }
    
    return eth, nil
}
```

#### Location: `eth/api_backend.go`

```go
// Add new methods to EthAPIBackend
func (b *EthAPIBackend) MigrationRPCService() *rpc.Client {
    return b.eth.migrationRPCService
}

func (b *EthAPIBackend) MigrationBlock() *uint64 {
    return b.eth.migrationBlock
}
```

### 3. API Modifications

#### Location: `internal/ethapi/backend.go`

```go
type Backend interface {
    // ... existing methods ...
    
    // Migration support
    MigrationRPCService() *rpc.Client
    MigrationBlock() *uint64
}
```

#### Location: `internal/ethapi/api.go`

```go
// Helper function to determine if request should be routed to XLayer-Erigon
func (api *BlockChainAPI) shouldRouteToPP(blockNumber *big.Int) bool {
    if api.b.MigrationRPCService() == nil || api.b.MigrationBlock() == nil {
        return false
    }
    
    if blockNumber == nil {
        return false
    }
    
    migrationBlock := new(big.Int).SetUint64(*api.b.MigrationBlock())
    return blockNumber.Cmp(migrationBlock) < 0
}

// Modified GetBlockByNumber with routing logic
func (api *BlockChainAPI) GetBlockByNumber(ctx context.Context, number rpc.BlockNumber, fullTx bool) (map[string]interface{}, error) {
    // Special handling for special block numbers
    if number < 0 {
        // Latest, pending, safe, finalized blocks always from op-geth
        return api.getBlockByNumberLocal(ctx, number, fullTx)
    }
    
    // Check if should route to PP
    blockNum := big.NewInt(int64(number))
    if api.shouldRouteToPP(blockNum) {
        var result map[string]interface{}
        err := api.b.MigrationRPCService().CallContext(ctx, &result, 
            "eth_getBlockByNumber", number, fullTx)
        if err != nil {
            return nil, fmt.Errorf("PP RPC error: %w", err)
        }
        return result, nil
    }
    
    // Default to local processing
    return api.getBlockByNumberLocal(ctx, number, fullTx)
}

// Modified GetBlockByHash with routing logic
func (api *BlockChainAPI) GetBlockByHash(ctx context.Context, hash common.Hash, fullTx bool) (map[string]interface{}, error) {
    // First, try to get block locally to check its number
    block, err := api.b.BlockByHash(ctx, hash)
    if err == nil && block != nil {
        if api.shouldRouteToPP(block.Number()) {
            var result map[string]interface{}
            err := api.b.MigrationRPCService().CallContext(ctx, &result, 
                "eth_getBlockByHash", hash, fullTx)
            if err != nil {
                return nil, fmt.Errorf("PP RPC error: %w", err)
            }
            return result, nil
        }
        return RPCMarshalBlock(ctx, block, true, fullTx, api.b.ChainConfig(), api.b)
    }
    
    // If block not found locally, try PP if configured
    if api.b.MigrationRPCService() != nil {
        var result map[string]interface{}
        err := api.b.MigrationRPCService().CallContext(ctx, &result, 
            "eth_getBlockByHash", hash, fullTx)
        if err == nil {
            return result, nil
        }
    }
    
    return nil, err
}

// Modified GetStorageAt with routing logic
func (api *BlockChainAPI) GetStorageAt(ctx context.Context, address common.Address, hexKey string, blockNrOrHash rpc.BlockNumberOrHash) (hexutil.Bytes, error) {
    header, err := headerByNumberOrHash(ctx, api.b, blockNrOrHash)
    if err != nil {
        return nil, err
    }
    
    // Check migration routing
    if api.shouldRouteToPP(header.Number) {
        var res hexutil.Bytes
        err := api.b.MigrationRPCService().CallContext(ctx, &res, 
            "eth_getStorageAt", address, hexKey, blockNrOrHash)
        if err != nil {
            return nil, fmt.Errorf("PP RPC error: %w", err)
        }
        return res, nil
    }
    
    // Default to local processing
    return api.getStorageAtLocal(ctx, address, hexKey, blockNrOrHash)
}

// Modified GetLogs with routing logic
func (api *FilterAPI) GetLogs(ctx context.Context, crit FilterCriteria) ([]*types.Log, error) {
    // Determine block range
    var fromBlock, toBlock *big.Int
    
    if crit.FromBlock != nil {
        fromBlock = crit.FromBlock.Int
    }
    if crit.ToBlock != nil {
        toBlock = crit.ToBlock.Int
    }
    
    // Check if entire range is before migration block
    if api.b.MigrationRPCService() != nil && api.b.MigrationBlock() != nil {
        migrationBlock := new(big.Int).SetUint64(*api.b.MigrationBlock())
        
        // If entire range is before migration, route to PP
        if toBlock != nil && toBlock.Cmp(migrationBlock) < 0 {
            var result []*types.Log
            err := api.b.MigrationRPCService().CallContext(ctx, &result, 
                "eth_getLogs", crit)
            if err != nil {
                return nil, fmt.Errorf("PP RPC error: %w", err)
            }
            return result, nil
        }
        
        // If range spans migration block, need to split query
        if fromBlock != nil && fromBlock.Cmp(migrationBlock) < 0 && 
           (toBlock == nil || toBlock.Cmp(migrationBlock) >= 0) {
            // Query PP for pre-migration logs
            ppCrit := crit
            ppCrit.ToBlock = (*rpc.BlockNumber)(migrationBlock.Uint64() - 1)
            
            var ppLogs []*types.Log
            err := api.b.MigrationRPCService().CallContext(ctx, &ppLogs, 
                "eth_getLogs", ppCrit)
            if err != nil {
                return nil, fmt.Errorf("PP RPC error: %w", err)
            }
            
            // Query local for post-migration logs
            localCrit := crit
            localCrit.FromBlock = migrationBlock
            localLogs, err := api.getLogsLocal(ctx, localCrit)
            if err != nil {
                return nil, err
            }
            
            // Combine results
            return append(ppLogs, localLogs...), nil
        }
    }
    
    // Default to local processing
    return api.getLogsLocal(ctx, crit)
}
```

#### Location: `internal/ethapi/transaction_api.go`

```go
// Modified GetTransactionByHash with routing logic
func (api *TransactionAPI) GetTransactionByHash(ctx context.Context, hash common.Hash) (*RPCTransaction, error) {
    // Try to get transaction locally first
    found, tx, blockHash, blockNumber, index := api.b.GetCanonicalTransaction(hash)
    
    if found {
        // Check if transaction is in a pre-migration block
        if api.shouldRouteToPP(new(big.Int).SetUint64(blockNumber)) {
            var result *RPCTransaction
            err := api.b.MigrationRPCService().CallContext(ctx, &result, 
                "eth_getTransactionByHash", hash)
            if err != nil {
                return nil, fmt.Errorf("PP RPC error: %w", err)
            }
            return result, nil
        }
        
        // Process locally
        header, err := api.b.HeaderByHash(ctx, blockHash)
        if err != nil {
            return nil, err
        }
        return NewRPCTransaction(tx, blockHash, blockNumber, header.Time, index, 
            header.BaseFee, api.b.ChainConfig(), header.Difficulty), nil
    }
    
    // Check pool
    if tx := api.b.GetPoolTransaction(hash); tx != nil {
        return NewRPCPendingTransaction(tx, api.b.CurrentHeader(), api.b.ChainConfig()), nil
    }
    
    // Try PP if configured and not found locally
    if api.b.MigrationRPCService() != nil {
        var result *RPCTransaction
        err := api.b.MigrationRPCService().CallContext(ctx, &result, 
            "eth_getTransactionByHash", hash)
        if err == nil {
            return result, nil
        }
    }
    
    // Transaction not found
    if !api.b.TxIndexDone() {
        return nil, NewTxIndexingError()
    }
    return nil, nil
}

// Modified GetTransactionReceipt with routing logic
func (api *TransactionAPI) GetTransactionReceipt(ctx context.Context, hash common.Hash) (map[string]interface{}, error) {
    found, tx, blockHash, blockNumber, index := api.b.GetCanonicalTransaction(hash)
    
    if !found {
        // Try PP if configured
        if api.b.MigrationRPCService() != nil {
            var result map[string]interface{}
            err := api.b.MigrationRPCService().CallContext(ctx, &result, 
                "eth_getTransactionReceipt", hash)
            if err == nil {
                return result, nil
            }
        }
        
        if !api.b.TxIndexDone() {
            return nil, NewTxIndexingError()
        }
        return nil, nil
    }
    
    // Check if receipt is in a pre-migration block
    if api.shouldRouteToPP(new(big.Int).SetUint64(blockNumber)) {
        var result map[string]interface{}
        err := api.b.MigrationRPCService().CallContext(ctx, &result, 
            "eth_getTransactionReceipt", hash)
        if err != nil {
            return nil, fmt.Errorf("PP RPC error: %w", err)
        }
        return result, nil
    }
    
    // Process receipt locally
    return api.getTransactionReceiptLocal(ctx, hash)
}
```

### 4. Fallback Routing for Unimplemented Methods

#### Location: `node/rpcstack.go`

```go
// Add fallback handler for unimplemented methods
type MigrationFallbackHandler struct {
    migrationClient *rpc.Client
    localHandler    http.Handler
}

func (h *MigrationFallbackHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // Parse JSON-RPC request to check method
    var req map[string]interface{}
    if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
        method, ok := req["method"].(string)
        if ok && h.shouldFallbackToPP(method) {
            // Forward to PP RPC
            h.forwardToPP(w, req)
            return
        }
    }
    
    // Default to local handler
    h.localHandler.ServeHTTP(w, r)
}

func (h *MigrationFallbackHandler) shouldFallbackToPP(method string) bool {
    // List of methods that should fallback to PP if not implemented
    fallbackMethods := map[string]bool{
        "debug_traceTransaction": true,
        "debug_traceBlockByNumber": true,
        "debug_traceBlockByHash": true,
        // Add more methods as needed
    }
    return fallbackMethods[method]
}
```

### 5. Command Line Integration

#### Location: `cmd/geth/main.go`

```go
// Add migration flags to node flags
var nodeFlags = slices.Concat([]cli.Flag{
    // ... existing flags ...
    utils.MigrationBlockFlag,
    utils.PPRPCUrlFlag,
}, existingFlags)
```

#### Location: `cmd/geth/config.go`

```go
func makeFullNode(ctx *cli.Context) *node.Node {
    stack, cfg := makeConfigNode(ctx)
    
    // ... existing configuration ...
    
    // Set migration configuration
    if ctx.IsSet(utils.MigrationBlockFlag.Name) {
        v := ctx.Uint64(utils.MigrationBlockFlag.Name)
        cfg.Eth.MigrationBlock = &v
    }
    
    if ctx.IsSet(utils.PPRPCUrlFlag.Name) {
        cfg.Eth.PPRPCUrl = ctx.String(utils.PPRPCUrlFlag.Name)
    }
    
    // ... rest of function ...
}
```

## Usage Example

### Configuration File (TOML)

```toml
[Eth]
# Migration configuration
MigrationBlock = 1000000
PPRPCUrl = "http://xlayer-erigon-node:8545"
```

### Command Line

```bash
geth \
  --migration-block 1000000 \
  --pp-rpc-url "http://xlayer-erigon-node:8545" \
  --http \
  --http.api eth,net,web3,txpool
```

## Testing Strategy

### Unit Tests

1. **Configuration Tests**: Verify migration parameters are correctly loaded
2. **Routing Logic Tests**: Test block height comparison and routing decisions
3. **API Tests**: Mock RPC calls to verify correct routing behavior

### Integration Tests

1. **End-to-End Tests**: Set up both op-geth and mock Erigon RPC server
2. **Boundary Tests**: Test behavior at migration block boundary
3. **Error Handling**: Test failures in PP RPC connection

### Performance Tests

1. **Latency Tests**: Measure overhead of routing logic
2. **Load Tests**: Verify system behavior under high request volume
3. **Failover Tests**: Test behavior when PP RPC is unavailable

## Rollout Plan

### Phase 1: Development
- Implement configuration parameters
- Add routing logic to affected RPC methods
- Create unit tests

### Phase 2: Testing
- Deploy to test environment
- Run integration tests
- Performance benchmarking

### Phase 3: Staging
- Deploy to staging with production-like data
- Monitor routing behavior
- Validate data consistency

### Phase 4: Production
- Gradual rollout with monitoring
- Real-time validation of responses
- Rollback plan if issues detected

## Monitoring and Observability

### Metrics to Track

1. **Request Routing Metrics**
   - Total requests routed to PP
   - Total requests handled locally
   - Routing decision time

2. **Performance Metrics**
   - PP RPC response time
   - Local processing time
   - End-to-end latency

3. **Error Metrics**
   - PP RPC failures
   - Routing errors
   - Fallback triggers

### Logging

```go
// Add detailed logging for debugging
log.Debug("Routing RPC request", 
    "method", method,
    "blockNumber", blockNumber,
    "routedToPP", routedToPP,
    "migrationBlock", migrationBlock)
```

## Security Considerations

1. **RPC Authentication**: Ensure PP RPC connection uses appropriate authentication
2. **TLS/SSL**: Use encrypted connections for PP RPC communication
3. **Rate Limiting**: Implement rate limiting to prevent abuse
4. **Input Validation**: Validate all inputs before routing

## Maintenance and Updates

1. **Configuration Updates**: Support dynamic configuration updates without restart
2. **Health Checks**: Regular health checks for PP RPC connection
3. **Graceful Degradation**: Fallback to local processing if PP unavailable
4. **Version Compatibility**: Ensure compatibility between op-geth and Erigon RPC versions

## Conclusion

This solution provides a robust, maintainable approach to implementing migration logic in op-geth. By leveraging existing patterns and infrastructure, we minimize complexity while ensuring reliable request routing based on block height. The phased rollout plan and comprehensive monitoring ensure safe deployment to production.
