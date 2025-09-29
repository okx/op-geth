package apollo

import (
	"fmt"
	"math"
	"math/big"

	"github.com/apolloconfig/agollo/v4/storage"
	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/eth/gasprice"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"github.com/urfave/cli/v2"
)

// loadL2GasPricer loads the apollo l2gaspricer config cache on startup
func (c *Client) loadL2GasPricer(value interface{}) {
	ctx, _, err := c.getConfigContext(value)
	if err != nil {
		log.Error(fmt.Sprintf("load l2gaspricer from apollo config failed, err: %v", err))
	}

	// Load l2gaspricer config changes
	loadL2GasPricerConfig(ctx)
	log.Info(fmt.Sprintf("loaded l2gaspricer from apollo config: %+v", value.(string)))
}

// fireL2GasPricer fires the apollo l2gaspricer config change
func (c *Client) fireL2GasPricer(ctx *cli.Context, value *storage.ConfigChange) {
	loadL2GasPricerConfig(ctx)
	log.Info(fmt.Sprintf("apollo l2gaspricer old config : %+v", value.OldValue.(string)))
	log.Info(fmt.Sprintf("apollo l2gaspricer config changed: %+v", value.NewValue.(string)))
}

// loadL2GasPricerConfig loads the dynamic gas pricer apollo configurations
func loadL2GasPricerConfig(ctx *cli.Context) {
	UnsafeGetApolloConfig().Lock()
	defer UnsafeGetApolloConfig().Unlock()

	loadNodeL2GasPricerConfig(ctx, &UnsafeGetApolloConfig().NodeCfg)
	loadEthL2GasPricerConfig(ctx, &UnsafeGetApolloConfig().EthCfg)
}

// loadNodeL2GasPricerConfig loads the dynamic gas pricer apollo node configurations
func loadNodeL2GasPricerConfig(ctx *cli.Context, nodeCfg *node.Config) {
	// Load l2gaspricer config
}

// loadEthL2GasPricerConfig loads the dynamic gas pricer apollo eth configurations
func loadEthL2GasPricerConfig(ctx *cli.Context, ethCfg *ethconfig.Config) {
	// Load l2gaspricer config
	if ctx.IsSet(utils.EffectiveGasPriceEthTransfer.Name) {
		effectiveGasPriceForEthTransferVal := ctx.Float64(utils.EffectiveGasPriceEthTransfer.Name)
		effectiveGasPriceForEthTransferVal = math.Max(effectiveGasPriceForEthTransferVal, 0)
		effectiveGasPriceForEthTransferVal = math.Min(effectiveGasPriceForEthTransferVal, 1)
		ethCfg.XLayer.L2GasPricer.EffectiveGasPriceEthTransfer = float64(math.Round(effectiveGasPriceForEthTransferVal * 255.0))
	}
	if ctx.IsSet(utils.EffectiveGasPriceERC20Transfer.Name) {
		effectiveGasPriceForErc20TransferVal := ctx.Float64(utils.EffectiveGasPriceERC20Transfer.Name)
		effectiveGasPriceForErc20TransferVal = math.Max(effectiveGasPriceForErc20TransferVal, 0)
		effectiveGasPriceForErc20TransferVal = math.Min(effectiveGasPriceForErc20TransferVal, 1)
		ethCfg.XLayer.L2GasPricer.EffectiveGasPriceERC20Transfer = float64(math.Round(effectiveGasPriceForErc20TransferVal * 255.0))
	}
	if ctx.IsSet(utils.EffectiveGasPriceContractInvocation.Name) {
		effectiveGasPriceForContractInvocationVal := ctx.Float64(utils.EffectiveGasPriceContractInvocation.Name)
		effectiveGasPriceForContractInvocationVal = math.Max(effectiveGasPriceForContractInvocationVal, 0)
		effectiveGasPriceForContractInvocationVal = math.Min(effectiveGasPriceForContractInvocationVal, 1)
		ethCfg.XLayer.L2GasPricer.EffectiveGasPriceContractInvocation = float64(math.Round(effectiveGasPriceForContractInvocationVal * 255.0))
	}
	if ctx.IsSet(utils.EffectiveGasPriceContractDeployment.Name) {
		effectiveGasPriceForContractDeploymentVal := ctx.Float64(utils.EffectiveGasPriceContractDeployment.Name)
		effectiveGasPriceForContractDeploymentVal = math.Max(effectiveGasPriceForContractDeploymentVal, 0)
		effectiveGasPriceForContractDeploymentVal = math.Min(effectiveGasPriceForContractDeploymentVal, 1)
		ethCfg.XLayer.L2GasPricer.EffectiveGasPriceContractDeployment = float64(math.Round(effectiveGasPriceForContractDeploymentVal * 255.0))
	}
	if ctx.IsSet(utils.DefaultGasPrice.Name) {
		ethCfg.XLayer.L2GasPricer.DefaultGasPrice = ctx.Uint64(utils.DefaultGasPrice.Name)
	}
	if ctx.IsSet(utils.GpoMaxGasPriceFlag.Name) {
		ethCfg.GPO.MaxPrice = big.NewInt(ctx.Int64(utils.GpoMaxGasPriceFlag.Name))
	}
	if ctx.IsSet(utils.GpoFactor.Name) {
		ethCfg.GPO.XLayer.Factor = ctx.Float64(utils.GpoFactor.Name)
	}
	if ctx.IsSet(utils.GpoCongestionThreshold.Name) {
		ethCfg.GPO.XLayer.CongestionThreshold = ctx.Int(utils.GpoCongestionThreshold.Name)
	}

	log.Info(fmt.Sprintf("apollo new value effective gas price eth transfer: %+v", ethCfg.XLayer.L2GasPricer.EffectiveGasPriceEthTransfer))
	log.Info(fmt.Sprintf("apollo new value effective gas price erc20 transfer: %+v", ethCfg.XLayer.L2GasPricer.EffectiveGasPriceERC20Transfer))
	log.Info(fmt.Sprintf("apollo new value effective gas price contract invocation: %+v", ethCfg.XLayer.L2GasPricer.EffectiveGasPriceContractInvocation))
	log.Info(fmt.Sprintf("apollo new value effective gas price contract deployment: %+v", ethCfg.XLayer.L2GasPricer.EffectiveGasPriceContractDeployment))
	log.Info(fmt.Sprintf("apollo new value default gas price: %+v", ethCfg.XLayer.L2GasPricer.DefaultGasPrice))
	log.Info(fmt.Sprintf("apollo new value gpo max gas price: %+v", ethCfg.GPO.MaxPrice))
	log.Info(fmt.Sprintf("apollo new value gpo factor: %+v", ethCfg.GPO.XLayer.Factor))
	log.Info(fmt.Sprintf("apollo new value gpo congestion threshold: %+v", ethCfg.GPO.XLayer.CongestionThreshold))

	ethCfg.GPO = ethconfig.Defaults.GPO
	utils.SetApolloGPOXLayer(ctx, &ethCfg.GPO)
}

func GetApolloGasPricerConfig() gasprice.Config {
	UnsafeGetApolloConfig().Lock()
	defer UnsafeGetApolloConfig().Unlock()
	return UnsafeGetApolloConfig().EthCfg.GPO
}
