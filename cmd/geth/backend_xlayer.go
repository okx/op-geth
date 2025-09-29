package main

import (
	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/xlayer/apollo"
)

// addXLayerBackend adds the X Layer backend to the node
func addXLayerBackend(stack *node.Node, cfg *gethConfig) {
	if stack == nil || cfg == nil {
		utils.Fatalf("Stack or config is nil")
	}

	// Initialize Apollo configuration if enabled
	if cfg.Eth.XLayer.Apollo.Enable {
		apollo.SetApolloConfig(cfg.Eth, cfg.Node)
		client, err := apollo.GetInstance(&cfg.Eth)
		if err != nil {
			utils.Fatalf("Failed to initialize Apollo configuration: %v", err)
		} else {
			log.Info("Apollo client initialized for dynamic gas price configuration")
		}
		// Register cleanup function for Apollo
		stack.RegisterLifecycle(client)
		client.LoadConfig()
	}
}
