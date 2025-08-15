package rtclient

import (
	"context"
)

// RealtimeDumpStateCache dumps the state cache
func (rc *RealtimeClient) RealtimeDumpCache(ctx context.Context) error {
	var result interface{}
	err := rc.c.CallContext(ctx, &result, "debug_realtimeDumpCache")
	if err != nil {
		return err
	}
	return nil
}

// RealtimeDumpStateCache dumps the state cache
func (rc *RealtimeClient) RealtimeCompareStateCache(ctx context.Context) ([]string, error) {
	var result RealtimeDebugResult
	err := rc.c.CallContext(ctx, &result, "debug_realtimeCompareStateCache")
	if err != nil {
		return nil, err
	}
	return result.Mismatches, nil
}
