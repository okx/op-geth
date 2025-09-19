package state

import (
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/triedb"
	"github.com/ethereum/go-ethereum/triedb/hashdb"
	"testing"
)

// goos: darwin
// goarch: arm64
// pkg: github.com/ethereum/go-ethereum/core/state
// cpu: Apple M2 Max
// BenchmarkStackTrieInsertion
// BenchmarkStackTrieInsertion/Size_1000
// BenchmarkStackTrieInsertion/Size_1000-12         	     736	   1655859 ns/op	  607383 B/op	    9131 allocs/op
// BenchmarkStackTrieInsertion/Size_10000
// BenchmarkStackTrieInsertion/Size_10000-12        	      69	  16987653 ns/op	 5987054 B/op	   91794 allocs/op
// BenchmarkStackTrieInsertion/Size_100000
// BenchmarkStackTrieInsertion/Size_100000-12       	       6	 175602243 ns/op	63707266 B/op	  916970 allocs/op
// BenchmarkStackTrieInsertion/Size_1000000
// BenchmarkStackTrieInsertion/Size_1000000-12      	       1	1932725916 ns/op	606781832 B/op	 9085183 allocs/op
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
// BenchmarkRegularTrieInsertion
// BenchmarkRegularTrieInsertion/Size_1000
// BenchmarkRegularTrieInsertion/Size_1000-12         	     496	   2471376 ns/op	 2085254 B/op	   27792 allocs/op
// BenchmarkRegularTrieInsertion/Size_10000
// BenchmarkRegularTrieInsertion/Size_10000-12        	      62	  18114384 ns/op	20124467 B/op	  275523 allocs/op
// BenchmarkRegularTrieInsertion/Size_100000
// BenchmarkRegularTrieInsertion/Size_100000-12       	       6	 179106090 ns/op	207246184 B/op	 2741874 allocs/op
// BenchmarkRegularTrieInsertion/Size_1000000
// BenchmarkRegularTrieInsertion/Size_1000000-12      	       1	2398303500 ns/op	2098971800 B/op	27043820 allocs/op
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
