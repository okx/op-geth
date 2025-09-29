package types

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
)

type Changeset struct {
	DeletedAccounts map[common.Address]struct{}
	BalanceChanges  map[common.Address]*uint256.Int
	NonceChanges    map[common.Address]uint64
	CodeHashChanges map[common.Address]common.Hash
	CodeChanges     map[common.Hash][]byte
	StorageChanges  map[common.Address]map[common.Hash]common.Hash
}

func NewChangeset() *Changeset {
	return &Changeset{
		DeletedAccounts: make(map[common.Address]struct{}),
		BalanceChanges:  make(map[common.Address]*uint256.Int),
		NonceChanges:    make(map[common.Address]uint64),
		CodeHashChanges: make(map[common.Address]common.Hash),
		CodeChanges:     make(map[common.Hash][]byte),
		StorageChanges:  make(map[common.Address]map[common.Hash]common.Hash),
	}
}
