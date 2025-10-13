package test

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/realtime/rtclient"
	"github.com/stretchr/testify/require"
)

var (
	erc20ABI, _            = abi.JSON(strings.NewReader(erc20ABIJson))
	factoryABI, _          = abi.JSON(strings.NewReader(factoryABIJson))
	destroyABI, _          = abi.JSON(strings.NewReader(destroyABIJson))
	createDestroyABI, _    = abi.JSON(strings.NewReader(createDestroyABIJson))
	precompileCallerABI, _ = abi.JSON(strings.NewReader(precompileCallerABIJson))
)

// setupRealtimeTestEnvironment creates a test environment with necessary data for tests
func setupRealtimeTestEnvironment(t *testing.T, ctx context.Context, client *rtclient.RealtimeClient) uint64 {
	// Wait for at least one block to be available
	var blockNumber uint64
	var err error
	for i := 0; i < 30; i++ {
		blockNumber, err = client.RealtimeBlockNumber(ctx, "latest")
		require.NoError(t, err)
		fmt.Printf("Realtime block number: %d, attempt: %v\n", blockNumber, i)
		if blockNumber > 0 {
			break
		}
		time.Sleep(1 * time.Second)
	}
	require.Greater(t, blockNumber, uint64(0), "Realtime block number should be greater than 0")
	return blockNumber
}

// GetAuth configures and returns an auth object.
func GetAuth(privateKeyStr string, chainID uint64) (*bind.TransactOpts, error) {
	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(privateKeyStr, "0x"))
	if err != nil {
		return nil, err
	}

	return bind.NewKeyedTransactorWithChainID(privateKey, big.NewInt(0).SetUint64(chainID))
}

func GetTestChainConfig(chainID uint64) *params.ChainConfig {
	return &params.ChainConfig{
		ChainID:                 big.NewInt(int64(chainID)),
		HomesteadBlock:          big.NewInt(0),
		DAOForkBlock:            nil,
		DAOForkSupport:          false,
		EIP150Block:             big.NewInt(0),
		EIP155Block:             big.NewInt(0),
		EIP158Block:             big.NewInt(0),
		ByzantiumBlock:          big.NewInt(0),
		ConstantinopleBlock:     big.NewInt(0),
		PetersburgBlock:         big.NewInt(0),
		IstanbulBlock:           big.NewInt(0),
		MuirGlacierBlock:        big.NewInt(0),
		BerlinBlock:             big.NewInt(0),
		LondonBlock:             big.NewInt(0),
		ArrowGlacierBlock:       big.NewInt(0),
		GrayGlacierBlock:        big.NewInt(0),
		MergeNetsplitBlock:      nil,
		ShanghaiTime:            nil,
		CancunTime:              nil,
		PragueTime:              nil,
		OsakaTime:               nil,
		VerkleTime:              nil,
		TerminalTotalDifficulty: big.NewInt(math.MaxInt64),
		Ethash:                  new(params.EthashConfig),
		Clique:                  nil,
	}
}

func nativeTransferTx(t *testing.T, ctx context.Context, client *rtclient.RealtimeClient, amount *big.Int, toAddress string) *types.Transaction {
	chainID, err := client.ChainID(ctx)
	require.NoError(t, err)
	auth, err := GetAuth(DefaultL2AdminPrivateKey, chainID.Uint64())
	require.NoError(t, err)
	nonce, err := client.RealtimeGetTransactionCount(ctx, auth.From)
	require.NoError(t, err)
	gasPrice, err := client.SuggestGasPrice(ctx)
	require.NoError(t, err)

	to := common.HexToAddress(toAddress)
	gas, err := client.EstimateGas(ctx, ethereum.CallMsg{
		From:  auth.From,
		To:    &to,
		Value: amount,
	})
	require.NoError(t, err)

	tx := types.NewTransaction(
		nonce,
		to,
		amount,
		gas,
		gasPrice,
		nil,
	)

	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(DefaultL2AdminPrivateKey, "0x"))
	require.NoError(t, err)

	signer := types.MakeSigner(GetTestChainConfig(DefaultL2ChainID), big.NewInt(1), 0)
	signedTx, err := types.SignTx(tx, signer, privateKey)
	require.NoError(t, err)

	err = client.SendTransaction(ctx, signedTx)
	require.NoError(t, err)

	return signedTx
}

func deployERC20Contract(
	t *testing.T,
	ctx context.Context,
	privateKey *ecdsa.PrivateKey,
	client *rtclient.RealtimeClient,
) common.Address {
	publicKey := privateKey.Public()
	tmpPublicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	require.True(t, ok)
	fromAddress := crypto.PubkeyToAddress(*tmpPublicKeyECDSA)
	fmt.Printf("Sender: %s\n", fromAddress)

	nonce, err := client.RealtimeGetTransactionCount(ctx, fromAddress)
	require.NoError(t, err)

	// Define gas parameters
	gasPrice, err := client.SuggestGasPrice(ctx)
	require.NoError(t, err)

	// Set up transaction options
	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, big.NewInt(int64(DefaultL2ChainID)))
	require.NoError(t, err)

	auth.Nonce = big.NewInt(int64(nonce))
	auth.Value = big.NewInt(0)
	auth.GasLimit = uint64(3000000)
	auth.GasPrice = gasPrice

	// Deploy the contract
	erc20Bytecode, err := hex.DecodeString(erc20BytecodeStr)
	require.NoError(t, err)
	erc20Address, tx, _, err := bind.DeployContract(auth, erc20ABI, erc20Bytecode, client)
	require.NoError(t, err)

	// Wait for contract deployment to be mined
	err = WaitTxToBeMined(ctx, client, tx, DefaultTimeoutTxToBeMined)
	require.NoError(t, err)
	fmt.Printf("ERC20 Contract deployed at: %s, transaction hash: %s\n", erc20Address.Hex(), tx.Hash().Hex())

	return erc20Address
}

func erc20TransferTx(
	t *testing.T,
	ctx context.Context,
	privateKey *ecdsa.PrivateKey,
	client *rtclient.RealtimeClient,
	amount *big.Int,
	gasPrice *big.Int,
	toAddress common.Address,
	erc20Address common.Address,
	nonce uint64,
) *types.Transaction {

	// Prepare transfer data
	data, err := erc20ABI.Pack("transfer", toAddress, amount)
	require.NoError(t, err)

	if gasPrice == nil {
		gasPrice, err = client.SuggestGasPrice(ctx)
		require.NoError(t, err)
	}

	transferERC20TokenTx := types.NewTransaction(
		nonce,
		erc20Address,
		big.NewInt(0),
		60000,
		gasPrice,
		data,
	)

	signer := types.MakeSigner(GetTestChainConfig(DefaultL2ChainID), big.NewInt(1), 0)
	signedTx, err := types.SignTx(transferERC20TokenTx, signer, privateKey)
	require.NoError(t, err)

	err = client.SendTransaction(ctx, signedTx)
	require.NoError(t, err)

	return signedTx
}

func transErc20TokenBatch(t *testing.T, ctx context.Context, client *rtclient.RealtimeClient, nonRtClient ethClienter, erc20Address common.Address, amount *big.Int, toAddress string, batchSize int) ([]string, uint64, common.Hash) {
	var txHashes []string
	var transactions []*types.Transaction
	gasPrice, err := client.SuggestGasPrice(ctx)
	require.NoError(t, err)
	privKey, err := crypto.HexToECDSA(strings.TrimPrefix(DefaultL2AdminPrivateKey, "0x"))
	require.NoError(t, err)
	startNonce, err := client.RealtimeGetTransactionCount(ctx, common.HexToAddress(DefaultL2AdminAddress))
	require.NoError(t, err)
	to := common.HexToAddress(toAddress)

	for i := 1; i < batchSize; i++ {
		tx := erc20TransferTx(t, ctx, privKey, client, amount, gasPrice, to, erc20Address, startNonce+uint64(i))
		txHashes = append(txHashes, tx.Hash().String())
		transactions = append(transactions, tx)
	}

	tx := erc20TransferTx(t, ctx, privKey, client, amount, gasPrice, to, erc20Address, startNonce)
	txHashes = append(txHashes, tx.Hash().String())
	transactions = append(transactions, tx)

	// Wait for all transactions to be mined
	for _, tx := range transactions {
		err := WaitTxToBeMined(ctx, client, tx, DefaultTimeoutTxToBeMined)
		require.NoError(t, err)
		if nonRtClient != nil {
			err = WaitEthTxToBeMined(ctx, nonRtClient, tx, DefaultTimeoutTxToBeMined)
			require.NoError(t, err)
		}
	}

	log.Info(fmt.Sprintf("All %d transactions have been mined successfully", len(transactions)))

	receipt, err := client.RealtimeGetTransactionReceipt(ctx, common.HexToHash(txHashes[len(txHashes)-1]))
	require.NoError(t, err)
	require.NotNil(t, receipt, "Transaction receipt should not be nil")
	return txHashes, receipt.BlockNumber.Uint64(), receipt.BlockHash
}

func transNativeToken(t *testing.T, ctx context.Context, client *rtclient.RealtimeClient, amount *big.Int, toAddress string) string {
	chainID, err := client.ChainID(ctx)
	require.NoError(t, err)
	auth, err := GetAuth(DefaultL2AdminPrivateKey, chainID.Uint64())
	require.NoError(t, err)
	nonce, err := client.RealtimeGetTransactionCount(ctx, auth.From)
	require.NoError(t, err)
	gasPrice, err := client.SuggestGasPrice(ctx)
	require.NoError(t, err)

	to := common.HexToAddress(toAddress)
	gas := uint64(21000)
	require.NoError(t, err)
	log.Info(fmt.Sprintf("gas: %d", gas))
	log.Info(fmt.Sprintf("gasPrice: %d", gasPrice))

	tx := types.NewTransaction(
		nonce,
		to,
		amount,
		gas,
		gasPrice,
		nil,
	)

	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(DefaultL2AdminPrivateKey, "0x"))
	require.NoError(t, err)

	signer := types.MakeSigner(GetTestChainConfig(DefaultL2ChainID), big.NewInt(1), 0)
	signedTx, err := types.SignTx(tx, signer, privateKey)
	require.NoError(t, err)

	err = client.SendTransaction(ctx, signedTx)
	require.NoError(t, err)

	err = WaitTxToBeMined(ctx, client, signedTx, DefaultTimeoutTxToBeMined)
	require.NoError(t, err)

	return signedTx.Hash().String()
}

func DeployFactoryContract(t *testing.T, ctx context.Context, client *rtclient.RealtimeClient) common.Address {
	// Deploy Factory contract
	chainID, err := client.ChainID(ctx)
	require.NoError(t, err)
	auth, err := GetAuth(DefaultL2AdminPrivateKey, chainID.Uint64())
	require.NoError(t, err)
	nonce, err := client.RealtimeGetTransactionCount(ctx, auth.From)
	require.NoError(t, err)
	gasPrice, err := client.SuggestGasPrice(ctx)
	require.NoError(t, err)

	auth.Nonce = big.NewInt(int64(nonce))
	auth.Value = big.NewInt(0)
	auth.GasLimit = uint64(3000000)
	auth.GasPrice = gasPrice

	factoryBytecode, err := hex.DecodeString(factoryBytecodeStr)
	require.NoError(t, err)
	factoryAddr, tx, _, err := bind.DeployContract(auth, factoryABI, factoryBytecode, client)
	require.NoError(t, err)

	fmt.Printf("Factory contract deployed at: %s, transaction hash: %s\n", factoryAddr.Hex(), tx.Hash().Hex())
	bind.WaitDeployed(ctx, client, tx)

	return factoryAddr
}

func SendDeployDestroyContractTx(t *testing.T, ctx context.Context, client *rtclient.RealtimeClient, privateKey *ecdsa.PrivateKey, factoryAddr common.Address, salt *big.Int) {
	destroyBytecode, err := hex.DecodeString(destroyBytecodeStr)
	require.NoError(t, err)
	data, err := factoryABI.Pack("deploy", destroyBytecode, salt)
	require.NoError(t, err)

	nonce, err := client.RealtimeGetTransactionCount(ctx, common.HexToAddress(DefaultL2AdminAddress))
	require.NoError(t, err)
	gasPrice, err := client.SuggestGasPrice(ctx)
	require.NoError(t, err)
	deployTx := types.NewTransaction(
		nonce,
		factoryAddr,
		big.NewInt(0),
		3000000,
		gasPrice,
		data,
	)

	signer := types.MakeSigner(GetTestChainConfig(DefaultL2ChainID), big.NewInt(1), 0)
	require.NoError(t, err)
	signedTx, err := types.SignTx(deployTx, signer, privateKey)
	require.NoError(t, err)
	err = client.SendTransaction(ctx, signedTx)
	require.NoError(t, err)
}

func SendDestroyContractTx(t *testing.T, ctx context.Context, client *rtclient.RealtimeClient, privateKey *ecdsa.PrivateKey, destroyAddr common.Address) {
	data, err := destroyABI.Pack("destroy", common.HexToAddress(DefaultL2AdminAddress))
	require.NoError(t, err)

	nonce, err := client.RealtimeGetTransactionCount(ctx, common.HexToAddress(DefaultL2AdminAddress))
	require.NoError(t, err)
	gasPrice, err := client.SuggestGasPrice(ctx)
	require.NoError(t, err)
	destroyTx := types.NewTransaction(
		nonce,
		destroyAddr,
		big.NewInt(0),
		3000000,
		gasPrice,
		data,
	)

	signer := types.MakeSigner(GetTestChainConfig(DefaultL2ChainID), big.NewInt(1), 0)
	require.NoError(t, err)
	signedTx, err := types.SignTx(destroyTx, signer, privateKey)
	require.NoError(t, err)
	err = client.SendTransaction(ctx, signedTx)
	require.NoError(t, err)
}

func GetContractAddressFromFactory(t *testing.T, ctx context.Context, client *rtclient.RealtimeClient, factoryAddr common.Address, salt *big.Int) common.Address {
	destroyBytecode, err := hex.DecodeString(destroyBytecodeStr)
	require.NoError(t, err)

	// Pack the call to computeAddress
	computeInput, err := factoryABI.Pack("computeAddress", destroyBytecode, salt)
	require.NoError(t, err)
	result, err := client.RealtimeCall(ctx, common.HexToAddress(DefaultL2AdminAddress), factoryAddr, "0x300000", "0x0", fmt.Sprintf("0x%x", computeInput))
	require.NoError(t, err)

	// Unpack the result
	return common.HexToAddress(result)
}

func DeployCreateDestroyContract(t *testing.T, ctx context.Context, client *rtclient.RealtimeClient) common.Address {
	// Deploy Factory contract
	chainID, err := client.ChainID(ctx)
	require.NoError(t, err)
	auth, err := GetAuth(DefaultL2AdminPrivateKey, chainID.Uint64())
	require.NoError(t, err)
	nonce, err := client.RealtimeGetTransactionCount(ctx, auth.From)
	require.NoError(t, err)
	gasPrice, err := client.SuggestGasPrice(ctx)
	require.NoError(t, err)

	auth.Nonce = big.NewInt(int64(nonce))
	auth.Value = big.NewInt(0)
	auth.GasLimit = uint64(3000000)
	auth.GasPrice = gasPrice

	createDestroyBytecode, err := hex.DecodeString(createDestroyBytecodeStr)
	require.NoError(t, err)
	createDestroyAddr, tx, _, err := bind.DeployContract(auth, createDestroyABI, createDestroyBytecode, client)
	require.NoError(t, err)

	fmt.Printf("Create Destroy contract deployed at: %s, transaction hash: %s\n", createDestroyAddr.Hex(), tx.Hash().Hex())
	bind.WaitDeployed(ctx, client, tx)

	return createDestroyAddr
}

func SendCreateAndDestroyTx(t *testing.T, ctx context.Context, client *rtclient.RealtimeClient, privateKey *ecdsa.PrivateKey, createDestroyAddr common.Address, salt *big.Int) {
	data, err := createDestroyABI.Pack("createAndDestroy", salt, common.HexToAddress(DefaultL2AdminAddress))
	require.NoError(t, err)

	nonce, err := client.RealtimeGetTransactionCount(ctx, common.HexToAddress(DefaultL2AdminAddress))
	require.NoError(t, err)
	gasPrice, err := client.SuggestGasPrice(ctx)
	require.NoError(t, err)
	createAndDestroyTx := types.NewTransaction(
		nonce,
		createDestroyAddr,
		big.NewInt(0),
		3000000,
		gasPrice,
		data,
	)

	signer := types.MakeSigner(GetTestChainConfig(DefaultL2ChainID), big.NewInt(1), 0)
	require.NoError(t, err)
	signedTx, err := types.SignTx(createAndDestroyTx, signer, privateKey)
	require.NoError(t, err)
	err = client.SendTransaction(ctx, signedTx)
	require.NoError(t, err)
}

func GetContractAddressFromCreateDestroy(t *testing.T, ctx context.Context, client *rtclient.RealtimeClient, createDestroyAddr common.Address, salt *big.Int) common.Address {
	destroyBytecode, err := hex.DecodeString(destroyBytecodeStr)
	require.NoError(t, err)

	// Pack the call to computeAddress
	computeInput, err := factoryABI.Pack("computeAddress", destroyBytecode, salt)
	require.NoError(t, err)
	result, err := client.RealtimeCall(ctx, common.HexToAddress(DefaultL2AdminAddress), createDestroyAddr, "0x300000", "0x0", fmt.Sprintf("0x%x", computeInput))
	require.NoError(t, err)

	// Unpack the result
	return common.HexToAddress(result)
}

func DeployPrecompileCallerContract(t *testing.T, ctx context.Context, client *rtclient.RealtimeClient) common.Address {
	chainID, err := client.ChainID(ctx)
	require.NoError(t, err)
	auth, err := GetAuth(DefaultL2AdminPrivateKey, chainID.Uint64())
	require.NoError(t, err)
	nonce, err := client.RealtimeGetTransactionCount(ctx, auth.From)
	require.NoError(t, err)
	gasPrice, err := client.SuggestGasPrice(ctx)
	require.NoError(t, err)

	auth.Nonce = big.NewInt(int64(nonce))
	auth.Value = big.NewInt(0)
	auth.GasLimit = uint64(3000000)
	auth.GasPrice = gasPrice

	precompileCallerBytecode, err := hex.DecodeString(precompileCallerBytecodeStr)
	require.NoError(t, err)
	precompileCallerAddr, tx, _, err := bind.DeployContract(auth, precompileCallerABI, precompileCallerBytecode, client)
	require.NoError(t, err)

	fmt.Printf("Precompile caller contract deployed at: %s, transaction hash: %s\n", precompileCallerAddr.Hex(), tx.Hash().Hex())
	bind.WaitDeployed(ctx, client, tx)

	return precompileCallerAddr
}

func SendCallPrecompileTx(t *testing.T, ctx context.Context, client *rtclient.RealtimeClient, privateKey *ecdsa.PrivateKey, precompileCallerAddr common.Address) *types.Transaction {
	data, err := precompileCallerABI.Pack("callPrecompile2")
	require.NoError(t, err)

	nonce, err := client.RealtimeGetTransactionCount(ctx, common.HexToAddress(DefaultL2AdminAddress))
	require.NoError(t, err)
	gasPrice, err := client.SuggestGasPrice(ctx)
	require.NoError(t, err)

	callPrecompileTx := types.NewTransaction(
		nonce,
		precompileCallerAddr,
		big.NewInt(0),
		3000000,
		gasPrice,
		data,
	)

	signer := types.MakeSigner(GetTestChainConfig(DefaultL2ChainID), big.NewInt(1), 0)
	signedTx, err := types.SignTx(callPrecompileTx, signer, privateKey)
	require.NoError(t, err)

	err = client.SendTransaction(ctx, signedTx)
	require.NoError(t, err)
	log.Info(fmt.Sprintf("signedTx: %s", signedTx.Hash().String()))

	return signedTx
}

// WaitMined waits for tx to be mined on the blockchain.
// It stops waiting when the context is canceled.
func WaitMined(ctx context.Context, b bind.DeployBackend, txHash common.Hash) (*types.Receipt, error) {
	queryTicker := time.NewTicker(time.Millisecond)
	defer queryTicker.Stop()

	for {
		receipt, err := b.TransactionReceipt(ctx, txHash)
		if err == nil {
			return receipt, nil
		}

		// Wait for the next round.
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-queryTicker.C:
		}
	}
}

type ethClienter interface {
	ethereum.TransactionReader
	ethereum.ChainStateReader
	ethereum.ContractCaller
	bind.DeployBackend
}

func GetErc20Balance(ctx context.Context, client ethClienter, addr common.Address, erc20Addr common.Address, height *big.Int) (*big.Int, error) {
	// Pack the balanceOf function call
	data, err := erc20ABI.Pack("balanceOf", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to pack balanceOf call: %v", err)
	}

	// Make the eth_call
	result, err := client.CallContract(ctx, ethereum.CallMsg{
		To:   &erc20Addr,
		Data: data,
	}, height)
	if err != nil {
		return nil, fmt.Errorf("failed to call contract: %v", err)
	}

	// Unpack the result
	var balance *big.Int
	err = erc20ABI.UnpackIntoInterface(&balance, "balanceOf", result)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack result: %v", err)
	}

	return balance, nil
}

// RevertReason returns the revert reason for a tx that has a receipt with failed status
func RevertReason(ctx context.Context, c ethClienter, tx *types.Transaction, blockNumber *big.Int) (string, error) {
	if tx == nil {
		return "", nil
	}

	signer := types.MakeSigner(GetTestChainConfig(DefaultL2ChainID), big.NewInt(1), 0)
	from, err := types.Sender(signer, tx)
	if err != nil {
		return "", err
	}

	msg := ethereum.CallMsg{
		From: from,
		To:   tx.To(),
		Gas:  tx.Gas(),

		Value: tx.Value(),
		Data:  tx.Data(),
	}
	hex, err := c.CallContract(ctx, msg, blockNumber)
	if err != nil {
		return "", err
	}

	unpackedMsg, err := abi.UnpackRevert(hex)
	if err != nil {
		fmt.Printf("failed to get the revert message for tx %v: %v\n", tx.Hash(), err)
		return "", errors.New("execution reverted")
	}

	return unpackedMsg, nil
}

// RevertReasonRealtime returns the revert reason for a tx that has a receipt with failed status
func RevertReasonRealtime(
	ctx context.Context,
	client *rtclient.RealtimeClient,
	tx *types.Transaction,
) (string, error) {
	if tx == nil {
		return "", nil
	}

	signer := types.MakeSigner(GetTestChainConfig(DefaultL2ChainID), big.NewInt(1), 0)
	from, err := types.Sender(signer, tx)
	if err != nil {
		return "", err
	}

	hex, err := client.RealtimeCall(ctx, from, *tx.To(), fmt.Sprintf("0x%x", tx.Gas()), fmt.Sprintf("0x%x", tx.Value()), fmt.Sprintf("0x%x", tx.Data()))
	if err != nil {
		return "", err
	}

	unpackedMsg, err := abi.UnpackRevert(common.FromHex(hex))
	if err != nil {
		fmt.Printf("failed to get the revert message for tx %v: %v\n", tx.Hash(), err)
		return "", errors.New("execution reverted")
	}

	return unpackedMsg, nil
}

// convertBlockParam converts string block parameters to uint64 for realtime client
func convertBlockParam(ctx context.Context, client *rtclient.RealtimeClient, blockParam string) (uint64, error) {
	switch blockParam {
	case "latest", "pending":
		return client.RealtimeBlockNumber(ctx, blockParam)
	case "earliest":
		return 0, nil
	default:
		if strings.HasPrefix(blockParam, "0x") {
			bigInt := new(big.Int)
			_, ok := bigInt.SetString(blockParam[2:], 16)
			if !ok {
				return 0, fmt.Errorf("invalid hex block number: %s", blockParam)
			}
			return bigInt.Uint64(), nil
		}
		return 0, fmt.Errorf("unsupported block parameter: %s", blockParam)
	}
}
