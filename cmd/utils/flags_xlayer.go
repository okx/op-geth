package utils

import (
	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/eth/filters"
	"github.com/ethereum/go-ethereum/internal/ethapi"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/rpc"
	"time"

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
		Category: flags.XLayerCategory,
		Value:    false,
	}
	OkPaySenderAccountsList = &cli.StringFlag{
		Name:     "okpay.sender-accounts-list",
		Usage:    "List of OkPay sender accounts",
		Category: flags.XLayerCategory,
		Value:    "",
	}
	OkPayBlockPriorityTxsLimit = &cli.Uint64Flag{
		Name:     "okpay.block-priority-txs-limit",
		Usage:    "Max number of OkPay txs that we will prioritize per block",
		Category: flags.XLayerCategory,
		Value:    0,
	}
	// InnerTx
	InnerTxFlag = &cli.BoolFlag{
		Name:     "innertx",
		Usage:    "Enable inner transaction capture and storage (disabled by default)",
		Category: flags.XLayerCategory,
		Value:    false,
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
	// XLayerFlags are the default flags for X Layer features
	XLayerFlags = []cli.Flag{
		OkPayPriorityEnableFlag,
		OkPaySenderAccountsList,
		OkPayBlockPriorityTxsLimit,
		InnerTxFlag,
		MigrationBlockFlag,
		PPRPCUrlFlag,
		PPRPCTimeoutFlag,
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

func setInnerTxXLayer(ctx *cli.Context, cfg *ethconfig.Config) {
	if ctx.IsSet(InnerTxFlag.Name) {
		cfg.EnableInnerTx = ctx.Bool(InnerTxFlag.Name)
	}
}

func setMigrationXLayer(ctx *cli.Context, cfg *ethconfig.Config) {
	// Migration configuration
	if ctx.IsSet(MigrationBlockFlag.Name) {
		migrationBlock := ctx.Uint64(MigrationBlockFlag.Name)
		cfg.XLayer.RpcMigration.MigrationBlock = &migrationBlock
	}
	if ctx.IsSet(PPRPCUrlFlag.Name) {
		cfg.XLayer.RpcMigration.PPRPCUrl = ctx.String(PPRPCUrlFlag.Name)
	}
	if ctx.IsSet(PPRPCTimeoutFlag.Name) {
		cfg.XLayer.RpcMigration.PPRPCTimeout = ctx.Duration(PPRPCTimeoutFlag.Name)
	} else if cfg.XLayer.RpcMigration.PPRPCTimeout == 0 && cfg.XLayer.RpcMigration.PPRPCUrl != "" {
		cfg.XLayer.RpcMigration.PPRPCTimeout = 10 * time.Second
	}
}

// SetOkPayXLayer is a public wrapper function to internally call setOkPayXLayer
func SetXLayerConfig(ctx *cli.Context, cfg *ethconfig.Config) {
	setOkPayXLayer(ctx, cfg)
	setInnerTxXLayer(ctx, cfg)
	setMigrationXLayer(ctx, cfg)
}

// RegisterMigrationFilterAPI adds the eth log filtering RPC API to the node.
func RegisterMigrationFilterAPI(stack *node.Node, backend ethapi.Backend, ethcfg *ethconfig.Config) *filters.FilterSystem {
	filterSystem := filters.NewFilterSystem(backend, filters.Config{
		LogCacheSize: ethcfg.FilterLogCacheSize,
	})
	migrationCfg, err := eth.NewMigrationConfig(ethcfg)
	if err != nil {
		panic(err)
	}
	originalFilterApi := filters.NewFilterAPI(filterSystem)
	migrationFilterApi := rpc.API{
		Namespace: "eth",
		Service:   eth.NewMigrationFilterAPI(originalFilterApi, migrationCfg),
	}
	stack.RegisterAPIs([]rpc.API{migrationFilterApi})
	return filterSystem
}
