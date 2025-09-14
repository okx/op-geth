package tests

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

const (
	// Local op-geth dev node URL
	DevNodeURL = "http://localhost:8545"
)

// TestClient wraps RPC and ETH clients for inner transaction testing
type TestClient struct {
	rpcClient  *rpc.Client
	ethClient  *ethclient.Client
	devAccount common.Address
}

// TestData holds all the transaction data for the integration tests
type TestData struct {
	Client               *TestClient
	ContractCAddr        common.Address
	ContractCTxHash      common.Hash
	ContractBAddr        common.Address
	ContractBTxHash      common.Hash
	ContractAAddr        common.Address
	ContractATxHash      common.Hash
	DummyTxHash          common.Hash
	ContractCallTxHash   common.Hash
	ContractCallBlockNum uint64
	EthTransferTxHash    common.Hash
}

// NewTestClient creates a new test client connected to the dev node
func NewTestClient(t *testing.T) *TestClient {
	rpcClient, err := rpc.Dial(DevNodeURL)
	require.NoError(t, err, "Failed to connect to RPC client")

	ethClient, err := ethclient.Dial(DevNodeURL)
	require.NoError(t, err, "Failed to connect to ETH client")

	// Get the dev account
	var accounts []common.Address
	err = rpcClient.Call(&accounts, "eth_accounts")
	require.NoError(t, err, "Failed to get accounts")
	require.NotEmpty(t, accounts, "No accounts available")

	return &TestClient{
		rpcClient:  rpcClient,
		ethClient:  ethClient,
		devAccount: accounts[0],
	}
}

// Close closes the client connections
func (tc *TestClient) Close() {
	tc.rpcClient.Close()
	tc.ethClient.Close()
}

// DeployContract deploys a contract using eth_sendTransaction
func (tc *TestClient) DeployContract(t *testing.T, bytecode string) (common.Address, common.Hash) {
	// Ensure bytecode has 0x prefix
	if !strings.HasPrefix(bytecode, "0x") {
		bytecode = "0x" + bytecode
	}

	t.Log("devAccount Address:", tc.devAccount)

	// Send deployment transaction
	var txHash common.Hash
	err := tc.rpcClient.Call(&txHash, "eth_sendTransaction", map[string]interface{}{
		"from": tc.devAccount,
		"data": bytecode,
		"gas":  "0x300000",
	})
	require.NoError(t, err, "Failed to deploy contract")

	// Wait for transaction to be mined
	time.Sleep(1 * time.Second)

	// Get transaction receipt
	receipt, err := tc.ethClient.TransactionReceipt(context.Background(), txHash)
	require.NoError(t, err, "Failed to get transaction receipt")
	require.Equal(t, uint64(1), receipt.Status, "Contract deployment failed")

	return receipt.ContractAddress, txHash
}

// SendTransaction sends a transaction to a contract
func (tc *TestClient) SendTransaction(t *testing.T, to common.Address, data string) common.Hash {
	var txHash common.Hash
	err := tc.rpcClient.Call(&txHash, "eth_sendTransaction", map[string]interface{}{
		"from": tc.devAccount,
		"to":   to,
		"data": data,
		"gas":  "0x300000",
	})
	require.NoError(t, err, "Failed to send transaction")

	// Wait for transaction to be mined
	time.Sleep(3 * time.Second)

	return txHash
}

// GetInternalTransactions calls eth_getInternalTransactions
func (tc *TestClient) GetInternalTransactions(t *testing.T, txHash common.Hash) []*types.InnerTx {
	var result []*types.InnerTx
	err := tc.rpcClient.Call(&result, "eth_getInternalTransactions", txHash)
	if err != nil {
		t.Logf("Warning: Failed to get internal transactions: %v", err)
		return []*types.InnerTx{}
	}
	return result
}

// GetBlockInternalTransactions calls eth_getBlockInternalTransactions
func (tc *TestClient) GetBlockInternalTransactions(t *testing.T, blockNumber interface{}) map[common.Hash][]*types.InnerTx {
	var result map[common.Hash][]*types.InnerTx
	err := tc.rpcClient.Call(&result, "eth_getBlockInternalTransactions", blockNumber)
	if err != nil {
		t.Logf("Warning: Failed to get block internal transactions: %v", err)
		return map[common.Hash][]*types.InnerTx{}
	}
	return result
}

// GetTransactionReceipt gets transaction receipt and block number
func (tc *TestClient) GetTransactionReceipt(t *testing.T, txHash common.Hash) (*types.Receipt, uint64) {
	receipt, err := tc.ethClient.TransactionReceipt(context.Background(), txHash)
	require.NoError(t, err, "Failed to get transaction receipt")
	return receipt, receipt.BlockNumber.Uint64()
}

// setupTestData deploys contracts and sends transactions for testing
func setupTestData(t *testing.T) *TestData {
	client := NewTestClient(t)

	t.Logf("Using dev account: %s", client.devAccount.Hex())

	// Deploy ContractC first (simple storage contract)
	contractCAddr, contractCTxHash := client.DeployContract(t, ContractCBytecodeStr)
	t.Logf("ContractC deployed at: %s (tx: %s)", contractCAddr.Hex(), contractCTxHash.Hex())

	constructorDataB := fmt.Sprintf("%s%064s", ContractBBytecodeStr, strings.TrimPrefix(contractCAddr.Hex(), "0x"))
	contractBAddr, contractBTxHash := client.DeployContract(t, constructorDataB)
	t.Logf("ContractB deployed at: %s (tx: %s)", contractBAddr.Hex(), contractBTxHash.Hex())

	constructorDataA := fmt.Sprintf("%s%064s", ContractABytecodeStr, strings.TrimPrefix(contractBAddr.Hex(), "0x"))
	contractAAddr, contractATxHash := client.DeployContract(t, constructorDataA)
	t.Logf("ContractA deployed at: %s (tx: %s)", contractAAddr.Hex(), contractATxHash.Hex())

	// Deploy erc20 contract
	erc20contractAddr, erc20contractTxHash := client.DeployContract(t, Erc20BytecodeStr)
	t.Logf("erc20contract deployed at: %s (tx: %s)", erc20contractAddr.Hex(), erc20contractTxHash.Hex())

	// dummy() signature: 0x32e43a11
	dummyData := "0x32e43a11"
	dummyTxHash := client.SendTransaction(t, contractBAddr, dummyData)
	t.Logf("Dummy tx: %s", dummyTxHash.Hex())

	// Use triggerCall() function on ContractA which calls ContractB.dummy()
	// triggerCall() signature: 0xf18c388a
	triggerCallData := "0xf18c388a"
	contractCallTxHash := client.SendTransaction(t, contractAAddr, triggerCallData)
	t.Logf("Contract-to-contract call tx: %s", contractCallTxHash.Hex())

	_, blockNum := client.GetTransactionReceipt(t, contractCallTxHash)

	// Send simple ETH transfer (should not generate inner transactions)
	var ethTransferTxHash common.Hash
	err := client.rpcClient.Call(&ethTransferTxHash, "eth_sendTransaction", map[string]interface{}{
		"from":  client.devAccount,
		"to":    "0x742d35Cc6AB13552F90B8A3d8A8e6E4A4E2D8c10",
		"value": "0x1000000000000000", // 0.001 ETH
		"gas":   "0x21000",
	})
	if err != nil {
		t.Fatalf("Failed to send ETH transfer: %v", err)
	}
	time.Sleep(3 * time.Second)
	t.Logf("ETH transfer tx: %s", ethTransferTxHash.Hex())

	return &TestData{
		Client:               client,
		ContractCAddr:        contractCAddr,
		ContractCTxHash:      contractCTxHash,
		ContractBAddr:        contractBAddr,
		ContractBTxHash:      contractBTxHash,
		ContractAAddr:        contractAAddr,
		ContractATxHash:      contractATxHash,
		DummyTxHash:          dummyTxHash,
		ContractCallTxHash:   contractCallTxHash,
		ContractCallBlockNum: blockNum,
		EthTransferTxHash:    ethTransferTxHash,
	}
}
