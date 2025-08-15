package ethconfig

import "github.com/ethereum/go-ethereum/realtime"

// XLayerConfig is the X Layer config used on the eth backend
type XLayerConfig struct {
	Realtime    realtime.RealtimeConfig `toml:",omitempty"`
	IsSequencer bool                    `toml:",omitempty"`
}
