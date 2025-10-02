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
	EthCfg  ethconfig.Config
	NodeCfg node.Config
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
func SetApolloConfig(ethCfg ethconfig.Config, nodeCfg node.Config) {
	configMutex.Lock()
	defer configMutex.Unlock()

	if globalApolloConfig == nil {
		globalApolloConfig = &ApolloConfigImpl{}
	}

	globalApolloConfig.EthCfg = ethCfg
	globalApolloConfig.NodeCfg = nodeCfg
}

// IsApolloConfigSet checks if Apollo configuration has been set
func IsApolloConfigSet() bool {
	configMutex.RLock()
	defer configMutex.RUnlock()
	return globalApolloConfig != nil
}

// GethConfigHandler implements geth-specific configuration change logic
type GethConfigHandler struct{}

// HandleConfigChange implements geth-specific configuration change logic
func (g *GethConfigHandler) HandleConfigChange(prefix string, ctx *cli.Context, key string, value *storage.ConfigChange) {
	log.Info("Geth HandleConfigChange called", "prefix", prefix, "key", key, "value", value.NewValue)
	switch prefix {
	case apollo.L2GasPricer:
		log.Info("Lucas Geth L2GasPricer config changed", "key", key, "value", value.NewValue)
		fireL2GasPricer(ctx, value)
	default:
		log.Info("Lucas Geth unknown config prefix", "prefix", prefix, "key", key, "value", value.NewValue)
	}
}

// NewGethConfigHandler creates a new geth-specific config handler
func NewGethConfigHandler() *GethConfigHandler {
	return &GethConfigHandler{}
}
