package cache

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
