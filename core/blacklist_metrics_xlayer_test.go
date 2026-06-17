package core

import "testing"

func TestBlacklistMetrics(t *testing.T) {
	// exec_revert counters increment for known categories and ignore unknown.
	for _, cat := range []string{HookLog, HookSelfdestruct, HookEthBalance} {
		before := blacklistExecRevert[cat].Snapshot().Count()
		MetricBlacklistExecRevert(cat)
		if got := blacklistExecRevert[cat].Snapshot().Count(); got != before+1 {
			t.Fatalf("exec_revert[%s] = %d, want %d", cat, got, before+1)
		}
	}
	// Unknown category must be a safe no-op.
	MetricBlacklistExecRevert("nonexistent")

	// cache_size gauge tracks the latest value.
	MetricBlacklistCacheSize(42)
	if got := blacklistCacheSize.Snapshot().Value(); got != 42 {
		t.Fatalf("cache_size = %d, want 42", got)
	}

	// snapshot read duration observe must not panic.
	MetricBlacklistSnapshotRead(1234)
}
