package state

import (
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/triedb"
	"github.com/ethereum/go-ethereum/triedb/hashdb"
	"github.com/stretchr/testify/assert"
	"math/rand"
	"testing"
)

func generateRandomKeyValuePairs(n int) map[common.Hash]common.Hash {
	result := make(map[common.Hash]common.Hash, n)

	for i := 0; i < n; i++ {
		var key common.Hash
		rand.Read(key[:])

		var value common.Hash
		rand.Read(value[:])

		result[key] = value
	}

	return result
}

func TestStackTrieCompatibility(t *testing.T) {
	config := &triedb.Config{
		Preimages: false,
		IsVerkle:  false,
		HashDB:    hashdb.Defaults,
	}

	storage := generateRandomKeyValuePairs(1000)
	owner := common.HexToHash("0x1234567890123456789012345678901234567890123456789012345678901234")
	addr := common.Address(owner[:20])

	triedbWrite := triedb.NewDatabase(rawdb.NewMemoryDatabase(), config)
	cachingDB := NewDatabase(triedbWrite, nil)
	tr, err := cachingDB.OpenStorageStackTrie(common.Hash{}, addr, common.Hash{}, nil)
	if err != nil {
		panic(err)
	}

	root, _, err := tr.UpdateStorageBatch(addr, storage)
	if err != nil {
		panic(err)
	}

	regularTriedbWrite := triedb.NewDatabase(rawdb.NewMemoryDatabase(), config)
	cachingRegularDB := NewDatabase(regularTriedbWrite, nil)
	regularTrie, err := cachingRegularDB.OpenStorageTrie(common.Hash{}, addr, common.Hash{}, nil)
	if err != nil {
		panic(err)
	}

	for key, v := range storage {
		err = regularTrie.UpdateStorage(addr, key[:], common.TrimLeftZeroes(v[:]))
		if err != nil {
			panic(err)
		}
	}

	regularTrieRootHash, _ := regularTrie.Commit(true)
	assert.Equal(t, regularTrieRootHash, root)
}

// goos: darwin
// goarch: arm64
// pkg: github.com/ethereum/go-ethereum/core/state
// cpu: Apple M2 Max
// BenchmarkStackTrieInsertion/Size_1000
// BenchmarkStackTrieInsertion/Size_1000-12         	     739	   1617923 ns/op	  582738 B/op	    9122 allocs/op
// BenchmarkStackTrieInsertion/Size_10000
// BenchmarkStackTrieInsertion/Size_10000-12        	      70	  16953160 ns/op	 5805821 B/op	   92059 allocs/op
// BenchmarkStackTrieInsertion/Size_100000
// BenchmarkStackTrieInsertion/Size_100000-12       	       6	 177203236 ns/op	61306298 B/op	  917144 allocs/op
// BenchmarkStackTrieInsertion/Size_1000000
// BenchmarkStackTrieInsertion/Size_1000000-12      	       1	1964037625 ns/op	582744808 B/op	 9085699 allocs/op
func BenchmarkStackTrieInsertion(b *testing.B) {
	config := &triedb.Config{
		Preimages: false,
		IsVerkle:  false,
		HashDB:    hashdb.Defaults,
	}

	datasetSizes := []int{1000, 10000, 100000, 1000000}

	for _, size := range datasetSizes {
		b.Run(fmt.Sprintf("Size_%d", size), func(b *testing.B) {
			b.ReportAllocs()

			storage := generateRandomKeyValuePairs(size)
			owner := common.HexToHash("0x1234567890123456789012345678901234567890123456789012345678901234")
			addr := common.Address(owner[:20])

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				triedbWrite := triedb.NewDatabase(rawdb.NewMemoryDatabase(), config)
				cachingDB := NewDatabase(triedbWrite, nil)
				tr, err := cachingDB.OpenStorageStackTrie(common.Hash{}, addr, common.Hash{}, nil)
				if err != nil {
					panic(err)
				}

				root, _, err := tr.UpdateStorageBatch(addr, storage)
				if err != nil {
					panic(err)
				}
				_ = root
			}
		})
	}
}

// goos: darwin
// goarch: arm64
// pkg: github.com/ethereum/go-ethereum/core/state
// cpu: Apple M2 Max
// BenchmarkRegularTrieInsertion/Size_1000
// BenchmarkRegularTrieInsertion/Size_1000-12         	     482	   2429196 ns/op	 2080139 B/op	   27682 allocs/op
// BenchmarkRegularTrieInsertion/Size_10000
// BenchmarkRegularTrieInsertion/Size_10000-12        	      70	  16680217 ns/op	20157853 B/op	  275211 allocs/op
// BenchmarkRegularTrieInsertion/Size_100000
// BenchmarkRegularTrieInsertion/Size_100000-12       	       6	 180554403 ns/op	207196216 B/op	 2741910 allocs/op
// BenchmarkRegularTrieInsertion/Size_1000000
// BenchmarkRegularTrieInsertion/Size_1000000-12      	       1	2423703250 ns/op	2099059016 B/op	27041567 allocs/op
func BenchmarkRegularTrieInsertion(b *testing.B) {
	config := &triedb.Config{
		Preimages: false,
		IsVerkle:  false,
		HashDB:    hashdb.Defaults,
	}

	datasetSizes := []int{1000, 10000, 100000, 1000000}

	for _, size := range datasetSizes {
		b.Run(fmt.Sprintf("Size_%d", size), func(b *testing.B) {
			b.ReportAllocs()

			storage := generateRandomKeyValuePairs(size)
			owner := common.HexToHash("0x1234567890123456789012345678901234567890123456789012345678901234")
			addr := common.Address(owner[:20])

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				triedbWrite := triedb.NewDatabase(rawdb.NewMemoryDatabase(), config)
				cachingDB := NewDatabase(triedbWrite, nil)
				tr, err := cachingDB.OpenStorageTrie(common.Hash{}, addr, common.Hash{}, nil)
				if err != nil {
					panic(err)
				}

				for k, v := range storage {
					err = tr.UpdateStorage(addr, k[:], common.TrimLeftZeroes(v[:]))
					if err != nil {
						panic(err)
					}
				}

				root, _ := tr.Commit(true)
				_ = root
			}
		})
	}
}
