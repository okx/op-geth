package cache

import (
	"bytes"
	"context"
	"fmt"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	realtimeTypes "github.com/ethereum/go-ethereum/realtime/types"
)

const DefaultPlainStateCacheSize = 1_000

// BlockStateCache is a double-linked list that implements the plain state reader
// with a changeset cache layer. The block state cache holds the block chainstate,
// and the previous block state cache reader.
type BlockStateCache struct {
	// Blockchain backend
	ctx        context.Context
	blockchain *core.BlockChain

	// Cache
	cacheLock  sync.RWMutex
	cache      *plainStateCache
	height     uint64
	prevHeight uint64

	// Double-linked list holding previous and next block state caches
	prevCache *BlockStateCache
	nextCache *BlockStateCache
}

func NewBlockStateCache(ctx context.Context, blockchain *core.BlockChain, height uint64) *BlockStateCache {
	return &BlockStateCache{
		ctx:        ctx,
		blockchain: blockchain,
		height:     height,
		prevHeight: height - 1,
		cache:      newPlainStateCache(DefaultPlainStateCacheSize),
		nextCache:  nil,
		prevCache:  nil,
	}
}

// -------------- Block and linked list operations --------------
func (cache *BlockStateCache) GetNextBlockCache() *BlockStateCache {
	cache.cacheLock.RLock()
	defer cache.cacheLock.RUnlock()
	return cache.nextCache
}

func (cache *BlockStateCache) SetNextBlockCache(nextBlockCache *BlockStateCache) {
	cache.cacheLock.Lock()
	defer cache.cacheLock.Unlock()
	cache.nextCache = nextBlockCache
}

func (cache *BlockStateCache) GetPrevBlockCache() *BlockStateCache {
	cache.cacheLock.RLock()
	defer cache.cacheLock.RUnlock()
	return cache.prevCache
}

func (cache *BlockStateCache) SetPrevBlockCache(prevBlockCache *BlockStateCache) {
	cache.cacheLock.Lock()
	defer cache.cacheLock.Unlock()
	cache.prevCache = prevBlockCache
}

func (cache *BlockStateCache) Clear() {
	cache.cacheLock.Lock()
	defer cache.cacheLock.Unlock()

	// Clear the plain state cache
	cache.cache.Clear()

	// Clear linked list references to prevent circular references
	cache.nextCache = nil
	cache.prevCache = nil
}

// -------------- State apply operations --------------
func (cache *BlockStateCache) ApplyChangeset(changeset *realtimeTypes.Changeset, blockNumber uint64) error {
	cache.cacheLock.Lock()
	defer cache.cacheLock.Unlock()

	// Handle account data changes
	addressChanges := make(map[common.Address]*types.StateAccount)
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
		log.Debug(fmt.Sprintf("[Realtime] ApplyChangeset: %s", address))
	}

	return nil
}

func (cache *BlockStateCache) applyChangesetToAccountData(changeset *realtimeTypes.Changeset, addressChanges map[common.Address]*types.StateAccount) (err error) {
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

func (cache *BlockStateCache) getOrCreateAccount(address common.Address, addressChanges map[common.Address]*types.StateAccount) (*types.StateAccount, error) {
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

func (cache *BlockStateCache) unsafeReadAccountData(address common.Address) (*types.StateAccount, error) {
	acc, ok := cache.cache.accountCache[address]
	if ok {
		return acc, nil
	}

	// Cache miss
	if cache.prevCache == nil {
		reader, err := cache.GetDbStateReader()
		if err != nil {
			return nil, err
		}
		return reader.Account(address)
	}
	return cache.prevCache.Account(address)
}

func (cache *BlockStateCache) createAccount() *types.StateAccount {
	return types.NewEmptyStateAccount()
}

// -------------- StateReader implementation --------------
func (cache *BlockStateCache) Account(addr common.Address) (*types.StateAccount, error) {
	cache.cacheLock.RLock()
	defer cache.cacheLock.RUnlock()
	acc, ok := cache.cache.accountCache[addr]
	if ok {
		accCopy := acc.Copy()
		return accCopy, nil
	}

	// Cache miss
	if cache.prevCache == nil {
		reader, err := cache.GetDbStateReader()
		if err != nil {
			return nil, err
		}
		return reader.Account(addr)
	}
	return cache.prevCache.Account(addr)
}

func (cache *BlockStateCache) Storage(addr common.Address, slot common.Hash) (common.Hash, error) {
	compositeKey := GenerateCompositeStorageKey(addr.Bytes(), slot.Bytes())
	cache.cacheLock.RLock()
	defer cache.cacheLock.RUnlock()

	storage, ok := cache.cache.storageCache[string(compositeKey)]
	if ok {
		return storage, nil
	}

	// Cache miss
	if cache.prevCache == nil {
		reader, err := cache.GetDbStateReader()
		if err != nil {
			return common.Hash{}, err
		}
		return reader.Storage(addr, slot)
	}
	return cache.prevCache.Storage(addr, slot)
}

func (cache *BlockStateCache) Code(addr common.Address, codeHash common.Hash) ([]byte, error) {
	if bytes.Equal(codeHash.Bytes(), types.EmptyCodeHash[:]) {
		return nil, nil
	}
	cache.cacheLock.RLock()
	defer cache.cacheLock.RUnlock()

	code, ok := cache.cache.codeCache[codeHash]
	if ok {
		return code, nil
	}
	// Cache miss
	if cache.prevCache == nil {
		reader, err := cache.GetDbStateReader()
		if err != nil {
			return nil, err
		}
		return reader.Code(addr, codeHash)
	}
	return cache.prevCache.Code(addr, codeHash)
}

func (cache *BlockStateCache) CodeSize(addr common.Address, codeHash common.Hash) (int, error) {
	code, err := cache.Code(addr, codeHash)
	return len(code), err
}

func (cache *BlockStateCache) GetDbStateReader() (state.Reader, error) {
	prevRoot := cache.blockchain.GetHeaderByNumber(cache.prevHeight).Root
	reader, err := cache.blockchain.StateCache().Reader(prevRoot)
	if err != nil {
		return nil, err
	}
	return reader, nil
}
