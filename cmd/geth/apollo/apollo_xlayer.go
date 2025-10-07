package apollo

import (
	"strings"
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

// UnsafeGetApolloConfig returns the global Apollo configuration
// This is unsafe and should be used carefully
func TryUnsafeGetApolloConfig() *ApolloConfigImpl {
	return globalApolloConfig
}

// SetApolloConfig sets the global Apollo configuration
func SetApolloConfig(ethCfg *ethconfig.Config, nodeCfg *node.Config) {
	if globalApolloConfig == nil {
		globalApolloConfig = &ApolloConfigImpl{}
	}

	globalApolloConfig.EthCfg = ethCfg
	globalApolloConfig.NodeCfg = nodeCfg
}

func IsApolloConfigSet() bool {
	return globalApolloConfig != nil
}

type GethConfigHandler struct{}

// HandleConfigChange implements op-geth-specific configuration change logic
func (g *GethConfigHandler) HandleConfigChange(prefix string, ctx *cli.Context, key string, value *storage.ConfigChange) {
	// prefix is the full namespace (e.g. "opgeth_l2gaspricer"), extract component
	component := getComponentFromNamespace(prefix)

	// Validate that this is for op-geth component
	if component != apollo.OpGethComponent {
		log.Warn("OpGeth received config change for non-opgeth namespace, ignoring", "component", component, "prefix", prefix)
		return
	}

	log.Info("OpGeth handling config change", "component", component, "prefix", prefix, "key", key)

	switch prefix {
	case apollo.L2GasPricer:
		log.Info("opgeth l2gaspricer config changed", "key", key, "value", value.NewValue)
		fireL2GasPricer(ctx, value)
	default:
		log.Info("Geth unknown config prefix", "prefix", prefix, "key", key, "value", value.NewValue)
	}
}

// LoadConfig implements op-geth-specific configuration loading logic
func (g *GethConfigHandler) LoadConfig(prefix string, ctx *cli.Context) {
	// prefix is the full namespace (e.g. "opgeth_l2gaspricer"), extract component
	component := getComponentFromNamespace(prefix)

	// Validate that this is for op-geth component
	if component != apollo.OpGethComponent {
		log.Warn("OpGeth received config load request for non-opgeth namespace, ignoring", "component", component, "prefix", prefix)
		return
	}

	log.Info("OpGeth loading config", "component", component, "prefix", prefix)

	switch prefix {
	case apollo.L2GasPricer:
		g.loadL2GasPricer(ctx)
	default:
		log.Info("OpGeth unknown namespace for loading", "prefix", prefix)
	}
}

// getComponentFromNamespace extracts the component prefix from namespace
// e.g. "opgeth_l2gaspricer" -> "opgeth"
func getComponentFromNamespace(namespace string) string {
	parts := strings.Split(namespace, "_")
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

// NewGethConfigHandler creates a new geth-specific config handler
func NewGethConfigHandler() *GethConfigHandler {
	return &GethConfigHandler{}
}
