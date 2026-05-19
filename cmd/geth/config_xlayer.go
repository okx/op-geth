package main

import (
	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/urfave/cli/v2"
)

func applyXLayerP2PConfig(ctx *cli.Context) {
	var cfg ethconfig.XLayerP2PConfig
	utils.SetXLayerP2PConfig(ctx, &cfg)
	p2p.SetETH69CompatEnabled(cfg.ETH69Compat)
}
