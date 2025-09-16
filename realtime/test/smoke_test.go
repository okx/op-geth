package test

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/realtime/realtimeapi"
	"github.com/ethereum/go-ethereum/realtime/rtclient"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/triedb"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

const Gwei = 1000000000

func TestRealtimeRPC(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	// Preapre to deploy a ERC20 contract
	ctx := context.Background()
	ec, err := ethclient.Dial(DefaultL2NetworkRealtimeURL)
	require.NoError(t, err)
	client, err := rtclient.NewRealtimeClient(ctx, ec, DefaultL2NetworkRealtimeURL)
	require.NoError(t, err)
	blockNumber := setupRealtimeTestEnvironment(t, ctx, client)

	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(DefaultL2AdminPrivateKey, "0x"))
	require.NoError(t, err)
	fromAddress := common.HexToAddress(DefaultL2AdminAddress)
	fmt.Printf("Sender: %s\n", fromAddress)

	// Default test address for tests that require an address
	testAddress := common.HexToAddress("0x1234567890123456789012345678901234567890")

	// Used to check whether the result returned by the interface call is correct
	time.Sleep(1 * time.Second)
	originNonce, err := client.RealtimeGetTransactionCount(ctx, fromAddress)
	require.NoError(t, err)
	originBalance, err := client.RealtimeGetBalance(ctx, testAddress)
	require.NoError(t, err)

	// Transfer native token
	txHash := transToken(t, ctx, client, big.NewInt(Gwei), testAddress.String())

	// Deploy the contract
	erc20Address := deployERC20Contract(t, ctx, privateKey, client)

	t.Run("RealtimeLatestBlockNumber", func(t *testing.T) {
		blockNumber, err := client.RealtimeBlockNumber(ctx, "latest")
		require.NoError(t, err)
		fmt.Printf("RealtimeLatestBlockNumber result: %d\n", blockNumber)
	})

	t.Run("RealtimePendingBlockNumber", func(t *testing.T) {
		blockNumber, err := client.RealtimeBlockNumber(ctx, "pending")
		require.NoError(t, err)
		fmt.Printf("RealtimePendingBlockNumber result: %d\n", blockNumber)
	})

	t.Run("RealtimeGetBlockTransactionCountByNumber", func(t *testing.T) {
		transactionCount, err := client.RealtimeGetBlockTransactionCountByNumber(ctx, blockNumber)
		require.NoError(t, err)
		fmt.Printf("RealtimeGetBlockTransactionCountByNumber result: %d\n", transactionCount)
	})

	t.Run("RealtimeLatestGetBlockTransactionCount", func(t *testing.T) {
		transactionCount, err := client.RealtimeGetBlockTransactionCount(ctx, "latest")
		require.NoError(t, err)
		fmt.Printf("RealtimeGetLatestBlockTransactionCount result: %d\n", transactionCount)
	})

	t.Run("RealtimePendingGetBlockTransactionCount", func(t *testing.T) {
		transactionCount, err := client.RealtimeGetBlockTransactionCount(ctx, "pending")
		require.NoError(t, err)
		fmt.Printf("RealtimeGetPendingBlockTransactionCount result: %d\n", transactionCount)
	})

	t.Run("RealtimeGetTransactionByHash", func(t *testing.T) {
		result, err := client.RealtimeGetTransactionByHash(ctx, common.HexToHash(txHash))
		require.NoError(t, err)

		receipt, err := client.RealtimeGetTransactionReceipt(ctx, common.HexToHash(txHash))
		require.NoError(t, err)
		require.NotNil(t, receipt, "GetTransactionReceipt should return receipt")

		txHashIndex := int(*result.TransactionIndex)
		receiptIndex := int(receipt.TransactionIndex)
		require.Equal(t, receiptIndex, txHashIndex)
		fmt.Printf("RealtimeGetTransactionByHash result type: %T\n", result)
	})

	t.Run("RealtimeGetRawTransactionByHash", func(t *testing.T) {
		result, err := client.RealtimeGetRawTransactionByHash(ctx, common.HexToHash(txHash))
		require.NoError(t, err)
		fmt.Printf("RealtimeGetRawTransactionByHash result type: %T\n", result)
	})

	t.Run("RealtimeGetTransactionReceipt", func(t *testing.T) {
		receipt, err := client.RealtimeGetTransactionReceipt(ctx, common.HexToHash(txHash))
		require.NoError(t, err)
		require.NotNil(t, receipt)
		fmt.Printf("RealtimeGetTransactionReceipt result type: %T\n", receipt)
	})

	t.Run("RealtimeGetInternalTransactions", func(t *testing.T) {
		tx, err := client.RealtimeGetInternalTransactions(ctx, common.HexToHash(txHash))
		require.NoError(t, err)
		fmt.Printf("RealtimeGetInternalTransactions result type: %T\n", tx)
	})

	t.Run("RealtimeGetBalance", func(t *testing.T) {
		balance, err := client.RealtimeGetBalance(ctx, testAddress)
		require.NoError(t, err)
		require.Equal(t, originBalance.Add(originBalance, big.NewInt(Gwei)).String(), balance.String(), "Balance should increase by 1 Gwei")
		fmt.Printf("RealtimeGetBalance result for test address: %s\n", balance.String())
	})

	t.Run("RealtimeGetTransactionCount", func(t *testing.T) {
		nonce, err := client.RealtimeGetTransactionCount(ctx, fromAddress)
		require.NoError(t, err)
		require.Equal(t, originNonce+2, nonce)
		fmt.Printf("RealtimeGetTransactionCount result for sender address: %d\n", nonce)
	})

	t.Run("RealtimeGetCode", func(t *testing.T) {
		code, err := client.RealtimeGetCode(ctx, erc20Address)
		require.NoError(t, err)
		require.NotEmpty(t, code, "Contract code should not be empty")
		fmt.Printf("RealtimeGetCode result for erc20 contract %s: %s\n", erc20Address, code)
	})

	t.Run("RealtimeGetStorageAt", func(t *testing.T) {
		// 0x2 is refered to _totalSupply field
		value, err := client.RealtimeGetStorageAt(ctx, erc20Address, "0x2")
		require.NoError(t, err)
		require.Equal(t, "0x00000000000000000000000000000000000000000052b7d2dcc80cd2e4000000", value, "Storage at index 0x2 should be equal to 1000000000000000000000")
		fmt.Printf("RealtimeGetStorageAt result for erc20 contract %s at index %s: %s\n", erc20Address, "0x2", value)
	})

	t.Run("RealtimeCall", func(t *testing.T) {
		data, err := erc20ABI.Pack("balanceOf", fromAddress)
		require.NoError(t, err)
		value, err := client.RealtimeCall(ctx, testAddress, erc20Address, "0x100000", "0x1", "0x0", fmt.Sprintf("0x%x", data))
		require.NoError(t, err)
		require.Equal(t, "0x00000000000000000000000000000000000000000052b7d2dcc80cd2e4000000", value, fmt.Sprintf("Balance of %s should be equal to 1000000000000000000000", fromAddress))
		fmt.Printf("RealtimeCall result for erc20 contract %s calling method balanceOf %s: %s\n", erc20Address, fromAddress, value)
	})

	t.Run("RealtimeEstimateGas", func(t *testing.T) {
		// Test standard eth_estimateGas with a simple transfer
		transferArgs := map[string]interface{}{
			"from":  fromAddress,
			"to":    testAddress,
			"value": (*hexutil.Big)(big.NewInt(1)),
		}

		gasEstimate, err := client.RealtimeEstimateGas(ctx, transferArgs)
		require.NoError(t, err)
		require.Equal(t, gasEstimate, uint64(21_000), "Mative transfer txs gas should be 21_000")

		// Test gas estimation for a contract call (ERC20 transfer)
		transferData, err := erc20ABI.Pack("transfer", erc20Address, big.NewInt(1))
		require.NoError(t, err)

		contractCallArgs := map[string]interface{}{
			"from": fromAddress,
			"to":   erc20Address,
			"data": (*hexutil.Bytes)(&transferData),
		}

		gasEstimateCall, err := client.RealtimeEstimateGas(ctx, contractCallArgs)
		require.NoError(t, err)
		require.Greater(t, gasEstimateCall, gasEstimate, "Contract call should require more gas than simple transfer")
	})

	// Test call for block height specific
	t.Run("RealtimeCallWithHeight", func(t *testing.T) {
		data, err := erc20ABI.Pack("balanceOf", fromAddress)
		require.NoError(t, err)

		startValue, err := client.RealtimeCall(ctx, testAddress, erc20Address, "0x100000", "0x1", "0x0", fmt.Sprintf("0x%x", data))
		require.NoError(t, err)
		require.Equal(t, "0x00000000000000000000000000000000000000000052b7d2dcc80cd2e4000000", startValue, fmt.Sprintf("Balance of %s should be equal to 1e+26", fromAddress))

		// Send balance transfer
		transferAmount := new(big.Int).Mul(big.NewInt(1), big.NewInt(1e18)) // Adjust for token decimals (18 in this case)
		nonce, err := client.RealtimeGetTransactionCount(ctx, fromAddress)
		require.NoError(t, err)
		signedTx := erc20TransferTx(t, ctx, privateKey, client, transferAmount, nil, testAddress, erc20Address, nonce)
		err = WaitTxToBeMined(ctx, client, signedTx, DefaultTimeoutTxToBeMined)
		require.NoError(t, err)

		// Get tx block number
		receipt, err := client.RealtimeGetTransactionReceipt(ctx, signedTx.Hash())
		require.NoError(t, err)
		require.NotNil(t, receipt)
		targetBlockNumber := receipt.BlockNumber.Uint64()

		correctValue, err := client.RealtimeCall(ctx, testAddress, erc20Address, "0x100000", "0x1", "0x0", fmt.Sprintf("0x%x", data))
		require.NoError(t, err)
		require.Equal(t, "0x00000000000000000000000000000000000000000052b7d2cee7561f3c9c0000", correctValue, fmt.Sprintf("Balance of %s should be equal to 9.9999999e+25 after transfer", fromAddress))
		require.NotEqual(t, startValue, correctValue)

		// Send balance transfer
		signedTx = erc20TransferTx(t, ctx, privateKey, client, transferAmount, nil, testAddress, erc20Address, nonce+1)
		err = WaitTxToBeMined(ctx, client, signedTx, DefaultTimeoutTxToBeMined)
		require.NoError(t, err)

		endValue, err := client.RealtimeCall(ctx, testAddress, erc20Address, "0x100000", "0x1", "0x0", fmt.Sprintf("0x%x", data))
		require.NoError(t, err)
		require.NotEqual(t, endValue, correctValue)
		require.Equal(t, "0x00000000000000000000000000000000000000000052b7d2c1069f6b95380000", endValue, fmt.Sprintf("Balance of %s should be equal to 9.9999998e+25 after transfer", fromAddress))

		// Get block height specific state
		testValue, err := GetErc20Balance(ctx, client, testAddress, erc20Address, new(big.Int).SetUint64(targetBlockNumber))
		require.NoError(t, err)
		require.NotEqual(t, testValue, correctValue)
	})

	t.Run("RealtimeGetBlockByNumber", func(t *testing.T) {
		latestBlockNumber, err := client.RealtimeBlockNumber(ctx, "latest")
		if err != nil {
			fmt.Printf("RealtimeGetBlockNumber error: %v\n", err)
		}
		block, err := client.RealtimeGetBlockByNumber(ctx, latestBlockNumber)
		require.NoError(t, err)
		require.NotNil(t, block, "Block should not be nil")
		require.NotNil(t, block["hash"], "Block hash should not be nil")
		fmt.Printf("RealtimeGetBlockByNumber result block number: %v, hash: %v, txCount: %v\n", block["number"], block["hash"], len(block["transactions"].([]interface{})))
	})

	t.Run("RealtimeGetPendingBlock", func(t *testing.T) {
		pendingBlock, err := client.RealtimeGetBlock(ctx, "pending")
		require.NoError(t, err)
		require.NotNil(t, pendingBlock, "Pending block should not be nil")
		require.Nil(t, pendingBlock["hash"], "Block hash should be nil")
		fmt.Printf("RealtimeGetBlock result block number: %v, txCount: %v\n", pendingBlock["number"], len(pendingBlock["transactions"].([]interface{})))
	})

	t.Run("RealtimeGetBlockByHash", func(t *testing.T) {
		latestBlockNumber, err := client.RealtimeBlockNumber(ctx, "latest")
		require.NoError(t, err)
		require.Greater(t, latestBlockNumber, uint64(0), "Latest block number should be greater than 0")

		// Get the block by number
		fmt.Printf("Getting finalized block by number: %v\n", latestBlockNumber)
		blockByNumber, err := client.RealtimeGetBlockByNumber(ctx, latestBlockNumber)
		require.NoError(t, err)
		require.NotNil(t, blockByNumber, "Block by number should not be nil")

		// Extract the block hash
		blockHashStr, ok := blockByNumber["hash"].(string)
		require.True(t, ok, "Block hash should be a string")
		require.NotEmpty(t, blockHashStr, "Block hash should not be empty")

		// Test getting the same block by hash
		blockByHash, err := client.RealtimeGetBlockByHash(ctx, common.HexToHash(blockHashStr), true)
		require.NoError(t, err)
		require.NotNil(t, blockByHash, "Block should not be nil")
		require.NotNil(t, blockByHash["hash"], "Block hash should not be nil")

		// Verify that both methods return the same block
		require.Equal(t, blockByNumber["hash"], blockByHash["hash"], "Block hashes should match")
		require.Equal(t, blockByNumber["number"], blockByHash["number"], "Block numbers should match")

		fmt.Printf("RealtimeGetBlockByHash result - finalized block number: %v, hash: %v, txCount: %v\n", blockByHash["number"], blockByHash["hash"], len(blockByHash["transactions"].([]interface{})))
	})

	t.Run("RealtimeGetBlockTransactionCountByHash", func(t *testing.T) {
		numberOfTransactions := 10

		// Create the specified number of transactions and wait for them to be mined
		txHashes := transTokenBatch(t, context.Background(), client, big.NewInt(Gwei), testAddress.String(), numberOfTransactions)
		lastTxHash := txHashes[len(txHashes)-1]

		// Get the block information from the last transaction's receipt
		receipt, err := client.RealtimeGetTransactionReceipt(ctx, common.HexToHash(lastTxHash))
		require.NoError(t, err)
		require.NotNil(t, receipt, "Transaction receipt should not be nil")

		targetBlockNumber := receipt.BlockNumber.Uint64()
		targetBlockHash := receipt.BlockHash

		// Get the actual transaction count for this block by number
		actualTxCount, err := client.RealtimeGetBlockTransactionCountByNumber(ctx, targetBlockNumber)
		require.NoError(t, err)

		// Test getting transaction count by hash
		transactionCount, err := client.RealtimeGetBlockTransactionCountByHash(ctx, targetBlockHash)
		require.NoError(t, err)

		require.Equal(t, actualTxCount, transactionCount, fmt.Sprintf("Transaction count by hash should match count by number (%d)", actualTxCount))

		fmt.Printf("RealtimeGetBlockTransactionCountByHash result: %d (verified against block content) ✓\n", transactionCount)

	})

	t.Run("RealtimeGetBlockInternalTransactions", func(t *testing.T) {
		txHash := transToken(t, ctx, client, big.NewInt(Gwei), testAddress.String())

		var targetBlockNumber uint64

		// Get the block number directly from the transaction receipt
		receipt, err := client.RealtimeGetTransactionReceipt(ctx, common.HexToHash(txHash))
		require.NoError(t, err)

		targetBlockNumber = receipt.BlockNumber.Uint64()

		// Test getting internal transactions for the block
		internalTxs, err := client.RealtimeGetBlockInternalTransactions(ctx, targetBlockNumber)
		require.NoError(t, err)
		require.NotNil(t, internalTxs, "Internal transactions map should not be nil")

		// Count total internal transactions
		totalInternalTxs := 0
		for _, innerTxs := range internalTxs {
			totalInternalTxs += len(innerTxs)
		}

		require.IsType(t, map[common.Hash][]*types.InnerTx{}, internalTxs, "Should return correct type")
		fmt.Printf("RealtimeGetBlockInternalTransactions successfully returned data for block %d\n", targetBlockNumber)
	})

	t.Run("RealtimeGetBlockReceipts", func(t *testing.T) {
		numberOfTransactions := 10

		// Create the specified number of transactions and wait for them to be mined
		txHashes := transTokenBatch(t, context.Background(), client, big.NewInt(Gwei), testAddress.String(), numberOfTransactions)
		lastTxHash := txHashes[len(txHashes)-1]

		// Get the block information from the last transaction's receipt
		receipt, err := client.RealtimeGetTransactionReceipt(ctx, common.HexToHash(lastTxHash))
		require.NoError(t, err)
		require.NotNil(t, receipt, "Transaction receipt should not be nil")

		receiptsByNumber, err := client.RealtimeGetBlockReceiptsByNumber(ctx, receipt.BlockNumber.Uint64())
		require.NoError(t, err)
		require.NotNil(t, receiptsByNumber, "Transaction receipts by number should not be nil")
		for _, receipt := range receiptsByNumber {
			require.NoError(t, err)
			require.NotNil(t, receipt)
			log.Info(fmt.Sprintf("RealtimeGetBlockReceiptsByNumber result type: %T", receipt))
		}

		receiptsByHash, err := client.RealtimeGetBlockReceiptsByHash(ctx, receipt.BlockHash)
		require.NoError(t, err)
		require.NotNil(t, receiptsByHash, "Transaction receipts by hash should not be nil")
		for _, receipt := range receiptsByHash {
			require.NoError(t, err)
			require.NotNil(t, receipt)
			log.Info(fmt.Sprintf("RealtimeGetBlockReceiptsByHash result type: %T", receipt))
		}
	})

	t.Run("RealtimeEnabled", func(t *testing.T) {
		// Test with valid "pending" tag
		isEnabled, err := client.RealtimeEnabled(ctx)
		require.NoError(t, err)
		require.IsType(t, bool(false), isEnabled, "RealtimeEnabled should return bool")

		if isEnabled {
			fmt.Printf("RealtimeEnabled: Realtime feature is enabled and cache is ready\n")
		} else {
			fmt.Printf("RealtimeEnabled: Realtime feature is disabled or cache is not ready\n")
		}
	})

	t.Run("RealtimeLatest", func(t *testing.T) {
		latestBlockNum, err := client.RealtimeBlockNumber(ctx, "latest")
		require.NoError(t, err)
		require.Greater(t, latestBlockNum, uint64(0), "Latest block number should be greater than 0")

		// Change chain-state
		fromAddress := common.HexToAddress(DefaultL2AdminAddress)
		testAddress := common.HexToAddress("0x1234567890123456789012345678901234567890")
		transferAmount := new(big.Int).Mul(big.NewInt(1), big.NewInt(1e18))
		nonce, err := client.RealtimeGetTransactionCount(ctx, fromAddress)
		require.NoError(t, err)
		signedTx := erc20TransferTx(t, ctx, privateKey, client, transferAmount, nil, testAddress, erc20Address, nonce)
		err = WaitTxToBeMined(ctx, client, signedTx, DefaultTimeoutTxToBeMined)
		require.NoError(t, err)

		// Test state APIs
		balance, err := client.RealtimeGetBalance(ctx, fromAddress)
		require.NoError(t, err)
		require.Greater(t, balance.Uint64(), uint64(0), "Balance should be greater than 0")

		tokenBalance, err := client.RealtimeGetTokenBalance(ctx, fromAddress, testAddress, erc20Address)
		require.NoError(t, err)
		require.Greater(t, tokenBalance.Uint64(), uint64(0), "Token balance should be greater than 0")

		// Test stateless APIs
		block, err := client.RealtimeGetBlockByNumber(ctx, latestBlockNum)
		require.NoError(t, err)
		require.NotNil(t, block, "Block should not be nil")
		require.NotNil(t, block["hash"], "Block hash should not be nil")

		blockByHash, err := client.RealtimeGetBlockByHash(ctx, common.HexToHash(block["hash"].(string)), true)
		require.NoError(t, err)
		require.NotNil(t, blockByHash, "Block should not be nil")
		require.NotNil(t, blockByHash["hash"], "Block hash should not be nil")

		require.Equal(t, block["hash"], blockByHash["hash"], "Block hashes should match")
		require.Equal(t, block["number"], blockByHash["number"], "Block numbers should match")
	})

	t.Run("RealtimeSubscriptionWorking", func(t *testing.T) {
		var iterations = 11
		latestBlockNum, err := client.RealtimeBlockNumber(ctx, "latest")
		require.NoError(t, err)
		require.Greater(t, latestBlockNum, uint64(0), "Latest block number should be greater than 0")

		wsClient, err := rpc.Dial(DefaultL2NetworkWSURL)
		require.NoError(t, err)

		realtimeMsgCh := make(chan realtimeapi.RealtimeSubResult)
		realtimeSub, err := wsClient.Subscribe(ctx, "eth", realtimeMsgCh, "realtime", map[string]bool{"NewHeads": false, "TransactionExtraInfo": true, "TransactionReceipt": true, "TransactionInnerTxs": true})
		require.NoError(t, err)
		defer realtimeSub.Unsubscribe()

		for i := 0; i < iterations; i++ {
			// Send tx
			signedTx := nativeTransferTx(t, ctx, client, big.NewInt(Gwei), testAddress.String())
			g, _ := errgroup.WithContext(ctx)

			// realtime subscription
			g.Go(func() error {

				for {
					select {
					case msg := <-realtimeMsgCh:
						// BlockTime should always be returned
						if msg.BlockTime == 0 {
							return fmt.Errorf("block time is 0 and not sent")
						}
						// Since we set the TransactionExtraInfo, TransactionReceipt, and TransactionInnerTxs to true, these fields should not be nil
						if msg.TxData == nil {
							return fmt.Errorf("tx data is nil")
						}
						if msg.Receipt == nil {
							return fmt.Errorf("receipt is nil")
						}
						if msg.InnerTxs == nil {
							return fmt.Errorf("inner transactions is nil")
						}
						if msg.TxHash == signedTx.Hash().String() {
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
		}
	})
}

func TestRealtimeStateIsConsistent(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	ctx := context.Background()
	ec, err := ethclient.Dial(DefaultL2NetworkRealtimeURL)
	require.NoError(t, err)
	client, err := rtclient.NewRealtimeClient(ctx, ec, DefaultL2NetworkRealtimeURL)
	require.NoError(t, err)

	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(DefaultL2AdminPrivateKey, "0x"))
	require.NoError(t, err)
	signer := types.MakeSigner(GetTestChainConfig(DefaultL2ChainID), big.NewInt(1), 0)

	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	require.True(t, ok)
	fromAddress := crypto.PubkeyToAddress(*publicKeyECDSA)
	fmt.Printf("Sender: %s\n", fromAddress)

	erc20ABI, err := abi.JSON(strings.NewReader(erc20ABIJson))
	require.NoError(t, err)

	// Deploy the contract
	erc20Address := deployERC20Contract(t, ctx, privateKey, client)

	// Get the sender's nonce
	nonce, err := client.RealtimeGetTransactionCount(ctx, fromAddress)
	require.NoError(t, err)

	for i := int64(0); i < 10; i++ {
		// Transfer erc20 tokens amount
		amount := new(big.Int).Mul(big.NewInt(1), big.NewInt(1e18)) // Adjust for token decimals (18 in this case)
		// Prepare transfer data
		recevier := common.HexToAddress(fmt.Sprintf("0x000000000000000000000000000000000010%04x", i))
		data, err := erc20ABI.Pack("transfer", recevier, amount)
		require.NoError(t, err)

		gasPrice, err := client.SuggestGasPrice(ctx)
		require.NoError(t, err)
		transferERC20TokenTx := types.NewTransaction(
			nonce+uint64(i),
			erc20Address,
			big.NewInt(0),
			60000,
			gasPrice,
			data,
		)

		signedTx, err := types.SignTx(transferERC20TokenTx, signer, privateKey)
		require.NoError(t, err)
		err = client.SendTransaction(ctx, signedTx)
		require.NoError(t, err)
		err = WaitTxToBeMined(ctx, client, signedTx, DefaultTimeoutTxToBeMined)
		require.NoError(t, err)
		receipt, err := client.RealtimeGetTransactionReceipt(ctx, signedTx.Hash())
		require.NoError(t, err)
		require.NotNil(t, receipt)
		fmt.Printf("receipt: %+v\n", receipt)
	}

	// Dump state cache for further checking
	err = client.RealtimeDumpCache(ctx)
	require.NoError(t, err)
	// compareCacheWithSequenceDB(t, DefaultSequncerDBPath, DefaultStateCachePath)
}

func compareCacheWithSequenceDB(t *testing.T, dbDir, cacheDir string) {
	// Cache Files list
	cacheFiles := map[string]string{
		"account_cache.json": "",
		"storage_cache.json": "",
		"code_cache.json":    "",
	}

	for fileName := range cacheFiles {
		filePath := filepath.Join(cacheDir, fileName)
		_, err := os.Stat(filePath)
		require.NoError(t, err)
		data, err := ioutil.ReadFile(filePath)
		require.NoError(t, err)

		cacheFiles[fileName] = string(data)
	}

	tempDbDir, err := ioutil.TempDir("", "db_copy")
	require.NoError(t, err, "Failed to create temp db dir")
	defer os.RemoveAll(tempDbDir)

	cmd := exec.Command("cp", "-r", dbDir, tempDbDir)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Failed to copy db dir with cp -r: %s, output: %s", dbDir, string(output))
	copiedDbDir := filepath.Join(tempDbDir, filepath.Base(dbDir))

	// Create a node stack to access the database
	stack, err := node.New(&node.Config{
		DataDir: copiedDbDir,
	})
	require.NoError(t, err)
	defer stack.Close()

	db, err := stack.OpenDatabaseWithFreezer("chaindata", 0, 0, "", "eth/db/chaindata/", false)
	require.NoError(t, err)
	defer db.Close()

	// Get the latest state root
	headHeader := rawdb.ReadHeadHeader(db)
	require.NotNil(t, headHeader, "Head header should not be nil")

	// Create a trie database and state database
	trieDB := triedb.NewDatabase(db, nil)
	stateDB := state.NewDatabase(trieDB, nil)

	// Compare account data
	if cacheFiles["account_cache.json"] != "" {
		var accountCache map[string]string
		err := json.Unmarshal([]byte(cacheFiles["account_cache.json"]), &accountCache)
		require.NoError(t, err)

		// Get a state reader
		stateReader, err := stateDB.Reader(headHeader.Root)
		require.NoError(t, err)

		for k, v := range accountCache {
			// The key is the account address as hex string
			addrBytes, err := hex.DecodeString(k)
			require.NoError(t, err)
			address := common.BytesToAddress(addrBytes)

			// Get the account from the state
			account, err := stateReader.Account(address)
			require.NoError(t, err)

			// Decode the cache value
			vBytes, _ := hex.DecodeString(v)
			var cacheAccount types.StateAccount
			err = rlp.DecodeBytes(vBytes, &cacheAccount)
			require.NoError(t, err)

			// Compare account data
			require.Equal(t, cacheAccount.Nonce, account.Nonce, "Nonce mismatch for account %s, from cache: %d, from db: %d", k, cacheAccount.Nonce, account.Nonce)
			require.Equal(t, cacheAccount.Balance, account.Balance, "Balance mismatch for account %s, from cache: %s, from db: %s", k, cacheAccount.Balance.String(), account.Balance.String())
			require.Equal(t, cacheAccount.Root, types.EmptyRootHash, "Root mismatch for account %s should be empty roothash, from cache: %s", k, cacheAccount.Root.Hex())
			require.Equal(t, cacheAccount.CodeHash, account.CodeHash, "CodeHash mismatch for account %s, from cache: %s, from db: %s", k, hex.EncodeToString(cacheAccount.CodeHash), hex.EncodeToString(account.CodeHash))
		}
	}

	// Compare storage data
	if cacheFiles["storage_cache.json"] != "" {
		var storageCache map[string]string
		err := json.Unmarshal([]byte(cacheFiles["storage_cache.json"]), &storageCache)
		require.NoError(t, err)

		// Get a state reader
		stateReader, err := stateDB.Reader(headHeader.Root)
		require.NoError(t, err)

		for k, v := range storageCache {
			// The key format is "address(20 bytes) + storageKey(32 bytes)" as hex string
			keyBytes, err := hex.DecodeString(k)
			require.NoError(t, err)

			// Parse the composite key to get address and storage key
			if len(keyBytes) != 20+32 {
				continue // Skip invalid keys
			}
			address := common.BytesToAddress(keyBytes[:20])
			storageKey := common.BytesToHash(keyBytes[20:52])

			// Get the storage value from the state
			storageValue, err := stateReader.Storage(address, storageKey)
			require.NoError(t, err)

			// Compare storage values
			expectedValue, _ := hex.DecodeString(v)
			require.Equal(t, expectedValue, storageValue.Bytes(), "Storage mismatch for key %s, from cache: %s, from db: %s", k, v, storageValue.Hex())
		}
	}

	// Compare code data
	if cacheFiles["code_cache.json"] != "" {
		var codeCache map[string]string
		err := json.Unmarshal([]byte(cacheFiles["code_cache.json"]), &codeCache)
		require.NoError(t, err)

		for k, v := range codeCache {
			// The key is the code hash as hex string
			codeHashBytes, err := hex.DecodeString(k)
			require.NoError(t, err)
			codeHash := common.BytesToHash(codeHashBytes)

			// Get the code from the database using the code hash
			code := rawdb.ReadCodeWithPrefix(db, codeHash)
			require.NotNil(t, code, "Code should not be nil for hash %s", codeHash.Hex())

			// Compare code data
			expectedCode, _ := hex.DecodeString(v)
			require.Equal(t, expectedCode, code, "Code mismatch for key %s, from cache: %s, from db: %s", k, v, hex.EncodeToString(code))
		}
	}
}
