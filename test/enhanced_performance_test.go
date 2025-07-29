package test

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"os"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"pumppill/rag/cache"
	"pumppill/rag/core"
	"pumppill/rag/schema"
	"pumppill/rag/session"

	_ "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// EnhancedPerformanceTestSuite 增强性能测试套件
type EnhancedPerformanceTestSuite struct {
	config         *EnhancedTestConfig
	db             *sql.DB
	logger         core.Logger
	schemaManager  *schema.Manager
	cacheManager   *cache.Manager
	sessionManager *session.Manager
	queryProcessor core.RAGQueryProcessor
	testData       *EnhancedTestDataSet
}

// EnhancedTestConfig 增强测试配置
type EnhancedTestConfig struct {
	DatabaseDSN       string
	MaxConcurrency    int
	TestDuration      time.Duration
	WarmupDuration    time.Duration
	RequestsPerSecond int
	MemoryLimitMB     int
	EnableProfiling   bool
}

// EnhancedTestDataSet 增强测试数据集
type EnhancedTestDataSet struct {
	SimpleQueries  []string
	ComplexQueries []string
	ErrorQueries   []string
	UserSessions   []string
}

// EnhancedPerformanceMetrics 增强性能指标
type EnhancedPerformanceMetrics struct {
	TestName           string
	TotalRequests      int64
	SuccessfulRequests int64
	FailedRequests     int64
	ErrorRate          float64
	Latencies          []time.Duration
	MinLatency         time.Duration
	MaxLatency         time.Duration
	AvgLatency         time.Duration
	P50Latency         time.Duration
	P95Latency         time.Duration
	P99Latency         time.Duration
	RequestsPerSecond  float64
	TestDuration       time.Duration
	MemoryMetrics      *EnhancedMemoryMetrics
}

// EnhancedMemoryMetrics 增强内存指标
type EnhancedMemoryMetrics struct {
	InitialMemory    uint64
	PeakMemory       uint64
	FinalMemory      uint64
	MemoryGrowth     uint64
	GCCount          uint32
	GCPauseTotal     time.Duration
	AllocatedObjects uint64
	HeapSize         uint64
}

// NewEnhancedPerformanceTestSuite 创建增强性能测试套件
func NewEnhancedPerformanceTestSuite() *EnhancedPerformanceTestSuite {
	config := &EnhancedTestConfig{
		DatabaseDSN:       getEnvOrDefault("PERF_DB_DSN", "root:123456@tcp(localhost:3306)/rag_test?charset=utf8mb4&parseTime=True&loc=Local"),
		MaxConcurrency:    getEnvIntOrDefault("PERF_MAX_CONCURRENCY", 20),
		TestDuration:      getEnvDurationOrDefault("PERF_TEST_DURATION", 1*time.Minute),
		WarmupDuration:    getEnvDurationOrDefault("PERF_WARMUP_DURATION", 10*time.Second),
		RequestsPerSecond: getEnvIntOrDefault("PERF_REQUESTS_PER_SECOND", 50),
		MemoryLimitMB:     getEnvIntOrDefault("PERF_MEMORY_LIMIT_MB", 500),
		EnableProfiling:   getEnvBoolOrDefault("PERF_ENABLE_PROFILING", false),
	}

	return &EnhancedPerformanceTestSuite{
		config: config,
		testData: &EnhancedTestDataSet{
			SimpleQueries: []string{
				"查找所有用户",
				"统计用户数量",
				"查找最新订单",
				"获取用户信息",
				"查看产品列表",
			},
			ComplexQueries: []string{
				"查找年龄在25-35岁之间且订单金额超过1000元的用户，按注册时间排序",
				"统计每个月的销售额和订单数量，包括同比增长率",
				"分析用户购买行为，找出最受欢迎的产品组合",
				"计算每个用户的生命周期价值和流失概率",
			},
			ErrorQueries: []string{
				"", // 空查询
				"SELECT * FROM non_existent_table",
				"查找不存在的字段",
			},
			UserSessions: make([]string, 50),
		},
	}
}

// Setup 设置测试环境
func (suite *EnhancedPerformanceTestSuite) Setup() error {
	// 创建日志记录器
	zapLogger, err := zap.NewDevelopment()
	if err != nil {
		return err
	}
	suite.logger = NewZapLoggerAdapter(zapLogger)

	// 连接数据库
	suite.db, err = sql.Open("mysql", suite.config.DatabaseDSN)
	if err != nil {
		return err
	}

	// 尝试连接数据库，如果失败则使用 Mock 模式
	if err := suite.db.Ping(); err != nil {
		suite.logger.Warn("数据库连接失败，使用 Mock 模式", "error", err)
		suite.db.Close()
		suite.db = nil
	}

	// 优化数据库连接池
	suite.db.SetMaxOpenConns(suite.config.MaxConcurrency * 2)
	suite.db.SetMaxIdleConns(suite.config.MaxConcurrency)
	suite.db.SetConnMaxLifetime(time.Hour)

	// 初始化组件
	return suite.setupComponents()
}

// setupComponents 设置组件
func (suite *EnhancedPerformanceTestSuite) setupComponents() error {
	// 创建缓存管理器
	cacheConfig := cache.DefaultCacheConfig()
	cacheConfig.Type = cache.CacheTypeMemory
	cacheConfig.MaxCacheSize = "200MB"
	var err error
	suite.cacheManager, err = cache.NewManager(cacheConfig, suite.logger, &MockMetricsCollector{})
	if err != nil {
		return fmt.Errorf("创建缓存管理器失败: %w", err)
	}

	// 创建 Schema 管理器
	schemaLoader := &MockSchemaLoader{db: suite.db}
	config := &core.Config{
		Database: &core.DatabaseConfig{
			Database: "enhanced_test_db",
		},
	}
	suite.schemaManager = schema.NewManager(schemaLoader, suite.cacheManager, suite.logger, config, nil)

	// 创建会话管理器
	suite.sessionManager = session.NewManager(2*time.Hour, suite.cacheManager, suite.logger, &MockMetricsCollector{})

	// 创建查询处理器
	suite.queryProcessor = &MockQueryProcessor{
		logger: suite.logger,
	}

	// 初始化测试数据
	suite.initializeTestData()

	return nil
}

// initializeTestData 初始化测试数据
func (suite *EnhancedPerformanceTestSuite) initializeTestData() {
	// 生成用户会话ID
	for i := 0; i < len(suite.testData.UserSessions); i++ {
		suite.testData.UserSessions[i] = fmt.Sprintf("enhanced_user_%d", i)
	}
}

// Cleanup 清理测试环境
func (suite *EnhancedPerformanceTestSuite) Cleanup() {
	if suite.db != nil {
		suite.db.Close()
	}
}

// TestEnhancedConcurrentQueries 测试增强并发查询性能
func TestEnhancedConcurrentQueries(t *testing.T) {
	if os.Getenv("ENHANCED_PERFORMANCE_TEST") != "true" {
		t.Skip("跳过增强性能测试，设置 ENHANCED_PERFORMANCE_TEST=true 启用")
	}

	suite := NewEnhancedPerformanceTestSuite()
	err := suite.Setup()
	require.NoError(t, err)
	defer suite.Cleanup()

	// 加载 Schema
	ctx := context.Background()
	err = suite.schemaManager.LoadSchema(ctx)
	require.NoError(t, err)

	// 预热系统
	suite.warmupSystem(ctx)

	// 执行并发查询测试
	metrics := suite.runConcurrentQueriesTest(ctx)

	// 验证性能指标
	suite.validateConcurrentQueryMetrics(t, metrics)

	// 打印报告
	suite.printPerformanceReport(metrics)
}

// TestEnhancedMemoryStress 测试增强内存压力
func TestEnhancedMemoryStress(t *testing.T) {
	if os.Getenv("ENHANCED_PERFORMANCE_TEST") != "true" {
		t.Skip("跳过增强性能测试，设置 ENHANCED_PERFORMANCE_TEST=true 启用")
	}

	suite := NewEnhancedPerformanceTestSuite()
	err := suite.Setup()
	require.NoError(t, err)
	defer suite.Cleanup()

	// 执行内存压力测试
	metrics := suite.runMemoryStressTest(context.Background())

	// 验证内存使用
	suite.validateMemoryStressMetrics(t, metrics)

	// 打印报告
	suite.printPerformanceReport(metrics)
}

// TestEnhancedLongRunningStability 测试增强长时间运行稳定性
func TestEnhancedLongRunningStability(t *testing.T) {
	if os.Getenv("ENHANCED_PERFORMANCE_TEST") != "true" {
		t.Skip("跳过增强性能测试，设置 ENHANCED_PERFORMANCE_TEST=true 启用")
	}

	suite := NewEnhancedPerformanceTestSuite()
	err := suite.Setup()
	require.NoError(t, err)
	defer suite.Cleanup()

	// 加载 Schema
	ctx := context.Background()
	err = suite.schemaManager.LoadSchema(ctx)
	require.NoError(t, err)

	// 执行长时间稳定性测试
	metrics := suite.runLongRunningStabilityTest(ctx)

	// 验证稳定性指标
	suite.validateStabilityMetrics(t, metrics)

	// 打印报告
	suite.printPerformanceReport(metrics)
}

// warmupSystem 预热系统
func (suite *EnhancedPerformanceTestSuite) warmupSystem(ctx context.Context) {
	suite.logger.Info("开始系统预热", "duration", suite.config.WarmupDuration)

	start := time.Now()
	for time.Since(start) < suite.config.WarmupDuration {
		// 执行各种类型的查询进行预热
		for _, query := range suite.testData.SimpleQueries {
			request := &core.QueryRequest{
				Query:     query,
				RequestID: fmt.Sprintf("warmup_%d", time.Now().UnixNano()),
			}
			suite.queryProcessor.ProcessQuery(ctx, request)
		}

		time.Sleep(10 * time.Millisecond)
	}

	suite.logger.Info("系统预热完成")
}

// runConcurrentQueriesTest 运行并发查询测试
func (suite *EnhancedPerformanceTestSuite) runConcurrentQueriesTest(ctx context.Context) *EnhancedPerformanceMetrics {
	suite.logger.Info("开始并发查询测试",
		"concurrency", suite.config.MaxConcurrency,
		"duration", suite.config.TestDuration)

	metrics := &EnhancedPerformanceMetrics{
		TestName:      "concurrent_queries",
		MemoryMetrics: &EnhancedMemoryMetrics{},
	}

	var totalRequests int64
	var successfulRequests int64
	var failedRequests int64
	var latencies []time.Duration
	var latencyMutex sync.Mutex

	// 记录初始内存
	var initialMem runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&initialMem)
	metrics.MemoryMetrics.InitialMemory = initialMem.Alloc

	start := time.Now()
	testCtx, cancel := context.WithTimeout(ctx, suite.config.TestDuration)
	defer cancel()

	// 创建工作池
	semaphore := make(chan struct{}, suite.config.MaxConcurrency)
	var wg sync.WaitGroup

	// 启动请求生成器
	requestChan := make(chan *core.QueryRequest, suite.config.MaxConcurrency*2)
	go suite.generateTestRequests(testCtx, requestChan)

	// 启动工作协程
	for i := 0; i < suite.config.MaxConcurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for {
				select {
				case <-testCtx.Done():
					return
				case request, ok := <-requestChan:
					if !ok {
						return
					}

					semaphore <- struct{}{} // 获取信号量

					requestStart := time.Now()
					_, err := suite.queryProcessor.ProcessQuery(testCtx, request)
					requestLatency := time.Since(requestStart)

					<-semaphore // 释放信号量

					// 更新统计
					atomic.AddInt64(&totalRequests, 1)
					if err != nil {
						atomic.AddInt64(&failedRequests, 1)
					} else {
						atomic.AddInt64(&successfulRequests, 1)
					}

					latencyMutex.Lock()
					latencies = append(latencies, requestLatency)
					latencyMutex.Unlock()
				}
			}
		}(i)
	}

	wg.Wait()
	close(requestChan)

	actualDuration := time.Since(start)

	// 计算统计信息
	metrics.TotalRequests = totalRequests
	metrics.SuccessfulRequests = successfulRequests
	metrics.FailedRequests = failedRequests
	metrics.TestDuration = actualDuration
	metrics.Latencies = latencies

	if totalRequests > 0 {
		metrics.ErrorRate = float64(failedRequests) / float64(totalRequests) * 100
		metrics.RequestsPerSecond = float64(totalRequests) / actualDuration.Seconds()
	}

	suite.calculateLatencyStatistics(metrics)

	// 记录最终内存
	var finalMem runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&finalMem)
	metrics.MemoryMetrics.FinalMemory = finalMem.Alloc
	metrics.MemoryMetrics.PeakMemory = finalMem.Sys
	metrics.MemoryMetrics.GCCount = finalMem.NumGC - initialMem.NumGC
	metrics.MemoryMetrics.GCPauseTotal = time.Duration(finalMem.PauseTotalNs - initialMem.PauseTotalNs)

	if metrics.MemoryMetrics.FinalMemory > metrics.MemoryMetrics.InitialMemory {
		metrics.MemoryMetrics.MemoryGrowth = metrics.MemoryMetrics.FinalMemory - metrics.MemoryMetrics.InitialMemory
	}

	return metrics
}

// generateTestRequests 生成测试请求
func (suite *EnhancedPerformanceTestSuite) generateTestRequests(ctx context.Context, requestChan chan<- *core.QueryRequest) {
	defer close(requestChan)

	requestID := 0
	allQueries := append(suite.testData.SimpleQueries, suite.testData.ComplexQueries...)

	ticker := time.NewTicker(time.Second / time.Duration(suite.config.RequestsPerSecond))
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			query := allQueries[requestID%len(allQueries)]
			userID := suite.testData.UserSessions[requestID%len(suite.testData.UserSessions)]

			request := &core.QueryRequest{
				Query:     fmt.Sprintf("%s [%d]", query, requestID),
				RequestID: fmt.Sprintf("enhanced_%d", requestID),
				UserID:    userID,
			}

			select {
			case requestChan <- request:
				requestID++
			case <-ctx.Done():
				return
			}
		}
	}
}

// runMemoryStressTest 运行内存压力测试
func (suite *EnhancedPerformanceTestSuite) runMemoryStressTest(ctx context.Context) *EnhancedPerformanceMetrics {
	suite.logger.Info("开始内存压力测试")

	metrics := &EnhancedPerformanceMetrics{
		TestName:      "memory_stress",
		MemoryMetrics: &EnhancedMemoryMetrics{},
	}

	// 记录初始内存
	var initialMem runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&initialMem)
	metrics.MemoryMetrics.InitialMemory = initialMem.Alloc

	// 创建大量会话和查询
	sessions := make([]string, 2000)
	var totalRequests int64
	var successfulRequests int64

	for i := 0; i < 2000; i++ {
		userID := fmt.Sprintf("memory_stress_user_%d", i)
		session, err := suite.sessionManager.CreateSession(ctx, userID)
		if err != nil {
			continue
		}
		sessions[i] = session.SessionID

		// 每个会话执行多个查询
		for j := 0; j < 10; j++ {
			query := suite.testData.ComplexQueries[j%len(suite.testData.ComplexQueries)]
			request := &core.QueryRequest{
				Query:     fmt.Sprintf("%s [session_%d_query_%d]", query, i, j),
				SessionID: session.SessionID,
				RequestID: fmt.Sprintf("mem_stress_%d_%d", i, j),
				Context: map[string]any{
					"test_data": make([]byte, 512), // 512B 数据
				},
			}

			_, err := suite.queryProcessor.ProcessQuery(ctx, request)
			totalRequests++
			if err == nil {
				successfulRequests++
			}
		}

		// 定期检查内存使用
		if i%200 == 0 {
			var currentMem runtime.MemStats
			runtime.ReadMemStats(&currentMem)
			if currentMem.Alloc > metrics.MemoryMetrics.PeakMemory {
				metrics.MemoryMetrics.PeakMemory = currentMem.Alloc
			}

			// 如果内存使用过高，触发 GC
			if currentMem.Alloc > uint64(suite.config.MemoryLimitMB)*1024*1024 {
				runtime.GC()
			}
		}
	}

	// 清理会话
	for _, sessionID := range sessions {
		if sessionID != "" {
			suite.sessionManager.DeleteSession(ctx, sessionID)
		}
	}

	// 强制 GC
	runtime.GC()
	runtime.GC()

	// 记录最终内存
	var finalMem runtime.MemStats
	runtime.ReadMemStats(&finalMem)
	metrics.MemoryMetrics.FinalMemory = finalMem.Alloc
	metrics.MemoryMetrics.GCCount = finalMem.NumGC - initialMem.NumGC
	metrics.MemoryMetrics.GCPauseTotal = time.Duration(finalMem.PauseTotalNs - initialMem.PauseTotalNs)

	if metrics.MemoryMetrics.FinalMemory > metrics.MemoryMetrics.InitialMemory {
		metrics.MemoryMetrics.MemoryGrowth = metrics.MemoryMetrics.FinalMemory - metrics.MemoryMetrics.InitialMemory
	}

	metrics.TotalRequests = totalRequests
	metrics.SuccessfulRequests = successfulRequests
	metrics.FailedRequests = totalRequests - successfulRequests

	if totalRequests > 0 {
		metrics.ErrorRate = float64(metrics.FailedRequests) / float64(totalRequests) * 100
	}

	return metrics
}

// runLongRunningStabilityTest 运行长时间稳定性测试
func (suite *EnhancedPerformanceTestSuite) runLongRunningStabilityTest(ctx context.Context) *EnhancedPerformanceMetrics {
	suite.logger.Info("开始长时间稳定性测试", "duration", suite.config.TestDuration)

	metrics := &EnhancedPerformanceMetrics{
		TestName:      "long_running_stability",
		MemoryMetrics: &EnhancedMemoryMetrics{},
	}

	var totalRequests int64
	var successfulRequests int64
	var failedRequests int64
	var latencies []time.Duration
	var latencyMutex sync.Mutex

	// 记录初始内存
	var initialMem runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&initialMem)
	metrics.MemoryMetrics.InitialMemory = initialMem.Alloc

	start := time.Now()
	testCtx, cancel := context.WithTimeout(ctx, suite.config.TestDuration)
	defer cancel()

	// 启动多个长期运行的工作协程
	var wg sync.WaitGroup
	concurrency := suite.config.MaxConcurrency / 2 // 使用较低的并发度进行长期测试

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			queryCount := 0
			queries := append(suite.testData.SimpleQueries, suite.testData.ComplexQueries...)

			for {
				select {
				case <-testCtx.Done():
					return
				default:
					query := queries[queryCount%len(queries)]
					request := &core.QueryRequest{
						Query:     fmt.Sprintf("%s [stability_worker_%d_query_%d]", query, workerID, queryCount),
						RequestID: fmt.Sprintf("stability_%d_%d", workerID, queryCount),
						UserID:    fmt.Sprintf("stability_user_%d", workerID),
					}

					requestStart := time.Now()
					_, err := suite.queryProcessor.ProcessQuery(testCtx, request)
					requestLatency := time.Since(requestStart)

					atomic.AddInt64(&totalRequests, 1)
					if err != nil {
						atomic.AddInt64(&failedRequests, 1)
					} else {
						atomic.AddInt64(&successfulRequests, 1)
					}

					latencyMutex.Lock()
					latencies = append(latencies, requestLatency)
					latencyMutex.Unlock()

					queryCount++

					// 模拟真实负载的间隔
					time.Sleep(time.Duration(rand.Intn(20)) * time.Millisecond)

					// 定期检查系统状态
					if queryCount%50 == 0 {
						var currentMem runtime.MemStats
						runtime.ReadMemStats(&currentMem)
						if currentMem.Alloc > metrics.MemoryMetrics.PeakMemory {
							metrics.MemoryMetrics.PeakMemory = currentMem.Alloc
						}
					}
				}
			}
		}(i)
	}

	wg.Wait()
	actualDuration := time.Since(start)

	// 计算统计信息
	metrics.TotalRequests = totalRequests
	metrics.SuccessfulRequests = successfulRequests
	metrics.FailedRequests = failedRequests
	metrics.TestDuration = actualDuration
	metrics.Latencies = latencies

	if totalRequests > 0 {
		metrics.ErrorRate = float64(failedRequests) / float64(totalRequests) * 100
		metrics.RequestsPerSecond = float64(totalRequests) / actualDuration.Seconds()
	}

	suite.calculateLatencyStatistics(metrics)

	// 记录最终内存
	var finalMem runtime.MemStats
	runtime.ReadMemStats(&finalMem)
	metrics.MemoryMetrics.FinalMemory = finalMem.Alloc
	metrics.MemoryMetrics.GCCount = finalMem.NumGC - initialMem.NumGC
	metrics.MemoryMetrics.GCPauseTotal = time.Duration(finalMem.PauseTotalNs - initialMem.PauseTotalNs)

	if metrics.MemoryMetrics.FinalMemory > metrics.MemoryMetrics.InitialMemory {
		metrics.MemoryMetrics.MemoryGrowth = metrics.MemoryMetrics.FinalMemory - metrics.MemoryMetrics.InitialMemory
	}

	return metrics
}

// calculateLatencyStatistics 计算延迟统计信息
func (suite *EnhancedPerformanceTestSuite) calculateLatencyStatistics(metrics *EnhancedPerformanceMetrics) {
	if len(metrics.Latencies) == 0 {
		return
	}

	// 排序以计算百分位数
	sort.Slice(metrics.Latencies, func(i, j int) bool {
		return metrics.Latencies[i] < metrics.Latencies[j]
	})

	// 基本统计
	metrics.MinLatency = metrics.Latencies[0]
	metrics.MaxLatency = metrics.Latencies[len(metrics.Latencies)-1]

	var total time.Duration
	for _, d := range metrics.Latencies {
		total += d
	}
	metrics.AvgLatency = total / time.Duration(len(metrics.Latencies))

	// 百分位数
	metrics.P50Latency = metrics.Latencies[len(metrics.Latencies)*50/100]
	metrics.P95Latency = metrics.Latencies[len(metrics.Latencies)*95/100]
	metrics.P99Latency = metrics.Latencies[len(metrics.Latencies)*99/100]
}

// printPerformanceReport 打印性能报告
func (suite *EnhancedPerformanceTestSuite) printPerformanceReport(metrics *EnhancedPerformanceMetrics) {
	suite.logger.Info("增强性能测试报告",
		"test_name", metrics.TestName,
		"total_requests", metrics.TotalRequests,
		"successful_requests", metrics.SuccessfulRequests,
		"failed_requests", metrics.FailedRequests,
		"error_rate_percent", fmt.Sprintf("%.2f", metrics.ErrorRate),
		"requests_per_second", fmt.Sprintf("%.2f", metrics.RequestsPerSecond),
		"test_duration", metrics.TestDuration,
		"min_latency", metrics.MinLatency,
		"avg_latency", metrics.AvgLatency,
		"max_latency", metrics.MaxLatency,
		"p50_latency", metrics.P50Latency,
		"p95_latency", metrics.P95Latency,
		"p99_latency", metrics.P99Latency,
	)

	// 打印内存指标
	if metrics.MemoryMetrics != nil {
		suite.logger.Info("内存使用情况",
			"initial_memory_mb", metrics.MemoryMetrics.InitialMemory/1024/1024,
			"peak_memory_mb", metrics.MemoryMetrics.PeakMemory/1024/1024,
			"final_memory_mb", metrics.MemoryMetrics.FinalMemory/1024/1024,
			"memory_growth_mb", metrics.MemoryMetrics.MemoryGrowth/1024/1024,
			"gc_count", metrics.MemoryMetrics.GCCount,
			"gc_pause_total", metrics.MemoryMetrics.GCPauseTotal,
		)
	}

	// 生成性能建议
	recommendations := suite.generateRecommendations(metrics)
	for _, rec := range recommendations {
		suite.logger.Info("性能建议", "recommendation", rec)
	}
}

// generateRecommendations 生成性能建议
func (suite *EnhancedPerformanceTestSuite) generateRecommendations(metrics *EnhancedPerformanceMetrics) []string {
	var recommendations []string

	// 错误率建议
	if metrics.ErrorRate > 5.0 {
		recommendations = append(recommendations, "错误率过高，建议检查查询逻辑和数据库连接")
	}

	// 延迟建议
	if metrics.P95Latency > 500*time.Millisecond {
		recommendations = append(recommendations, "P95延迟过高，建议优化查询性能或增加缓存")
	}

	// QPS 建议
	if metrics.RequestsPerSecond < 20 {
		recommendations = append(recommendations, "QPS较低，建议优化系统性能或增加并发处理能力")
	}

	// 内存建议
	if metrics.MemoryMetrics != nil && metrics.MemoryMetrics.MemoryGrowth > 100*1024*1024 {
		recommendations = append(recommendations, "内存增长过多，建议检查内存泄漏")
	}

	return recommendations
}

// 验证方法

// validateConcurrentQueryMetrics 验证并发查询指标
func (suite *EnhancedPerformanceTestSuite) validateConcurrentQueryMetrics(t *testing.T, metrics *EnhancedPerformanceMetrics) {
	require.True(t, metrics.ErrorRate < 10.0, "并发查询错误率过高: %.2f%%", metrics.ErrorRate)
	require.True(t, metrics.AvgLatency < 1*time.Second, "并发查询平均延迟过高: %v", metrics.AvgLatency)
	require.True(t, metrics.P95Latency < 2*time.Second, "并发查询P95延迟过高: %v", metrics.P95Latency)
	require.True(t, metrics.RequestsPerSecond > 5, "并发查询QPS过低: %.2f", metrics.RequestsPerSecond)
}

// validateMemoryStressMetrics 验证内存压力指标
func (suite *EnhancedPerformanceTestSuite) validateMemoryStressMetrics(t *testing.T, metrics *EnhancedPerformanceMetrics) {
	require.True(t, metrics.ErrorRate < 20.0, "内存压力测试错误率过高: %.2f%%", metrics.ErrorRate)
	require.True(t, metrics.TotalRequests > 10000, "内存压力测试请求数过少: %d", metrics.TotalRequests)

	if metrics.MemoryMetrics != nil {
		peakMemoryMB := metrics.MemoryMetrics.PeakMemory / 1024 / 1024
		require.True(t, peakMemoryMB < 1000, "内存使用过高: %d MB", peakMemoryMB)
	}
}

// validateStabilityMetrics 验证稳定性指标
func (suite *EnhancedPerformanceTestSuite) validateStabilityMetrics(t *testing.T, metrics *EnhancedPerformanceMetrics) {
	require.True(t, metrics.ErrorRate < 5.0, "稳定性测试错误率过高: %.2f%%", metrics.ErrorRate)
	expectedMinRequests := int64(suite.config.MaxConcurrency * 2) // 每个工作协程至少2个请求
	require.True(t, metrics.TotalRequests >= expectedMinRequests, "稳定性测试请求数过少: %d", metrics.TotalRequests)

	if metrics.MemoryMetrics != nil {
		memoryGrowthMB := metrics.MemoryMetrics.MemoryGrowth / 1024 / 1024
		require.True(t, memoryGrowthMB < 200, "长时间运行内存增长过多: %d MB", memoryGrowthMB)
	}
}
