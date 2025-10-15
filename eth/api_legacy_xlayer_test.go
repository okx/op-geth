package eth

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/internal/ethapi"
	"github.com/ethereum/go-ethereum/rpc"
)

// Mock implementations for testing
type mockBlockChainAPI struct{}

func (m *mockBlockChainAPI) GetBlockByNumber(ctx context.Context, number rpc.BlockNumber, fullTx bool) (map[string]interface{}, error) {
	// Return nil to simulate block not found locally
	return nil, nil
}

func (m *mockBlockChainAPI) GetBlockByHash(ctx context.Context, hash common.Hash, fullTx bool) (map[string]interface{}, error) {
	// Return nil to simulate block not found locally
	return nil, nil
}

func (m *mockBlockChainAPI) GetStorageAt(ctx context.Context, address common.Address, hexKey string, blockNrOrHash rpc.BlockNumberOrHash) (hexutil.Bytes, error) {
	// Return empty storage
	return hexutil.Bytes{}, nil
}

type mockTransactionAPI struct{}

func (m *mockTransactionAPI) GetTransactionByHash(ctx context.Context, hash common.Hash) (*ethapi.RPCTransaction, error) {
	// Return nil to simulate transaction not found locally
	return nil, nil
}

func (m *mockTransactionAPI) GetTransactionReceipt(ctx context.Context, hash common.Hash) (map[string]interface{}, error) {
	// Return nil to simulate receipt not found locally
	return nil, nil
}

// Mock Erigon RPC service
type mockErigonService struct {
	blocks       map[uint64]*types.Block
	transactions map[common.Hash]*types.Transaction
	receipts     map[common.Hash]*types.Receipt
	storage      map[string]hexutil.Bytes
	logs         []*types.Log
}

func newMockErigonService() *mockErigonService {
	return &mockErigonService{
		blocks:       make(map[uint64]*types.Block),
		transactions: make(map[common.Hash]*types.Transaction),
		receipts:     make(map[common.Hash]*types.Receipt),
		storage:      make(map[string]hexutil.Bytes),
		logs:         make([]*types.Log, 0),
	}
}

// Mock methods for eth_* RPC calls
func (s *mockErigonService) GetBlockByNumber(number hexutil.Uint64, fullTx bool) (map[string]interface{}, error) {
	block, ok := s.blocks[uint64(number)]
	if !ok {
		return nil, nil
	}

	// Convert block to RPC format
	result := map[string]interface{}{
		"number":           hexutil.Uint64(block.NumberU64()),
		"hash":             block.Hash(),
		"parentHash":       block.ParentHash(),
		"nonce":            block.Nonce(),
		"mixHash":          block.MixDigest(),
		"sha3Uncles":       block.UncleHash(),
		"logsBloom":        block.Bloom(),
		"stateRoot":        block.Root(),
		"miner":            block.Coinbase(),
		"difficulty":       (*hexutil.Big)(block.Difficulty()),
		"extraData":        hexutil.Bytes(block.Extra()),
		"size":             hexutil.Uint64(block.Size()),
		"gasLimit":         hexutil.Uint64(block.GasLimit()),
		"gasUsed":          hexutil.Uint64(block.GasUsed()),
		"timestamp":        hexutil.Uint64(block.Time()),
		"transactionsRoot": block.TxHash(),
		"receiptsRoot":     block.ReceiptHash(),
	}

	if fullTx {
		txs := make([]interface{}, len(block.Transactions()))
		for i, tx := range block.Transactions() {
			txs[i] = map[string]interface{}{
				"hash":  tx.Hash(),
				"from":  common.HexToAddress("0x1234567890123456789012345678901234567890"),
				"to":    tx.To(),
				"value": (*hexutil.Big)(tx.Value()),
				"nonce": hexutil.Uint64(tx.Nonce()),
			}
		}
		result["transactions"] = txs
	} else {
		txs := make([]common.Hash, len(block.Transactions()))
		for i, tx := range block.Transactions() {
			txs[i] = tx.Hash()
		}
		result["transactions"] = txs
	}

	return result, nil
}

func (s *mockErigonService) GetBlockByHash(hash common.Hash, fullTx bool) (map[string]interface{}, error) {
	// Find block by hash
	for _, block := range s.blocks {
		if block.Hash() == hash {
			return s.GetBlockByNumber(hexutil.Uint64(block.NumberU64()), fullTx)
		}
	}
	return nil, nil
}

func (s *mockErigonService) GetStorageAt(address common.Address, key string, blockNrOrHash interface{}) (hexutil.Bytes, error) {
	storageKey := fmt.Sprintf("%s:%s", address.Hex(), key)
	if val, ok := s.storage[storageKey]; ok {
		return val, nil
	}
	return hexutil.Bytes{}, nil
}

func (s *mockErigonService) GetTransactionByHash(hash common.Hash) (*ethapi.RPCTransaction, error) {
	tx, ok := s.transactions[hash]
	if !ok {
		return nil, nil
	}

	// Create a minimal RPCTransaction
	return &ethapi.RPCTransaction{
		Hash:  tx.Hash(),
		From:  common.HexToAddress("0x1234567890123456789012345678901234567890"),
		To:    tx.To(),
		Value: (*hexutil.Big)(tx.Value()),
		Nonce: hexutil.Uint64(tx.Nonce()),
	}, nil
}

func (s *mockErigonService) GetTransactionReceipt(hash common.Hash) (map[string]interface{}, error) {
	receipt, ok := s.receipts[hash]
	if !ok {
		return nil, nil
	}

	return map[string]interface{}{
		"transactionHash": hash,
		"blockNumber":     hexutil.Uint64(receipt.BlockNumber.Uint64()),
		"status":          hexutil.Uint64(receipt.Status),
		"gasUsed":         hexutil.Uint64(receipt.GasUsed),
	}, nil
}

func (s *mockErigonService) GetLogs(crit interface{}) ([]*types.Log, error) {
	// Return all logs for simplicity in testing
	return s.logs, nil
}

// createMockErigonServer creates an httptest server that simulates an Erigon RPC endpoint
func createMockErigonServer(t *testing.T) (*httptest.Server, *mockErigonService) {
	service := newMockErigonService()

	// Create RPC server
	rpcServer := rpc.NewServer()
	if err := rpcServer.RegisterName("eth", service); err != nil {
		t.Fatalf("Failed to register service: %v", err)
	}

	// Create HTTP server
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rpcServer.ServeHTTP(w, r)
	}))

	return httpServer, service
}

// Test NewXlayerLegacyRPCService
func TestNewMigrationConfig(t *testing.T) {
	t.Parallel()

	// Test case 1: No migration configured
	config1 := &ethconfig.Config{}
	mc1, err := NewXlayerLegacyRPCService(config1)
	if err != nil {
		t.Errorf("Unexpected error for empty legacyRpc: %v", err)
	}
	if mc1 != nil {
		t.Error("Expected nil XlayerLegacyRPCService when not configured")
	}

	// Test case 2: Migration configured with valid URL
	server, _ := createMockErigonServer(t)
	defer server.Close()

	migrationBlock := uint64(100)
	config2 := &ethconfig.Config{XLayer: ethconfig.XLayerConfig{LegacyPp: ethconfig.MigrationConfig{
		MigrationBlock: &migrationBlock,
		PPRPCUrl:       server.URL,
		PPRPCTimeout:   5 * time.Second,
	}}}

	mc2, err := NewXlayerLegacyRPCService(config2)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if mc2 == nil {
		t.Fatal("Expected non-nil XlayerLegacyRPCService")
	}
	if mc2.MigrationBlock != migrationBlock {
		t.Errorf("MigrationBlock mismatch: got %d, want %d", mc2.MigrationBlock, migrationBlock)
	}
	if mc2.ErigonClient == nil {
		t.Error("Expected non-nil ErigonClient")
	}
	defer mc2.Close()

	// Test case 3: Invalid URL
	migrationBlock3 := uint64(100)
	config3 := &ethconfig.Config{
		XLayer: ethconfig.XLayerConfig{LegacyPp: ethconfig.MigrationConfig{
			MigrationBlock: &migrationBlock3,
			PPRPCUrl:       "invalid://url",
			PPRPCTimeout:   1 * time.Second,
		}}}

	mc3, err := NewXlayerLegacyRPCService(config3)
	if err == nil {
		t.Error("Expected error for invalid URL")
	}
	if mc3 != nil {
		t.Error("Expected nil XlayerLegacyRPCService on error")
	}
}

// Test XlayerHybridBlockChainAPI.GetBlockByNumber
func TestMigrationBlockChainAPI_GetBlockByNumber(t *testing.T) {
	t.Parallel()

	// Setup mock Erigon server
	server, erigonService := createMockErigonServer(t)
	defer server.Close()

	// Create test blocks for Erigon (blocks 0-99)
	for i := uint64(0); i < 100; i++ {
		block := types.NewBlockWithHeader(&types.Header{
			Number:     big.NewInt(int64(i)),
			ParentHash: common.Hash{},
			Time:       uint64(time.Now().Unix()),
			Difficulty: big.NewInt(1),
			GasLimit:   8000000,
		})
		erigonService.blocks[i] = block
	}

	// Create migration legacyRpc
	client, _ := rpc.Dial(server.URL)
	migrationBlock := uint64(100)
	config := &XlayerLegacyRPCService{
		MigrationBlock: migrationBlock,
		ErigonClient:   client,
	}
	defer config.Close()

	// Note: We can't easily test the full GetBlockByNumber without a complete backend
	// But we can test the routing logic through shouldProxy

	// Test shouldProxy logic
	if !config.shouldProxy(50) {
		t.Error("Block 50 should be proxied")
	}
	if config.shouldProxy(100) {
		t.Error("Block 100 should not be proxied")
	}
	if config.shouldProxy(150) {
		t.Error("Block 150 should not be proxied")
	}

	// Verify we can call Erigon directly
	ctx := context.Background()
	var result map[string]interface{}
	err := config.ErigonClient.CallContext(ctx, &result, "eth_getBlockByNumber", hexutil.Uint64(50), false)
	if err != nil {
		t.Errorf("Failed to call Erigon directly: %v", err)
	}
	if result == nil {
		t.Error("Expected non-nil result from Erigon")
	}
}

// Test XlayerHybridBlockChainAPI.GetBlockByHash
func TestMigrationBlockChainAPI_GetBlockByHash(t *testing.T) {
	t.Parallel()

	// Setup mock Erigon server
	server, erigonService := createMockErigonServer(t)
	defer server.Close()

	// Create a test block in Erigon
	erigonBlock := types.NewBlockWithHeader(&types.Header{
		Number:     big.NewInt(50),
		ParentHash: common.Hash{},
		Time:       uint64(time.Now().Unix()),
		Difficulty: big.NewInt(1),
		GasLimit:   8000000,
	})
	erigonService.blocks[50] = erigonBlock

	// Create migration legacyRpc
	client, _ := rpc.Dial(server.URL)
	config := &XlayerLegacyRPCService{
		MigrationBlock: 100,
		ErigonClient:   client,
	}
	defer config.Close()

	// Test that we can call Erigon to get block by hash
	ctx := context.Background()
	var result map[string]interface{}
	err := config.ErigonClient.CallContext(ctx, &result, "eth_getBlockByHash", erigonBlock.Hash(), false)
	if err != nil {
		t.Errorf("Failed to call Erigon directly: %v", err)
	}
	if result == nil {
		t.Error("Expected non-nil result from Erigon")
	}

	// Test non-existent block
	nonExistentHash := common.HexToHash("0x1234567890abcdef")
	var nilResult map[string]interface{}
	err = config.ErigonClient.CallContext(ctx, &nilResult, "eth_getBlockByHash", nonExistentHash, false)
	if err != nil {
		t.Errorf("Unexpected error for non-existent block: %v", err)
	}
	if nilResult != nil {
		t.Error("Expected nil result for non-existent block")
	}
}

// Test XlayerHybridBlockChainAPI.GetStorageAt
func TestMigrationBlockChainAPI_GetStorageAt(t *testing.T) {
	t.Parallel()

	// Setup mock Erigon server
	server, erigonService := createMockErigonServer(t)
	defer server.Close()

	// Set up storage value in Erigon
	testAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")
	testKey := "0x0"
	testValue := hexutil.Bytes{0x01, 0x02, 0x03}
	erigonService.storage[fmt.Sprintf("%s:%s", testAddr.Hex(), testKey)] = testValue

	// Create migration legacyRpc
	client, _ := rpc.Dial(server.URL)
	config := &XlayerLegacyRPCService{
		MigrationBlock: 100,
		ErigonClient:   client,
	}
	defer config.Close()

	// Test that we can call Erigon to get storage
	ctx := context.Background()
	blockNum := rpc.BlockNumber(50)
	var result hexutil.Bytes
	err := config.ErigonClient.CallContext(ctx, &result, "eth_getStorageAt", testAddr, testKey,
		rpc.BlockNumberOrHash{BlockNumber: &blockNum})
	if err != nil {
		t.Errorf("Failed to call Erigon directly: %v", err)
	}
	if !reflect.DeepEqual(result, testValue) {
		t.Errorf("Storage value mismatch: got %v, want %v", result, testValue)
	}
}

// Test XlayerHybridTransactionAPI
func TestMigrationTransactionAPI(t *testing.T) {
	t.Parallel()

	// Setup mock Erigon server
	server, erigonService := createMockErigonServer(t)
	defer server.Close()

	// Create test transaction for Erigon
	erigonTx := types.NewTransaction(1, common.Address{0x01}, big.NewInt(1000), 21000, big.NewInt(1), nil)
	erigonTxHash := erigonTx.Hash()
	erigonService.transactions[erigonTxHash] = erigonTx
	erigonService.receipts[erigonTxHash] = &types.Receipt{
		Status:      1,
		BlockNumber: big.NewInt(50),
		GasUsed:     21000,
	}

	// Create migration legacyRpc
	client, _ := rpc.Dial(server.URL)
	config := &XlayerLegacyRPCService{
		MigrationBlock: 100,
		ErigonClient:   client,
	}
	defer config.Close()

	ctx := context.Background()

	// Test GetTransactionByHash via Erigon
	var tx *ethapi.RPCTransaction
	err := config.ErigonClient.CallContext(ctx, &tx, "eth_getTransactionByHash", erigonTxHash)
	if err != nil {
		t.Errorf("Failed to get transaction: %v", err)
	}
	if tx == nil {
		t.Error("Expected non-nil transaction")
	}
	if tx != nil && tx.Hash != erigonTxHash {
		t.Errorf("Transaction hash mismatch: got %v, want %v", tx.Hash, erigonTxHash)
	}

	// Test GetTransactionReceipt via Erigon
	var receipt map[string]interface{}
	err = config.ErigonClient.CallContext(ctx, &receipt, "eth_getTransactionReceipt", erigonTxHash)
	if err != nil {
		t.Errorf("Failed to get receipt: %v", err)
	}
	if receipt == nil {
		t.Error("Expected non-nil receipt")
	}

	// Test non-existent transaction
	nonExistentHash := common.HexToHash("0xdeadbeef")
	var nilTx *ethapi.RPCTransaction
	err = config.ErigonClient.CallContext(ctx, &nilTx, "eth_getTransactionByHash", nonExistentHash)
	if err != nil {
		t.Errorf("Unexpected error for non-existent tx: %v", err)
	}
	if nilTx != nil {
		t.Error("Expected nil for non-existent transaction")
	}
}

// Test Close method
func TestMigrationConfig_Close(t *testing.T) {
	t.Parallel()

	// Test closing legacyRpc with client
	server, _ := createMockErigonServer(t)
	defer server.Close()

	client, _ := rpc.Dial(server.URL)
	config := &XlayerLegacyRPCService{
		MigrationBlock: 100,
		ErigonClient:   client,
	}

	config.Close() // Should close the client without error

	// Try to use the client after closing (should fail)
	var result json.RawMessage
	err := config.ErigonClient.Call(&result, "eth_blockNumber")
	if err == nil {
		t.Error("Expected error when using closed client")
	}
}
