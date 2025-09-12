package utils

import (
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/realtime"
	"github.com/ethereum/go-ethereum/realtime/kafka"
	"github.com/urfave/cli/v2"
)

const EnvKafkaConsumerGroupID = "REALTIME_KAFKA_CONSUMER_GROUP_ID"

var (
	// OkPay
	OkPayPriorityEnableFlag = &cli.BoolFlag{
		Name:  "okpay.priority-enable-flag",
		Usage: "OkPay",
		Value: false,
	}
	OkPaySenderAccountsList = &cli.StringFlag{
		Name:  "okpay.sender-accounts-list",
		Usage: "List of OkPay sender accounts",
		Value: "",
	}
	OkPayBlockPriorityTxsLimit = &cli.Uint64Flag{
		Name:  "okpay.block-priority-txs-limit",
		Usage: "Max number of OkPay txs that we will prioritize per block",
		Value: 0,
	}
	// InnerTx
	InnerTxFlag = &cli.BoolFlag{
		Name:  "innertx",
		Usage: "Enable inner transaction capture and storage (disabled by default)",
		Value: false,
	}
	// Realtime feature
	RealtimeEnableFlag = &cli.BoolFlag{
		Name:  "realtime.enable-flag",
		Usage: "Kafka sync enable flag",
		Value: true,
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
	RealtimeCacheDumpPath = &cli.StringFlag{
		Name:  "realtime.cache-dump-path",
		Usage: "Cache dump path",
		Value: "/home/erigon/data/cache",
	}

	// XLayerFlags are the default flags for X Layer features
	XLayerFlags = []cli.Flag{
		OkPayPriorityEnableFlag,
		OkPaySenderAccountsList,
		OkPayBlockPriorityTxsLimit,
		InnerTxFlag,
		RealtimeEnableFlag,
		RealtimeEnableSubscribeFlag,
		RealtimeCacheHeightThreshold,
		RealtimeKafkaSyncBootstrapServers,
		RealtimeKafkaSyncBlockTopic,
		RealtimeKafkaSyncTxTopic,
		RealtimeKafkaSyncErrorTopic,
		RealtimeKafkaSyncClientID,
		RealtimeKafkaSyncGroupID,
		RealtimeCacheDumpPath,
	}
)

func SetXLayerConfig(ctx *cli.Context, cfg *ethconfig.Config) {
	setOkPayXLayer(ctx, cfg)
	setInnerTxXLayer(ctx, cfg)
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

func setRealtimeXLayer(ctx *cli.Context, cfg *ethconfig.Config) {
	// For realtime. Get GroupID from flag
	groupID := ctx.String(RealtimeKafkaSyncGroupID.Name)
	if envGroupID := os.Getenv(EnvKafkaConsumerGroupID); envGroupID != "" {
		// Override consumer group id if env variable is set
		groupID = envGroupID
	}

	cfg.XLayer = ethconfig.XLayerConfig{
		Realtime: realtime.RealtimeConfig{
			Enable:               ctx.Bool(RealtimeEnableFlag.Name),
			EnableSubscribe:      ctx.Bool(RealtimeEnableSubscribeFlag.Name),
			CacheHeightThreshold: ctx.Uint64(RealtimeCacheHeightThreshold.Name),
			CacheDumpPath:        ctx.String(RealtimeCacheDumpPath.Name),
			Kafka: kafka.KafkaConfig{
				BootstrapServers: strings.Split(ctx.String(RealtimeKafkaSyncBootstrapServers.Name), ","),
				BlockTopic:       ctx.String(RealtimeKafkaSyncBlockTopic.Name),
				TxTopic:          ctx.String(RealtimeKafkaSyncTxTopic.Name),
				ErrorTopic:       ctx.String(RealtimeKafkaSyncErrorTopic.Name),
				ClientID:         ctx.String(RealtimeKafkaSyncClientID.Name),
				GroupID:          groupID,
			},
		},
	}
}

func setInnerTxXLayer(ctx *cli.Context, cfg *ethconfig.Config) {
	if ctx.IsSet(InnerTxFlag.Name) {
		cfg.EnableInnerTx = ctx.Bool(InnerTxFlag.Name)
	}
}
