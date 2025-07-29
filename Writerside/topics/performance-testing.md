# RAG 性能测试指南

本文档描述了如何运行和理解 RAG 系统的性能测试。

## 概述

RAG 性能测试套件包含三种主要的测试类型：

1. **并发查询压力测试** - 测试系统在高并发查询下的性能表现
2. **内存使用压力测试** - 测试系统在大量会话和查询下的内存使用情况
3. **长时间稳定性测试** - 测试系统长时间运行的稳定性和资源使用

## 快速开始

### 使用脚本运行测试

最简单的方式是使用提供的脚本：

```bash
# 运行所有测试（默认配置）
./run_performance_tests.sh

# 运行特定测试
./run_performance_tests.sh concurrent -d 30s -c 10
./run_performance_tests.sh memory -s 100 -q 5
./run_performance_tests.sh stability -d 5m

# 查看帮助
./run_performance_tests.sh --help
```

### 直接使用 Go 测试

```bash
# 设置环境变量启用测试
export STRESS_TEST=true

# 运行并发查询测试
STRESS_TEST_DURATION=30s STRESS_MAX_CONCURRENCY=10 go test -v -run TestStressConcurrentQueries

# 运行内存压力测试
STRESS_MEMORY_SESSIONS=100 STRESS_QUERIES_PER_SESSION=5 go test -v -run TestStressMemoryUsage

# 运行稳定性测试
STRESS_TEST_DURATION=5m go test -v -run TestStressLongRunningStability
```

## 测试类型详解

### 1. 并发查询压力测试 (TestStressConcurrentQueries)

**目的**: 测试系统在高并发查询负载下的性能表现。

**测试流程**:

1. 系统预热（默认 5 秒）
2. 启动多个并发工作协程
3. 持续发送查询请求
4. 收集性能指标

**关键指标**:

- 总请求数和成功率
- 平均延迟、P50、P95、P99 延迟
- 每秒请求数 (QPS)
- 内存使用情况
- 协程数量变化

**配置参数**:

- `STRESS_MAX_CONCURRENCY`: 最大并发数（默认 50）
- `STRESS_TEST_DURATION`: 测试持续时间（默认 2 分钟）
- `STRESS_REQUESTS_PER_SECOND`: 每秒请求数（默认 100）

### 2. 内存使用压力测试 (TestStressMemoryUsage)

**目的**: 测试系统在大量会话和查询下的内存使用情况，检测内存泄漏。

**测试流程**:

1. 创建大量用户会话
2. 每个会话执行多个查询
3. 监控内存使用情况
4. 清理所有会话
5. 强制垃圾回收

**关键指标**:

- 初始、峰值、最终内存使用
- 内存增长量
- GC 次数和暂停时间
- 堆对象数量

**配置参数**:

- `STRESS_MEMORY_SESSIONS`: 创建的会话数（默认 1000）
- `STRESS_QUERIES_PER_SESSION`: 每会话查询数（默认 10）
- `STRESS_MEMORY_LIMIT_MB`: 内存限制（默认 1000MB）

### 3. 长时间稳定性测试 (TestStressLongRunningStability)

**目的**: 测试系统长时间运行的稳定性，检测资源泄漏和性能退化。

**测试流程**:

1. 启动多个长期运行的工作协程
2. 持续执行查询请求
3. 定期检查系统资源使用
4. 监控性能指标变化

**关键指标**:

- 长期运行的错误率
- 延迟分布的稳定性
- 内存使用的增长趋势
- 协程数量的变化

**配置参数**:

- `STRESS_TEST_DURATION`: 测试持续时间（默认 2 分钟）
- `STRESS_MAX_CONCURRENCY`: 并发数（默认 50）

## 环境变量配置

| 变量名                       | 描述             | 默认值    |
| ---------------------------- | ---------------- | --------- |
| `STRESS_TEST`                | 启用压力测试     | false     |
| `STRESS_DB_DSN`              | 数据库连接字符串 | Mock 模式 |
| `STRESS_MAX_CONCURRENCY`     | 最大并发数       | 50        |
| `STRESS_TEST_DURATION`       | 测试持续时间     | 2m        |
| `STRESS_WARMUP_DURATION`     | 预热时间         | 5s        |
| `STRESS_REQUESTS_PER_SECOND` | 每秒请求数       | 100       |
| `STRESS_MEMORY_LIMIT_MB`     | 内存限制         | 1000      |
| `STRESS_MEMORY_SESSIONS`     | 内存测试会话数   | 1000      |
| `STRESS_QUERIES_PER_SESSION` | 每会话查询数     | 10        |
| `STRESS_ENABLE_PROFILING`    | 启用性能分析     | false     |

## 性能指标解读

### 延迟指标

- **平均延迟 (AvgLatency)**: 所有请求的平均响应时间
- **P50 延迟**: 50%的请求响应时间低于此值
- **P95 延迟**: 95%的请求响应时间低于此值
- **P99 延迟**: 99%的请求响应时间低于此值

### 吞吐量指标

- **QPS (Requests Per Second)**: 每秒处理的请求数
- **错误率 (Error Rate)**: 失败请求占总请求的百分比

### 资源使用指标

- **内存使用**: 初始、峰值、最终内存使用量
- **内存增长**: 测试期间的内存增长量
- **GC 统计**: 垃圾回收次数和暂停时间
- **协程数量**: 协程的创建和销毁情况

## 性能基准

### 预期性能指标

在标准测试环境下（MacBook Pro M1, 16GB RAM），预期的性能指标：

**并发查询测试**:

- QPS: > 10
- 平均延迟: < 1 秒
- P95 延迟: < 2 秒
- 错误率: < 10%

**内存压力测试**:

- 峰值内存: < 2GB
- 内存增长: < 500MB
- 错误率: < 20%

**稳定性测试**:

- 错误率: < 5%
- 内存增长: < 200MB
- 协程增长: < 100

### 性能建议

测试完成后，系统会自动生成性能建议：

- **QPS 较低**: 建议优化系统性能或增加并发处理能力
- **延迟过高**: 建议优化查询性能或增加缓存
- **内存增长过多**: 建议检查内存泄漏
- **协程增长过多**: 建议检查协程泄漏
- **存在异常慢查询**: 建议检查查询优化和索引

## 故障排除

### 常见问题

1. **测试超时**

   - 减少测试持续时间或并发数
   - 检查数据库连接是否正常

2. **内存使用过高**

   - 减少会话数量或每会话查询数
   - 检查是否存在内存泄漏

3. **错误率过高**

   - 检查数据库连接配置
   - 查看详细错误日志

4. **QPS 过低**
   - 检查系统资源使用情况
   - 优化查询处理逻辑

### 调试技巧

1. **启用详细日志**:

   ```bash
   ./run_performance_tests.sh all -v
   ```

2. **使用较小的测试参数**:

   ```bash
   ./run_performance_tests.sh concurrent -d 10s -c 3
   ```

3. **检查系统资源**:

   ```bash
   # 监控内存使用
   top -pid $(pgrep -f "go test")

   # 监控网络连接
   netstat -an | grep 3306
   ```

## 持续集成

### GitHub Actions 集成

```yaml
name: Performance Tests

on:
  schedule:
    - cron: "0 2 * * *" # 每天凌晨2点运行
  workflow_dispatch:

jobs:
  performance:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v4
        with:
          go-version: "1.24"

      - name: Run Performance Tests
        run: |
          cd rag
          ./run_performance_tests.sh all -d 1m -c 5
```

### 性能回归检测

建议定期运行性能测试，并记录关键指标的变化趋势：

```bash
# 记录性能基准
./run_performance_tests.sh all > performance_baseline.log

# 比较性能变化
./run_performance_tests.sh all > performance_current.log
diff performance_baseline.log performance_current.log
```

## 扩展测试

### 自定义测试场景

可以通过修改 `stress_test.go` 文件来添加自定义的测试场景：

```go
func TestCustomStressScenario(t *testing.T) {
    if os.Getenv("CUSTOM_STRESS_TEST") != "true" {
        t.Skip("跳过自定义压力测试")
    }

    // 自定义测试逻辑
}
```

### 集成真实数据库

默认情况下，测试使用 Mock 数据库。要使用真实数据库：

```bash
export STRESS_DB_DSN="user:password@tcp(localhost:3306)/testdb"
./run_performance_tests.sh
```

## 总结

RAG 性能测试套件提供了全面的性能评估工具，帮助开发者：

1. 验证系统在高负载下的性能表现
2. 检测内存泄漏和资源使用问题
3. 确保系统长期运行的稳定性
4. 获得性能优化的具体建议

定期运行这些测试可以帮助维护系统的高性能和稳定性。
