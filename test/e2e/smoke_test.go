//go:build !skip_smoke
// +build !skip_smoke

package e2e

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/test/constants"
	"github.com/ethereum/go-ethereum/test/operations"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

const (
	Gwei         = 1000000000
	testVerified = true
)

func TestSendTx(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	client, err := ethclient.Dial(operations.DefaultL2NetworkURL)
	require.NoError(t, err)
	TransToken(t, ctx, client, uint256.NewInt(params.GWei), operations.DefaultRichAddress)

	from := common.HexToAddress(operations.DefaultRichAddress)
	to := common.HexToAddress(operations.DefaultRichAddress)
	nonce, err := client.PendingNonceAt(ctx, from)
	require.NoError(t, err)
	gas, err := client.EstimateGas(ctx, ethereum.CallMsg{
		From:  from,
		To:    &to,
		Value: big.NewInt(10),
	})
	require.NoError(t, err)
	tx := types.NewTransaction(nonce, to, big.NewInt(10), gas, big.NewInt(100*params.GWei), nil)
	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(operations.DefaultRichPrivateKey, "0x"))
	require.NoError(t, err)

	signer := types.MakeSigner(operations.GetTestChainConfig(operations.DefaultL2ChainID), big.NewInt(1), 0)
	signedTx, err := types.SignTx(tx, signer, privateKey)
	require.NoError(t, err)

	err = client.SendTransaction(ctx, signedTx)
	require.NoError(t, err)

	err = operations.WaitTxToBeMined(ctx, client, signedTx, operations.DefaultTimeoutTxToBeMined)
	require.NoError(t, err)
}

func TestChainID(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	chainID, err := operations.EthChainID()
	require.NoError(t, err)
	require.Equal(t, chainID, operations.DefaultL2ChainID)
}

func TestEthTransfer(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	if !testVerified {
		return
	}

	ctx := context.Background()
	auth, err := operations.GetAuth(operations.DefaultRichPrivateKey, operations.DefaultL2ChainID)
	require.NoError(t, err)
	client, err := ethclient.Dial(operations.DefaultL2NetworkURL)
	require.NoError(t, err)

	from := common.HexToAddress(operations.DefaultRichAddress)
	to := common.HexToAddress(operations.DefaultL2NewAcc1Address)
	nonce, err := client.PendingNonceAt(ctx, from)
	require.NoError(t, err)
	tx := types.NewTransaction(nonce, to, big.NewInt(0), 21000, big.NewInt(10*params.GWei), nil)
	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(operations.DefaultRichPrivateKey, "0x"))
	require.NoError(t, err)
	signer := types.MakeSigner(operations.GetTestChainConfig(operations.DefaultL2ChainID), big.NewInt(1), 0)
	signedTx, err := types.SignTx(tx, signer, privateKey)
	require.NoError(t, err)
	var txs []*types.Transaction
	txs = append(txs, signedTx)
	_, err = operations.ApplyL2Txs(ctx, txs, auth, client, operations.VerifiedConfirmationLevel)
	require.NoError(t, err)
}

func TestDebugTraceRPC(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	// Wait for at least one block to be available
	var blockNumber uint64
	var err error
	for i := 0; i < 30; i++ {
		blockNumber, err = operations.GetBlockNumber()
		require.NoError(t, err)
		log.Info("Block number: %d, attempt: %v", blockNumber, i)
		if blockNumber > 3 {
			break
		}
		time.Sleep(1 * time.Second)
	}
	require.Greater(t, blockNumber, uint64(0), "Block number should be greater than 0")

	// Get a block to trace
	blockNum, err := operations.GetBlockNumber()
	require.NoError(t, err)

	// Use the working RPC method instead of the broken GetBlockByNumber
	blockNumberHex := fmt.Sprintf("0x%x", blockNum)
	blockData, err := operations.EthGetBlockByNumber(blockNumberHex, true)
	require.NoError(t, err)
	require.NotNil(t, blockData, "Block data should not be nil")

	blockHash := common.Hash{}
	if blockMap, ok := blockData.(map[string]interface{}); ok {
		if hashStr, exists := blockMap["hash"].(string); exists && hashStr != "" {
			blockHash = common.HexToHash(hashStr)
		}
	}

	// Test debug_traceBlockByHash
	t.Run("DebugTraceBlockByHash", func(t *testing.T) {
		require.NotEqual(t, common.Hash{}, blockHash, "Block hash should not be empty")

		traceResult, err := operations.DebugTraceBlockByHash(blockHash)
		require.NoError(t, err)
		require.NotNil(t, traceResult, "Trace result should not be nil")

		log.Info("DebugTraceBlockByHash result type: %T", traceResult)
	})

	// Test debug_traceBlockByNumber
	t.Run("DebugTraceBlockByNumber", func(t *testing.T) {
		traceResult, err := operations.DebugTraceBlockByNumber(blockNum)
		require.NoError(t, err)
		require.NotNil(t, traceResult, "Trace result should not be nil")

		log.Info("DebugTraceBlockByNumber result type: %T", traceResult)
	})

	// Test debug_traceTransaction
	t.Run("DebugTraceTransaction", func(t *testing.T) {
		blockInfo, err := operations.EthGetBlockByHash(blockHash, true)
		require.NoError(t, err)
		var blockInfoMap map[string]interface{}
		var txsInterface interface{}
		var txs []interface{}
		var txMap map[string]interface{}
		var txHashStr string
		var exists bool
		var ok bool

		if blockInfoMap, ok = blockInfo.(map[string]interface{}); !ok {
			t.Error("Block info not in expected format")
		}
		if txsInterface, exists = blockInfoMap["transactions"]; !exists {
			t.Error("Transactions field not found in block data")
		}

		if txs, ok = txsInterface.([]interface{}); !ok || len(txs) == 0 {
			t.Error("No transactions found in block")
		}

		if txMap, ok = txs[0].(map[string]interface{}); !ok {
			t.Error("Transaction data not in expected format")
		}

		if txHashStr, exists = txMap["hash"].(string); !exists {
			t.Error("Transaction hash not found in transaction data")
		}

		txHash := common.HexToHash(txHashStr)
		require.NotEqual(t, common.Hash{}, txHash, "Transaction hash should not be empty")

		traceResult, err := operations.DebugTraceTransaction(txHash)
		require.NoError(t, err)
		require.NotNil(t, traceResult, "Trace result should not be nil")

		log.Info("DebugTraceTransaction result type: %T", traceResult)
	})
}

// TestEthereumBasicRPC tests basic Ethereum RPC methods
func TestEthereumBasicRPC(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	_, _ = SetupTestEnvironment(t)

	// Default test address for tests that require an address
	testAddress := common.HexToAddress("0x1234567890123456789012345678901234567890")

	// Test eth_chainId
	t.Run("EthChainID", func(t *testing.T) {
		chainID, err := operations.EthChainID()
		require.NoError(t, err)
		require.NotEqual(t, uint64(0), chainID, "Chain ID should not be zero")
		log.Info("EthChainID result: %d", chainID)
	})

	// Test eth_syncing
	t.Run("EthSyncing", func(t *testing.T) {
		syncing, err := operations.EthSyncing()
		require.NoError(t, err)
		log.Info("EthSyncing result: %t", syncing)
	})

	// Test eth_getBalance
	t.Run("EthGetBalance", func(t *testing.T) {
		balance, err := operations.EthGetBalance(testAddress, "latest")
		require.NoError(t, err)
		log.Info("EthGetBalance result for test address: %s", balance.String())
	})

	// Test eth_getCode
	t.Run("EthGetCode", func(t *testing.T) {
		code, err := operations.EthGetCode(testAddress, "latest")
		require.NoError(t, err)
		log.Info("EthGetCode result length: %d", len(code))
	})

	// Test eth_getTransactionCount
	t.Run("EthGetTransactionCount", func(t *testing.T) {
		txCount, err := operations.EthGetTransactionCount(testAddress, "latest")
		require.NoError(t, err)
		log.Info("EthGetTransactionCount result: %d", txCount)
	})

	// Test eth_blockNumber
	t.Run("EthBlockNumber", func(t *testing.T) {
		blockNumber, err := operations.EthBlockNumber()
		require.NoError(t, err)
		require.Greater(t, blockNumber, uint64(0), "Block number should be greater than 0")
		log.Info("EthBlockNumber result: %d", blockNumber)
	})

	// Test eth_gasPrice
	t.Run("EthGasPrice", func(t *testing.T) {
		gasPrice, err := operations.EthGasPrice()
		require.NoError(t, err)
		require.Greater(t, gasPrice.Cmp(big.NewInt(0)), 0, "Gas price should be greater than 0")
		log.Info("EthGasPrice result: %s", gasPrice.String())
	})

	// Test eth_getStorageAt
	t.Run("EthGetStorageAt", func(t *testing.T) {
		storage, err := operations.EthGetStorageAt(testAddress, "0x0", "latest")
		require.NoError(t, err)
		log.Info("EthGetStorageAt result: %s", storage)
	})
}

// TestEthereumBlockRPC tests Ethereum block-related RPC methods
func TestEthereumBlockRPC(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	EnsureContractsDeployed(t)

	blockHash, blockNumber := SetupTestEnvironment(t)
	ctx := context.Background()
	client, err := ethclient.Dial(operations.DefaultL2NetworkURL)
	require.NoError(t, err)
	defer client.Close()

	toAddr := common.HexToAddress(operations.DefaultL2NewAcc1Address)

	// Test eth_getBlockByHash
	t.Run("EthGetBlockByHash", func(t *testing.T) {
		block, err := operations.EthGetBlockByHash(blockHash, true)
		require.NoError(t, err)
		require.NotNil(t, block, "Block should not be nil")
		log.Info("EthGetBlockByHash result type: %T", block)
	})

	// Test eth_getBlockByNumber
	t.Run("EthGetBlockByNumber", func(t *testing.T) {
		blockNumberHex := fmt.Sprintf("0x%x", blockNumber)
		block, err := operations.EthGetBlockByNumber(blockNumberHex, true)
		require.NoError(t, err)
		require.NotNil(t, block, "Block should not be nil")
		log.Info("EthGetBlockByNumber result type: %T", block)
	})

	// Test eth_getBlockTransactionCountByHash
	t.Run("EthGetBlockTransactionCountByHash", func(t *testing.T) {
		txCount, err := operations.EthGetBlockTransactionCountByHash(blockHash)
		require.NoError(t, err)
		log.Info("EthGetBlockTransactionCountByHash result: %d", txCount)
	})

	// Test eth_getBlockTransactionCountByNumber
	t.Run("EthGetBlockTransactionCountByNumber", func(t *testing.T) {
		// Use current block instead of pruned block #1
		currentBlockHex := fmt.Sprintf("0x%x", blockNumber)
		txCount, err := operations.EthGetBlockTransactionCountByNumber(currentBlockHex)
		require.NoError(t, err)
		log.Info("EthGetBlockTransactionCountByNumber result: %d", txCount)
	})

	// Test eth_getTransactionByBlockHashAndIndex
	t.Run("EthGetTransactionByBlockHashAndIndex", func(t *testing.T) {
		tx, err := operations.EthGetTransactionByBlockHashAndIndex(blockHash, "0x0")
		require.NoError(t, err)
		log.Info("EthGetTransactionByBlockHashAndIndex result type: %T", tx)
	})

	// Test eth_getTransactionByBlockNumberAndIndex
	t.Run("EthGetTransactionByBlockNumberAndIndex", func(t *testing.T) {
		// Use current block instead of pruned block #1
		currentBlockHex := fmt.Sprintf("0x%x", blockNumber)
		tx, err := operations.EthGetTransactionByBlockNumberAndIndex(currentBlockHex, "0x0")
		require.NoError(t, err)
		require.NotNil(t, tx, "Transaction should not be nil")
		log.Info("EthGetTransactionByBlockNumberAndIndex result type: %T", tx)
	})

	t.Run("EthGetBlockReceipts", func(t *testing.T) {
		numberOfTransactions := 10
		_, targetBlockNumber, targetBlockHash := transErc20TokenBatch(t, context.Background(), client, ERC20Addr, big.NewInt(Gwei), toAddr.String(), numberOfTransactions)

		// Test getting receipts by number
		receiptsByNumber, err := client.BlockReceipts(ctx, rpc.BlockNumberOrHashWithNumber(rpc.BlockNumber(targetBlockNumber)))
		require.NoError(t, err)
		require.NotNil(t, receiptsByNumber, "Transaction receipts by number should not be nil")
		for _, receipt := range receiptsByNumber {
			require.NoError(t, err)
			require.NotNil(t, receipt)
			log.Info(fmt.Sprintf("RealtimeGetBlockReceiptsByNumber result type: %T", receipt))
		}

		// Test getting receipts by hash
		receiptsByHash, err := client.BlockReceipts(ctx, rpc.BlockNumberOrHashWithHash(targetBlockHash, true))
		require.NoError(t, err)
		require.NotNil(t, receiptsByHash, "Transaction receipts by hash should not be nil")
		for _, receipt := range receiptsByHash {
			require.NoError(t, err)
			require.NotNil(t, receipt)
			log.Info(fmt.Sprintf("RealtimeGetBlockReceiptsByHash result type: %T", receipt))
		}
	})
}

// TestEthereumTransactionRPC tests Ethereum transaction-related RPC methods
func TestEthereumTransactionRPC(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	EnsureContractsDeployed(t)
	fromAddr := common.HexToAddress(operations.DefaultRichAddress)
	toAddr := common.HexToAddress(operations.DefaultL2NewAcc1Address)

	ctx := context.Background()
	client, err := ethclient.Dial(operations.DefaultL2NetworkURL)
	require.NoError(t, err)
	defer client.Close()

	txhash := TransToken(t, ctx, client, uint256.NewInt(params.GWei), operations.DefaultRichAddress)
	fmt.Printf("TransToken txhash: %s\n", txhash)

	t.Run("EthEstimateGas", func(t *testing.T) {
		balance, err := client.BalanceAt(ctx, fromAddr, nil)
		require.NoError(t, err)
		require.Greater(t, balance.Cmp(big.NewInt(0)), 0, "From address should have balance")

		t.Run("SimpleTransfer", func(t *testing.T) {
			transferAmount := big.NewInt(1000000000000000)

			gas, err := client.EstimateGas(ctx, ethereum.CallMsg{
				From:  fromAddr,
				To:    &toAddr,
				Value: transferAmount,
			})
			require.NoError(t, err)
			require.Equal(t, uint64(21000), gas, "Simple transfer should use exactly 21000 gas")

			fmt.Printf("EthEstimateGas SimpleTransfer result: %d gas\n", gas)
		})

		// Test 2: Contract call gas estimation
		t.Run("ContractCall", func(t *testing.T) {
			contractAABI, err := abi.JSON(strings.NewReader(constants.ContractAABIJson))
			require.NoError(t, err)
			calldata, err := contractAABI.Pack("triggerCall")
			require.NoError(t, err)

			gas, err := client.EstimateGas(ctx, ethereum.CallMsg{
				From: fromAddr,
				To:   &ContractAAddr,
				Data: calldata,
			})
			require.NoError(t, err)
			require.Greater(t, gas, uint64(21000), "Contract call should use more than 21000 gas")

			fmt.Printf("EthEstimateGas ContractCall result: %d gas\n", gas)
		})
	})

	t.Run("EthCall", func(t *testing.T) {
		// Get balance before the call
		balanceBefore, err := client.BalanceAt(ctx, fromAddr, nil)
		require.NoError(t, err)
		require.Greater(t, balanceBefore.Cmp(big.NewInt(0)), 0, "From address should have balance")

		contractCABI, err := abi.JSON(strings.NewReader(constants.ContractCABIJson))
		require.NoError(t, err)
		calldata, err := contractCABI.Pack("getValue")
		require.NoError(t, err)

		result, err := client.CallContract(ctx, ethereum.CallMsg{
			From: fromAddr,
			To:   &ContractCAddr,
			Data: calldata,
		}, nil)
		require.NoError(t, err)
		require.NotNil(t, result, "Call result should not be nil")

		balanceAfter, err := client.BalanceAt(ctx, fromAddr, nil)
		require.NoError(t, err)
		require.Equal(t, balanceBefore.String(), balanceAfter.String(), "Balance should remain unchanged after eth_call")
	})

	t.Run("EthGetTransactionByHash", func(t *testing.T) {
		result, err := operations.EthGetTransactionByHash(common.HexToHash(txhash))
		require.NoError(t, err)

		txData, _ := result.(map[string]interface{})

		receipt, err := client.TransactionReceipt(ctx, common.HexToHash(txhash))
		require.NoError(t, err)
		require.NotNil(t, receipt, "Receipt should not be nil")

		txHashStr, exists := txData["hash"].(string)
		require.True(t, exists, "Transaction should have hash field")
		require.Equal(t, txhash, txHashStr, "Transaction hash should match")

		txIndexHex := txData["transactionIndex"].(string)
		txIndex, err := hexutil.DecodeUint64(txIndexHex)
		require.NoError(t, err, "Transaction index should be valid hex")
		require.Equal(t, uint64(receipt.TransactionIndex), txIndex, "Transaction index should match between transaction and receipt")

		fromAddr, exists := txData["from"].(string)
		require.True(t, exists, "Transaction should have from field")
		require.Equal(t, strings.ToLower(operations.DefaultRichAddress), strings.ToLower(fromAddr), "From address should match")
	})

	t.Run("EthGetTransactionReceipt", func(t *testing.T) {
		// Get receipt using operations RPC method
		result, err := operations.EthGetTransactionReceipt(common.HexToHash(txhash))
		require.NoError(t, err)

		receiptData, _ := result.(map[string]interface{})

		fromAddrStr, exists := receiptData["from"].(string)
		require.True(t, exists, "Receipt should have from field")
		require.Equal(t, strings.ToLower(operations.DefaultRichAddress), strings.ToLower(fromAddrStr), "from address should match sender")

		statusStr, exists := receiptData["status"].(string)
		require.True(t, exists, "Receipt should have status field")
		require.Equal(t, "0x1", statusStr, "status should be 0x1 for successful transaction")

		toAddrStr, exists := receiptData["to"].(string)
		require.True(t, exists, "Receipt should have to field")
		require.Equal(t, strings.ToLower(operations.DefaultRichAddress), strings.ToLower(toAddrStr), "to address should match recipient")

		txHashStr, exists := receiptData["transactionHash"].(string)
		require.True(t, exists, "Receipt should have transactionHash field")
		require.Equal(t, txhash, txHashStr, "transactionHash should match the original transaction hash")
	})
}

// TestEthereumLogsRPC tests Ethereum logs-related RPC methods
func TestEthereumLogsRPC(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	_, blockNumber := SetupTestEnvironment(t)

	// Test eth_getLogs
	t.Run("EthGetLogs", func(t *testing.T) {
		fromBlock := fmt.Sprintf("0x%x", blockNumber)
		toBlock := fmt.Sprintf("0x%x", blockNumber)
		address := common.HexToAddress("0x1234567890123456789012345678901234567890")

		logs, err := operations.EthGetLogs(fromBlock, toBlock, address)
		require.NoError(t, err)
		require.NotNil(t, logs, "Logs should not be nil")
		log.Info("EthGetLogs result type: %T", logs)
	})
}

// TestTxPoolRPC tests transaction pool related RPC methods
func TestTxPoolRPC(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	_, _ = SetupTestEnvironment(t)

	// Test txpool_content - This might return a large object, so only log type
	t.Run("TxPoolContent", func(t *testing.T) {
		content, err := operations.TxPoolContent()
		require.NoError(t, err)
		log.Info("TxPoolContent result type: %T", content)
	})

	// Test txpool_status
	t.Run("TxPoolStatus", func(t *testing.T) {
		status, err := operations.TxPoolStatus()
		require.NoError(t, err)
		log.Info("TxPoolStatus result type: %T", status)
	})
}

// TestInnerTransactionRPC tests inner transaction related RPC methods
func TestInnerTx(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	ctx := context.Background()
	client, err := ethclient.Dial(operations.DefaultL2NetworkURL)
	require.NoError(t, err)
	defer client.Close()

	EnsureContractsDeployed(t)

	preexecPrivateKey, err := crypto.HexToECDSA(TmpSenderPrivateKey)
	require.NoError(t, err)
	preexecFrom := crypto.PubkeyToAddress(preexecPrivateKey.PublicKey)

	// Call ContractA's triggerCall function which will call ContractB's dummy function
	// triggerCall() function selector: 0xf18c388a
	triggerCallData := common.Hex2Bytes("f18c388a")

	signedContractATxHash, err := MakeContractCall(t, ctx, client, preexecPrivateKey, ContractAAddr, triggerCallData, 200000, nil)
	require.NoError(t, err)

	contractAReceipt, err := client.TransactionReceipt(ctx, signedContractATxHash)
	require.NoError(t, err)

	// Call ContractC's setValue function
	contractCSetValueData := common.Hex2Bytes("552410770000000000000000000000000000000000000000000000000000000000000123")
	signedContractCSetValueTxHash, err := MakeContractCall(t, ctx, client, preexecPrivateKey, ContractCAddr, contractCSetValueData, 200000, nil)
	require.NoError(t, err)
	fmt.Printf("signedContractCSetValueTxHash: %s\n", signedContractCSetValueTxHash.Hex())

	// Call ContractC's getValue function
	contractCGetValueData := common.Hex2Bytes("20965255")
	signedContractCGetValueTxHash, err := MakeContractCall(t, ctx, client, preexecPrivateKey, ContractCAddr, contractCGetValueData, 200000, nil)
	require.NoError(t, err)
	contractCGetValueReceipt, err := client.TransactionReceipt(ctx, signedContractCGetValueTxHash)
	require.NoError(t, err)

	t.Run("GetInternalTransactions", func(t *testing.T) {
		innerTxs, err := operations.EthGetInternalTransactions(signedContractCGetValueTxHash)
		require.NoError(t, err)
		require.NotNil(t, innerTxs, "Inner transactions result should not be nil")
		require.Len(t, innerTxs, 1, "Should have exactly 1 inner transaction for getValue call")

		innerTx := innerTxs[0]
		require.NotNil(t, innerTx, "innerTx should not be nil")

		expectedInnerTx := &types.InnerTx{
			Dept:          *big.NewInt(0),
			InternalIndex: *big.NewInt(0),
			CallType:      "",
			Name:          "",
			TraceAddress:  "",
			CodeAddress:   "",
			From:          preexecFrom.Hex(),
			To:            ContractCAddr.Hex(),
			Input:         "",
			Output:        "0x0000000000000000000000000000000000000000000000000000000000000123",
			IsError:       false,
			Error:         "",
			Value:         "",
			ValueWei:      "0",
			CallValueWei:  "0x0",
		}

		ValidateInnerTransactionMatch(t, innerTx, expectedInnerTx, "ContractC getValue inner transaction")
		require.Equal(t, contractCGetValueReceipt.GasUsed, innerTx.GasUsed, "GasUsed should be the same as the gasUsed in transaction receipt for getValue call")
		require.Equal(t, uint64(200000), innerTx.Gas, "Gas should be the same as the gas in transaction receipt for getValue call")
	})

	t.Run("GetInternalTransactions_WithDeepInnerTransactions", func(t *testing.T) {
		innerTxs, err := operations.EthGetInternalTransactions(signedContractATxHash)
		require.NoError(t, err)
		require.NotNil(t, innerTxs, "Inner transactions result should not be nil")
		require.Len(t, innerTxs, 2, "Should have exactly 2 inner transactions: EOA->ContractA and ContractA->ContractB")

		// Validate first inner transaction: EOA -> ContractA (triggerCall)
		expectedInnertx1 := &types.InnerTx{
			Dept:          *big.NewInt(0),
			InternalIndex: *big.NewInt(0),
			CallType:      "",
			Name:          "",
			TraceAddress:  "",
			CodeAddress:   "",
			From:          preexecFrom.Hex(),
			To:            ContractAAddr.Hex(),
			Input:         "",
			Output:        "",
			IsError:       false,
			Error:         "",
			Value:         "",
			ValueWei:      "0",
			CallValueWei:  "0x0",
		}
		ValidateInnerTransactionMatch(t, innerTxs[0], expectedInnertx1, "First inner transaction (EOA -> ContractA)")

		// gasUsed of first inner transaction should be the same as the gasUsed in transaction receipt
		require.Equal(t, contractAReceipt.GasUsed, innerTxs[0].GasUsed, "First inner transaction GasUsed should be the same as contractAReceipt.GasUsed")
		require.Equal(t, uint64(200000), innerTxs[0].Gas, "Gas should be the same as the gas in transaction receipt for getValue call")

		// Validate second inner transaction: ContractA -> ContractB (dummy call)
		expectedInnertx2 := &types.InnerTx{
			Dept:          *big.NewInt(1),
			InternalIndex: *big.NewInt(0),
			CallType:      "call",
			Name:          "call_0",
			TraceAddress:  "",
			CodeAddress:   "",
			From:          ContractAAddr.Hex(),
			To:            ContractBAddr.Hex(),
			Input:         "0x32e43a11",
			Output:        "",
			IsError:       false,
			Error:         "",
			Value:         "",
			ValueWei:      "0",
			CallValueWei:  "0x0",
		}
		ValidateInnerTransactionMatch(t, innerTxs[1], expectedInnertx2, "Second inner transaction (ContractA -> ContractB)")

		// gasUsed of second inner transaction should be less than the gasUsed in transaction receipt
		require.Less(t, innerTxs[1].GasUsed, contractAReceipt.GasUsed, "Second inner transaction GasUsed should be the less than contractAReceipt.GasUsed")
	})

	t.Run("GetInternalTransactions_WithOutput", func(t *testing.T) {
		innerTxs, err := operations.EthGetInternalTransactions(signedContractCGetValueTxHash)
		require.NoError(t, err)
		require.NotNil(t, innerTxs, "Inner transactions result should not be nil")
		require.Len(t, innerTxs, 1, "Should have exactly 1 inner transaction for getValue call")

		innerTx := innerTxs[0]
		require.NotNil(t, innerTx, "innerTx should not be nil")

		expectedInnerTx := &types.InnerTx{
			Dept:          *big.NewInt(0),
			InternalIndex: *big.NewInt(0),
			CallType:      "",
			Name:          "",
			TraceAddress:  "",
			CodeAddress:   "",
			From:          preexecFrom.Hex(),
			To:            ContractCAddr.Hex(),
			Input:         "",
			Output:        "0x0000000000000000000000000000000000000000000000000000000000000123",
			IsError:       false,
			Error:         "",
			Value:         "",
			ValueWei:      "0",
			CallValueWei:  "0x0",
		}

		ValidateInnerTransactionMatch(t, innerTx, expectedInnerTx, "ContractC getValue inner transaction with output")
		require.Equal(t, contractCGetValueReceipt.GasUsed, innerTx.GasUsed, "GasUsed should be the same as the gasUsed in transaction receipt")
	})
	t.Run("GetInternalTransactions_Batch", func(t *testing.T) {
		// Send multiple transactions in a batch
		txHashes := TransTokenBatch(t, ctx, client, uint256.NewInt(params.GWei), operations.DefaultL2NewAcc1Address, 10, operations.DefaultRichPrivateKey)
		require.Len(t, txHashes, 10, "Should have created 10 transactions")

		// Verify each transaction has exactly 1 inner transaction
		numInnerTxs := 0
		for i, txHash := range txHashes {
			fmt.Printf("Getting inner transactions for tx %d: %s\n", i, txHash)
			innerTxs, err := operations.EthGetInternalTransactions(common.HexToHash(txHash))
			require.NoError(t, err, "Failed to get inner transactions for tx %d: %s", i, txHash)
			require.Len(t, innerTxs, 1, "Transaction %d (%s) should have exactly 1 inner transaction, got %d", i, txHash, len(innerTxs))

			txReceipt, err := client.TransactionReceipt(ctx, common.HexToHash(txHash))
			require.NoError(t, err, "Failed to get transaction receipt for tx %d: %s", i, txHash)

			innerTx := innerTxs[0]
			require.NotNil(t, innerTx, "innerTx should not be nil for tx %d", i)

			expectedInnerTx := &types.InnerTx{
				Dept:          *big.NewInt(0),
				InternalIndex: *big.NewInt(0),
				CallType:      "",
				Name:          "",
				TraceAddress:  "",
				CodeAddress:   "",
				From:          operations.DefaultRichAddress,
				To:            operations.DefaultL2NewAcc1Address,
				Input:         "",
				Output:        "",
				IsError:       false,
				Error:         "",
				Value:         "",
				ValueWei:      strconv.FormatUint(params.GWei, 10),
				CallValueWei:  hexutil.EncodeUint64(params.GWei),
			}

			ValidateInnerTransactionMatch(t, innerTx, expectedInnerTx, fmt.Sprintf("Batch transaction %d", i))
			require.Equal(t, txReceipt.GasUsed, innerTx.GasUsed, "GasUsed should be the same as the gasUsed in transaction receipt for batch transaction %d", i)
			numInnerTxs++
		}
		require.Equal(t, 10, numInnerTxs, "Should have 10 inner transactions for all 10 transactions")
	})

	t.Run("GetBlockInternalTransactions_Batch", func(t *testing.T) {
		// Send multiple transactions in a batch
		txHashes := TransTokenBatch(t, ctx, client, uint256.NewInt(params.GWei), operations.DefaultL2NewAcc1Address, 5, operations.DefaultRichPrivateKey)
		require.Len(t, txHashes, 5, "Should have created 5 transactions")

		blockNumbers := make(map[uint64][]int)

		for i, txHashStr := range txHashes {
			txHash := common.HexToHash(txHashStr)
			receipt, err := client.TransactionReceipt(ctx, txHash)
			require.NoError(t, err, "Failed to get receipt for tx %d", i)

			blockNum := receipt.BlockNumber.Uint64()
			blockNumbers[blockNum] = append(blockNumbers[blockNum], i)
		}

		totalValidatedTxs := 0

		for blockNum, txIndices := range blockNumbers {
			fmt.Printf("Testing block %d with %d transactions\n", blockNum, len(txIndices))

			blockInnerTxs, err := operations.EthGetBlockInternalTransactions(rpc.BlockNumber(blockNum))
			require.NoError(t, err, "Failed to get block internal transactions for block %d", blockNum)
			require.NotNil(t, blockInnerTxs, "Block inner transactions should not be nil for block %d", blockNum)

			// Verify all transactions in this block are present in block internal transactions
			batchTxsInBlock := 0
			for _, txIdx := range txIndices {
				txHash := common.HexToHash(txHashes[txIdx])
				blockInnerTxsForTx, exists := blockInnerTxs[txHash]
				require.True(t, exists, "Transaction %d (%s) should be in block %d internal transactions", txIdx, txHashes[txIdx], blockNum)
				require.Len(t, blockInnerTxsForTx, 1, "Transaction %d should have exactly 1 inner transaction", txIdx)
				batchTxsInBlock++

				innerTx := blockInnerTxsForTx[0]

				// Compare with individual transaction inner transactions
				individualInnerTxs, err := operations.EthGetInternalTransactions(txHash)
				require.NoError(t, err, "Failed to get individual inner transactions for tx %d", txIdx)
				require.Len(t, individualInnerTxs, 1, "Individual inner transactions should have 1 entry for tx %d", txIdx)

				ValidateInnerTransactionMatch(t, individualInnerTxs[0], innerTx, fmt.Sprintf("batch tx %d comparison", txIdx))
			}

			totalValidatedTxs += batchTxsInBlock
		}

		require.Equal(t, len(txHashes), totalValidatedTxs, "Should have validated all batch transactions")
		fmt.Printf("Successfully validated all %d transactions across %d blocks\n", totalValidatedTxs, len(blockNumbers))
	})

	t.Run("GetInnerTransactions_FailedTransactions", func(t *testing.T) {
		amount := uint256.NewInt(params.GWei)
		toAddress := ContractAAddr.String()

		txHash := TransTokenFail(t, ctx, client, TmpSenderPrivateKey, amount, toAddress)

		innerTxs, err := operations.EthGetInternalTransactions(txHash)
		require.NoError(t, err, "Should be able to get inner transactions")
		require.Len(t, innerTxs, 1, "Should have exactly 1 inner transaction for failed transfer")

		txReceipt, err := client.TransactionReceipt(ctx, txHash)
		require.NoError(t, err, "Should be able to get transaction receipt")

		innerTx := innerTxs[0]
		require.NotNil(t, innerTx, "innerTx should not be nil")

		senderPrivateKey, err := crypto.HexToECDSA(TmpSenderPrivateKey)
		require.NoError(t, err)
		senderAddress := crypto.PubkeyToAddress(senderPrivateKey.PublicKey)

		expectedInnerTx := &types.InnerTx{
			Dept:          *big.NewInt(0),
			InternalIndex: *big.NewInt(0),
			CallType:      "",
			Name:          "",
			TraceAddress:  "",
			CodeAddress:   "",
			From:          senderAddress.Hex(),
			To:            toAddress,
			Input:         "",
			Output:        "",
			IsError:       true,
			Error:         "execution reverted",
			Value:         "",
			ValueWei:      strconv.FormatUint(amount.Uint64(), 10),
			CallValueWei:  hexutil.EncodeUint64(amount.Uint64()),
		}

		ValidateInnerTransactionMatch(t, innerTx, expectedInnerTx, "Failed transaction inner transaction")
		require.Equal(t, txReceipt.GasUsed, innerTx.GasUsed, "GasUsed should be the same as the gasUsed in transaction receipt for failed transaction")
	})

	t.Run("SpecialBlockNumberFormats", func(t *testing.T) {
		// Test "latest" format
		latestResult, err := operations.EthGetBlockInternalTransactions(rpc.LatestBlockNumber)
		require.NoError(t, err)
		require.NotNil(t, latestResult, "Latest block should return a valid map")
		fmt.Printf("'latest' block contains %d transactions with inner transactions\n", len(latestResult))

		// Test "earliest" format
		earliestResult, err := operations.EthGetBlockInternalTransactions(rpc.EarliestBlockNumber)
		require.NoError(t, err)
		require.NotNil(t, earliestResult, "Earliest block should return a valid map")
		fmt.Printf("'earliest' block contains %d transactions with inner transactions\n", len(earliestResult))
	})
}

func TestTransactionPreExec(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	EnsureContractsDeployed(t)

	contractAABI, err := abi.JSON(strings.NewReader(constants.ContractAABIJson))
	require.NoError(t, err)
	calldata, err := contractAABI.Pack("triggerCall")
	require.NoError(t, err)

	ethClient, err := ethclient.Dial(operations.DefaultL2NetworkURL)
	require.NoError(t, err)
	defer ethClient.Close()

	fromAddr := common.HexToAddress("0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266")

	// Fund the test address for gas validation tests
	ctx := context.Background()
	fundingAmount := uint256.NewInt(5000000000000000000)
	fundingTxHash := TransToken(t, ctx, ethClient, fundingAmount, fromAddr.String())
	t.Logf("Funded test address %s with 5 ETH, tx: %s", fromAddr.Hex(), fundingTxHash)

	t.Run("InnerTransactionTracking", func(t *testing.T) {
		// Test contract call with inner transactions
		txRequest := map[string]interface{}{
			"from": fromAddr.Hex(), "to": ContractAAddr.Hex(), "gas": "0x30000",
			"gasPrice": "0x4a817c800", "value": "0x0", "nonce": "0x1",
			"data": fmt.Sprintf("0x%x", calldata),
		}
		stateOverride := map[string]interface{}{
			fromAddr.Hex(): map[string]interface{}{"balance": "0x1000000000000000000000"},
		}

		result, err := operations.EthTransactionPreExec(txRequest, "latest", stateOverride)
		require.NoError(t, err)

		preExecResults, ok := result.([]interface{})
		require.True(t, ok, "Expected result to be an array")
		require.Len(t, preExecResults, 1)

		preExecResult, ok := preExecResults[0].(map[string]interface{})
		require.True(t, ok, "Expected preExecResult to be a map")
		require.NotNil(t, preExecResult["logs"])
		require.NotNil(t, preExecResult["stateDiff"])
		require.NotNil(t, preExecResult["gasUsed"])
		require.NotNil(t, preExecResult["blockNumber"])

		innerTxList, ok := preExecResult["innerTxs"].([]interface{})
		require.True(t, ok)
		require.GreaterOrEqual(t, len(innerTxList), 1)

		firstInnerTx := innerTxList[0].(map[string]interface{})
		require.Equal(t, "call", firstInnerTx["call_type"])
		require.Equal(t, strings.ToLower(ContractAAddr.Hex()), strings.ToLower(common.HexToAddress(firstInnerTx["to"].(string)).Hex()))
		require.Equal(t, "0xf18c388a", firstInnerTx["input"].(string))
		require.False(t, firstInnerTx["is_error"].(bool), "Expected is_error to be false for the first inner transaction")

		secondInnerTx := innerTxList[1].(map[string]interface{})
		require.Equal(t, "call", secondInnerTx["call_type"])
		require.Equal(t, strings.ToLower(ContractAAddr.Hex()), strings.ToLower(common.HexToAddress(secondInnerTx["from"].(string)).Hex()))
		require.Equal(t, strings.ToLower(ContractBAddr.Hex()), strings.ToLower(common.HexToAddress(secondInnerTx["to"].(string)).Hex()))
		require.Equal(t, "0x32e43a11", secondInnerTx["input"].(string))

		name := secondInnerTx["name"].(string)
		require.True(t, name[len(name)-1] >= '0' && name[len(name)-1] <= '9')
		require.False(t, secondInnerTx["is_error"].(bool), "Expected is_error to be false for the second inner transaction")
	})

	t.Run("GasValidation", func(t *testing.T) {
		balance, err := ethClient.BalanceAt(ctx, fromAddr, nil)
		require.NoError(t, err)
		balanceETH := new(big.Float).Quo(new(big.Float).SetInt(balance), new(big.Float).SetFloat64(1e18))
		t.Logf("Test address balance: %s ETH", balanceETH.String())

		t.Run("ContractCall", func(t *testing.T) {
			txRequest := map[string]interface{}{
				"from": fromAddr.Hex(), "to": ContractAAddr.Hex(), "gas": "0x100000",
				"gasPrice": "0x4a817c800", "value": "0x0", "nonce": "0x1",
				"data": fmt.Sprintf("0x%x", calldata),
			}

			// Get gasUsed from eth_transactionPreExec
			result, err := operations.EthTransactionPreExec(txRequest, "latest", nil)
			require.NoError(t, err)
			resultSlice := result.([]interface{})
			validationResult := ValidateResult(t, resultSlice[0], "gas_comparison_contract_call")
			preExecGasUsed := validationResult.GasUsed

			// Get gas estimate from eth_estimateGas
			estimateGasRequest := map[string]interface{}{
				"from": fromAddr.Hex(), "to": ContractAAddr.Hex(),
				"data": fmt.Sprintf("0x%x", calldata),
			}

			var estimateResult string
			err = ethClient.Client().Call(&estimateResult, "eth_estimateGas", estimateGasRequest, "latest")
			require.NoError(t, err)

			estimatedGas, err := strconv.ParseUint(strings.TrimPrefix(estimateResult, "0x"), 16, 64)
			require.NoError(t, err)

			// Validation: Both should be very close
			tolerance := uint64(5000) // Allow 5K gas difference for binary search precision

			require.Greater(t, preExecGasUsed, uint64(21000), "Gas should be > 21000 for contract call")
			require.Greater(t, estimatedGas, uint64(21000), "Estimated gas should be > 21000 for contract call")

			diff := uint64(0)
			if estimatedGas > preExecGasUsed {
				diff = estimatedGas - preExecGasUsed
			} else {
				diff = preExecGasUsed - estimatedGas
			}

			require.LessOrEqual(t, diff, tolerance,
				"Gas difference too large: preExec=%d, estimate=%d, diff=%d",
				preExecGasUsed, estimatedGas, diff)
		})

		t.Run("SimpleTransfer", func(t *testing.T) {
			transferTx := map[string]interface{}{
				"from": fromAddr.Hex(), "to": "0x742d35Cc4cF52f9234E96bC29d7F6a0c91d87b06",
				"value": "0x1000000000000000", "gas": "0x5208", // 21000 in hex
				"gasPrice": "0x4a817c800", "nonce": "0x2",
			}

			// PreExec gas usage
			result, err := operations.EthTransactionPreExec(transferTx, "latest", nil)
			require.NoError(t, err)
			resultSlice := result.([]interface{})
			validationResult := ValidateResult(t, resultSlice[0], "gas_comparison_transfer")
			transferPreExecGas := validationResult.GasUsed

			// Estimate gas usage
			transferEstimate := map[string]interface{}{
				"from": fromAddr.Hex(), "to": "0x742d35Cc4cF52f9234E96bC29d7F6a0c91d87b06",
				"value": "0x1000000000000000",
			}
			var estimateResult string
			err = ethClient.Client().Call(&estimateResult, "eth_estimateGas", transferEstimate, "latest")
			require.NoError(t, err)
			transferEstimatedGas, err := strconv.ParseUint(strings.TrimPrefix(estimateResult, "0x"), 16, 64)
			require.NoError(t, err)

			// Simple transfers should be exactly 21000 gas
			require.Equal(t, uint64(21000), transferPreExecGas, "Simple transfer should use exactly 21000 gas")
			require.Equal(t, uint64(21000), transferEstimatedGas, "Simple transfer estimate should be exactly 21000 gas")
		})

		t.Run("CreateOperation", func(t *testing.T) {
			factoryABI, err := abi.JSON(strings.NewReader(constants.ContractFactoryABIJson))
			require.NoError(t, err)
			createCalldata, err := factoryABI.Pack("createSimpleStorage", big.NewInt(999))
			require.NoError(t, err)

			createTx := map[string]interface{}{
				"from": fromAddr.Hex(), "to": FactoryAddr.Hex(), "gas": "0x200000",
				"gasPrice": "0x4a817c800", "value": "0x0", "nonce": "0x3",
				"data": fmt.Sprintf("0x%x", createCalldata),
			}

			// PreExec gas usage for CREATE
			result, err := operations.EthTransactionPreExec(createTx, "latest", nil)
			require.NoError(t, err)
			resultSlice := result.([]interface{})
			validationResult := ValidateResult(t, resultSlice[0], "gas_comparison_create")
			createPreExecGas := validationResult.GasUsed

			// Estimate gas for CREATE
			createEstimate := map[string]interface{}{
				"from": fromAddr.Hex(), "to": FactoryAddr.Hex(),
				"data": fmt.Sprintf("0x%x", createCalldata),
			}
			var estimateResult string
			err = ethClient.Client().Call(&estimateResult, "eth_estimateGas", createEstimate, "latest")
			require.NoError(t, err)
			createEstimatedGas, err := strconv.ParseUint(strings.TrimPrefix(estimateResult, "0x"), 16, 64)
			require.NoError(t, err)

			createDiff := uint64(0)
			if createEstimatedGas > createPreExecGas {
				createDiff = createEstimatedGas - createPreExecGas
			} else {
				createDiff = createPreExecGas - createEstimatedGas
			}

			require.LessOrEqual(t, createDiff, uint64(50000),
				"CREATE gas difference too large: preExec=%d, estimate=%d, diff=%d",
				createPreExecGas, createEstimatedGas, createDiff)
		})
	})

	t.Run("SimpleEthTransfer", func(t *testing.T) {
		transactionArgs := CreateBasicTransaction(
			"0x0165878a594ca255338adfa4d48449f69242eb8f",
			"0x1111111111111111111111111111111111111111",
			"0x0", "0x5208", "0x4a817c800", "0x0", "")

		stateOverrides := CreateDefaultStateOverrides()
		stateOverrides["0x0165878a594ca255338adfa4d48449f69242eb8f"].(map[string]interface{})["code"] = "0x608060405234801561001057600080fd5b50600436106100b45760003560e01c80638da5cb5b116100715780638da5cb5b1461013b57"

		result, err := operations.EthTransactionPreExec(transactionArgs, "latest", stateOverrides)
		require.NoError(t, err)
		resultSlice := result.([]interface{})
		validationResult := ValidateResult(t, resultSlice[0], "simple_eth_transfer")

		CheckSuccessfulResult(t, validationResult, "0x0165878a594ca255338adfa4d48449f69242eb8f", "simple ETH transfer")
		require.Empty(t, validationResult.InnerTxs, "Simple ETH transfer should not have inner transactions")
		require.Equal(t, validationResult.GasUsed, uint64(21000), "Simple ETH transfer should use exactly 21000 gas")
	})

	t.Run("EIP1559Transactions", func(t *testing.T) {
		testCases := []struct {
			name     string
			txFields map[string]interface{}
		}{
			{"maxFeePerGas_only", map[string]interface{}{"maxFeePerGas": "0x4a817c800"}},
			{"maxFeePerGas_with_maxPriorityFeePerGas", map[string]interface{}{
				"maxFeePerGas":         "0x4a817c800",
				"maxPriorityFeePerGas": "0x3b9aca00",
			}},
		}

		stateOverrides := CreateDefaultStateOverrides()

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Create base transaction with EIP-1559 fields
				transactionArgs := CreateBasicTransaction(
					"0x0165878a594ca255338adfa4d48449f69242eb8f",
					"0x1111111111111111111111111111111111111111",
					"0x0", "0x5208", "0x4a817c800", "0x0", "")

				for key, value := range tc.txFields {
					transactionArgs[key] = value
				}

				// Remove gasPrice field for EIP-1559 transactions
				delete(transactionArgs, "gasPrice")

				result, err := operations.EthTransactionPreExec(transactionArgs, "latest", stateOverrides)
				require.NoError(t, err)
				resultSlice := result.([]interface{})
				validationResult := ValidateResult(t, resultSlice[0], tc.name)
				CheckSuccessfulResult(t, validationResult, "0x0165878a594ca255338adfa4d48449f69242eb8f", "Support EIP-1559 transactions")
				require.GreaterOrEqual(t, validationResult.GasUsed, uint64(21000), "Transaction should use more than 21000 gas")
				require.Greater(t, validationResult.BlockNumber.Uint64(), uint64(1), "Block Number should be greater than 1")
				require.Empty(t, validationResult.InnerTxs, "Inner Transactions should be empty")
				require.Empty(t, validationResult.Logs, "Logs should be empty")
			})
		}
	})

	t.Run("EIP7702Transactions", func(t *testing.T) {
		testCases := []struct {
			name      string
			addresses []string
		}{
			{"authorizationList_single", []string{"0x1111111111111111111111111111111111111111"}},
			{"authorizationList_multiple", []string{"0x1111111111111111111111111111111111111111", "0x2222222222222222222222222222222222222222"}},
		}

		stateOverrides := CreateDefaultStateOverrides()

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Create base transaction with authorization list
				transactionArgs := CreateBasicTransaction(
					"0x0165878a594ca255338adfa4d48449f69242eb8f",
					"0x1111111111111111111111111111111111111111",
					"0x0", "0x15f90", "0x4a817c800", "0x0", "")
				transactionArgs["authorizationList"] = CreateAuthorizationList(tc.addresses)

				result, err := operations.EthTransactionPreExec(transactionArgs, "latest", stateOverrides)
				require.NoError(t, err)
				resultSlice := result.([]interface{})
				validationResult := ValidateResult(t, resultSlice[0], tc.name)
				CheckSuccessfulResult(t, validationResult, "0x0165878a594ca255338adfa4d48449f69242eb8f", "Support EIP-7702 transactions")
				require.GreaterOrEqual(t, validationResult.GasUsed, uint64(21000), "Transaction should use more than 21000 gas")
				require.Greater(t, validationResult.BlockNumber.Uint64(), uint64(1), "Block Number should be greater than 1")
				require.Empty(t, validationResult.InnerTxs, "Inner Transactions should be empty")
				require.Empty(t, validationResult.Logs, "Logs should be empty")
			})
		}
	})

	t.Run("ContractCallWithStateOverrides", func(t *testing.T) {
		contractBAddr := "0x2222222222222222222222222222222222222222"
		contractCAddr := "0x3333333333333333333333333333333333333333"

		stateOverrides := CreateDefaultStateOverrides()

		// Set up ContractB with ContractC's address in storage slot 1
		stateOverrides[contractBAddr] = map[string]interface{}{
			"code": "0x" + constants.ContractBBytecodeStr,
			"storage": map[string]interface{}{
				"0x0000000000000000000000000000000000000000000000000000000000000001": "0x0000000000000000000000003333333333333333333333333333333333333333", // ContractC address in slot 1
			},
		}

		// Set up ContractC with initial storage value
		stateOverrides[contractCAddr] = map[string]interface{}{
			"code": "0x" + constants.ContractCBytecodeStr,
			"storage": map[string]interface{}{
				"0x0000000000000000000000000000000000000000000000000000000000000000": "0x0000000000000000000000000000000000000000000000000000000000000064", // Initial value = 100
			},
		}

		transactionArgs := CreateBasicTransaction(
			"0x0165878a594ca255338adfa4d48449f69242eb8f",
			contractBAddr, "0x0", "0x200000", "0x4a817c800",
			"0x0", "0x32e43a11") // dummy() function selector

		result, err := operations.EthTransactionPreExec(transactionArgs, "latest", stateOverrides)
		require.NoError(t, err)
		resultSlice := result.([]interface{})
		validationResult := ValidateResult(t, resultSlice[0], "dummy_call")

		for _, txResult := range resultSlice {
			testName := "ContractCallWithStateOverrides"
			validationResult := ValidateResult(t, txResult, testName)

			CheckSuccessfulResult(t, validationResult, "0x0165878a594ca255338adfa4d48449f69242eb8f", testName)
		}

		require.Greater(t, validationResult.GasUsed, uint64(21000), "Contract call should use more than 21000 gas")
		require.Equal(t, 0, len(validationResult.InnerTxs), "Should have no inner transactions")
	})

	t.Run("NonceTooLow", func(t *testing.T) {
		// Test transactions with nonces lower than the account's current nonce
		transactions := []map[string]interface{}{
			CreateBasicTransaction(
				"0x0165878a594ca255338adfa4d48449f69242eb8f",
				"0x5fbdb2315678afecb367f032d93f642f64180aa3",
				"0x0", "0x30000", "0x4a817c800", "0x1", ""), // nonce = 1
			CreateBasicTransaction(
				"0x0165878a594ca255338adfa4d48449f69242eb8f",
				"0x5fbdb2315678afecb367f032d93f642f64180aa3",
				"0x0", "0x30000", "0x4a817c800", "0x2", ""), // nonce = 2
		}

		stateOverrides := map[string]interface{}{
			"0x0165878a594ca255338adfa4d48449f69242eb8f": map[string]interface{}{
				"balance": "0x56bc75e2d630eb20000",
				"code":    "0x608060405234801561001057600080fd5b50600436106100b45760003560e01c80638da5cb5b116100715780638da5cb5b1461013b57",
				"nonce":   "0x3", // Account nonce = 3
			},
		}

		var result []interface{}
		for _, tx := range transactions {
			txResult, err := operations.EthTransactionPreExec(tx, "latest", stateOverrides)
			require.NoError(t, err)
			txResultSlice := txResult.([]interface{})
			require.Len(t, txResultSlice, 1, "Expected single transaction result")
			result = append(result, txResultSlice[0])
		}

		for i, txResult := range result {
			testName := fmt.Sprintf("transaction_%d_nonce_too_low", i+1)
			validationResult := ValidateResult(t, txResult, testName)

			CheckErrorResult(t, validationResult, "nonce too low", testName)

			require.Equal(t, uint64(0), validationResult.GasUsed, "%s should use 0 gas when rejected", testName)
			require.Len(t, validationResult.InnerTxs, 0, "%s should have no inner transactions when rejected", testName)
		}
	})
}

func TestNewTransactionTypes(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	ctx := context.Background()
	client, err := ethclient.Dial(operations.DefaultL2NetworkURL)
	require.NoError(t, err)
	defer client.Close()

	EnsureContractsDeployed(t)

	chainID, err := client.ChainID(ctx)
	require.NoError(t, err)

	privateKey, err := crypto.HexToECDSA(TmpSenderPrivateKey)
	require.NoError(t, err)
	fromAddress := crypto.PubkeyToAddress(privateKey.PublicKey)
	toAddress := common.HexToAddress("0x1111111111111111111111111111111111111111")

	fundingAmount := uint256.NewInt(10000000000000000000)
	fundingTxHash := TransToken(t, ctx, client, fundingAmount, fromAddress.String())
	t.Logf("Funded test address %s with 10 ETH, tx: %s", fromAddress.Hex(), fundingTxHash)

	t.Run("EIP1559Transactions", func(t *testing.T) {
		t.Run("SimpleTransferWithMaxFeePerGas", func(t *testing.T) {
			nonce, err := client.PendingNonceAt(ctx, fromAddress)
			require.NoError(t, err)

			value := big.NewInt(1000000000000000000)
			gasLimit := uint64(21000)
			maxFeePerGas := big.NewInt(20000000000)
			maxPriorityFeePerGas := big.NewInt(1000000000)

			tx := types.NewTx(&types.DynamicFeeTx{
				ChainID:   chainID,
				Nonce:     nonce,
				GasTipCap: maxPriorityFeePerGas,
				GasFeeCap: maxFeePerGas,
				Gas:       gasLimit,
				To:        &toAddress,
				Value:     value,
				Data:      nil,
			})

			txHash, receipt := SignAndSendTransaction(ctx, t, client, tx, privateKey)

			require.Equal(t, types.ReceiptStatusSuccessful, receipt.Status, "Transaction should succeed")
			require.Equal(t, uint64(21000), receipt.GasUsed, "Should use exactly 21000 gas for simple transfer")
			require.Equal(t, uint8(types.DynamicFeeTxType), receipt.Type, "Receipt type should be DynamicFeeTx")

			t.Logf("EIP-1559 transfer successful: %s, gas used: %d", txHash.Hex(), receipt.GasUsed)
		})

		t.Run("ContractCallWithEIP1559", func(t *testing.T) {
			// Test EIP-1559 transaction calling a smart contract
			nonce, err := client.PendingNonceAt(ctx, fromAddress)
			require.NoError(t, err)

			contractAABI, err := abi.JSON(strings.NewReader(constants.ContractAABIJson))
			require.NoError(t, err)
			calldata, err := contractAABI.Pack("triggerCall")
			require.NoError(t, err)

			gasLimit := uint64(200000)
			maxFeePerGas := big.NewInt(20000000000)
			maxPriorityFeePerGas := big.NewInt(2000000000)

			// Create EIP-1559 contract call transaction
			tx := types.NewTx(&types.DynamicFeeTx{
				ChainID:   chainID,
				Nonce:     nonce,
				GasTipCap: maxPriorityFeePerGas,
				GasFeeCap: maxFeePerGas,
				Gas:       gasLimit,
				To:        &ContractAAddr,
				Value:     big.NewInt(0),
				Data:      calldata,
			})

			txHash, receipt := SignAndSendTransaction(ctx, t, client, tx, privateKey)

			require.Equal(t, types.ReceiptStatusSuccessful, receipt.Status, "Contract call should succeed")
			require.Greater(t, receipt.GasUsed, uint64(21000), "Contract call should use more than 21000 gas")

			t.Logf("EIP-1559 contract call successful: %s, gas used: %d", txHash.Hex(), receipt.GasUsed)
		})

		t.Run("eth_feeHistory", func(t *testing.T) {
			numberOfTransactions := 10
			transErc20TokenBatch(t, ctx, client, ERC20Addr, big.NewInt(Gwei), toAddress.String(), numberOfTransactions)

			// Get fee history for last 20 blocks
			blockCount := uint64(20)
			history, err := client.FeeHistory(ctx, blockCount, nil, nil)
			require.NoError(t, err)

			oldestBlockNum := history.OldestBlock.Uint64()
			t.Logf("Fee history oldest block: %d, blocks returned: %d", oldestBlockNum, blockCount)

			for i := uint64(0); i < blockCount; i++ {
				blockNum := big.NewInt(int64(oldestBlockNum) + int64(i))
				block, err := client.BlockByNumber(ctx, blockNum)
				require.NoError(t, err, "Failed to get block %d", blockNum.Uint64())

				feeHistoryBaseFee := history.BaseFee[i]
				blockHeaderBaseFee := block.BaseFee()
				require.Equal(t, feeHistoryBaseFee, blockHeaderBaseFee, "Base fee should match between fee history and block headers")
			}
		})
	})

	t.Run("EIP2930Transaction", func(t *testing.T) {
		nonce, err := client.PendingNonceAt(ctx, fromAddress)
		require.NoError(t, err)

		gasLimit := uint64(26000)
		gasPrice, err := client.SuggestGasPrice(ctx)
		require.NoError(t, err)

		// Create access list
		accessList := types.AccessList{
			{
				Address: toAddress,
				StorageKeys: []common.Hash{
					common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000000"),
				},
			},
		}

		tx := types.NewTx(&types.AccessListTx{
			ChainID:    chainID,
			Nonce:      nonce,
			GasPrice:   gasPrice,
			Gas:        gasLimit,
			To:         &toAddress,
			Value:      big.NewInt(0),
			Data:       nil,
			AccessList: accessList,
		})

		txHash, receipt := SignAndSendTransaction(ctx, t, client, tx, privateKey)

		require.Equal(t, types.ReceiptStatusSuccessful, receipt.Status, "EIP-2930 transaction with access list should succeed")
		require.Equal(t, uint8(types.AccessListTxType), receipt.Type, "Receipt type should be AccessListTx")

		t.Logf("EIP-2930 AccessListTx successful: %s, gas used: %d", txHash.Hex(), receipt.GasUsed)
	})

	t.Run("EIP7702Transaction", func(t *testing.T) {
		nonce, err := client.PendingNonceAt(ctx, fromAddress)
		require.NoError(t, err)

		// Create authorization to delegate the fromAddress's code to ContractC
		auth := types.SetCodeAuthorization{
			ChainID: *uint256.MustFromBig(chainID),
			Address: ContractCAddr,
			Nonce:   nonce,
		}

		signedAuth, err := types.SignSetCode(privateKey, auth)
		require.NoError(t, err)

		authority, err := signedAuth.Authority()
		require.NoError(t, err)
		require.Equal(t, fromAddress, authority, "Authority should match the signing address")

		gasLimit := uint64(100000)
		maxFeePerGas := big.NewInt(20000000000)
		maxPriorityFeePerGas := big.NewInt(1000000000)

		// Set the fromAddress to delegate to ContractC's code
		tx := types.NewTx(&types.SetCodeTx{
			ChainID:   uint256.MustFromBig(chainID),
			Nonce:     nonce,
			GasTipCap: uint256.MustFromBig(maxPriorityFeePerGas),
			GasFeeCap: uint256.MustFromBig(maxFeePerGas),
			Gas:       gasLimit,
			To:        toAddress,
			Value:     uint256.NewInt(0),
			Data:      nil,
			AuthList:  []types.SetCodeAuthorization{signedAuth},
		})

		require.Equal(t, uint8(types.SetCodeTxType), tx.Type(), "Transaction type should be SetCodeTx")

		authList := tx.SetCodeAuthorizations()
		require.NotNil(t, authList, "Authorization list should not be nil")
		require.Len(t, authList, 1, "Should have exactly 1 authorization")
		require.Equal(t, ContractCAddr, authList[0].Address, "Authorization should delegate to ContractC")

		txHash, receipt := SignAndSendTransaction(ctx, t, client, tx, privateKey)

		require.Equal(t, types.ReceiptStatusSuccessful, receipt.Status, "EIP-7702 transaction should succeed")
		require.Equal(t, uint8(types.SetCodeTxType), receipt.Type, "Receipt type should be SetCodeTx")
		require.Greater(t, receipt.GasUsed, uint64(21000), "Should use more than 21000 gas due to authorization processing")

		minedTx, _, err := client.TransactionByHash(ctx, txHash)
		require.NoError(t, err)

		txSigner := types.MakeSigner(operations.GetTestChainConfig(operations.DefaultL2ChainID), big.NewInt(1), 0)
		txFromAddress, err := types.Sender(txSigner, minedTx)
		require.NoError(t, err)
		require.Equal(t, fromAddress, txFromAddress, "Transaction from address should match the EOA address")
		require.Equal(t, toAddress, *minedTx.To(), "Transaction to address should match the recipient address")

		t.Logf("EIP-7702 transaction mined successfully in block %d, gas used: %d", receipt.BlockNumber.Uint64(), receipt.GasUsed)
	})

	t.Run("EIP3198Transaction", func(t *testing.T) {
		// BASEFEE (0x48) + STOP (0x00)
		var result hexutil.Bytes

		err := client.Client().CallContext(ctx, &result, "eth_call", map[string]interface{}{
			"to":   nil,      // Contract creation context
			"data": "0x4800", // BASEFEE + STOP
		}, "latest")
		require.NoError(t, err, "BASEFEE opcode should execute without error (EIP-3198 is supported)")

		latestBlock, err := client.BlockByNumber(ctx, nil)
		require.NoError(t, err)
		require.NotNil(t, latestBlock, "Block should not be nil")
		require.NotNil(t, latestBlock.BaseFee(), "Block should have a base fee field (EIP-3198)")

		t.Logf("Current block %d base fee: %s wei", latestBlock.Number().Uint64(), latestBlock.BaseFee().String())
	})

	t.Run("EIP3529Transaction", func(t *testing.T) {
		// Constructor bytecode: CALLER (0x33), SELFDESTRUCT (0xff)
		// This will self-destruct immediately during deployment, sending any value to the caller
		deployBytecode := common.Hex2Bytes("33ff")

		nonce, err := client.PendingNonceAt(ctx, fromAddress)
		require.NoError(t, err)

		gasPrice, err := client.SuggestGasPrice(ctx)
		require.NoError(t, err)

		deployTx := types.NewContractCreation(nonce, big.NewInt(0), 100000, gasPrice, deployBytecode)

		txHash, deployReceipt := SignAndSendTransaction(ctx, t, client, deployTx, privateKey)

		contractAddress := deployReceipt.ContractAddress
		t.Logf("Contract address (self-destructed): %s, tx hash: %s", contractAddress.Hex(), txHash.Hex())

		// Use debug_traceTransaction to get the refund counter
		var traceResult map[string]interface{}
		err = client.Client().CallContext(ctx, &traceResult, "debug_traceTransaction", txHash, map[string]interface{}{})
		require.NoError(t, err)

		refundCounter := GetRefundCounterFromTrace(traceResult, "SELFDESTRUCT")
		t.Logf("Refund counter after SELFDESTRUCT: %d", refundCounter)

		require.Equal(t, uint64(0), refundCounter, "SELFDESTRUCT refund should be 0 with EIP-3529")

		codeAfter, err := client.CodeAt(ctx, contractAddress, deployReceipt.BlockNumber)
		require.NoError(t, err)
		require.Empty(t, codeAfter, "Contract should have no code after SELFDESTRUCT")
	})

	t.Run("EIP4844Transactions", func(t *testing.T) {
		// OP Stack disables EIP-4844 blob transactions on Layer 2
		// So we check if blob fields are present in block data
		result, err := operations.EthGetBlockByNumber("latest", true)
		require.NoError(t, err)

		blockdata, _ := result.(map[string]interface{})
		require.NotNil(t, blockdata, "Block data should not be nil")

		blockGasUsed, exists := blockdata["blobGasUsed"].(string)
		require.True(t, exists, "Block should have blockGasUsed field")

		excessBlobGas, exists := blockdata["excessBlobGas"].(string)
		require.True(t, exists, "Block should have excessBlobGas field")

		t.Logf("Block gas used: %s", blockGasUsed)
		t.Logf("Block excess blob gas: %s", excessBlobGas)
	})
}

// Run with: RUN_OKPAY_PRIORITY_TEST=1 go test -v ./test/e2e/ -run TestOkPayPriority
func TestOkPayPriority(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}

	if os.Getenv("RUN_OKPAY_PRIORITY_TEST") != "1" {
		t.Skip("Skipping OkPay priority test. Set RUN_OKPAY_PRIORITY_TEST=1 to run")
	}

	ctx := context.Background()
	client, err := ethclient.Dial(operations.DefaultL2NetworkURL)
	require.NoError(t, err)
	defer client.Close()

	// Fund OkPay account
	richAccountNonce, err := client.PendingNonceAt(ctx, common.HexToAddress(operations.DefaultRichAddress))
	require.NoError(t, err)

	gasPrice, err := client.SuggestGasPrice(ctx)
	require.NoError(t, err)

	chainID, err := client.ChainID(ctx)
	require.NoError(t, err)

	recipient := common.HexToAddress(operations.DefaultOkPaySenderAddress)
	fundTx := types.NewTransaction(richAccountNonce, recipient, big.NewInt(1_000_000_000), params.TxGas, gasPrice, nil)

	richPrivateKey, err := crypto.HexToECDSA(strings.TrimPrefix(operations.DefaultRichPrivateKey, "0x"))
	require.NoError(t, err)

	signedFundTx, err := types.SignTx(fundTx, types.NewEIP155Signer(chainID), richPrivateKey)
	require.NoError(t, err)

	err = client.SendTransaction(ctx, signedFundTx)
	require.NoError(t, err)

	// Test 1: Basic Priority Override
	t.Run("BasicPriorityOverride", func(t *testing.T) {
		testOkPayBasicPriority(t, ctx, client)
	})

	// Test 2: Nonce-Based Ordering
	t.Run("NonceBasedOrdering", func(t *testing.T) {
		testOkPayNonceOrdering(t, ctx, client)
	})

	// Test 3: Transaction Limit Enforcement
	t.Run("TransactionLimit", func(t *testing.T) {
		testOkPayTransactionLimit(t, ctx, client)
	})
}

// testOkPayBasicPriority tests that OkPay transactions with low gas price
// are prioritized over normal transactions with high gas price
func testOkPayBasicPriority(t *testing.T, ctx context.Context, client *ethclient.Client) {
	// Get OkPay sender account
	okPayPrivateKey, err := crypto.HexToECDSA(strings.TrimPrefix(operations.DefaultOkPaySenderPrivateKey, "0x"))
	require.NoError(t, err)
	okPayAddress := crypto.PubkeyToAddress(okPayPrivateKey.PublicKey)

	normalPrivateKey, err := crypto.HexToECDSA(strings.TrimPrefix(operations.DefaultRichPrivateKey, "0x"))
	require.NoError(t, err)
	normalAddress := crypto.PubkeyToAddress(normalPrivateKey.PublicKey)

	recipient := common.HexToAddress("0x1111111111111111111111111111111111111111")

	okPayBalance, err := client.BalanceAt(ctx, okPayAddress, nil)
	require.NoError(t, err)
	require.True(t, okPayBalance.Cmp(big.NewInt(0)) > 0, "OkPay account must have balance")

	normalBalance, err := client.BalanceAt(ctx, normalAddress, nil)
	require.NoError(t, err)
	require.True(t, normalBalance.Cmp(big.NewInt(0)) > 0, "Normal account must have balance")

	okPayNonce, err := client.PendingNonceAt(ctx, okPayAddress)
	require.NoError(t, err)
	normalNonce, err := client.PendingNonceAt(ctx, normalAddress)
	require.NoError(t, err)

	chainID, err := client.ChainID(ctx)
	require.NoError(t, err)

	gasPrice, err := client.SuggestGasPrice(ctx)
	require.NoError(t, err)

	// Create normal tx with HIGH gas price (3x)
	normalHighGasPrice := new(big.Int).Mul(gasPrice, big.NewInt(3))
	normalTx := types.NewTransaction(normalNonce, recipient, big.NewInt(10), params.TxGas, normalHighGasPrice, nil)
	signedNormalTx, err := types.SignTx(normalTx, types.NewEIP155Signer(chainID), normalPrivateKey)
	require.NoError(t, err)

	// Create OkPay tx with LOW gas price (1x)
	okPayTx := types.NewTransaction(okPayNonce, recipient, big.NewInt(10), params.TxGas, gasPrice, nil)
	signedOkPayTx, err := types.SignTx(okPayTx, types.NewEIP155Signer(chainID), okPayPrivateKey)
	require.NoError(t, err)

	t.Logf("Normal tx: hash=%s, gasPrice=%s", signedNormalTx.Hash().Hex(), normalHighGasPrice.String())
	t.Logf("OkPay tx: hash=%s, gasPrice=%s", signedOkPayTx.Hash().Hex(), gasPrice.String())

	err = client.SendTransaction(ctx, signedNormalTx)
	require.NoError(t, err)

	err = client.SendTransaction(ctx, signedOkPayTx)
	require.NoError(t, err)

	okPayReceipt := waitForReceipt(t, ctx, client, signedOkPayTx.Hash(), 120*time.Second)
	normalReceipt := waitForReceipt(t, ctx, client, signedNormalTx.Hash(), 120*time.Second)

	require.Equal(t, types.ReceiptStatusSuccessful, okPayReceipt.Status)
	require.Equal(t, types.ReceiptStatusSuccessful, normalReceipt.Status)

	verifyTransactionPriority(t, ctx, client, okPayReceipt, normalReceipt, signedOkPayTx.Hash(), signedNormalTx.Hash(), "OkPay", "Normal")
}

// testOkPayNonceOrdering tests that multiple OkPay transactions are ordered by nonce
func testOkPayNonceOrdering(t *testing.T, ctx context.Context, client *ethclient.Client) {
	okPayPrivateKey, err := crypto.HexToECDSA(strings.TrimPrefix(operations.DefaultOkPaySenderPrivateKey, "0x"))
	require.NoError(t, err)
	okPayAddress := crypto.PubkeyToAddress(okPayPrivateKey.PublicKey)

	recipient := common.HexToAddress("0x2222222222222222222222222222222222222222")

	nonce, err := client.PendingNonceAt(ctx, okPayAddress)
	require.NoError(t, err)

	chainID, err := client.ChainID(ctx)
	require.NoError(t, err)

	gasPrice, err := client.SuggestGasPrice(ctx)
	require.NoError(t, err)

	// Create and send 3 OkPay transactions with increasing nonces
	var txHashes []common.Hash
	count := 3
	for i := range count {
		tx := types.NewTransaction(nonce+uint64(i), recipient, big.NewInt(10), params.TxGas, gasPrice, nil)
		signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), okPayPrivateKey)
		require.NoError(t, err)

		err = client.SendTransaction(ctx, signedTx)
		require.NoError(t, err)
		txHashes = append(txHashes, signedTx.Hash())

		t.Logf("Sent OkPay tx %d: %s", i+1, signedTx.Hash().Hex())
	}

	var receipts []*types.Receipt
	for _, hash := range txHashes {
		receipt := waitForReceipt(t, ctx, client, hash, 120*time.Second)
		receipts = append(receipts, receipt)
	}

	// All should be in same block or consecutive blocks
	blockNum := receipts[0].BlockNumber.Uint64()
	t.Logf("First OkPay tx in block %d", blockNum)

	block, err := client.BlockByNumber(ctx, receipts[0].BlockNumber)
	require.NoError(t, err)

	var indices []int
	for _, targetHash := range txHashes {
		for i, tx := range block.Transactions() {
			if tx.Hash() == targetHash {
				indices = append(indices, i)
				break
			}
		}
	}

	require.Equal(t, len(txHashes), len(indices), "Not all OkPay transactions found in block")

	for i := 1; i < len(indices); i++ {
		require.Less(t, indices[i-1], indices[i],
			"OkPay tx %d (sent earlier) should appear before OkPay tx %d in block", i, i+1)
	}

	t.Logf("Nonce ordering verified: OkPay txs at indices %v in block %d", indices, blockNum)
}

// testOkPayTransactionLimit tests that the BlockPriorityTxsLimit is enforced
func testOkPayTransactionLimit(t *testing.T, ctx context.Context, client *ethclient.Client) {
	okPayPrivateKey, err := crypto.HexToECDSA(strings.TrimPrefix(operations.DefaultOkPaySenderPrivateKey, "0x"))
	require.NoError(t, err)
	okPayAddress := crypto.PubkeyToAddress(okPayPrivateKey.PublicKey)

	normalPrivateKey, err := crypto.HexToECDSA(strings.TrimPrefix(operations.DefaultRichPrivateKey, "0x"))
	require.NoError(t, err)
	normalAddress := crypto.PubkeyToAddress(normalPrivateKey.PublicKey)

	recipient := common.HexToAddress("0x3333333333333333333333333333333333333333")

	okPayNonce, err := client.PendingNonceAt(ctx, okPayAddress)
	require.NoError(t, err)
	normalNonce, err := client.PendingNonceAt(ctx, normalAddress)
	require.NoError(t, err)

	chainID, err := client.ChainID(ctx)
	require.NoError(t, err)

	gasPrice, err := client.SuggestGasPrice(ctx)
	require.NoError(t, err)

	// Create 10 OkPay transaction (exceeds limit set of 5)
	var okPayTxHashes []common.Hash
	for i := 0; i < 10; i++ {
		tx := types.NewTransaction(okPayNonce+uint64(i), recipient, big.NewInt(10), params.TxGas, gasPrice, nil)
		signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), okPayPrivateKey)
		require.NoError(t, err)

		err = client.SendTransaction(ctx, signedTx)
		require.NoError(t, err)
		okPayTxHashes = append(okPayTxHashes, signedTx.Hash())
	}

	// Create 1 normal transaction with high gas price
	normalHighGasPrice := new(big.Int).Mul(gasPrice, big.NewInt(5))
	t.Logf("Normal high gas price: %s, gas price: %s", normalHighGasPrice.String(), gasPrice.String())
	normalTx := types.NewTransaction(normalNonce, recipient, big.NewInt(1000), params.TxGas, normalHighGasPrice, nil)
	signedNormalTx, err := types.SignTx(normalTx, types.NewEIP155Signer(chainID), normalPrivateKey)
	require.NoError(t, err)

	err = client.SendTransaction(ctx, signedNormalTx)
	require.NoError(t, err)

	t.Logf("Sent 10 OkPay txs and 1 high-value normal tx")

	// Wait for transactions
	firstOkPayReceipt := waitForReceipt(t, ctx, client, okPayTxHashes[0], 120*time.Second)
	_ = waitForReceipt(t, ctx, client, signedNormalTx.Hash(), 120*time.Second)

	// Get the block
	block, err := client.BlockByNumber(ctx, firstOkPayReceipt.BlockNumber)
	require.NoError(t, err)

	// Count how many OkPay txs appear before the normal tx
	normalIndex := -1
	okPayCount := 0

	for i, tx := range block.Transactions() {
		if tx.Hash() == signedNormalTx.Hash() {
			normalIndex = i
		}
		for _, okPayHash := range okPayTxHashes {
			if tx.Hash() == okPayHash && (normalIndex == -1 || i < normalIndex) {
				okPayCount++
			}
		}
	}

	t.Logf("Found %d OkPay txs before normal tx (index %d) in block %d", okPayCount, normalIndex, block.NumberU64())

	// The limit should be enforced - not all 10 OkPay txs should be prioritized
	// At least some OkPay txs should appear before normal tx, but not necessarily all
	require.Greater(t, okPayCount, 0, "At least some OkPay txs should be prioritized")
	if okPayCount < 10 {
		t.Logf("Transaction limit enforced: Only %d of 10 OkPay txs were prioritized", okPayCount)
	} else {
		t.Fatalf("Transaction limit not enforced: All 10 OkPay txs were prioritized past the limit of 5")
	}
}

func waitForReceipt(t *testing.T, ctx context.Context, client *ethclient.Client, txHash common.Hash, timeout time.Duration) *types.Receipt {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		receipt, err := client.TransactionReceipt(ctx, txHash)
		if err == nil && receipt != nil {
			t.Logf("Transaction %s mined in block %d", txHash.Hex(), receipt.BlockNumber.Uint64())
			return receipt
		}
		time.Sleep(2 * time.Second)
	}
	t.Fatalf("Transaction %s was not mined within timeout", txHash.Hex())
	return nil
}

func verifyTransactionPriority(t *testing.T, ctx context.Context, client *ethclient.Client,
	prioReceipt, normalReceipt *types.Receipt, prioHash, normalHash common.Hash, name1, name2 string) {
	require.LessOrEqual(t, prioReceipt.BlockNumber.Uint64(), normalReceipt.BlockNumber.Uint64(),
		"%s tx should be in same or earlier block than %s tx", name1, name2)

	if prioReceipt.BlockNumber.Uint64() == normalReceipt.BlockNumber.Uint64() {
		block, err := client.BlockByNumber(ctx, prioReceipt.BlockNumber)
		require.NoError(t, err)

		var prioIndex, normalIndex int = -1, -1
		for i, tx := range block.Transactions() {
			if tx.Hash() == prioHash {
				prioIndex = i
			}
			if tx.Hash() == normalHash {
				normalIndex = i
			}
		}

		require.NotEqual(t, -1, prioIndex, "%s transaction not found in block", name1)
		require.NotEqual(t, -1, normalIndex, "%s transaction not found in block", name2)
		require.Less(t, prioIndex, normalIndex, "%s tx (index %d) should come before %s tx (index %d) in block %d",
			name1, prioIndex, name2, normalIndex, block.NumberU64())

		t.Logf("Priority verified: %s[%d] < %s[%d] in block %d", name1, prioIndex, name2, normalIndex, block.NumberU64())
	}
}
