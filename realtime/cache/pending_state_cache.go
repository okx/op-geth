package cache

import (
	"bytes"
	"fmt"
	"sync"

	libcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	realtimeTypes "github.com/ethereum/go-ethereum/realtime/types"
)

// GlobalStateCache implements the plain state reader with a changeset cache layer.
// The pending cache holds the pending chainstate - it holds the global state cache,
// with a changeset cache layer that stores in-memory the pending state changes.
type PendingStateCache struct {
	globalCache *GlobalStateCache
	cacheLock   sync.RWMutex
	cache       *stateCache
}

func NewPendingStateCache(globalCache *GlobalStateCache, size int) *PendingStateCache {
	return &PendingStateCache{
		globalCache: globalCache,
		cache:       newStateCache(size),
	}
}

func (cache *PendingStateCache) ApplyChangeset(changeset *realtimeTypes.Changeset, blockNumber uint64, txIndex uint) error {
	cache.cacheLock.Lock()
	defer cache.cacheLock.Unlock()

	// Handle account data changes
	addressChanges := make(map[libcommon.Address]*types.StateAccount)
	cache.applyChangesetToAccountData(changeset, addressChanges)

	// Apply code changes
	for codeHash, code := range changeset.CodeChanges {
		cache.cache.codeCache[codeHash] = code
	}

	// Apply storage changes
	for address, storage := range changeset.StorageChanges {
		for key, value := range storage {
			compositeKey := GenerateCompositeStorageKey(address.Bytes(), key.Bytes())
			cache.cache.storageCache[string(compositeKey)] = value
		}
	}

	// Apply deleted accounts changes
	for address := range changeset.DeletedAccounts {
		// Non-existent / deleted accounts are set to nil
		addressChanges[address] = nil
	}

	// Apply account changes
	for address, account := range addressChanges {
		delete(cache.cache.accountCache, address)
		cache.cache.accountCache[address] = account
		log.Debug("[Realtime] ApplyChangeset: ", address)
	}

	log.Debug(fmt.Sprintf("[Realtime] Apply changeset from tx with height: %d, txIndex: %d\n", blockNumber, txIndex))

	return nil
}

func (cache *PendingStateCache) applyChangesetToAccountData(changeset *realtimeTypes.Changeset, addressChanges map[libcommon.Address]*types.StateAccount) (err error) {
	// Apply balance changes
	for address, balance := range changeset.BalanceChanges {
		if _, ok := changeset.DeletedAccounts[address]; ok {
			continue
		}

		account, err := cache.getOrCreateAccount(address, addressChanges)
		if err != nil {
			return fmt.Errorf("apply balance changes failed: %v", err)
		}
		account.Balance.Set(balance)
	}

	// Apply nonce changes
	for address, nonce := range changeset.NonceChanges {
		if _, ok := changeset.DeletedAccounts[address]; ok {
			continue
		}

		account, err := cache.getOrCreateAccount(address, addressChanges)
		if err != nil {
			return fmt.Errorf("apply nonce changes failed: %v", err)
		}
		account.Nonce = nonce
	}

	// Apply code hash changes
	for address, codeHash := range changeset.CodeHashChanges {
		if _, ok := changeset.DeletedAccounts[address]; ok {
			continue
		}

		account, err := cache.getOrCreateAccount(address, addressChanges)
		if err != nil {
			return fmt.Errorf("apply code hash changes failed: %v", err)
		}
		account.CodeHash = codeHash[:]
	}

	return nil
}

func (cache *PendingStateCache) getOrCreateAccount(address libcommon.Address, addressChanges map[libcommon.Address]*types.StateAccount) (*types.StateAccount, error) {
	account, ok := addressChanges[address]
	if !ok {
		var err error
		account, err = cache.unsafeReadAccountData(address)
		if err != nil {
			return nil, err
		}

		if account == nil {
			// Non-existent account, create new account
			account = cache.createAccount()
		}
		addressChanges[address] = account
	}

	return account, nil
}

func (cache *PendingStateCache) unsafeReadAccountData(address libcommon.Address) (*types.StateAccount, error) {
	acc, ok := cache.cache.accountCache[address]
	if ok {
		return acc, nil
	}

	// Cache miss, read from global cache
	return cache.globalCache.Account(address)
}

func (cache *PendingStateCache) createAccount() *types.StateAccount {
	return types.NewEmptyStateAccount()
}

// -------------- StateReader implementation --------------
func (cache *PendingStateCache) Account(addr libcommon.Address) (*types.StateAccount, error) {
	cache.cacheLock.RLock()
	acc, ok := cache.cache.accountCache[addr]
	if ok {
		accCopy := acc.Copy()
		cache.cacheLock.RUnlock()
		return accCopy, nil
	}
	cache.cacheLock.RUnlock()

	// Cache miss, read from global cache
	return cache.globalCache.Account(addr)
}

func (cache *PendingStateCache) Storage(addr libcommon.Address, slot libcommon.Hash) (libcommon.Hash, error) {
	compositeKey := GenerateCompositeStorageKey(addr.Bytes(), slot.Bytes())

	cache.cacheLock.RLock()
	storage, ok := cache.cache.storageCache[string(compositeKey)]
	if ok {
		cache.cacheLock.RUnlock()
		return storage, nil
	}
	cache.cacheLock.RUnlock()

	// Cache miss, read from global cache
	return cache.globalCache.Storage(addr, slot)
}

func (cache *PendingStateCache) Code(addr libcommon.Address, codeHash libcommon.Hash) ([]byte, error) {
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

	// Cache miss, read from global cache
	return cache.globalCache.Code(addr, codeHash)
}

func (cache *PendingStateCache) CodeSize(addr libcommon.Address, codeHash libcommon.Hash) (int, error) {
	code, err := cache.Code(addr, codeHash)
	return len(code), err
}
