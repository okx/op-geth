package core

import (
	"context"
	"math/big"
	"runtime"
	"sync"
	"time"

	"github.com/bytedance/gopkg/util/logger"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ledgerwatch/erigon-lib/kv"
)

type KeyRange struct {
	Start []byte
	End   []byte
}

type StorageEntry struct {
	Key   common.Hash
	Value common.Hash
}

// generatePowerOfTwoKeyRanges splits the 256-bit space into numChunks
// equal ranges, each of size 2^(256 - log2(numChunks)).
// numChunks must be a power of two.
func generatePowerOfTwoKeyRanges(numChunks int) []KeyRange {
	// check that numChunks is a power of two
	if numChunks <= 0 || (numChunks&(numChunks-1)) != 0 {
		panic("numChunks must be a power of two")
	}

	bitShift := 256 - log2(numChunks)
	chunkSize := new(big.Int).Lsh(big.NewInt(1), uint(bitShift)) // 2^(256 - m)

	ranges := make([]KeyRange, numChunks)

	ranges[0].Start = make([]byte, 32)
	ranges[0].End = intToBytes32(chunkSize)
	for i := 1; i < numChunks; i++ {
		end := new(big.Int).Mul(big.NewInt(int64(i+1)), chunkSize)
		if i == numChunks-1 {
			end = end.Sub(end, big.NewInt(1))
		}
		
		tmp := make([]byte, 32)
		copy(tmp, ranges[i-1].End)
		ranges[i] = KeyRange{
			Start: tmp,
			End:   intToBytes32(end),
		}
	}
	return ranges
}

func log2(n int) int {
	// assumes n is a power of two
	p := 0
	for n > 1 {
		n >>= 1
		p++
	}
	return p
}

func intToBytes32(x *big.Int) []byte {
	b := x.Bytes()
	if len(b) > 32 {
		panic("overflow")
	}
	padded := make([]byte, 32)
	copy(padded[32-len(b):], b)
	return padded
}

func largestPowerOfTwo(n int) int {
	if n <= 0 {
		return 1
	}
	power := 1
	for power*2 <= n {
		power *= 2
	}
	return power
}

func processScalableAddressStorageConcurrently(db kv.RoDB, prefix []byte) (map[common.Hash]common.Hash, uint64, error) {

	numWorkers := runtime.NumCPU()
	keyRanges := generatePowerOfTwoKeyRanges(numWorkers)

	var wg sync.WaitGroup
	results := make(chan []StorageEntry, numWorkers)

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			start := time.Now()
			keyRange := keyRanges[workerID]

			if err := db.View(context.Background(), func(workerTx kv.Tx) error {
				chunkStorage := make([]StorageEntry, 0, 1<<20)

				startKey := make([]byte, 60)
				copy(startKey, prefix[0:28])
				copy(startKey[28:], keyRange.Start)

				endKey := make([]byte, 60)
				copy(endKey, prefix[0:28])
				copy(endKey[28:], keyRange.End)

				iter, err := workerTx.Range(kv.PlainState, startKey, endKey)
				if err != nil {
					logger.Error("failed to create range iterator", "worker", workerID, "error", err)
					results <- chunkStorage
					return err
				}

				for iter.HasNext() {
					keyStorage, valStorage, err := iter.Next()
					if err != nil {
						logger.Error("failed to read value from cursor", "worker", workerID, "error", err)
						break
					}
					if len(keyStorage) > 28 {

						chunkStorage = append(chunkStorage, StorageEntry{
							Key:   common.BytesToHash(keyStorage[28:]),
							Value: common.BytesToHash(valStorage),
						})
					}
				}

				results <- chunkStorage
				logger.Info("worker completed", "id", workerID, "elapsed", time.Since(start))
				return nil
			}); err != nil {
				logger.Error("worker transaction failed", "worker", workerID, "error", err)
				results <- make([]StorageEntry, 0, 1024)
			}
		}(i)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	start := time.Now()
	var totalStorage uint64
	chunkCount := 0

	var allChunks [][]StorageEntry
	for chunkStorage := range results {
		allChunks = append(allChunks, chunkStorage)
		totalStorage += uint64(len(chunkStorage))
		chunkCount++
	}

	storage := make(map[common.Hash]common.Hash, int(totalStorage))

	for _, chunk := range allChunks {
		for _, entry := range chunk {
			storage[entry.Key] = entry.Value
		}
	}

	logger.Info("Scalable address total storage items", "count", totalStorage, "chunk count", chunkCount, "elapsed", time.Since(start))
	return storage, totalStorage, nil
}
