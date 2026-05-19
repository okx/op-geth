package utils

import (
	"flag"
	"testing"

	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/urfave/cli/v2"
)

func TestSetXLayerP2PConfig_Default(t *testing.T) {
	app := &cli.App{
		Flags: []cli.Flag{P2PETH69CompatFlag},
		Action: func(ctx *cli.Context) error {
			var cfg ethconfig.XLayerP2PConfig
			SetXLayerP2PConfig(ctx, &cfg)
			if !cfg.ETH69Compat {
				t.Error("default should be true")
			}
			return nil
		},
	}
	app.Run([]string{"test"})
}

func TestSetXLayerP2PConfig_SetFalse(t *testing.T) {
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	set.Bool("p2p.eth69-compat", false, "")
	ctx := cli.NewContext(nil, set, nil)

	var cfg ethconfig.XLayerP2PConfig
	SetXLayerP2PConfig(ctx, &cfg)
	if cfg.ETH69Compat {
		t.Error("expected ETH69Compat to be false when flag explicitly set to false")
	}
}

func TestSetXLayerP2PConfig_SetTrue(t *testing.T) {
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	set.Bool("p2p.eth69-compat", true, "")
	set.Parse([]string{"--p2p.eth69-compat=true"})
	ctx := cli.NewContext(nil, set, nil)

	var cfg ethconfig.XLayerP2PConfig
	SetXLayerP2PConfig(ctx, &cfg)
	if !cfg.ETH69Compat {
		t.Error("expected ETH69Compat to be true when flag explicitly set to true")
	}
}
