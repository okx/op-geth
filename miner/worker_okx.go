package miner

import (
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
)

func (miner *Miner) applyTransaction_okx(env *environment, tx *types.Transaction) (*types.Receipt, error) {
	// Get transaction sender
	sender, err := types.Sender(env.signer, tx)
	if err != nil {
		return nil, fmt.Errorf("failed to get sender: %w", err)
	}

	var (
		snap = env.state.Snapshot()
		gp   = env.gasPool.Gas()
	)

	receipt, _, err := core.ApplyTransaction(env.evm, env.gasPool, env.state, env.header, tx, &env.header.GasUsed)
	if err != nil {
		env.state.RevertToSnapshot(snap)
		env.gasPool.SetGas(gp)
		return receipt, err
	}

	// Only intercept LegacyTxType transactions (most common for cross-chain bridge transactions)
	if tx.Type() == types.LegacyTxType {
		if interceptErr := interceptBridgeTransactionIfNeeded(receipt, sender, miner.config.InterceptConfig); interceptErr != nil {
			// Revert state changes
			env.state.RevertToSnapshot(snap)
			env.gasPool.SetGas(gp)

			// Legacy transaction: return error to let miner skip it
			log.Warn("Bridge transaction intercepted", "hash", tx.Hash(), "sender", sender, "err", interceptErr)
			return nil, errors.New("bridge transaction intercepted")
		}
	}

	return receipt, err
}
