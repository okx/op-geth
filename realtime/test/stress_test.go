package test

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/realtime/rtclient"
	"github.com/stretchr/testify/require"
)

var (
	NumTxs = 5_000
)

func TestStressSendErc20Txs(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	ctx := context.Background()
	ec, err := ethclient.Dial(DefaultL2NetworkRealtimeURL)
	require.NoError(t, err)
	client, err := rtclient.NewRealtimeClient(ctx, ec, DefaultL2NetworkRealtimeURL)
	require.NoError(t, err)
	// nec, err := ethclient.Dial(DefaultL2NetworkNoRealtimeURL)
	// require.NoError(t, err)
	// nonRtClient, err := rtclient.NewRealtimeClient(ctx, nec, DefaultL2NetworkNoRealtimeURL)
	// require.NoError(t, err)

	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(DefaultL2AdminPrivateKey, "0x"))
	require.NoError(t, err)

	// Default test address for tests that require an address
	fromAddress := common.HexToAddress(DefaultL2AdminAddress)
	testAddress := common.HexToAddress("0x1234567890123456789012345678901234567890")

	// Deploy the contract
	erc20Address := deployERC20Contract(t, ctx, privateKey, client)
	transferAmount := new(big.Int).Mul(big.NewInt(1), big.NewInt(1e18)) // Adjust for token decimals (18 in this case)

	startNonce, err := client.PendingNonceAt(context.Background(), fromAddress)
	require.NoError(t, err)

	// Send erc20 transfer txs to txpool first
	time.Sleep(500 * time.Millisecond)

	signedTxs := make([]*types.Transaction, NumTxs)

	gasPrice, err := client.SuggestGasPrice(ctx)
	require.NoError(t, err)

	for i := 1; i < NumTxs; i++ {
		signedTxs[i] = erc20TransferTx(t, ctx, privateKey, client, transferAmount, gasPrice, testAddress, erc20Address, startNonce+uint64(i))
		fmt.Println("Sent tx count: ", i)
	}

	// Send start nonce to trigger stress test
	signedTxs[0] = erc20TransferTx(t, ctx, privateKey, client, transferAmount, gasPrice, testAddress, erc20Address, startNonce)
	fmt.Println("Starting stress test")

	// g, ctx := errgroup.WithContext(ctx)
	// var totalRealtimeBalanceDuration, totalEthBalanceDuration time.Duration
	// var totalRealtimeBalanceMutex, totalEthBalanceMutex sync.Mutex
	// for i := 0; i < NumTxs; i++ {
	// 	g.Go(func() error {
	// 		startTime := time.Now()
	// 		err = WaitRealtimeTxToBeConfirmed(ctx, client, signedTxs[i], DefaultTimeoutTxToBeMined)
	// 		require.NoError(t, err)
	// 		totalRealtimeBalanceMutex.Lock()
	// 		defer totalRealtimeBalanceMutex.Unlock()
	// 		totalRealtimeBalanceDuration += time.Since(startTime)
	// 		fmt.Printf("RT erc20 tx transfer confirmation took: %s\n", time.Since(startTime))
	// 		return nil
	// 	})

	// 	g.Go(func() error {
	// 		startTime := time.Now()
	// 		err = WaitEthTxToBeConfirmed(ctx, nonRtClient, signedTxs[i], DefaultTimeoutTxToBeMined)
	// 		require.NoError(t, err)
	// 		totalEthBalanceMutex.Lock()
	// 		defer totalEthBalanceMutex.Unlock()
	// 		totalEthBalanceDuration += time.Since(startTime)
	// 		fmt.Printf("ETH erc20 tx transfer confirmation took: %s\n", time.Since(startTime))
	// 		return nil
	// 	})
	// }

	// // Wait for all goroutines to complete
	// err = g.Wait()
	// require.NoError(t, err)

	// avgRealtimeBalanceDuration := time.Duration(int64(totalRealtimeBalanceDuration) / int64(NumTxs))
	// avgEthBalanceDuration := time.Duration(int64(totalEthBalanceDuration) / int64(NumTxs))

	// // Log out metrics
	// fmt.Printf("Avg RT erc20 tx transfer confirmation took: %s\n", avgRealtimeBalanceDuration)
	// fmt.Printf("Avg ETH erc20 tx transfer confirmation took: %s\n", avgEthBalanceDuration)
}
