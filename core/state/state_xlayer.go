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
	entries  []journalEntry
	snapshot int
}

// SetReader sets the reader for the state database.
func (s *StateDB) SetReaderXLayer(reader Reader) {
	s.reader = reader
	s.RealtimeReaderFlag = true
}

func CollectChangeset(entries Entries) *realtimeTypes.Changeset {
	changeset := realtimeTypes.NewChangeset()
	for _, entry := range (entries.entries)[(entries).snapshot:] {
		entry.collectChangeset(changeset)
	}
	return changeset
}

func (s *StateDB) GenerateChangeset() *realtimeTypes.Changeset {
	ce := make([]journalEntry, len(s.journal.entries))
	for i, entry := range s.journal.entries {
		ce[i] = entry.copy()
	}
	entries := Entries{
		entries:  ce,
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
	entries := make([]journalEntry, len(s.journal.entries))
	for i, entry := range s.journal.entries {
		entries[i] = entry.copy()
	}
	return Entries{
		entries:  entries,
		snapshot: snapshot,
	}
}
