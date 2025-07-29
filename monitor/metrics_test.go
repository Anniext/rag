package monitor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMetricsManager(t *testing.T) {
	manager := NewMetricsManager()
	assert.NotNil(t, manager)
	assert.NotNil(t, manager.queryErrors)
	assert.NotNil(t, manager.llmProviderStats)
	assert.NotNil(t, manager.cacheStats)
	assert.NotNil(t, manager.timeWindows)
	assert.NotNil(t, manager.systemMetrics)
}

func TestMetricsManager_QueryMetrics(t *testing.T) {
	manager := NewMetricsManager()

	// 记录一些查询指标
	manager.RecordQuerySuccess()
	manager.RecordQueryDuration(100 * time.Millisecond)
	manager.RecordQueryResult(10)

	manager.RecordQueryError("validation_error")
	manager.RecordQueryDuration(200 * time.Millisecond)
	manager.RecordQueryResult(0)

	manager.RecordQuerySuccess()
	manager.RecordQueryDuration(150 * time.Millisecond)
	manager.RecordQueryResult(5)

	// 获取查询指标
	metrics := manager.GetQueryMetrics()

	assert.Equal(t, int64(3), metrics.TotalQueries)
	assert.Equal(t, int64(2), metrics.SuccessQueries)
	assert.Equal(t, int64(1), metrics.ErrorQueries)
	assert.InDelta(t, 0.667, metrics.SuccessRate, 0.01)
	assert.InDelta(t, 0.333, metrics.ErrorRate, 0.01)
	assert.Equal(t, 150*time.Millisecond, metrics.AvgDuration)
	assert.InDelta(t, 5.0, metrics.AvgRowCount, 0.01)
	assert.Equal(t, int64(1), metrics.ErrorsByType["validation_error"])
}

func TestMetricsManager_LLMMetrics(t *testing.T) {
	manager := NewMetricsManager()

	// 记录LLM调用
	manager.RecordLLMCall("openai", "gpt-4", 100, 50, 2*time.Second, 0.05)
	manager.RecordLLMCall("openai", "gpt-4", 200, 100, 3*time.Second, 0.10)
	manager.RecordLLMCall("anthropic", "claude", 150, 75, 2500*time.Millisecond, 0.08)
	manager.RecordLLMError("openai", "gpt-4", "rate_limit")

	// 获取LLM指标
	metrics := manager.GetLLMMetrics()

	assert.Equal(t, int64(3), metrics.TotalCalls)
	assert.Equal(t, int64(1), metrics.TotalErrors)
	assert.InDelta(t, 0.667, metrics.SuccessRate, 0.01)
	assert.Equal(t, int64(450), metrics.TotalPromptTokens)
	assert.Equal(t, int64(225), metrics.TotalCompletionTokens)
	assert.Equal(t, int64(675), metrics.TotalTokens)
	assert.InDelta(t, 0.23, metrics.TotalCost, 0.01)
	assert.InDelta(t, 0.077, metrics.AvgCostPerCall, 0.01)

	// 检查提供商统计
	assert.Len(t, metrics.ProviderStats, 2)

	openaiStats := metrics.ProviderStats["openai:gpt-4"]
	require.NotNil(t, openaiStats)
	assert.Equal(t, int64(2), openaiStats.CallCount)
	assert.Equal(t, int64(1), openaiStats.ErrorCount)
	assert.Equal(t, int64(300), openaiStats.PromptTokens)
	assert.Equal(t, int64(150), openaiStats.CompletionTokens)

	claudeStats := metrics.ProviderStats["anthropic:claude"]
	require.NotNil(t, claudeStats)
	assert.Equal(t, int64(1), claudeStats.CallCount)
	assert.Equal(t, int64(0), claudeStats.ErrorCount)
}

func TestMetricsManager_SystemMetrics(t *testing.T) {
	manager := NewMetricsManager()

	// 记录系统指标
	manager.RecordSystemMetrics()

	// 获取系统指标
	metrics := manager.GetSystemMetrics()

	assert.Greater(t, metrics.MemoryUsage, int64(0))
	assert.Greater(t, metrics.MemoryTotal, int64(0))
	assert.GreaterOrEqual(t, metrics.MemoryUsagePercent, 0.0)
	assert.Greater(t, metrics.GoroutineCount, 0)
	assert.GreaterOrEqual(t, metrics.GCCount, uint32(0))
	assert.Greater(t, metrics.HeapSize, int64(0))
	assert.Greater(t, metrics.HeapInUse, int64(0))
	assert.False(t, metrics.Timestamp.IsZero())
}

func TestMetricsManager_DatabaseMetrics(t *testing.T) {
	manager := NewMetricsManager()

	// 记录数据库指标
	manager.RecordDBConnection(5, 3, 10)
	manager.RecordDBQuery(50*time.Millisecond, true)
	manager.RecordDBQuery(100*time.Millisecond, true)
	manager.RecordDBQuery(200*time.Millisecond, false)

	// 获取数据库指标
	metrics := manager.GetDBMetrics()

	assert.Equal(t, int64(5), metrics.ActiveConnections)
	assert.Equal(t, int64(3), metrics.IdleConnections)
	assert.Equal(t, int64(10), metrics.TotalConnections)
	assert.Equal(t, int64(3), metrics.TotalQueries)
	assert.Equal(t, int64(2), metrics.SuccessQueries)
	assert.InDelta(t, 0.667, metrics.SuccessRate, 0.01)
	assert.Equal(t, 116*time.Millisecond+666*time.Microsecond, metrics.AvgQueryDuration)
}

func TestMetricsManager_CacheMetrics(t *testing.T) {
	manager := NewMetricsManager()

	// 记录缓存指标
	manager.RecordCacheHit("redis")
	manager.RecordCacheHit("redis")
	manager.RecordCacheMiss("redis")
	manager.RecordCacheOperation("redis", "get", 10*time.Millisecond)
	manager.RecordCacheOperation("redis", "set", 20*time.Millisecond)
	manager.RecordCacheOperation("redis", "get", 15*time.Millisecond)

	manager.RecordCacheHit("memory")
	manager.RecordCacheMiss("memory")
	manager.RecordCacheMiss("memory")

	// 获取缓存指标
	metrics := manager.GetCacheMetrics()

	assert.Len(t, metrics.Stats, 2)

	redisStats := metrics.Stats["redis"]
	require.NotNil(t, redisStats)
	assert.Equal(t, int64(2), redisStats.HitCount)
	assert.Equal(t, int64(1), redisStats.MissCount)
	assert.Equal(t, int64(3), redisStats.TotalRequests)
	assert.InDelta(t, 0.667, redisStats.HitRate, 0.01)
	assert.InDelta(t, 0.333, redisStats.MissRate, 0.01)

	// 检查操作统计
	assert.Len(t, redisStats.OperationStats, 2)
	getStats := redisStats.OperationStats["get"]
	require.NotNil(t, getStats)
	assert.Equal(t, int64(2), getStats.Count)
	assert.Equal(t, 25*time.Millisecond, getStats.TotalDuration)
	assert.Equal(t, 12*time.Millisecond+500*time.Microsecond, getStats.AvgDuration)

	memoryStats := metrics.Stats["memory"]
	require.NotNil(t, memoryStats)
	assert.Equal(t, int64(1), memoryStats.HitCount)
	assert.Equal(t, int64(2), memoryStats.MissCount)
	assert.InDelta(t, 0.333, memoryStats.HitRate, 0.01)
	assert.InDelta(t, 0.667, memoryStats.MissRate, 0.01)
}

func TestMetricsManager_GetMetrics(t *testing.T) {
	manager := NewMetricsManager()

	// 记录各种指标
	manager.RecordQuerySuccess()
	manager.RecordQueryDuration(100 * time.Millisecond)
	manager.RecordLLMCall("openai", "gpt-4", 100, 50, 2*time.Second, 0.05)
	manager.RecordSystemMetrics()
	manager.RecordDBConnection(5, 3, 10)
	manager.RecordCacheHit("redis")

	// 获取完整指标快照
	snapshot := manager.GetMetrics()

	assert.NotNil(t, snapshot.Query)
	assert.NotNil(t, snapshot.LLM)
	assert.NotNil(t, snapshot.System)
	assert.NotNil(t, snapshot.Database)
	assert.NotNil(t, snapshot.Cache)
	assert.False(t, snapshot.Timestamp.IsZero())
	assert.Greater(t, snapshot.Uptime, time.Duration(0))

	// 验证各个指标的基本数据
	assert.Equal(t, int64(1), snapshot.Query.TotalQueries)
	assert.Equal(t, int64(1), snapshot.LLM.TotalCalls)
	assert.Equal(t, int64(5), snapshot.Database.ActiveConnections)
	assert.Len(t, snapshot.Cache.Stats, 1)
}

func TestMetricsManager_Reset(t *testing.T) {
	manager := NewMetricsManager()

	// 记录一些指标
	manager.RecordQuerySuccess()
	manager.RecordQueryError("test_error")
	manager.RecordLLMCall("openai", "gpt-4", 100, 50, 2*time.Second, 0.05)
	manager.RecordDBQuery(50*time.Millisecond, true)
	manager.RecordCacheHit("redis")

	// 验证指标已记录
	snapshot1 := manager.GetMetrics()
	assert.Equal(t, int64(2), snapshot1.Query.TotalQueries)
	assert.Equal(t, int64(1), snapshot1.LLM.TotalCalls)

	// 重置指标
	manager.Reset()

	// 验证指标已重置
	snapshot2 := manager.GetMetrics()
	assert.Equal(t, int64(0), snapshot2.Query.TotalQueries)
	assert.Equal(t, int64(0), snapshot2.LLM.TotalCalls)
	assert.Equal(t, int64(0), snapshot2.Database.TotalQueries)
	assert.Len(t, snapshot2.Cache.Stats, 0)
}

func TestMetricsManager_StartPeriodicCollection(t *testing.T) {
	manager := NewMetricsManager()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// 启动定期收集
	go manager.StartPeriodicCollection(ctx, 50*time.Millisecond)

	// 等待一段时间让定期收集运行
	time.Sleep(150 * time.Millisecond)

	// 验证系统指标已更新
	metrics := manager.GetSystemMetrics()
	assert.False(t, metrics.Timestamp.IsZero())
	assert.Greater(t, metrics.GoroutineCount, 0)
}

func TestGlobalMetrics(t *testing.T) {
	// 初始化全局指标
	InitGlobalMetrics()

	// 获取全局指标收集器
	collector := GetGlobalMetrics()
	assert.NotNil(t, collector)

	// 记录一些指标
	collector.RecordQuerySuccess()
	collector.RecordQueryDuration(100 * time.Millisecond)
	collector.RecordLLMCall("openai", "gpt-4", 100, 50, 2*time.Second, 0.05)

	// 获取指标
	metrics := collector.GetMetrics()
	assert.Equal(t, int64(1), metrics.Query.TotalQueries)
	assert.Equal(t, int64(1), metrics.LLM.TotalCalls)
}

func TestNoopMetricsCollector(t *testing.T) {
	collector := &noopMetricsCollector{}

	// 所有方法都不应该panic
	collector.RecordQueryDuration(time.Second)
	collector.RecordQuerySuccess()
	collector.RecordQueryError("test")
	collector.RecordQueryResult(10)
	collector.RecordLLMCall("openai", "gpt-4", 100, 50, time.Second, 0.05)
	collector.RecordLLMError("openai", "gpt-4", "error")
	collector.RecordSystemMetrics()
	collector.RecordDBConnection(1, 2, 3)
	collector.RecordDBQuery(time.Second, true)
	collector.RecordCacheHit("redis")
	collector.RecordCacheMiss("redis")
	collector.RecordCacheOperation("redis", "get", time.Second)

	// 获取指标应该返回空值
	metrics := collector.GetMetrics()
	assert.NotNil(t, metrics)

	queryMetrics := collector.GetQueryMetrics()
	assert.NotNil(t, queryMetrics)
	assert.Equal(t, int64(0), queryMetrics.TotalQueries)

	llmMetrics := collector.GetLLMMetrics()
	assert.NotNil(t, llmMetrics)
	assert.Equal(t, int64(0), llmMetrics.TotalCalls)

	systemMetrics := collector.GetSystemMetrics()
	assert.NotNil(t, systemMetrics)

	dbMetrics := collector.GetDBMetrics()
	assert.NotNil(t, dbMetrics)
	assert.Equal(t, int64(0), dbMetrics.TotalQueries)

	cacheMetrics := collector.GetCacheMetrics()
	assert.NotNil(t, cacheMetrics)

	// Reset不应该panic
	collector.Reset()
}

func TestMetricsManager_ConcurrentAccess(t *testing.T) {
	manager := NewMetricsManager()

	// 并发记录指标
	done := make(chan bool)
	numGoroutines := 10
	numOperations := 100

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer func() { done <- true }()
			for j := 0; j < numOperations; j++ {
				manager.RecordQuerySuccess()
				manager.RecordQueryDuration(time.Duration(j) * time.Millisecond)
				manager.RecordLLMCall("openai", "gpt-4", j, j/2, time.Duration(j)*time.Millisecond, float64(j)*0.01)
				manager.RecordCacheHit("redis")
				manager.RecordDBQuery(time.Duration(j)*time.Millisecond, true)
			}
		}()
	}

	// 等待所有goroutine完成
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// 验证指标
	metrics := manager.GetMetrics()
	assert.Equal(t, int64(numGoroutines*numOperations), metrics.Query.TotalQueries)
	assert.Equal(t, int64(numGoroutines*numOperations), metrics.LLM.TotalCalls)
	assert.Equal(t, int64(numGoroutines*numOperations), metrics.Database.TotalQueries)
}

// 基准测试
func BenchmarkMetricsManager_RecordQuerySuccess(b *testing.B) {
	manager := NewMetricsManager()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			manager.RecordQuerySuccess()
		}
	})
}

func BenchmarkMetricsManager_RecordLLMCall(b *testing.B) {
	manager := NewMetricsManager()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			manager.RecordLLMCall("openai", "gpt-4", 100, 50, time.Second, 0.05)
		}
	})
}

func BenchmarkMetricsManager_GetMetrics(b *testing.B) {
	manager := NewMetricsManager()

	// 预先记录一些指标
	for i := 0; i < 1000; i++ {
		manager.RecordQuerySuccess()
		manager.RecordLLMCall("openai", "gpt-4", 100, 50, time.Second, 0.05)
		manager.RecordCacheHit("redis")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.GetMetrics()
	}
}

func TestLLMProviderStats_Calculations(t *testing.T) {
	manager := NewMetricsManager()

	// 记录多次调用同一个提供商
	manager.RecordLLMCall("openai", "gpt-4", 100, 50, 1*time.Second, 0.05)
	manager.RecordLLMCall("openai", "gpt-4", 200, 100, 2*time.Second, 0.10)
	manager.RecordLLMCall("openai", "gpt-4", 150, 75, 1500*time.Millisecond, 0.075)

	metrics := manager.GetLLMMetrics()
	stats := metrics.ProviderStats["openai:gpt-4"]
	require.NotNil(t, stats)

	assert.Equal(t, int64(3), stats.CallCount)
	assert.Equal(t, int64(450), stats.PromptTokens)
	assert.Equal(t, int64(225), stats.CompletionTokens)
	assert.Equal(t, 4500*time.Millisecond, stats.TotalDuration)
	assert.InDelta(t, 0.225, stats.TotalCost, 0.001)
	assert.Equal(t, 1500*time.Millisecond, stats.AvgDuration)
	assert.InDelta(t, 0.075, stats.AvgCostPerCall, 0.001)
}

func TestCacheStats_HitRateCalculations(t *testing.T) {
	manager := NewMetricsManager()

	// 记录缓存操作
	manager.RecordCacheHit("test")
	manager.RecordCacheHit("test")
	manager.RecordCacheHit("test")
	manager.RecordCacheMiss("test")
	manager.RecordCacheMiss("test")

	metrics := manager.GetCacheMetrics()
	stats := metrics.Stats["test"]
	require.NotNil(t, stats)

	assert.Equal(t, int64(3), stats.HitCount)
	assert.Equal(t, int64(2), stats.MissCount)
	assert.Equal(t, int64(5), stats.TotalRequests)
	assert.InDelta(t, 0.6, stats.HitRate, 0.01)
	assert.InDelta(t, 0.4, stats.MissRate, 0.01)
}
