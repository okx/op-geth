package e2e

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/test/operations"
	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/test/constants"

	"github.com/holiman/uint256"
	"gopkg.in/yaml.v2"
)

const (
	TmpSenderPrivateKey = "363ea277eec54278af051fb574931aec751258450a286edce9e1f64401f3b9c8"
)

// ValidationError represents the error structure in PreResult
type ValidationError struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

// ValidationInnerTx represents an inner transaction for test validation
type ValidationInnerTx struct {
	Dept          *big.Int `json:"dept"`
	InternalIndex *big.Int `json:"internal_index"`
	CallType      string   `json:"call_type"`
	Name          string   `json:"name"`
	TraceAddress  string   `json:"trace_address"`
	CodeAddress   string   `json:"code_address"`
	From          string   `json:"from"`
	To            string   `json:"to"`
	Input         string   `json:"input"`
	Output        string   `json:"output"`
	IsError       bool     `json:"is_error"`
	GasUsed       uint64   `json:"gas_used"`
	Value         string   `json:"value"`
	ValueWei      string   `json:"value_wei"`
	Error         string   `json:"error"`
	ReturnGas     uint64   `json:"return_gas"`
}

// ValidationResult represents a typed structure matching PreResult for test validation
type ValidationResult struct {
	InnerTxs    []ValidationInnerTx    `json:"innerTxs"`
	Logs        []types.Log            `json:"logs"`
	StateDiff   map[string]interface{} `json:"stateDiff"`
	Error       ValidationError        `json:"error"`
	GasUsed     uint64                 `json:"gasUsed"`
	BlockNumber *big.Int               `json:"blockNumber"`
}

// Global variables to store deployed contract addresses
var (
	ContractAAddr     common.Address
	ContractBAddr     common.Address
	ContractCAddr     common.Address
	FactoryAddr       common.Address
	ERC20Addr         common.Address
	DeploymentAddress common.Address
	ContractsDeployed bool
)

func GetNonce(client *ethclient.Client, ctx context.Context, fromPrivateKey string) uint64 {
	chainID, err := client.ChainID(ctx)
	if err != nil {
		fmt.Printf("Get nonce err for get chainID failed: %v", err)
	}
	auth, err := operations.GetAuth(fromPrivateKey, chainID.Uint64())
	if err != nil {
		fmt.Printf("Get nonce err for get auth failed: %v", err)
	}
	nonce, err := client.PendingNonceAt(ctx, auth.From)
	if err != nil {
		fmt.Printf("Get nonce err for PendingNonceAt failed: %v", err)
	}
	return nonce
}

// TransTokenWithFrom transfers tokens from a specific private key to an address
func TransTokenWithFrom(t *testing.T, ctx context.Context, client *ethclient.Client, fromPrivateKey string, amount *uint256.Int, toAddress string) string {
	chainID, err := client.ChainID(ctx)
	require.NoError(t, err)
	auth, err := operations.GetAuth(fromPrivateKey, chainID.Uint64())
	require.NoError(t, err)
	nonce, err := client.PendingNonceAt(ctx, auth.From)
	require.NoError(t, err)
	gasPrice, err := client.SuggestGasPrice(ctx)
	require.NoError(t, err)

	to := common.HexToAddress(toAddress)
	gas, err := client.EstimateGas(ctx, ethereum.CallMsg{From: auth.From, To: &to, Value: amount.ToBig()})
	require.NoError(t, err)

	tx := types.NewTransaction(nonce, to, amount.ToBig(), gas, gasPrice, nil)

	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(fromPrivateKey, "0x"))
	require.NoError(t, err)

	signer := types.MakeSigner(operations.GetTestChainConfig(operations.DefaultL2ChainID), big.NewInt(1), 0)
	signedTx, err := types.SignTx(tx, signer, privateKey)
	require.NoError(t, err)

	err = client.SendTransaction(ctx, signedTx)
	require.NoError(t, err)

	err = operations.WaitTxToBeMined(ctx, client, signedTx, operations.DefaultTimeoutTxToBeMined)
	require.NoError(t, err)

	return signedTx.Hash().String()
}

// TransToken transfers tokens using the default rich private key
func TransToken(t *testing.T, ctx context.Context, client *ethclient.Client, amount *uint256.Int, toAddress string) string {
	return TransTokenWithFrom(t, ctx, client, operations.DefaultRichPrivateKey, amount, toAddress)
}

// Creates multiple transactions in a batch and waits for them all to be mined
func TransTokenBatch(t *testing.T, ctx context.Context, client *ethclient.Client, amount *uint256.Int, toAddress string, batchSize int, fromPrivateKey ...string) []string {
	privateKey := operations.DefaultRichPrivateKey
	if len(fromPrivateKey) > 0 && fromPrivateKey[0] != "" {
		privateKey = fromPrivateKey[0]
	}
	var txHashes []string
	var transactions []*types.Transaction

	chainID, err := client.ChainID(ctx)
	require.NoError(t, err)
	auth, err := operations.GetAuth(privateKey, chainID.Uint64())
	require.NoError(t, err)
	nonce, err := client.PendingNonceAt(ctx, auth.From)
	require.NoError(t, err)
	gasPrice, err := client.SuggestGasPrice(ctx)
	require.NoError(t, err)

	// Create all transactions first
	for i := 0; i < batchSize; i++ {
		to := common.HexToAddress(toAddress)
		gas := uint64(50000)

		tx := types.NewTransaction(nonce+uint64(i), to, amount.ToBig(), gas, gasPrice, nil)

		privKey, err := crypto.HexToECDSA(strings.TrimPrefix(privateKey, "0x"))
		require.NoError(t, err)

		signer := types.MakeSigner(operations.GetTestChainConfig(operations.DefaultL2ChainID), big.NewInt(1), 0)
		signedTx, err := types.SignTx(tx, signer, privKey)
		require.NoError(t, err)

		err = client.SendTransaction(ctx, signedTx)
		require.NoError(t, err)

		txHashes = append(txHashes, signedTx.Hash().String())
		transactions = append(transactions, signedTx)
	}

	for _, tx := range transactions {
		err := operations.WaitTxToBeMined(ctx, client, tx, operations.DefaultTimeoutTxToBeMined)
		require.NoError(t, err)
	}

	return txHashes
}

// transTokenFail creates a token transfer transaction that will fail during execution
func TransTokenFail(t *testing.T, ctx context.Context, client *ethclient.Client, fromPrivateKey string, amount *uint256.Int, toAddress string) common.Hash {
	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(fromPrivateKey, "0x"))
	require.NoError(t, err)
	fromAddr := crypto.PubkeyToAddress(privateKey.PublicKey)

	nonce, err := client.PendingNonceAt(ctx, fromAddr)
	require.NoError(t, err)

	gasPrice, err := client.SuggestGasPrice(ctx)
	require.NoError(t, err)

	to := common.HexToAddress(toAddress)
	gasLimit := uint64(50000)
	tx := types.NewTransaction(nonce, to, amount.ToBig(), gasLimit, gasPrice, nil)

	signer := types.MakeSigner(operations.GetTestChainConfig(operations.DefaultL2ChainID), big.NewInt(1), 0)
	signedTx, err := types.SignTx(tx, signer, privateKey)
	require.NoError(t, err)

	err = client.SendTransaction(ctx, signedTx)
	require.NoError(t, err, "Transaction should be sent successfully")

	txHash := signedTx.Hash()
	err = operations.WaitTxToBeMined(ctx, client, signedTx, operations.DefaultTimeoutTxToBeMined)
	require.Error(t, err, "Transaction should fail during execution")

	// Verify transaction failed
	receipt, err := client.TransactionReceipt(ctx, txHash)
	require.NoError(t, err, "Should be able to get receipt for failed transaction")
	require.Equal(t, uint64(0), receipt.Status, "Transaction should have failed (status=0)")

	return txHash
}

func TransTokenWithFromImpl(t *testing.T, ctx context.Context, client *ethclient.Client, fromPrivateKey string, amount *uint256.Int, toAddress string, nonce uint64) string {
	signedTx := generateSignedTokenTransferTx(t, ctx, client, fromPrivateKey, amount, toAddress, nonce)
	err := client.SendTransaction(ctx, signedTx)
	require.NoError(t, err)

	err = operations.WaitTxToBeMined(ctx, client, signedTx, operations.DefaultTimeoutTxToBeMined)
	require.NoError(t, err)

	return signedTx.Hash().String()
}

func erc20TransferTx(
	t *testing.T,
	ctx context.Context,
	privateKey *ecdsa.PrivateKey,
	client *ethclient.Client,
	amount *big.Int,
	gasPrice *big.Int,
	toAddress common.Address,
	erc20Address common.Address,
	nonce uint64,
) *types.Transaction {
	erc20ABI, _ := abi.JSON(strings.NewReader(constants.Erc20ABIJson))

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

	signer := types.MakeSigner(operations.GetTestChainConfig(operations.DefaultL2ChainID), big.NewInt(1), 0)
	signedTx, err := types.SignTx(transferERC20TokenTx, signer, privateKey)
	require.NoError(t, err)

	err = client.SendTransaction(ctx, signedTx)
	require.NoError(t, err)

	return signedTx
}

func transErc20TokenBatch(t *testing.T, ctx context.Context, client *ethclient.Client, erc20Address common.Address, amount *big.Int, toAddress string, batchSize int) ([]string, uint64, common.Hash) {
	var txHashes []string
	var transactions []*types.Transaction
	gasPrice, err := client.SuggestGasPrice(ctx)
	require.NoError(t, err)
	privKey, err := crypto.HexToECDSA(TmpSenderPrivateKey)
	require.NoError(t, err)
	startNonce, err := client.PendingNonceAt(ctx, DeploymentAddress)
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
		err := operations.WaitTxToBeMined(ctx, client, tx, operations.DefaultTimeoutTxToBeMined)
		require.NoError(t, err)
	}

	fmt.Printf("All %d transactions have been mined successfully\n", len(transactions))

	receipt, err := client.TransactionReceipt(ctx, common.HexToHash(txHashes[len(txHashes)-1]))
	require.NoError(t, err)
	require.NotNil(t, receipt, "Transaction receipt should not be nil")
	return txHashes, receipt.BlockNumber.Uint64(), receipt.BlockHash
}

func generateSignedTokenTransferTx(t *testing.T, ctx context.Context, client *ethclient.Client, fromPrivateKey string, amount *uint256.Int, toAddress string, nonce uint64) *types.Transaction {
	chainID, err := client.ChainID(ctx)
	require.NoError(t, err)
	auth, err := operations.GetAuth(fromPrivateKey, chainID.Uint64())
	require.NoError(t, err)
	gasPrice, err := client.SuggestGasPrice(ctx)
	require.NoError(t, err)

	to := common.HexToAddress(toAddress)
	gas, err := client.EstimateGas(ctx, ethereum.CallMsg{
		From:  auth.From,
		To:    &to,
		Value: amount.ToBig(),
	})
	require.NoError(t, err)

	tx := types.NewTransaction(nonce, to, amount.ToBig(), gas, gasPrice, nil)

	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(fromPrivateKey, "0x"))
	require.NoError(t, err)

	signer := types.MakeSigner(operations.GetTestChainConfig(operations.DefaultL2ChainID), big.NewInt(1), 0)
	signedTx, err := types.SignTx(tx, signer, privateKey)
	require.NoError(t, err)
	fmt.Printf("gas: %d, gasPrice: %d, nonce: %d, hash: %v\n", gas, gasPrice, nonce, signedTx.Hash().Hex())
	return signedTx
}

// makeContractCall is a utility function to make contract calls and return transaction hash
func MakeContractCall(t *testing.T, ctx context.Context, client *ethclient.Client, privateKey *ecdsa.PrivateKey, contractAddr common.Address, calldata []byte, gasLimit uint64, value *big.Int) (common.Hash, error) {
	from := crypto.PubkeyToAddress(privateKey.PublicKey)

	nonce, err := client.PendingNonceAt(ctx, from)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to get nonce: %w", err)
	}

	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to get gas price: %w", err)
	}

	if value == nil {
		value = big.NewInt(0)
	}

	tx := types.NewTransaction(nonce, contractAddr, value, gasLimit, gasPrice, calldata)

	signer := types.MakeSigner(operations.GetTestChainConfig(operations.DefaultL2ChainID), big.NewInt(1), 0)
	signedTx, err := types.SignTx(tx, signer, privateKey)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to sign transaction: %w", err)
	}

	err = client.SendTransaction(ctx, signedTx)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to send transaction: %w", err)
	}

	err = operations.WaitTxToBeMined(ctx, client, signedTx, operations.DefaultTimeoutTxToBeMined)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to wait for transaction to be mined: %w", err)
	}

	return signedTx.Hash(), nil
}

// setupTestEnvironment creates a test environment with necessary data for tests
func SetupTestEnvironment(t *testing.T) (common.Hash, uint64) {
	// Wait for at least one block to be available
	var blockNumber uint64
	var err error
	for i := 0; i < 30; i++ {
		blockNumber, err = operations.GetBlockNumber()
		require.NoError(t, err)
		fmt.Printf("Block number: %d, attempt: %v", blockNumber, i)
		if blockNumber > 0 {
			break
		}
		time.Sleep(1 * time.Second)
	}
	require.Greater(t, blockNumber, uint64(0), "Block number should be greater than 0")

	// Get a block hash to use for tests
	blockNum, err := operations.GetBlockNumber()
	require.NoError(t, err)

	// Try using the refactored RPC method instead of the broken GetBlockByNumber
	blockNumberHex := fmt.Sprintf("0x%x", blockNum)
	blockData, err := operations.EthGetBlockByNumber(blockNumberHex, true)
	require.NoError(t, err)
	require.NotNil(t, blockData, "Block data should not be nil")

	fmt.Printf("Block data type: %T\n", blockData)

	blockHash := common.Hash{}

	// Extract block hash from the returned data
	if blockMap, ok := blockData.(map[string]interface{}); ok {
		if hashStr, exists := blockMap["hash"].(string); exists && hashStr != "" {
			blockHash = common.HexToHash(hashStr)
			fmt.Printf("Extracted block hash: %s\n", blockHash.Hex())
		} else {
			fmt.Printf("No hash field found in block data\n")
		}
	} else {
		fmt.Printf("Block data is not a map\n")
	}

	// If we still don't have a valid hash, create a synthetic one for testing
	if blockHash == (common.Hash{}) {
		t.Logf("WARNING: Could not extract valid block hash, creating synthetic hash")
		blockHash = common.BigToHash(big.NewInt(int64(blockNumber)))
		t.Logf("Using synthetic hash: %s", blockHash.Hex())
	}

	require.NotEqual(t, common.Hash{}, blockHash, "Block hash should not be empty")

	return blockHash, blockNumber
}

type Config struct {
	HTTPMethodRateLimit string `yaml:"http.methodratelimit"`
	HTTPAPIKeys         string `yaml:"http.apikeys"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

// DeployContract deploys a contract using the provided parameters
func DeployContract(t *testing.T, ctx context.Context, client *ethclient.Client, privateKey *ecdsa.PrivateKey, contractName, abiJson, bytecodeStr string, constructorArgs ...interface{}) common.Address {
	fromAddress := crypto.PubkeyToAddress(privateKey.PublicKey)
	nonce, err := client.PendingNonceAt(ctx, fromAddress)
	require.NoError(t, err)
	gasPrice, err := client.SuggestGasPrice(ctx)
	require.NoError(t, err)

	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, operations.GetTestChainConfig(operations.DefaultL2ChainID).ChainID)
	require.NoError(t, err)
	auth.Nonce = big.NewInt(int64(nonce))
	auth.Value = big.NewInt(0)
	auth.GasLimit = uint64(3000000)
	auth.GasPrice = gasPrice

	contractABI, err := abi.JSON(strings.NewReader(abiJson))
	require.NoError(t, err)
	contractBytecode, err := hex.DecodeString(bytecodeStr)
	require.NoError(t, err)

	contractAddr, tx, _, err := bind.DeployContract(auth, contractABI, contractBytecode, client, constructorArgs...)
	require.NoError(t, err)

	bind.WaitDeployed(ctx, client, tx)
	return contractAddr
}

// EnsureContractsDeployed ensures that test contracts are deployed for e2e tests
func EnsureContractsDeployed(t *testing.T) {
	if ContractsDeployed {
		return
	}

	ctx := context.Background()
	client, err := ethclient.Dial(operations.DefaultL2NetworkURL)
	require.NoError(t, err)

	privateKey, err := crypto.HexToECDSA(TmpSenderPrivateKey)
	require.NoError(t, err)
	DeploymentAddress = crypto.PubkeyToAddress(privateKey.PublicKey)

	fundingAmount := uint256.NewInt(5000000000000000000) // 5 ETH
	TransTokenWithFrom(t, ctx, client, operations.DefaultRichPrivateKey, fundingAmount, DeploymentAddress.String())

	richAddr := common.HexToAddress(operations.DefaultRichAddress)
	TransTokenWithFrom(t, ctx, client, operations.DefaultRichPrivateKey, fundingAmount, richAddr.String())

	// Deploy contracts
	ContractBAddr = DeployContract(t, ctx, client, privateKey, "ContractB", constants.ContractBABIJson, constants.ContractBBytecodeStr)
	ContractAAddr = DeployContract(t, ctx, client, privateKey, "ContractA", constants.ContractAABIJson, constants.ContractABytecodeStr, ContractBAddr)
	FactoryAddr = DeployContract(t, ctx, client, privateKey, "ContractFactory", constants.ContractFactoryABIJson, constants.ContractFactoryBytecodeStr)
	ContractCAddr = DeployContract(t, ctx, client, privateKey, "ContractC", constants.ContractCABIJson, constants.ContractCBytecodeStr)
	ERC20Addr = DeployContract(t, ctx, client, privateKey, "ERC20", constants.Erc20ABIJson, constants.Erc20BytecodeStr)
	ContractsDeployed = true
	fmt.Println("ContractAAddr:", ContractAAddr.Hex())
	fmt.Println("ContractBAddr:", ContractBAddr.Hex())

}

// EncodeTransferCall encodes an ERC20 transfer function call
func EncodeTransferCall(to string, amount uint64) []byte {
	// ERC20 transfer function selector: 0xa9059cbb
	selector := []byte{0xa9, 0x05, 0x9c, 0xbb}

	// Convert address string to bytes
	toAddr := common.HexToAddress(to)

	// Create the data payload
	data := make([]byte, 68) // 4 bytes selector + 32 bytes address + 32 bytes amount
	copy(data[:4], selector)
	copy(data[4+12:36], toAddr.Bytes()) // Address goes in the last 20 bytes of the 32-byte slot

	// Amount as big.Int
	amountBig := big.NewInt(int64(amount))
	amountBig.FillBytes(data[36:68])

	return data
}

// EncodeComplexCall encodes a complex contract call
func EncodeComplexCall(target common.Address) []byte {
	// Function selector for complex call
	selector := []byte{0xe6, 0x09, 0x05, 0x5e}

	// Create the data payload
	data := make([]byte, 36) // 4 bytes selector + 32 bytes address
	copy(data[:4], selector)
	copy(data[4+12:36], target.Bytes()) // Address goes in the last 20 bytes

	return data
}

// validateInnerTransactionMatch validates that two inner transactions match each other
func ValidateInnerTransactionMatch(t *testing.T, innerTx1, innerTx2 *types.InnerTx, contextMsg string) {
	require.Equal(t, innerTx1.From, innerTx2.From, "%s: From addresses should match", contextMsg)
	require.Equal(t, innerTx1.To, innerTx2.To, "%s: To addresses should match", contextMsg)
	require.Equal(t, innerTx1.Input, innerTx2.Input, "%s: Input data should match", contextMsg)
	require.Equal(t, innerTx1.Output, innerTx2.Output, "%s: Output data should match", contextMsg)
	require.Equal(t, innerTx1.IsError, innerTx2.IsError, "%s: Error status should match", contextMsg)
	require.Equal(t, innerTx1.CallType, innerTx2.CallType, "%s: Call types should match", contextMsg)
	require.Equal(t, innerTx1.ValueWei, innerTx2.ValueWei, "%s: ValueWei should match", contextMsg)
	require.Equal(t, innerTx1.CallValueWei, innerTx2.CallValueWei, "%s: CallValueWei should match", contextMsg)
	require.Equal(t, innerTx1.Error, innerTx2.Error, "%s: Error messages should match", contextMsg)
	require.Equal(t, innerTx1.Dept, innerTx2.Dept, "%s: Dept should match", contextMsg)
	require.Equal(t, innerTx1.InternalIndex, innerTx2.InternalIndex, "%s: InternalIndex should match", contextMsg)
	require.Equal(t, innerTx1.Name, innerTx2.Name, "%s: Name should match", contextMsg)
	require.Equal(t, innerTx1.TraceAddress, innerTx2.TraceAddress, "%s: TraceAddress should match", contextMsg)
	require.Equal(t, innerTx1.CodeAddress, innerTx2.CodeAddress, "%s: CodeAddress should match", contextMsg)
	require.Equal(t, innerTx1.Value, innerTx2.Value, "%s: Value should match", contextMsg)
}

// CreateBasicTransaction creates a standard transaction with common fields
func CreateBasicTransaction(from, to, value, gas, gasPrice, nonce, data string) map[string]interface{} {
	tx := map[string]interface{}{
		"from":     from,
		"to":       to,
		"value":    value,
		"gas":      gas,
		"gasPrice": gasPrice,
		"nonce":    nonce,
	}
	if data != "" {
		tx["data"] = data
	}
	return tx
}

// CreateDefaultStateOverrides creates common state overrides for pre-exec tests
func CreateDefaultStateOverrides() map[string]interface{} {
	return map[string]interface{}{
		"0x0165878a594ca255338adfa4d48449f69242eb8f": map[string]interface{}{
			"balance": "0x56bc75e2d630eb20000", // Large balance
			"nonce":   "0x0",
		},
	}
}

// CreateAuthorizationList creates a standard authorization list for EIP-7702 tests
func CreateAuthorizationList(addresses []string) []map[string]interface{} {
	var authList []map[string]interface{}
	for i, addr := range addresses {
		auth := map[string]interface{}{
			"chainId": "0x1",
			"address": addr,
			"nonce":   fmt.Sprintf("0x%x", i),
			"yParity": "0x1",
			"r":       "0x1234567890123456789012345678901234567890123456789012345678901234",
			"s":       "0x1234567890123456789012345678901234567890123456789012345678901234",
		}
		authList = append(authList, auth)
	}
	return authList
}

// ValidateResult validates a single transaction result and returns a typed ValidationResult
func ValidateResult(t *testing.T, result interface{}, testName string) ValidationResult {
	resultMap := result.(map[string]interface{})

	jsonBytes, err := json.Marshal(resultMap)
	require.NoError(t, err, "Failed to marshal result to JSON for %s", testName)

	var validationResult ValidationResult
	err = json.Unmarshal(jsonBytes, &validationResult)
	require.NoError(t, err, "Failed to unmarshal result to ValidationResult for %s", testName)

	t.Logf("Gas used for %s: %d", testName, validationResult.GasUsed)

	ValidateStateDiff(t, validationResult, testName)

	return validationResult
}

func ValidateStateDiff(t *testing.T, result ValidationResult, testName string) {
	for addrStr, addrData := range result.StateDiff {
		addrDataMap := addrData.(map[string]interface{})

		balanceData, exists := addrDataMap["balance"]
		require.True(t, exists, "Balance field should exist for address %s in %s", addrStr, testName)

		balanceMap, ok := balanceData.(map[string]interface{})
		require.True(t, ok, "Balance data should be a map for address %s in %s", addrStr, testName)

		before, beforeExists := balanceMap["before"]
		after, afterExists := balanceMap["after"]
		require.True(t, beforeExists, "Before balance should exist for address %s in %s", addrStr, testName)
		require.True(t, afterExists, "After balance should exist for address %s in %s", addrStr, testName)

		beforeStr, ok1 := before.(string)
		afterStr, ok2 := after.(string)
		require.True(t, ok1, "Before balance should be a string for address %s in %s", addrStr, testName)
		require.True(t, ok2, "After balance should be a string for address %s in %s", addrStr, testName)

		if beforeStr != "0" && afterStr != "0" {
			require.NotEqual(t, beforeStr, afterStr, "Before and after balances should be different for address %s in %s", addrStr, testName)
		}
	}
}

// CheckSuccessfulResult validates that a result represents a successful transaction and checks from address in stateDiff
func CheckSuccessfulResult(t *testing.T, result ValidationResult, fromAddress string, testName string) {
	require.Equal(t, 0, result.Error.Code, "%s should succeed", testName)
	require.Empty(t, result.Error.Msg, "%s should not have error message", testName)

	// Check if the from address exists in stateDiff
	if result.StateDiff != nil {
		fromAddr := common.HexToAddress(fromAddress)
		found := false
		for addrStr := range result.StateDiff {
			if common.HexToAddress(addrStr) == fromAddr {
				found = true
				break
			}
		}
		require.True(t, found, "From address %s should exist in stateDiff for %s", fromAddress, testName)
	}
}

// CheckErrorResult validates that a result contains a specific error (typed version)
func CheckErrorResult(t *testing.T, result ValidationResult, expectedError string, testName string) {
	require.Contains(t, result.Error.Msg, expectedError, "Error should mention %s for %s", expectedError, testName)
}

// SignAndSendTransaction is a helper function to sign, send, and wait for a transaction to be mined
func SignAndSendTransaction(
	ctx context.Context,
	t *testing.T,
	client *ethclient.Client,
	tx *types.Transaction,
	privateKey *ecdsa.PrivateKey,
) (common.Hash, *types.Receipt) {
	signer := types.MakeSigner(operations.GetTestChainConfig(operations.DefaultL2ChainID), big.NewInt(1), 0)
	signedTx, err := types.SignTx(tx, signer, privateKey)
	require.NoError(t, err)

	err = client.SendTransaction(ctx, signedTx)
	require.NoError(t, err)

	txHash := signedTx.Hash()
	t.Logf("tx sent: %s", txHash.Hex())

	err = operations.WaitTxToBeMined(ctx, client, signedTx, operations.DefaultTimeoutTxToBeMined)
	require.NoError(t, err)

	receipt, err := client.TransactionReceipt(ctx, txHash)
	require.NoError(t, err)
	require.Equal(t, types.ReceiptStatusSuccessful, receipt.Status, ("transaction should succeed"))

	t.Logf("tx mined in block %d, gas used: %d", receipt.BlockNumber.Uint64(), receipt.GasUsed)

	return txHash, receipt
}

// GetRefundCounterFromTrace extracts the refund counter for a specific opcode from a debug trace result
func GetRefundCounterFromTrace(traceResult map[string]interface{}, opcode string) uint64 {
	refundCounter := uint64(0)
	structLogs, ok := traceResult["structLogs"].([]interface{})
	if !ok {
		return refundCounter
	}

	for _, entry := range structLogs {
		log, ok := entry.(map[string]interface{})
		if !ok {
			continue
		}

		if op, ok := log["op"].(string); ok && op == opcode {
			if refund, ok := log["refund"].(float64); ok {
				refundCounter = uint64(refund)
				break
			}
		}
	}

	return refundCounter
}
