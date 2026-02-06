package utils

import (
	"time"

	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/eth/filters"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/internal/ethapi"
	"github.com/ethereum/go-ethereum/internal/flags"
	"github.com/urfave/cli/v2"
)

var (
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
	// Transaction trace related flags
	TraceLogPath = &cli.StringFlag{
		Name:  "tx-trace.output-path",
		Usage: "Path to write transaction trace output file. If the path ends with a directory separator or has no extension, trace.log will be appended",
	}
	EnableTraceLog = &cli.BoolFlag{
		Name:  "tx-trace.enable",
		Usage: "Enable transaction tracing",
		Value: false,
	}

	// XLayerFlags are the default flags for X Layer features
	XLayerFlags = []cli.Flag{
		MigrationBlockFlag,
		PPRPCUrlFlag,
		PPRPCTimeoutFlag,
		TraceLogPath,
		EnableTraceLog,
	}
)

// SetXLayerConfig is a public wrapper function to internally call all XLayer configuration functions
func SetXLayerConfig(ctx *cli.Context, cfg *ethconfig.Config) {
	setMigrationXLayer(ctx, cfg)
	setMonitorXLayer(ctx, cfg)
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

// setMonitorXLayer applies transaction trace-related command line flags to the config.
// This matches the reth implementation for consistency.
// If enabled but path is not set, use datadir/logs/trace.log as default
func setMonitorXLayer(ctx *cli.Context, cfg *ethconfig.Config) {
	if ctx.IsSet(EnableTraceLog.Name) {
		cfg.XLayer.Monitor.EnableTraceLog = ctx.Bool(EnableTraceLog.Name)
	}
	if ctx.IsSet(TraceLogPath.Name) {
		path := ctx.String(TraceLogPath.Name)
		// Match reth's path handling: if path ends with directory separator or has no extension,
		// trace.log will be appended in InitTraceLogger
		cfg.XLayer.Monitor.TraceLogPath = path
	} else if cfg.XLayer.Monitor.EnableTraceLog && cfg.XLayer.Monitor.TraceLogPath == "" {
		// If enabled but path not specified, use default: datadir/logs/trace.log
		// This matches reth's behavior of using a default path when enabled
		cfg.XLayer.Monitor.TraceLogPath = "logs/trace.log"
	}
}
