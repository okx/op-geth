package ethconfig

import "github.com/ethereum/go-ethereum/common"

// XLayerConfig is the X Layer config used on the eth backend
type XLayerConfig struct {
	OkPay OkPayConfig `toml:",omitempty"`
}

type OkPayConfig struct {
	PriorityEnable bool `toml:",omitempty"`
	// SenderAccountsList is the list of OkX Pay sender accounts
	SenderAccountsList []common.Address
	// BlockPriorityTxsLimit is the max number of OkX Pay txs that we will prioritize per block
	BlockPriorityTxsLimit uint64
}
