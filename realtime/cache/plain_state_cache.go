package cache

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type plainStateCache struct {
	accountCache map[common.Address]*types.StateAccount
	storageCache map[string]common.Hash
	codeCache    map[common.Hash][]byte
}

func newPlainStateCache(size int) *plainStateCache {
	return &plainStateCache{
		accountCache: make(map[common.Address]*types.StateAccount, size),
		storageCache: make(map[string]common.Hash, size),
		codeCache:    make(map[common.Hash][]byte, size),
	}
}

func (cache *plainStateCache) Clear() {
	for k := range cache.accountCache {
		delete(cache.accountCache, k)
	}
	for k := range cache.storageCache {
		delete(cache.storageCache, k)
	}
	for k := range cache.codeCache {
		delete(cache.codeCache, k)
	}
}

func (cache *plainStateCache) Flatten(incoming *plainStateCache) {
	// Apply account changes
	for address, account := range incoming.accountCache {
		delete(cache.accountCache, address)
		cache.accountCache[address] = account
	}

	// Apply code changes
	for codeHash, code := range incoming.codeCache {
		delete(cache.codeCache, codeHash)
		cache.codeCache[codeHash] = code
	}

	// Apply storage changes
	for key, value := range incoming.storageCache {
		delete(cache.storageCache, key)
		cache.storageCache[key] = value
	}
}
