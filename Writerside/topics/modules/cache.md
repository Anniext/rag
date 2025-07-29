# RAG 缓存和性能优化模块

本模块实现了 RAG 系统的缓存和性能优化功能，包括多层缓存系统、查询性能优化和并发处理优化。

## 功能特性

### 1. 多层缓存系统 (manager.go, memory.go, redis.go)

- **CacheManager**: 统一的缓存管理接口，支持多种缓存后端
- **MemoryCache**: 内存缓存实现，支持 LRU 淘汰策略和 TTL 过期
- **RedisCache**: Redis 缓存实现，支持分布式缓存
- **混合缓存**: 支持 Redis + 内存的多层缓存架构

#### 主要功能：

- 智能缓存策略：Schema 信息和查询结果的分层缓存
- TTL 支持：支持不同类型数据的不同过期时间
- 自动清理：过期数据自动清理和内存管理
- 统计监控：详细的缓存命中率和性能统计
- 并发安全：支持高并发读写操作

### 2. 查询性能优化 (optimizer.go)

- **QueryOptimizer**: SQL 查询性能分析和优化引擎
- **慢查询检测**: 自动检测和告警慢查询
- **查询分析**: 深度分析查询复杂度和性能瓶颈
- **优化建议**: 智能生成查询优化建议

#### 主要功能：

- 查询类型识别：自动识别 SELECT、INSERT、UPDATE 等查询类型
- 表关系分析：提取表名、列名和关联关系
- 复杂度计算：基于多个维度计算查询复杂度
- 问题检测：检测 SELECT \*、缺少 LIMIT、高复杂度等问题
- 性能评分：综合评估查询性能并给出分数
- 自动优化：支持安全的查询自动优化

### 3. 并发处理优化 (pool.go)

- **ConnectionPool**: 通用连接池实现
- **LoadBalancer**: 负载均衡器（轮询、加权）
- **RateLimiter**: 令牌桶限流器
- **并发控制**: 流量控制和资源管理

#### 主要功能：

- 连接池管理：支持最小/最大连接数配置
- 健康检查：自动检测和清理无效连接
- 负载均衡：支持轮询和加权负载均衡算法
- 速率限制：令牌桶算法实现请求限流
- 统计监控：详细的连接池使用统计

## 使用示例

### 缓存管理器

```go
// 创建缓存管理器
config := &CacheConfig{
    Type:         CacheTypeMemory,
    MaxCacheSize: "100MB",
    SchemaTTL:    time.Hour,
    QueryTTL:     10 * time.Minute,
}

manager, err := NewManager(config, logger, metrics)
if err != nil {
    log.Fatal(err)
}
defer manager.Close()

// 使用缓存
ctx := context.Background()

// 设置 Schema 缓存
err = manager.SetSchemaCache(ctx, "users", schemaInfo)

// 获取 Schema 缓存
schemaInfo, err := manager.GetSchemaCache(ctx, "users")

// 设置查询结果缓存
err = manager.SetQueryCache(ctx, "SELECT * FROM users", queryResult)

// 获取查询结果缓存
result, err := manager.GetQueryCache(ctx, "SELECT * FROM users")
```

### 查询优化器

```go
// 创建查询优化器
config := &OptimizerConfig{
    SlowQueryThreshold:    time.Second,
    EnableQueryRewrite:    true,
    EnableIndexSuggestion: true,
}

optimizer := NewQueryOptimizer(config, logger, metrics)

// 分析查询
analysis, err := optimizer.AnalyzeQuery(ctx, sql, executionTime)
if err != nil {
    log.Error("Failed to analyze query", err)
}

// 检查慢查询
alert := optimizer.CheckSlowQuery(ctx, sql, executionTime, userID, sessionID)
if alert != nil {
    log.Warn("Slow query detected", "alert", alert)
}

// 优化查询
optimizedSQL, suggestions, err := optimizer.OptimizeQuery(ctx, sql)
if err != nil {
    log.Error("Failed to optimize query", err)
}
```

### 连接池

```go
// 创建连接工厂
factory := func(ctx context.Context) (Connection, error) {
    return newDatabaseConnection(ctx)
}

// 创建连接池
config := &PoolConfig{
    MinConnections:    5,
    MaxConnections:    50,
    MaxIdleTime:       30 * time.Minute,
    ConnectionTimeout: 10 * time.Second,
}

pool, err := NewPool(config, factory, logger, metrics)
if err != nil {
    log.Fatal(err)
}
defer pool.Close()

// 使用连接池
ctx := context.Background()

conn, err := pool.Get(ctx)
if err != nil {
    log.Error("Failed to get connection", err)
    return
}

// 使用连接执行操作
// ...

// 归还连接
err = pool.Put(conn)
if err != nil {
    log.Error("Failed to return connection", err)
}
```

## 配置说明

### 缓存配置

```yaml
cache:
  type: "memory" # memory, redis, hybrid
  host: "localhost"
  port: 6379
  password: ""
  database: 0
  schema_ttl: "1h"
  query_result_ttl: "10m"
  max_cache_size: "100MB"
```

### 优化器配置

```yaml
optimizer:
  slow_query_threshold: "1s"
  enable_query_plan: true
  enable_index_suggestion: true
  max_query_length: 10000
  enable_query_rewrite: true
```

### 连接池配置

```yaml
pool:
  min_connections: 5
  max_connections: 50
  max_idle_time: "30m"
  connection_timeout: "10s"
  idle_check_interval: "5m"
  health_check_enabled: true
  health_check_interval: "1m"
```

## 监控指标

系统提供了丰富的监控指标：

- `cache_operations_total`: 缓存操作总数
- `cache_hit_ratio`: 缓存命中率
- `query_analysis_duration_seconds`: 查询分析耗时
- `slow_queries_total`: 慢查询总数
- `connection_pool_total_connections`: 连接池总连接数
- `connection_pool_active_connections`: 活跃连接数

## 测试

运行所有测试：

```bash
go test ./rag/cache -v
```

运行特定测试：

```bash
go test ./rag/cache -run TestManager -v
go test ./rag/cache -run TestQueryOptimizer -v
go test ./rag/cache -run TestTokenBucket -v
```

## 性能特性

- **高并发**: 支持数千并发请求
- **低延迟**: 内存缓存亚毫秒级响应
- **高可用**: 支持缓存故障转移
- **可扩展**: 支持水平扩展和负载均衡
- **监控友好**: 丰富的指标和日志输出

## 注意事项

1. 内存缓存适合单机部署，Redis 缓存适合分布式部署
2. 查询优化器的自动优化功能默认只应用安全的优化
3. 连接池需要根据实际负载调整最小/最大连接数
4. 慢查询阈值应根据业务需求合理设置
5. 定期监控缓存命中率和连接池使用情况
