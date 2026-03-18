package metrics

import (
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

func TestStatsStorePutGetDelete(t *testing.T) {
	store := NewStatsStore(5 * time.Second)
	h := common.HexToHash("0x01")

	// Use a real Statistics implementation
	stat := NewLogStatistics()
	stat.CumulativeTiming(ProposeTotalMs, 10*time.Millisecond)

	// Put
	store.Put(h, stat)

	// GetAndDelete should return the same object
	got, ok := store.GetAndDelete(h)
	if !ok {
		t.Fatalf("expected ok=true from GetAndDelete")
	}
	if got == nil {
		t.Fatalf("expected non-nil Statistics from store")
	}
	if got != stat {
		t.Fatalf("expected retrieved Statistics to equal original instance")
	}

	// Second GetAndDelete should miss
	if _, ok2 := store.GetAndDelete(h); ok2 {
		t.Fatalf("expected ok=false on second GetAndDelete after deletion")
	}
}

func TestStatsStoreCleanupTTL(t *testing.T) {
	store := NewStatsStore(10 * time.Millisecond)
	h1 := common.HexToHash("0x11")
	h2 := common.HexToHash("0x22")

	s1 := NewLogStatistics()
	s2 := NewLogStatistics()

	store.Put(h1, s1)
	store.Put(h2, s2)

	// Ensure entries exist initially
	if _, ok := store.GetAndDelete(common.HexToHash("0xdeadbeef")); ok {
		t.Fatalf("unexpected hit for non-existent key")
	}

	// Sleep longer than TTL and cleanup
	time.Sleep(20 * time.Millisecond)
	store.Cleanup()

	// Both entries should be expired and not retrievable
	if _, ok := store.GetAndDelete(h1); ok {
		t.Fatalf("expected h1 to be expired after Cleanup")
	}
	if _, ok := store.GetAndDelete(h2); ok {
		t.Fatalf("expected h2 to be expired after Cleanup")
	}
}
