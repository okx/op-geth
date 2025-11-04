package metrics

import (
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

// StatsStore is a simple TTL store for propose stats snapshots keyed by block hash.
type StatsStore struct {
	mu    sync.Mutex
	items map[common.Hash]storedItem
	ttl   time.Duration
}

type storedItem struct {
	stat Statistics
	ts   time.Time
}

var GlobalStatsStore = NewStatsStore(60 * time.Second)

func NewStatsStore(ttl time.Duration) *StatsStore {
	return &StatsStore{items: make(map[common.Hash]storedItem), ttl: ttl}
}

func (s *StatsStore) Put(hash common.Hash, stat Statistics) {
	if s == nil || stat == nil {
		return
	}
	s.mu.Lock()
	s.items[hash] = storedItem{stat: stat, ts: time.Now()}
	s.mu.Unlock()
}

func (s *StatsStore) GetAndDelete(hash common.Hash) (Statistics, bool) {
	if s == nil {
		return nil, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	it, ok := s.items[hash]
	if !ok {
		return nil, false
	}
	delete(s.items, hash)
	return it.stat, true
}

// Cleanup removes expired items. Caller may run it periodically.
func (s *StatsStore) Cleanup() {
	if s == nil {
		return
	}
	cutoff := time.Now().Add(-s.ttl)
	s.mu.Lock()
	for k, it := range s.items {
		if it.ts.Before(cutoff) {
			delete(s.items, k)
		}
	}
	s.mu.Unlock()
}
