package tests

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInnerTransactionIntegration(t *testing.T) {
	testData := setupTestData(t)
	defer testData.Client.Close()

	// Test 1: Verify contract-to-contract call generates inner transactions
	t.Run("ContractCallGeneratesInnerTransactions", func(t *testing.T) {
		// Check if the transaction was successful first
		receipt, _ := testData.Client.GetTransactionReceipt(t, testData.ContractCallTxHash)
		require.Equal(t, uint64(1), receipt.Status, "Contract call transaction should be successful")

		innerTxs := testData.Client.GetInternalTransactions(t, testData.ContractCallTxHash)
		require.NotEmpty(t, innerTxs, "Contract-to-contract call should generate inner transactions")

		for _, innerTx := range innerTxs {
			require.False(t, innerTx.IsError, "Inner tx should not be an error")
			require.Greater(t, innerTx.Gas, uint64(0), "Inner tx should have gas limit")
			require.Greater(t, innerTx.GasUsed, uint64(0), "Inner tx should have gas used")
		}
	})

	// Test 2: Verify block-level inner transaction retrieval with hex string and uint64 block number formats
	t.Run("BlockLevelInnerTransactionRetrieval", func(t *testing.T) {
		blockNum := testData.ContractCallBlockNum

		// Test with decimal block number (converted to hex string for RPC)
		blockNumHexFromDec := fmt.Sprintf("0x%x", blockNum)
		blockInnerTxsDec := testData.Client.GetBlockInternalTransactions(t, blockNumHexFromDec)
		require.NotNil(t, blockInnerTxsDec, "Block should return a valid map")
		t.Logf("Block %d (as %s) contains %d transactions with inner transactions", blockNum, blockNumHexFromDec, len(blockInnerTxsDec))

		// Check if contract call transaction has inner transactions
		if innerTxsForContractCall, exists := blockInnerTxsDec[testData.ContractCallTxHash]; exists {
			t.Logf("Contract call transaction %s has %d inner transactions", testData.ContractCallTxHash.Hex(), len(innerTxsForContractCall))
		} else {
			t.Logf("Contract call transaction %s has no inner transactions in block map", testData.ContractCallTxHash.Hex())
		}

		// Test with hexadecimal block number
		blockNumHex := fmt.Sprintf("0x%x", blockNum)
		blockInnerTxsHex := testData.Client.GetBlockInternalTransactions(t, blockNumHex)
		require.Equal(t, len(blockInnerTxsDec), len(blockInnerTxsHex), "Decimal-as-hex and hex should return same number of transactions")

		// Compare the maps
		for txHash, innerTxsDec := range blockInnerTxsDec {
			innerTxsHex, exists := blockInnerTxsHex[txHash]
			require.True(t, exists, "Transaction %s should exist in both results", txHash.Hex())
			require.Equal(t, len(innerTxsDec), len(innerTxsHex), "Transaction %s should have same inner transaction count", txHash.Hex())
		}
	})

	// Test 3: Verify special block number formats work
	t.Run("SpecialBlockNumberFormats", func(t *testing.T) {
		// Test "latest" format
		latestResult := testData.Client.GetBlockInternalTransactions(t, "latest")
		require.NotNil(t, latestResult, "Latest block should return a valid map")
		t.Logf("'latest' block contains %d transactions with inner transactions", len(latestResult))

		// Test "earliest" format
		earliestResult := testData.Client.GetBlockInternalTransactions(t, "earliest")
		require.NotNil(t, earliestResult, "Earliest block should return a valid map")
		t.Logf("'earliest' block contains %d transactions with inner transactions", len(earliestResult))
	})

	// Test 4: Verify nested contract calls in same block
	t.Run("NestedContractCallsInSameBlock", func(t *testing.T) {
		triggerCallData := "0xf18c388a" // triggerCall() signature

		nestedCallTxHash := testData.Client.SendTransaction(t, testData.ContractAAddr, triggerCallData)
		t.Logf("Nested call transaction: %s", nestedCallTxHash.Hex())

		receipt, _ := testData.Client.GetTransactionReceipt(t, nestedCallTxHash)
		require.Equal(t, uint64(1), receipt.Status, "Nested call transaction should be successful")

		innerTxs := testData.Client.GetInternalTransactions(t, nestedCallTxHash)

		// First inner tx should be from devAccount to contract A (depth 0)
		innerTx1 := innerTxs[0]

		require.Equal(t, testData.Client.devAccount.Hex(), innerTx1.From, "First inner tx from should be devAccount")
		require.Equal(t, testData.ContractAAddr.Hex(), innerTx1.To, "First inner tx to should be ContractA")
		require.Equal(t, int64(0), innerTx1.Dept.Int64(), "First inner tx should be at depth 0")
		require.False(t, innerTx1.IsError, "First inner tx should not have errors")

		// Second inner tx should be A2 -> B2 (depth 1)
		innerTx2 := innerTxs[1]

		require.Equal(t, "call", innerTx2.CallType, "Second inner tx should be a call")
		require.Equal(t, testData.ContractAAddr.Hex(), innerTx2.From, "Second inner tx from should be ContractA")
		require.Equal(t, testData.ContractBAddr.Hex(), innerTx2.To, "Second inner tx to should be ContractB")
		require.Equal(t, int64(1), innerTx2.Dept.Int64(), "Second inner tx should be at depth 1")
		require.False(t, innerTx2.IsError, "Second inner tx should not have errors")

		// Third inner tx should be B -> C (depth 2)
		innerTx3 := innerTxs[2]

		require.Equal(t, "call", innerTx3.CallType, "Third inner tx should be a call")
		require.Equal(t, testData.ContractBAddr.Hex(), innerTx3.From, "Third inner tx from should be ContractB")
		require.Equal(t, testData.ContractCAddr.Hex(), innerTx3.To, "Third inner tx to should be ContractC")
		require.Equal(t, int64(2), innerTx3.Dept.Int64(), "Third inner tx should be at depth 2")
		require.False(t, innerTx3.IsError, "Third inner tx should not have errors")

		// Test block-level retrieval
		blockNum := receipt.BlockNumber.Uint64()
		blockInnerTxs := testData.Client.GetBlockInternalTransactions(t, fmt.Sprintf("0x%x", blockNum))

		if innerTxsInBlock, exists := blockInnerTxs[nestedCallTxHash]; exists {
			require.Equal(t, 3, len(innerTxsInBlock), "Block should contain 3 inner transactions")
		} else {
			t.Errorf("Nested call transaction not found in block inner transactions")
		}
	})

	// Test 5: Verify recursive calls generate multiple inner transactions
	t.Run("RecursiveCallGeneratesInnerTransactions", func(t *testing.T) {
		recursiveCallData := "0xec49254c0000000000000000000000000000000000000000000000000000000000000003"

		recursiveTxHash := testData.Client.SendTransaction(t, testData.ContractBAddr, recursiveCallData)
		t.Logf("Recursive call transaction: %s", recursiveTxHash.Hex())

		receipt, _ := testData.Client.GetTransactionReceipt(t, recursiveTxHash)
		require.Equal(t, uint64(1), receipt.Status, "Recursive call transaction should be successful")

		innerTxs := testData.Client.GetInternalTransactions(t, recursiveTxHash)

		require.Greater(t, len(innerTxs), 0, "Recursive call should generate inner transactions")

		// Log details of each inner transaction
		for i, innerTx := range innerTxs {
			if i >= 1 { // Check for second inner tx onwards
				require.Equal(t, innerTx.To, testData.ContractBAddr.Hex(), "Inner tx should be to ContractB")
				require.Equal(t, innerTx.From, testData.ContractBAddr.Hex(), "Inner tx should be from ContractB")
				require.Greater(t, innerTx.Gas, uint64(0), "Inner tx should have gas limit")
				require.Greater(t, innerTx.GasUsed, uint64(0), "Inner tx should have gas used")
				require.False(t, innerTx.IsError, "Inner tx should not have errors")
			}
		}

		// Test block-level retrieval
		blockNum := receipt.BlockNumber.Uint64()
		blockInnerTxs := testData.Client.GetBlockInternalTransactions(t, fmt.Sprintf("0x%x", blockNum))

		if innerTxsInBlock, exists := blockInnerTxs[recursiveTxHash]; exists {
			require.Equal(t, len(innerTxs), len(innerTxsInBlock), "Block should contain our recursive inner transactions")
		} else {
			t.Errorf("Recursive call transaction not found in block inner transactions")
		}
	})
}
