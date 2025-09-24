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

package types_test

import (
	"fmt"
	"math/big"
	"math/rand"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/trie"
)

// generateTestTransactions creates a slice of test transactions for benchmarking
func generateTestTransactions(count int) (types.Transactions, error) {
	txs := make(types.Transactions, 0, count)

	for i := 0; i < count; i++ {
		// Create a random private key
		key, err := crypto.GenerateKey()
		if err != nil {
			return nil, err
		}

		// Create a random address
		to := common.BigToAddress(big.NewInt(rand.Int63()))

		// Create transaction data
		tx := types.NewTransaction(
			uint64(i),                     // nonce
			to,                            // to
			big.NewInt(rand.Int63()),      // value
			21000,                         // gas limit
			big.NewInt(1000000000),        // gas price
			make([]byte, rand.Intn(1000)), // data
		)

		// Sign the transaction
		signer := types.NewEIP155Signer(big.NewInt(1))
		signedTx, err := types.SignTx(tx, signer, key)
		if err != nil {
			return nil, err
		}

		txs = append(txs, signedTx)
	}

	return txs, nil
}

// generateTestReceipts creates a slice of test receipts for benchmarking
func generateTestReceipts(count int) (types.Receipts, error) {
	receipts := make(types.Receipts, 0, count)

	for i := 0; i < count; i++ {
		// Create a random transaction hash
		txHash := common.BigToHash(big.NewInt(rand.Int63()))

		// Create a random contract address (50% chance of being empty)
		var contractAddr common.Address
		if rand.Intn(2) == 1 {
			contractAddr = common.BigToAddress(big.NewInt(rand.Int63()))
		}

		// Create random logs
		logCount := rand.Intn(5) // 0-4 logs per receipt
		logs := make([]*types.Log, 0, logCount)
		for j := 0; j < logCount; j++ {
			log := &types.Log{
				Address: common.BigToAddress(big.NewInt(rand.Int63())),
				Topics:  make([]common.Hash, rand.Intn(4)+1), // 1-4 topics
				Data:    make([]byte, rand.Intn(200)),        // 0-199 bytes of data
			}
			// Fill topics with random hashes
			for k := range log.Topics {
				log.Topics[k] = common.BigToHash(big.NewInt(rand.Int63()))
			}
			logs = append(logs, log)
		}

		// Create receipt
		receipt := &types.Receipt{
			Type:              uint8(rand.Intn(4)),  // 0-3 transaction types
			PostState:         make([]byte, 32),     // 32-byte state root
			Status:            uint64(rand.Intn(2)), // 0 or 1
			CumulativeGasUsed: uint64(21000 + rand.Intn(100000)),
			Bloom:             types.Bloom{}, // Will be filled by DeriveFields
			Logs:              logs,
			TxHash:            txHash,
			ContractAddress:   contractAddr,
			GasUsed:           uint64(21000 + rand.Intn(50000)),
			EffectiveGasPrice: big.NewInt(1000000000 + rand.Int63n(1000000000)),
			BlockHash:         common.BigToHash(big.NewInt(rand.Int63())),
			BlockNumber:       big.NewInt(rand.Int63()),
			TransactionIndex:  uint(i),
		}

		// Fill PostState with random data
		for k := range receipt.PostState {
			receipt.PostState[k] = byte(rand.Intn(256))
		}

		receipts = append(receipts, receipt)
	}

	return receipts, nil
}

// TestParallelDeriveShaCorrectness verifies that parallel implementation produces same results
func TestParallelDeriveShaCorrectness(t *testing.T) {
	// Test with different transaction counts
	testCounts := []int{0, 1, 10, 100, 1000, 6000}

	for _, count := range testCounts {
		t.Run(fmt.Sprintf("Count_%d", count), func(t *testing.T) {
			txs, err := generateTestTransactions(count)
			if err != nil {
				t.Fatal(err)
			}

			// Test with StackTrie
			stackTrie1 := trie.NewStackTrie(nil)
			stackTrie2 := trie.NewStackTrie(nil)

			originalHash := types.DeriveSha(txs, stackTrie1)
			parallelHash := types.ParallelDeriveSha(txs, stackTrie2)

			if originalHash != parallelHash {
				t.Errorf("StackTrie: Hash mismatch for count %d: original=%x, parallel=%x",
					count, originalHash, parallelHash)
			}
		})
	}
}

// TestParallelDeriveShaReceiptsCorrectness verifies that parallel implementation produces same results for receipts
func TestParallelDeriveShaReceiptsCorrectness(t *testing.T) {
	// Test with different receipt counts
	testCounts := []int{0, 1, 10, 100, 1000, 6000}

	for _, count := range testCounts {
		t.Run(fmt.Sprintf("Count_%d", count), func(t *testing.T) {
			receipts, err := generateTestReceipts(count)
			if err != nil {
				t.Fatal(err)
			}

			// Test with StackTrie
			stackTrie1 := trie.NewStackTrie(nil)
			stackTrie2 := trie.NewStackTrie(nil)

			originalHash := types.DeriveSha(receipts, stackTrie1)
			parallelHash := types.ParallelDeriveSha(receipts, stackTrie2)

			if originalHash != parallelHash {
				t.Errorf("StackTrie: Receipt hash mismatch for count %d: original=%x, parallel=%x",
					count, originalHash, parallelHash)
			}
		})
	}
}

// PerformanceTestWithTiming provides detailed timing information
func TestPerformanceWithTiming(t *testing.T) {
	const N = 100
	var txs [N]types.Transactions
	// Generate 6000 transactions
	for i := 0; i < N; i++ {
		var err error
		txs[i], err = generateTestTransactions(6000)
		if err != nil {
			t.Fatal(err)
		}
	}

	// Test original implementation
	start := time.Now()
	for i := 0; i < N; i++ {
		stackTrie := trie.NewStackTrie(nil)
		types.DeriveSha(txs[i], stackTrie)
	}
	originalTime := time.Since(start)

	// Test parallel implementation
	start = time.Now()
	for i := 0; i < N; i++ {
		stackTrie := trie.NewStackTrie(nil)
		types.ParallelDeriveSha(txs[i], stackTrie)
	}
	parallelTime := time.Since(start)

	// Calculate improvements
	originalAvg := float64(originalTime.Nanoseconds()) / N
	parallelAvg := float64(parallelTime.Nanoseconds()) / N

	improvement1 := (originalAvg - parallelAvg) / originalAvg * 100

	t.Logf("=== Performance Results for 6000 Transactions ===")
	t.Logf("Original DeriveSha:           %v (avg: %.2f ns)", originalTime, originalAvg)
	t.Logf("Parallel DeriveSha:           %v (avg: %.2f ns)", parallelTime, parallelAvg)
	t.Logf("Improvement (Parallel):       %.2f%%", improvement1)

	// Verify correctness
	for i := 0; i < N; i++ {
		stackTrie1 := trie.NewStackTrie(nil)
		stackTrie2 := trie.NewStackTrie(nil)

		originalHash := types.DeriveSha(txs[i], stackTrie1)
		parallelHash := types.ParallelDeriveSha(txs[i], stackTrie2)

		if originalHash != parallelHash {
			t.Errorf("Hash mismatch: original=%x, parallel=%x", originalHash, parallelHash)
		}
	}
}

// TestReceiptPerformanceWithTiming provides detailed timing information for receipts
func TestReceiptPerformanceWithTiming(t *testing.T) {
	// Generate 6000 receipts
	const N = 100
	var receipts [N]types.Receipts
	for i := 0; i < N; i++ {
		var err error
		receipts[i], err = generateTestReceipts(6000)
		if err != nil {
			t.Fatal(err)
		}
	}

	// Test original implementation
	start := time.Now()
	for i := 0; i < N; i++ {
		stackTrie := trie.NewStackTrie(nil)
		types.DeriveSha(receipts[i], stackTrie)
	}
	originalTime := time.Since(start)

	// Test parallel implementation
	start = time.Now()
	for i := 0; i < 100; i++ {
		stackTrie := trie.NewStackTrie(nil)
		types.ParallelDeriveSha(receipts[i], stackTrie)
	}
	parallelTime := time.Since(start)

	// Calculate improvements
	originalAvg := float64(originalTime.Nanoseconds()) / N
	parallelAvg := float64(parallelTime.Nanoseconds()) / N

	improvement1 := (originalAvg - parallelAvg) / originalAvg * 100

	t.Logf("=== Performance Results for 6000 Receipts ===")
	t.Logf("Original DeriveSha:           %v (avg: %.2f ns)", originalTime, originalAvg)
	t.Logf("Parallel DeriveSha:           %v (avg: %.2f ns)", parallelTime, parallelAvg)
	t.Logf("Improvement (Parallel):       %.2f%%", improvement1)

	// Verify correctness
	for i := 0; i < N; i++ {
		stackTrie1 := trie.NewStackTrie(nil)
		stackTrie2 := trie.NewStackTrie(nil)

		originalHash := types.DeriveSha(receipts[i], stackTrie1)
		parallelHash := types.ParallelDeriveSha(receipts[i], stackTrie2)

		if originalHash != parallelHash {
			t.Errorf("Receipt hash mismatch: original=%x, parallel=%x", originalHash, parallelHash)
		}
	}
}

// BenchmarkMemoryUsage tests memory allocation patterns
func BenchmarkMemoryUsage(b *testing.B) {
	txs, err := generateTestTransactions(6000)
	if err != nil {
		b.Fatal(err)
	}

	b.Run("Original_Memory", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			stackTrie := trie.NewStackTrie(nil)
			types.DeriveSha(txs, stackTrie)
		}
	})

	b.Run("Parallel_Memory", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			stackTrie := trie.NewStackTrie(nil)
			types.ParallelDeriveSha(txs, stackTrie)
		}
	})
}

// BenchmarkReceiptMemoryUsage tests memory allocation patterns for receipts
func BenchmarkReceiptMemoryUsage(b *testing.B) {
	receipts, err := generateTestReceipts(6000)
	if err != nil {
		b.Fatal(err)
	}

	b.Run("Original_Receipt_Memory", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			stackTrie := trie.NewStackTrie(nil)
			types.DeriveSha(receipts, stackTrie)
		}
	})

	b.Run("Parallel_Receipt_Memory", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			stackTrie := trie.NewStackTrie(nil)
			types.ParallelDeriveSha(receipts, stackTrie)
		}
	})
}
