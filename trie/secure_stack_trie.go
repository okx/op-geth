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
	//TODO implement me
	panic("implement me")
}

func (t *StateStackTrie) GetAccount(address common.Address) (*types.StateAccount, error) {
	//TODO implement me
	panic("implement me")
}

func (t *StateStackTrie) GetStorage(addr common.Address, key []byte) ([]byte, error) {
	//TODO implement me
	panic("implement me")
}

func (t *StateStackTrie) UpdateAccount(address common.Address, account *types.StateAccount, codeLen int) error {
	//TODO implement me
	panic("implement me")
}

func (t *StateStackTrie) UpdateStorage(addr common.Address, key, value []byte) error {
	//TODO implement me
	panic("implement me")
}

func (t *StateStackTrie) DeleteAccount(address common.Address) error {
	//TODO implement me
	panic("implement me")
}

func (t *StateStackTrie) DeleteStorage(addr common.Address, key []byte) error {
	//TODO implement me
	panic("implement me")
}

func (t *StateStackTrie) UpdateContractCode(address common.Address, codeHash common.Hash, code []byte) error {
	//TODO implement me
	panic("implement me")
}

func (t *StateStackTrie) Hash() common.Hash {
	//TODO implement me
	panic("implement me")
}

func (t *StateStackTrie) Commit(collectLeaf bool) (common.Hash, *trienode.NodeSet) {
	//TODO implement me
	panic("implement me")
}

func (t *StateStackTrie) Witness() map[string]struct{} {
	//TODO implement me
	panic("implement me")
}

func (t *StateStackTrie) NodeIterator(startKey []byte) (NodeIterator, error) {
	//TODO implement me
	panic("implement me")
}

func (t *StateStackTrie) Prove(key []byte, proofDb ethdb.KeyValueWriter) error {
	//TODO implement me
	panic("implement me")
}

func (t *StateStackTrie) IsVerkle() bool {
	//TODO implement me
	panic("implement me")
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

func (t *StateStackTrie) UpdateStorageBatch(_ common.Address, storage map[common.Hash]common.Hash) (common.Hash, trienode.NodeSet, error) {

	rawStorage := make(map[[32]byte][]byte, len(storage))

	hashKeys := make([][32]byte, 0, len(storage))
	for k, v := range storage {

		hk := t.hashKey(k[:])
		v, _ := rlp.EncodeToBytes(common.TrimLeftZeroes(v[:]))

		rawHashKey := [32]byte{}
		copy(rawHashKey[:], hk[:32])

		rawStorage[rawHashKey] = v

		hashKeys = append(hashKeys, rawHashKey)

	}

	sort.Slice(hashKeys, func(i, j int) bool {
		return bytes.Compare(hashKeys[i][:], hashKeys[j][:]) < 0
	})

	for _, key := range hashKeys {
		if err := t.trie.Update(key[:], rawStorage[key]); err != nil {
			panic(err)
		}
	}
	rootHash := t.trie.Hash()

	return rootHash, t.nodeSet, nil
}
