package cache

import (
	"github.com/ethereum/go-ethereum/core/types"
	realtimeTypes "github.com/ethereum/go-ethereum/realtime/types"
)

const (
	AddrLength = 20
	HashLength = 32
)

// AddrHash + KeyHash for contract storage
func GenerateCompositeStorageKey(address []byte, key []byte) []byte {
	compositeKey := make([]byte, AddrLength+HashLength)
	copy(compositeKey, address)
	copy(compositeKey[AddrLength:], key)
	return compositeKey
}

func newReceiptsList(size int) *realtimeTypes.OrderedList[*types.Receipt] {
	return realtimeTypes.NewOrderedList(size, func(a, b *types.Receipt) int {
		return int(a.TransactionIndex) - int(b.TransactionIndex)
	})
}
