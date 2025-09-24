package main

import (
	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/eth/xlayer/apollo"
	"github.com/ethereum/go-ethereum/node"
)

// addXLayerBackend adds the X Layer backend to the node
func addXLayerBackend(stack *node.Node, cfg *gethConfig) {
	if stack == nil || cfg == nil {
		utils.Fatalf("Stack or config is nil")
	}

	// Initialize Apollo configuration if enabled
	if cfg.Eth.XLayer.Apollo.Enable {
		client, err := apollo.GetInstance(&cfg.Eth)
		if err != nil {
			utils.Fatalf("Failed to initialize Apollo configuration: %v", err)
		}
		// Register cleanup function for Apollo
		stack.RegisterLifecycle(client)
	}
}
