package apollo

import (
	"sync"

	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/node"
)

// Config holds Apollo configuration parameters
type Config struct {
	AppID         string
	IP            string
	Cluster       string
	NamespaceName string
}

// ApolloConfig holds the global Apollo configuration state
type ApolloConfig struct {
	sync.RWMutex
	EthCfg  ethconfig.Config
	NodeCfg node.Config
}

// Global Apollo configuration instance
var globalApolloConfig *ApolloConfig
var configMutex sync.RWMutex

// UnsafeGetApolloConfig returns the global Apollo configuration
// This is unsafe and should be used carefully
func UnsafeGetApolloConfig() *ApolloConfig {
	configMutex.RLock()
	defer configMutex.RUnlock()
	return globalApolloConfig
}

// SetApolloConfig sets the global Apollo configuration
func SetApolloConfig(ethCfg ethconfig.Config, nodeCfg node.Config) {
	configMutex.Lock()
	defer configMutex.Unlock()

	if globalApolloConfig == nil {
		globalApolloConfig = &ApolloConfig{}
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
