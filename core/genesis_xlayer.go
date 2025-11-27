// X Layer specific genesis and configuration logic

package core

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
)

// EnsureXLayerHardcodedForksInDB ensures that X Layer fork times are correctly stored in the database.
// This function is called during genesis setup to guarantee that the database contains the correct
// hardcoded fork activation times for X Layer chains (mainnet and testnet).
//
// On first startup or when fork times are outdated:
//   - Detects X Layer chains by ChainID (196 for mainnet, 1952 for testnet)
//   - Compares database values with hardcoded values
//   - Writes updated configuration to database if needed
//
// On subsequent startups:
//   - Quickly verifies configuration is up-to-date
//   - Returns without database writes if no changes needed
//
// This ensures the database is the single source of truth for chain configuration,
// eliminating the need for runtime overrides on every config read.
func EnsureXLayerHardcodedForksInDB(db ethdb.Database, ghash common.Hash) error {
	// Read current configuration from database
	storedCfg := rawdb.ReadChainConfig(db, ghash)
	if storedCfg == nil || storedCfg.ChainID == nil {
		// Database empty or invalid, nothing to do
		return nil
	}

	chainID := storedCfg.ChainID.Uint64()
	xlayerForks, exists := params.XLayerHardcodedForks[chainID]
	if !exists {
		// Not an X Layer chain, nothing to do
		return nil
	}

	// Check if database configuration needs updating
	needsUpdate := false

	// Check JovianTime
	if xlayerForks.JovianTime != nil {
		if storedCfg.JovianTime == nil || *storedCfg.JovianTime != *xlayerForks.JovianTime {
			needsUpdate = true
			log.Warn("X Layer: JovianTime in database is outdated or missing",
				"chainID", chainID,
				"network", xlayerForks.NetworkName,
				"database", storedCfg.JovianTime,
				"hardcoded", *xlayerForks.JovianTime)
		}
	} else {
		// Hardcoded as nil, should be disabled
		if storedCfg.JovianTime != nil {
			needsUpdate = true
			log.Warn("X Layer: JovianTime should be disabled (hardcoded as nil)",
				"chainID", chainID,
				"network", xlayerForks.NetworkName,
				"database", *storedCfg.JovianTime)
		}
	}

	// Add checks for future forks here (e.g., InteropTime)

	if !needsUpdate {
		// Configuration is up-to-date, fast path
		log.Info("X Layer: Database fork configuration is up-to-date",
			"chainID", chainID,
			"network", xlayerForks.NetworkName)
		return nil
	}

	// Configuration needs update, apply hardcoded forks and write to database
	log.Info("X Layer: Updating database with hardcoded fork times",
		"chainID", chainID,
		"network", xlayerForks.NetworkName)

	// Apply hardcoded fork configuration
	newCfg := params.ApplyXLayerHardcodedForks(storedCfg)

	// Write updated configuration to database
	rawdb.WriteChainConfig(db, ghash, newCfg)

	log.Info("X Layer: Successfully wrote hardcoded fork times to database",
		"chainID", chainID,
		"network", xlayerForks.NetworkName,
		"jovianTime", newCfg.JovianTime)

	return nil
}
