// X Layer hardcoded fork configurations test

package params

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestApplyXLayerHardcodedForks tests the X Layer hardcoded fork configuration application
func TestApplyXLayerHardcodedForks(t *testing.T) {
	tests := []struct {
		name           string
		inputConfig    *ChainConfig
		expectedConfig *ChainConfig
		checkLog       bool // whether to check if specific fields were applied
	}{
		{
			name:           "nil config should return nil",
			inputConfig:    nil,
			expectedConfig: nil,
		},
		{
			name: "config with nil ChainID should return unchanged",
			inputConfig: &ChainConfig{
				ChainID:    nil,
				JovianTime: newUint64(999),
			},
			expectedConfig: &ChainConfig{
				ChainID:    nil,
				JovianTime: newUint64(999),
			},
		},
		{
			name: "non-XLayer chain should return unchanged",
			inputConfig: &ChainConfig{
				ChainID:    big.NewInt(1), // Ethereum mainnet
				JovianTime: newUint64(999),
			},
			expectedConfig: &ChainConfig{
				ChainID:    big.NewInt(1),
				JovianTime: newUint64(999),
			},
		},
		{
			name: "XLayer mainnet should apply hardcoded forks (JovianTime = specific value)",
			inputConfig: &ChainConfig{
				ChainID:           big.NewInt(XLayerMainnetChainID),
				BedrockBlock:      big.NewInt(0),
				RegolithTime:      newUint64(0),
				CanyonTime:        newUint64(0),
				EcotoneTime:       newUint64(0),
				FjordTime:         newUint64(0),
				GraniteTime:       newUint64(0),
				HoloceneTime:      newUint64(0),
				IsthmusTime:       newUint64(0),
				JovianTime:        newUint64(123456789), // Should be overridden to hardcoded value
				LegacyXLayerBlock: big.NewInt(12241700),
			},
			expectedConfig: &ChainConfig{
				ChainID:           big.NewInt(XLayerMainnetChainID),
				BedrockBlock:      big.NewInt(0),
				RegolithTime:      newUint64(0),
				CanyonTime:        newUint64(0),
				EcotoneTime:       newUint64(0),
				FjordTime:         newUint64(0),
				GraniteTime:       newUint64(0),
				HoloceneTime:      newUint64(0),
				IsthmusTime:       newUint64(0),
				JovianTime:        newUint64(1764691201), // Hardcoded value
				LegacyXLayerBlock: big.NewInt(12241700),
			},
		},
		{
			name: "XLayer testnet should apply hardcoded forks (JovianTime = specific value)",
			inputConfig: &ChainConfig{
				ChainID:           big.NewInt(XLayerTestnetChainID),
				BedrockBlock:      big.NewInt(0),
				RegolithTime:      newUint64(0),
				CanyonTime:        newUint64(0),
				EcotoneTime:       newUint64(0),
				FjordTime:         newUint64(0),
				GraniteTime:       newUint64(0),
				HoloceneTime:      newUint64(0),
				IsthmusTime:       newUint64(0),
				JovianTime:        nil, // Should be set to hardcoded value
				LegacyXLayerBlock: big.NewInt(12241700),
			},
			expectedConfig: &ChainConfig{
				ChainID:           big.NewInt(XLayerTestnetChainID),
				BedrockBlock:      big.NewInt(0),
				RegolithTime:      newUint64(0),
				CanyonTime:        newUint64(0),
				EcotoneTime:       newUint64(0),
				FjordTime:         newUint64(0),
				GraniteTime:       newUint64(0),
				HoloceneTime:      newUint64(0),
				IsthmusTime:       newUint64(0),
				JovianTime:        newUint64(1764327600), // Hardcoded value
				LegacyXLayerBlock: big.NewInt(12241700),
			},
		},
		{
			name: "XLayer testnet with different JovianTime should override",
			inputConfig: &ChainConfig{
				ChainID:    big.NewInt(XLayerTestnetChainID),
				JovianTime: newUint64(999999999), // Wrong value, should be overridden
			},
			expectedConfig: &ChainConfig{
				ChainID:    big.NewInt(XLayerTestnetChainID),
				JovianTime: newUint64(1764327600), // Correct hardcoded value
			},
		},
		{
			name: "XLayer mainnet with different JovianTime should override",
			inputConfig: &ChainConfig{
				ChainID:    big.NewInt(XLayerMainnetChainID),
				JovianTime: newUint64(999999999), // Should be overridden to hardcoded value
			},
			expectedConfig: &ChainConfig{
				ChainID:    big.NewInt(XLayerMainnetChainID),
				JovianTime: newUint64(1764691201), // Hardcoded value
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Deep copy input to avoid mutation affecting test
			var input *ChainConfig
			if tt.inputConfig != nil {
				inputCopy := *tt.inputConfig
				input = &inputCopy
			}

			result := ApplyXLayerHardcodedForks(input)

			// For nil cases
			if tt.expectedConfig == nil {
				require.Nil(t, result, "Expected nil result")
				return
			}

			require.NotNil(t, result, "Expected non-nil result")

			// Check ChainID
			if tt.expectedConfig.ChainID == nil {
				require.Nil(t, result.ChainID, "Expected nil ChainID")
			} else {
				require.NotNil(t, result.ChainID, "Expected non-nil ChainID")
				require.Equal(t, tt.expectedConfig.ChainID.Uint64(), result.ChainID.Uint64(), "ChainID mismatch")
			}

			// Check JovianTime
			if tt.expectedConfig.JovianTime == nil {
				require.Nil(t, result.JovianTime, "Expected nil JovianTime")
			} else {
				require.NotNil(t, result.JovianTime, "Expected non-nil JovianTime")
				require.Equal(t, *tt.expectedConfig.JovianTime, *result.JovianTime, "JovianTime mismatch")
			}

			// Check other fields are preserved (not modified)
			if tt.expectedConfig.BedrockBlock != nil {
				require.NotNil(t, result.BedrockBlock, "BedrockBlock should be preserved")
				require.Equal(t, tt.expectedConfig.BedrockBlock.Uint64(), result.BedrockBlock.Uint64(), "BedrockBlock should not be modified")
			}
			if tt.expectedConfig.RegolithTime != nil {
				require.NotNil(t, result.RegolithTime, "RegolithTime should be preserved")
				require.Equal(t, *tt.expectedConfig.RegolithTime, *result.RegolithTime, "RegolithTime should not be modified")
			}
			if tt.expectedConfig.LegacyXLayerBlock != nil {
				require.NotNil(t, result.LegacyXLayerBlock, "LegacyXLayerBlock should be preserved")
				require.Equal(t, tt.expectedConfig.LegacyXLayerBlock.Uint64(), result.LegacyXLayerBlock.Uint64(), "LegacyXLayerBlock should not be modified")
			}
		})
	}
}

// TestXLayerHardcodedForksConfiguration verifies the hardcoded fork configurations
func TestXLayerHardcodedForksConfiguration(t *testing.T) {
	// Verify XLayer mainnet configuration
	mainnetForks, exists := XLayerHardcodedForks[XLayerMainnetChainID]
	require.True(t, exists, "XLayer mainnet fork configuration should exist")
	require.Equal(t, uint64(196), mainnetForks.ChainID, "XLayer mainnet ChainID should be 196")
	require.Equal(t, "xlayer-mainnet", mainnetForks.NetworkName, "XLayer mainnet network name should be correct")
	require.NotNil(t, mainnetForks.JovianTime, "XLayer mainnet JovianTime should be set")
	require.Equal(t, uint64(1764691201), *mainnetForks.JovianTime, "XLayer mainnet JovianTime should be same as OP/Base")

	// Verify XLayer testnet configuration
	testnetForks, exists := XLayerHardcodedForks[XLayerTestnetChainID]
	require.True(t, exists, "XLayer testnet fork configuration should exist")
	require.Equal(t, uint64(1952), testnetForks.ChainID, "XLayer testnet ChainID should be 1952")
	require.Equal(t, "xlayer-testnet", testnetForks.NetworkName, "XLayer testnet network name should be correct")
	require.NotNil(t, testnetForks.JovianTime, "XLayer testnet JovianTime should be set")
	require.Equal(t, uint64(1764327600), *testnetForks.JovianTime, "XLayer testnet JovianTime should be 2025-11-28 11:00:00 UTC")
}

// TestApplyXLayerHardcodedForksIdempotency tests that applying forks multiple times is safe
func TestApplyXLayerHardcodedForksIdempotency(t *testing.T) {
	config := &ChainConfig{
		ChainID:    big.NewInt(XLayerTestnetChainID),
		JovianTime: nil,
	}

	// Apply once
	result1 := ApplyXLayerHardcodedForks(config)
	require.NotNil(t, result1.JovianTime)
	require.Equal(t, uint64(1764327600), *result1.JovianTime)

	// Apply again to the result
	result2 := ApplyXLayerHardcodedForks(result1)
	require.NotNil(t, result2.JovianTime)
	require.Equal(t, uint64(1764327600), *result2.JovianTime)

	// Results should be the same
	require.Equal(t, result1.JovianTime, result2.JovianTime, "Applying forks multiple times should be idempotent")
}
