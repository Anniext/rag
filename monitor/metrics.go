// 本文件实现了性能监控指标收集系统，用于监控查询响应时间、成功率、系统资源使用情况和LLM调用统计。
// 提供实时指标收集、统计分析和成本分析功能。
// 主要功能：
// 1. 查询性能指标收集（响应时间、成功率、错误率）
// 2. 系统资源监控（CPU、内存、磁盘、网络）
// 3. LLM调用统计和成本分析
// 4. 数据库连接池监控
// 5. 缓存命中率统计
// 6. 实时指标聚合和报告

package monitor

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// MetricsCollector 指标收集器接口
type MetricsCollector interface {
	// 查询指标
	RecordQueryDuration(duration time.Duration)
	RecordQuerySuccess()
	RecordQueryError(errorType string)
	RecordQueryResult(rowCount int)

	// LLM指标
	RecordLLMCall(provider, model string, promptTokens, completionTokens int, duration time.Duration, cost float64)
	RecordLLMError(provider, model string, errorType string)

	// 系统资源指标
	RecordSystemMetrics()

	// 数据库指标
	RecordDBConnection(active, idle, total int)
	RecordDBQuery(duration time.Duration, success bool)

	// 缓存指标
	RecordCacheHit(cacheType string)
	RecordCacheMiss(cacheType string)
	RecordCacheOperation(cacheType, operation string, duration time.Duration)

	// 获取指标
	GetMetrics() *MetricsSnapshot
	GetQueryMetrics() *QueryMetrics
	GetLLMMetrics() *LLMMetrics
	GetSystemMetrics() *SystemMetrics
	GetDBMetrics() *DatabaseMetrics
	GetCacheMetrics() *CacheMetrics

	// 重置指标
	Reset()
}

// MetricsManager 指标管理器
type MetricsManager struct {
	// 查询指标
	queryCount         int64
	querySuccessCount  int64
	queryErrorCount    int64
	queryDurationSum   int64 // 纳秒
	queryDurationCount int64
	queryRowCountSum   int64
	queryErrors        map[string]int64

	// LLM指标
	llmCallCount        int64
	llmErrorCount       int64
	llmPromptTokens     int64
	llmCompletionTokens int64
	llmDurationSum      int64 // 纳秒
	llmCostSum          int64 // 分（避免浮点数精度问题）
	llmProviderStats    map[string]*LLMProviderStats

	// 系统指标
	systemMetrics *SystemMetrics

	// 数据库指标
	dbActiveConns       int64
	dbIdleConns         int64
	dbTotalConns        int64
	dbQueryCount        int64
	dbQuerySuccessCount int64
	dbQueryDurationSum  int64

	// 缓存指标
	cacheStats map[string]*CacheStats

	// 时间窗口统计
	timeWindows map[string]*TimeWindowStats

	mutex     sync.RWMutex
	startTime time.Time
}

// QueryMetrics 查询指标
type QueryMetrics struct {
	TotalQueries     int64            `json:"total_queries"`
	SuccessQueries   int64            `json:"success_queries"`
	ErrorQueries     int64            `json:"error_queries"`
	SuccessRate      float64          `json:"success_rate"`
	ErrorRate        float64          `json:"error_rate"`
	AvgDuration      time.Duration    `json:"avg_duration"`
	AvgRowCount      float64          `json:"avg_row_count"`
	ErrorsByType     map[string]int64 `json:"errors_by_type"`
	QueriesPerSecond float64          `json:"queries_per_second"`
}

// LLMMetrics LLM指标
type LLMMetrics struct {
	TotalCalls            int64                        `json:"total_calls"`
	TotalErrors           int64                        `json:"total_errors"`
	SuccessRate           float64                      `json:"success_rate"`
	TotalPromptTokens     int64                        `json:"total_prompt_tokens"`
	TotalCompletionTokens int64                        `json:"total_completion_tokens"`
	TotalTokens           int64                        `json:"total_tokens"`
	AvgDuration           time.Duration                `json:"avg_duration"`
	TotalCost             float64                      `json:"total_cost"`
	AvgCostPerCall        float64                      `json:"avg_cost_per_call"`
	ProviderStats         map[string]*LLMProviderStats `json:"provider_stats"`
	CallsPerSecond        float64                      `json:"calls_per_second"`
}

// LLMProviderStats LLM提供商统计
type LLMProviderStats struct {
	Provider         string        `json:"provider"`
	Model            string        `json:"model"`
	CallCount        int64         `json:"call_count"`
	ErrorCount       int64         `json:"error_count"`
	PromptTokens     int64         `json:"prompt_tokens"`
	CompletionTokens int64         `json:"completion_tokens"`
	TotalDuration    time.Duration `json:"total_duration"`
	TotalCost        float64       `json:"total_cost"`
	AvgDuration      time.Duration `json:"avg_duration"`
	AvgCostPerCall   float64       `json:"avg_cost_per_call"`
}

// SystemMetrics 系统指标
type SystemMetrics struct {
	CPUUsage           float64       `json:"cpu_usage"`
	MemoryUsage        int64         `json:"memory_usage"`
	MemoryTotal        int64         `json:"memory_total"`
	MemoryUsagePercent float64       `json:"memory_usage_percent"`
	GoroutineCount     int           `json:"goroutine_count"`
	GCCount            uint32        `json:"gc_count"`
	GCPauseTotal       time.Duration `json:"gc_pause_total"`
	HeapSize           int64         `json:"heap_size"`
	HeapInUse          int64         `json:"heap_in_use"`
	StackInUse         int64         `json:"stack_in_use"`
	Timestamp          time.Time     `json:"timestamp"`
}

// DatabaseMetrics 数据库指标
type DatabaseMetrics struct {
	ActiveConnections int64         `json:"active_connections"`
	IdleConnections   int64         `json:"idle_connections"`
	TotalConnections  int64         `json:"total_connections"`
	TotalQueries      int64         `json:"total_queries"`
	SuccessQueries    int64         `json:"success_queries"`
	SuccessRate       float64       `json:"success_rate"`
	AvgQueryDuration  time.Duration `json:"avg_query_duration"`
	QueriesPerSecond  float64       `json:"queries_per_second"`
}

// CacheMetrics 缓存指标
type CacheMetrics struct {
	Stats map[string]*CacheStats `json:"stats"`
}

// CacheStats 缓存统计
type CacheStats struct {
	CacheType      string                     `json:"cache_type"`
	HitCount       int64                      `json:"hit_count"`
	MissCount      int64                      `json:"miss_count"`
	TotalRequests  int64                      `json:"total_requests"`
	HitRate        float64                    `json:"hit_rate"`
	MissRate       float64                    `json:"miss_rate"`
	OperationStats map[string]*OperationStats `json:"operation_stats"`
}

// OperationStats 操作统计
type OperationStats struct {
	Operation     string        `json:"operation"`
	Count         int64         `json:"count"`
	TotalDuration time.Duration `json:"total_duration"`
	AvgDuration   time.Duration `json:"avg_duration"`
}

// TimeWindowStats 时间窗口统计
type TimeWindowStats struct {
	WindowSize     time.Duration `json:"window_size"`
	QueryCount     int64         `json:"query_count"`
	ErrorCount     int64         `json:"error_count"`
	TotalDuration  time.Duration `json:"total_duration"`
	StartTime      time.Time     `json:"start_time"`
	LastUpdateTime time.Time     `json:"last_update_time"`
}

// MetricsSnapshot 指标快照
type MetricsSnapshot struct {
	Query     *QueryMetrics    `json:"query"`
	LLM       *LLMMetrics      `json:"llm"`
	System    *SystemMetrics   `json:"system"`
	Database  *DatabaseMetrics `json:"database"`
	Cache     *CacheMetrics    `json:"cache"`
	Timestamp time.Time        `json:"timestamp"`
	Uptime    time.Duration    `json:"uptime"`
}

// NewMetricsManager 创建指标管理器
func NewMetricsManager() *MetricsManager {
	return &MetricsManager{
		queryErrors:      make(map[string]int64),
		llmProviderStats: make(map[string]*LLMProviderStats),
		cacheStats:       make(map[string]*CacheStats),
		timeWindows:      make(map[string]*TimeWindowStats),
		systemMetrics:    &SystemMetrics{},
		startTime:        time.Now(),
	}
}

// RecordQueryDuration 记录查询持续时间
func (m *MetricsManager) RecordQueryDuration(duration time.Duration) {
	atomic.AddInt64(&m.queryDurationSum, int64(duration))
	atomic.AddInt64(&m.queryDurationCount, 1)
}

// RecordQuerySuccess 记录查询成功
func (m *MetricsManager) RecordQuerySuccess() {
	atomic.AddInt64(&m.queryCount, 1)
	atomic.AddInt64(&m.querySuccessCount, 1)
}

// RecordQueryError 记录查询错误
func (m *MetricsManager) RecordQueryError(errorType string) {
	atomic.AddInt64(&m.queryCount, 1)
	atomic.AddInt64(&m.queryErrorCount, 1)

	m.mutex.Lock()
	m.queryErrors[errorType]++
	m.mutex.Unlock()
}

// RecordQueryResult 记录查询结果行数
func (m *MetricsManager) RecordQueryResult(rowCount int) {
	atomic.AddInt64(&m.queryRowCountSum, int64(rowCount))
}

// RecordLLMCall 记录LLM调用
func (m *MetricsManager) RecordLLMCall(provider, model string, promptTokens, completionTokens int, duration time.Duration, cost float64) {
	atomic.AddInt64(&m.llmCallCount, 1)
	atomic.AddInt64(&m.llmPromptTokens, int64(promptTokens))
	atomic.AddInt64(&m.llmCompletionTokens, int64(completionTokens))
	atomic.AddInt64(&m.llmDurationSum, int64(duration))
	atomic.AddInt64(&m.llmCostSum, int64(cost*100)) // 转换为分

	m.mutex.Lock()
	defer m.mutex.Unlock()

	key := fmt.Sprintf("%s:%s", provider, model)
	if stats, exists := m.llmProviderStats[key]; exists {
		stats.CallCount++
		stats.PromptTokens += int64(promptTokens)
		stats.CompletionTokens += int64(completionTokens)
		stats.TotalDuration += duration
		stats.TotalCost += cost
		stats.AvgDuration = stats.TotalDuration / time.Duration(stats.CallCount)
		stats.AvgCostPerCall = stats.TotalCost / float64(stats.CallCount)
	} else {
		m.llmProviderStats[key] = &LLMProviderStats{
			Provider:         provider,
			Model:            model,
			CallCount:        1,
			PromptTokens:     int64(promptTokens),
			CompletionTokens: int64(completionTokens),
			TotalDuration:    duration,
			TotalCost:        cost,
			AvgDuration:      duration,
			AvgCostPerCall:   cost,
		}
	}
}

// RecordLLMError 记录LLM错误
func (m *MetricsManager) RecordLLMError(provider, model string, errorType string) {
	atomic.AddInt64(&m.llmErrorCount, 1)

	m.mutex.Lock()
	defer m.mutex.Unlock()

	key := fmt.Sprintf("%s:%s", provider, model)
	if stats, exists := m.llmProviderStats[key]; exists {
		stats.ErrorCount++
	} else {
		m.llmProviderStats[key] = &LLMProviderStats{
			Provider:   provider,
			Model:      model,
			ErrorCount: 1,
		}
	}
}

// RecordSystemMetrics 记录系统指标
func (m *MetricsManager) RecordSystemMetrics() {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.systemMetrics = &SystemMetrics{
		MemoryUsage:        int64(memStats.Alloc),
		MemoryTotal:        int64(memStats.Sys),
		MemoryUsagePercent: float64(memStats.Alloc) / float64(memStats.Sys) * 100,
		GoroutineCount:     runtime.NumGoroutine(),
		GCCount:            memStats.NumGC,
		GCPauseTotal:       time.Duration(memStats.PauseTotalNs),
		HeapSize:           int64(memStats.HeapSys),
		HeapInUse:          int64(memStats.HeapInuse),
		StackInUse:         int64(memStats.StackInuse),
		Timestamp:          time.Now(),
	}
}

// RecordDBConnection 记录数据库连接
func (m *MetricsManager) RecordDBConnection(active, idle, total int) {
	atomic.StoreInt64(&m.dbActiveConns, int64(active))
	atomic.StoreInt64(&m.dbIdleConns, int64(idle))
	atomic.StoreInt64(&m.dbTotalConns, int64(total))
}

// RecordDBQuery 记录数据库查询
func (m *MetricsManager) RecordDBQuery(duration time.Duration, success bool) {
	atomic.AddInt64(&m.dbQueryCount, 1)
	atomic.AddInt64(&m.dbQueryDurationSum, int64(duration))

	if success {
		atomic.AddInt64(&m.dbQuerySuccessCount, 1)
	}
}

// RecordCacheHit 记录缓存命中
func (m *MetricsManager) RecordCacheHit(cacheType string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if stats, exists := m.cacheStats[cacheType]; exists {
		stats.HitCount++
		stats.TotalRequests++
		stats.HitRate = float64(stats.HitCount) / float64(stats.TotalRequests)
		stats.MissRate = float64(stats.MissCount) / float64(stats.TotalRequests)
	} else {
		m.cacheStats[cacheType] = &CacheStats{
			CacheType:      cacheType,
			HitCount:       1,
			TotalRequests:  1,
			HitRate:        1.0,
			MissRate:       0.0,
			OperationStats: make(map[string]*OperationStats),
		}
	}
}

// RecordCacheMiss 记录缓存未命中
func (m *MetricsManager) RecordCacheMiss(cacheType string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if stats, exists := m.cacheStats[cacheType]; exists {
		stats.MissCount++
		stats.TotalRequests++
		stats.HitRate = float64(stats.HitCount) / float64(stats.TotalRequests)
		stats.MissRate = float64(stats.MissCount) / float64(stats.TotalRequests)
	} else {
		m.cacheStats[cacheType] = &CacheStats{
			CacheType:      cacheType,
			MissCount:      1,
			TotalRequests:  1,
			HitRate:        0.0,
			MissRate:       1.0,
			OperationStats: make(map[string]*OperationStats),
		}
	}
}

// RecordCacheOperation 记录缓存操作
func (m *MetricsManager) RecordCacheOperation(cacheType, operation string, duration time.Duration) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if stats, exists := m.cacheStats[cacheType]; exists {
		if opStats, opExists := stats.OperationStats[operation]; opExists {
			opStats.Count++
			opStats.TotalDuration += duration
			opStats.AvgDuration = opStats.TotalDuration / time.Duration(opStats.Count)
		} else {
			stats.OperationStats[operation] = &OperationStats{
				Operation:     operation,
				Count:         1,
				TotalDuration: duration,
				AvgDuration:   duration,
			}
		}
	} else {
		m.cacheStats[cacheType] = &CacheStats{
			CacheType: cacheType,
			OperationStats: map[string]*OperationStats{
				operation: {
					Operation:     operation,
					Count:         1,
					TotalDuration: duration,
					AvgDuration:   duration,
				},
			},
		}
	}
}

// GetMetrics 获取所有指标快照
func (m *MetricsManager) GetMetrics() *MetricsSnapshot {
	return &MetricsSnapshot{
		Query:     m.GetQueryMetrics(),
		LLM:       m.GetLLMMetrics(),
		System:    m.GetSystemMetrics(),
		Database:  m.GetDBMetrics(),
		Cache:     m.GetCacheMetrics(),
		Timestamp: time.Now(),
		Uptime:    time.Since(m.startTime),
	}
}

// GetQueryMetrics 获取查询指标
func (m *MetricsManager) GetQueryMetrics() *QueryMetrics {
	totalQueries := atomic.LoadInt64(&m.queryCount)
	successQueries := atomic.LoadInt64(&m.querySuccessCount)
	errorQueries := atomic.LoadInt64(&m.queryErrorCount)
	durationSum := atomic.LoadInt64(&m.queryDurationSum)
	durationCount := atomic.LoadInt64(&m.queryDurationCount)
	rowCountSum := atomic.LoadInt64(&m.queryRowCountSum)

	var successRate, errorRate float64
	var avgDuration time.Duration
	var avgRowCount float64
	var queriesPerSecond float64

	if totalQueries > 0 {
		successRate = float64(successQueries) / float64(totalQueries)
		errorRate = float64(errorQueries) / float64(totalQueries)
		avgRowCount = float64(rowCountSum) / float64(totalQueries)

		uptime := time.Since(m.startTime)
		if uptime > 0 {
			queriesPerSecond = float64(totalQueries) / uptime.Seconds()
		}
	}

	if durationCount > 0 {
		avgDuration = time.Duration(durationSum / durationCount)
	}

	m.mutex.RLock()
	errorsByType := make(map[string]int64)
	for k, v := range m.queryErrors {
		errorsByType[k] = v
	}
	m.mutex.RUnlock()

	return &QueryMetrics{
		TotalQueries:     totalQueries,
		SuccessQueries:   successQueries,
		ErrorQueries:     errorQueries,
		SuccessRate:      successRate,
		ErrorRate:        errorRate,
		AvgDuration:      avgDuration,
		AvgRowCount:      avgRowCount,
		ErrorsByType:     errorsByType,
		QueriesPerSecond: queriesPerSecond,
	}
}

// GetLLMMetrics 获取LLM指标
func (m *MetricsManager) GetLLMMetrics() *LLMMetrics {
	totalCalls := atomic.LoadInt64(&m.llmCallCount)
	totalErrors := atomic.LoadInt64(&m.llmErrorCount)
	promptTokens := atomic.LoadInt64(&m.llmPromptTokens)
	completionTokens := atomic.LoadInt64(&m.llmCompletionTokens)
	durationSum := atomic.LoadInt64(&m.llmDurationSum)
	costSum := atomic.LoadInt64(&m.llmCostSum)

	var successRate float64
	var avgDuration time.Duration
	var totalCost, avgCostPerCall float64
	var callsPerSecond float64

	if totalCalls > 0 {
		successRate = float64(totalCalls-totalErrors) / float64(totalCalls)
		avgDuration = time.Duration(durationSum / totalCalls)
		totalCost = float64(costSum) / 100 // 转换回元
		avgCostPerCall = totalCost / float64(totalCalls)

		uptime := time.Since(m.startTime)
		if uptime > 0 {
			callsPerSecond = float64(totalCalls) / uptime.Seconds()
		}
	}

	m.mutex.RLock()
	providerStats := make(map[string]*LLMProviderStats)
	for k, v := range m.llmProviderStats {
		providerStats[k] = &LLMProviderStats{
			Provider:         v.Provider,
			Model:            v.Model,
			CallCount:        v.CallCount,
			ErrorCount:       v.ErrorCount,
			PromptTokens:     v.PromptTokens,
			CompletionTokens: v.CompletionTokens,
			TotalDuration:    v.TotalDuration,
			TotalCost:        v.TotalCost,
			AvgDuration:      v.AvgDuration,
			AvgCostPerCall:   v.AvgCostPerCall,
		}
	}
	m.mutex.RUnlock()

	return &LLMMetrics{
		TotalCalls:            totalCalls,
		TotalErrors:           totalErrors,
		SuccessRate:           successRate,
		TotalPromptTokens:     promptTokens,
		TotalCompletionTokens: completionTokens,
		TotalTokens:           promptTokens + completionTokens,
		AvgDuration:           avgDuration,
		TotalCost:             totalCost,
		AvgCostPerCall:        avgCostPerCall,
		ProviderStats:         providerStats,
		CallsPerSecond:        callsPerSecond,
	}
}

// GetSystemMetrics 获取系统指标
func (m *MetricsManager) GetSystemMetrics() *SystemMetrics {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// 返回副本避免并发问题
	return &SystemMetrics{
		CPUUsage:           m.systemMetrics.CPUUsage,
		MemoryUsage:        m.systemMetrics.MemoryUsage,
		MemoryTotal:        m.systemMetrics.MemoryTotal,
		MemoryUsagePercent: m.systemMetrics.MemoryUsagePercent,
		GoroutineCount:     m.systemMetrics.GoroutineCount,
		GCCount:            m.systemMetrics.GCCount,
		GCPauseTotal:       m.systemMetrics.GCPauseTotal,
		HeapSize:           m.systemMetrics.HeapSize,
		HeapInUse:          m.systemMetrics.HeapInUse,
		StackInUse:         m.systemMetrics.StackInUse,
		Timestamp:          m.systemMetrics.Timestamp,
	}
}

// GetDBMetrics 获取数据库指标
func (m *MetricsManager) GetDBMetrics() *DatabaseMetrics {
	activeConns := atomic.LoadInt64(&m.dbActiveConns)
	idleConns := atomic.LoadInt64(&m.dbIdleConns)
	totalConns := atomic.LoadInt64(&m.dbTotalConns)
	totalQueries := atomic.LoadInt64(&m.dbQueryCount)
	successQueries := atomic.LoadInt64(&m.dbQuerySuccessCount)
	durationSum := atomic.LoadInt64(&m.dbQueryDurationSum)

	var successRate float64
	var avgDuration time.Duration
	var queriesPerSecond float64

	if totalQueries > 0 {
		successRate = float64(successQueries) / float64(totalQueries)
		avgDuration = time.Duration(durationSum / totalQueries)

		uptime := time.Since(m.startTime)
		if uptime > 0 {
			queriesPerSecond = float64(totalQueries) / uptime.Seconds()
		}
	}

	return &DatabaseMetrics{
		ActiveConnections: activeConns,
		IdleConnections:   idleConns,
		TotalConnections:  totalConns,
		TotalQueries:      totalQueries,
		SuccessQueries:    successQueries,
		SuccessRate:       successRate,
		AvgQueryDuration:  avgDuration,
		QueriesPerSecond:  queriesPerSecond,
	}
}

// GetCacheMetrics 获取缓存指标
func (m *MetricsManager) GetCacheMetrics() *CacheMetrics {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	stats := make(map[string]*CacheStats)
	for k, v := range m.cacheStats {
		opStats := make(map[string]*OperationStats)
		for opK, opV := range v.OperationStats {
			opStats[opK] = &OperationStats{
				Operation:     opV.Operation,
				Count:         opV.Count,
				TotalDuration: opV.TotalDuration,
				AvgDuration:   opV.AvgDuration,
			}
		}

		stats[k] = &CacheStats{
			CacheType:      v.CacheType,
			HitCount:       v.HitCount,
			MissCount:      v.MissCount,
			TotalRequests:  v.TotalRequests,
			HitRate:        v.HitRate,
			MissRate:       v.MissRate,
			OperationStats: opStats,
		}
	}

	return &CacheMetrics{
		Stats: stats,
	}
}

// Reset 重置所有指标
func (m *MetricsManager) Reset() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 重置查询指标
	atomic.StoreInt64(&m.queryCount, 0)
	atomic.StoreInt64(&m.querySuccessCount, 0)
	atomic.StoreInt64(&m.queryErrorCount, 0)
	atomic.StoreInt64(&m.queryDurationSum, 0)
	atomic.StoreInt64(&m.queryDurationCount, 0)
	atomic.StoreInt64(&m.queryRowCountSum, 0)
	m.queryErrors = make(map[string]int64)

	// 重置LLM指标
	atomic.StoreInt64(&m.llmCallCount, 0)
	atomic.StoreInt64(&m.llmErrorCount, 0)
	atomic.StoreInt64(&m.llmPromptTokens, 0)
	atomic.StoreInt64(&m.llmCompletionTokens, 0)
	atomic.StoreInt64(&m.llmDurationSum, 0)
	atomic.StoreInt64(&m.llmCostSum, 0)
	m.llmProviderStats = make(map[string]*LLMProviderStats)

	// 重置数据库指标
	atomic.StoreInt64(&m.dbActiveConns, 0)
	atomic.StoreInt64(&m.dbIdleConns, 0)
	atomic.StoreInt64(&m.dbTotalConns, 0)
	atomic.StoreInt64(&m.dbQueryCount, 0)
	atomic.StoreInt64(&m.dbQuerySuccessCount, 0)
	atomic.StoreInt64(&m.dbQueryDurationSum, 0)

	// 重置缓存指标
	m.cacheStats = make(map[string]*CacheStats)

	// 重置时间窗口
	m.timeWindows = make(map[string]*TimeWindowStats)

	// 重置开始时间
	m.startTime = time.Now()
}

// StartPeriodicCollection 启动定期指标收集
func (m *MetricsManager) StartPeriodicCollection(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.RecordSystemMetrics()
		}
	}
}

// 全局指标管理器
var (
	globalMetricsManager *MetricsManager
	globalMetricsMutex   sync.RWMutex
)

// InitGlobalMetrics 初始化全局指标管理器
func InitGlobalMetrics() {
	globalMetricsMutex.Lock()
	defer globalMetricsMutex.Unlock()

	globalMetricsManager = NewMetricsManager()
}

// GetGlobalMetrics 获取全局指标管理器
func GetGlobalMetrics() MetricsCollector {
	globalMetricsMutex.RLock()
	defer globalMetricsMutex.RUnlock()

	if globalMetricsManager == nil {
		return &noopMetricsCollector{}
	}

	return globalMetricsManager
}

// noopMetricsCollector 空操作指标收集器
type noopMetricsCollector struct{}

func (n *noopMetricsCollector) RecordQueryDuration(duration time.Duration) {}
func (n *noopMetricsCollector) RecordQuerySuccess()                        {}
func (n *noopMetricsCollector) RecordQueryError(errorType string)          {}
func (n *noopMetricsCollector) RecordQueryResult(rowCount int)             {}
func (n *noopMetricsCollector) RecordLLMCall(provider, model string, promptTokens, completionTokens int, duration time.Duration, cost float64) {
}
func (n *noopMetricsCollector) RecordLLMError(provider, model string, errorType string) {}
func (n *noopMetricsCollector) RecordSystemMetrics()                                    {}
func (n *noopMetricsCollector) RecordDBConnection(active, idle, total int)              {}
func (n *noopMetricsCollector) RecordDBQuery(duration time.Duration, success bool)      {}
func (n *noopMetricsCollector) RecordCacheHit(cacheType string)                         {}
func (n *noopMetricsCollector) RecordCacheMiss(cacheType string)                        {}
func (n *noopMetricsCollector) RecordCacheOperation(cacheType, operation string, duration time.Duration) {
}
func (n *noopMetricsCollector) GetMetrics() *MetricsSnapshot     { return &MetricsSnapshot{} }
func (n *noopMetricsCollector) GetQueryMetrics() *QueryMetrics   { return &QueryMetrics{} }
func (n *noopMetricsCollector) GetLLMMetrics() *LLMMetrics       { return &LLMMetrics{} }
func (n *noopMetricsCollector) GetSystemMetrics() *SystemMetrics { return &SystemMetrics{} }
func (n *noopMetricsCollector) GetDBMetrics() *DatabaseMetrics   { return &DatabaseMetrics{} }
func (n *noopMetricsCollector) GetCacheMetrics() *CacheMetrics   { return &CacheMetrics{} }
func (n *noopMetricsCollector) Reset()                           {}
