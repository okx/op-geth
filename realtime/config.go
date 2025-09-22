package realtime

import (
	"github.com/ethereum/go-ethereum/realtime/kafka"
	"github.com/ethereum/go-ethereum/realtime/relayer/streamclient"
)

type RealtimeConfig struct {
	Enable               bool                            `toml:",omitempty"`
	RealtimeRpc          bool                            `toml:",omitempty"`
	EnableSubscribe      bool                            `toml:",omitempty"`
	CacheHeightThreshold uint64                          `toml:",omitempty"`
	SubscribeKafka       bool                            `toml:",omitempty"`
	SubscribeWebsocket   bool                            `toml:",omitempty"`
	Kafka                kafka.KafkaConfig               `toml:",omitempty"`
	WSConn               streamclient.StreamClientConfig `toml:",omitempty"`
	CacheDumpPath        string                          `toml:",omitempty"`
}
