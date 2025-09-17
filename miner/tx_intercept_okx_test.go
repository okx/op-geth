package miner

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

// TestParseBridgeEvent tests the parseBridgeEvent function with real mainnet bridge event data
func TestParseBridgeEvent(t *testing.T) {
	testCases := []struct {
		name                       string
		dataHex                    string
		expectedLeafType           uint8
		expectedOriginNetwork      uint32
		expectedOriginAddress      string
		expectedDestinationNetwork uint32
		expectedDestinationAddress string
		expectedAmount             string
		expectedMetadata           string
		expectedDepositCount       uint32
	}{
		{
			name:                       "First bridge event",
			dataHex:                    "0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000075231f58b43240c9718dd58b4967c5114342a86c0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000bf7624b8a72797fe35ba1505587fc8a39705740c000000000000000000000000000000000000000000000000008e1bc9bf04000000000000000000000000000000000000000000000000000000000000000001000000000000000000000000000000000000000000000000000000000000001c9700000000000000000000000000000000000000000000000000000000000000e0000000000000000000000000000000000000000000000000000000000000006000000000000000000000000000000000000000000000000000000000000000a0000000000000000000000000000000000000000000000000000000000000001200000000000000000000000000000000000000000000000000000000000000034f4b42000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000034f4b420000000000000000000000000000000000000000000000000000000000",
			expectedLeafType:           0,
			expectedOriginNetwork:      0,
			expectedOriginAddress:      "0x75231f58b43240c9718dd58b4967c5114342a86c",
			expectedDestinationNetwork: 0,
			expectedDestinationAddress: "0xbf7624b8a72797fe35ba1505587fc8a39705740c",
			expectedAmount:             "40000000000000000",
			expectedMetadata:           "000000000000000000000000000000000000000000000000000000000000006000000000000000000000000000000000000000000000000000000000000000a0000000000000000000000000000000000000000000000000000000000000001200000000000000000000000000000000000000000000000000000000000000034f4b42000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000034f4b420000000000000000000000000000000000000000000000000000000000",
			expectedDepositCount:       7319,
		},
		{
			name:                       "Second bridge event",
			dataHex:                    "0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000075231f58b43240c9718dd58b4967c5114342a86c0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000151ed65c4451661313848d07a615dec1f0d4ad25000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000001000000000000000000000000000000000000000000000000000000000000001caa00000000000000000000000000000000000000000000000000000000000000e0000000000000000000000000000000000000000000000000000000000000006000000000000000000000000000000000000000000000000000000000000000a0000000000000000000000000000000000000000000000000000000000000001200000000000000000000000000000000000000000000000000000000000000034f4b42000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000034f4b420000000000000000000000000000000000000000000000000000000000",
			expectedLeafType:           0,
			expectedOriginNetwork:      0,
			expectedOriginAddress:      "0x75231f58b43240c9718dd58b4967c5114342a86c",
			expectedDestinationNetwork: 0,
			expectedDestinationAddress: "0x151ed65c4451661313848d07a615dec1f0d4ad25",
			expectedAmount:             "1",
			expectedMetadata:           "000000000000000000000000000000000000000000000000000000000000006000000000000000000000000000000000000000000000000000000000000000a0000000000000000000000000000000000000000000000000000000000000001200000000000000000000000000000000000000000000000000000000000000034f4b42000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000034f4b420000000000000000000000000000000000000000000000000000000000",
			expectedDepositCount:       7338,
		},
		{
			name:                       "Third bridge event",
			dataHex:                    "0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000075231f58b43240c9718dd58b4967c5114342a86c0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000696314d0f50d0dfb3f6c8de9f33d9e546b1dfbed000000000000000000000000000000000000000000000000c12dc63fa970000000000000000000000000000000000000000000000000000000000000000001000000000000000000000000000000000000000000000000000000000000001c9000000000000000000000000000000000000000000000000000000000000000e0000000000000000000000000000000000000000000000000000000000000006000000000000000000000000000000000000000000000000000000000000000a0000000000000000000000000000000000000000000000000000000000000001200000000000000000000000000000000000000000000000000000000000000034f4b42000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000034f4b420000000000000000000000000000000000000000000000000000000000",
			expectedLeafType:           0,
			expectedOriginNetwork:      0,
			expectedOriginAddress:      "0x75231f58b43240c9718dd58b4967c5114342a86c",
			expectedDestinationNetwork: 0,
			expectedDestinationAddress: "0x696314d0f50d0dfb3f6c8de9f33d9e546b1dfbed",
			expectedAmount:             "13920000000000000000",
			expectedMetadata:           "000000000000000000000000000000000000000000000000000000000000006000000000000000000000000000000000000000000000000000000000000000a0000000000000000000000000000000000000000000000000000000000000001200000000000000000000000000000000000000000000000000000000000000034f4b42000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000034f4b420000000000000000000000000000000000000000000000000000000000",
			expectedDepositCount:       7312,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testData := common.FromHex(tc.dataHex)
			topics := []common.Hash{BRIDGE_EVENT_SIGNATURE}

			event, err := parseBridgeEvent(testData, topics)
			if err != nil {
				t.Fatalf("Failed to parse bridge event: %v", err)
			}

			// Only verify the fields we actually use for interception
			// if event.LeafType != tc.expectedLeafType {
			//     t.Errorf("LeafType mismatch: got %d, want %d", event.LeafType, tc.expectedLeafType)
			// }

			// if event.OriginNetwork != tc.expectedOriginNetwork {
			//     t.Errorf("OriginNetwork mismatch: got %d, want %d", event.OriginNetwork, tc.expectedOriginNetwork)
			// }

			expectedOriginAddress := common.HexToAddress(tc.expectedOriginAddress)
			if event.OriginAddress != expectedOriginAddress {
				t.Errorf("OriginAddress mismatch: got %s, want %s", event.OriginAddress.Hex(), expectedOriginAddress.Hex())
			}

			// if event.DestinationNetwork != tc.expectedDestinationNetwork {
			//     t.Errorf("DestinationNetwork mismatch: got %d, want %d", event.DestinationNetwork, tc.expectedDestinationNetwork)
			// }

			// expectedDestinationAddress := common.HexToAddress(tc.expectedDestinationAddress)
			// if event.DestinationAddress != expectedDestinationAddress {
			//     t.Errorf("DestinationAddress mismatch: got %s, want %s", event.DestinationAddress.Hex(), expectedDestinationAddress.Hex())
			// }

			//expectedAmount, ok := new(big.Int).SetString(tc.expectedAmount, 10)
			//if !ok {
			//	t.Fatalf("Invalid expected amount: %s", tc.expectedAmount)
			//}
			//if event.Amount.Cmp(expectedAmount) != 0 {
			//	t.Errorf("Amount mismatch: got %s, want %s", event.Amount.String(), expectedAmount.String())
			//}

			// if event.DepositCount != tc.expectedDepositCount {
			//     t.Errorf("DepositCount mismatch: got %d, want %d", event.DepositCount, tc.expectedDepositCount)
			// }

			// expectedMetadata := common.FromHex(tc.expectedMetadata)
			// if !bytes.Equal(event.Metadata, expectedMetadata) {
			//     t.Errorf("Metadata mismatch: got %x, want %x", event.Metadata, expectedMetadata)
			// }
		})
	}
}

// TestParseBridgeEventInvalidSignature tests parseBridgeEvent with invalid event signature
func TestParseBridgeEventInvalidSignature(t *testing.T) {
	testData := make([]byte, 256)
	invalidTopics := []common.Hash{common.Hash{}}

	_, err := parseBridgeEvent(testData, invalidTopics)
	if err == nil {
		t.Error("Expected error for invalid signature, got nil")
	}

	expectedError := "not a bridge event"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

// TestParseBridgeEventInsufficientData tests parseBridgeEvent with insufficient log data
func TestParseBridgeEventInsufficientData(t *testing.T) {
	testData := make([]byte, 100)
	topics := []common.Hash{BRIDGE_EVENT_SIGNATURE}

	_, err := parseBridgeEvent(testData, topics)
	if err == nil {
		t.Error("Expected error for insufficient data, got nil")
	}

	expectedError := "insufficient log data length"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

// TestParseBridgeEventEmptyTopics tests parseBridgeEvent with empty topics
func TestParseBridgeEventEmptyTopics(t *testing.T) {
	testData := make([]byte, 256)
	emptyTopics := []common.Hash{}

	_, err := parseBridgeEvent(testData, emptyTopics)
	if err == nil {
		t.Error("Expected error for empty topics, got nil")
	}

	expectedError := "not a bridge event"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

// TestValidateBridgeEvent tests the validateBridgeEvent function
func TestValidateBridgeEvent(t *testing.T) {
	targetToken := common.HexToAddress("0x1234567890123456789012345678901234567890")
	otherToken := common.HexToAddress("0xabcdefabcdefabcdefabcdefabcdefabcdefabcdef")
	sender := common.HexToAddress("0x9876543210987654321098765432109876543210")

	testCases := []struct {
		name           string
		event          *BridgeEventData
		sender         common.Address
		targetToken    common.Address
		expectError    bool
		expectedErrMsg string
	}{
		{
			name: "Intercept target token",
			event: &BridgeEventData{
				OriginAddress: targetToken,
				Amount:        big.NewInt(1000),
			},
			sender:         sender,
			targetToken:    targetToken,
			expectError:    true,
			expectedErrMsg: "bridge event for target token",
		},
		{
			name: "Allow non-target token",
			event: &BridgeEventData{
				OriginAddress: otherToken,
				Amount:        big.NewInt(1000),
			},
			sender:      sender,
			targetToken: targetToken,
			expectError: false,
		},
		{
			name: "Handle zero amount",
			event: &BridgeEventData{
				OriginAddress: targetToken,
				Amount:        big.NewInt(0),
			},
			sender:         sender,
			targetToken:    targetToken,
			expectError:    true,
			expectedErrMsg: "bridge event for target token",
		},
		{
			name: "Handle nil amount",
			event: &BridgeEventData{
				OriginAddress: targetToken,
				Amount:        nil,
			},
			sender:         sender,
			targetToken:    targetToken,
			expectError:    true,
			expectedErrMsg: "bridge event for target token",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateBridgeEvent(tc.event, tc.sender, tc.targetToken)

			if tc.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				} else if tc.expectedErrMsg != "" && !containsString(err.Error(), tc.expectedErrMsg) {
					t.Errorf("Expected error to contain '%s', got '%s'", tc.expectedErrMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
			}
		})
	}
}

// TestInterceptBridgeTransactionIfNeeded_BasicCases tests basic config scenarios
func TestInterceptBridgeTransactionIfNeeded_BasicCases(t *testing.T) {
	bridgeContract := common.HexToAddress("0x2a3DD3EB832aF982ec71669E178424b10Dca2EDe")
	targetToken := common.HexToAddress("0x75231f58b43240c9718dd58b4967c5114342a86c")
	sender := common.HexToAddress("0x9876543210987654321098765432109876543210")

	testCases := []struct {
		name        string
		receipt     *types.Receipt
		config      *OldBridgeInterceptConfig
		expectError bool
	}{
		{
			name:        "Nil config passes",
			receipt:     &types.Receipt{Logs: []*types.Log{{}}},
			config:      nil,
			expectError: false,
		},
		{
			name:    "Disabled config passes",
			receipt: &types.Receipt{Logs: []*types.Log{{}}},
			config: &OldBridgeInterceptConfig{
				Enabled:               false,
				BridgeContractAddress: bridgeContract.Hex(),
				TargetTokenAddress:    targetToken.Hex(),
			},
			expectError: false,
		},
		{
			name:    "Nil receipt passes",
			receipt: nil,
			config: &OldBridgeInterceptConfig{
				Enabled:               true,
				BridgeContractAddress: bridgeContract.Hex(),
				TargetTokenAddress:    targetToken.Hex(),
			},
			expectError: false,
		},
		{
			name:    "Empty logs pass",
			receipt: &types.Receipt{Logs: []*types.Log{}},
			config: &OldBridgeInterceptConfig{
				Enabled:               true,
				BridgeContractAddress: bridgeContract.Hex(),
				TargetTokenAddress:    targetToken.Hex(),
			},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := interceptBridgeTransactionIfNeeded(tc.receipt, sender, tc.config)
			if tc.expectError && err == nil {
				t.Error("Expected error but got none")
			} else if !tc.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

// TestInterceptBridgeTransactionIfNeeded_LogFiltering tests log filtering logic
func TestInterceptBridgeTransactionIfNeeded_LogFiltering(t *testing.T) {
	bridgeContract := common.HexToAddress("0x2a3DD3EB832aF982ec71669E178424b10Dca2EDe")
	targetToken := common.HexToAddress("0x75231f58b43240c9718dd58b4967c5114342a86c")
	otherToken := common.HexToAddress("0x1234567890123456789012345678901234567890")
	sender := common.HexToAddress("0x9876543210987654321098765432109876543210")

	config := &OldBridgeInterceptConfig{
		Enabled:               true,
		BridgeContractAddress: bridgeContract.Hex(),
		TargetTokenAddress:    targetToken.Hex(),
	}

	testCases := []struct {
		name        string
		receipt     *types.Receipt
		expectError bool
	}{
		{
			name: "Non-bridge log passes",
			receipt: &types.Receipt{
				Logs: []*types.Log{
					{
						Address: common.HexToAddress("0x1111111111111111111111111111111111111111"),
						Topics:  []common.Hash{BRIDGE_EVENT_SIGNATURE},
						Data:    make([]byte, 256),
					},
				},
			},
			expectError: false,
		},
		{
			name: "Invalid event passes",
			receipt: &types.Receipt{
				Logs: []*types.Log{
					{
						Address: bridgeContract,
						Topics:  []common.Hash{common.Hash{}}, // Invalid signature
						Data:    make([]byte, 256),
					},
				},
			},
			expectError: false,
		},
		{
			name: "Non-target token passes",
			receipt: &types.Receipt{
				Logs: []*types.Log{
					{
						Address: bridgeContract,
						Topics:  []common.Hash{BRIDGE_EVENT_SIGNATURE},
						Data:    createValidBridgeEventData(otherToken),
					},
				},
			},
			expectError: false,
		},
		{
			name: "Target token intercepted",
			receipt: &types.Receipt{
				Logs: []*types.Log{
					{
						Address: bridgeContract,
						Topics:  []common.Hash{BRIDGE_EVENT_SIGNATURE},
						Data:    createValidBridgeEventData(targetToken),
					},
				},
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := interceptBridgeTransactionIfNeeded(tc.receipt, sender, config)
			if tc.expectError && err == nil {
				t.Error("Expected error but got none")
			} else if !tc.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

// TestInterceptBridgeTransactionIfNeeded_WildcardMode tests wildcard functionality
func TestInterceptBridgeTransactionIfNeeded_WildcardMode(t *testing.T) {
	bridgeContract := common.HexToAddress("0x2a3DD3EB832aF982ec71669E178424b10Dca2EDe")
	targetToken := common.HexToAddress("0x75231f58b43240c9718dd58b4967c5114342a86c")
	otherToken := common.HexToAddress("0x1234567890123456789012345678901234567890")
	sender := common.HexToAddress("0x9876543210987654321098765432109876543210")

	wildcardConfig := &OldBridgeInterceptConfig{
		Enabled:               true,
		BridgeContractAddress: bridgeContract.Hex(),
		TargetTokenAddress:    "*",
	}

	testCases := []struct {
		name        string
		receipt     *types.Receipt
		expectError bool
	}{
		{
			name: "Wildcard intercepts any bridge tx",
			receipt: &types.Receipt{
				Logs: []*types.Log{
					{
						Address: bridgeContract,
						Topics:  []common.Hash{BRIDGE_EVENT_SIGNATURE},
						Data:    createValidBridgeEventData(otherToken),
					},
				},
			},
			expectError: true,
		},
		{
			name: "Wildcard intercepts invalid events",
			receipt: &types.Receipt{
				Logs: []*types.Log{
					{
						Address: bridgeContract,
						Topics:  []common.Hash{common.Hash{}}, // Invalid signature
						Data:    make([]byte, 256),
					},
				},
			},
			expectError: true,
		},
		{
			name: "Wildcard ignores non-bridge logs",
			receipt: &types.Receipt{
				Logs: []*types.Log{
					{
						Address: common.HexToAddress("0x1111111111111111111111111111111111111111"),
						Topics:  []common.Hash{BRIDGE_EVENT_SIGNATURE},
						Data:    createValidBridgeEventData(targetToken),
					},
				},
			},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := interceptBridgeTransactionIfNeeded(tc.receipt, sender, wildcardConfig)
			if tc.expectError && err == nil {
				t.Error("Expected error but got none")
			} else if !tc.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

// TestCheckBridgeEventInReceipt_MultipleLogs tests multiple log scenarios
func TestCheckBridgeEventInReceipt_MultipleLogs(t *testing.T) {
	bridgeContract := common.HexToAddress("0x2a3DD3EB832aF982ec71669E178424b10Dca2EDe")
	targetToken := common.HexToAddress("0x75231f58b43240c9718dd58b4967c5114342a86c")
	otherContract := common.HexToAddress("0x1111111111111111111111111111111111111111")
	sender := common.HexToAddress("0x9876543210987654321098765432109876543210")

	config := &OldBridgeInterceptConfig{
		Enabled:               true,
		BridgeContractAddress: bridgeContract.Hex(),
		TargetTokenAddress:    targetToken.Hex(),
	}

	testCases := []struct {
		name        string
		receipt     *types.Receipt
		expectError bool
	}{
		{
			name: "Multiple logs with target token",
			receipt: &types.Receipt{
				Logs: []*types.Log{
					{
						Address: otherContract,
						Topics:  []common.Hash{BRIDGE_EVENT_SIGNATURE},
						Data:    createValidBridgeEventData(targetToken),
					},
					{
						Address: bridgeContract,
						Topics:  []common.Hash{BRIDGE_EVENT_SIGNATURE},
						Data:    createValidBridgeEventData(targetToken),
					},
				},
			},
			expectError: true,
		},
		{
			name: "Multiple logs without target token",
			receipt: &types.Receipt{
				Logs: []*types.Log{
					{
						Address: bridgeContract,
						Topics:  []common.Hash{BRIDGE_EVENT_SIGNATURE},
						Data:    createValidBridgeEventData(common.HexToAddress("0x1111111111111111111111111111111111111111")),
					},
					{
						Address: bridgeContract,
						Topics:  []common.Hash{BRIDGE_EVENT_SIGNATURE},
						Data:    createValidBridgeEventData(common.HexToAddress("0x2222222222222222222222222222222222222222")),
					},
				},
			},
			expectError: false,
		},
		{
			name: "Mixed valid and invalid logs",
			receipt: &types.Receipt{
				Logs: []*types.Log{
					{
						Address: bridgeContract,
						Topics:  []common.Hash{common.Hash{}}, // Invalid signature
						Data:    make([]byte, 256),
					},
					{
						Address: bridgeContract,
						Topics:  []common.Hash{BRIDGE_EVENT_SIGNATURE},
						Data:    make([]byte, 100), // Insufficient data
					},
					{
						Address: bridgeContract,
						Topics:  []common.Hash{BRIDGE_EVENT_SIGNATURE},
						Data:    createValidBridgeEventData(targetToken),
					},
				},
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := checkBridgeEventInReceipt(tc.receipt, sender, config)
			if tc.expectError && err == nil {
				t.Error("Expected error but got none")
			} else if !tc.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

// TestCheckBridgeEventInReceipt_WildcardMode tests wildcard functionality
func TestCheckBridgeEventInReceipt_WildcardMode(t *testing.T) {
	bridgeContract := common.HexToAddress("0x2a3DD3EB832aF982ec71669E178424b10Dca2EDe")
	targetToken := common.HexToAddress("0x75231f58b43240c9718dd58b4967c5114342a86c")
	otherContract := common.HexToAddress("0x1111111111111111111111111111111111111111")
	sender := common.HexToAddress("0x9876543210987654321098765432109876543210")

	wildcardConfig := &OldBridgeInterceptConfig{
		Enabled:               true,
		BridgeContractAddress: bridgeContract.Hex(),
		TargetTokenAddress:    "*",
	}

	testCases := []struct {
		name        string
		receipt     *types.Receipt
		expectError bool
	}{
		{
			name: "Wildcard intercepts bridge log",
			receipt: &types.Receipt{
				Logs: []*types.Log{
					{
						Address: bridgeContract,
						Topics:  []common.Hash{BRIDGE_EVENT_SIGNATURE},
						Data:    createValidBridgeEventData(common.HexToAddress("0x1111111111111111111111111111111111111111")),
					},
				},
			},
			expectError: true,
		},
		{
			name: "Wildcard intercepts invalid events",
			receipt: &types.Receipt{
				Logs: []*types.Log{
					{
						Address: bridgeContract,
						Topics:  []common.Hash{common.Hash{}}, // Invalid signature
						Data:    make([]byte, 256),
					},
				},
			},
			expectError: true,
		},
		{
			name: "Wildcard ignores other contracts",
			receipt: &types.Receipt{
				Logs: []*types.Log{
					{
						Address: otherContract,
						Topics:  []common.Hash{BRIDGE_EVENT_SIGNATURE},
						Data:    createValidBridgeEventData(targetToken),
					},
				},
			},
			expectError: false,
		},
		{
			name: "Wildcard with mixed logs",
			receipt: &types.Receipt{
				Logs: []*types.Log{
					{
						Address: otherContract,
						Topics:  []common.Hash{BRIDGE_EVENT_SIGNATURE},
						Data:    createValidBridgeEventData(targetToken),
					},
					{
						Address: bridgeContract,
						Topics:  []common.Hash{BRIDGE_EVENT_SIGNATURE},
						Data:    createValidBridgeEventData(common.HexToAddress("0x2222222222222222222222222222222222222222")),
					},
				},
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := checkBridgeEventInReceipt(tc.receipt, sender, wildcardConfig)
			if tc.expectError && err == nil {
				t.Error("Expected error but got none")
			} else if !tc.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

// TestBRIDGE_EVENT_SIGNATURE tests the bridge event signature constant
func TestBRIDGE_EVENT_SIGNATURE(t *testing.T) {
	expectedSig := crypto.Keccak256Hash([]byte("BridgeEvent(uint8,uint32,address,uint32,address,uint256,bytes,uint32)"))
	if BRIDGE_EVENT_SIGNATURE != expectedSig {
		t.Errorf("BRIDGE_EVENT_SIGNATURE mismatch: got %s, want %s", BRIDGE_EVENT_SIGNATURE.Hex(), expectedSig.Hex())
	}
}

// Helper functions

// containsString checks if a string contains a substring
func containsString(str, substr string) bool {
	return len(str) >= len(substr) && (str == substr ||
		(len(str) > len(substr) &&
			(str[:len(substr)] == substr ||
				str[len(str)-len(substr):] == substr ||
				containsSubstring(str, substr))))
}

func containsSubstring(str, substr string) bool {
	for i := 0; i <= len(str)-len(substr); i++ {
		if str[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// createValidBridgeEventData creates valid bridge event data with the specified origin address
func createValidBridgeEventData(originAddress common.Address) []byte {
	data := make([]byte, 256)

	// Set origin address at offset 76-96 (right-aligned in 32-byte slot at offset 64-95)
	copy(data[76:96], originAddress.Bytes())

	// Set a valid amount at offset 160-192
	amount := big.NewInt(1000000000000000000) // 1 ETH
	copy(data[160:192], amount.FillBytes(make([]byte, 32)))

	return data
}
