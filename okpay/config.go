package okpay

import "github.com/ethereum/go-ethereum/common"

type OkPayConfig struct {
	Enable bool `toml:",omitempty"`
	// OkPaySenderAccountsList is the list of OkPay sender accounts
	OkPaySenderAccountsList []common.Address
	// OkPayBlockPriorityTxsLimit is the max number of OkPay txs that we will prioritize per block
	OkPayBlockPriorityTxsLimit uint64
}
