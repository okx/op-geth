package subscription

import "github.com/ethereum/go-ethereum/common"

type StreamCriteria struct {
	NewHeads             bool
	TransactionExtraInfo bool
	TransactionReceipt   bool
	TransactionInnerTxs  bool
	SubscribedAddresses  []common.Address
}
