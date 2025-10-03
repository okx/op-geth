package apollo

import (
	"sync"

	"github.com/apolloconfig/agollo/v4/storage"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/xlayer/apollo"
	"github.com/urfave/cli/v2"
)

// ApolloConfig holds the global Apollo configuration state
type ApolloConfigImpl struct {
	sync.RWMutex
	EthCfg  *ethconfig.Config
	NodeCfg *node.Config
}

// Global Apollo configuration instance
var globalApolloConfig *ApolloConfigImpl
var configMutex sync.RWMutex

// UnsafeGetApolloConfig returns the global Apollo configuration
// This is unsafe and should be used carefully
func UnsafeGetApolloConfig() *ApolloConfigImpl {
	configMutex.RLock()
	defer configMutex.RUnlock()
	return globalApolloConfig
}

// SetApolloConfig sets the global Apollo configuration
func SetApolloConfig(ethCfg *ethconfig.Config, nodeCfg *node.Config) {
	configMutex.Lock()
	defer configMutex.Unlock()

	if globalApolloConfig == nil {
		globalApolloConfig = &ApolloConfigImpl{}
	}

	globalApolloConfig.EthCfg = ethCfg
	globalApolloConfig.NodeCfg = nodeCfg
}

func IsApolloConfigSet() bool {
	configMutex.RLock()
	defer configMutex.RUnlock()
	return globalApolloConfig != nil
}

type GethConfigHandler struct{}

// HandleConfigChange implements geth-specific configuration change logic
func (g *GethConfigHandler) HandleConfigChange(prefix string, ctx *cli.Context, key string, value *storage.ConfigChange) {
	switch prefix {
	case apollo.L2GasPricer:
		log.Info("Geth L2GasPricer config changed", "key", key, "value", value.NewValue)
		fireL2GasPricer(ctx, value)
	default:
		log.Info("Geth unknown config prefix", "prefix", prefix, "key", key, "value", value.NewValue)
	}
}

// LoadConfig implements geth-specific configuration loading logic
func (g *GethConfigHandler) LoadConfig(prefix string, ctx *cli.Context) {
	log.Info("Geth LoadConfig called", "prefix", prefix)
	switch prefix {
	case apollo.L2GasPricer:
		g.loadL2GasPricer(ctx)
	default:
		log.Info("Geth unknown config prefix for loading", "prefix", prefix)
	}
}

// NewGethConfigHandler creates a new geth-specific config handler
func NewGethConfigHandler() *GethConfigHandler {
	return &GethConfigHandler{}
}
