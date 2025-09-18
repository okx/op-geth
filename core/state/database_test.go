package state

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/triedb"
	"github.com/ethereum/go-ethereum/triedb/hashdb"
	"testing"
)

func BenchmarkStackTrieInsertion(b *testing.B) {
	config := &triedb.Config{
		Preimages: false,
		IsVerkle:  false,
		HashDB:    hashdb.Defaults,
	}
	storage := generateRandomKeyValuePairs(1000000)
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
		_ = root
	}
}
func BenchmarkRegularTrieInsertion(b *testing.B) {
	// Generate test data once
	config := &triedb.Config{
		Preimages: false,
		IsVerkle:  false,
		HashDB:    hashdb.Defaults,
	}
	storage := generateRandomKeyValuePairs(1000000)
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
}
