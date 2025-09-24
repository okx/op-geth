// Copyright 2024 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package core

import (
	"math/big"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common"
	lru "github.com/ethereum/go-ethereum/common/lru"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/metrics"
)

// CachedPayloadResult represents a cached block execution result
type CachedPayloadResult struct {
	// Core execution results
	ProcessResult *ProcessResult // Contains receipts, requests, logs, gasUsed
	StateDB       *state.StateDB // State after execution

	// Block identification
	BlockHash   common.Hash // Block hash as cache key
	BlockNumber *big.Int    // Block number
	ParentHash  common.Hash // Parent block hash for validation

	// Metadata
	Timestamp time.Time // Cache timestamp for TTL
}

// PayloadCacheConfig contains configuration for the payload cache
type PayloadCacheConfig struct {
	Size int           // Maximum number of cached entries
	TTL  time.Duration // Time-to-live for cache entries
}

// DefaultPayloadCacheConfig returns the default cache configuration
func DefaultPayloadCacheConfig() *PayloadCacheConfig {
	return &PayloadCacheConfig{
		Size: 20,
		TTL:  30 * time.Second,
	}
}

// PayloadCache manages cached payload execution results
type PayloadCache struct {
	cache *lru.Cache[common.Hash, *CachedPayloadResult]
	mu    sync.RWMutex

	// Configuration
	config *PayloadCacheConfig

	// Statistics
	hits   atomic.Uint64
	misses atomic.Uint64
}

// NewPayloadCache creates a new payload cache with the given configuration
func NewPayloadCache(config *PayloadCacheConfig) *PayloadCache {
	if config == nil {
		config = DefaultPayloadCacheConfig()
	}

	cache := lru.NewCache[common.Hash, *CachedPayloadResult](config.Size)
	return &PayloadCache{
		cache:  cache,
		config: config,
	}
}

// Add stores a payload result in the cache
func (pc *PayloadCache) Add(blockHash common.Hash, result *CachedPayloadResult) {
	if pc == nil || result == nil {
		return
	}

	pc.mu.Lock()
	defer pc.mu.Unlock()

	result.Timestamp = time.Now()
	pc.cache.Add(blockHash, result)

	// Update metrics
	metrics.PayloadCacheAddMeter.Mark(1)
	metrics.PayloadCacheSizeGauge.Update(int64(pc.cache.Len()))

	log.Debug("Added payload to cache",
		"hash", blockHash,
		"number", result.BlockNumber,
		"parent", result.ParentHash)
}

// Get retrieves a cached payload result
func (pc *PayloadCache) Get(blockHash common.Hash) (*CachedPayloadResult, bool) {
	if pc == nil {
		return nil, false
	}

	pc.mu.RLock()
	defer pc.mu.RUnlock()

	if result, ok := pc.cache.Get(blockHash); ok {
		// Check if the entry has expired
		if time.Since(result.Timestamp) < pc.config.TTL {
			pc.hits.Add(1)
			metrics.PayloadCacheHitMeter.Mark(1)

			// Update hit rate
			total := float64(pc.hits.Load() + pc.misses.Load())
			if total > 0 {
				metrics.PayloadCacheHitRateGauge.Update(float64(pc.hits.Load()) / total)
			}

			log.Debug("Payload cache hit",
				"hash", blockHash,
				"number", result.BlockNumber,
				"age", time.Since(result.Timestamp))
			return result, true
		}
		// Entry has expired, remove it
		pc.cache.Remove(blockHash)
		metrics.PayloadCacheEvictMeter.Mark(1)
		log.Debug("Payload cache expired",
			"hash", blockHash,
			"age", time.Since(result.Timestamp))
	}

	pc.misses.Add(1)
	metrics.PayloadCacheMissMeter.Mark(1)

	// Update hit rate
	total := float64(pc.hits.Load() + pc.misses.Load())
	if total > 0 {
		metrics.PayloadCacheHitRateGauge.Update(float64(pc.hits.Load()) / total)
	}

	return nil, false
}

// Remove explicitly removes an entry from the cache
func (pc *PayloadCache) Remove(blockHash common.Hash) {
	if pc == nil {
		return
	}

	pc.mu.Lock()
	defer pc.mu.Unlock()

	pc.cache.Remove(blockHash)
	log.Debug("Removed payload from cache", "hash", blockHash)
}

// Clear removes all entries from the cache
func (pc *PayloadCache) Clear() {
	if pc == nil {
		return
	}

	pc.mu.Lock()
	defer pc.mu.Unlock()

	pc.cache.Purge()
	log.Info("Cleared payload cache")
}

// Size returns the current number of cached entries
func (pc *PayloadCache) Size() int {
	if pc == nil {
		return 0
	}

	pc.mu.RLock()
	defer pc.mu.RUnlock()

	return pc.cache.Len()
}

// Stats returns cache statistics
func (pc *PayloadCache) Stats() (hits, misses uint64, hitRate float64) {
	if pc == nil {
		return 0, 0, 0
	}

	hits = pc.hits.Load()
	misses = pc.misses.Load()
	total := hits + misses
	if total > 0 {
		hitRate = float64(hits) / float64(total)
	}
	return
}

// HandleReorg clears cache entries affected by a chain reorganization
func (pc *PayloadCache) HandleReorg(oldBlocks, newBlocks types.Blocks) {
	if pc == nil {
		return
	}

	pc.mu.Lock()
	defer pc.mu.Unlock()

	// Remove all blocks from the old chain
	for _, block := range oldBlocks {
		pc.cache.Remove(block.Hash())
	}

	log.Info("Handled chain reorg in payload cache",
		"removed", len(oldBlocks),
		"new", len(newBlocks))
}

// CopyStateDB creates a deep copy of the StateDB for safe reuse
func CopyStateDB(original *state.StateDB) *state.StateDB {
	if original == nil {
		return nil
	}
	// Use the StateDB's built-in Copy method
	return original.Copy()
}
