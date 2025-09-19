package trie

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/assert"
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
