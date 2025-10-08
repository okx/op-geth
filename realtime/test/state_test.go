//go:build !skip_smoke_realtime
// +build !skip_smoke_realtime

package test

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/realtime/rtclient"
	"github.com/stretchr/testify/require"
)

func TestStateUntilCommitToGlobal(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	ctx := context.Background()
	ec, err := ethclient.Dial(DefaultL2NetworkRealtimeURL)
	require.NoError(t, err)
	client, err := rtclient.NewRealtimeClient(ctx, ec, DefaultL2NetworkRealtimeURL)
	require.NoError(t, err)

	privateKey, err := crypto.HexToECDSA(DefaultL2AdminPrivateKey[2:])
	require.NoError(t, err)

	// Default test address for tests that require an address
	fromAddress := common.HexToAddress(DefaultL2AdminAddress)
	testAddress := common.HexToAddress("0x1234567890123456789012345678901234567890")

	// Test native transfer
	txs := make([]*types.Transaction, 10)
	for i := 0; i < 10; i++ {
		signedTx := nativeTransferTx(t, context.Background(), client, big.NewInt(Gwei), testAddress.String())
		txs[i] = signedTx
	}
	for _, tx := range txs {
		WaitTxToBeMined(ctx, client, tx, DefaultTimeoutTxToBeMined)
	}

	// Get balance. Balance should be consistent for 10 block ranges
	// 10 is the default for realtime cache threshold height
	startBalance, err := client.RealtimeGetBalance(ctx, testAddress)
	require.NoError(t, err)
	height, err := client.RealtimeBlockNumber(ctx, "latest")
	require.NoError(t, err)
	for {
		balance, err := client.RealtimeGetBalance(ctx, testAddress)
		require.NoError(t, err)
		require.Equal(t, balance.String(), startBalance.String())
		currHeight, err := client.RealtimeBlockNumber(ctx, "latest")
		require.NoError(t, err)
		if currHeight-height > 10 {
			break
		}
		time.Sleep(400 * time.Millisecond)
	}

	// Test ERC20 transfer
	erc20Address := deployERC20Contract(t, ctx, privateKey, client)
	transferAmount := new(big.Int).Mul(big.NewInt(1), big.NewInt(1e18))

	startNonce, err := client.PendingNonceAt(context.Background(), fromAddress)
	require.NoError(t, err)
	txs = make([]*types.Transaction, 10)
	for i := 0; i < 10; i++ {
		signedTx := erc20TransferTx(t, ctx, privateKey, client, transferAmount, nil, testAddress, erc20Address, startNonce+uint64(i))
		txs[i] = signedTx
	}
	for _, tx := range txs {
		WaitTxToBeMined(ctx, client, tx, DefaultTimeoutTxToBeMined)
	}

	// Get balance. Balance should be consistent for 10 block ranges
	// 10 is the default for realtime cache threshold height
	startBalance, err = client.RealtimeGetTokenBalance(ctx, fromAddress, testAddress, erc20Address)
	require.NoError(t, err)
	height, err = client.RealtimeBlockNumber(ctx, "latest")
	require.NoError(t, err)
	for {
		balance, err := client.RealtimeGetTokenBalance(ctx, fromAddress, testAddress, erc20Address)
		require.NoError(t, err)
		require.Equal(t, balance.String(), startBalance.String())
		currHeight, err := client.RealtimeBlockNumber(ctx, "latest")
		require.NoError(t, err)
		if currHeight-height > 10 {
			break
		}
		time.Sleep(400 * time.Millisecond)
	}
}
