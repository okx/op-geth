package realtime

import "github.com/ethereum/go-ethereum/realtime/kafka"

type RealtimeConfig struct {
	Enable               bool              `toml:",omitempty"`
	EnableSubscribe      bool              `toml:",omitempty"`
	CacheHeightThreshold uint64            `toml:",omitempty"`
	Kafka                kafka.KafkaConfig `toml:",omitempty"`
	CacheDumpPath        string            `toml:",omitempty"`
}
