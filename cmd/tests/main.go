package main

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
)

// GenesisConfig represents the structure of a genesis.json file
type GenesisConfig struct {
	Config     *params.ChainConfig `json:"config"`
	Nonce      hexutil.Uint64      `json:"nonce"`
	Timestamp  hexutil.Uint64      `json:"timestamp"`
	ExtraData  hexutil.Bytes       `json:"extraData"`
	GasLimit   hexutil.Uint64      `json:"gasLimit"`
	Difficulty *hexutil.Big        `json:"difficulty"`
	Mixhash    common.Hash         `json:"mixHash"`
	Coinbase   common.Address      `json:"coinbase"`
	Alloc      types.GenesisAlloc  `json:"alloc"`
	Number     hexutil.Uint64      `json:"number"`
	GasUsed    hexutil.Uint64      `json:"gasUsed"`
	ParentHash common.Hash         `json:"parentHash"`
	BaseFee    *hexutil.Big        `json:"baseFeePerGas"`
}

// RandomAccount represents a random account with storage
type RandomAccount struct {
	Balance *hexutil.Big                `json:"balance"`
	Nonce   hexutil.Uint64              `json:"nonce"`
	Code    hexutil.Bytes               `json:"code"`
	Storage map[common.Hash]common.Hash `json:"storage"`
}

func main() {
	// Generate random genesis
	genesis := generateRandomGenesis()

	// Write to file
	outputFile := "random_genesis.json"
	if len(os.Args) > 1 {
		outputFile = os.Args[1]
	}

	err := writeGenesisToFile(genesis, outputFile)
	if err != nil {
		fmt.Printf("Error writing genesis file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Random genesis.json generated: %s\n", outputFile)
	fmt.Printf("Accounts: %d\n", len(genesis.Alloc))
	fmt.Printf("Gas Limit: %d\n", genesis.GasLimit)
	fmt.Printf("Difficulty: %s\n", genesis.Difficulty.String())
}

func generateRandomGenesis() *core.Genesis {
	// Random number of accounts (10-1000)
	numAccounts := randomInt(5000, 5100)

	// Generate random accounts
	alloc := make(types.GenesisAlloc, numAccounts)

	for i := 0; i < numAccounts; i++ {
		addr := generateRandomAddress()
		account := generateRandomAccount()
		alloc[addr] = account
	}

	// Generate random chain config
	config := generateRandomChainConfig()

	// Generate random genesis parameters
	genesis := &core.Genesis{
		Config:     config,
		Nonce:      randomUint64(),
		Timestamp:  uint64(time.Now().Unix()),
		ExtraData:  generateRandomBytes(32),
		GasLimit:   randomUint64Range(8000000, 30000000),
		Difficulty: generateRandomBigInt(1000, 1000000),
		Mixhash:    generateRandomHash(),
		Coinbase:   generateRandomAddress(),
		Alloc:      alloc,
		Number:     0,
		GasUsed:    0,
		ParentHash: common.Hash{},
		BaseFee:    generateRandomBigInt(1000000000, 10000000000),
	}

	return genesis
}

func generateRandomAccount() types.Account {
	account := types.Account{
		Balance: generateRandomBigInt(10, 1000000000000000000), // Use *big.Int directly
		Nonce:   randomUint64Range(1, 100),                     // Use uint64 directly
		Code:    generateRandomCode(),
		Storage: generateRandomStorage(),
	}

	return account
}

func generateRandomCode() []byte {
	// 50% chance of having code (contract)
	if randomInt(0, 2) == 0 {
		return nil
	}

	// Generate random contract code (100-1000 bytes)
	codeSize := randomInt(100, 1000)
	return generateRandomBytes(codeSize)
}

func generateRandomStorage() map[common.Hash]common.Hash {
	storage := make(map[common.Hash]common.Hash)

	// Random number of storage slots (0-50)
	numSlots := randomInt(1000, 2000)

	for i := 0; i < numSlots; i++ {
		key := generateRandomHash()
		value := generateRandomHash()
		storage[key] = value
	}

	return storage
}

func generateRandomChainConfig() *params.ChainConfig {
	// Use mainnet config as base and randomize chain ID
	config := params.MainnetChainConfig
	config.ChainID = generateRandomBigInt(1, 1000000)

	// Generate fork times in correct chronological order
	shanghaiTime := generateRandomUint64Ptr(0, 40000)
	cancunTime := generateRandomUint64Ptr(*shanghaiTime, 50000)
	pragueTime := generateRandomUint64Ptr(*cancunTime, 60000)
	verkleTime := generateRandomUint64Ptr(*pragueTime, 70000)

	config.ShanghaiTime = shanghaiTime
	config.CancunTime = cancunTime
	config.PragueTime = pragueTime
	config.VerkleTime = verkleTime

	return config
}

// Helper functions for random generation

func generateRandomAddress() common.Address {
	var addr common.Address
	rand.Read(addr[:])
	return addr
}

func generateRandomHash() common.Hash {
	var hash common.Hash
	rand.Read(hash[:])
	return hash
}

func generateRandomBytes(size int) []byte {
	bytes := make([]byte, size)
	rand.Read(bytes)
	return bytes
}

func generateRandomBigInt(min, max int64) *big.Int {
	diff := max - min
	random := randomInt64(0, diff)
	return big.NewInt(min + random)
}

func randomInt(min, max int) int {
	return min + int(randomUint64()%uint64(max-min))
}

func randomInt64(min, max int64) int64 {
	return min + int64(randomUint64()%uint64(max-min))
}

func randomUint64() uint64 {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	var result uint64
	for i := 0; i < 8; i++ {
		result = result<<8 + uint64(bytes[i])
	}
	return result
}

func randomUint64Range(min, max uint64) uint64 {
	return min + randomUint64()%(max-min)
}

func generateRandomUint64Ptr(min, max uint64) *uint64 {
	val := randomUint64Range(min, max)
	return &val
}

func writeGenesisToFile(genesis *core.Genesis, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	return encoder.Encode(genesis)
}
