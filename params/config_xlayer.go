// X Layer hardcoded fork configurations

package params

import (
	"github.com/ethereum/go-ethereum/log"
)

// X Layer Chain IDs
const (
	XLayerMainnetChainID = 196  // X Layer mainnet
	XLayerTestnetChainID = 1952 // X Layer testnet (Sepolia)
)

// XLayerForkConfig defines fork time overrides for specific X Layer chains.
// Only fork times that need to be hardcoded are defined here.
// Other configuration fields are read from the database.
type XLayerForkConfig struct {
	ChainID     uint64
	NetworkName string
	// Only define fork times that need to be hardcoded
	JovianTime *uint64
	// Future forks can be added here (e.g., InteropTime when needed)
}

// XLayerHardcodedForks stores hardcoded fork configurations for all X Layer chains.
var XLayerHardcodedForks = map[uint64]*XLayerForkConfig{
	XLayerMainnetChainID: {
		ChainID:     XLayerMainnetChainID,
		NetworkName: "xlayer-mainnet",
		JovianTime:  newUint64(1764691201), // 2025-12-02 16:00:01 UTC
	},
	XLayerTestnetChainID: {
		ChainID:     XLayerTestnetChainID,
		NetworkName: "xlayer-testnet",
		JovianTime:  newUint64(1764327600), // 2025-11-28 11:00:00 UTC
	},
}

// ApplyXLayerHardcodedForks applies X Layer hardcoded fork configuration based on ChainID.
// This function only overrides specific fork times, keeping other configuration from the database.
// This function is primarily used during genesis setup to apply hardcoded values before writing
// to the database.
func ApplyXLayerHardcodedForks(cfg *ChainConfig) *ChainConfig {
	if cfg == nil || cfg.ChainID == nil {
		return cfg
	}

	chainID := cfg.ChainID.Uint64()
	xlayerForks, exists := XLayerHardcodedForks[chainID]

	if !exists {
		return cfg
	}

	log.Info("X Layer: Applying hardcoded fork configuration",
		"chainID", chainID,
		"network", xlayerForks.NetworkName)

	// Apply JovianTime
	if xlayerForks.JovianTime != nil {
		// If database already has a value and it differs, log a warning but still override
		if cfg.JovianTime != nil && *cfg.JovianTime != *xlayerForks.JovianTime {
			log.Warn("X Layer: Overriding database JovianTime with hardcoded value",
				"chainID", chainID,
				"database", *cfg.JovianTime,
				"hardcoded", *xlayerForks.JovianTime)
		}
		cfg.JovianTime = xlayerForks.JovianTime
		log.Info("X Layer: Applied JovianTime", "chainID", chainID, "time", *xlayerForks.JovianTime)
	} else {
		// Hardcoded as nil, ensure it's not activated
		if cfg.JovianTime != nil {
			log.Info("X Layer: Disabling JovianTime (hardcoded as nil)",
				"chainID", chainID,
				"previousValue", *cfg.JovianTime)
			cfg.JovianTime = nil
		}
	}

	return cfg
}
