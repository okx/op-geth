package apollo

import (
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/node"
)

type ApolloConfig interface {
	UnsafeGetApolloConfig() *ApolloConfig
	SetApolloConfig(ethCfg ethconfig.Config, nodeCfg node.Config)
	IsApolloConfigSet() bool
}
