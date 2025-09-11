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

// TestApplyTransaction_okx_DepositTransaction_NoIntercept tests deposit transaction with no interception
func TestApplyTransaction_okx_DepositTransaction_NoIntercept(t *testing.T) {
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
		TxHash:            common.Hash{4, 5, 6},
		Logs:              []*types.Log{},
	}

	sender := common.HexToAddress("0x1234567890123456789012345678901234567890")

	err := interceptBridgeTransactionIfNeeded(receipt, sender, config)
	if err != nil {
		t.Errorf("Expected no interception for deposit transaction with disabled config, got error: %v", err)
	}
}

// TestApplyTransaction_okx_DepositTransaction_Intercept tests deposit transaction with interception
func TestApplyTransaction_okx_DepositTransaction_Intercept(t *testing.T) {
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
		TxHash:            common.Hash{7, 8, 9},
		Logs:              []*types.Log{bridgeLog},
	}

	sender := common.HexToAddress("0x1234567890123456789012345678901234567890")

	err := interceptBridgeTransactionIfNeeded(receipt, sender, config)
	if err == nil {
		t.Error("Expected interception error for deposit transaction with target token")
	}
}

// TestCreateFailedDepositReceipt tests the createFailedDepositReceipt function
func TestCreateFailedDepositReceipt(t *testing.T) {
	to := common.HexToAddress("0x1234")
	tx := types.NewTx(&types.LegacyTx{
		Nonce:    0,
		To:       &to,
		Value:    big.NewInt(1000),
		Gas:      50000,
		GasPrice: big.NewInt(1000000000),
		Data:     []byte("test data"),
	})

	env := &environment{
		header: &types.Header{
			Number:   big.NewInt(1),
			GasUsed:  1000,
			GasLimit: 8000000,
			Time:     1234567890,
			Coinbase: common.HexToAddress("0x0000000000000000000000000000000000000000"),
		},
		tcount: 5,
	}

	originalGasUsed := env.header.GasUsed
	originalTxCount := env.tcount

	receipt := createFailedDepositReceipt(tx, env)

	if receipt == nil {
		t.Fatal("Expected receipt, got nil")
	}

	if receipt.Status != types.ReceiptStatusFailed {
		t.Errorf("Expected failed status, got: %d", receipt.Status)
	}

	if receipt.TxHash != tx.Hash() {
		t.Errorf("Expected tx hash %s, got %s", tx.Hash().Hex(), receipt.TxHash.Hex())
	}

	if receipt.TransactionIndex != uint(originalTxCount) {
		t.Errorf("Expected transaction index %d, got %d", originalTxCount, receipt.TransactionIndex)
	}

	if len(receipt.Logs) != 0 {
		t.Errorf("Expected empty logs for failed transaction, got %d logs", len(receipt.Logs))
	}

	if env.header.GasUsed <= originalGasUsed {
		t.Error("Expected gas usage to increase in environment")
	}

	if receipt.GasUsed == 0 {
		t.Error("Expected non-zero gas usage for failed receipt")
	}
}

// TestCreateFailedDepositReceipt_ContractCreation tests createFailedDepositReceipt with contract creation
func TestCreateFailedDepositReceipt_ContractCreation(t *testing.T) {
	tx := types.NewTx(&types.LegacyTx{
		Nonce:    0,
		To:       nil, // Contract creation
		Value:    big.NewInt(0),
		Gas:      100000,
		GasPrice: big.NewInt(1000000000),
		Data:     []byte("contract code"),
	})

	env := &environment{
		header: &types.Header{
			Number:   big.NewInt(1),
			GasUsed:  0,
			GasLimit: 8000000,
		},
		tcount: 0,
	}

	originalGasUsed := env.header.GasUsed

	receipt := createFailedDepositReceipt(tx, env)

	if receipt == nil {
		t.Fatal("Expected receipt, got nil")
	}

	if receipt.Status != types.ReceiptStatusFailed {
		t.Errorf("Expected failed status, got: %d", receipt.Status)
	}

	if env.header.GasUsed <= originalGasUsed {
		t.Error("Expected gas usage to increase for contract creation")
	}

	if tx.To() != nil {
		t.Error("Expected contract creation transaction (to == nil)")
	}
}

// TestCreateFailedDepositReceipt_WithAccessList tests createFailedDepositReceipt with access list transaction
func TestCreateFailedDepositReceipt_WithAccessList(t *testing.T) {
	accessList := types.AccessList{
		{Address: common.HexToAddress("0x1111111111111111111111111111111111111111"), StorageKeys: []common.Hash{{1}}},
	}

	to := common.HexToAddress("0x1234")
	tx := types.NewTx(&types.AccessListTx{
		ChainID:    big.NewInt(1),
		Nonce:      0,
		To:         &to,
		Value:      big.NewInt(1000),
		Gas:        50000,
		GasPrice:   big.NewInt(1000000000),
		Data:       []byte("test data"),
		AccessList: accessList,
	})

	env := &environment{
		header: &types.Header{
			Number:   big.NewInt(1),
			GasUsed:  0,
			GasLimit: 8000000,
		},
		tcount: 0,
	}

	originalGasUsed := env.header.GasUsed

	receipt := createFailedDepositReceipt(tx, env)

	if receipt == nil {
		t.Fatal("Expected receipt, got nil")
	}

	if receipt.Status != types.ReceiptStatusFailed {
		t.Errorf("Expected failed status, got: %d", receipt.Status)
	}

	if env.header.GasUsed <= originalGasUsed {
		t.Error("Expected gas usage to increase")
	}

	if receipt.Type != tx.Type() {
		t.Errorf("Expected receipt type %d, got %d", tx.Type(), receipt.Type)
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
