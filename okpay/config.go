package okpay

import libcommon "github.com/ethereum/go-ethereum/common"

type OkPayConfig struct {
	Enable bool `toml:",omitempty"`
	// OkPaySenderAccountsList is the list of OkPay sender accounts
	OkPaySenderAccountsList libcommon.OrderedList[libcommon.Address]
	// OkPayBlockPriorityTxsLimit is the max number of OkPay txs that we will prioritize per block
	OkPayBlockPriorityTxsLimit uint64
}
