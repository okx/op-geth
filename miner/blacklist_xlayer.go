// XLayer emergency-freeze blacklist — build-path wiring (XLOP-1099, FR-2/FR-3).
//
// Fork-local XLayer extension (KG naming rule): kept out of worker.go so the
// upstream file's footprint stays minimal. attachBlacklistGate builds the
// build-mode execution gate and rebuilds env.evm against a hooked state so the
// observational tracer (check③ OnBalanceChange) fires; the gate is then passed
// into core.ApplyTransaction by miner.applyTransaction.

package miner

import (
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/vm"
)

// attachBlacklistGate sets up the XLayer blacklist execution gate on the build
// path. It reads the block-head/parent snapshot once and, when active, rebuilds
// env.evm so the EVM runs against a hooked state that fires the observational
// tracer (multiplexed onto evm.Config.Tracer). Build mode → dropNormalHit=true:
// a committed normal-tx hit is dropped from the block (the import path uses
// dropNormalHit=false). A nil gate (disabled chain / empty list) leaves env.evm
// untouched (zero hot-path cost).
func (miner *Miner) attachBlacklistGate(env *environment) {
	chainID := miner.chainConfig.ChainID
	if chainID == nil {
		return
	}
	gate := core.NewBlacklistGate(env.state, env.header, miner.chainConfig, chainID.Uint64(), true)
	if gate == nil {
		return
	}
	env.blGate = gate
	// Multiplex via CombineBlacklistHooks (rather than replacing) for symmetry
	// with the import path (TD R-8). makeEnv builds env.evm with an empty
	// vm.Config (no base tracer), so this preserves any future base tracer too.
	hooks := core.CombineBlacklistHooks(env.evm.Config.Tracer, gate.Hooks())
	hooked := state.NewHookedState(env.state, hooks)
	env.evm = vm.NewEVM(
		core.NewEVMBlockContext(env.header, miner.chain, &env.coinbase, miner.chainConfig, env.state),
		hooked, miner.chainConfig, vm.Config{Tracer: hooks},
	)
}
