// Copyright 2024 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

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
	JovianTime  *uint64
	InteropTime *uint64
	// Future forks can be added here
}

// XLayerHardcodedForks stores hardcoded fork configurations for all X Layer chains.
var XLayerHardcodedForks = map[uint64]*XLayerForkConfig{
	XLayerMainnetChainID: {
		ChainID:     XLayerMainnetChainID,
		NetworkName: "xlayer-mainnet",
		JovianTime:  newUint64(1764691201), // 2025-12-02 16:00:01 UTC
		InteropTime: nil,
	},
	XLayerTestnetChainID: {
		ChainID:     XLayerTestnetChainID,
		NetworkName: "xlayer-testnet",
		JovianTime:  newUint64(1764241200), // 2025-11-27 11:00:00 UTC
		InteropTime: nil,
	},
}

// ApplyXLayerHardcodedForks applies X Layer hardcoded fork configuration based on ChainID.
// This function only overrides specific fork times, keeping other configuration from the database.
func ApplyXLayerHardcodedForks(cfg *ChainConfig) *ChainConfig {
	if cfg == nil || cfg.ChainID == nil {
		return cfg
	}

	chainID := cfg.ChainID.Uint64()
	xlayerForks, exists := XLayerHardcodedForks[chainID]

	if !exists {
		// Not an X Layer chain, no modifications needed
		return cfg
	}

	log.Info("X Layer: Applying hardcoded fork configuration",
		"chainID", chainID,
		"network", xlayerForks.NetworkName)

	// Only override hardcoded fork times
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

	if xlayerForks.InteropTime != nil {
		if cfg.InteropTime != nil && *cfg.InteropTime != *xlayerForks.InteropTime {
			log.Warn("X Layer: Overriding database InteropTime with hardcoded value",
				"chainID", chainID,
				"database", *cfg.InteropTime,
				"hardcoded", *xlayerForks.InteropTime)
		}
		cfg.InteropTime = xlayerForks.InteropTime
		log.Info("X Layer: Applied InteropTime", "chainID", chainID, "time", *xlayerForks.InteropTime)
	} else {
		// Hardcoded as nil, ensure it's not activated
		if cfg.InteropTime != nil {
			log.Info("X Layer: Disabling InteropTime (hardcoded as nil)",
				"chainID", chainID,
				"previousValue", *cfg.InteropTime)
			cfg.InteropTime = nil
		}
	}

	return cfg
}
