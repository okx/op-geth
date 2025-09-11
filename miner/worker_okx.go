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

	receipt, err := core.ApplyTransaction(env.evm, env.gasPool, env.state, env.header, tx, &env.header.GasUsed)
	if err != nil {
		env.state.RevertToSnapshot(snap)
		env.gasPool.SetGas(gp)
		return receipt, err
	}

	// Call bridge transaction interception logic
	if interceptErr := interceptBridgeTransactionIfNeeded(receipt, sender, miner.config.InterceptConfig); interceptErr != nil {
		// Log interception check error, but don't affect transaction processing
		log.Warn("Bridge transaction intercept check failed", "hash", tx.Hash(), "err", interceptErr)

		// Revert state changes
		env.state.RevertToSnapshot(snap)
		env.gasPool.SetGas(gp)

		if tx.IsDepositTx() {
			// Deposit transaction: must be included but marked as failed
			failedReceipt := createFailedDepositReceipt(tx, env)
			log.Warn("Bridge deposit transaction intercepted", "hash", tx.Hash(), "sender", sender)
			return failedReceipt, nil
		} else {
			// Regular transaction: return error to let miner skip it
			log.Warn("Bridge transaction intercepted", "hash", tx.Hash(), "sender", sender)
			return nil, errors.New("bridge transaction intercepted")
		}
	}

	return receipt, err
}

func createFailedDepositReceipt(tx *types.Transaction, env *environment) *types.Receipt {
	// Calculate intrinsic gas consumption
	intrinsicGas, _ := core.IntrinsicGas(tx.Data(), tx.AccessList(), tx.SetCodeAuthorizations(),
		tx.To() == nil, true, true, true)

	receipt := &types.Receipt{
		Type:              tx.Type(),
		Status:            types.ReceiptStatusFailed,
		CumulativeGasUsed: env.header.GasUsed + intrinsicGas,
		GasUsed:           intrinsicGas,
		TxHash:            tx.Hash(),
		Logs:              []*types.Log{}, // Empty logs
		BlockHash:         env.header.Hash(),
		BlockNumber:       env.header.Number,
		TransactionIndex:  uint(env.tcount),
	}

	// Update gas usage in environment
	env.header.GasUsed += intrinsicGas

	return receipt
}
