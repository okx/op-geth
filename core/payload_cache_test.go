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
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
)

// TestPayloadCacheBasic tests basic cache operations
func TestPayloadCacheBasic(t *testing.T) {
	// Create a cache with default config
	cache := NewPayloadCache(nil)

	// Create test data
	blockHash := common.HexToHash("0x1234")
	blockNumber := big.NewInt(100)
	parentHash := common.HexToHash("0x5678")

	result := &CachedPayloadResult{
		ProcessResult: &ProcessResult{
			Receipts: types.Receipts{},
			Logs:     []*types.Log{},
			GasUsed:  21000,
		},
		StateDB:     nil, // Would be a real StateDB in production
		BlockHash:   blockHash,
		BlockNumber: blockNumber,
		ParentHash:  parentHash,
	}

	// Test Add
	cache.Add(blockHash, result)

	// Test Get - should succeed
	cached, ok := cache.Get(blockHash)
	if !ok {
		t.Fatal("Failed to get cached result")
	}
	if cached.BlockNumber.Cmp(blockNumber) != 0 {
		t.Errorf("Block number mismatch: got %v, want %v", cached.BlockNumber, blockNumber)
	}
	if cached.ProcessResult.GasUsed != 21000 {
		t.Errorf("Gas used mismatch: got %v, want %v", cached.ProcessResult.GasUsed, 21000)
	}

	// Test Remove
	cache.Remove(blockHash)
	_, ok = cache.Get(blockHash)
	if ok {
		t.Fatal("Cache entry should have been removed")
	}
}

// TestPayloadCacheTTL tests cache TTL expiration
func TestPayloadCacheTTL(t *testing.T) {
	// Create a cache with 1 second TTL
	config := &PayloadCacheConfig{
		Size: 10,
		TTL:  100 * time.Millisecond,
	}
	cache := NewPayloadCache(config)

	blockHash := common.HexToHash("0xabc")
	result := &CachedPayloadResult{
		ProcessResult: &ProcessResult{
			GasUsed: 30000,
		},
		BlockHash: blockHash,
	}

	// Add to cache
	cache.Add(blockHash, result)

	// Should be retrievable immediately
	_, ok := cache.Get(blockHash)
	if !ok {
		t.Fatal("Cache should contain the entry")
	}

	// Wait for TTL to expire
	time.Sleep(150 * time.Millisecond)

	// Should not be retrievable after TTL
	_, ok = cache.Get(blockHash)
	if ok {
		t.Fatal("Cache entry should have expired")
	}
}

// TestPayloadCacheStats tests cache statistics
func TestPayloadCacheStats(t *testing.T) {
	cache := NewPayloadCache(nil)

	blockHash1 := common.HexToHash("0x111")
	blockHash2 := common.HexToHash("0x222")

	result := &CachedPayloadResult{
		ProcessResult: &ProcessResult{},
		BlockHash:     blockHash1,
	}

	// Add one entry
	cache.Add(blockHash1, result)

	// One hit
	cache.Get(blockHash1)

	// Two misses
	cache.Get(blockHash2)
	cache.Get(blockHash2)

	// Check stats
	hits, misses, hitRate := cache.Stats()
	if hits != 1 {
		t.Errorf("Expected 1 hit, got %d", hits)
	}
	if misses != 2 {
		t.Errorf("Expected 2 misses, got %d", misses)
	}
	expectedRate := 1.0 / 3.0
	if hitRate < expectedRate-0.01 || hitRate > expectedRate+0.01 {
		t.Errorf("Expected hit rate ~%.2f, got %.2f", expectedRate, hitRate)
	}
}

// TestPayloadCacheLRU tests LRU eviction
func TestPayloadCacheLRU(t *testing.T) {
	// Create a small cache that can only hold 2 items
	config := &PayloadCacheConfig{
		Size: 2,
		TTL:  time.Hour,
	}
	cache := NewPayloadCache(config)

	// Add 3 items
	for i := 1; i <= 3; i++ {
		hash := common.BytesToHash([]byte{byte(i)})
		result := &CachedPayloadResult{
			ProcessResult: &ProcessResult{
				GasUsed: uint64(i * 1000),
			},
			BlockHash:   hash,
			BlockNumber: big.NewInt(int64(i)),
		}
		cache.Add(hash, result)
	}

	// First item should be evicted
	_, ok := cache.Get(common.BytesToHash([]byte{1}))
	if ok {
		t.Fatal("First item should have been evicted")
	}

	// Second and third items should still be there
	_, ok = cache.Get(common.BytesToHash([]byte{2}))
	if !ok {
		t.Fatal("Second item should still be in cache")
	}
	_, ok = cache.Get(common.BytesToHash([]byte{3}))
	if !ok {
		t.Fatal("Third item should still be in cache")
	}
}

// TestPayloadCacheHandleReorg tests cache cleanup during reorg
func TestPayloadCacheHandleReorg(t *testing.T) {
	cache := NewPayloadCache(nil)

	// Add blocks that will be reorged
	oldBlocks := make(types.Blocks, 3)
	for i := 0; i < 3; i++ {
		header := &types.Header{
			Number: big.NewInt(int64(100 + i)),
			Time:   uint64(i),
		}
		oldBlocks[i] = types.NewBlockWithHeader(header)
		hash := oldBlocks[i].Hash()
		result := &CachedPayloadResult{
			ProcessResult: &ProcessResult{},
			BlockHash:     hash,
		}
		cache.Add(hash, result)
	}

	// Verify they're in cache
	for _, block := range oldBlocks {
		_, ok := cache.Get(block.Hash())
		if !ok {
			t.Fatal("Block should be in cache before reorg")
		}
	}

	// Handle reorg
	cache.HandleReorg(oldBlocks)

	// Verify they're removed
	for _, block := range oldBlocks {
		_, ok := cache.Get(block.Hash())
		if ok {
			t.Fatal("Block should be removed from cache after reorg")
		}
	}
}

// TestCopyStateDB tests StateDB copying (placeholder)
func TestCopyStateDB(t *testing.T) {
	// This is a placeholder test since we need a real StateDB to test
	// In production, this would test that CopyStateDB creates a proper deep copy
	var original *state.StateDB = nil
	copy := CopyStateDB(original)
	if copy != nil {
		t.Fatal("Copy of nil StateDB should be nil")
	}
}
