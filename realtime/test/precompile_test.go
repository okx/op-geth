package test

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/realtime/rtclient"
	"github.com/stretchr/testify/require"
)

func TestPrecompile(t *testing.T) {
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

	// Deploy precompile caller contract
	precompileCallerAddr := DeployPrecompileCallerContract(t, ctx, client)
	signedTx := SendCallPrecompileTx(t, ctx, client, privateKey, precompileCallerAddr)

	err = WaitTxToBeMined(ctx, client, signedTx, DefaultTimeoutTxToBeMined)
	require.NoError(t, err)

	txReceipt, err := client.RealtimeGetTransactionReceipt(ctx, signedTx.Hash())
	require.NoError(t, err)
	require.NotNil(t, txReceipt, "tx receipt not found")
	require.Equal(t, uint64(1), txReceipt.Status, "tx should be successful")

	// Compare state cache. Precompile should be found in state cache
	mismatches, err := client.RealtimeCompareStateCache(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, len(mismatches), "state cache should have 1 mismatch")
	require.Equal(t, "account 0x0000000000000000000000000000000000000002 not found in database", mismatches[0], "mismatch should be for precompile address")

	// Do eth call on precompile contract to execute sha256 operation with RT cache layer
	result, err := client.RealtimeCall(ctx, common.HexToAddress(DefaultL2AdminAddress), precompileCallerAddr, "0x37E11D600", "0x1", "0x0", "0x4935008e")
	require.NoError(t, err)
	require.Equal(t, "0x", result)
}

func TestTransferToPrecompileAddress(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	ctx := context.Background()
	ec, err := ethclient.Dial(DefaultL2NetworkRealtimeURL)
	require.NoError(t, err)
	client, err := rtclient.NewRealtimeClient(ctx, ec, DefaultL2NetworkRealtimeURL)
	require.NoError(t, err)

	// Create shared RPC client for direct JSON RPC calls to non-realtime node
	nonRealtimeRPCClient, err := ethclient.Dial(DefaultL2NetworkNoRealtimeURL)
	require.NoError(t, err)

	// Use 0x0000000000000000000000000000000000000002 as the testAddress
	testAddress := common.HexToAddress("0x0000000000000000000000000000000000000002")

	balanceBefore, err := client.RealtimeGetBalance(ctx, testAddress)
	require.NoError(t, err)

	// Send tx
	chainID, err := client.ChainID(ctx)
	require.NoError(t, err)
	auth, err := GetAuth(DefaultL2AdminPrivateKey, chainID.Uint64())
	require.NoError(t, err)
	nonce, err := client.RealtimeGetTransactionCount(ctx, auth.From)
	require.NoError(t, err)
	gasPrice, err := client.SuggestGasPrice(ctx)
	require.NoError(t, err)

	tx := types.NewTransaction(
		nonce,
		testAddress,
		big.NewInt(Gwei),
		uint64(1000000),
		gasPrice,
		nil,
	)

	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(DefaultL2AdminPrivateKey, "0x"))
	require.NoError(t, err)

	signer := types.MakeSigner(GetTestChainConfig(DefaultL2ChainID), big.NewInt(1), 0)
	signedTx, err := types.SignTx(tx, signer, privateKey)
	require.NoError(t, err)
	log.Info(fmt.Sprintf("signedTx: %s", signedTx.Hash().String()))

	err = client.SendTransaction(ctx, signedTx)
	require.NoError(t, err)
	err = WaitTxToBeMined(ctx, client, signedTx, DefaultTimeoutTxToBeMined)
	require.NoError(t, err)

	txReceipt, err := client.RealtimeGetTransactionReceipt(ctx, signedTx.Hash())
	require.NoError(t, err)
	require.NotNil(t, txReceipt, "tx receipt not found")
	require.Equal(t, uint64(1), txReceipt.Status, "tx should be successful")

	// Check to ensure realtime balance of precompile address is 1gwei
	balance, err := client.RealtimeGetBalance(ctx, testAddress)
	require.NoError(t, err)
	require.NotEqual(t, balanceBefore, balance, "realtime balance should have incremented")

	// Check to ensure non-realtime balance of precompile address is 1gwei
	nonRTBalance, err := nonRealtimeRPCClient.BalanceAt(ctx, testAddress, nil)
	require.NoError(t, err)
	require.NotEqual(t, balanceBefore, nonRTBalance, "non-realtime balance should have incremented")

	// Check to ensure realtime and non-realtime balances are equal
	require.Equal(t, balance.Uint64(), nonRTBalance.Uint64(), "realtime and non-realtime balances should be equal")

	// Compare state cache
	mismatches, err := client.RealtimeCompareStateCache(ctx)
	require.NoError(t, err)
	require.Empty(t, mismatches, "state cache should have no mismatches")
}
