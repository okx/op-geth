# Block Build Result Caching and Reuse

## 1. Background and Problem

### 1.1 Current Situation

In the current op-geth implementation, each block is fully executed twice:

1. **Propose stage** (`miner/worker.go::generateWork`)
   - Execute all transactions against the parent state
   - Produce a `newPayloadResult` containing the full execution outcome
   - Create the block, receipts, logs, etc.

2. **InsertChain stage** (`core/blockchain.go::processBlock`)
   - Re-execute all transactions against the same parent state
   - Reproduce the same execution results
   - Validate state

### 1.2 Performance Impact

This duplicate execution incurs significant cost:
- CPU: redundant EVM execution
- Memory: repeated state read/write operations
- I/O: redundant database access
- Time: overall block processing time increases by 30–50%

## 2. Core Insight

### 2.1 Key Finding

After analyzing the pipeline, we rely on these facts:
- **A block’s parent is fixed**: the `ParentHash` is determined at block creation and never changes
− **Execution baseline is always consistent**: InsertChain always obtains the parent state using the block’s declared `ParentHash`
− **Results are reusable**: the inputs for both executions are identical, therefore the results are identical

### 2.2 Concurrency Scenario Analysis

Consider this sequence:
1. Head is block 100
2. Propose 101 (based on 100) but not yet inserted
3. When proposing 102, `CurrentBlock()` is still 100, so 102 is also based on 100
4. 101 finishes InsertChain and becomes head
5. 102 attempts InsertChain, competing as a sibling block

In this case:
- 101 and 102 are both based on 100, forming a short fork
- 102’s cache entry remains valid because its execution baseline is always its declared parent (100)
- Cache correctness is maintained regardless of reorgs

### 2.3 Code Verification

```go
// Propose stage - miner/worker.go:376
state, err := miner.chain.StateAt(parent.Root)

// InsertChain stage - core/blockchain.go:1865-1867
parent = bc.GetHeader(block.ParentHash(), block.NumberU64()-1)
statedb, err := state.New(parent.Root, bc.statedb)
```

Both places create state from the same `parent.Root`; the execution baseline is identical.

## 3. Solution Design

### 3.1 Architecture Overview

```
┌─────────────────┐
│  Miner/Worker   │
│  (generateWork) │
└────────┬────────┘
         │ produce newPayloadResult
         ▼
┌─────────────────┐
│  PayloadCache   │◄──── LRU-backed store
│ (keyed by hash) │
└────────┬────────┘
         │ reuse cached result
         ▼
┌─────────────────┐
│   BlockChain    │
│  (insertChain)  │
└─────────────────┘
```

### 3.2 Cache Data Structures

```go
// core/payload_cache.go
type CachedPayloadResult struct {
    // Core execution results
    ProcessResult *ProcessResult  // receipts, requests, logs, gasUsed
    StateDB       *state.StateDB  // final state after execution

    // Metadata
    BlockHash     common.Hash     // block hash as cache key
    BlockNumber   *big.Int        // block height
    Timestamp     time.Time       // TTL tracking
}

type PayloadCache struct {
    cache *lru.Cache[common.Hash, *CachedPayloadResult]
    mu    sync.RWMutex

    // Configuration
    maxSize int           // maximum entries
    ttl     time.Duration // expiration

    // Statistics
    hits    uint64
    misses  uint64
}
```

## 4. Implementation Plan

### 4.1 Cache Manager

```go
// core/payload_cache.go
func NewPayloadCache(size int, ttl time.Duration) *PayloadCache {
    return &PayloadCache{
        cache:   lru.NewCache[common.Hash, *CachedPayloadResult](size),
        maxSize: size,
        ttl:     ttl,
    }
}

func (pc *PayloadCache) Add(blockHash common.Hash, result *CachedPayloadResult) {
    pc.mu.Lock()
    defer pc.mu.Unlock()

    result.Timestamp = time.Now()
    pc.cache.Add(blockHash, result)
}

func (pc *PayloadCache) Get(blockHash common.Hash) (*CachedPayloadResult, bool) {
    pc.mu.RLock()
    defer pc.mu.RUnlock()

    if result, ok := pc.cache.Get(blockHash); ok {
        // TTL validation
        if time.Since(result.Timestamp) < pc.ttl {
            atomic.AddUint64(&pc.hits, 1)
            return result, true
        }
        // Expired, remove
        pc.cache.Remove(blockHash)
    }

    atomic.AddUint64(&pc.misses, 1)
    return nil, false
}
```

### 4.2 Write to Cache (Propose)

Modify `miner/worker.go::generateWork`:

```go
func (miner *Miner) generateWork(params *generateParams, witness bool) *newPayloadResult {
    // ... existing execution logic ...

    result := &newPayloadResult{
        block:    block,
        fees:     totalFees(block, work.receipts),
        sidecars: work.sidecars,
        stateDB:  work.state,
        receipts: work.receipts,
        requests: requests,
        witness:  work.witness,
    }

    // Cache execution result
    if miner.payloadCache != nil && miner.config.EnablePayloadCache {
        cached := &CachedPayloadResult{
            ProcessResult: &ProcessResult{
                Receipts: work.receipts,
                Requests: requests,
                Logs:     allLogs,
                GasUsed:  block.GasUsed(),
            },
            StateDB:     work.state,
            BlockHash:   block.Hash(),
            BlockNumber: block.Number(),
            Sidecars:    work.sidecars,
            Witness:     work.witness,
        }
        miner.payloadCache.Add(block.Hash(), cached)
    }

    return result
}
```

### 4.3 Read from Cache (InsertChain)

Modify `core/blockchain.go::processBlock`:

```go
func (bc *BlockChain) processBlock(block *types.Block, statedb *state.StateDB, start time.Time, setHead bool) (*blockProcessingResult, error) {
    // Try cache first
    if bc.payloadCache != nil && bc.config.EnablePayloadCache {
        if cached, ok := bc.payloadCache.Get(block.Hash()); ok {
            // Use cached execution result
            res := cached.ProcessResult

            // StateDB reuse policy (deep copy)
            cachedState := cached.StateDB.Copy()

            // Still validate to ensure correctness
            if err := bc.validator.ValidateState(block, cachedState, res, false); err != nil {
                // Validation failed, fall back
                log.Warn("Cached payload validation failed", "block", block.NumberU64(), "err", err)
                bc.payloadCache.Remove(block.Hash())
                goto NORMAL_PROCESS
            }

            // Record perf improvement
            log.Debug("Used cached payload",
                "block", block.NumberU64(),
                "saved_time", time.Since(start))

            return &blockProcessingResult{
                state:    cachedState,
                receipts: res.Receipts,
                logs:     res.Logs,
                gasUsed:  res.GasUsed,
            }, nil
        }
    }

NORMAL_PROCESS:
    // Original processing path
    res, err := bc.processor.Process(block, statedb, bc.vmConfig)
    // ... continue original logic ...
}
```

## 5. StateDB Reuse Strategy

### 5.1 Deep Copy (Recommended)

**Pros**
- Safe, no concurrency risks
- Simple to implement via `Copy()`

**Cons**
- Memory overhead
- Copy time cost

**Implementation**
```go
cachedState := cached.StateDB.Copy()
```
## 6. Forks and Reorg Handling

### 6.1 Sibling Block Scenario

When multiple blocks are based on the same parent (e.g., both 101 and 102 on 100):

```
        ┌─── Block 101 (height: 101, parent: 100)
        │
Block 100 ─┤
        │
        └─── Block 102 (height: 101, parent: 100)
```

**Cache behavior**
- 101 and 102 caches are independent and valid
- Each cache accurately reflects execution based on block 100
- InsertChain consumes the matching cache entry

### 6.2 Reorg Handling

On reorgs:
1. Old chain entries naturally expire (TTL)
2. New chain entries are reused if present
3. Cache correctness is unaffected by reorgs

### 6.3 Cache Cleanup Strategy

```go
func (bc *BlockChain) handleReorg(oldChain, newChain types.Blocks) {
    // Proactively remove reorged-out blocks
    for _, block := range oldChain {
        bc.payloadCache.Remove(block.Hash())
    }
}
```

## 7. Configuration

```toml
# config.toml
[Miner]
# Enable payload cache
EnablePayloadCache = true

# Cache capacity (number of blocks)
PayloadCacheSize = 20

# Cache TTL (seconds)
PayloadCacheTTL = 30
```
or in cli 

```
--miner.enablepayloadcache=true
--miner.payloadcachesize=20
--miner.payloadcachettl=30s
```

## 7. Performance Expectations

### 7.1 Theoretical Analysis

| Scenario | Hit Rate | Perf Gain | Notes |
|----------|----------|-----------|-------|
| Sequencer | 85–95% | 40–45% | Locally proposed blocks inserted immediately |
| Validator | 50–70% | 20–30% | Depends on network latency |
| High load | 70–80% | 30–35% | More transactions amplify benefits |

### 7.2 Key Metrics

- Execution time reduction: 40–50%
- CPU usage reduction: 25–35%
- Memory overhead: ~200MB for 20 cached blocks
- Cache lookup overhead: < 1ms

## 8. Observability

### 8.1 Core Metrics

```go
// Cache effectiveness
payloadcache/hitrate            // hit rate
payloadcache/size               // current cache size
payloadcache/evict              // evictions

// Performance
payloadcache/time/saved         // time saved via cache
payloadcache/time/copy          // StateDB copy time
```

### 8.2 Dashboarding

Recommended Grafana panels:
- Cache hit rate trend
- Performance improvement vs. baseline
- Anomaly alerts

## 9. Risks and Mitigations

### 9.1 Potential Risks

| Risk | Impact | Mitigation |
|------|--------|------------|
| Memory leak | Unbounded memory | LRU eviction + TTL expiry |
| State inconsistency | Consensus failure | Keep `ValidateState` in place |
| Cache avalanche | Sharp perf drop | Progressive expiry + warmup |
| Fork handling | Sibling competition | Cache bound to declared parent |

### 9.2 Degradation Strategy

1. Config switch: runtime enable/disable
2. Auto degrade: disable if hit rate < 20%
3. Memory guard: aggressively clean under pressure

## 10. Conclusion

By introducing a cache between Propose and InsertChain, we avoid redundant transaction execution. Since both stages share the same execution baseline, the cache is simple and reliable. We expect a 30–45% performance gain, especially in sequencer mode with frequent block production.

## Appendix A: Code Map

- Propose: `miner/worker.go::generateWork`
- InsertChain: `core/blockchain.go::processBlock`
- StateProcessor: `core/state_processor.go::Process`
- BlockValidator: `core/block_validator.go::ValidateState`

## Appendix B: References

- [Ethereum Yellow Paper](https://ethereum.github.io/yellowpaper/paper.pdf)
- [go-ethereum State Processing](https://github.com/ethereum/go-ethereum/wiki/Design-Rationale)
- [LRU Cache Implementation](https://github.com/hashicorp/golang-lru)
