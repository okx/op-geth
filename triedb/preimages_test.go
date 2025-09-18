// Copyright 2023 The go-ethereum Authors
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

package triedb

import (
	"bytes"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/trie/trienode"
	"github.com/ethereum/go-ethereum/triedb/hashdb"
	"testing"
)

// TestDatabasePreimages tests the preimage functionality of the trie database.
func TestDatabasePreimages(t *testing.T) {
	// Create a database with preimages enabled
	memDB := rawdb.NewMemoryDatabase()
	config := &Config{
		Preimages: true,
		HashDB:    hashdb.Defaults,
	}
	db := NewDatabase(memDB, config)
	defer db.Close()

	// Test inserting and retrieving preimages
	preimages := make(map[common.Hash][]byte)
	for i := 0; i < 10; i++ {
		data := []byte{byte(i), byte(i + 1), byte(i + 2)}
		hash := common.BytesToHash(data)
		preimages[hash] = data
	}

	// Insert preimages into the database
	db.InsertPreimage(preimages)

	// Verify all preimages are retrievable
	for hash, data := range preimages {
		retrieved := db.Preimage(hash)
		if retrieved == nil {
			t.Errorf("Preimage for %x not found", hash)
		}
		if !bytes.Equal(retrieved, data) {
			t.Errorf("Preimage data mismatch: got %x want %x", retrieved, data)
		}
	}

	// Test non-existent preimage
	nonExistentHash := common.HexToHash("deadbeef")
	if data := db.Preimage(nonExistentHash); data != nil {
		t.Errorf("Unexpected preimage data for non-existent hash: %x", data)
	}

	// Force preimage commit and verify again
	db.WritePreimages()
	for hash, data := range preimages {
		retrieved := db.Preimage(hash)
		if retrieved == nil {
			t.Errorf("Preimage for %x not found after forced commit", hash)
		}
		if !bytes.Equal(retrieved, data) {
			t.Errorf("Preimage data mismatch after forced commit: got %x want %x", retrieved, data)
		}
	}
}

//// BenchmarkRegularTrieInsertion benchmarks regular trie insertion performance
//func BenchmarkRegularTrieInsertion(b *testing.B) {
//	// Generate test data once
//	testData := generateRandomKeyValuePairs(1000000)
//
//	b.ResetTimer()
//
//	for i := 0; i < b.N; i++ {
//		// Create fresh database and trie for each iteration
//		db := rawdb.NewMemoryDatabase()
//		triedb := NewDatabase(db, nil)
//		trie := trie2.NewEmpty(triedb)
//
//		// Sort keys within the benchmark function
//		sortedKeys := make([]string, len(keys))
//		copy(sortedKeys, keys)
//		sort.Strings(sortedKeys)
//
//		// Insert all key-value pairs
//		for j := 0; j < len(sortedKeys); j++ {
//			trie.Update([]byte(sortedKeys[j]), []byte(values[j]))
//		}
//
//		// Commit to get final root hash
//		root, _ := trie.Commit(true)
//		_ = root
//	}
//}
//
//// BenchmarkStackTrieInsertion benchmarks StackTrie insertion performance
//func BenchmarkStackTrieInsertion(b *testing.B) {
//	// Generate test data once
//	keys, values := generateRandomKeyValuePairs(1000000)
//
//	b.ResetTimer()
//
//	for i := 0; i < b.N; i++ {
//		// Create fresh database and trie for each iteration
//		//stackTrieDb := NewDatabase(rawdb.NewMemoryDatabase(), HashDefaults)
//
//		// Create NodeSet for StackTrie
//		nodeSet := trienode.NewNodeSet(common.Hash{})
//
//		// Create onTrieNode callback
//		onTrieNode := func(path []byte, hash common.Hash, blob []byte) {
//			nodeSet.AddNode(path, trienode.New(hash, blob))
//		}
//
//		st := trie2.NewStackTrie(onTrieNode)
//
//		// Sort keys within the benchmark function
//		sortedKeys := make([]string, len(keys))
//		copy(sortedKeys, keys)
//		sort.Strings(sortedKeys)
//
//		// Insert all key-value pairs in sorted order
//		for j := 0; j < len(sortedKeys); j++ {
//			st.Update([]byte(sortedKeys[j]), []byte(values[j]))
//		}
//
//		// Get final root hash
//		root := st.Hash()
//		_ = root
//	}
//}

func writeHexKey(dst []byte, key []byte) []byte {
	_ = dst[2*len(key)-1]
	for i, b := range key {
		dst[i*2] = b / 16
		dst[i*2+1] = b % 16
	}
	return dst[:2*len(key)]
}

// Helper function to print node set structure using reflection
func printNodeSetStructure(t *testing.T, name string, nodeSet *trienode.NodeSet) {
	fmt.Printf("\n--- %s Node Details (Ordered) ---\n", name)

	// Get node counts
	updates, deletes := nodeSet.Size()
	fmt.Printf("Total Updates: %d, Total Deletes: %d\n", updates, deletes)

	// Use ForEachWithOrder to iterate through nodes
	nodeSet.ForEachWithOrder(func(path string, n *trienode.Node) {
		fmt.Printf("Path: %x\n", path)
		fmt.Printf("  Hash: %x\n", n.Hash)

		if len(n.Blob) > 0 {
			fmt.Printf("  Blob: %x, len: %d \n", n.Blob, len(n.Blob))
		}
		fmt.Printf("  ---\n")
	})
}
