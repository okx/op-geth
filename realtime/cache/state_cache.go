package cache

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"

	libcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/log"
	realtimeTypes "github.com/ethereum/go-ethereum/realtime/types"
)

var (
	ErrNotReady = fmt.Errorf("state cache not initialized")
)

type stateCache struct {
	accountCache map[libcommon.Address]*types.StateAccount
	storageCache map[string]libcommon.Hash
	codeCache    map[libcommon.Hash][]byte
}

func newStateCache(size int) *stateCache {
	return &stateCache{
		accountCache: make(map[libcommon.Address]*types.StateAccount, size),
		storageCache: make(map[string]libcommon.Hash, size),
		codeCache:    make(map[libcommon.Hash][]byte, size),
	}
}

func (cache *stateCache) Clear() {
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

// GlobalStateCache implements the plain state reader with a changeset cache layer.
// The global cache holds the latest chainstate - it holds the chainstate db,
// with a changeset cache layer that stores in-memory the latest state changes.
type GlobalStateCache struct {
	ctx        context.Context
	db         state.Database
	initHeight atomic.Uint64

	cacheLock  sync.RWMutex
	latestRoot libcommon.Hash
	cache      *stateCache
}

func NewGlobalStateCache(ctx context.Context, db state.Database, size int) (*GlobalStateCache, error) {
	return &GlobalStateCache{
		ctx:        ctx,
		db:         db,
		initHeight: atomic.Uint64{},
		cache:      newStateCache(size),
	}, nil
}

func (cache *GlobalStateCache) TryInitCache(executionHeight uint64) error {
	cache.cacheLock.Lock()
	defer cache.cacheLock.Unlock()

	if cache.initHeight.Load() != 0 {
		return fmt.Errorf("state cache already initialized")
	}
	cache.initHeight.Store(executionHeight)

	return nil
}

func (cache *GlobalStateCache) UpdateLatestRoot(latestRoot libcommon.Hash) {
	cache.cacheLock.Lock()
	defer cache.cacheLock.Unlock()
	cache.latestRoot = latestRoot
}

func (cache *GlobalStateCache) GetLatestRoot() libcommon.Hash {
	cache.cacheLock.RLock()
	defer cache.cacheLock.RUnlock()
	return cache.latestRoot
}

func (cache *GlobalStateCache) Clear() {
	cache.cacheLock.Lock()
	defer cache.cacheLock.Unlock()

	// Clear all caches
	cache.cache.Clear()
	cache.initHeight.Store(0)
}

func (cache *GlobalStateCache) GetInitHeight() uint64 {
	return cache.initHeight.Load()
}

// -------------- Cache operations --------------
func (cache *GlobalStateCache) FlushState(stateCache *stateCache) error {
	cache.cacheLock.Lock()
	defer cache.cacheLock.Unlock()

	// Apply account changes
	for address, account := range stateCache.accountCache {
		delete(cache.cache.accountCache, address)
		cache.cache.accountCache[address] = account
	}

	// Apply code changes
	for codeHash, code := range stateCache.codeCache {
		delete(cache.cache.codeCache, codeHash)
		cache.cache.codeCache[codeHash] = code
	}

	// Apply storage changes
	for key, value := range stateCache.storageCache {
		delete(cache.cache.storageCache, key)
		cache.cache.storageCache[key] = value
	}

	return nil
}

// -------------- StateReader implementation --------------
func (cache *GlobalStateCache) Account(addr libcommon.Address) (*types.StateAccount, error) {
	if cache.initHeight.Load() == 0 {
		return nil, ErrNotReady
	}

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

func (cache *GlobalStateCache) Storage(addr libcommon.Address, slot libcommon.Hash) (libcommon.Hash, error) {
	if cache.initHeight.Load() == 0 {
		return libcommon.Hash{}, ErrNotReady
	}

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

func (cache *GlobalStateCache) Code(addr libcommon.Address, codeHash libcommon.Hash) ([]byte, error) {
	if bytes.Equal(codeHash.Bytes(), types.EmptyCodeHash[:]) {
		return nil, nil
	}

	if cache.initHeight.Load() == 0 {
		return nil, ErrNotReady
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

func (cache *GlobalStateCache) CodeSize(addr libcommon.Address, codeHash libcommon.Hash) (int, error) {
	code, err := cache.Code(addr, codeHash)
	return len(code), err
}

// -------------- Chainstate db reader operations --------------
func (cache *GlobalStateCache) GetAccountFromChainDb(address libcommon.Address) (*types.StateAccount, error) {
	reader, err := cache.db.Reader(cache.latestRoot)
	if err != nil {
		return nil, err
	}
	return reader.Account(address)
}

func (cache *GlobalStateCache) GetAccountStorageFromChainDb(address libcommon.Address, key libcommon.Hash) (libcommon.Hash, error) {
	reader, err := cache.db.Reader(cache.latestRoot)
	if err != nil {
		return libcommon.Hash{}, err
	}
	return reader.Storage(address, key)
}

func (cache *GlobalStateCache) GetAccountCodeFromChainDb(address libcommon.Address, codeHash libcommon.Hash) ([]byte, error) {
	reader, err := cache.db.Reader(cache.latestRoot)
	if err != nil {
		return nil, err
	}
	return reader.Code(address, codeHash)
}

func (cache *GlobalStateCache) GetAccountCodeSizeFromChainDb(address libcommon.Address, codeHash libcommon.Hash) (int, error) {
	reader, err := cache.db.Reader(cache.latestRoot)
	if err != nil {
		return 0, err
	}
	return reader.CodeSize(address, codeHash)
}

// -------------- Debug operations --------------
func (cache *GlobalStateCache) DebugDumpToFile(cacheDumpPath string) error {
	cache.cacheLock.RLock()
	defer cache.cacheLock.RUnlock()

	accountData := make(map[string]string)
	for addr, acc := range cache.cache.accountCache {
		value := types.SlimAccountRLP(*acc)
		accountData[hex.EncodeToString(addr[:])] = hex.EncodeToString(value)
	}
	if err := realtimeTypes.WriteToJSON(filepath.Join(cacheDumpPath, "account_cache.json"), accountData); err != nil {
		return fmt.Errorf("failed to dump account cache: %v", err)
	}

	storageData := make(map[string]string)
	for key, value := range cache.cache.storageCache {
		storageData[hex.EncodeToString([]byte(key))] = hex.EncodeToString(value.Bytes())
	}
	if err := realtimeTypes.WriteToJSON(filepath.Join(cacheDumpPath, "storage_cache.json"), storageData); err != nil {
		return fmt.Errorf("failed to dump storage cache: %v", err)
	}

	codeData := make(map[string]string)
	for hash, code := range cache.cache.codeCache {
		codeData[hex.EncodeToString(hash[:])] = hex.EncodeToString(code)
	}
	if err := realtimeTypes.WriteToJSON(filepath.Join(cacheDumpPath, "code_cache.json"), codeData); err != nil {
		return fmt.Errorf("failed to dump code cache: %v", err)
	}

	return nil
}

// DebugCompare compares the state cache with the chain-state db, and returns the
// list of account addresses that have differing states.
func (cache *GlobalStateCache) DebugCompare(statedb vm.StateDB) []string {
	cache.cacheLock.RLock()
	defer cache.cacheLock.RUnlock()

	mismatches := []string{}
	for addr, accCache := range cache.cache.accountCache {
		log.Info(fmt.Sprintf("[Realtime] Comparing account address: %s", addr.String()))

		accDbNonce := statedb.GetNonce(addr)
		if accCache.Nonce != accDbNonce {
			mismatch := fmt.Sprintf("nonce mismatch, account %s, cache nonce: %d, db nonce: %d", addr.String(), accCache.Nonce, accDbNonce)
			mismatches = append(mismatches, mismatch)
		}

		accDbBalance := statedb.GetBalance(addr)
		if accCache.Balance.Cmp(accDbBalance) != 0 {
			mismatch := fmt.Sprintf("balance mismatch, account %s, cache balance: %d, db balance: %d", addr.String(), accCache.Balance.ToBig(), accDbBalance.ToBig())
			mismatches = append(mismatches, mismatch)
		}

		// Note that realtime state cache does not update the state account storage trie root
		if accCache.Root != types.EmptyRootHash {
			mismatch := fmt.Sprintf("root mismatch, account %s, cache root: %s, db root: %s", addr.String(), accCache.Root.String(), types.EmptyRootHash)
			mismatches = append(mismatches, mismatch)
		}

		accDbCodeHash := statedb.GetCodeHash(addr)
		if !bytes.Equal(accCache.CodeHash, accDbCodeHash[:]) {
			mismatch := fmt.Sprintf("codehash mismatch, account %s, cache codehash: %s, db codehash: %s", addr.String(), hex.EncodeToString(accCache.CodeHash), accDbCodeHash.String())
			mismatches = append(mismatches, mismatch)
		}
	}

	return mismatches
}
