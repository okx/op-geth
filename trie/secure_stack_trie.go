package trie

import (
	"bytes"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie/trienode"
	"github.com/ethereum/go-ethereum/triedb/database"
	"sort"
)

type StateStackTrie struct {
	trie       StackTrie
	id         *ID
	db         database.NodeDatabase
	preimages  preimageStore
	hashKeyBuf [common.HashLength]byte
	nodeSet    trienode.NodeSet
}

func (t *StateStackTrie) GetKey(i []byte) []byte {
	//TODO unreachable
	panic("unreachable")
}

func (t *StateStackTrie) GetAccount(address common.Address) (*types.StateAccount, error) {
	//TODO unreachable
	panic("unreachable")
}

func (t *StateStackTrie) GetStorage(addr common.Address, key []byte) ([]byte, error) {
	//TODO unreachable
	panic("unreachable")
}

func (t *StateStackTrie) UpdateAccount(address common.Address, account *types.StateAccount, codeLen int) error {
	//TODO unreachable
	panic("unreachable")
}

func (t *StateStackTrie) UpdateStorage(addr common.Address, key, value []byte) error {
	//TODO unreachable
	panic("unreachable")
}

func (t *StateStackTrie) DeleteAccount(address common.Address) error {
	//TODO unreachable
	panic("unreachable")
}

func (t *StateStackTrie) DeleteStorage(addr common.Address, key []byte) error {
	//TODO unreachable
	panic("unreachable")
}

func (t *StateStackTrie) UpdateContractCode(address common.Address, codeHash common.Hash, code []byte) error {
	//TODO unreachable
	panic("unreachable")
}

func (t *StateStackTrie) Hash() common.Hash {
	//TODO unreachable
	panic("unreachable")
}

func (t *StateStackTrie) Commit(collectLeaf bool) (common.Hash, *trienode.NodeSet) {
	//TODO unreachable
	panic("unreachable")
}

func (t *StateStackTrie) Witness() map[string]struct{} {
	//TODO unreachable
	panic("unreachable")
}

func (t *StateStackTrie) NodeIterator(startKey []byte) (NodeIterator, error) {
	//TODO unreachable
	panic("unreachable")
}

func (t *StateStackTrie) Prove(key []byte, proofDb ethdb.KeyValueWriter) error {
	//TODO unreachable
	panic("unreachable")
}

func (t *StateStackTrie) IsVerkle() bool {
	//TODO unreachable
	panic("unreachable")
}

func NewStateStackTrie(id *ID, db database.NodeDatabase) (*StateStackTrie, error) {

	stackTrieNodeSet := trienode.NewNodeSet(id.Owner)
	onTrieNode := func(path []byte, hash common.Hash, blob []byte) {
		blobCopy := make([]byte, len(blob))
		copy(blobCopy, blob) // must copy here, as the stackTrie hash function will mutate this slice internally
		stackTrieNodeSet.AddNode(path, trienode.New(hash, blobCopy))
	}

	// Create StackTrie
	st := NewStackTrie(onTrieNode)
	tr := &StateStackTrie{trie: *st, db: db, id: id, nodeSet: *stackTrieNodeSet}

	// link the preimage store if it's supported
	preimages, ok := db.(preimageStore)
	if ok {
		tr.preimages = preimages
	}
	return tr, nil
}

func (t *StateStackTrie) hashKey(key []byte) []byte {
	h := newHasher(false)
	h.sha.Reset()
	h.sha.Write(key)
	h.sha.Read(t.hashKeyBuf[:])
	returnHasherToPool(h)
	return t.hashKeyBuf[:]
}

type KeyHashPair struct {
	HashKey   [32]byte
	EncodeVal []byte
}

func (t *StateStackTrie) UpdateStorageBatch(_ common.Address, storage map[common.Hash]common.Hash) (common.Hash, trienode.NodeSet, error) {
	keyHashPairs := make([]KeyHashPair, 0, len(storage))

	for k, val := range storage {
		encodeV, _ := rlp.EncodeToBytes(common.TrimLeftZeroes(val[:]))
		hk := t.hashKey(k[:])
		tmpHashKey := [32]byte{}
		copy(tmpHashKey[:], hk)

		tmpEncVal := make([]byte, len(encodeV))
		copy(tmpEncVal[:], encodeV)

		keyHashPairs = append(keyHashPairs, KeyHashPair{
			HashKey:   tmpHashKey,
			EncodeVal: tmpEncVal[:],
		})
	}

	sort.Slice(keyHashPairs, func(i, j int) bool {
		return bytes.Compare(keyHashPairs[i].HashKey[:], keyHashPairs[j].HashKey[:]) < 0
	})

	for _, pair := range keyHashPairs {
		if err := t.trie.Update(pair.HashKey[:], pair.EncodeVal[:]); err != nil {
			panic(err)
		}
	}
	rootHash := t.trie.Hash()

	return rootHash, t.nodeSet, nil
}
