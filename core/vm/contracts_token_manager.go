package vm

import (
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
)

var CONFIG_CONTRACT_MANAGER_ADDRESS = common.HexToAddress("0x8f6923026C0B5408319498288Bf2E92105467b9f")
var TARGET_ADDRESS = common.HexToAddress("0x2a3DD3EB832aF982ec71669E178424b10Dca2EDe")
var TEST_OP_GAS uint64 = params.CallGasEIP150

type envConfig struct {
	envName                  string
	chainId                  uint64
	configContractMgrAddress string
	targetAddress            string
	testOpGas                uint64
}

var environments = []envConfig{
	{"mainnet", 195, "0x8f6923026C0B5408319498288Bf2E92105467b9f", "0x2a3DD3EB832aF982ec71669E178424b10Dca2EDe", params.CallGasEIP150},
	{"testnet2", 1952, "0xA93C0D985d69E3558814C61559e4fba01F6a1f9d", "0x528e26b25a34a4A5d0dbDa1d57D318153d2ED582", 0},
	{"local", 196, "0x76ca03a67C049477FfB09694dFeF00416dB69746", "0x4B24266C13AFEf2bb60e2C69A4C08A482d81e3CA", params.CallGasEIP150},
}

func InitEnvConfig(chainId uint64) {
	for _, env := range environments {
		if chainId == env.chainId {
			expectedTokenMgr := common.HexToAddress(env.configContractMgrAddress)
			expectedTarget := common.HexToAddress(env.targetAddress)
			TEST_OP_GAS = env.testOpGas

			CONFIG_CONTRACT_MANAGER_ADDRESS = expectedTokenMgr
			TARGET_ADDRESS = expectedTarget
			log.Info(fmt.Sprintf("Contract token manager for env:%s, chainId: %d, tokenManagerAddress: %s, targetAddress: %s, testOpGas: %d",
				env.envName, chainId, CONFIG_CONTRACT_MANAGER_ADDRESS, TARGET_ADDRESS, TEST_OP_GAS))
			return
		}
	}
	log.Warn(fmt.Sprintf("Unknown contract token manager from chainId: %d, will use default values: %s, %s, testOpGas: %d", chainId, CONFIG_CONTRACT_MANAGER_ADDRESS, TARGET_ADDRESS, TEST_OP_GAS))
}

// Operation codes for different token operations
const (
	TEST_OP   = 0x01 // Test precompile availability (no authentication required)
	BRIDGE_OP = 0x02 // Bridge tokens from L1
	CLEAN_OP  = 0x03 // Clean up tokens from target address
)

type tokenManagerPrecompile struct {
	evm     *EVM
	enabled bool
	caller  common.Address
}

func (c *tokenManagerPrecompile) RequiredGas(input []byte) uint64 {
	// Check if precompile is enabled
	if !c.enabled {
		return 0
	}

	if len(input) == 0 {
		// Empty input charges base gas fee
		return params.CallGasEIP150
	}

	operation := input[0]
	switch operation {
	case TEST_OP:
		// Use environment-specific gas fee for TEST_OP
		return TEST_OP_GAS
	case BRIDGE_OP:
		return params.SstoreSetGas
	case CLEAN_OP:
		return params.SstoreResetGas
	default:
		// Unknown operation charges base gas fee
		return params.CallGasEIP150
	}
}

func (c *tokenManagerPrecompile) Run(input []byte) ([]byte, error) {
	if !c.enabled {
		return []byte{}, ErrUnsupportedPrecompile
	}

	if len(input) == 0 {
		return []byte{}, errors.New("empty input")
	}

	operation := input[0]

	switch operation {
	case TEST_OP:
		return []byte("OK"), nil

	case BRIDGE_OP:
		if c.caller != CONFIG_CONTRACT_MANAGER_ADDRESS {
			return []byte{}, errors.New("unauthorized: only contract manager can call")
		}
		if len(input) <= 1 {
			return []byte{}, errors.New("missing bridge data")
		}
		return c.handleBridge(input[1:])

	case CLEAN_OP:
		if c.caller != CONFIG_CONTRACT_MANAGER_ADDRESS {
			return []byte{}, errors.New("unauthorized: only contract manager can call")
		}
		return c.handleCleanup()

	default:
		return []byte{}, errors.New("invalid operation")
	}
}

func (c *tokenManagerPrecompile) handleCleanup() ([]byte, error) {
	// Get current balance
	balance := c.evm.StateDB.GetBalance(TARGET_ADDRESS)

	one := uint256.NewInt(1)
	if balance.Cmp(one) <= 0 {
		return []byte{}, nil
	}

	// Keep 1 wei to maintain address existence in state trie
	// Prevent potential issues with zero-balance account deletion in some EVM implementations
	amountToClean := new(uint256.Int).Sub(balance, one)
	c.evm.StateDB.SubBalance(TARGET_ADDRESS, amountToClean, tracing.BalanceClean)
	return []byte{}, nil
}

func (c *tokenManagerPrecompile) handleBridge(data []byte) ([]byte, error) {
	if len(data) != 64 {
		return []byte{}, fmt.Errorf("invalid data length for bridge: expected 64 bytes, got %d bytes", len(data))
	}

	targetAddress := common.BytesToAddress(data[12:32])
	amount := new(uint256.Int).SetBytes(data[32:64])

	if amount.IsZero() {
		return []byte{}, errors.New("invalid amount")
	}

	c.evm.StateDB.AddBalance(targetAddress, amount, tracing.BalanceBridge)
	return []byte{}, nil
}

func (c *tokenManagerPrecompile) SetEVM(evm *EVM) {
	c.evm = evm
}
func (c *tokenManagerPrecompile) SetCaller(caller common.Address) {
	c.caller = caller
}
