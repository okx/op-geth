package core

import (
	"bytes"
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum/core/types"
	"math/big"
	"math/bits"
	"runtime"
	"sync"
	"sync/atomic"
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

type AccountEntry struct {
	Address common.Address
	Account *types.Account
}

// generatePowerOfTwoKeyRanges splits the 256-bit space into numChunks
// equal ranges, each of size 2^(256 - log2(numChunks)).
// @param `keySpaceBitNum`, number of bits of the key space
// @param `numChunks`, num of chunks to be split into, must be a power of two.
func generatePowerOfTwoKeyRanges(keySpaceBitNum uint, numChunks uint) []KeyRange {
	// check that numChunks is a power of two
	if numChunks <= 0 || (numChunks&(numChunks-1)) != 0 {
		panic("numChunks must be a power of two")
	}

	bitShift := keySpaceBitNum - uint(log2Bits(numChunks))
	chunkSize := new(big.Int).Lsh(big.NewInt(1), uint(bitShift)) // 2^(256 - m)

	numOfBytes := keySpaceBitNum >> 3
	ranges := make([]KeyRange, numChunks)

	ranges[0].Start = make([]byte, numOfBytes)
	ranges[0].End = intToBytes(chunkSize, int(numOfBytes))
	for i := 1; i < int(numChunks); i++ {
		end := new(big.Int).Mul(big.NewInt(int64(i+1)), chunkSize)
		if i == int(numChunks)-1 {
			end = end.Sub(end, big.NewInt(1))
		}

		tmp := make([]byte, numOfBytes)
		copy(tmp, ranges[i-1].End)
		ranges[i] = KeyRange{
			Start: tmp,
			End:   intToBytes(end, int(numOfBytes)),
		}
	}
	return ranges
}

func log2Bits(n uint) int {
	return bits.Len(n) - 1
}

func intToBytes(x *big.Int, numOfBytes int) []byte {
	b := x.Bytes()
	if len(b) > numOfBytes {
		panic("overflow")
	}
	padded := make([]byte, numOfBytes)
	copy(padded[numOfBytes-len(b):], b)
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

func processAccountsConcurrently(db kv.RoDB) ([]AccountEntry, error) {
	numWorkers := largestPowerOfTwo(runtime.NumCPU())
	keyRanges := generatePowerOfTwoKeyRanges(160, uint(numWorkers))

	var storageScanned int64

	var wg sync.WaitGroup
	results := make(chan []AccountEntry, numWorkers)

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			start := time.Now()
			keyRange := keyRanges[workerID]

			if err := db.View(context.Background(), func(workerTx kv.Tx) error {
				var count uint64

				cursor, err := workerTx.Cursor(PlainStateBucket)
				if err != nil {
					return err
				}
				defer cursor.Close()

				//make([]byte, 60)
				startKey := make([]byte, 60)
				copy(startKey[:20], keyRange.Start)
				copy(startKey[20:], bytes.Repeat([]byte{0xFF}, 40))

				endKey := make([]byte, 60)
				copy(endKey[:20], keyRange.End)
				copy(endKey[20:], bytes.Repeat([]byte{0xFF}, 40))

				chunkAccts := make([]AccountEntry, 0, 1<<16)
				fmt.Printf("acct start", keyRange.Start, "end", keyRange.End)

				iter, err := workerTx.Range(kv.PlainState, startKey, endKey)
				if err != nil {
					logger.Error("failed to create range iterator", "worker", workerID, "error", err)
					results <- chunkAccts
					return err
				}

				for iter.HasNext() {
					keyAcct, valAcct, err := iter.Next()
					if err != nil {
						logger.Error("failed to read value from cursor", "worker", workerID, "error", err)
						break
					}
					if len(keyAcct) == common.AddressLength {
						addr := common.BytesToAddress(keyAcct[:common.AddressLength])

						decodedAcct, err := decodeAccountData(valAcct)
						if err != nil {
							logger.Warn("processAccountsConcurrently: failed to decode account", "address", addr.Hex(), "error", err)
							return nil
						}

						// Get code data if account has code
						if len(decodedAcct.Code) > 0 {
							codeHash := common.BytesToHash(decodedAcct.Code)
							if codeHash != EmptyCodeHash {
								code, err := workerTx.GetOne(CodeBucket, codeHash[:])
								if err == nil && len(code) > 0 {
									// Make a copy to avoid potential memory issues
									decodedAcct.Code = make([]byte, len(code))
									copy(decodedAcct.Code, code)
								}
							}
						}

						chunkAccts = append(chunkAccts, AccountEntry{
							Address: addr,
							Account: decodedAcct,
						})
					} else {
						//logger.Warn("found unexpected acct key", "len", len(keyAcct), "key", keyAcct)
						atomic.AddInt64(&storageScanned, 1)
						//panic("yeah, storage")
					}
				}

				results <- chunkAccts
				logger.Info("worker completed", "workerID", workerID, "count", count, "duration", time.Since(start))
				return nil
			}); err != nil {
				logger.Error("worker error", "workerID", workerID, "error", err)
				results <- []AccountEntry{} // Send empty result on error
			}
		}(i)
	}
	go func() {
		wg.Wait()
		close(results)
	}()

	var allAccounts []AccountEntry

	for accounts := range results {
		allAccounts = append(allAccounts, accounts...)
	}

	logger.Info("concurrent account processing completed", "storageScanned", storageScanned)
	return allAccounts, nil
}

func processScalableAddressStorageConcurrently(db kv.RoDB, prefix []byte) (map[common.Hash]common.Hash, uint64, error) {

	numWorkers := largestPowerOfTwo(runtime.NumCPU())
	keyRanges := generatePowerOfTwoKeyRanges(256, uint(numWorkers))

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
