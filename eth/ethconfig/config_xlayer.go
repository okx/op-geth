package ethconfig

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/realtime"
	"github.com/ethereum/go-ethereum/realtime/kafka"
)

var DefaultXLayerConfig = XLayerConfig{
	IsSequencer: false,
	Realtime: realtime.RealtimeConfig{
		Enable:               false,
		EnableSubscribe:      false,
		CacheHeightThreshold: 10,
		Kafka:                kafka.KafkaConfig{},
		CacheDumpPath:        "",
	},
	OkPay: OkPayConfig{
		PriorityEnable:        false,
		SenderAccountsList:    []common.Address{},
		BlockPriorityTxsLimit: 0,
	},
}

// XLayerConfig is the X Layer config used on the eth backend
type XLayerConfig struct {
	IsSequencer bool                    `toml:",omitempty"`
	Realtime    realtime.RealtimeConfig `toml:",omitempty"`
	OkPay       OkPayConfig             `toml:",omitempty"`
}

type OkPayConfig struct {
	PriorityEnable bool `toml:",omitempty"`
	// SenderAccountsList is the list of OkX Pay sender accounts
	SenderAccountsList []common.Address
	// BlockPriorityTxsLimit is the max number of OkX Pay txs that we will prioritize per block
	BlockPriorityTxsLimit uint64
}
