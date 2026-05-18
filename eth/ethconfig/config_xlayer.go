package ethconfig

import (
	"time"
)

var DefaultXLayerConfig = XLayerConfig{
	LegacyPp: MigrationConfig{
		MigrationBlock: nil,
		PPRPCUrl:       "",
		PPRPCTimeout:   0,
	},
	Monitor: MonitorConfig{
		EnableTraceLog: false,
		TraceLogPath:   "",
	},
	P2P: P2PConfig{
		ETH69Compat: true,
	},
}

// XLayerConfig is the X Layer config used on the eth backend
type XLayerConfig struct {
	LegacyPp MigrationConfig `toml:",omitempty"`
	Monitor  MonitorConfig   `toml:",omitempty"`
	P2P      P2PConfig       `toml:",omitempty"`
}

// P2PConfig contains P2P-layer configuration for XLayer compatibility shims.
type P2PConfig struct {
	ETH69Compat bool `toml:",omitempty"`
}

type MigrationConfig struct {
	MigrationBlock *uint64       `toml:",omitempty"`
	PPRPCUrl       string        `toml:",omitempty"`
	PPRPCTimeout   time.Duration `toml:",omitempty"`
}

// MonitorConfig contains configuration for transaction monitoring
type MonitorConfig struct {
	EnableTraceLog bool   `toml:",omitempty"`
	TraceLogPath   string `toml:",omitempty"`
}
