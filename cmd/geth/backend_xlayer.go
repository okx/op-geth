package main

import (
	"github.com/apolloconfig/agollo/v4/env/config"
	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/xlayer/apollo"
)

// initApollo adds the X Layer backend to the node
func initApollo(stack *node.Node, cfg *gethConfig) {
	if stack == nil || cfg == nil {
		utils.Fatalf("Stack or config is nil")
	}

	// Initialize Apollo configuration if enabled
	if cfg.Eth.XLayer.Apollo.Enable {
		apolloService, err := apollo.TryInitialize(&config.AppConfig{
			AppID:         cfg.Eth.XLayer.Apollo.AppID,
			IP:            cfg.Eth.XLayer.Apollo.IP,
			Cluster:       cfg.Eth.XLayer.Apollo.Cluster,
			NamespaceName: cfg.Eth.XLayer.Apollo.NamespaceName,
		})

		if err != nil {
			utils.Fatalf("Failed to initialize Apollo configuration: %v", err)
		} else {
			log.Info("Apollo client initialized")
		}

		// Register cleanup function for Apollo
		stack.RegisterLifecycle(apolloService)
	}
}
