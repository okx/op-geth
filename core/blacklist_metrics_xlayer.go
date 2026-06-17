// XLayer emergency-freeze blacklist — observability metrics (XLOP-1099, FR-7).
//
// Fork-local XLayer extension (KG naming rule). Reuses the standard
// go-ethereum metrics registry (same pattern as legacypool's metrics). Metric
// names are slash-separated; the Prometheus exporter renders them as
// xlayer_blacklist_* to match xlayer-reth.

package core

import "github.com/ethereum/go-ethereum/metrics"

var (
	// blacklistCacheSize is the current block-head snapshot set size.
	// Prometheus: xlayer_blacklist_cache_size.
	blacklistCacheSize = metrics.NewRegisteredGauge("xlayer/blacklist/cache_size", nil)

	// blacklistSnapshotReadDuration observes block-head snapshot read latency in
	// nanoseconds (go-ethereum's native duration unit; the metrics Histogram is
	// int64 and the Prometheus exporter applies no unit scaling, so the name must
	// state the recorded unit). Prometheus: xlayer_blacklist_snapshot_read_duration_nanoseconds.
	blacklistSnapshotReadDuration = metrics.NewRegisteredHistogram("xlayer/blacklist/snapshot_read_duration_nanoseconds", nil, metrics.NewExpDecaySample(1028, 0.015))

	// blacklistExecRevert counts execution-gate reverts, labeled by the matched
	// hook category. Prometheus: xlayer_blacklist_exec_revert_total{hook=...}.
	blacklistExecRevert = map[string]*metrics.Counter{
		HookLog:          metrics.NewRegisteredCounter("xlayer/blacklist/exec_revert/log", nil),
		HookSelfdestruct: metrics.NewRegisteredCounter("xlayer/blacklist/exec_revert/selfdestruct", nil),
		HookEthBalance:   metrics.NewRegisteredCounter("xlayer/blacklist/exec_revert/eth_balance", nil),
	}
)

// MetricBlacklistCacheSize records the current snapshot size after a refresh.
func MetricBlacklistCacheSize(size int) { blacklistCacheSize.Update(int64(size)) }

// MetricBlacklistSnapshotRead observes a block-head snapshot read latency (ns).
func MetricBlacklistSnapshotRead(nanos int64) { blacklistSnapshotReadDuration.Update(nanos) }

// MetricBlacklistExecRevert increments the execution-gate revert counter for the
// given hook category (call|log|selfdestruct|eth_balance). Unknown categories
// are ignored.
func MetricBlacklistExecRevert(category string) {
	if c, ok := blacklistExecRevert[category]; ok {
		c.Inc(1)
	}
}
