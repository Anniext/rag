package test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"runtime"
	"sort"
	"sync"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// PerformanceTestConfig 性能测试配置
type PerformanceTestConfig struct {
	DatabaseDSN      string
	ConcurrentUsers  int
	QueriesPerUser   int
	TestDuration     time.Duration
	WarmupDuration   time.Duration
	EnableProfiling  bool
	EnableMemoryTest bool
}

// PerformanceTestSuite 性能测试套件
type PerformanceTestSuite struct {
	config         *PerformanceTestConfig
	db             *sql.DB
	logger         core.Logger
	schemaManager  *schema.Manager
	cacheManager   *cache.Manager
	sessionManager *session.Manager
	queryProcessor core.RAGQueryProcessor
}

// NewPerformanceTestSuite 创建性能测试套件
func NewPerformanceTestSuite() *PerformanceTestSuite {
	config := &PerformanceTestConfig{
		DatabaseDSN:      getEnvOrDefault("PERF_DB_DSN", "root:123456@tcp(localhost:3306)/rag_test?charset=utf8mb4&parseTime=True&loc=Local"),
		ConcurrentUsers:  getEnvIntOrDefault("PERF_CONCURRENT_USERS", 10),
		QueriesPerUser:   getEnvIntOrDefault("PERF_QUERIES_PER_USER", 100),
		TestDuration:     getEnvDurationOrDefault("PERF_TEST_DURATION", 60*time.Second),
		WarmupDuration:   getEnvDurationOrDefault("PERF_WARMUP_DURATION", 10*time.Second),
		EnableProfiling:  getEnvBoolOrDefault("PERF_ENABLE_PROFILING", false),
		EnableMemoryTest: getEnvBoolOrDefault("PERF_ENABLE_MEMORY_TEST", true),
	}

	return &PerformanceTestSuite{
		config: config,
	}
}

// Setup 设置测试环境
func (suite *PerformanceTestSuite) Setup() error {
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

	// 测试数据库连接
	if err := suite.db.Ping(); err != nil {
		return fmt.Errorf("数据库连接失败: %w", err)
	}

	// 设置连接池参数
	suite.db.SetMaxOpenConns(100)
	suite.db.SetMaxIdleConns(20)
	suite.db.SetConnMaxLifetime(time.Hour)

	// 创建组件
	return suite.setupComponents()
}

// setupComponents 设置组件
func (suite *PerformanceTestSuite) setupComponents() error {
	// 创建 Schema 加载器
	schemaLoader := &MockSchemaLoader{db: suite.db}

	// 创建缓存管理器
	cacheConfig := cache.DefaultCacheConfig()
	cacheConfig.Type = cache.CacheTypeMemory
	var err error
	suite.cacheManager, err = cache.NewManager(cacheConfig, suite.logger, &MockMetricsCollector{})
	if err != nil {
		return fmt.Errorf("创建缓存管理器失败: %w", err)
	}

	// 创建 Schema 管理器
	config := &core.Config{
		Database: &core.DatabaseConfig{
			Database: "test_db",
		},
	}
	suite.schemaManager = schema.NewManager(schemaLoader, suite.cacheManager, suite.logger, config, nil)

	// 创建会话管理器
	suite.sessionManager = session.NewManager(2*time.Hour, suite.cacheManager, suite.logger, &MockMetricsCollector{})

	// 创建查询处理器
	suite.queryProcessor = &MockQueryProcessor{
		logger: suite.logger,
	}

	return nil
}

// Cleanup 清理测试环境
func (suite *PerformanceTestSuite) Cleanup() {
	if suite.db != nil {
		suite.db.Close()
	}
}

// PerformanceMetrics 性能指标
type PerformanceMetrics struct {
	TotalRequests      int64
	SuccessfulRequests int64
	FailedRequests     int64
	TotalDuration      time.Duration
	MinDuration        time.Duration
	MaxDuration        time.Duration
	AvgDuration        time.Duration
	P50Duration        time.Duration
	P95Duration        time.Duration
	P99Duration        time.Duration
	RequestsPerSecond  float64
	MemoryUsage        MemoryMetrics
	ErrorRate          float64
}

// MemoryMetrics 内存指标
type MemoryMetrics struct {
	InitialMemory  uint64
	PeakMemory     uint64
	FinalMemory    uint64
	MemoryIncrease uint64
	GCCount        uint32
	GCPauseTotal   time.Duration
	AllocatedBytes uint64
	HeapObjects    uint64
}

// TestQueryPerformance 测试查询性能
func TestQueryPerformance(t *testing.T) {
	if os.Getenv("PERFORMANCE_TEST") != "true" {
		t.Skip("跳过性能测试，设置 PERFORMANCE_TEST=true 启用")
	}

	suite := NewPerformanceTestSuite()
	err := suite.Setup()
	require.NoError(t, err)
	defer suite.Cleanup()

	// 加载 Schema
	ctx := context.Background()
	err = suite.schemaManager.LoadSchema(ctx)
	require.NoError(t, err)

	// 预热
	suite.warmup(ctx)

	// 执行性能测试
	metrics := suite.runQueryPerformanceTest(ctx)

	// 输出结果
	suite.printMetrics("查询性能测试", metrics)

	// 验证性能指标
	suite.validatePerformanceMetrics(t, metrics)
}

// TestConcurrentQueryPerformance 测试并发查询性能
func TestConcurrentQueryPerformance(t *testing.T) {
	if os.Getenv("PERFORMANCE_TEST") != "true" {
		t.Skip("跳过性能测试，设置 PERFORMANCE_TEST=true 启用")
	}

	suite := NewPerformanceTestSuite()
	err := suite.Setup()
	require.NoError(t, err)
	defer suite.Cleanup()

	// 加载 Schema
	ctx := context.Background()
	err = suite.schemaManager.LoadSchema(ctx)
	require.NoError(t, err)

	// 预热
	suite.warmup(ctx)

	// 执行并发性能测试
	metrics := suite.runConcurrentQueryTest(ctx)

	// 输出结果
	suite.printMetrics("并发查询性能测试", metrics)

	// 验证性能指标
	suite.validateConcurrentPerformanceMetrics(t, metrics)
}

// TestMemoryUsage 测试内存使用情况
func TestMemoryUsage(t *testing.T) {
	if os.Getenv("PERFORMANCE_TEST") != "true" || !NewPerformanceTestSuite().config.EnableMemoryTest {
		t.Skip("跳过内存测试")
	}

	suite := NewPerformanceTestSuite()
	err := suite.Setup()
	require.NoError(t, err)
	defer suite.Cleanup()

	// 执行内存测试
	metrics := suite.runMemoryTest(context.Background())

	// 输出结果
	suite.printMemoryMetrics("内存使用测试", metrics.MemoryUsage)

	// 验证内存使用
	suite.validateMemoryUsage(t, metrics.MemoryUsage)
}

// TestLongRunningStability 测试长时间运行稳定性
func TestLongRunningStability(t *testing.T) {
	if os.Getenv("PERFORMANCE_TEST") != "true" {
		t.Skip("跳过稳定性测试，设置 PERFORMANCE_TEST=true 启用")
	}

	suite := NewPerformanceTestSuite()
	err := suite.Setup()
	require.NoError(t, err)
	defer suite.Cleanup()

	// 加载 Schema
	ctx := context.Background()
	err = suite.schemaManager.LoadSchema(ctx)
	require.NoError(t, err)

	// 执行长时间稳定性测试
	metrics := suite.runStabilityTest(ctx)

	// 输出结果
	suite.printMetrics("长时间稳定性测试", metrics)

	// 验证稳定性指标
	suite.validateStabilityMetrics(t, metrics)
}

// warmup 预热系统
func (suite *PerformanceTestSuite) warmup(ctx context.Context) {
	suite.logger.Info("开始预热", zap.Duration("duration", suite.config.WarmupDuration))

	start := time.Now()
	for time.Since(start) < suite.config.WarmupDuration {
		request := &core.QueryRequest{
			Query:     "SELECT COUNT(*) FROM users",
			RequestID: fmt.Sprintf("warmup_%d", time.Now().UnixNano()),
		}

		suite.queryProcessor.ProcessQuery(ctx, request)
		time.Sleep(10 * time.Millisecond)
	}

	suite.logger.Info("预热完成")
}

// runQueryPerformanceTest 运行查询性能测试
func (suite *PerformanceTestSuite) runQueryPerformanceTest(ctx context.Context) *PerformanceMetrics {
	suite.logger.Info("开始查询性能测试")

	var metrics PerformanceMetrics
	var durations []time.Duration
	var mu sync.Mutex

	// 记录初始内存
	var initialMem runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&initialMem)
	metrics.MemoryUsage.InitialMemory = initialMem.Alloc

	start := time.Now()

	// 执行查询
	for i := 0; i < suite.config.QueriesPerUser; i++ {
		queryStart := time.Now()

		request := &core.QueryRequest{
			Query:     fmt.Sprintf("查找用户 %d", i%100),
			RequestID: fmt.Sprintf("perf_%d", i),
		}

		_, err := suite.queryProcessor.ProcessQuery(ctx, request)

		queryDuration := time.Since(queryStart)

		mu.Lock()
		durations = append(durations, queryDuration)
		metrics.TotalRequests++
		if err != nil {
			metrics.FailedRequests++
		} else {
			metrics.SuccessfulRequests++
		}
		mu.Unlock()
	}

	metrics.TotalDuration = time.Since(start)

	// 计算统计信息
	suite.calculateStatistics(&metrics, durations)

	// 记录最终内存
	var finalMem runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&finalMem)
	metrics.MemoryUsage.FinalMemory = finalMem.Alloc
	metrics.MemoryUsage.PeakMemory = finalMem.Sys
	metrics.MemoryUsage.GCCount = finalMem.NumGC - initialMem.NumGC
	metrics.MemoryUsage.GCPauseTotal = time.Duration(finalMem.PauseTotalNs - initialMem.PauseTotalNs)

	return &metrics
}

// runConcurrentQueryTest 运行并发查询测试
func (suite *PerformanceTestSuite) runConcurrentQueryTest(ctx context.Context) *PerformanceMetrics {
	suite.logger.Info("开始并发查询测试",
		zap.Int("concurrent_users", suite.config.ConcurrentUsers),
		zap.Int("queries_per_user", suite.config.QueriesPerUser))

	var metrics PerformanceMetrics
	var durations []time.Duration
	var mu sync.Mutex
	var wg sync.WaitGroup

	// 记录初始内存
	var initialMem runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&initialMem)
	metrics.MemoryUsage.InitialMemory = initialMem.Alloc

	start := time.Now()

	// 启动并发用户
	for user := 0; user < suite.config.ConcurrentUsers; user++ {
		wg.Add(1)
		go func(userID int) {
			defer wg.Done()

			for i := 0; i < suite.config.QueriesPerUser; i++ {
				queryStart := time.Now()

				request := &core.QueryRequest{
					Query:     fmt.Sprintf("用户 %d 查询 %d", userID, i),
					RequestID: fmt.Sprintf("concurrent_%d_%d", userID, i),
					UserID:    fmt.Sprintf("user_%d", userID),
				}

				_, err := suite.queryProcessor.ProcessQuery(ctx, request)

				queryDuration := time.Since(queryStart)

				mu.Lock()
				durations = append(durations, queryDuration)
				metrics.TotalRequests++
				if err != nil {
					metrics.FailedRequests++
				} else {
					metrics.SuccessfulRequests++
				}
				mu.Unlock()
			}
		}(user)
	}

	wg.Wait()
	metrics.TotalDuration = time.Since(start)

	// 计算统计信息
	suite.calculateStatistics(&metrics, durations)

	// 记录最终内存
	var finalMem runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&finalMem)
	metrics.MemoryUsage.FinalMemory = finalMem.Alloc
	metrics.MemoryUsage.PeakMemory = finalMem.Sys
	metrics.MemoryUsage.GCCount = finalMem.NumGC - initialMem.NumGC
	metrics.MemoryUsage.GCPauseTotal = time.Duration(finalMem.PauseTotalNs - initialMem.PauseTotalNs)

	return &metrics
}

// runMemoryTest 运行内存测试
func (suite *PerformanceTestSuite) runMemoryTest(ctx context.Context) *PerformanceMetrics {
	suite.logger.Info("开始内存测试")

	var metrics PerformanceMetrics

	// 记录初始内存
	var initialMem runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&initialMem)
	metrics.MemoryUsage.InitialMemory = initialMem.Alloc

	// 创建大量会话和查询
	sessions := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		sessionMemory, _ := suite.sessionManager.CreateSession(ctx, fmt.Sprintf("user_%d", i))
		sessions[i] = sessionMemory.SessionID

		// 执行多个查询
		for j := 0; j < 10; j++ {
			request := &core.QueryRequest{
				Query:     fmt.Sprintf("内存测试查询 %d-%d", i, j),
				SessionID: sessionMemory.SessionID,
				RequestID: fmt.Sprintf("mem_test_%d_%d", i, j),
			}

			suite.queryProcessor.ProcessQuery(ctx, request)
		}
	}

	// 记录峰值内存
	var peakMem runtime.MemStats
	runtime.ReadMemStats(&peakMem)
	metrics.MemoryUsage.PeakMemory = peakMem.Alloc

	// 清理会话
	for _, sessionID := range sessions {
		suite.sessionManager.DeleteSession(ctx, sessionID)
	}

	// 强制 GC
	runtime.GC()
	runtime.GC()

	// 记录最终内存
	var finalMem runtime.MemStats
	runtime.ReadMemStats(&finalMem)
	metrics.MemoryUsage.FinalMemory = finalMem.Alloc
	metrics.MemoryUsage.GCCount = finalMem.NumGC - initialMem.NumGC
	metrics.MemoryUsage.GCPauseTotal = time.Duration(finalMem.PauseTotalNs - initialMem.PauseTotalNs)
	metrics.MemoryUsage.AllocatedBytes = finalMem.TotalAlloc - initialMem.TotalAlloc
	metrics.MemoryUsage.HeapObjects = finalMem.HeapObjects

	if metrics.MemoryUsage.FinalMemory > metrics.MemoryUsage.InitialMemory {
		metrics.MemoryUsage.MemoryIncrease = metrics.MemoryUsage.FinalMemory - metrics.MemoryUsage.InitialMemory
	}

	return &metrics
}

// runStabilityTest 运行稳定性测试
func (suite *PerformanceTestSuite) runStabilityTest(ctx context.Context) *PerformanceMetrics {
	suite.logger.Info("开始稳定性测试", zap.Duration("duration", suite.config.TestDuration))

	var metrics PerformanceMetrics
	var durations []time.Duration
	var mu sync.Mutex

	// 记录初始内存
	var initialMem runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&initialMem)
	metrics.MemoryUsage.InitialMemory = initialMem.Alloc

	start := time.Now()
	testCtx, cancel := context.WithTimeout(ctx, suite.config.TestDuration)
	defer cancel()

	// 启动多个并发 goroutine
	var wg sync.WaitGroup
	for i := 0; i < suite.config.ConcurrentUsers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			queryCount := 0
			for {
				select {
				case <-testCtx.Done():
					return
				default:
					queryStart := time.Now()

					request := &core.QueryRequest{
						Query:     fmt.Sprintf("稳定性测试 worker %d query %d", workerID, queryCount),
						RequestID: fmt.Sprintf("stability_%d_%d", workerID, queryCount),
						UserID:    fmt.Sprintf("stability_user_%d", workerID),
					}

					_, err := suite.queryProcessor.ProcessQuery(testCtx, request)

					queryDuration := time.Since(queryStart)

					mu.Lock()
					durations = append(durations, queryDuration)
					metrics.TotalRequests++
					if err != nil {
						metrics.FailedRequests++
					} else {
						metrics.SuccessfulRequests++
					}
					mu.Unlock()

					queryCount++

					// 短暂休息以避免过度负载
					time.Sleep(time.Millisecond)
				}
			}
		}(i)
	}

	wg.Wait()
	metrics.TotalDuration = time.Since(start)

	// 计算统计信息
	suite.calculateStatistics(&metrics, durations)

	// 记录最终内存
	var finalMem runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&finalMem)
	metrics.MemoryUsage.FinalMemory = finalMem.Alloc
	metrics.MemoryUsage.PeakMemory = finalMem.Sys
	metrics.MemoryUsage.GCCount = finalMem.NumGC - initialMem.NumGC
	metrics.MemoryUsage.GCPauseTotal = time.Duration(finalMem.PauseTotalNs - initialMem.PauseTotalNs)

	return &metrics
}

// calculateStatistics 计算统计信息
func (suite *PerformanceTestSuite) calculateStatistics(metrics *PerformanceMetrics, durations []time.Duration) {
	if len(durations) == 0 {
		return
	}

	// 排序以计算百分位数
	sort.Slice(durations, func(i, j int) bool {
		return durations[i] < durations[j]
	})

	// 基本统计
	metrics.MinDuration = durations[0]
	metrics.MaxDuration = durations[len(durations)-1]

	var total time.Duration
	for _, d := range durations {
		total += d
	}
	metrics.AvgDuration = total / time.Duration(len(durations))

	// 百分位数
	metrics.P50Duration = durations[len(durations)*50/100]
	metrics.P95Duration = durations[len(durations)*95/100]
	metrics.P99Duration = durations[len(durations)*99/100]

	// QPS
	if metrics.TotalDuration > 0 {
		metrics.RequestsPerSecond = float64(metrics.TotalRequests) / metrics.TotalDuration.Seconds()
	}

	// 错误率
	if metrics.TotalRequests > 0 {
		metrics.ErrorRate = float64(metrics.FailedRequests) / float64(metrics.TotalRequests) * 100
	}
}

// printMetrics 打印性能指标
func (suite *PerformanceTestSuite) printMetrics(testName string, metrics *PerformanceMetrics) {
	suite.logger.Info("性能测试结果",
		zap.String("test", testName),
		zap.Int64("total_requests", metrics.TotalRequests),
		zap.Int64("successful_requests", metrics.SuccessfulRequests),
		zap.Int64("failed_requests", metrics.FailedRequests),
		zap.Float64("error_rate_percent", metrics.ErrorRate),
		zap.Duration("total_duration", metrics.TotalDuration),
		zap.Duration("min_duration", metrics.MinDuration),
		zap.Duration("max_duration", metrics.MaxDuration),
		zap.Duration("avg_duration", metrics.AvgDuration),
		zap.Duration("p50_duration", metrics.P50Duration),
		zap.Duration("p95_duration", metrics.P95Duration),
		zap.Duration("p99_duration", metrics.P99Duration),
		zap.Float64("requests_per_second", metrics.RequestsPerSecond),
	)
}

// printMemoryMetrics 打印内存指标
func (suite *PerformanceTestSuite) printMemoryMetrics(testName string, memory MemoryMetrics) {
	suite.logger.Info("内存使用情况",
		zap.String("test", testName),
		zap.String("initial_memory", formatBytes(memory.InitialMemory)),
		zap.String("peak_memory", formatBytes(memory.PeakMemory)),
		zap.String("final_memory", formatBytes(memory.FinalMemory)),
		zap.String("memory_increase", formatBytes(memory.MemoryIncrease)),
		zap.Uint32("gc_count", memory.GCCount),
		zap.Duration("gc_pause_total", memory.GCPauseTotal),
		zap.String("allocated_bytes", formatBytes(memory.AllocatedBytes)),
		zap.Uint64("heap_objects", memory.HeapObjects),
	)
}

// validatePerformanceMetrics 验证性能指标
func (suite *PerformanceTestSuite) validatePerformanceMetrics(t *testing.T, metrics *PerformanceMetrics) {
	// 验证错误率
	require.True(t, metrics.ErrorRate < 5.0, "错误率过高: %.2f%%", metrics.ErrorRate)

	// 验证平均响应时间
	require.True(t, metrics.AvgDuration < 100*time.Millisecond, "平均响应时间过长: %v", metrics.AvgDuration)

	// 验证 P95 响应时间
	require.True(t, metrics.P95Duration < 200*time.Millisecond, "P95 响应时间过长: %v", metrics.P95Duration)

	// 验证 QPS
	require.True(t, metrics.RequestsPerSecond > 50, "QPS 过低: %.2f", metrics.RequestsPerSecond)
}

// validateConcurrentPerformanceMetrics 验证并发性能指标
func (suite *PerformanceTestSuite) validateConcurrentPerformanceMetrics(t *testing.T, metrics *PerformanceMetrics) {
	// 并发测试的标准相对宽松
	require.True(t, metrics.ErrorRate < 10.0, "并发测试错误率过高: %.2f%%", metrics.ErrorRate)
	require.True(t, metrics.AvgDuration < 500*time.Millisecond, "并发测试平均响应时间过长: %v", metrics.AvgDuration)
	require.True(t, metrics.P95Duration < 1*time.Second, "并发测试 P95 响应时间过长: %v", metrics.P95Duration)
	require.True(t, metrics.RequestsPerSecond > 20, "并发测试 QPS 过低: %.2f", metrics.RequestsPerSecond)
}

// validateMemoryUsage 验证内存使用
func (suite *PerformanceTestSuite) validateMemoryUsage(t *testing.T, memory MemoryMetrics) {
	// 验证内存增长不超过 100MB
	require.True(t, memory.MemoryIncrease < 100*1024*1024, "内存增长过多: %s", formatBytes(memory.MemoryIncrease))

	// 验证 GC 暂停时间不超过总时间的 10%
	if memory.GCPauseTotal > 0 {
		// 这里需要总测试时间来计算比例，简化处理
		require.True(t, memory.GCPauseTotal < 1*time.Second, "GC 暂停时间过长: %v", memory.GCPauseTotal)
	}
}

// validateStabilityMetrics 验证稳定性指标
func (suite *PerformanceTestSuite) validateStabilityMetrics(t *testing.T, metrics *PerformanceMetrics) {
	// 稳定性测试要求更严格的错误率
	require.True(t, metrics.ErrorRate < 1.0, "稳定性测试错误率过高: %.2f%%", metrics.ErrorRate)

	// 验证请求总数合理
	expectedMinRequests := int64(suite.config.ConcurrentUsers * 10) // 每个用户至少 10 个请求
	require.True(t, metrics.TotalRequests >= expectedMinRequests, "稳定性测试请求数过少: %d", metrics.TotalRequests)
}
