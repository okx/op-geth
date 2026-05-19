package utils

import (
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/internal/flags"
	"github.com/urfave/cli/v2"
)

var (
	P2PETH69CompatFlag = &cli.BoolFlag{
		Name:     "p2p.eth69-compat",
		Usage:    "Enable eth/69 capability trim for Geth-identified peers (default: enabled; set to false to allow eth/69 negotiation)",
		Value:    true,
		Category: flags.NetworkingCategory,
		EnvVars:  []string{"OP_P2P_ETH69_COMPAT"},
	}
)

func SetXLayerP2PConfig(ctx *cli.Context, cfg *ethconfig.XLayerP2PConfig) {
	if ctx.IsSet(P2PETH69CompatFlag.Name) {
		cfg.ETH69Compat = ctx.Bool(P2PETH69CompatFlag.Name)
	} else {
		cfg.ETH69Compat = P2PETH69CompatFlag.Value
	}
}
