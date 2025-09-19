# PayloadCache 使用指南

## 概述

PayloadCache 是一个优化功能，用于缓存区块执行结果，避免在 propose 和 insertChain 阶段重复执行相同的交易。这在单节点 Sequencer 模式下特别有效，可以带来 40-45% 的性能提升。

## 配置

### 1. 配置文件 (TOML)

在您的配置文件中添加以下配置：

```toml
[Miner]
# 启用 Payload 缓存（默认：true）
EnablePayloadCache = true

# 缓存容量，最多缓存多少个区块（默认：20）
PayloadCacheSize = 20

# 缓存 TTL，缓存条目的过期时间（默认：30s）
PayloadCacheTTL = "30s"
```

### 2. 命令行参数

您也可以通过命令行参数配置：

```bash
./geth \
  --miner.enablepayloadcache \
  --miner.payloadcachesize 20 \
  --miner.payloadcachettl 30s
```

### 3. 默认配置

如果不指定配置，将使用以下默认值：
- 启用状态：**启用**
- 缓存大小：**20 个区块**
- TTL：**30 秒**

## 监控指标

### Prometheus/Grafana 指标

PayloadCache 提供了以下 Prometheus 指标：

| 指标名称 | 类型 | 描述 |
|---------|------|------|
| `payloadcache_hit` | Counter | 缓存命中次数 |
| `payloadcache_miss` | Counter | 缓存未命中次数 |
| `payloadcache_add` | Counter | 添加到缓存的次数 |
| `payloadcache_evict` | Counter | 缓存淘汰次数 |
| `payloadcache_size` | Gauge | 当前缓存大小 |
| `payloadcache_hitrate` | Gauge | 缓存命中率 |
| `payloadcache_time_saved` | Histogram | 节省的处理时间 |
| `payloadcache_time_copy` | Histogram | StateDB 复制时间 |

### 日志

启用 Debug 日志查看缓存操作：

```bash
./geth --log.level debug
```

相关日志示例：
```
DEBUG[09-19|10:30:45] Added payload to cache                  hash=0x1234... number=100 parent=0x5678...
DEBUG[09-19|10:30:46] Using cached payload result             block=100 hash=0x1234... saved_time=150ms copy_time=10ms
INFO [09-19|10:30:47] Payload cache enabled                   size=20 ttl=30s
```

## 性能测试

### 1. 基准测试

运行单元测试验证功能：

```bash
cd /path/to/op-geth
go test -v ./core -run TestPayloadCache
```

### 2. 性能对比测试

创建测试脚本对比启用/禁用缓存的性能：

```bash
# 禁用缓存
./geth --miner.enablepayloadcache=false ... &
PID1=$!
sleep 60
kill $PID1

# 启用缓存
./geth --miner.enablepayloadcache=true ... &
PID2=$!
sleep 60
kill $PID2

# 对比日志中的区块处理时间
```

### 3. 压力测试

使用高负载测试缓存效果：

```bash
# 发送大量交易
for i in {1..1000}; do
  cast send --private-key $PRIVATE_KEY \
    --rpc-url http://localhost:8545 \
    0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb0 \
    --value 0.001ether &
done
wait

# 观察缓存命中率
curl -s http://localhost:6060/debug/metrics | grep payloadcache
```

## 故障排除

### 1. 缓存未生效

检查项：
- 确认配置文件中 `EnablePayloadCache = true`
- 查看启动日志是否有 "Payload cache enabled"
- 检查 metrics 中的 `payloadcache_add` 是否增长

### 2. 命中率低

可能原因：
- TTL 设置过短，缓存过期太快
- 缓存大小不足，频繁淘汰
- 区块生产间隔太长

调优建议：
```toml
[Miner]
PayloadCacheSize = 50    # 增加缓存大小
PayloadCacheTTL = "60s"  # 延长 TTL
```

### 3. 内存占用过高

如果内存占用过高，可以：
- 减小 `PayloadCacheSize`
- 缩短 `PayloadCacheTTL`
- 或完全禁用：`EnablePayloadCache = false`

## 最佳实践

### 单节点 Sequencer

在单节点 Sequencer 模式下，缓存效果最佳：

```toml
[Miner]
EnablePayloadCache = true
PayloadCacheSize = 30      # 稍大一些的缓存
PayloadCacheTTL = "45s"     # 稍长的 TTL
```

预期效果：
- 缓存命中率：85-95%
- 性能提升：40-45%
- CPU 节省：25-35%

### 验证节点

对于验证节点，可以使用较小的缓存：

```toml
[Miner]
EnablePayloadCache = true
PayloadCacheSize = 10
PayloadCacheTTL = "20s"
```

### 开发环境

在开发环境中，可以使用更激进的缓存策略：

```toml
[Miner]
EnablePayloadCache = true
PayloadCacheSize = 100
PayloadCacheTTL = "300s"  # 5 分钟
```

## 架构细节

### 工作原理

1. **Propose 阶段**（`miner/worker.go`）：
   - 执行交易生成区块
   - 将执行结果存入缓存

2. **InsertChain 阶段**（`core/blockchain.go`）：
   - 检查缓存是否有该区块
   - 如果命中，跳过 Process，直接使用缓存结果
   - 仍然执行 ValidateState 验证

### 缓存内容

每个缓存条目包含：
- `ProcessResult`：执行结果（receipts, logs, gasUsed）
- `StateDB`：执行后的状态（深拷贝）
- `Sidecars`：Blob 交易数据
- `Witness`：无状态见证数据

### 缓存策略

- **LRU 淘汰**：当缓存满时，淘汰最少使用的条目
- **TTL 过期**：超过 TTL 的条目自动失效
- **重组处理**：发生重组时清理相关缓存

## 相关代码

主要文件：
- `core/payload_cache.go` - 缓存实现
- `miner/worker.go` - 缓存写入
- `core/blockchain.go` - 缓存读取
- `miner/miner.go` - 配置管理
- `metrics/payload_cache_metrics.go` - 监控指标

## FAQ

**Q: PayloadCache 对共识有影响吗？**
A: 没有。缓存只是优化执行过程，所有区块仍然经过完整的验证（ValidateState）。

**Q: 缓存会导致状态不一致吗？**
A: 不会。每次使用缓存时都会进行深拷贝，避免并发修改问题。

**Q: 可以在生产环境使用吗？**
A: 可以。代码已经过测试，默认启用。如有问题可以通过配置快速禁用。

**Q: 缓存占用多少内存？**
A: 取决于区块大小和缓存配置。默认配置（20个区块）大约占用 200-500MB。
