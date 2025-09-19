package trie

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/assert"
	"math/rand"
	"testing"
)

func TestStateStackTrieUpdateStorageBatch(t *testing.T) {
	tests := []struct {
		name              string
		storage           map[common.Hash]common.Hash
		expectedRoot      string
		expectedNodeCount int
	}{
		{
			name:              "empty storage",
			storage:           map[common.Hash]common.Hash{},
			expectedRoot:      types.EmptyRootHash.Hex(),
			expectedNodeCount: 0,
		},
		{
			name: "single storage slot",
			storage: map[common.Hash]common.Hash{
				common.HexToHash("0x1234567890123456789012345678901234567890123456789012345678901234"): common.HexToHash("0xabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdef"),
			},
			expectedRoot:      "0xacc18ecc16d7c0265035cf2d12b3769d2c68b52d8d9a54145a2fbd5c76992992",
			expectedNodeCount: 1,
		},
		{
			name: "multiple storage slots",
			storage: map[common.Hash]common.Hash{
				common.HexToHash("0x1111111111111111111111111111111111111111111111111111111111111111"): common.HexToHash("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
				common.HexToHash("0x2222222222222222222222222222222222222222222222222222222222222222"): common.HexToHash("0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"),
				common.HexToHash("0x3333333333333333333333333333333333333333333333333333333333333333"): common.HexToHash("0xcccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"),
			},
			expectedRoot:      "0xbd1385295a0cf2ba86cc030c13d37d98affb91d9882b123dd54fc31819bfd3ae",
			expectedNodeCount: 4,
		},
		{
			name: "zero values",
			storage: map[common.Hash]common.Hash{
				common.HexToHash("0x1111111111111111111111111111111111111111111111111111111111111111"): common.Hash{},                                                                          // zero value
				common.HexToHash("0x2222222222222222222222222222222222222222222222222222222222222222"): common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000001"), // non-zero
			},
			expectedRoot:      "0x4c0c2d5665951e806ca3b8c38147261e98ccd8fe3331a60b6e7413f1d34cb3c8",
			expectedNodeCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			id := TrieID(types.EmptyRootHash)

			triedb := newTestDatabase(rawdb.NewMemoryDatabase(), rawdb.HashScheme)

			stackTrie, err := NewStateStackTrie(id, triedb)
			if err != nil {
				t.Fatalf("Failed to create StateStackTrie: %v", err)
			}

			rootHash, nodeSet, err := stackTrie.UpdateStorageBatch(common.Address{}, tt.storage)
			if err != nil {
				t.Fatalf("UpdateStorageBatch failed: %v", err)
			}

			if rootHash == (common.Hash{}) {
				t.Error("Root hash should not be empty")
			}
			assert.Equal(t, tt.expectedRoot, rootHash.Hex())
			assert.Equal(t, tt.expectedNodeCount, len(nodeSet.Nodes))
		})
	}
}

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

//
//func TestStackTrieQueryConsistencywithTrie(t *testing.T) {
//
//	testData := generateRandomKeyValuePairs(2)
//	owner := common.HexToHash("0x1234567890123456789012345678901234567890123456789012345678901234")
//	addr := common.Address(owner[:20])
//	id := TrieID(types.EmptyRootHash)
//	//id.Owner = owner
//	//trie, err := NewStateStackTrie(id, newTestDatabase(rawdb.NewMemoryDatabase(), rawdb.HashScheme))
//	//if err != nil {
//	//	t.Fatal(err)
//	//}
//	//
//	//stRootHash, _, err := trie.UpdateStorageBatch(addr, testData)
//	//if err != nil {
//	//	t.Fatal(err)
//	//}
//	//
//	//// Create an empty regular trie
//	triedb := newTestDatabase(rawdb.NewMemoryDatabase(), rawdb.HashScheme)
//	regulartTrie, _ := NewStateTrie(id, triedb)
//	// update the same data into regular trie
//	regulartTrie.UpdateAccount(addr, &types.StateAccount{
//		Nonce:    0,
//		Balance:  uint256.NewInt(0), // *uint256.Int
//		Root:     common.Hash{},     // merkle root of the storage trie
//		CodeHash: []byte{},
//	}, 0)
//	for key, value := range testData {
//		valTrim := common.TrimLeftZeroes(value[:])
//		fmt.Printf("write to regular trie, address: %v, key: %v, value: %v\n", addr, key[:], valTrim)
//		regulartTrie.UpdateStorage(addr, key[:], valTrim)
//	}
//
//	_, regularNodeSet := regulartTrie.Commit(true)
//	//if err := triedb.Update(regularTrieRootHash, types.EmptyRootHash, trienode.NewWithNodeSet(nodes)); err != nil {
//	//	panic(fmt.Errorf("failed to commit db %v", err))
//	//}
//	//assert.Equal(t, stRootHash, regularTrieRootHash)
//
//	//printNodeSetStructure(t, "stack", &stackTrieNodeSet)
//	printNodeSetStructure(t, "regular", regularNodeSet)
//	//
//	reconstructedTrieDb := rawdb.NewMemoryDatabase()
//	//reconstructedTrie, _ := NewStateTrie(id, reconstructedTrieDb)
//	//// use node collected by stackTrie to reconstruct a regular trie
//	regularNodeSet.ForEachWithOrder(func(path string, n *trienode.Node) {
//		if !n.IsDeleted() {
//			fmt.Printf("write hash: %v, blob: %v \n", n.Hash, n.Blob)
//			rawdb.WriteLegacyTrieNode(reconstructedTrieDb, n.Hash, n.Blob)
//		}
//	})
//	//
//
//	// Create trie database
//	ctx := &cli.Context{}
//
//	triedbRead := utils.MakeTrieDatabase(ctx, reconstructedTrieDb, ctx.Bool(utils.CachePreimagesFlag.Name), true, false)
//	defer triedbRead.Close()
//	stateDB := state.NewDatabase(triedbRead, nil)
//	// Create state database (shared across workers)
//	state, err := state.New(id.Root, stateDB)
//	if err != nil {
//		t.Fatal(err)
//	}
//
//	for key, val := range testData {
//		//expectedValue := common.TrimLeftZeroes(val[:])
//		actualValue := state.GetState(addr, key)
//		if err != nil {
//			t.Fatalf("Failed to get value: %v", err)
//		}
//
//		if !bytes.Equal(actualValue[:], val[:]) {
//			t.Errorf("❌ Value mismatch for key '%v': expected='%v', got='%v'",
//				key[:], val, actualValue)
//			assert.True(t, false)
//		} else {
//			fmt.Printf("✅ Key '%s' -> Value '%s'\n", key, val)
//		}
//	}
//
//}
//
//func printNodeSetStructure(t *testing.T, name string, nodeSet *trienode.NodeSet) {
//	fmt.Printf("\n--- %s Node Details (Ordered) ---\n", name)
//
//	// Get node counts
//	updates, deletes := nodeSet.Size()
//	fmt.Printf("Total Updates: %d, Total Deletes: %d\n", updates, deletes)
//
//	// Use ForEachWithOrder to iterate through nodes
//	nodeSet.ForEachWithOrder(func(path string, n *trienode.Node) {
//		fmt.Printf("Path: %x\n", path)
//		fmt.Printf("  Hash: %x\n", n.Hash)
//
//		if len(n.Blob) > 0 {
//			fmt.Printf("  Blob: %x, len: %d \n", n.Blob, len(n.Blob))
//		}
//		fmt.Printf("  ---\n")
//	})
//}
