package apollo

import (
	"fmt"

	"github.com/apolloconfig/agollo/v4/storage"
	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/eth/gasprice"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"github.com/urfave/cli/v2"
)

// loadL2GasPricer loads the apollo l2gaspricer config cache on startup
func (g *GethConfigHandler) loadL2GasPricer(ctx *cli.Context) {
	// Load l2gaspricer config changes
	loadL2GasPricerConfig(ctx)
	log.Info(fmt.Sprintf("loaded l2gaspricer from apollo config"))
}

// fireL2GasPricer fires the apollo l2gaspricer config change
func fireL2GasPricer(ctx *cli.Context, value *storage.ConfigChange) {
	loadL2GasPricerConfig(ctx)
	log.Info(fmt.Sprintf("apollo l2gaspricer old config : %+v", value.OldValue.(string)))
	log.Info(fmt.Sprintf("apollo l2gaspricer config changed: %+v", value.NewValue.(string)))
}

// loadL2GasPricerConfig loads the dynamic gas pricer apollo configurations
func loadL2GasPricerConfig(ctx *cli.Context) {
	TryUnsafeGetApolloConfig().Lock()
	defer TryUnsafeGetApolloConfig().Unlock()

	config := TryUnsafeGetApolloConfig()
	if config == nil {
		log.Warn("Apollo config is nil, skipping L2GasPricer config load")
		return
	}
	loadNodeL2GasPricerConfig(ctx, TryUnsafeGetApolloConfig().NodeCfg)
	loadEthL2GasPricerConfig(ctx, TryUnsafeGetApolloConfig().EthCfg)
}

// loadNodeL2GasPricerConfig loads the dynamic gas pricer apollo node configurations
func loadNodeL2GasPricerConfig(ctx *cli.Context, nodeCfg *node.Config) {
	// Load l2gaspricer config
}

// loadEthL2GasPricerConfig loads the dynamic gas pricer apollo eth configurations
func loadEthL2GasPricerConfig(ctx *cli.Context, ethCfg *ethconfig.Config) {
	ethCfg.GPO = ethconfig.Defaults.GPO
	utils.SetApolloGPOXLayer(ctx, &ethCfg.GPO)
}

func GetApolloGasPricerConfig() gasprice.Config {
	TryUnsafeGetApolloConfig().Lock()
	defer TryUnsafeGetApolloConfig().Unlock()
	return TryUnsafeGetApolloConfig().EthCfg.GPO
}
