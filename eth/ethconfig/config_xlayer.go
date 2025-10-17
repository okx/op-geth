package ethconfig

import (
	"time"

	"github.com/ethereum/go-ethereum/common"
)

// XLayerConfig is the X Layer config used on the eth backend
type XLayerConfig struct {
	OkPay    OkPayConfig     `toml:",omitempty"`
	Apollo   ApolloConfig    `toml:",omitempty"`
	LegacyPp MigrationConfig `toml:",omitempty"` // The erigon RPC endpoint URL for pre-migration blocks
	P2P      P2PConfig       `toml:",omitempty"` // P2P configuration for XLayer
}

type MigrationConfig struct {
	MigrationBlock                 *uint64       `toml:",omitempty"` // Block height threshold for migration routing
	PPRPCUrl                       string        `toml:",omitempty"` // XLayer-Erigon RPC endpoint URL
	PPRPCTimeout                   time.Duration `toml:",omitempty"` // Timeout for PP RPC calls (default: 10s)
	PPRPCLegacyHeaderSyncRateLimit int           `toml:",omitempty"` // Rate limit for legacy header sync in QPS (default: 80)
}

type OkPayConfig struct {
	PriorityEnable bool `toml:",omitempty"`
	// SenderAccountsList is the list of OkX Pay sender accounts
	SenderAccountsList []common.Address
	// BlockPriorityTxsLimit is the max number of OkX Pay txs that we will prioritize per block
	BlockPriorityTxsLimit uint64
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

// P2PConfig contains P2P-specific configuration for XLayer
type P2PConfig struct {
	// MaxQueuedTxs is the maximum number of transactions to queue up before dropping older broadcasts
	MaxQueuedTxs uint64 `toml:",omitempty"`
	// MaxQueuedTxAnns is the maximum number of transaction announcements to queue up before dropping older announcements
	MaxQueuedTxAnns uint64 `toml:",omitempty"`
}
