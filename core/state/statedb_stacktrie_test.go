package state

import (
	"bytes"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/ethereum/go-ethereum/trie/trienode"
	"github.com/ethereum/go-ethereum/triedb"
	"github.com/ethereum/go-ethereum/triedb/hashdb"
	"github.com/holiman/uint256"
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

func TestStackTrieQueryConsistencywithTrie(t *testing.T) {
	config := &triedb.Config{
		Preimages: false,
		IsVerkle:  false,
		HashDB:    hashdb.Defaults,
	}

	testData := generateRandomKeyValuePairs(2)
	owner := common.HexToHash("0x1234567890123456789012345678901234567890123456789012345678901234")
	addr := common.Address(owner[:20])
	id := trie.StorageTrieID(common.Hash{}, owner, common.Hash{})

	//trie, err := NewStateStackTrie(id, newTestDatabase(rawdb.NewMemoryDatabase(), rawdb.HashScheme))
	//if err != nil {
	//	t.Fatal(err)
	//}
	//
	//stRootHash, _, err := trie.UpdateStorageBatch(addr, testData)
	//if err != nil {
	//	t.Fatal(err)
	//}
	//
	//// Create an empty regular trie

	accountTrieDBWrite := triedb.NewDatabase(rawdb.NewMemoryDatabase(), config)
	regulartTrieWrite, err := trie.NewStateTrie(id, accountTrieDBWrite)
	if err != nil {
		t.Fatal(err)
	}

	regulartTrieWrite.UpdateAccount(addr, &types.StateAccount{
		Nonce:    0,
		Balance:  uint256.NewInt(0), // *uint256.Int
		Root:     common.Hash{},     // merkle root of the storage trie
		CodeHash: []byte{},
	}, 0)
	for key, value := range testData {
		valTrim := common.TrimLeftZeroes(value[:])
		fmt.Printf("write to regular trie, address: %v, key: %v, value: %v\n", addr, key[:], valTrim)
		regulartTrieWrite.UpdateStorage(addr, key[:], valTrim)
	}

	rootHash, regularNodeSet := regulartTrieWrite.Commit(true)
	//if err := triedb.Update(regularTrieRootHash, types.EmptyRootHash, trienode.NewWithNodeSet(nodes)); err != nil {
	//	panic(fmt.Errorf("failed to commit db %v", err))
	//}
	//assert.Equal(t, stRootHash, regularTrieRootHash)

	//printNodeSetStructure(t, "stack", &stackTrieNodeSet)
	//printNodeSetStructure(t, "regular", regularNodeSet)
	//
	reconstructedTrieDb := rawdb.NewMemoryDatabase()
	//reconstructedTrie, _ := NewStateTrie(id, reconstructedTrieDb)
	//// use node collected by stackTrie to reconstruct a regular trie
	regularNodeSet.ForEachWithOrder(func(path string, n *trienode.Node) {
		if !n.IsDeleted() {
			fmt.Printf("write hash: %v, blob: %v \n", n.Hash, n.Blob)
			rawdb.WriteLegacyTrieNode(reconstructedTrieDb, n.Hash, n.Blob)
		}
	})

	triedbRead := triedb.NewDatabase(reconstructedTrieDb, config)
	stateDB := NewDatabase(triedbRead, nil)
	// Create state database (shared across workers)
	state, err := New(rootHash, stateDB)
	if err != nil {
		t.Fatal(err)
	}

	for key, val := range testData {
		//expectedValue := common.TrimLeftZeroes(val[:])
		actualValue := state.GetState(addr, key)
		if err != nil {
			t.Fatalf("Failed to get value: %v", err)
		}

		if !bytes.Equal(actualValue[:], val[:]) {
			t.Errorf("❌ Value mismatch for key '%v': expected='%v', got='%v'",
				key[:], val, actualValue)
			assert.True(t, false)
		} else {
			fmt.Printf("✅ Key '%s' -> Value '%s'\n", key, val)
		}
	}

}

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
