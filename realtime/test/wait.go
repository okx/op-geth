package test

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/realtime/rtclient"
)

// WaitTxToBeMined waits until a tx has been mined or the given timeout expires.
func WaitTxToBeMined(parentCtx context.Context, client *rtclient.RealtimeClient, tx *types.Transaction, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(parentCtx, timeout)
	defer cancel()
	receipt, err := WaitMined(ctx, client, tx.Hash())
	if errors.Is(err, context.DeadlineExceeded) {
		return err
	} else if err != nil {
		fmt.Printf("error waiting tx %s to be mined: %v\n", tx.Hash(), err)
		return err
	}
	if receipt.Status == types.ReceiptStatusFailed {
		// Get revert reason
		reason, reasonErr := RevertReasonRealtime(ctx, client, tx)
		if reasonErr != nil {
			reason = reasonErr.Error()
		}
		return fmt.Errorf("transaction has failed, reason: %s, receipt: %+v. tx: %+v, gas: %v", reason, receipt, tx, tx.Gas())
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		currNum, err := client.BlockNumber(ctx)
		if err != nil {
			return err
		}

		if currNum >= receipt.BlockNumber.Uint64() {
			break
		}
	}

	fmt.Printf("Transaction successfully mined: %v\n", tx.Hash())
	return nil
}

// WaitEthTxToBeMined waits until a tx has been mined or the given timeout expires.
func WaitEthTxToBeMined(parentCtx context.Context, client ethClienter, tx *types.Transaction, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(parentCtx, timeout)
	defer cancel()
	receipt, err := WaitMined(ctx, client, tx.Hash())
	if errors.Is(err, context.DeadlineExceeded) {
		return err
	} else if err != nil {
		fmt.Printf("error waiting tx %s to be mined: %v\n", tx.Hash(), err)
		return err
	}
	if receipt.Status == types.ReceiptStatusFailed {
		// Get revert reason
		reason, reasonErr := RevertReason(ctx, client, tx, receipt.BlockNumber)
		if reasonErr != nil {
			reason = reasonErr.Error()
		}
		return fmt.Errorf("transaction has failed, reason: %s, receipt: %+v. tx: %+v, gas: %v", reason, receipt, tx, tx.Gas())
	}

	fmt.Printf("Eth transaction successfully mined: %v\n", tx.Hash())
	return err
}

// WaitRealtimeTxToBeConfirmed waits until a tx has been confirmed or the given timeout expires.
func WaitRealtimeTxToBeConfirmed(parentCtx context.Context, client *rtclient.RealtimeClient, tx *types.Transaction, timeout time.Duration, toAddress common.Address) error {
	ctx, cancel := context.WithTimeout(parentCtx, timeout)
	defer cancel()
	receipt, err := WaitMined(ctx, client, tx.Hash())
	if errors.Is(err, context.DeadlineExceeded) {
		return err
	} else if err != nil {
		fmt.Printf("error waiting tx %s to be confirmed: %v\n", tx.Hash(), err)
		return err
	}
	if receipt.Status == types.ReceiptStatusFailed {
		// Get revert reason
		reason, reasonErr := RevertReasonRealtime(ctx, client, tx)
		if reasonErr != nil {
			reason = reasonErr.Error()
		}
		return fmt.Errorf("transaction has failed, reason: %s, receipt: %+v. tx: %+v, gas: %v", reason, receipt, tx, tx.Gas())
	}
	fmt.Printf("Realtime transaction successfully confirmed: %v\n", tx.Hash())
	return nil
}

// WaitEthTxToBeConfirmed waits until a tx has been confirmed or the given timeout expires.
func WaitEthTxToBeConfirmed(parentCtx context.Context, client ethClienter, tx *types.Transaction, timeout time.Duration, toAddress common.Address) error {
	ctx, cancel := context.WithTimeout(parentCtx, timeout)
	defer cancel()
	receipt, err := WaitMined(ctx, client, tx.Hash())
	if errors.Is(err, context.DeadlineExceeded) {
		return err
	} else if err != nil {
		fmt.Printf("error waiting tx %s to be mined: %v\n", tx.Hash(), err)
		return err
	}
	if receipt.Status == types.ReceiptStatusFailed {
		// Get revert reason
		reason, reasonErr := RevertReason(ctx, client, tx, receipt.BlockNumber)
		if reasonErr != nil {
			reason = reasonErr.Error()
		}
		return fmt.Errorf("transaction has failed, reason: %s, receipt: %+v. tx: %+v, gas: %v", reason, receipt, tx, tx.Gas())
	}
	fmt.Printf("Eth transaction successfully confirmed: %v\n", tx.Hash())
	return nil
}

// WaitRealtimeErc20TxToBeConfirmed waits until an erc20 tx has been confirmed or the given timeout expires.
func WaitRealtimeErc20TxToBeConfirmed(parentCtx context.Context, client *rtclient.RealtimeClient, tx *types.Transaction, timeout time.Duration, fromAddress, toAddress common.Address) error {
	ctx, cancel := context.WithTimeout(parentCtx, timeout)
	defer cancel()
	receipt, err := WaitMined(ctx, client, tx.Hash())
	if errors.Is(err, context.DeadlineExceeded) {
		return err
	} else if err != nil {
		fmt.Printf("error waiting tx %s to be confirmed: %v\n", tx.Hash(), err)
		return err
	}
	if receipt.Status == types.ReceiptStatusFailed {
		// Get revert reason
		reason, reasonErr := RevertReasonRealtime(ctx, client, tx)
		if reasonErr != nil {
			reason = reasonErr.Error()
		}
		return fmt.Errorf("transaction has failed, reason: %s, receipt: %+v. tx: %+v, gas: %v", reason, receipt, tx, tx.Gas())
	}
	fmt.Printf("Realtime transaction successfully confirmed: %v\n", tx.Hash())
	return nil
}

// WaitEthErc20TxToBeConfirmed waits until an erc20 tx has been confirmed or the given timeout expires.
func WaitEthErc20TxToBeConfirmed(parentCtx context.Context, client ethClienter, tx *types.Transaction, timeout time.Duration, toAddress common.Address) error {
	ctx, cancel := context.WithTimeout(parentCtx, timeout)
	defer cancel()
	receipt, err := WaitMined(ctx, client, tx.Hash())
	if errors.Is(err, context.DeadlineExceeded) {
		return err
	} else if err != nil {
		fmt.Printf("error waiting tx %s to be mined: %v\n", tx.Hash(), err)
		return err
	}
	if receipt.Status == types.ReceiptStatusFailed {
		// Get revert reason
		reason, reasonErr := RevertReason(ctx, client, tx, receipt.BlockNumber)
		if reasonErr != nil {
			reason = reasonErr.Error()
		}
		return fmt.Errorf("transaction has failed, reason: %s, receipt: %+v. tx: %+v, gas: %v", reason, receipt, tx, tx.Gas())
	}
	fmt.Printf("Eth transaction successfully confirmed: %v\n", tx.Hash())
	return nil
}
