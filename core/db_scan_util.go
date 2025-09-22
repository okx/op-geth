package core

import (
	"bytes"
	"context"
	"github.com/bytedance/gopkg/util/logger"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ledgerwatch/erigon-lib/kv"
	"sync"
	"time"
)

type KeyRange struct {
	Start []byte
	End   []byte
}

type StorageEntry struct {
	Key   common.Hash
	Value common.Hash
}

// for storage key, it is of 32 bytes. we split the storage key space into chunks lexically
func generateKeyRanges(numChunks int) []KeyRange {
	keyRanges := make([]KeyRange, numChunks)

	for i := 0; i < numChunks; i++ {
		startKey := make([]byte, 32)
		startKey[0] = byte(i * (256 / numChunks))

		var endKey []byte
		if i == numChunks-1 {
			endKey = bytes.Repeat([]byte{0xFF}, 32)
		} else {
			endKey = make([]byte, 32)
			endKey[0] = byte((i + 1) * (256 / numChunks))
		}

		keyRanges[i] = KeyRange{Start: startKey, End: endKey}
	}

	return keyRanges
}

func processScalableAddressStorageConcurrently(db kv.RoDB, prefix []byte, acct types.Account) (uint64, error) {

	numWorkers := 32
	keyRanges := generateKeyRanges(numWorkers)

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

	acct.Storage = make(map[common.Hash]common.Hash, int(totalStorage))

	for _, chunk := range allChunks {
		for _, entry := range chunk {
			acct.Storage[entry.Key] = entry.Value
		}
	}

	logger.Info("Scalable address total storage items", "count", totalStorage, "chunk count", chunkCount, "elapsed", time.Since(start))
	return totalStorage, nil
}
