package utils

import (
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/realtime"
	"github.com/ethereum/go-ethereum/realtime/kafka"
	"github.com/urfave/cli/v2"
)

const EnvKafkaConsumerGroupID = "REALTIME_KAFKA_CONSUMER_GROUP_ID"

var (
	// For realtime features
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
