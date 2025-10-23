package main

import (
	"fmt"
	"math/big"
	"time"

	"github.com/ledgerwatch/erigon-lib/kv"

	"github.com/urfave/cli/v2"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/log"
)

type PreinstallAddresses struct {
	// Core utilities
	MultiCall3      common.Address
	Create2Deployer common.Address
	CreateX         common.Address

	// Safe ecosystem
	Safe_v130              common.Address
	SafeL2_v130            common.Address
	MultiSendCallOnly_v130 common.Address
	SafeSingletonFactory   common.Address
	MultiSend_v130         common.Address

	// Deployment tools
	DeterministicDeploymentProxy common.Address

	// Token standards
	Permit2 common.Address

	// Account Abstraction (ERC-4337)
	SenderCreator_v060 common.Address
	EntryPoint_v060    common.Address
	SenderCreator_v070 common.Address
	EntryPoint_v070    common.Address

	// Ethereum upgrades
	BeaconBlockRoots       common.Address
	BeaconBlockRootsSender common.Address
	HistoryStorage         common.Address
	HistoryStorageSender   common.Address
}

// GetPreinstallAddresses returns all preinstall contract addresses
func GetPreinstallAddresses() PreinstallAddresses {
	return PreinstallAddresses{
		// Core utilities
		MultiCall3:      common.HexToAddress("0xcA11bde05977b3631167028862bE2a173976CA11"),
		Create2Deployer: common.HexToAddress("0x13b0D85CcB8bf860b6b79AF3029fCA081AE9beF2"),
		CreateX:         common.HexToAddress("0xba5Ed099633D3B313e4D5F7bdc1305d3c28ba5Ed"),

		// Safe ecosystem
		Safe_v130:              common.HexToAddress("0x69f4D1788e39c87893C980c06EdF4b7f686e2938"),
		SafeL2_v130:            common.HexToAddress("0xfb1bffC9d739B8D520DaF37dF666da4C687191EA"),
		MultiSendCallOnly_v130: common.HexToAddress("0xA1dabEF33b3B82c7814B6D82A79e50F4AC44102B"),
		SafeSingletonFactory:   common.HexToAddress("0x914d7Fec6aaC8cd542e72Bca78B30650d45643d7"),
		MultiSend_v130:         common.HexToAddress("0x998739BFdAAdde7C933B942a68053933098f9EDa"),

		// Deployment tools
		DeterministicDeploymentProxy: common.HexToAddress("0x4e59b44847b379578588920cA78FbF26c0B4956C"),

		// Token standards
		Permit2: common.HexToAddress("0x000000000022D473030F116dDEE9F6B43aC78BA3"),

		// Account Abstraction (ERC-4337)
		SenderCreator_v060: common.HexToAddress("0x7fc98430eAEdbb6070B35B39D798725049088348"),
		EntryPoint_v060:    common.HexToAddress("0x5FF137D4b0FDCD49DcA30c7CF57E578a026d2789"),
		SenderCreator_v070: common.HexToAddress("0xEFC2c1444eBCC4Db75e7613d20C6a62fF67A167C"),
		EntryPoint_v070:    common.HexToAddress("0x0000000071727De22E5E9d8BAf0edAc6f37da032"),

		// Ethereum upgrades
		BeaconBlockRoots:       common.HexToAddress("0x000F3df6D732807Ef1319fB7B8bB8522d0Beac02"),
		BeaconBlockRootsSender: common.HexToAddress("0x0B799C86a49DEeb90402691F1041aa3AF2d3C875"),
		HistoryStorage:         common.HexToAddress("0x0000F90827F1C53a10cb7A02335B175320002935"),
		HistoryStorageSender:   common.HexToAddress("0x3462413Af4609098e1E27A490f554f260213D685"),
	}
}

func GetAllPreinstallAddresses() []common.Address {
	addrs := GetPreinstallAddresses()
	return []common.Address{
		addrs.MultiCall3,
		addrs.Create2Deployer,
		addrs.Safe_v130,
		addrs.SafeL2_v130,
		addrs.MultiSendCallOnly_v130,
		addrs.SafeSingletonFactory,
		addrs.DeterministicDeploymentProxy,
		addrs.MultiSend_v130,
		addrs.Permit2,
		addrs.SenderCreator_v060,
		addrs.EntryPoint_v060,
		addrs.SenderCreator_v070,
		addrs.EntryPoint_v070,
		addrs.CreateX,
		addrs.BeaconBlockRoots,
		addrs.BeaconBlockRootsSender,
		addrs.HistoryStorage,
		addrs.HistoryStorageSender,
	}
}

const (
	// PrecompileCount represents the number of precompile addresses
	// starting from `address(0)` to PrecompileCount that are funded
	// with a single wei in the genesis state.
	PrecompileCount = 256

	// PredeployCount is the number of predeploy-namespace addresses reserved for protocol usage
	PredeployCount = 2048
	// PredeployPrefix is the base address for all predeploys
	// 0x420 << 148 = 0x4200000000000000000000000000000000000000
	PredeployPrefix     = "0x4200000000000000000000000000000000000000"
	CodeNAmeSpacePrefix = "0xC0D3C0d3C0d3C0D3c0d3C0d3c0D3C0d3c0d30000"

	PreinstallCount = 18

	IgnoredErigonScalableAddress = "0x000000000000000000000000000000005ca1ab1e"
)

var OP_PREDEPLOY [PredeployCount]common.Address
var OP_PREDEPLOY_SHIFTED [PredeployCount]common.Address

var OP_PRECOMPILE [PrecompileCount]common.Address
var OP_PREINSTALL [PrecompileCount]common.Address

func generatePredeployAddress(index uint64) string {
	prefix, _ := new(big.Int).SetString(PredeployPrefix, 0)
	address := new(big.Int).Add(prefix, big.NewInt(int64(index)))
	return fmt.Sprintf("0x%040x", address)
}

func PredeployToCodeNamespace(addr common.Address) common.Address {

	var out common.Address
	prefix := common.HexToAddress(CodeNAmeSpacePrefix)
	copy(out[:], prefix[:])
	out[18] = addr[18]
	out[19] = addr[19]
	return out
}

func init() {
	for i := 0; i < PrecompileCount; i++ {
		addr := common.BytesToAddress([]byte{byte(i)})
		OP_PRECOMPILE[i] = addr
	}

	preInstallAddrs := GetAllPreinstallAddresses()
	for i := 0; i < PreinstallCount; i++ {
		OP_PREINSTALL[i] = preInstallAddrs[i]
	}

	for i := 0; i < PredeployCount; i++ {
		OP_PREDEPLOY[i] = common.HexToAddress(generatePredeployAddress(uint64(i)))
		OP_PREDEPLOY_SHIFTED[i] = PredeployToCodeNamespace(OP_PREDEPLOY[i])
	}

}

func verifyMigrateGenesis(ctx *cli.Context) error {

	start := time.Now()

	chainDataPath := ctx.String("chaindata")
	if chainDataPath == "" {
		return fmt.Errorf("migration path is required")
	}

	isStandaloneSMT := ctx.Bool("standalone-smt")
	kv.InitStandaloneSMT(isStandaloneSMT)
	erigonAlloc, err := core.LoadErigonGenesisData(chainDataPath)

	opProtocolAddress := GetOpProtocolAddress()

	skippedAddresses := make([]common.Address, 0, len(opProtocolAddress)+1)
	skippedAddresses = append(skippedAddresses, opProtocolAddress...)
	skippedAddresses = append(skippedAddresses, common.HexToAddress(IgnoredErigonScalableAddress))
	err = verifyGenesisInternal(ctx, erigonAlloc, 0, skippedAddresses)

	if err != nil {
		return fmt.Errorf("verification failed, %v", err)
	} else {
		log.Info("verification success", "elapsed", time.Since(start))
		return nil
	}

}

func GetOpProtocolAddress() []common.Address {
	protocolAddresses := make([]common.Address, 0, PredeployCount*2+PrecompileCount+PreinstallCount)

	for _, addr := range OP_PREDEPLOY {
		protocolAddresses = append(protocolAddresses, addr)
	}

	for _, addr := range OP_PREDEPLOY_SHIFTED {
		protocolAddresses = append(protocolAddresses, addr)
	}

	for _, addr := range OP_PRECOMPILE {
		protocolAddresses = append(protocolAddresses, addr)
	}

	for _, addr := range OP_PREINSTALL {
		protocolAddresses = append(protocolAddresses, addr)
	}

	return protocolAddresses
}
