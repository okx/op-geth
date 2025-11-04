package ethconfig

import (
	"time"

	"github.com/ethereum/go-ethereum/common"
)

var DefaultXLayerConfig = XLayerConfig{
	EnableInnerTx: true,
	OkPay: OkPayConfig{
		PriorityEnable:        false,
		SenderAccountsList:    []common.Address{},
		BlockPriorityTxsLimit: 0,
	},
	Apollo: ApolloConfig{
		Enable:        false,
		AppID:         "",
		IP:            "",
		Cluster:       "",
		NamespaceName: "",
	},
	LegacyPp: MigrationConfig{
		MigrationBlock: nil,
		PPRPCUrl:       "",
		PPRPCTimeout:   0,
	},
	Monitor: MonitorConfig{
		EnableTraceLog: false,
		TraceLogPath:   "/var/log/op-geth/trace.log",
	},
}

// XLayerConfig is the X Layer config used on the eth backend
type XLayerConfig struct {
	EnableInnerTx bool            `toml:",omitempty"`
	OkPay         OkPayConfig     `toml:",omitempty"`
	Apollo        ApolloConfig    `toml:",omitempty"`
	LegacyPp      MigrationConfig `toml:",omitempty"` // The erigon RPC endpoint URL for pre-migration blocks
	Monitor       MonitorConfig   `toml:",omitempty"` // Transaction monitoring configuration
}

type MigrationConfig struct {
	MigrationBlock *uint64       `toml:",omitempty"` // Block height threshold for migration routing
	PPRPCUrl       string        `toml:",omitempty"` // XLayer-Erigon RPC endpoint URL
	PPRPCTimeout   time.Duration `toml:",omitempty"` // Timeout for PP RPC calls (default: 10s)
}

type OkPayConfig struct {
	PriorityEnable bool `toml:",omitempty"`
	// SenderAccountsList is the list of OkX Pay sender accounts
	SenderAccountsList []common.Address
	// BlockPriorityTxsLimit is the max number of OkX Pay txs that we will prioritize per block
	BlockPriorityTxsLimit uint64
}

// MonitorConfig contains configuration for transaction monitoring
type MonitorConfig struct {
	EnableTraceLog bool   `toml:",omitempty"`
	TraceLogPath   string `toml:",omitempty"`
}

type ApolloConfig struct {
	// Enable Apollo service
	Enable bool `toml:",omitempty"`
	// Apollo app ID
	AppID string `toml:",omitempty"`
	// Apollo server endpoint
	IP string `toml:",omitempty"`
	// Apollo cluster name
	Cluster string `toml:",omitempty"`
	// Apollo namespace
	NamespaceName string `toml:",omitempty"`
}
