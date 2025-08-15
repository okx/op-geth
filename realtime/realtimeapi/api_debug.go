package realtimeapi

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/internal/ethapi"
	"github.com/ethereum/go-ethereum/log"
	realtimeCache "github.com/ethereum/go-ethereum/realtime/cache"
	"github.com/ethereum/go-ethereum/rpc"
)

type RealtimeDebugApiImpl struct {
	cacheDB *realtimeCache.RealtimeCache
	b       ethapi.Backend
}

func NewRealtimeDebugAPI(
	cacheDB *realtimeCache.RealtimeCache,
	base ethapi.Backend,
) *RealtimeDebugApiImpl {
	return &RealtimeDebugApiImpl{
		cacheDB: cacheDB,
		b:       base,
	}
}

func (api *RealtimeDebugApiImpl) RealtimeDumpCache(ctx context.Context) error {
	if api.cacheDB == nil || !api.cacheDB.ReadyFlag.Load() {
		// Custom for realtime
		return ErrRealtimeNotEnabled
	}

	if err := api.cacheDB.DebugDumpToFile(); err != nil {
		log.Error("[Realtime] Failed to dump state cache", "error", err)
		return fmt.Errorf("failed to dump state cache: %v", err)
	}

	return nil
}

func (api *RealtimeDebugApiImpl) RealtimeCompareStateCache(ctx context.Context) (*RealtimeDebugResult, error) {
	if api.cacheDB == nil || !api.cacheDB.ReadyFlag.Load() {
		// Custom for realtime
		return nil, ErrRealtimeNotEnabled
	}

	reader, _, err := api.b.StateAndHeaderByNumber(ctx, rpc.LatestBlockNumber)
	if err != nil {
		return nil, fmt.Errorf("compareStateCache cannot create latest state reader: %w", err)
	}

	return &RealtimeDebugResult{
		ConfirmHeight:   api.cacheDB.GetHighestConfirmHeight(),
		ExecutionHeight: api.cacheDB.GetExecutionHeight(),
		Mismatches:      api.cacheDB.State.DebugCompare(reader),
	}, nil
}
