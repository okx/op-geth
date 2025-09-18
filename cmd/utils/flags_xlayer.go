package utils

import (
	"time"

	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/eth/filters"
	"github.com/ethereum/go-ethereum/internal/ethapi"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/internal/flags"
	"github.com/urfave/cli/v2"
)

var (
	// OkPay
	OkPayPriorityEnableFlag = &cli.BoolFlag{
		Name:     "okpay.priority-enable-flag",
		Usage:    "OkPay",
		Value:    false,
		Category: flags.XLayerCategory,
	}
	OkPaySenderAccountsList = &cli.StringFlag{
		Name:     "okpay.sender-accounts-list",
		Usage:    "List of OkPay sender accounts",
		Value:    "",
		Category: flags.XLayerCategory,
	}
	OkPayBlockPriorityTxsLimit = &cli.Uint64Flag{
		Name:     "okpay.block-priority-txs-limit",
		Usage:    "Max number of OkPay txs that we will prioritize per block",
		Value:    0,
		Category: flags.XLayerCategory,
	}

	// Xlayer Intercept feature
	InterceptEnabled = &cli.BoolFlag{
		Name:     "intercept.enabled",
		Usage:    "Enable the intercept feature",
		Value:    ethconfig.Defaults.Miner.InterceptConfig.Enabled,
		Category: flags.XLayerCategory,
	}
	InterceptBridgeContractAddress = &cli.StringFlag{
		Name:     "intercept.bridgeContractAddress",
		Usage:    "The target bridge contract address to intercept",
		Value:    ethconfig.Defaults.Miner.InterceptConfig.BridgeContractAddress,
		Category: flags.XLayerCategory,
	}
	InterceptTargetTokenAddress = &cli.StringFlag{
		Name:     "intercept.targetTokenAddress",
		Usage:    "The target token address to intercept",
		Value:    ethconfig.Defaults.Miner.InterceptConfig.TargetTokenAddress,
		Category: flags.XLayerCategory,
	}

	// InnerTx
	InnerTxFlag = &cli.BoolFlag{
		Name:     "innertx",
		Usage:    "Enable inner transaction capture and storage (disabled by default)",
		Value:    false,
		Category: flags.XLayerCategory,
	}
	// Migration flags for XLayer routing
	MigrationBlockFlag = &cli.Uint64Flag{
		Name:     "migration-block",
		Usage:    "Block height threshold for migration routing from erigon to op-geth",
		Category: flags.XLayerCategory,
		EnvVars:  []string{"OP_MIGRATION_BLOCK"},
	}
	PPRPCUrlFlag = &cli.StringFlag{
		Name:     "pp-rpc-url",
		Usage:    "XLayer-Erigon RPC endpoint URL for pre-migration blocks",
		Category: flags.XLayerCategory,
		EnvVars:  []string{"OP_PP_RPC_URL"},
	}
	PPRPCTimeoutFlag = &cli.DurationFlag{
		Name:     "pp-rpc-timeout",
		Usage:    "Timeout for PP RPC calls",
		Value:    10 * time.Second,
		Category: flags.XLayerCategory,
		EnvVars:  []string{"OP_PP_RPC_TIMEOUT"},
	}
	TraceLogPath = &cli.StringFlag{
		Name:  "monitor.trace-log-path",
		Usage: "Path of trace.log for transaction monitoring",
		Value: "/var/log/op-geth/trace.log",
	}

	EnableTraceLog = &cli.BoolFlag{
		Name:  "monitor.enable-trace-log",
		Usage: "Enable full transaction trace log",
		Value: false,
	}
	// XLayerFlags are the default flags for X Layer features
	XLayerFlags = []cli.Flag{
		OkPayPriorityEnableFlag,
		OkPaySenderAccountsList,
		OkPayBlockPriorityTxsLimit,
		InterceptEnabled,
		InterceptBridgeContractAddress,
		InterceptTargetTokenAddress,
		InnerTxFlag,
		MigrationBlockFlag,
		PPRPCUrlFlag,
		PPRPCTimeoutFlag,
		TraceLogPath,
		EnableTraceLog,
	}
)

func setOkPayXLayer(ctx *cli.Context, cfg *ethconfig.Config) {
	if ctx.IsSet(OkPayPriorityEnableFlag.Name) {
		cfg.XLayer.OkPay.PriorityEnable = ctx.Bool(OkPayPriorityEnableFlag.Name)
	}
	if !cfg.XLayer.OkPay.PriorityEnable {
		return
	}
	if ctx.IsSet(OkPayBlockPriorityTxsLimit.Name) {
		cfg.XLayer.OkPay.BlockPriorityTxsLimit = ctx.Uint64(OkPayBlockPriorityTxsLimit.Name)
	}
	if ctx.IsSet(OkPaySenderAccountsList.Name) {
		addrHexes := SplitAndTrim(ctx.String(OkPaySenderAccountsList.Name))
		cfg.XLayer.OkPay.SenderAccountsList = make([]common.Address, 0, len(addrHexes))
		for _, senderHex := range addrHexes {
			cfg.XLayer.OkPay.SenderAccountsList = append(cfg.XLayer.OkPay.SenderAccountsList, common.HexToAddress(senderHex))
		}
	}
}

func setXLayerIntercept(ctx *cli.Context, cfg *ethconfig.Config) {
	if ctx.IsSet(InterceptEnabled.Name) {
		cfg.Miner.InterceptConfig.Enabled = ctx.Bool(InterceptEnabled.Name)
	}
	if ctx.IsSet(InterceptBridgeContractAddress.Name) {
		cfg.Miner.InterceptConfig.BridgeContractAddress = ctx.String(InterceptBridgeContractAddress.Name)
	}
	if ctx.IsSet(InterceptTargetTokenAddress.Name) {
		cfg.Miner.InterceptConfig.TargetTokenAddress = ctx.String(InterceptTargetTokenAddress.Name)
	}
}

func setInnerTxXLayer(ctx *cli.Context, cfg *ethconfig.Config) {
	if ctx.IsSet(InnerTxFlag.Name) {
		cfg.EnableInnerTx = ctx.Bool(InnerTxFlag.Name)
	}
}

func setMigrationXLayer(ctx *cli.Context, cfg *ethconfig.Config) {
	// Migration configuration
	if ctx.IsSet(MigrationBlockFlag.Name) {
		migrationBlock := ctx.Uint64(MigrationBlockFlag.Name)
		cfg.XLayer.LegacyPp.MigrationBlock = &migrationBlock
	}
	if ctx.IsSet(PPRPCUrlFlag.Name) {
		cfg.XLayer.LegacyPp.PPRPCUrl = ctx.String(PPRPCUrlFlag.Name)
	}
	if ctx.IsSet(PPRPCTimeoutFlag.Name) {
		cfg.XLayer.LegacyPp.PPRPCTimeout = ctx.Duration(PPRPCTimeoutFlag.Name)
	} else if cfg.XLayer.LegacyPp.PPRPCTimeout == 0 && cfg.XLayer.LegacyPp.PPRPCUrl != "" {
		cfg.XLayer.LegacyPp.PPRPCTimeout = 10 * time.Second
	}
}

// SetOkPayXLayer is a public wrapper function to internally call setOkPayXLayer
func SetXLayerConfig(ctx *cli.Context, cfg *ethconfig.Config) {
	setOkPayXLayer(ctx, cfg)
	setXLayerIntercept(ctx, cfg)
	setInnerTxXLayer(ctx, cfg)
	setMigrationXLayer(ctx, cfg)
	setMonitor(ctx, &cfg.Monitor)
}

// RegisterXlayerHybridFilterAPI adds the eth log filtering RPC API to the node.
func RegisterXlayerHybridFilterAPI(stack *node.Node, backend ethapi.Backend, ethcfg *ethconfig.Config) *filters.FilterSystem {
	filterSystem := filters.NewFilterSystem(backend, filters.Config{
		LogCacheSize: ethcfg.FilterLogCacheSize,
	})
	xlayerLegacyRpcService, err := eth.NewXlayerLegacyRPCService(ethcfg)
	if err != nil {
		panic(err)
	}
	originalFilterApi := filters.NewFilterAPI(filterSystem)
	xlayerLegacyFilterApi := rpc.API{
		Namespace: "eth",
		Service:   eth.NewXlayerHybridFilterAPI(originalFilterApi, xlayerLegacyRpcService),
	}
	stack.RegisterAPIs([]rpc.API{xlayerLegacyFilterApi})
	return filterSystem
}

// setMonitor applies monitor-related command line flags to the config.
func setMonitor(ctx *cli.Context, cfg *ethconfig.MonitorConfig) {
	if ctx.IsSet(EnableTraceLog.Name) {
		cfg.EnableTraceLog = ctx.Bool(EnableTraceLog.Name)
	}
	if ctx.IsSet(TraceLogPath.Name) {
		cfg.TraceLogPath = ctx.String(TraceLogPath.Name)
	}
}
