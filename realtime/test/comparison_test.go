//go:build !skip_smoke_realtime
// +build !skip_smoke_realtime

package test

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/realtime/rtclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

// TestRealtimeComparison is the main test function that compares various RPC methods
// between realtime and non-realtime enabled nodes to ensure output is identical
func TestRealtimeComparison(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	ctx := context.Background()
	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(DefaultL2AdminPrivateKey, "0x"))
	require.NoError(t, err)
	ec, err := ethclient.Dial(DefaultL2NetworkRealtimeURL)
	require.NoError(t, err)
	client, err := rtclient.NewRealtimeClient(ctx, ec, DefaultL2NetworkRealtimeURL)
	require.NoError(t, err)

	// Create shared RPC client for direct JSON RPC calls to non-realtime node
	nonRealtimeRPCClient, err := ethclient.Dial(DefaultL2NetworkNoRealtimeURL)
	require.NoError(t, err)
	rawNonRealtimeRPCClient, err := rpc.Dial(DefaultL2NetworkNoRealtimeURL)
	require.NoError(t, err)
	defer nonRealtimeRPCClient.Close()

	latestBlockNumber, err := client.RealtimeBlockNumber(ctx, "latest")
	require.NoError(t, err)
	fmt.Printf("Latest block number at test start: %d\n", latestBlockNumber)

	testBlocks := []string{}
	for i := 0; i < 10; i++ {
		testBlocks = append(testBlocks, fmt.Sprintf("0x%x", latestBlockNumber-uint64(i)))
	}

	fromAddress := common.HexToAddress(DefaultL2AdminAddress)
	fmt.Printf("Sender: %s\n", fromAddress)
	testAddress := common.HexToAddress("0x1234567890123456789012345678901234567890")
	erc20Address := deployERC20Contract(t, ctx, privateKey, client)
	fmt.Printf("Starting realtime comparison test, realtimeURL: %s, nonRealtimeURL: %s\n", DefaultL2NetworkRealtimeURL, DefaultL2NetworkNoRealtimeURL)

	// TestStatelessAPIs - Block and Transaction Data
	t.Run("TestStatelessAPIs", func(t *testing.T) {
		fmt.Println("Running stateless comparison tests")

		t.Run("blockNumber", func(t *testing.T) {
			realtimeBlockNumber, err := client.RealtimeBlockNumber(ctx, "latest")
			require.NoError(t, err)
			nonRealtimeBlockNumber, err := nonRealtimeRPCClient.BlockNumber(ctx)
			require.NoError(t, err)
			require.GreaterOrEqual(t, realtimeBlockNumber, nonRealtimeBlockNumber)
		})

		t.Run("getBlockByNumber", func(t *testing.T) {
			for _, blockParam := range testBlocks {
				blockNumber, err := convertBlockParam(ctx, client, blockParam)
				require.NoError(t, err)
				realtimeBlock, err := client.RealtimeGetBlockByNumber(ctx, blockNumber)
				require.NoError(t, err)
				var nonRealtimeBlock rtclient.RpcBlock
				err = rawNonRealtimeRPCClient.CallContext(context.Background(), &nonRealtimeBlock, "eth_getBlockByNumber", blockParam, true)
				require.NoError(t, err)
				require.Equal(t, realtimeBlock, nonRealtimeBlock, fmt.Sprintf("block_%v", blockParam))
			}
		})

		t.Run("getBlockByHash", func(t *testing.T) {
			for _, blockParam := range testBlocks {
				blockNumber, err := convertBlockParam(ctx, client, blockParam)
				require.NoError(t, err)

				blockByNumber, err := client.RealtimeGetBlockByNumber(ctx, blockNumber)
				require.NoError(t, err)
				blockHash := *blockByNumber.Hash
				fmt.Printf("Comparing block %v by hash: %s\n", blockParam, blockHash.Hex())

				realtimeBlock, err := client.RealtimeGetBlockByHash(ctx, blockHash, true)
				require.NoError(t, err)
				var nonRealtimeMap rtclient.RpcBlock
				err = rawNonRealtimeRPCClient.CallContext(context.Background(), &nonRealtimeMap, "eth_getBlockByHash", blockHash, true)
				require.NoError(t, err)
				require.Equal(t, realtimeBlock, nonRealtimeMap, fmt.Sprintf("block_%v_hash", blockParam))
			}
		})

		t.Run("getBlockTransactionCount", func(t *testing.T) {
			numberOfTransactions := 5
			_, targetBlockNumber, targetBlockHash := transErc20TokenBatch(t, context.Background(), client, nonRealtimeRPCClient, erc20Address, big.NewInt(Gwei), testAddress.String(), numberOfTransactions)

			realtimeTxCountByNumber, err := client.RealtimeGetBlockTransactionCountByNumber(ctx, targetBlockNumber)
			require.NoError(t, err)
			nonRealtimeTxCount, err := nonRealtimeRPCClient.TransactionCount(ctx, targetBlockHash)
			require.NoError(t, err)
			realtimeTxCountByHash, err := client.RealtimeGetBlockTransactionCountByHash(ctx, targetBlockHash)
			require.NoError(t, err)
			require.Equal(t, realtimeTxCountByNumber, uint64(nonRealtimeTxCount))
			require.Equal(t, realtimeTxCountByHash, uint64(nonRealtimeTxCount))
		})

		t.Run("getBlockInternalTransactions", func(t *testing.T) {
			numberOfTransactions := 5
			_, targetBlockNumber, _ := transErc20TokenBatch(t, context.Background(), client, nonRealtimeRPCClient, erc20Address, big.NewInt(Gwei), testAddress.String(), numberOfTransactions)

			realtimeInternalTxs, err := client.RealtimeGetBlockInternalTransactions(ctx, targetBlockNumber)
			require.NoError(t, err)
			require.NotNil(t, realtimeInternalTxs)
			blockNumberHex := fmt.Sprintf("0x%x", targetBlockNumber)
			var nonRealtimeInternalTxs map[common.Hash][]*types.InnerTx
			err = rawNonRealtimeRPCClient.CallContext(context.Background(), &nonRealtimeInternalTxs, "eth_getBlockInternalTransactions", blockNumberHex)
			require.NoError(t, err)
			require.NotNil(t, nonRealtimeInternalTxs)
			require.Equal(t, realtimeInternalTxs, nonRealtimeInternalTxs)
		})

		t.Run("getTransactionByHash", func(t *testing.T) {
			numberOfTransactions := 5
			txHashes, _, _ := transErc20TokenBatch(t, context.Background(), client, nonRealtimeRPCClient, erc20Address, big.NewInt(Gwei), testAddress.String(), numberOfTransactions)
			for _, txHash := range txHashes {
				realtimeTransaction, err := client.RealtimeGetTransactionByHash(ctx, common.HexToHash(txHash))
				require.NoError(t, err)
				var nonRealtimeTransaction rtclient.RpcTransaction
				err = rawNonRealtimeRPCClient.CallContext(context.Background(), &nonRealtimeTransaction, "eth_getTransactionByHash", common.HexToHash(txHash))
				require.NoError(t, err)
				require.Equal(t, realtimeTransaction, nonRealtimeTransaction)
			}
		})

		t.Run("getRawTransactionByHash", func(t *testing.T) {
			numberOfTransactions := 5
			txHashes, _, _ := transErc20TokenBatch(t, context.Background(), client, nonRealtimeRPCClient, erc20Address, big.NewInt(Gwei), testAddress.String(), numberOfTransactions)
			for _, txHash := range txHashes {
				realtimeTransactionBytes, err := client.RealtimeGetRawTransactionByHash(ctx, common.HexToHash(txHash))
				require.NoError(t, err)
				realtimeTransactionHex := "0x" + hex.EncodeToString(realtimeTransactionBytes)
				var nonRealtimeTransaction string
				err = rawNonRealtimeRPCClient.CallContext(context.Background(), &nonRealtimeTransaction, "eth_getRawTransactionByHash", common.HexToHash(txHash))
				require.NoError(t, err)
				require.Equal(t, realtimeTransactionHex, nonRealtimeTransaction)
			}

		})

		t.Run("getTransactionReceipt", func(t *testing.T) {
			numberOfTransactions := 5
			txHashes, _, _ := transErc20TokenBatch(t, context.Background(), client, nonRealtimeRPCClient, erc20Address, big.NewInt(Gwei), testAddress.String(), numberOfTransactions)
			for _, txHash := range txHashes {
				realtimeReceipt, err := client.RealtimeGetTransactionReceipt(ctx, common.HexToHash(txHash))
				require.NoError(t, err)
				nonRealtimeReceipt, err := nonRealtimeRPCClient.TransactionReceipt(ctx, common.HexToHash(txHash))
				require.NoError(t, err)
				require.Equal(t, realtimeReceipt, nonRealtimeReceipt)
			}
		})

		t.Run("getInternalTransactions", func(t *testing.T) {
			numberOfTransactions := 5
			txHashes, _, _ := transErc20TokenBatch(t, context.Background(), client, nonRealtimeRPCClient, erc20Address, big.NewInt(Gwei), testAddress.String(), numberOfTransactions)
			for _, txHash := range txHashes {
				realtimeInternalTxs, err := client.RealtimeGetInternalTransactions(ctx, common.HexToHash(txHash))
				require.NoError(t, err)

				var nonRealtimeInternalTxs []types.InnerTx
				err = rawNonRealtimeRPCClient.CallContext(context.Background(), &nonRealtimeInternalTxs, "eth_getInternalTransactions", common.HexToHash(txHash))
				require.NoError(t, err)
				require.NotNil(t, nonRealtimeInternalTxs)
				require.Equal(t, realtimeInternalTxs, nonRealtimeInternalTxs)
			}
		})

		t.Run("getTransactionByBlockAndIndex", func(t *testing.T) {
			numberOfTransactions := 5
			txHashes, _, _ := transErc20TokenBatch(t, context.Background(), client, nonRealtimeRPCClient, erc20Address, big.NewInt(Gwei), testAddress.String(), numberOfTransactions)
			for _, txHash := range txHashes {
				receipt, err := client.RealtimeGetTransactionReceipt(ctx, common.HexToHash(txHash))
				require.NoError(t, err)
				require.NotNil(t, receipt)
				realtimeTxByNumber, err := client.RealtimeGetTransactionByBlockNumberAndIndex(ctx, receipt.BlockNumber.Uint64(), receipt.TransactionIndex)
				require.NoError(t, err)
				realtimeTxByHash, err := client.RealtimeGetTransactionByBlockHashAndIndex(ctx, receipt.BlockHash, receipt.TransactionIndex)
				require.NoError(t, err)
				require.Equal(t, realtimeTxByNumber, realtimeTxByHash)

				var nonRealtimeTxByNumber rtclient.RpcTransaction
				err = rawNonRealtimeRPCClient.CallContext(context.Background(), &nonRealtimeTxByNumber, "eth_getTransactionByBlockNumberAndIndex", receipt.BlockNumber.Uint64(), receipt.TransactionIndex)
				require.NoError(t, err)
				require.Equal(t, realtimeTxByNumber, nonRealtimeTxByNumber)

				var nonRealtimeTxByHash rtclient.RpcTransaction
				err = rawNonRealtimeRPCClient.CallContext(context.Background(), &nonRealtimeTxByHash, "eth_getTransactionByBlockHashAndIndex", receipt.BlockHash, receipt.TransactionIndex)
				require.NoError(t, err)
				require.Equal(t, realtimeTxByHash, nonRealtimeTxByHash)

				require.Equal(t, realtimeTxByNumber, nonRealtimeTxByNumber)
				require.Equal(t, realtimeTxByHash, nonRealtimeTxByHash)
			}
		})

		t.Run("getBlockReceipts", func(t *testing.T) {
			numberOfTransactions := 5
			_, targetBlockNumber, targetBlockHash := transErc20TokenBatch(t, context.Background(), client, nonRealtimeRPCClient, erc20Address, big.NewInt(Gwei), testAddress.String(), numberOfTransactions)

			receiptsByNumber, err := client.BlockReceipts(ctx, rpc.BlockNumberOrHashWithNumber(rpc.BlockNumber(targetBlockNumber)))
			require.NoError(t, err)
			require.NotNil(t, receiptsByNumber)
			receiptsByHash, err := client.BlockReceipts(ctx, rpc.BlockNumberOrHashWithHash(targetBlockHash, true))
			require.NoError(t, err)
			require.NotNil(t, receiptsByHash)

			nonRealtimeReceiptsByNumber, err := nonRealtimeRPCClient.BlockReceipts(ctx, rpc.BlockNumberOrHashWithNumber(rpc.BlockNumber(targetBlockNumber)))
			require.NoError(t, err)
			require.NotNil(t, nonRealtimeReceiptsByNumber)
			nonRealtimeReceiptsByHash, err := nonRealtimeRPCClient.BlockReceipts(ctx, rpc.BlockNumberOrHashWithHash(targetBlockHash, true))
			require.NoError(t, err)
			require.NotNil(t, nonRealtimeReceiptsByHash)

			require.Equal(t, receiptsByNumber, nonRealtimeReceiptsByNumber)
			require.Equal(t, receiptsByHash, nonRealtimeReceiptsByHash)
		})
	})

	// TestStateAPIs - Balances, Code, Storage, and Contract Calls
	t.Run("TestStateAPIs", func(t *testing.T) {
		log.Info("Running state comparison tests")

		numberOfTransactions := 5
		_, targetBlockNumber, targetBlockHash := transErc20TokenBatch(t, context.Background(), client, nonRealtimeRPCClient, erc20Address, big.NewInt(Gwei), testAddress.String(), numberOfTransactions)

		t.Run("call", func(t *testing.T) {
			data, err := erc20ABI.Pack("balanceOf", fromAddress)
			require.NoError(t, err)
			callArgs := ethereum.CallMsg{
				From: testAddress,
				To:   &erc20Address,
				Gas:  0x100000,
				Data: data,
			}

			// Test block number
			realtimeCall, err := client.CallContract(ctx, callArgs, big.NewInt(int64(targetBlockNumber)))
			require.NoError(t, err)
			nonRealtimeCall, err := nonRealtimeRPCClient.CallContract(ctx, callArgs, big.NewInt(int64(targetBlockNumber)))
			require.NoError(t, err)
			require.Equal(t, realtimeCall, nonRealtimeCall)

			// Test block hash
			realtimeCall, err = client.CallContractAtHash(ctx, callArgs, targetBlockHash)
			require.NoError(t, err)
			nonRealtimeCall, err = nonRealtimeRPCClient.CallContractAtHash(ctx, callArgs, targetBlockHash)
			require.NoError(t, err)
			require.Equal(t, realtimeCall, nonRealtimeCall)
		})

		t.Run("getBalance", func(t *testing.T) {
			// Test block number
			realtimeBalance, err := client.BalanceAt(ctx, testAddress, big.NewInt(int64(targetBlockNumber)))
			require.NoError(t, err)
			nonRealtimeBalance, err := nonRealtimeRPCClient.BalanceAt(ctx, testAddress, big.NewInt(int64(targetBlockNumber)))
			require.NoError(t, err)
			require.Equal(t, realtimeBalance, nonRealtimeBalance)

			// Test block hash
			realtimeBalance, err = client.BalanceAtHash(ctx, testAddress, targetBlockHash)
			require.NoError(t, err)
			nonRealtimeBalance, err = nonRealtimeRPCClient.BalanceAtHash(ctx, testAddress, targetBlockHash)
			require.NoError(t, err)
			require.Equal(t, realtimeBalance, nonRealtimeBalance)
		})

		t.Run("getTransactionCount", func(t *testing.T) {
			// Test block number
			realtimeTransactionCount, err := client.NonceAt(ctx, testAddress, big.NewInt(int64(targetBlockNumber)))
			require.NoError(t, err)
			nonRealtimeTransactionCount, err := nonRealtimeRPCClient.NonceAt(ctx, testAddress, big.NewInt(int64(targetBlockNumber)))
			require.NoError(t, err)
			require.Equal(t, realtimeTransactionCount, nonRealtimeTransactionCount)

			// Test block hash
			realtimeTransactionCount, err = client.NonceAtHash(ctx, testAddress, targetBlockHash)
			require.NoError(t, err)
			nonRealtimeTransactionCount, err = nonRealtimeRPCClient.NonceAtHash(ctx, testAddress, targetBlockHash)
			require.NoError(t, err)
			require.Equal(t, realtimeTransactionCount, nonRealtimeTransactionCount)
		})

		t.Run("getCode", func(t *testing.T) {
			// Test block number
			realtimeCode, err := client.CodeAt(ctx, erc20Address, big.NewInt(int64(targetBlockNumber)))
			require.NoError(t, err)
			nonRealtimeCode, err := nonRealtimeRPCClient.CodeAt(ctx, erc20Address, big.NewInt(int64(targetBlockNumber)))
			require.NoError(t, err)
			require.Equal(t, realtimeCode, nonRealtimeCode)

			// Test block hash
			realtimeCode, err = client.CodeAtHash(ctx, erc20Address, targetBlockHash)
			require.NoError(t, err)
			nonRealtimeCode, err = nonRealtimeRPCClient.CodeAtHash(ctx, erc20Address, targetBlockHash)
			require.NoError(t, err)
			require.Equal(t, realtimeCode, nonRealtimeCode)
		})

		t.Run("getStorageAt", func(t *testing.T) {
			// Test block number
			realtimeStorage, err := client.StorageAt(ctx, erc20Address, common.HexToHash("0x2"), big.NewInt(int64(targetBlockNumber)))
			require.NoError(t, err)
			nonRealtimeStorage, err := nonRealtimeRPCClient.StorageAt(ctx, erc20Address, common.HexToHash("0x2"), big.NewInt(int64(targetBlockNumber)))
			require.NoError(t, err)
			require.Equal(t, realtimeStorage, nonRealtimeStorage)

			// Test block hash
			realtimeStorage, err = client.StorageAtHash(ctx, erc20Address, common.HexToHash("0x2"), targetBlockHash)
			require.NoError(t, err)
			nonRealtimeStorage, err = nonRealtimeRPCClient.StorageAtHash(ctx, erc20Address, common.HexToHash("0x2"), targetBlockHash)
			require.NoError(t, err)
			require.Equal(t, realtimeStorage, nonRealtimeStorage)
		})
	})
}
