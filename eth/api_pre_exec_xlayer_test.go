package eth

import (
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/tests"
	"github.com/stretchr/testify/require"
)

// Simple ETH transfer test
func TestTransactionPreExec_SimpleEthTransfer(t *testing.T) {
	client, ctx := SetupRPCClient(t)

	transactionArgs := CreateBasicTransaction(
		"0x0165878a594ca255338adfa4d48449f69242eb8f",
		"0x1111111111111111111111111111111111111111",
		"0x0", "0x5208", "0x4a817c800", "0x0", "")

	stateOverrides := CreateDefaultStateOverrides()
	stateOverrides["0x0165878a594ca255338adfa4d48449f69242eb8f"].(map[string]interface{})["code"] = "0x608060405234801561001057600080fd5b50600436106100b45760003560e01c80638da5cb5b116100715780638da5cb5b1461013b57"

	result := ExecutePreExec(t, client, ctx, []map[string]interface{}{transactionArgs}, stateOverrides)
	resultMap := ValidateResult(t, result[0], "simple_eth_transfer")

	CheckSuccessfulResult(t, resultMap, "simple ETH transfer")
	require.Empty(t, resultMap["innerTxs"], "Simple ETH transfer should not have inner transactions")
}

// EIP 1559 transaction test
func TestTransactionPreExec_EIP1559(t *testing.T) {
	client, ctx := SetupRPCClient(t)

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

			result := ExecutePreExec(t, client, ctx, []map[string]interface{}{transactionArgs}, stateOverrides)
			resultMap := ValidateResult(t, result[0], tc.name)
			CheckSuccessfulResult(t, resultMap, "Support EIP-1559 transactions")

		})
	}
}

// EIP-7702 transaction test
func TestTransactionPreExec_EIP7702(t *testing.T) {
	client, ctx := SetupRPCClient(t)

	testCases := []struct {
		name      string
		addresses []string
	}{
		{"authorizationList_single", []string{"0x1111111111111111111111111111111111111111"}},
		{"authorizationList_multiple", []string{"0x1111111111111111111111111111111111111111", "0x2222222222222222222222222222222222222222"}},
	}

	stateOverrides := CreateDefaultStateOverrides()

	// Run all test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create base transaction with authorization list
			transactionArgs := CreateBasicTransaction(
				"0x0165878a594ca255338adfa4d48449f69242eb8f",
				"0x1111111111111111111111111111111111111111",
				"0x0", "0x15f90", "0x4a817c800", "0x0", "")
			transactionArgs["authorizationList"] = CreateAuthorizationList(tc.addresses)

			// Execute and validate
			result := ExecutePreExec(t, client, ctx, []map[string]interface{}{transactionArgs}, stateOverrides)
			resultMap := ValidateResult(t, result[0], tc.name)
			CheckSuccessfulResult(t, resultMap, "Support EIP-7702 transactions")
		})
	}
}

// Contract call with state overrides test
func TestTransactionPreExec_ContractCallWithStateOverrides(t *testing.T) {
	client, ctx := SetupRPCClient(t)

	// Contract addresses
	contractBAddr := "0x2222222222222222222222222222222222222222"
	contractCAddr := "0x3333333333333333333333333333333333333333"

	contractBBytecode := tests.ContractBBytecodeStr
	contractCBytecode := tests.ContractCBytecodeStr

	// Define test cases
	testCases := []struct {
		name string
		data string
	}{
		{"dummy_call", "0x32e43a11"}, // dummy() function selector
	}

	stateOverrides := CreateDefaultStateOverrides()

	// Set up ContractB with ContractC's address in storage slot 1
	stateOverrides[contractBAddr] = map[string]interface{}{
		"code": "0x" + contractBBytecode,
		"storage": map[string]interface{}{
			"0x0000000000000000000000000000000000000000000000000000000000000001": "0x0000000000000000000000003333333333333333333333333333333333333333", // ContractC address in slot 1
		},
	}

	// Set up ContractC with initial storage value
	stateOverrides[contractCAddr] = map[string]interface{}{
		"code": "0x" + contractCBytecode,
		"storage": map[string]interface{}{
			"0x0000000000000000000000000000000000000000000000000000000000000000": "0x0000000000000000000000000000000000000000000000000000000000000064", // Initial value = 100
		},
	}

	for i, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			transactionArgs := CreateBasicTransaction(
				"0x0165878a594ca255338adfa4d48449f69242eb8f",
				contractBAddr, "0x0", "0x200000", "0x4a817c800",
				fmt.Sprintf("0x%x", i), tc.data)

			result := ExecutePreExec(t, client, ctx, []map[string]interface{}{transactionArgs}, stateOverrides)
			resultMap := ValidateResult(t, result[0], tc.name)

			if gasUsed, exists := resultMap["gasUsed"]; exists {
				require.Greater(t, gasUsed.(float64), float64(21000), "Contract call should use more than 21000 gas")
			}

			if innerTxs, exists := resultMap["innerTxs"]; exists {
				if innerTxsArray, ok := innerTxs.([]interface{}); ok {
					require.Greater(t, len(innerTxsArray), 0, "Should have inner transactions from contract-to-contract calls")

					for j, innerTx := range innerTxsArray {
						if innerTxMap, ok := innerTx.(map[string]interface{}); ok {
							t.Logf("InnerTx %d: CallType=%v, From=%v, To=%v, Input=%v",
								j, innerTxMap["call_type"], innerTxMap["from"], innerTxMap["to"], innerTxMap["input"])
						}
					}
				}
			}

		})
	}
}

// Nonce too low test - transactions with nonces lower than account's current nonce should fail
func TestTransactionPreExec_NonceTooLow(t *testing.T) {
	client, ctx := SetupRPCClient(t)

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

	result := ExecutePreExec(t, client, ctx, transactions, stateOverrides)

	for i, txResult := range result {
		testName := fmt.Sprintf("transaction_%d_nonce_too_low", i+1)
		resultMap := ValidateResult(t, txResult, testName)

		CheckErrorResult(t, resultMap, "nonce too low", testName)

		if gasUsed, exists := resultMap["gasUsed"]; exists {
			require.Equal(t, float64(0), gasUsed.(float64), "%s should use 0 gas when rejected", testName)
		}

		if innerTxs, exists := resultMap["innerTxs"]; exists {
			if innerTxsArray, ok := innerTxs.([]interface{}); ok {
				require.Len(t, innerTxsArray, 0, "%s should have no inner transactions when rejected", testName)
			}
		}
	}
}
