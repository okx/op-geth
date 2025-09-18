package eth

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

// setupRPCClient establishes connection to the dev node
func SetupRPCClient(t *testing.T) (*rpc.Client, context.Context) {
	client, err := rpc.Dial("http://localhost:8545")
	if err != nil {
		t.Skipf("No dev node available at http://localhost:8545 - start dev node to run integration tests: %v", err)
	}
	t.Cleanup(func() { client.Close() })
	return client, context.Background()
}

// executePreExec makes the RPC call and validates basic response structure
func ExecutePreExec(t *testing.T, client *rpc.Client, ctx context.Context, transactions []map[string]interface{}, stateOverrides map[string]interface{}) []interface{} {
	var result []interface{}
	err := client.CallContext(ctx, &result, "eth_transactionPreExec", transactions, nil, stateOverrides)
	require.NoError(t, err)
	require.NotNil(t, result, "Result should not be nil")
	require.Len(t, result, len(transactions), "Should have result for each transaction")
	return result
}

// validateResult validates a single transaction result and returns the result map
func ValidateResult(t *testing.T, result interface{}, testName string) map[string]interface{} {
	resultBytes, _ := json.MarshalIndent(result, "", "  ")
	t.Logf("Result for %s:\n%s", testName, string(resultBytes))

	resultMap, ok := result.(map[string]interface{})
	require.True(t, ok, "Result should be a map")

	if gasUsed, exists := resultMap["gasUsed"]; exists {
		t.Logf("Gas used: %v", gasUsed)
	}

	return resultMap
}

// checkSuccessfulResult validates that a result represents a successful transaction
func CheckSuccessfulResult(t *testing.T, resultMap map[string]interface{}, testName string) {
	errorMap := resultMap["error"].(map[string]interface{})
	errorCode := errorMap["code"].(float64)
	errorMsg := errorMap["msg"].(string)

	require.Equal(t, float64(0), errorCode, "%s should succeed", testName)
	require.Empty(t, errorMsg, "%s should not have error message", testName)
}

// checkErrorResult validates that a result contains a specific error
func CheckErrorResult(t *testing.T, resultMap map[string]interface{}, expectedError string, testName string) {
	errorMap := resultMap["error"].(map[string]interface{})
	errorMsg := errorMap["msg"].(string)

	require.Contains(t, errorMsg, expectedError, "Error should mention %s for %s", expectedError, testName)
}

// CreateBasicTransaction creates a standard transaction with common fields
func CreateBasicTransaction(from, to, value, gas, gasPrice, nonce, data string) map[string]interface{} {
	tx := map[string]interface{}{
		"from":     from,
		"to":       to,
		"value":    value,
		"gas":      gas,
		"gasPrice": gasPrice,
		"nonce":    nonce,
	}
	if data != "" {
		tx["data"] = data
	}
	return tx
}

// CreateDefaultStateOverrides creates common state overrides
func CreateDefaultStateOverrides() map[string]interface{} {
	return map[string]interface{}{
		"0x0165878a594ca255338adfa4d48449f69242eb8f": map[string]interface{}{
			"balance": "0x56bc75e2d630eb20000", // Large balance
			"nonce":   "0x0",
		},
	}
}

// CreateAuthorizationList creates a standard authorization list for EIP-7702 tests
func CreateAuthorizationList(addresses []string) []map[string]interface{} {
	var authList []map[string]interface{}
	for i, addr := range addresses {
		auth := map[string]interface{}{
			"chainId": "0x1",
			"address": addr,
			"nonce":   fmt.Sprintf("0x%x", i),
			"yParity": "0x1",
			"r":       "0x1234567890123456789012345678901234567890123456789012345678901234",
			"s":       "0x1234567890123456789012345678901234567890123456789012345678901234",
		}
		authList = append(authList, auth)
	}
	return authList
}
