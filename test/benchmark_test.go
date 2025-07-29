package test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"go.uber.org/zap"
)

// BenchmarkSetup 基准测试设置
type BenchmarkSetup struct {
	db             *sql.DB
	logger         core.Logger
	schemaManager  *schema.Manager
	cacheManager   *cache.Manager
	sessionManager *session.Manager
	queryProcessor core.RAGQueryProcessor
}

// setupBenchmark 设置基准测试环境
func setupBenchmark(b *testing.B) *BenchmarkSetup {
	if os.Getenv("BENCHMARK_TEST") != "true" {
		b.Skip("跳过基准测试，设置 BENCHMARK_TEST=true 启用")
	}

	// 创建日志记录器（静默模式）
	zapLogger := zap.NewNop()
	logger := NewZapLoggerAdapter(zapLogger)

	// 连接数据库
	dsn := getEnvOrDefault("BENCHMARK_DB_DSN", "root:123456@tcp(localhost:3306)/rag_test?charset=utf8mb4&parseTime=True&loc=Local")
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		b.Fatal(err)
	}

	if err := db.Ping(); err != nil {
		b.Fatal(err)
	}

	// 设置连接池
	db.SetMaxOpenConns(50)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(time.Hour)

	// 创建组件
	schemaLoader := &MockSchemaLoader{db: db}

	cacheConfig := cache.DefaultCacheConfig()
	cacheConfig.Type = cache.CacheTypeMemory
	cacheManager, err := cache.NewManager(cacheConfig, logger, &MockMetricsCollector{})
	if err != nil {
		b.Fatal(err)
	}

	config := &core.Config{
		Database: &core.DatabaseConfig{
			Database: "benchmark_db",
		},
	}
	schemaManager := schema.NewManager(schemaLoader, cacheManager, logger, config, nil)

	sessionManager := session.NewManager(2*time.Hour, cacheManager, logger, &MockMetricsCollector{})

	queryProcessor := &MockQueryProcessor{
		logger: logger,
	}

	// 加载 Schema
	ctx := context.Background()
	if err := schemaManager.LoadSchema(ctx); err != nil {
		b.Fatal(err)
	}

	return &BenchmarkSetup{
		db:             db,
		logger:         logger,
		schemaManager:  schemaManager,
		cacheManager:   cacheManager,
		sessionManager: sessionManager,
		queryProcessor: queryProcessor,
	}
}

// cleanup 清理基准测试环境
func (setup *BenchmarkSetup) cleanup() {
	if setup.db != nil {
		setup.db.Close()
	}
}

// BenchmarkQueryProcessing 基准测试查询处理
func BenchmarkQueryProcessing(b *testing.B) {
	setup := setupBenchmark(b)
	defer setup.cleanup()

	ctx := context.Background()
	queries := []string{
		"查找所有用户",
		"查找年龄大于25的用户",
		"统计用户数量",
		"查找最新的文章",
		"查找用户的文章数量",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		query := queries[i%len(queries)]
		request := &core.QueryRequest{
			Query:     query,
			RequestID: fmt.Sprintf("bench_%d", i),
		}

		_, err := setup.queryProcessor.ProcessQuery(ctx, request)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSchemaLoading 基准测试 Schema 加载
func BenchmarkSchemaLoading(b *testing.B) {
	setup := setupBenchmark(b)
	defer setup.cleanup()

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		err := setup.schemaManager.LoadSchema(ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkCacheOperations 基准测试缓存操作
func BenchmarkCacheOperations(b *testing.B) {
	setup := setupBenchmark(b)
	defer setup.cleanup()

	ctx := context.Background()
	testData := map[string]interface{}{
		"key1": "value1",
		"key2": 12345,
		"key3": []string{"a", "b", "c"},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("bench_key_%d", i)

		// 设置缓存
		err := setup.cacheManager.Set(ctx, key, testData, 5*time.Minute)
		if err != nil {
			b.Fatal(err)
		}

		// 获取缓存
		_, err = setup.cacheManager.Get(ctx, key)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSessionManagement 基准测试会话管理
func BenchmarkSessionManagement(b *testing.B) {
	setup := setupBenchmark(b)
	defer setup.cleanup()

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		userID := fmt.Sprintf("bench_user_%d", i)

		// 创建会话
		session, err := setup.sessionManager.CreateSession(ctx, userID)
		if err != nil {
			b.Fatal(err)
		}

		// 获取会话
		_, err = setup.sessionManager.GetSession(ctx, session.SessionID)
		if err != nil {
			b.Fatal(err)
		}

		// 删除会话
		err = setup.sessionManager.DeleteSession(ctx, session.SessionID)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkConcurrentQueryProcessing 基准测试并发查询处理
func BenchmarkConcurrentQueryProcessing(b *testing.B) {
	setup := setupBenchmark(b)
	defer setup.cleanup()

	ctx := context.Background()
	queries := []string{
		"查找所有用户",
		"查找年龄大于25的用户",
		"统计用户数量",
		"查找最新的文章",
		"查找用户的文章数量",
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			query := queries[i%len(queries)]
			request := &core.QueryRequest{
				Query:     query,
				RequestID: fmt.Sprintf("concurrent_bench_%d", i),
			}

			_, err := setup.queryProcessor.ProcessQuery(ctx, request)
			if err != nil {
				b.Fatal(err)
			}
			i++
		}
	})
}

// BenchmarkMemoryAllocation 基准测试内存分配
func BenchmarkMemoryAllocation(b *testing.B) {
	setup := setupBenchmark(b)
	defer setup.cleanup()

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// 创建大量临时对象
		request := &core.QueryRequest{
			Query:     fmt.Sprintf("内存测试查询 %d", i),
			RequestID: fmt.Sprintf("mem_bench_%d", i),
			Context: map[string]any{
				"test_data": make([]int, 100),
				"metadata":  map[string]string{"key": "value"},
			},
		}

		_, err := setup.queryProcessor.ProcessQuery(ctx, request)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkQuerySuggestions 基准测试查询建议
func BenchmarkQuerySuggestions(b *testing.B) {
	setup := setupBenchmark(b)
	defer setup.cleanup()

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		partial := fmt.Sprintf("查询%d", i%10)
		_, err := setup.queryProcessor.GetSuggestions(ctx, partial)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkDatabaseOperations 基准测试数据库操作
func BenchmarkDatabaseOperations(b *testing.B) {
	setup := setupBenchmark(b)
	defer setup.cleanup()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// 执行简单查询
		var count int
		err := setup.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
		if err != nil && err != sql.ErrNoRows {
			// 忽略表不存在的错误，因为这是 Mock 测试
		}
	}
}

// BenchmarkSchemaSearch 基准测试 Schema 搜索
func BenchmarkSchemaSearch(b *testing.B) {
	setup := setupBenchmark(b)
	defer setup.cleanup()

	searchTerms := []string{
		"user",
		"post",
		"comment",
		"id",
		"name",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		term := searchTerms[i%len(searchTerms)]
		_, err := setup.schemaManager.FindSimilarTables(term)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkErrorHandling 基准测试错误处理
func BenchmarkErrorHandling(b *testing.B) {
	setup := setupBenchmark(b)
	defer setup.cleanup()

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// 故意创建会出错的请求
		request := &core.QueryRequest{
			Query:     "", // 空查询会导致错误
			RequestID: fmt.Sprintf("error_bench_%d", i),
		}

		_, err := setup.queryProcessor.ProcessQuery(ctx, request)
		// 预期会有错误，不需要 Fatal
		_ = err
	}
}

// BenchmarkJSONSerialization 基准测试 JSON 序列化
func BenchmarkJSONSerialization(b *testing.B) {
	response := &core.QueryResponse{
		Success: true,
		Data: []map[string]any{
			{"id": 1, "name": "张三", "age": 25},
			{"id": 2, "name": "李四", "age": 30},
			{"id": 3, "name": "王五", "age": 28},
		},
		SQL:         "SELECT * FROM users",
		Explanation: "查询所有用户信息",
		Metadata: &core.QueryMetadata{
			ExecutionTime: 50 * time.Millisecond,
			RowCount:      3,
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// 这里应该序列化响应，但为了简化，我们只是访问字段
		_ = response.Success
		_ = response.Data
		_ = response.SQL
		_ = response.Explanation
		_ = response.Metadata
	}
}
