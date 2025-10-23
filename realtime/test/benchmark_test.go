//go:build !skip_smoke_realtime
// +build !skip_smoke_realtime

package test

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/realtime/realtimeapi"
	"github.com/ethereum/go-ethereum/realtime/rtclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

var (
	Iterations = 11
)

func TestRealtimeBenchmarkNativeTransferConfirmation(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	ctx := context.Background()
	ec, err := ethclient.Dial(DefaultL2NetworkRealtimeURL)
	require.NoError(t, err)
	client, err := rtclient.NewRealtimeClient(ctx, ec, DefaultL2NetworkRealtimeURL)
	require.NoError(t, err)
	nonRtClient, err := ethclient.Dial(DefaultL2NetworkNoRealtimeURL)
	require.NoError(t, err)

	// Default test address for tests that require an address
	testAddress := common.HexToAddress("0x1234567890123456789012345678901234567890")

	// Benchmark transfer tx to test address
	time.Sleep(3 * time.Second)
	var totalRealtimeBalanceDuration, totalEthBalanceDuration time.Duration
	for i := 0; i < Iterations; i++ {
		// Ensure heights are consistent
		realtimeHeight, err := client.RealtimeBlockNumber(ctx, "latest")
		require.NoError(t, err)
		nonRtHeight, err := nonRtClient.BlockNumber(ctx)
		require.NoError(t, err)
		commonHeight := nonRtHeight
		if realtimeHeight < nonRtHeight {
			commonHeight = realtimeHeight
		}

		ethBalance, err := nonRtClient.BalanceAt(ctx, testAddress, big.NewInt(int64(commonHeight)))
		require.NoError(t, err)
		realtimeBalance, err := client.BalanceAt(ctx, testAddress, big.NewInt(int64(commonHeight)))
		require.NoError(t, err)
		require.Equal(t, ethBalance.String(), realtimeBalance.String())

		// Send tx
		signedTx := nativeTransferTx(t, context.Background(), client, big.NewInt(Gwei), testAddress.String())
		fmt.Printf("Sent tx: %s\n", signedTx.Hash().String())

		// Run state benchmark
		g, ctx := errgroup.WithContext(ctx)
		var realtimeBalanceDuration, ethBalanceDuration time.Duration
		g.Go(func() error {
			startTime := time.Now()
			err := WaitRealtimeTxToBeConfirmed(ctx, client, signedTx, DefaultTimeoutTxToBeMined, testAddress, realtimeBalance)
			require.NoError(t, err)
			realtimeBalanceDuration = time.Since(startTime)
			return nil
		})

		g.Go(func() error {
			startTime := time.Now()
			err := WaitEthTxToBeConfirmed(ctx, nonRtClient, signedTx, DefaultTimeoutTxToBeMined, testAddress, ethBalance)
			require.NoError(t, err)
			ethBalanceDuration = time.Since(startTime)
			return nil
		})

		// Wait for all goroutines to complete
		err = g.Wait()
		require.NoError(t, err)

		if i == 0 {
			continue
		}
		totalRealtimeBalanceDuration += realtimeBalanceDuration
		totalEthBalanceDuration += ethBalanceDuration

		fmt.Printf("Iteration %v:\n", i)
		fmt.Printf("RT native tx transfer confirmation took: %s\n", realtimeBalanceDuration)
		fmt.Printf("ETH native tx transfer confirmation took: %s\n", ethBalanceDuration)
	}

	avgRealtimeBalanceDuration := time.Duration(int64(totalRealtimeBalanceDuration) / int64(Iterations-1))
	avgEthBalanceDuration := time.Duration(int64(totalEthBalanceDuration) / int64(Iterations-1))

	// Log out metrics
	fmt.Printf("Avg RT native tx transfer confirmation took: %s\n", avgRealtimeBalanceDuration)
	fmt.Printf("Avg ETH native tx transfer confirmation took: %s\n", avgEthBalanceDuration)
}

func TestRealtimeBenchmarkERC20TransferConfirmation(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	ctx := context.Background()
	ec, err := ethclient.Dial(DefaultL2NetworkRealtimeURL)
	require.NoError(t, err)
	client, err := rtclient.NewRealtimeClient(ctx, ec, DefaultL2NetworkRealtimeURL)
	require.NoError(t, err)
	nonRtClient, err := ethclient.Dial(DefaultL2NetworkNoRealtimeURL)
	require.NoError(t, err)

	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(DefaultL2AdminPrivateKey, "0x"))
	require.NoError(t, err)

	// Default test address for tests that require an address
	fromAddress := common.HexToAddress(DefaultL2AdminAddress)
	testAddress := common.HexToAddress("0x1234567890123456789012345678901234567890")

	// Deploy the contract
	erc20Address := deployERC20Contract(t, ctx, privateKey, client)
	transferAmount := new(big.Int).Mul(big.NewInt(1), big.NewInt(1e18))

	startNonce, err := client.PendingNonceAt(context.Background(), fromAddress)
	require.NoError(t, err)

	// Benchmark erc20 transfer tx
	time.Sleep(3 * time.Second)
	var totalRealtimeBalanceDuration, totalEthBalanceDuration time.Duration
	for i := 0; i < Iterations; i++ {
		ethBalance, err := GetErc20Balance(ctx, nonRtClient, testAddress, erc20Address, nil)
		require.NoError(t, err)
		realtimeBalance, err := client.RealtimeGetTokenBalance(ctx, fromAddress, testAddress, erc20Address)
		require.NoError(t, err)
		require.Equal(t, ethBalance.String(), realtimeBalance.String())

		signedTx := erc20TransferTx(t, ctx, privateKey, client, transferAmount, nil, testAddress, erc20Address, startNonce+uint64(i))
		fmt.Printf("Sent tx: %s\n", signedTx.Hash().String())

		// Run state benchmark
		g, ctx := errgroup.WithContext(ctx)
		var realtimeBalanceDuration, ethBalanceDuration time.Duration
		g.Go(func() error {
			startTime := time.Now()
			err := WaitRealtimeErc20TxToBeConfirmed(ctx, client, signedTx, DefaultTimeoutTxToBeMined, fromAddress, testAddress, realtimeBalance)
			require.NoError(t, err)
			realtimeBalanceDuration = time.Since(startTime)
			return nil
		})

		g.Go(func() error {
			startTime := time.Now()
			err := WaitEthErc20TxToBeConfirmed(ctx, nonRtClient, signedTx, DefaultTimeoutTxToBeMined, testAddress, ethBalance)
			require.NoError(t, err)
			ethBalanceDuration = time.Since(startTime)
			return nil
		})

		// Wait for all goroutines to complete
		err = g.Wait()
		require.NoError(t, err)

		if i == 0 {
			continue
		}
		totalRealtimeBalanceDuration += realtimeBalanceDuration
		totalEthBalanceDuration += ethBalanceDuration

		fmt.Printf("Iteration %v:\n", i)
		fmt.Printf("RT erc20 tx transfer confirmation took: %s\n", realtimeBalanceDuration)
		fmt.Printf("ETH erc20 tx transfer confirmation took: %s\n", ethBalanceDuration)
	}

	avgRealtimeBalanceDuration := time.Duration(int64(totalRealtimeBalanceDuration) / int64(Iterations-1))
	avgEthBalanceDuration := time.Duration(int64(totalEthBalanceDuration) / int64(Iterations-1))

	// Log out metrics
	fmt.Printf("Avg RT erc20 tx transfer confirmation took: %s\n", avgRealtimeBalanceDuration)
	fmt.Printf("Avg ETH erc20 tx transfer confirmation took: %s\n", avgEthBalanceDuration)
}

func TestRealtimeBenchmarNewTransactionSubscription(t *testing.T) {
	ctx := context.Background()
	ec, err := ethclient.Dial(DefaultL2NetworkRealtimeURL)
	require.NoError(t, err)
	client, err := rtclient.NewRealtimeClient(ctx, ec, DefaultL2NetworkRealtimeURL)
	require.NoError(t, err)

	wsClient, err := rpc.Dial(DefaultL2NetworkWSURL)
	require.NoError(t, err)

	// Default test address for tests that require an address
	testAddress := common.HexToAddress("0x1234567890123456789012345678901234567890")

	// Benchmark variables
	var totalRealtimeDuration time.Duration

	realtimeMsgCh := make(chan realtimeapi.RealtimeSubResult)
	realtimeSub, err := wsClient.Subscribe(ctx, "eth", realtimeMsgCh, "realtime", map[string]bool{"NewHeads": false, "TransactionExtraInfo": false, "TransactionReceipt": false, "TransactionInnerTxs": false})
	require.NoError(t, err)
	defer realtimeSub.Unsubscribe()

	for i := 0; i < Iterations; i++ {
		// Send tx
		signedTx := nativeTransferTx(t, ctx, client, big.NewInt(Gwei), testAddress.String())
		fmt.Printf("Iteration %v:\n", i)
		fmt.Printf("Sent tx: %s\n", signedTx.Hash().String())

		g, _ := errgroup.WithContext(ctx)
		var subDuration time.Duration

		// realtime subscription
		g.Go(func() error {
			startTime := time.Now()

			for {
				select {
				case msg := <-realtimeMsgCh:
					if msg.TxHash == signedTx.Hash().String() {
						subDuration = time.Since(startTime)
						return nil
					}
				case err := <-realtimeSub.Err():
					return err
				case <-time.After(DefaultTimeoutTxToBeMined):
					return fmt.Errorf("realtime subscription timeout")
				}
			}
		})

		// Wait for all goroutines to complete
		err = g.Wait()
		require.NoError(t, err)

		if i == 0 {
			continue
		}
		totalRealtimeDuration += subDuration
		fmt.Printf("RT newTx sub duration: %s\n", subDuration)
	}

	avgDuration := time.Duration(int64(totalRealtimeDuration) / int64(Iterations-1))

	// Log out metrics
	fmt.Printf("Avg RT newTx sub duration: %s\n", avgDuration)
}
