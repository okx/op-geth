package state

import (
	"fmt"
	"sort"

	"github.com/ethereum/go-ethereum/core/types"
	realtimeTypes "github.com/ethereum/go-ethereum/realtime/types"
)

type TxInfo struct {
	BlockNumber uint64
	BlockTime   uint64
	Tx          *types.Transaction
	Receipt     *types.Receipt
	InnerTxs    []*types.InnerTx
	Entries     Entries
}

type Entries struct {
	entries  *[]journalEntry
	snapshot int
}

// SetReader sets the reader for the state database.
func (s *StateDB) SetReader(reader Reader) {
	s.reader = reader
}

func CollectChangeset(entries Entries) *realtimeTypes.Changeset {
	changeset := realtimeTypes.NewChangeset()
	for _, entry := range (*entries.entries)[(entries).snapshot:] {
		entry.collectChangeset(changeset)
	}
	return changeset
}

func (s *StateDB) GenerateChangeset() *realtimeTypes.Changeset {
	entries := Entries{
		entries:  &s.journal.entries,
		snapshot: 0,
	}
	return CollectChangeset(entries)
}

func (s *StateDB) GenerateEntriesSinceSnapshot(revid int) Entries {
	// Find the snapshot in the stack of valid snapshots.
	idx := sort.Search(len(s.journal.validRevisions), func(i int) bool {
		return s.journal.validRevisions[i].id >= revid
	})
	if idx == len(s.journal.validRevisions) || s.journal.validRevisions[idx].id != revid {
		panic(fmt.Errorf("revision id %v cannot be reverted", revid))
	}
	snapshot := s.journal.validRevisions[idx].journalIndex
	return Entries{
		entries:  &s.journal.entries,
		snapshot: snapshot,
	}
}
