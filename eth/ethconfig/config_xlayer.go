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
