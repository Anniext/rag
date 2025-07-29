package test

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/Anniext/rag/cache"
	"github.com/Anniext/rag/core"
	"github.com/Anniext/rag/schema"
	"github.com/Anniext/rag/session"

	"math/rand"
	"os"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// StressTestSuite 压力测试套件
type StressTestSuite struct {
	config         *StressTestConfig
	db             *sql.DB
	logger         core.Logger
	schemaManager  *schema.Manager
	cacheManager   *cache.Manager
	sessionManager *session.Manager
	queryProcessor core.RAGQueryProcessor
	testData       *StressTestData
}

// StressTestConfig 压力测试配置
type StressTestConfig struct {
	DatabaseDSN       string
	MaxConcurrency    int
	TestDuration      time.Duration
	WarmupDuration    time.Duration
	RequestsPerSecond int
	MemoryLimitMB     int
	EnableProfiling   bool
}

// StressTestData 压力测试数据
type StressTestData struct {
	SimpleQueries    []string
	ComplexQueries   []string
	ErrorQueries     []string
	LargeDataQueries []string
	UserSessions     []string
}

// StressTestMetrics 压力测试指标
type StressTestMetrics struct {
	TestName           string
	StartTime          time.Time
	EndTime            time.Time
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
	MemoryMetrics      *StressMemoryMetrics
	ResourceMetrics    *StressResourceMetrics
}

// StressMemoryMetrics 压力测试内存指标
type StressMemoryMetrics struct {
	InitialMemory    uint64
	PeakMemory       uint64
	FinalMemory      uint64
	MemoryGrowth     uint64
	GCCount          uint32
	GCPauseTotal     time.Duration
	AllocatedObjects uint64
	HeapSize         uint64
}

// StressResourceMetrics 压力测试资源指标
type StressResourceMetrics struct {
	InitialGoroutines int
	PeakGoroutines    int
	FinalGoroutines   int
	GoroutineGrowth   int
	CPUUsagePercent   float64
	DatabaseConnCount int
	CacheHitRate      float64
}

// NewStressTestSuite 创建压力测试套件
func NewStressTestSuite() *StressTestSuite {
	config := &StressTestConfig{
		DatabaseDSN:       getEnvOrDefault("STRESS_DB_DSN", "root:123456@tcp(localhost:3306)/rag_test?charset=utf8mb4&parseTime=True&loc=Local"),
		MaxConcurrency:    getEnvIntOrDefault("STRESS_MAX_CONCURRENCY", 50),
		TestDuration:      getEnvDurationOrDefault("STRESS_TEST_DURATION", 2*time.Minute),
		WarmupDuration:    getEnvDurationOrDefault("STRESS_WARMUP_DURATION", 5*time.Second),
		RequestsPerSecond: getEnvIntOrDefault("STRESS_REQUESTS_PER_SECOND", 100),
		MemoryLimitMB:     getEnvIntOrDefault("STRESS_MEMORY_LIMIT_MB", 1000),
		EnableProfiling:   getEnvBoolOrDefault("STRESS_ENABLE_PROFILING", false),
	}

	return &StressTestSuite{
		config: config,
		testData: &StressTestData{
			SimpleQueries: []string{
				"查找所有用户",
				"统计用户数量",
				"查找最新订单",
				"获取用户信息",
				"查看产品列表",
				"获取订单状态",
				"查询用户余额",
				"统计今日订单",
			},
			ComplexQueries: []string{
				"查找年龄在25-35岁之间且订单金额超过1000元的用户，按注册时间排序",
				"统计每个月的销售额和订单数量，包括同比增长率",
				"分析用户购买行为，找出最受欢迎的产品组合",
				"计算每个用户的生命周期价值和流失概率",
				"生成销售漏斗分析报告，包含各阶段转化率",
				"分析产品销售趋势，预测未来三个月的销量",
				"计算用户留存率和活跃度指标",
				"生成多维度的业务分析报告",
			},
			ErrorQueries: []string{
				"", // 空查询
				"SELECT * FROM non_existent_table",
				"查找不存在的字段",
				"执行无效的SQL语句",
				"查询超长的字符串" + string(make([]byte, 10000)),
			},
			LargeDataQueries: []string{
				"生成包含所有用户详细信息、订单历史、产品偏好、行为分析的综合报告，数据范围覆盖最近三年",
				"执行复杂的多表关联查询，包含用户、订单、产品、分类、评价、物流等所有相关表",
				"进行大数据量的聚合分析，计算各维度的统计指标和趋势分析",
				"生成详细的用户画像分析，包含行为轨迹、偏好分析、价值评估等多个维度",
			},
			UserSessions: make([]string, 200),
		},
	}
}

// Setup 设置测试环境
func (suite *StressTestSuite) Setup() error {
	// 创建日志记录器
	zapLogger, err := zap.NewDevelopment()
	if err != nil {
		return err
	}
	suite.logger = NewZapLoggerAdapter(zapLogger)

	// 尝试连接数据库，如果失败则使用 Mock 模式
	suite.db, err = sql.Open("mysql", suite.config.DatabaseDSN)
	if err != nil {
		suite.logger.Warn("数据库连接失败，使用 Mock 模式", "error", err)
		suite.db = nil
	} else if err := suite.db.Ping(); err != nil {
		suite.logger.Warn("数据库连接失败，使用 Mock 模式", "error", err)
		suite.db.Close()
		suite.db = nil
	}

	// 如果有数据库连接，优化连接池
	if suite.db != nil {
		suite.db.SetMaxOpenConns(suite.config.MaxConcurrency * 2)
		suite.db.SetMaxIdleConns(suite.config.MaxConcurrency)
		suite.db.SetConnMaxLifetime(time.Hour)
	}

	// 初始化组件
	return suite.setupComponents()
}

// setupComponents 设置组件
func (suite *StressTestSuite) setupComponents() error {
	// 创建缓存管理器
	cacheConfig := cache.DefaultCacheConfig()
	cacheConfig.Type = cache.CacheTypeMemory
	cacheConfig.MaxCacheSize = "500MB"
	var err error
	suite.cacheManager, err = cache.NewManager(cacheConfig, suite.logger, &MockMetricsCollector{})
	if err != nil {
		return fmt.Errorf("创建缓存管理器失败: %w", err)
	}

	// 创建 Schema 管理器
	schemaLoader := &MockSchemaLoader{db: suite.db}
	config := &core.Config{
		Database: &core.DatabaseConfig{
			Database: "stress_test_db",
		},
	}
	suite.schemaManager = schema.NewManager(schemaLoader, suite.cacheManager, suite.logger, config, nil)

	// 创建会话管理器
	suite.sessionManager = session.NewManager(4*time.Hour, suite.cacheManager, suite.logger, &MockMetricsCollector{})

	// 创建查询处理器
	suite.queryProcessor = &MockQueryProcessor{
		logger: suite.logger,
	}

	// 初始化测试数据
	suite.initializeTestData()

	return nil
}

// initializeTestData 初始化测试数据
func (suite *StressTestSuite) initializeTestData() {
	// 生成用户会话ID
	for i := range suite.testData.UserSessions {
		suite.testData.UserSessions[i] = fmt.Sprintf("stress_user_%d", i)
	}
}

// Cleanup 清理测试环境
func (suite *StressTestSuite) Cleanup() {
	if suite.db != nil {
		suite.db.Close()
	}
}

// TestStressConcurrentQueries 测试并发查询压力
func TestStressConcurrentQueries(t *testing.T) {
	if os.Getenv("STRESS_TEST") != "true" {
		t.Skip("跳过压力测试，设置 STRESS_TEST=true 启用")
	}

	suite := NewStressTestSuite()
	err := suite.Setup()
	require.NoError(t, err)
	defer suite.Cleanup()

	// 加载 Schema
	ctx := context.Background()
	err = suite.schemaManager.LoadSchema(ctx)
	require.NoError(t, err)

	// 预热系统
	suite.warmupSystem(ctx)

	// 执行并发查询压力测试
	metrics := suite.runConcurrentQueriesStressTest(ctx)

	// 验证性能指标
	suite.validateConcurrentQueryMetrics(t, metrics)

	// 打印报告
	suite.printStressTestReport(metrics)
}

// TestStressMemoryUsage 测试内存使用压力
func TestStressMemoryUsage(t *testing.T) {
	if os.Getenv("STRESS_TEST") != "true" {
		t.Skip("跳过压力测试，设置 STRESS_TEST=true 启用")
	}

	suite := NewStressTestSuite()
	err := suite.Setup()
	require.NoError(t, err)
	defer suite.Cleanup()

	// 执行内存压力测试
	metrics := suite.runMemoryStressTest(context.Background())

	// 验证内存使用
	suite.validateMemoryStressMetrics(t, metrics)

	// 打印报告
	suite.printStressTestReport(metrics)
}

// TestStressLongRunningStability 测试长时间运行稳定性
func TestStressLongRunningStability(t *testing.T) {
	if os.Getenv("STRESS_TEST") != "true" {
		t.Skip("跳过压力测试，设置 STRESS_TEST=true 启用")
	}

	suite := NewStressTestSuite()
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
	suite.printStressTestReport(metrics)
}

// warmupSystem 预热系统
func (suite *StressTestSuite) warmupSystem(ctx context.Context) {
	suite.logger.Info("开始系统预热", "duration", suite.config.WarmupDuration)

	start := time.Now()
	warmupCtx, cancel := context.WithTimeout(ctx, suite.config.WarmupDuration)
	defer cancel()

	// 执行各种类型的查询进行预热
	for time.Since(start) < suite.config.WarmupDuration {
		select {
		case <-warmupCtx.Done():
			break
		default:
			for _, query := range suite.testData.SimpleQueries {
				request := &core.QueryRequest{
					Query:     query,
					RequestID: fmt.Sprintf("warmup_%d", time.Now().UnixNano()),
				}
				suite.queryProcessor.ProcessQuery(warmupCtx, request)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}

	suite.logger.Info("系统预热完成")
}

// runConcurrentQueriesStressTest 运行并发查询压力测试
func (suite *StressTestSuite) runConcurrentQueriesStressTest(ctx context.Context) *StressTestMetrics {
	suite.logger.Info("开始并发查询压力测试",
		"concurrency", suite.config.MaxConcurrency,
		"duration", suite.config.TestDuration)

	metrics := &StressTestMetrics{
		TestName:        "concurrent_queries_stress",
		StartTime:       time.Now(),
		MemoryMetrics:   &StressMemoryMetrics{},
		ResourceMetrics: &StressResourceMetrics{},
	}

	var totalRequests int64
	var successfulRequests int64
	var failedRequests int64
	var latencies []time.Duration
	var latencyMutex sync.Mutex

	// 记录初始状态
	var initialMem runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&initialMem)
	metrics.MemoryMetrics.InitialMemory = initialMem.Alloc
	metrics.ResourceMetrics.InitialGoroutines = runtime.NumGoroutine()

	// 创建测试上下文
	testCtx, cancel := context.WithTimeout(ctx, suite.config.TestDuration)
	defer cancel()

	// 创建工作协程
	var wg sync.WaitGroup
	requestChan := make(chan *core.QueryRequest, suite.config.MaxConcurrency*2)

	// 启动请求生成器
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(requestChan)
		suite.generateStressRequests(testCtx, requestChan)
	}()

	// 启动工作协程
	for i := range suite.config.MaxConcurrency {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for request := range requestChan {
				select {
				case <-testCtx.Done():
					return
				default:
					requestStart := time.Now()
					_, err := suite.queryProcessor.ProcessQuery(testCtx, request)
					requestLatency := time.Since(requestStart)

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

					// 定期检查资源使用
					if atomic.LoadInt64(&totalRequests)%1000 == 0 {
						suite.checkResourceUsage(metrics)
					}
				}
			}
		}(i)
	}

	wg.Wait()

	metrics.EndTime = time.Now()
	actualDuration := metrics.EndTime.Sub(metrics.StartTime)

	// 计算统计信息
	metrics.TotalRequests = totalRequests
	metrics.SuccessfulRequests = successfulRequests
	metrics.FailedRequests = failedRequests
	metrics.Latencies = latencies

	if totalRequests > 0 {
		metrics.ErrorRate = float64(failedRequests) / float64(totalRequests) * 100
		metrics.RequestsPerSecond = float64(totalRequests) / actualDuration.Seconds()
	}

	suite.calculateLatencyStatistics(metrics)
	suite.recordFinalMetrics(metrics)

	return metrics
}

// generateStressRequests 生成压力测试请求
func (suite *StressTestSuite) generateStressRequests(ctx context.Context, requestChan chan<- *core.QueryRequest) {
	requestID := int64(0)
	allQueries := append(suite.testData.SimpleQueries, suite.testData.ComplexQueries...)
	allQueries = append(allQueries, suite.testData.LargeDataQueries...)
	allQueries = append(allQueries, suite.testData.ErrorQueries...)

	ticker := time.NewTicker(time.Second / time.Duration(suite.config.RequestsPerSecond))
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			queryIndex := int(atomic.AddInt64(&requestID, 1) - 1)
			query := allQueries[queryIndex%len(allQueries)]
			userID := suite.testData.UserSessions[queryIndex%len(suite.testData.UserSessions)]

			request := &core.QueryRequest{
				Query:     fmt.Sprintf("%s [stress_%d]", query, queryIndex),
				RequestID: fmt.Sprintf("stress_%d", queryIndex),
				UserID:    userID,
				Context: map[string]any{
					"test_type":  "stress",
					"request_id": queryIndex,
					"timestamp":  time.Now().Unix(),
				},
			}

			select {
			case requestChan <- request:
			case <-ctx.Done():
				return
			}
		}
	}
}

// runMemoryStressTest 运行内存压力测试
func (suite *StressTestSuite) runMemoryStressTest(ctx context.Context) *StressTestMetrics {
	suite.logger.Info("开始内存压力测试")

	metrics := &StressTestMetrics{
		TestName:        "memory_stress",
		StartTime:       time.Now(),
		MemoryMetrics:   &StressMemoryMetrics{},
		ResourceMetrics: &StressResourceMetrics{},
	}

	// 记录初始内存
	var initialMem runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&initialMem)
	metrics.MemoryMetrics.InitialMemory = initialMem.Alloc

	// 创建大量会话和查询
	sessionCount := getEnvIntOrDefault("STRESS_MEMORY_SESSIONS", 1000)
	sessions := make([]string, sessionCount)
	var totalRequests int64
	var successfulRequests int64

	suite.logger.Info("创建大量会话进行内存压力测试", "session_count", len(sessions))

	for i := range sessions {
		userID := fmt.Sprintf("memory_stress_user_%d", i)
		session, err := suite.sessionManager.CreateSession(ctx, userID)
		if err != nil {
			continue
		}
		sessions[i] = session.SessionID

		// 每个会话执行多个查询
		queriesPerSession := getEnvIntOrDefault("STRESS_QUERIES_PER_SESSION", 10)
		for j := range queriesPerSession {
			query := suite.testData.ComplexQueries[j%len(suite.testData.ComplexQueries)]
			request := &core.QueryRequest{
				Query:     fmt.Sprintf("%s [memory_session_%d_query_%d]", query, i, j),
				SessionID: session.SessionID,
				RequestID: fmt.Sprintf("mem_stress_%d_%d", i, j),
				Context: map[string]any{
					"large_data": make([]byte, 2048), // 2KB 数据
					"metadata":   map[string]string{"session": session.SessionID},
					"test_data":  make([]int, 256), // 额外的测试数据
				},
			}

			_, err := suite.queryProcessor.ProcessQuery(ctx, request)
			totalRequests++
			if err == nil {
				successfulRequests++
			}
		}

		// 定期检查内存使用和触发 GC
		if i%100 == 0 {
			var currentMem runtime.MemStats
			runtime.ReadMemStats(&currentMem)
			if currentMem.Alloc > metrics.MemoryMetrics.PeakMemory {
				metrics.MemoryMetrics.PeakMemory = currentMem.Alloc
			}

			suite.logger.Info("内存压力测试进度",
				"sessions_created", i,
				"memory_mb", currentMem.Alloc/1024/1024,
				"gc_count", currentMem.NumGC)

			// 如果内存使用过高，触发 GC
			if currentMem.Alloc > uint64(suite.config.MemoryLimitMB)*1024*1024 {
				runtime.GC()
			}
		}
	}

	// 清理会话
	suite.logger.Info("清理测试会话")
	for _, sessionID := range sessions {
		if sessionID != "" {
			suite.sessionManager.DeleteSession(ctx, sessionID)
		}
	}

	// 强制 GC
	runtime.GC()
	runtime.GC()

	metrics.EndTime = time.Now()
	metrics.TotalRequests = totalRequests
	metrics.SuccessfulRequests = successfulRequests
	metrics.FailedRequests = totalRequests - successfulRequests

	if totalRequests > 0 {
		metrics.ErrorRate = float64(metrics.FailedRequests) / float64(totalRequests) * 100
	}

	suite.recordFinalMetrics(metrics)

	return metrics
}

// runLongRunningStabilityTest 运行长时间稳定性测试
func (suite *StressTestSuite) runLongRunningStabilityTest(ctx context.Context) *StressTestMetrics {
	suite.logger.Info("开始长时间稳定性测试", "duration", suite.config.TestDuration)

	metrics := &StressTestMetrics{
		TestName:        "long_running_stability",
		StartTime:       time.Now(),
		MemoryMetrics:   &StressMemoryMetrics{},
		ResourceMetrics: &StressResourceMetrics{},
	}

	var totalRequests int64
	var successfulRequests int64
	var failedRequests int64
	var latencies []time.Duration
	var latencyMutex sync.Mutex

	// 记录初始状态
	var initialMem runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&initialMem)
	metrics.MemoryMetrics.InitialMemory = initialMem.Alloc
	metrics.ResourceMetrics.InitialGoroutines = runtime.NumGoroutine()

	testCtx, cancel := context.WithTimeout(ctx, suite.config.TestDuration)
	defer cancel()

	// 启动多个长期运行的工作协程
	var wg sync.WaitGroup
	concurrency := suite.config.MaxConcurrency / 2 // 使用较低的并发度进行长期测试

	for i := range concurrency {
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
					time.Sleep(time.Duration(rand.Intn(100)) * time.Millisecond)

					// 定期检查系统状态
					if queryCount%100 == 0 {
						suite.checkResourceUsage(metrics)
					}
				}
			}
		}(i)
	}

	wg.Wait()

	metrics.EndTime = time.Now()
	actualDuration := metrics.EndTime.Sub(metrics.StartTime)

	// 计算统计信息
	metrics.TotalRequests = totalRequests
	metrics.SuccessfulRequests = successfulRequests
	metrics.FailedRequests = failedRequests
	metrics.Latencies = latencies

	if totalRequests > 0 {
		metrics.ErrorRate = float64(failedRequests) / float64(totalRequests) * 100
		metrics.RequestsPerSecond = float64(totalRequests) / actualDuration.Seconds()
	}

	suite.calculateLatencyStatistics(metrics)
	suite.recordFinalMetrics(metrics)

	return metrics
}

// checkResourceUsage 检查资源使用情况
func (suite *StressTestSuite) checkResourceUsage(metrics *StressTestMetrics) {
	var currentMem runtime.MemStats
	runtime.ReadMemStats(&currentMem)

	if currentMem.Alloc > metrics.MemoryMetrics.PeakMemory {
		metrics.MemoryMetrics.PeakMemory = currentMem.Alloc
	}

	currentGoroutines := runtime.NumGoroutine()
	if currentGoroutines > metrics.ResourceMetrics.PeakGoroutines {
		metrics.ResourceMetrics.PeakGoroutines = currentGoroutines
	}
}

// recordFinalMetrics 记录最终指标
func (suite *StressTestSuite) recordFinalMetrics(metrics *StressTestMetrics) {
	var finalMem runtime.MemStats
	runtime.ReadMemStats(&finalMem)

	metrics.MemoryMetrics.FinalMemory = finalMem.Alloc
	metrics.MemoryMetrics.GCCount = finalMem.NumGC
	metrics.MemoryMetrics.GCPauseTotal = time.Duration(finalMem.PauseTotalNs)
	metrics.MemoryMetrics.AllocatedObjects = finalMem.HeapObjects
	metrics.MemoryMetrics.HeapSize = finalMem.HeapSys

	if metrics.MemoryMetrics.FinalMemory > metrics.MemoryMetrics.InitialMemory {
		metrics.MemoryMetrics.MemoryGrowth = metrics.MemoryMetrics.FinalMemory - metrics.MemoryMetrics.InitialMemory
	}

	metrics.ResourceMetrics.FinalGoroutines = runtime.NumGoroutine()
	metrics.ResourceMetrics.GoroutineGrowth = metrics.ResourceMetrics.FinalGoroutines - metrics.ResourceMetrics.InitialGoroutines
}

// calculateLatencyStatistics 计算延迟统计信息
func (suite *StressTestSuite) calculateLatencyStatistics(metrics *StressTestMetrics) {
	if len(metrics.Latencies) == 0 {
		return
	}

	// 排序以计算百分位数
	slices.Sort(metrics.Latencies)

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

// printStressTestReport 打印压力测试报告
func (suite *StressTestSuite) printStressTestReport(metrics *StressTestMetrics) {
	duration := metrics.EndTime.Sub(metrics.StartTime)

	suite.logger.Info("压力测试报告",
		"test_name", metrics.TestName,
		"duration", duration,
		"total_requests", metrics.TotalRequests,
		"successful_requests", metrics.SuccessfulRequests,
		"failed_requests", metrics.FailedRequests,
		"error_rate_percent", fmt.Sprintf("%.2f", metrics.ErrorRate),
		"requests_per_second", fmt.Sprintf("%.2f", metrics.RequestsPerSecond),
	)

	if len(metrics.Latencies) > 0 {
		suite.logger.Info("延迟统计",
			"min_latency", metrics.MinLatency,
			"avg_latency", metrics.AvgLatency,
			"max_latency", metrics.MaxLatency,
			"p50_latency", metrics.P50Latency,
			"p95_latency", metrics.P95Latency,
			"p99_latency", metrics.P99Latency,
		)
	}

	// 打印内存指标
	if metrics.MemoryMetrics != nil {
		suite.logger.Info("内存使用情况",
			"initial_memory_mb", metrics.MemoryMetrics.InitialMemory/1024/1024,
			"peak_memory_mb", metrics.MemoryMetrics.PeakMemory/1024/1024,
			"final_memory_mb", metrics.MemoryMetrics.FinalMemory/1024/1024,
			"memory_growth_mb", metrics.MemoryMetrics.MemoryGrowth/1024/1024,
			"gc_count", metrics.MemoryMetrics.GCCount,
			"gc_pause_total", metrics.MemoryMetrics.GCPauseTotal,
			"heap_objects", metrics.MemoryMetrics.AllocatedObjects,
		)
	}

	// 打印资源指标
	if metrics.ResourceMetrics != nil {
		suite.logger.Info("资源使用情况",
			"initial_goroutines", metrics.ResourceMetrics.InitialGoroutines,
			"peak_goroutines", metrics.ResourceMetrics.PeakGoroutines,
			"final_goroutines", metrics.ResourceMetrics.FinalGoroutines,
			"goroutine_growth", metrics.ResourceMetrics.GoroutineGrowth,
		)
	}

	// 生成性能建议
	recommendations := suite.generateRecommendations(metrics)
	for _, rec := range recommendations {
		suite.logger.Info("性能建议", "recommendation", rec)
	}
}

// generateRecommendations 生成性能建议
func (suite *StressTestSuite) generateRecommendations(metrics *StressTestMetrics) []string {
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
	if metrics.RequestsPerSecond < 50 {
		recommendations = append(recommendations, "QPS较低，建议优化系统性能或增加并发处理能力")
	}

	// 内存建议
	if metrics.MemoryMetrics != nil && metrics.MemoryMetrics.MemoryGrowth > 200*1024*1024 {
		recommendations = append(recommendations, "内存增长过多，建议检查内存泄漏")
	}

	// 协程建议
	if metrics.ResourceMetrics != nil && metrics.ResourceMetrics.GoroutineGrowth > 100 {
		recommendations = append(recommendations, "协程增长过多，建议检查协程泄漏")
	}

	// 延迟分布建议
	if len(metrics.Latencies) > 0 && metrics.MaxLatency > metrics.AvgLatency*10 {
		recommendations = append(recommendations, "存在异常慢查询，建议检查查询优化和索引")
	}

	return recommendations
}

// 验证方法

// validateConcurrentQueryMetrics 验证并发查询指标
func (suite *StressTestSuite) validateConcurrentQueryMetrics(t *testing.T, metrics *StressTestMetrics) {
	require.True(t, metrics.ErrorRate < 15.0, "并发查询错误率过高: %.2f%%", metrics.ErrorRate)

	// 根据配置的并发数和测试时间计算最小请求数
	minRequests := int64(suite.config.MaxConcurrency * 2) // 每个并发至少2个请求
	require.True(t, metrics.TotalRequests >= minRequests, "并发查询请求数过少: %d, 期望至少: %d", metrics.TotalRequests, minRequests)

	if len(metrics.Latencies) > 0 {
		require.True(t, metrics.AvgLatency < 2*time.Second, "并发查询平均延迟过高: %v", metrics.AvgLatency)
		require.True(t, metrics.P95Latency < 5*time.Second, "并发查询P95延迟过高: %v", metrics.P95Latency)
	}

	require.True(t, metrics.RequestsPerSecond > 1, "并发查询QPS过低: %.2f", metrics.RequestsPerSecond)
}

// validateMemoryStressMetrics 验证内存压力指标
func (suite *StressTestSuite) validateMemoryStressMetrics(t *testing.T, metrics *StressTestMetrics) {
	require.True(t, metrics.ErrorRate < 25.0, "内存压力测试错误率过高: %.2f%%", metrics.ErrorRate)

	// 根据配置的会话数量和每会话查询数量计算最小请求数
	sessionCount := getEnvIntOrDefault("STRESS_MEMORY_SESSIONS", 1000)
	queriesPerSession := getEnvIntOrDefault("STRESS_QUERIES_PER_SESSION", 10)
	minRequests := int64(sessionCount * queriesPerSession / 2) // 允许50%的失败率
	require.True(t, metrics.TotalRequests >= minRequests, "内存压力测试请求数过少: %d, 期望至少: %d", metrics.TotalRequests, minRequests)

	if metrics.MemoryMetrics != nil {
		peakMemoryMB := metrics.MemoryMetrics.PeakMemory / 1024 / 1024
		require.True(t, peakMemoryMB < 2000, "内存使用过高: %d MB", peakMemoryMB)
	}
}

// validateStabilityMetrics 验证稳定性指标
func (suite *StressTestSuite) validateStabilityMetrics(t *testing.T, metrics *StressTestMetrics) {
	require.True(t, metrics.ErrorRate < 10.0, "稳定性测试错误率过高: %.2f%%", metrics.ErrorRate)
	expectedMinRequests := int64(suite.config.MaxConcurrency * 10) // 每个工作协程至少10个请求
	require.True(t, metrics.TotalRequests >= expectedMinRequests, "稳定性测试请求数过少: %d", metrics.TotalRequests)

	if metrics.MemoryMetrics != nil {
		memoryGrowthMB := metrics.MemoryMetrics.MemoryGrowth / 1024 / 1024
		require.True(t, memoryGrowthMB < 500, "长时间运行内存增长过多: %d MB", memoryGrowthMB)
	}

	if metrics.ResourceMetrics != nil {
		require.True(t, metrics.ResourceMetrics.GoroutineGrowth < 200, "协程增长过多: %d", metrics.ResourceMetrics.GoroutineGrowth)
	}
}
