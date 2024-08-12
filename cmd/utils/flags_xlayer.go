package utils

import (
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/urfave/cli/v2"
)

var (
	// X Layer nacos
	NacosURLsFlag = &cli.StringFlag{
		Name:  "nacos.urls",
		Usage: "Nacos urls.",
		Value: "",
	}
	NacosNamespaceIdFlag = &cli.StringFlag{
		Name:  "nacos.namespace-id",
		Usage: "Nacos namespace Id.",
		Value: "",
	}
	NacosApplicationNameFlag = &cli.StringFlag{
		Name:  "nacos.application-name",
		Usage: "Nacos application name",
		Value: "",
	}
	NacosExternalListenAddrFlag = &cli.StringFlag{
		Name:  "nacos.external-listen-addr",
		Usage: "Nacos external listen addr.",
		Value: "",
	}
)

func setXLayerNacos(ctx *cli.Context, cfg *ethconfig.NacosConfig) {
	if ctx.IsSet(NacosURLsFlag.Name) {
		cfg.URLs = ctx.String(NacosURLsFlag.Name)
	}

	if ctx.IsSet(NacosNamespaceIdFlag.Name) {
		cfg.NamespaceId = ctx.String(NacosNamespaceIdFlag.Name)
	}

	if ctx.IsSet(NacosApplicationNameFlag.Name) {
		cfg.ApplicationName = ctx.String(NacosApplicationNameFlag.Name)
	}

	if ctx.IsSet(NacosExternalListenAddrFlag.Name) {
		cfg.ExternalListenAddr = ctx.String(NacosExternalListenAddrFlag.Name)
	}
}
