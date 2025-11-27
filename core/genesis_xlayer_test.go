// X Layer specific genesis and configuration logic tests

package core

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/params"
	"github.com/stretchr/testify/require"
)

// TestEnsureXLayerHardcodedForksInDB tests the database fork time enforcement logic
func TestEnsureXLayerHardcodedForksInDB(t *testing.T) {
	tests := []struct {
		name           string
		chainID        uint64
		initialJovian  *uint64
		expectWrite    bool
		expectJovian   *uint64
		expectLogLevel string // "up-to-date" or "outdated"
	}{
		{
			name:           "X Layer mainnet - missing JovianTime",
			chainID:        params.XLayerMainnetChainID,
			initialJovian:  nil,
			expectWrite:    true,
			expectJovian:   params.XLayerHardcodedForks[params.XLayerMainnetChainID].JovianTime,
			expectLogLevel: "outdated",
		},
		{
			name:           "X Layer mainnet - correct JovianTime",
			chainID:        params.XLayerMainnetChainID,
			initialJovian:  params.XLayerHardcodedForks[params.XLayerMainnetChainID].JovianTime,
			expectWrite:    false,
			expectJovian:   params.XLayerHardcodedForks[params.XLayerMainnetChainID].JovianTime,
			expectLogLevel: "up-to-date",
		},
		{
			name:           "X Layer mainnet - outdated JovianTime",
			chainID:        params.XLayerMainnetChainID,
			initialJovian:  newUint64(1000000),
			expectWrite:    true,
			expectJovian:   params.XLayerHardcodedForks[params.XLayerMainnetChainID].JovianTime,
			expectLogLevel: "outdated",
		},
		{
			name:           "X Layer testnet - missing JovianTime",
			chainID:        params.XLayerTestnetChainID,
			initialJovian:  nil,
			expectWrite:    true,
			expectJovian:   params.XLayerHardcodedForks[params.XLayerTestnetChainID].JovianTime,
			expectLogLevel: "outdated",
		},
		{
			name:           "X Layer testnet - correct JovianTime",
			chainID:        params.XLayerTestnetChainID,
			initialJovian:  params.XLayerHardcodedForks[params.XLayerTestnetChainID].JovianTime,
			expectWrite:    false,
			expectJovian:   params.XLayerHardcodedForks[params.XLayerTestnetChainID].JovianTime,
			expectLogLevel: "up-to-date",
		},
		{
			name:           "Non-X Layer chain - ignored",
			chainID:        1, // Ethereum mainnet
			initialJovian:  nil,
			expectWrite:    false,
			expectJovian:   nil,
			expectLogLevel: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create in-memory database
			db := rawdb.NewMemoryDatabase()

			// Create genesis block with test chain config
			config := &params.ChainConfig{
				ChainID:    big.NewInt(int64(tt.chainID)),
				JovianTime: tt.initialJovian,
			}

			genesis := &Genesis{
				Config: config,
				Alloc:  GenesisAlloc{},
			}

			block := genesis.ToBlock()
			ghash := block.Hash()

			// Write genesis block and initial config to database
			rawdb.WriteBlock(db, block)
			rawdb.WriteCanonicalHash(db, ghash, 0)
			rawdb.WriteChainConfig(db, ghash, config)

			// Call EnsureXLayerHardcodedForksInDB
			err := EnsureXLayerHardcodedForksInDB(db, ghash)
			require.NoError(t, err)

			// Read back the configuration
			storedCfg := rawdb.ReadChainConfig(db, ghash)
			require.NotNil(t, storedCfg)

			// Verify JovianTime
			if tt.expectJovian != nil {
				require.NotNil(t, storedCfg.JovianTime, "JovianTime should not be nil")
				require.Equal(t, *tt.expectJovian, *storedCfg.JovianTime, "JovianTime mismatch")
			} else {
				require.Equal(t, tt.expectJovian, storedCfg.JovianTime, "JovianTime should match expected")
			}
		})
	}
}

// TestEnsureXLayerHardcodedForksInDB_Idempotency tests that calling the function multiple times is safe
func TestEnsureXLayerHardcodedForksInDB_Idempotency(t *testing.T) {
	db := rawdb.NewMemoryDatabase()

	config := &params.ChainConfig{
		ChainID:    big.NewInt(int64(params.XLayerMainnetChainID)),
		JovianTime: nil, // Missing
	}

	genesis := &Genesis{
		Config: config,
		Alloc:  GenesisAlloc{},
	}

	block := genesis.ToBlock()
	ghash := block.Hash()

	rawdb.WriteBlock(db, block)
	rawdb.WriteCanonicalHash(db, ghash, 0)
	rawdb.WriteChainConfig(db, ghash, config)

	// Call multiple times
	for i := 0; i < 3; i++ {
		err := EnsureXLayerHardcodedForksInDB(db, ghash)
		require.NoError(t, err, "call %d should not error", i+1)

		storedCfg := rawdb.ReadChainConfig(db, ghash)
		require.NotNil(t, storedCfg)
		require.NotNil(t, storedCfg.JovianTime)
		require.Equal(t, *params.XLayerHardcodedForks[params.XLayerMainnetChainID].JovianTime,
			*storedCfg.JovianTime, "JovianTime should remain consistent across calls")
	}
}

// TestEnsureXLayerHardcodedForksInDB_EmptyDatabase tests behavior with empty database
func TestEnsureXLayerHardcodedForksInDB_EmptyDatabase(t *testing.T) {
	db := rawdb.NewMemoryDatabase()
	ghash := params.MainnetGenesisHash

	// Call with empty database - should not error
	err := EnsureXLayerHardcodedForksInDB(db, ghash)
	require.NoError(t, err)
}

// Helper function
func newUint64(val uint64) *uint64 {
	return &val
}
