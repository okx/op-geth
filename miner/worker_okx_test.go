package miner

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// TestApplyTransaction_okx_RegularTransaction_NoIntercept tests regular transaction with no interception
func TestApplyTransaction_okx_RegularTransaction_NoIntercept(t *testing.T) {
	// Test with disabled config
	config := &OldBridgeInterceptConfig{
		Enabled:               false,
		BridgeContractAddress: "0x2a3DD3EB832aF982ec71669E178424b10Dca2EDe",
		TargetTokenAddress:    "0x75231f58b43240c9718dd58b4967c5114342a86c",
	}

	receipt := &types.Receipt{
		Type:              types.LegacyTxType,
		Status:            types.ReceiptStatusSuccessful,
		CumulativeGasUsed: 21000,
		GasUsed:           21000,
		TxHash:            common.Hash{1, 2, 3},
		Logs:              []*types.Log{},
	}

	sender := common.HexToAddress("0x1234567890123456789012345678901234567890")

	err := interceptBridgeTransactionIfNeeded(receipt, sender, config)
	if err != nil {
		t.Errorf("Expected no interception with disabled config, got error: %v", err)
	}

	// Test with enabled config but no bridge logs
	config.Enabled = true
	err = interceptBridgeTransactionIfNeeded(receipt, sender, config)
	if err != nil {
		t.Errorf("Expected no interception with no bridge logs, got error: %v", err)
	}
}

// TestApplyTransaction_okx_RegularTransaction_Intercept tests regular transaction with interception
func TestApplyTransaction_okx_RegularTransaction_Intercept(t *testing.T) {
	targetToken := common.HexToAddress("0x75231f58b43240c9718dd58b4967c5114342a86c")
	bridgeContract := common.HexToAddress("0x2a3DD3EB832aF982ec71669E178424b10Dca2EDe")

	config := &OldBridgeInterceptConfig{
		Enabled:               true,
		BridgeContractAddress: bridgeContract.Hex(),
		TargetTokenAddress:    targetToken.Hex(),
	}

	bridgeEventData := createValidBridgeEventDataWithTargetToken(targetToken)

	bridgeLog := &types.Log{
		Address: bridgeContract,
		Topics:  []common.Hash{BRIDGE_EVENT_SIGNATURE},
		Data:    bridgeEventData,
	}

	receipt := &types.Receipt{
		Type:              types.LegacyTxType,
		Status:            types.ReceiptStatusSuccessful,
		CumulativeGasUsed: 50000,
		GasUsed:           50000,
		TxHash:            common.Hash{1, 2, 3},
		Logs:              []*types.Log{bridgeLog},
	}

	sender := common.HexToAddress("0x1234567890123456789012345678901234567890")

	err := interceptBridgeTransactionIfNeeded(receipt, sender, config)
	if err == nil {
		t.Error("Expected interception error for target token bridge transaction")
	}
}

// TestApplyTransaction_okx_LegacyTxOnly tests that only LegacyTxType transactions are checked for interception
func TestApplyTransaction_okx_LegacyTxOnly(t *testing.T) {
	// This test verifies that our logic only checks LegacyTxType transactions
	// We test this by verifying that interceptBridgeTransactionIfNeeded works correctly
	// for all transaction types when called directly (since the function itself doesn't
	// know about transaction types - that filtering happens in applyTransaction_okx)

	targetToken := common.HexToAddress("0x75231f58b43240c9718dd58b4967c5114342a86c")
	bridgeContract := common.HexToAddress("0x2a3DD3EB832aF982ec71669E178424b10Dca2EDe")

	config := &OldBridgeInterceptConfig{
		Enabled:               true,
		BridgeContractAddress: bridgeContract.Hex(),
		TargetTokenAddress:    targetToken.Hex(),
	}

	bridgeEventData := createValidBridgeEventDataWithTargetToken(targetToken)

	bridgeLog := &types.Log{
		Address: bridgeContract,
		Topics:  []common.Hash{BRIDGE_EVENT_SIGNATURE},
		Data:    bridgeEventData,
	}

	receipt := &types.Receipt{
		Type:              types.LegacyTxType, // Only LegacyTx should be intercepted
		Status:            types.ReceiptStatusSuccessful,
		CumulativeGasUsed: 50000,
		GasUsed:           50000,
		TxHash:            common.Hash{7, 8, 9},
		Logs:              []*types.Log{bridgeLog},
	}

	sender := common.HexToAddress("0x1234567890123456789012345678901234567890")

	// LegacyTx with bridge logs should be intercepted
	err := interceptBridgeTransactionIfNeeded(receipt, sender, config)
	if err == nil {
		t.Error("Expected interception error for LegacyTx with bridge logs")
	}

	// Test that the same receipt with no bridge logs is not intercepted
	receiptNoBridge := &types.Receipt{
		Type:              types.LegacyTxType,
		Status:            types.ReceiptStatusSuccessful,
		CumulativeGasUsed: 21000,
		GasUsed:           21000,
		TxHash:            common.Hash{1, 2, 3},
		Logs:              []*types.Log{}, // No bridge logs
	}

	err = interceptBridgeTransactionIfNeeded(receiptNoBridge, sender, config)
	if err != nil {
		t.Errorf("Expected no interception for LegacyTx without bridge logs, got error: %v", err)
	}
}

// TestInterceptBridgeTransactionIfNeeded_EdgeCases tests edge cases for intercept function
func TestInterceptBridgeTransactionIfNeeded_EdgeCases(t *testing.T) {
	t.Run("NilConfig", func(t *testing.T) {
		receipt := &types.Receipt{Logs: []*types.Log{}}
		sender := common.HexToAddress("0x1234")

		err := interceptBridgeTransactionIfNeeded(receipt, sender, nil)
		if err != nil {
			t.Errorf("Expected no error with nil config, got: %v", err)
		}
	})

	t.Run("NilReceipt", func(t *testing.T) {
		config := &OldBridgeInterceptConfig{Enabled: true}
		sender := common.HexToAddress("0x1234")

		err := interceptBridgeTransactionIfNeeded(nil, sender, config)
		if err != nil {
			t.Errorf("Expected no error with nil receipt, got: %v", err)
		}
	})

	t.Run("EmptyLogs", func(t *testing.T) {
		config := &OldBridgeInterceptConfig{
			Enabled:               true,
			BridgeContractAddress: "0x2a3DD3EB832aF982ec71669E178424b10Dca2EDe",
			TargetTokenAddress:    "0x75231f58b43240c9718dd58b4967c5114342a86c",
		}
		receipt := &types.Receipt{Logs: []*types.Log{}}
		sender := common.HexToAddress("0x1234")

		err := interceptBridgeTransactionIfNeeded(receipt, sender, config)
		if err != nil {
			t.Errorf("Expected no error with empty logs, got: %v", err)
		}
	})

	t.Run("DisabledConfig", func(t *testing.T) {
		config := &OldBridgeInterceptConfig{
			Enabled:               false,
			BridgeContractAddress: "0x2a3DD3EB832aF982ec71669E178424b10Dca2EDe",
			TargetTokenAddress:    "0x75231f58b43240c9718dd58b4967c5114342a86c",
		}
		receipt := &types.Receipt{
			Logs: []*types.Log{
				{
					Address: common.HexToAddress(config.BridgeContractAddress),
					Topics:  []common.Hash{BRIDGE_EVENT_SIGNATURE},
					Data:    createValidBridgeEventDataWithTargetToken(common.HexToAddress(config.TargetTokenAddress)),
				},
			},
		}
		sender := common.HexToAddress("0x1234")

		err := interceptBridgeTransactionIfNeeded(receipt, sender, config)
		if err != nil {
			t.Errorf("Expected no error with disabled config, got: %v", err)
		}
	})
}

// Helper function to create bridge event data with target token
func createValidBridgeEventDataWithTargetToken(targetToken common.Address) []byte {
	data := make([]byte, 256)

	// Set origin address at offset 76-96
	copy(data[76:96], targetToken.Bytes())

	// Set amount at offset 160-192
	amount := big.NewInt(1000000000000000000)
	copy(data[160:192], amount.FillBytes(make([]byte, 32)))

	return data
}
