// X Layer hardcoded fork configurations

package params

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
)

// X Layer Chain IDs
const (
	XLayerMainnetChainID        = 196  // X Layer mainnet
	XLayerTestnetChainID        = 1952 // X Layer testnet (Sepolia)
	XLayerSepoliaTestnetChainID = 195  // X Layer Sepolia testnet (added for blacklist dispatch, XLOP-1099)
)

// --- XLayer emergency-freeze blacklist (XLOP-1099) ---
//
// Chain-level dispatch and on-chain data-source constants for the blacklist
// interception feature. No CLI flag and no runtime config exist by design
// (PRD FR-6 / FR-7 negative constraint): changing the enabled set or the
// mirror addresses requires a code upgrade + full-network release.

// XLayerBlacklistMirrorAddress maps an enabled chain_id to the hardcoded
// address of its L2BlacklistMirror contract. A chain_id is "blacklist enabled"
// iff (and only iff) it is a key of this map.
//
// NOTE (blocking open item B-1, TD §11.4): the three production mirror
// addresses are pending from the contracts repo. The values below are
// deterministic placeholders so the dispatch logic (IsBlacklistEnabled /
// BlacklistMirror) is testable; they MUST be replaced with the real deployed
// addresses before the feature is enabled network-wide. Tests assert via the
// map keys (IsBlacklistEnabled / BlacklistMirror), never literal addresses.
var XLayerBlacklistMirrorAddress = map[uint64]common.Address{
	XLayerSepoliaTestnetChainID: common.HexToAddress("0x000000000000000000000000626C61636B6C6973740195"), // TODO(B-1): real 195 address
	XLayerTestnetChainID:        common.HexToAddress("0x000000000000000000000000626C61636B6C6973741952"), // TODO(B-1): real 1952 address
	XLayerMainnetChainID:        common.HexToAddress("0x000000000000000000000000626C61636B6C6973740196"), // TODO(B-1): real 196 address
}

// IsBlacklistEnabled reports whether chain-level blacklist interception is
// active for the given chain_id. Unrecognized chains return false → the ingress
// filter passes through and the execution gate is a no-op (FR-6 AC2).
func IsBlacklistEnabled(chainID uint64) bool {
	_, ok := XLayerBlacklistMirrorAddress[chainID]
	return ok
}

// BlacklistMirror returns the L2BlacklistMirror contract address for the given
// chain_id and whether the chain is blacklist enabled.
func BlacklistMirror(chainID uint64) (common.Address, bool) {
	a, ok := XLayerBlacklistMirrorAddress[chainID]
	return a, ok
}

// l1AttributesDepositor is the fixed `from` of the per-block L1-attributes
// (L1Block.setL1BlockValues) deposit on OP Stack. Same value referenced as
// `systemAddress` at eth/downloader/receiptreference.go:28. Exported via the
// exempt-sender set below so the execution gate shares a single source of truth
// (avoids the literal being scattered across packages).
var l1AttributesDepositor = common.HexToAddress("0xDeaDDEaDDeAdDeAdDEAdDEaddeAddEAdDEAd0001")

// XLayerDepositExemptSenders is the precise, consensus-critical set of deposit
// `from` addresses that are NEVER intercepted by the execution gate, even when
// they touch/transfer a blacklisted address (FR-3 AC2, TD §4.1.1). The set is
// exhaustive — no "etc.":
//   - params.SystemAddress (0xff…fe): system-call originator (EIP-4788 beacon
//     root, EIP-2935 history storage, withdrawal/consolidation queues).
//   - l1AttributesDepositor (0xDeaD…0001): the fixed from of every block's
//     first L1-info deposit.
var XLayerDepositExemptSenders = map[common.Address]struct{}{
	SystemAddress:         {},
	l1AttributesDepositor: {},
}

// IsDepositExemptSender reports whether a deposit originating from `from` is
// exempt from blacklist interception (FR-3 AC2).
func IsDepositExemptSender(from common.Address) bool {
	_, ok := XLayerDepositExemptSenders[from]
	return ok
}

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
