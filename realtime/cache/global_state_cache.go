package cache

import (
	"bytes"
	"context"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
)

// GlobalStateCache implements the plain state reader with a changeset cache layer.
// The global cache holds the latest chainstate - it holds the chainstate db,
// with a changeset cache layer that stores in-memory the latest state changes.
type GlobalStateCache struct {
	ctx context.Context
	db  state.Database

	cacheLock  sync.RWMutex
	latestRoot common.Hash
	cache      *plainStateCache
}

func NewGlobalStateCache(ctx context.Context, db state.Database, size int) *GlobalStateCache {
	return &GlobalStateCache{
		ctx:   ctx,
		db:    db,
		cache: newPlainStateCache(size),
	}
}

func (cache *GlobalStateCache) Clear() {
	cache.cacheLock.Lock()
	defer cache.cacheLock.Unlock()

	// Clear all caches
	cache.cache.Clear()
}

func (cache *GlobalStateCache) FlushState(incoming *plainStateCache) error {
	cache.cacheLock.Lock()
	defer cache.cacheLock.Unlock()

	cache.cache.Flatten(incoming)
	return nil
}

func (cache *GlobalStateCache) UpdateLatestRoot(latestRoot common.Hash) {
	cache.cacheLock.Lock()
	defer cache.cacheLock.Unlock()
	cache.latestRoot = latestRoot
}

func (cache *GlobalStateCache) GetLatestRoot() common.Hash {
	cache.cacheLock.RLock()
	defer cache.cacheLock.RUnlock()
	return cache.latestRoot
}

// -------------- StateReader implementation --------------
func (cache *GlobalStateCache) Account(addr common.Address) (*types.StateAccount, error) {
	cache.cacheLock.RLock()
	acc, ok := cache.cache.accountCache[addr]
	if ok {
		accCopy := acc.Copy()
		cache.cacheLock.RUnlock()
		return accCopy, nil
	}
	cache.cacheLock.RUnlock()

	// Cache miss, read from chainstate db
	return cache.GetAccountFromChainDb(addr)
}

func (cache *GlobalStateCache) Storage(addr common.Address, slot common.Hash) (common.Hash, error) {
	compositeKey := GenerateCompositeStorageKey(addr.Bytes(), slot.Bytes())

	cache.cacheLock.RLock()
	storage, ok := cache.cache.storageCache[string(compositeKey)]
	if ok {
		cache.cacheLock.RUnlock()
		return storage, nil
	}
	cache.cacheLock.RUnlock()

	// Cache miss, read from chainstate db
	return cache.GetAccountStorageFromChainDb(addr, slot)
}

func (cache *GlobalStateCache) Code(addr common.Address, codeHash common.Hash) ([]byte, error) {
	if bytes.Equal(codeHash.Bytes(), types.EmptyCodeHash[:]) {
		return nil, nil
	}

	cache.cacheLock.RLock()
	code, ok := cache.cache.codeCache[codeHash]
	if ok {
		cache.cacheLock.RUnlock()
		return code, nil
	}
	cache.cacheLock.RUnlock()

	// Cache miss, read from chainstate db
	return cache.GetAccountCodeFromChainDb(addr, codeHash)
}

func (cache *GlobalStateCache) CodeSize(addr common.Address, codeHash common.Hash) (int, error) {
	code, err := cache.Code(addr, codeHash)
	return len(code), err
}

// -------------- Chainstate db reader operations --------------
func (cache *GlobalStateCache) GetAccountFromChainDb(address common.Address) (*types.StateAccount, error) {
	reader, err := cache.db.Reader(cache.latestRoot)
	if err != nil {
		return nil, err
	}
	return reader.Account(address)
}

func (cache *GlobalStateCache) GetAccountStorageFromChainDb(address common.Address, key common.Hash) (common.Hash, error) {
	reader, err := cache.db.Reader(cache.latestRoot)
	if err != nil {
		return common.Hash{}, err
	}
	return reader.Storage(address, key)
}

func (cache *GlobalStateCache) GetAccountCodeFromChainDb(address common.Address, codeHash common.Hash) ([]byte, error) {
	reader, err := cache.db.Reader(cache.latestRoot)
	if err != nil {
		return nil, err
	}
	return reader.Code(address, codeHash)
}

func (cache *GlobalStateCache) GetAccountCodeSizeFromChainDb(address common.Address, codeHash common.Hash) (int, error) {
	reader, err := cache.db.Reader(cache.latestRoot)
	if err != nil {
		return 0, err
	}
	return reader.CodeSize(address, codeHash)
}
