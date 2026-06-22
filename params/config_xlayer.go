// X Layer hardcoded fork configurations

package params

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
)

// X Layer Chain IDs
const (
	XLayerMainnetChainID = 196  // X Layer mainnet
	XLayerTestnetChainID = 1952 // X Layer testnet (Sepolia)
	XLayerDevnetChainID  = 195  // X Layer devnet (blacklist dispatch)
)

// --- XLayer emergency-freeze blacklist ---
//
// Chain-level dispatch and on-chain data-source constants for the blacklist
// interception feature. No CLI flag and no runtime config exist by design:
// changing the enabled set or the mirror addresses requires a code upgrade +
// full-network release.

// XLayerBlacklistMirrorAddress maps an enabled chain_id to the hardcoded
// address of its L2BlacklistMirror contract. A chain_id is "blacklist enabled"
// iff (and only iff) it is a key of this map.
//
// chain 195 (devnet) holds a deterministic mirror address: deployed from the
// test-mnemonic account index 19 at its L2 nonce 0, so the address is stable
// across every devnet rebuild and must stay byte-identical across all clients.
//
// NOTE: the testnet (1952) and mainnet (196) mirror addresses are still pending
// from the contracts repo; the values below are deterministic placeholders so
// the dispatch logic is testable and MUST be replaced with the real deployed
// addresses before the feature is enabled on those networks. Tests assert via
// the map keys (IsBlacklistEnabled / BlacklistMirror), never literal addresses.
var XLayerBlacklistMirrorAddress = map[uint64]common.Address{
	XLayerDevnetChainID:  common.HexToAddress("0x73511669fd4dE447feD18BB79bAFeAC93aB7F31f"),       // devnet: deterministic deploy address
	XLayerTestnetChainID: common.HexToAddress("0x000000000000000000000000626C61636B6C6973741952"), // TODO: real testnet address (pending contracts deployment)
	XLayerMainnetChainID: common.HexToAddress("0x000000000000000000000000626C61636B6C6973740196"), // TODO: real mainnet address (pending contracts deployment)
}

// IsBlacklistEnabled reports whether chain-level blacklist interception is
// active for the given chain_id. Unrecognized chains return false → the
// execution gate is a no-op.
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
// (L1Block.setL1BlockValues) deposit on OP Stack. Exported via the exempt-sender
// set below so the execution gate shares a single source of truth (avoids the
// literal being scattered across packages).
var l1AttributesDepositor = common.HexToAddress("0xDeaDDEaDDeAdDeAdDEAdDEaddeAddEAdDEAd0001")

// XLayerDepositExemptSenders is the precise, consensus-critical set of deposit
// `from` addresses that are NEVER intercepted by the execution gate, even when
// they touch/transfer a blacklisted address. The set is exhaustive — no "etc.":
//   - params.SystemAddress (0xff…fe): system-call originator (EIP-4788 beacon
//     root, EIP-2935 history storage, withdrawal/consolidation queues).
//   - l1AttributesDepositor (0xDeaD…0001): the fixed from of every block's
//     first L1-info deposit.
var XLayerDepositExemptSenders = map[common.Address]struct{}{
	SystemAddress:         {},
	l1AttributesDepositor: {},
}

// IsDepositExemptSender reports whether a deposit originating from `from` is
// exempt from blacklist interception.
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
