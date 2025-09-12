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

	wsClient, err := rpc.Dial(DefaultL2NetworkWSURL)
	require.NoError(t, err)

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
	signedTxs := make(map[string]struct{}, NumTxs)

	gasPrice, err := client.SuggestGasPrice(ctx)
	require.NoError(t, err)

	// Benchmark variables
	var totalRealtimeDuration time.Duration

	realtimeMsgCh := make(chan realtimeapi.RealtimeSubResult)
	realtimeSub, err := wsClient.Subscribe(ctx, "eth", realtimeMsgCh, "realtime", map[string]bool{"NewHeads": false, "TransactionExtraInfo": false, "TransactionReceipt": false, "TransactionInnerTxs": false})
	require.NoError(t, err)
	defer realtimeSub.Unsubscribe()

	for i := 1; i < NumTxs; i++ {
		signedTx := erc20TransferTx(t, ctx, privateKey, client, transferAmount, gasPrice, testAddress, erc20Address, startNonce+uint64(i))
		signedTxs[signedTx.Hash().String()] = struct{}{}
		fmt.Println("Sent tx count: ", i)
	}

	// Send start nonce to trigger stress test
	startTime := time.Now()
	signedTx := erc20TransferTx(t, ctx, privateKey, client, transferAmount, gasPrice, testAddress, erc20Address, startNonce)
	signedTxs[signedTx.Hash().String()] = struct{}{}
	fmt.Println("Starting stress test")

	count := 0
	for count < NumTxs {
		select {
		case msg := <-realtimeMsgCh:
			if _, ok := signedTxs[msg.TxHash]; ok {
				fmt.Printf("Confirmed tx: %s\n", msg.TxHash)
				count++
			}
		case err := <-realtimeSub.Err():
			require.NoError(t, err)
		}
	}
	totalRealtimeDuration = time.Since(startTime)
	fmt.Printf("Stress test took: %s\n", totalRealtimeDuration)
}
