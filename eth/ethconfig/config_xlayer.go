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
		TraceLogPath:   "", // Empty by default
	},
}

// XLayerConfig is the X Layer config used on the eth backend
type XLayerConfig struct {
	LegacyPp MigrationConfig `toml:",omitempty"` // The erigon RPC endpoint URL for pre-migration blocks
	Monitor       MonitorConfig   `toml:",omitempty"` // Transaction monitoring configuration
}

type MigrationConfig struct {
	MigrationBlock *uint64       `toml:",omitempty"` // Block height threshold for migration routing
	PPRPCUrl       string        `toml:",omitempty"` // XLayer-Erigon RPC endpoint URL
	PPRPCTimeout   time.Duration `toml:",omitempty"` // Timeout for PP RPC calls (default: 10s)
}

// MonitorConfig contains configuration for transaction monitoring
type MonitorConfig struct {
	EnableTraceLog bool   `toml:",omitempty"`
	TraceLogPath   string `toml:",omitempty"`
}
