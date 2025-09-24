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

package types

import (
	"bytes"
	"runtime"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
)

// ParallelDeriveSha creates the tree hashes of transactions using parallel processing.
// This is an optimized version of DeriveSha that processes transactions in parallel
// while maintaining the correct order for StackTrie insertion.
func ParallelDeriveSha(list DerivableList, hasher TrieHasher) common.Hash {
	hasher.Reset()

	length := list.Len()
	if length == 0 {
		return hasher.Hash()
	}

	// For small lists, use sequential processing to avoid overhead
	if length < 100 {
		return DeriveSha(list, hasher)
	}

	// Determine optimal number of workers based on CPU cores and list size
	numWorkers := runtime.NumCPU()
	if length < numWorkers*8 {
		numWorkers = 2 // Use at least 2 workers for medium-sized lists
	}
	if numWorkers > 8 {
		numWorkers = 8 // Cap to prevent excessive goroutines
	}

	// Pre-encode all transactions in parallel
	type encodedItem struct {
		index int
		key   []byte
		value []byte
	}

	// Use a more efficient approach with fixed-size slices
	results := make([]encodedItem, length)
	workChan := make(chan int, length)

	// Start worker goroutines
	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			workerBuf := encodeBufferPool.Get().(*bytes.Buffer)
			defer encodeBufferPool.Put(workerBuf)

			for index := range workChan {
				// Encode the transaction
				workerBuf.Reset()
				list.EncodeIndex(index, workerBuf)

				// Generate key
				var key []byte
				if index == 0 {
					key = rlp.AppendUint64(key, 0)
				} else {
					key = rlp.AppendUint64(key, uint64(index))
				}

				// Store result directly in slice
				results[index] = encodedItem{
					index: index,
					key:   key,
					value: common.CopyBytes(workerBuf.Bytes()),
				}
			}
		}()
	}

	// Distribute work
	go func() {
		defer close(workChan)
		for i := 0; i < length; i++ {
			workChan <- i
		}
	}()

	// Wait for all workers to complete
	wg.Wait()

	// Update hasher in the correct order (same as original DeriveSha)
	for i := 1; i < length && i <= 0x7f; i++ {
		hasher.Update(results[i].key, results[i].value)
	}
	if length > 0 {
		hasher.Update(results[0].key, results[0].value)
	}
	for i := 0x80; i < length; i++ {
		hasher.Update(results[i].key, results[i].value)
	}

	return hasher.Hash()
}
