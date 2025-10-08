package utils

import (
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/eth/filters"
	"github.com/ethereum/go-ethereum/eth/gasprice"
	"github.com/ethereum/go-ethereum/internal/flags"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/internal/ethapi"
	"github.com/urfave/cli/v2"
)

const EnvKafkaConsumerGroupID = "REALTIME_KAFKA_CONSUMER_GROUP_ID"

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
	// Monitor related flags
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
	// Realtime feature
	RealtimeEnableFlag = &cli.BoolFlag{
		Name:  "realtime.enable-flag",
		Usage: "Realtime enable flag",
		Value: false,
	}
	RealtimeRpcFlag = &cli.BoolFlag{
		Name:  "realtime.rpc-flag",
		Usage: "Realtime is rpc mode flag",
		Value: false,
	}
	RealtimeEnableSubscribeFlag = &cli.BoolFlag{
		Name:  "realtime.enable-subscribe-flag",
		Usage: "Enable subscribe flag",
		Value: false,
	}
	RealtimeCacheHeightThreshold = &cli.Uint64Flag{
		Name:  "realtime.cache-height-threshold",
		Usage: "Cache height threshold to clear",
		Value: 10,
	}
	RealtimeSubscribeKafka = &cli.BoolFlag{
		Name:  "realtime.subscribe-kafka",
		Usage: "Subscribe kafka",
		Value: false,
	}
	RealtimeKafkaSyncBootstrapServers = &cli.StringFlag{
		Name:  "realtime.kafka-sync-bootstrap-servers",
		Usage: "Kafka sync bootstrap servers",
		Value: "",
	}
	RealtimeKafkaSyncBlockTopic = &cli.StringFlag{
		Name:  "realtime.kafka-sync-block-topic",
		Usage: "Kafka block topic",
		Value: "",
	}
	RealtimeKafkaSyncTxTopic = &cli.StringFlag{
		Name:  "realtime.kafka-sync-tx-topic",
		Usage: "Kafka tx topic",
		Value: "",
	}
	RealtimeKafkaSyncErrorTopic = &cli.StringFlag{
		Name:  "realtime.kafka-sync-error-topic",
		Usage: "Kafka error trigger topic",
		Value: "",
	}
	RealtimeKafkaSyncClientID = &cli.StringFlag{
		Name:  "realtime.kafka-sync-client-id",
		Usage: "Kafka sync client id",
		Value: "",
	}
	RealtimeKafkaSyncGroupID = &cli.StringFlag{
		Name:  "realtime.kafka-sync-group-id",
		Usage: "Kafka sync group id",
		Value: "",
	}
	RealtimeSubscribeWebsocket = &cli.BoolFlag{
		Name:  "realtime.subscribe-wesocket",
		Usage: "Subscribe wesocket",
		Value: false,
	}
	RealtimeStreamerUrl = &cli.StringFlag{
		Name:  "realtime.streamer-url",
		Usage: "Streamer url",
		Value: "",
	}
	RealtimeStreamerUseTLS = &cli.BoolFlag{
		Name:  "realtime.streamer-use-tls",
		Usage: "Streamer use tls",
		Value: false,
	}
	RealtimeStreamerTimeout = &cli.DurationFlag{
		Name:  "realtime.streamer-timeout",
		Usage: "Streamer timeout",
		Value: 200 * time.Second,
	}
	RealtimeCacheDumpPath = &cli.StringFlag{
		Name:  "realtime.cache-dump-path",
		Usage: "Cache dump path",
		Value: "/home/erigon/data/cache",
	}
	// Apollo
	ApolloEnabledFlag = &cli.BoolFlag{
		Name:  "apollo.enabled",
		Usage: "Enable Apollo configuration service",
		Value: false,
	}
	ApolloAppIDFlag = &cli.StringFlag{
		Name:  "apollo.app-id",
		Usage: "Apollo app ID",
		Value: "",
	}
	ApolloIPFlag = &cli.StringFlag{
		Name:  "apollo.ip",
		Usage: "Apollo IP",
		Value: "",
	}
	ApolloClusterFlag = &cli.StringFlag{
		Name:  "apollo.cluster",
		Usage: "Apollo cluster name",
		Value: "default",
	}
	ApolloNamespaceFlag = &cli.StringFlag{
		Name:  "apollo.namespace",
		Usage: "Apollo namespace",
		Value: "application",
	}
	// GPO
	GpoType = &cli.StringFlag{
		Name:  "gpo.type",
		Usage: "GPO type",
		Value: "follower",
	}

	GpoUpdatePeriod = &cli.Uint64Flag{
		Name:  "gpo.update-period",
		Usage: "GPO update period",
		Value: 100000000000,
	}

	GpoFactor = &cli.Float64Flag{
		Name:  "gpo.factor",
		Usage: "raw gas price factor (Follower mode only)",
		Value: 0,
	}

	GpoKafkaURL = &cli.StringFlag{
		Name:  "gpo.kafka-url",
		Usage: "GPO kafka url",
		Value: "localhost:9092",
	}

	GpoTopic = &cli.StringFlag{
		Name:  "gpo.topic",
		Usage: "GPO topic",
		Value: "middle_coinPrice_push",
	}

	GpoGroupID = &cli.StringFlag{
		Name:  "gpo.group-id",
		Usage: "GPO group id",
		Value: "geth-consumer",
	}

	GpoL1CoinId = &cli.Uint64Flag{
		Name:  "gpo.l1-coin-id",
		Usage: "GPO l1 coin id",
		Value: 15756,
	}

	GpoL2CoinId = &cli.Uint64Flag{
		Name:  "gpo.l2-coin-id",
		Usage: "GPO l2 coin id",
		Value: 7184,
	}

	GpoDefaultL1CoinPrice = &cli.Float64Flag{
		Name:  "gpo.default-l1-coin-price",
		Usage: "GPO default l1 coin price",
		Value: 2000.0,
	}

	GpoDefaultL2CoinPrice = &cli.Float64Flag{
		Name:  "gpo.default-l2-coin-price",
		Usage: "GPO default l2 coin price",
		Value: 0.5,
	}

	GpoGasPriceUsdt = &cli.Float64Flag{
		Name:  "gpo.gas-price-usdt",
		Usage: "GPO gas price usdt",
		Value: 0,
	}

	GpoCongestionThreshold = &cli.Uint64Flag{
		Name:  "gpo.congestion-threshold",
		Usage: "GPO congestion threshold",
		Value: 0,
	}

	GpoDefault = &cli.StringFlag{
		Name:  "gpo.default",
		Usage: "GPO default",
		Value: "100000000",
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
		RealtimeEnableFlag,
		RealtimeRpcFlag,
		RealtimeEnableSubscribeFlag,
		RealtimeCacheHeightThreshold,
		RealtimeSubscribeKafka,
		RealtimeKafkaSyncBootstrapServers,
		RealtimeKafkaSyncBlockTopic,
		RealtimeKafkaSyncTxTopic,
		RealtimeKafkaSyncErrorTopic,
		RealtimeKafkaSyncClientID,
		RealtimeKafkaSyncGroupID,
		RealtimeSubscribeWebsocket,
		RealtimeStreamerUrl,
		RealtimeStreamerUseTLS,
		RealtimeStreamerTimeout,
		RealtimeCacheDumpPath,
		ApolloEnabledFlag,
		ApolloAppIDFlag,
		ApolloIPFlag,
		ApolloClusterFlag,
		ApolloNamespaceFlag,
		GpoType,
		GpoUpdatePeriod,
		GpoDefault,
		GpoKafkaURL,
		GpoTopic,
		GpoGroupID,
		GpoL1CoinId,
		GpoL2CoinId,
		GpoDefaultL1CoinPrice,
		GpoDefaultL2CoinPrice,
		GpoGasPriceUsdt,
		GpoCongestionThreshold,
		GpoFactor,
	}
)

func SetXLayerConfig(ctx *cli.Context, cfg *ethconfig.Config) {
	setOkPayXLayer(ctx, cfg)
	setXLayerIntercept(ctx, cfg)
	setInnerTxXLayer(ctx, cfg)
	setMigrationXLayer(ctx, cfg)
	setMonitorXLayer(ctx, cfg)
	setApolloXLayer(ctx, cfg)
	setGPOXLayer(ctx, cfg)
	setRealtimeXLayer(ctx, cfg)
}

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
		cfg.XLayer.EnableInnerTx = ctx.Bool(InnerTxFlag.Name)
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

func setApolloXLayer(ctx *cli.Context, cfg *ethconfig.Config) {
	if ctx.IsSet(ApolloEnabledFlag.Name) {
		cfg.XLayer.Apollo.Enable = ctx.Bool(ApolloEnabledFlag.Name)
	}
	if !cfg.XLayer.Apollo.Enable {
		return
	}
	if ctx.IsSet(ApolloAppIDFlag.Name) {
		cfg.XLayer.Apollo.AppID = ctx.String(ApolloAppIDFlag.Name)
	}
	if ctx.IsSet(ApolloIPFlag.Name) {
		cfg.XLayer.Apollo.IP = ctx.String(ApolloIPFlag.Name)
	}
	if ctx.IsSet(ApolloClusterFlag.Name) {
		cfg.XLayer.Apollo.Cluster = ctx.String(ApolloClusterFlag.Name)
	}
	if ctx.IsSet(ApolloNamespaceFlag.Name) {
		cfg.XLayer.Apollo.NamespaceName = ctx.String(ApolloNamespaceFlag.Name)
	}
}

// RegisterXlayerHybridFilterAPI adds the eth log filtering RPC API to the node.
func RegisterXlayerHybridFilterAPI(stack *node.Node, backend ethapi.Backend, ethcfg *ethconfig.Config) (*filters.FilterSystem, *filters.FilterAPI) {
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
	return filterSystem, originalFilterApi
}

// setMonitorXLayer applies monitor-related command line flags to the config.
func setMonitorXLayer(ctx *cli.Context, cfg *ethconfig.Config) {
	if ctx.IsSet(EnableTraceLog.Name) {
		cfg.XLayer.Monitor.EnableTraceLog = ctx.Bool(EnableTraceLog.Name)
	}
	if ctx.IsSet(TraceLogPath.Name) {
		cfg.XLayer.Monitor.TraceLogPath = ctx.String(TraceLogPath.Name)
	}
}

func setRealtimeXLayer(ctx *cli.Context, cfg *ethconfig.Config) {
	if ctx.IsSet(RealtimeKafkaSyncGroupID.Name) {
		cfg.XLayer.Realtime.Kafka.GroupID = ctx.String(RealtimeKafkaSyncGroupID.Name)
	}
	if envGroupID := os.Getenv(EnvKafkaConsumerGroupID); envGroupID != "" {
		// Override consumer group id if env variable is set
		cfg.XLayer.Realtime.Kafka.GroupID = envGroupID
	}
	if ctx.IsSet(RealtimeEnableFlag.Name) {
		cfg.XLayer.Realtime.Enable = ctx.Bool(RealtimeEnableFlag.Name)
	}
	if ctx.IsSet(RealtimeRpcFlag.Name) {
		cfg.XLayer.Realtime.RealtimeRpc = ctx.Bool(RealtimeRpcFlag.Name)
	}
	if ctx.IsSet(RealtimeEnableSubscribeFlag.Name) {
		cfg.XLayer.Realtime.EnableSubscribe = ctx.Bool(RealtimeEnableSubscribeFlag.Name)
	}
	if ctx.IsSet(RealtimeCacheHeightThreshold.Name) {
		cfg.XLayer.Realtime.CacheHeightThreshold = ctx.Uint64(RealtimeCacheHeightThreshold.Name)
	}
	if ctx.IsSet(RealtimeCacheDumpPath.Name) {
		cfg.XLayer.Realtime.CacheDumpPath = ctx.String(RealtimeCacheDumpPath.Name)
	}
	if ctx.IsSet(RealtimeSubscribeKafka.Name) {
		cfg.XLayer.Realtime.SubscribeKafka = ctx.Bool(RealtimeSubscribeKafka.Name)
	}
	if ctx.IsSet(RealtimeKafkaSyncBootstrapServers.Name) {
		cfg.XLayer.Realtime.Kafka.BootstrapServers = strings.Split(ctx.String(RealtimeKafkaSyncBootstrapServers.Name), ",")
	}
	if ctx.IsSet(RealtimeKafkaSyncBlockTopic.Name) {
		cfg.XLayer.Realtime.Kafka.BlockTopic = ctx.String(RealtimeKafkaSyncBlockTopic.Name)
	}
	if ctx.IsSet(RealtimeKafkaSyncTxTopic.Name) {
		cfg.XLayer.Realtime.Kafka.TxTopic = ctx.String(RealtimeKafkaSyncTxTopic.Name)
	}
	if ctx.IsSet(RealtimeKafkaSyncErrorTopic.Name) {
		cfg.XLayer.Realtime.Kafka.ErrorTopic = ctx.String(RealtimeKafkaSyncErrorTopic.Name)
	}
	if ctx.IsSet(RealtimeKafkaSyncClientID.Name) {
		cfg.XLayer.Realtime.Kafka.ClientID = ctx.String(RealtimeKafkaSyncClientID.Name)
	}
	if ctx.IsSet(RealtimeSubscribeWebsocket.Name) {
		cfg.XLayer.Realtime.SubscribeWebsocket = ctx.Bool(RealtimeSubscribeWebsocket.Name)
	}
	if ctx.IsSet(RealtimeStreamerUrl.Name) {
		cfg.XLayer.Realtime.WSConn.RealtimeStreamerUrl = ctx.String(RealtimeStreamerUrl.Name)
	}
	if ctx.IsSet(RealtimeStreamerUseTLS.Name) {
		cfg.XLayer.Realtime.WSConn.RealtimeStreamerUseTLS = ctx.Bool(RealtimeStreamerUseTLS.Name)
	}
	if ctx.IsSet(RealtimeStreamerTimeout.Name) {
		cfg.XLayer.Realtime.WSConn.RealtimeStreamerTimeout = ctx.Duration(RealtimeStreamerTimeout.Name)
	}

}

func setGPOXLayer(ctx *cli.Context, cfg *ethconfig.Config) {
	if ctx.IsSet(GpoDefault.Name) {
		cfg.GPO.XLayer.Default = big.NewInt(ctx.Int64(GpoDefault.Name))
	}
	if ctx.IsSet(GpoFactor.Name) {
		cfg.GPO.XLayer.Factor = ctx.Float64(GpoFactor.Name)
	}
	if ctx.IsSet(GpoCongestionThreshold.Name) {
		cfg.GPO.XLayer.CongestionThreshold = ctx.Int(GpoCongestionThreshold.Name)
	}
}

// SetApolloGPOXLayer is a public wrapper function to internally call setGPO
func SetApolloGPOXLayer(ctx *cli.Context, cfg *gasprice.Config) {
	setGPO(ctx, cfg)
}
