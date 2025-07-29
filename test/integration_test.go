package test

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/Anniext/rag/cache"
	"github.com/Anniext/rag/core"
	"github.com/Anniext/rag/schema"
	"github.com/Anniext/rag/session"
	"os"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// IntegrationTestSuite 集成测试套件
type IntegrationTestSuite struct {
	db             *sql.DB
	logger         core.Logger
	schemaManager  *schema.Manager
	cacheManager   *cache.Manager
	sessionManager *session.Manager
	queryProcessor core.RAGQueryProcessor
	mcpServer      core.MCPServer
}

// NewIntegrationTestSuite 创建集成测试套件
func NewIntegrationTestSuite() *IntegrationTestSuite {
	return &IntegrationTestSuite{}
}

// Setup 设置集成测试环境
func (suite *IntegrationTestSuite) Setup() error {
	// 创建日志记录器
	zapLogger, err := zap.NewDevelopment()
	if err != nil {
		return err
	}
	suite.logger = NewZapLoggerAdapter(zapLogger)

	// 连接数据库
	dsn := getEnvOrDefault("INTEGRATION_DB_DSN", "root:123456@tcp(localhost:3306)/rag_integration_test?charset=utf8mb4&parseTime=True&loc=Local")
	suite.db, err = sql.Open("mysql", dsn)
	if err != nil {
		return err
	}

	if err := suite.db.Ping(); err != nil {
		return fmt.Errorf("数据库连接失败: %w", err)
	}

	// 设置连接池
	suite.db.SetMaxOpenConns(20)
	suite.db.SetMaxIdleConns(5)
	suite.db.SetConnMaxLifetime(time.Hour)

	return suite.setupComponents()
}

// setupComponents 设置组件
func (suite *IntegrationTestSuite) setupComponents() error {
	// 创建 Schema 加载器
	schemaLoader := &IntegrationSchemaLoader{db: suite.db}

	// 创建缓存管理器
	cacheConfig := cache.DefaultCacheConfig()
	cacheConfig.Type = cache.CacheTypeMemory
	var err error
	suite.cacheManager, err = cache.NewManager(cacheConfig, suite.logger, &IntegrationMetricsCollector{})
	if err != nil {
		return fmt.Errorf("创建缓存管理器失败: %w", err)
	}

	// 创建 Schema 管理器
	config := &core.Config{
		Database: &core.DatabaseConfig{
			Database: "integration_test_db",
		},
	}
	suite.schemaManager = schema.NewManager(schemaLoader, suite.cacheManager, suite.logger, config, nil)

	// 创建会话管理器
	suite.sessionManager = session.NewManager(2*time.Hour, suite.cacheManager, suite.logger, &IntegrationMetricsCollector{})

	// 创建查询处理器
	suite.queryProcessor = &IntegrationQueryProcessor{
		logger:         suite.logger,
		schemaManager:  suite.schemaManager,
		sessionManager: suite.sessionManager,
	}

	// 创建 MCP 服务器
	suite.mcpServer = &IntegrationMCPServer{
		queryProcessor: suite.queryProcessor,
		logger:         suite.logger,
	}

	return nil
}

// Cleanup 清理测试环境
func (suite *IntegrationTestSuite) Cleanup() {
	if suite.db != nil {
		suite.db.Close()
	}
	// 日志器清理已在适配器中处理
}

// TestFullWorkflow 测试完整工作流程
func TestFullWorkflow(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") != "true" {
		t.Skip("跳过集成测试，设置 INTEGRATION_TEST=true 启用")
	}

	suite := NewIntegrationTestSuite()
	err := suite.Setup()
	require.NoError(t, err)
	defer suite.Cleanup()

	ctx := context.Background()

	// 1. 加载 Schema
	t.Run("LoadSchema", func(t *testing.T) {
		err := suite.schemaManager.LoadSchema(ctx)
		require.NoError(t, err)

		// 验证 Schema 加载成功
		schema := suite.schemaManager.GetSchema()
		require.NotNil(t, schema)
		assert.Equal(t, "integration_test_db", schema.Database)
		assert.True(t, len(schema.Tables) > 0)
	})

	// 2. 创建用户会话
	var sessionID string
	t.Run("CreateSession", func(t *testing.T) {
		sessionMemory, err := suite.sessionManager.CreateSession(ctx, "test_user_123")
		require.NoError(t, err)
		require.NotNil(t, sessionMemory)

		sessionID = sessionMemory.SessionID
		assert.Equal(t, "test_user_123", sessionMemory.UserID)
		assert.NotEmpty(t, sessionMemory.SessionID)
	})

	// 3. 执行查询
	t.Run("ProcessQueries", func(t *testing.T) {
		queries := []struct {
			name     string
			query    string
			expected bool
		}{
			{"简单查询", "查找所有用户", true},
			{"统计查询", "统计用户数量", true},
			{"复杂查询", "查找年龄大于25岁的活跃用户", true},
			{"空查询", "", false},
		}

		for _, tc := range queries {
			t.Run(tc.name, func(t *testing.T) {
				request := &core.QueryRequest{
					Query:     tc.query,
					SessionID: sessionID,
					UserID:    "test_user_123",
					RequestID: fmt.Sprintf("integration_test_%s", tc.name),
				}

				response, err := suite.queryProcessor.ProcessQuery(ctx, request)

				if tc.expected {
					require.NoError(t, err)
					require.NotNil(t, response)
					assert.True(t, response.Success)
					assert.NotEmpty(t, response.SQL)
					assert.NotEmpty(t, response.Explanation)
					assert.Equal(t, request.RequestID, response.RequestID)
				} else {
					// 空查询应该返回错误或失败响应
					if err == nil {
						assert.False(t, response.Success)
					}
				}
			})
		}
	})

	// 4. 测试查询建议
	t.Run("GetSuggestions", func(t *testing.T) {
		suggestions, err := suite.queryProcessor.GetSuggestions(ctx, "查找用户")
		require.NoError(t, err)
		assert.True(t, len(suggestions) > 0)
	})

	// 5. 测试查询解释
	t.Run("ExplainQuery", func(t *testing.T) {
		explanation, err := suite.queryProcessor.ExplainQuery(ctx, "查找所有用户")
		require.NoError(t, err)
		require.NotNil(t, explanation)
		assert.NotEmpty(t, explanation.Intent)
		assert.True(t, len(explanation.Tables) > 0)
	})

	// 6. 测试会话历史
	t.Run("SessionHistory", func(t *testing.T) {
		sessionMemory, err := suite.sessionManager.GetSession(ctx, sessionID)
		require.NoError(t, err)
		require.NotNil(t, sessionMemory)

		// 会话应该包含之前的查询历史
		assert.True(t, len(sessionMemory.History) > 0)

		// 验证历史记录
		for _, history := range sessionMemory.History {
			assert.NotEmpty(t, history.Query)
			assert.NotEmpty(t, history.SQL)
			assert.True(t, history.Success)
		}
	})

	// 7. 测试缓存功能
	t.Run("CacheFunction", func(t *testing.T) {
		// 执行相同查询两次，第二次应该更快（从缓存获取）
		request := &core.QueryRequest{
			Query:     "缓存测试查询",
			SessionID: sessionID,
			UserID:    "test_user_123",
			RequestID: "cache_test_1",
		}

		// 第一次查询
		start1 := time.Now()
		response1, err := suite.queryProcessor.ProcessQuery(ctx, request)
		duration1 := time.Since(start1)
		require.NoError(t, err)
		require.NotNil(t, response1)

		// 第二次查询（相同内容）
		request.RequestID = "cache_test_2"
		start2 := time.Now()
		response2, err := suite.queryProcessor.ProcessQuery(ctx, request)
		duration2 := time.Since(start2)
		require.NoError(t, err)
		require.NotNil(t, response2)

		// 验证缓存效果（第二次查询应该更快）
		assert.True(t, duration2 < duration1, "缓存查询应该更快")
		assert.Equal(t, response1.SQL, response2.SQL)
	})

	// 8. 测试 Schema 搜索
	t.Run("SchemaSearch", func(t *testing.T) {
		// 搜索表
		tables, err := suite.schemaManager.FindSimilarTables("user")
		require.NoError(t, err)
		assert.True(t, len(tables) > 0)

		// 搜索列
		columns, err := suite.schemaManager.SearchColumns("name")
		require.NoError(t, err)
		assert.True(t, len(columns) > 0)
	})

	// 9. 清理会话
	t.Run("CleanupSession", func(t *testing.T) {
		err := suite.sessionManager.DeleteSession(ctx, sessionID)
		require.NoError(t, err)

		// 验证会话已删除
		_, err = suite.sessionManager.GetSession(ctx, sessionID)
		assert.Error(t, err)
	})
}

// TestMCPProtocol 测试 MCP 协议
func TestMCPProtocol(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") != "true" {
		t.Skip("跳过集成测试，设置 INTEGRATION_TEST=true 启用")
	}

	suite := NewIntegrationTestSuite()
	err := suite.Setup()
	require.NoError(t, err)
	defer suite.Cleanup()

	ctx := context.Background()

	// 加载 Schema
	err = suite.schemaManager.LoadSchema(ctx)
	require.NoError(t, err)

	// 测试不同的 MCP 方法
	t.Run("QueryExecute", func(t *testing.T) {
		request := &core.MCPRequest{
			ID:     "mcp_test_1",
			Method: "query/execute",
			Params: map[string]any{
				"query":      "查找所有用户",
				"user_id":    "mcp_test_user",
				"session_id": "mcp_test_session",
			},
		}

		response, err := suite.mcpServer.(*IntegrationMCPServer).HandleRequest(ctx, request)
		require.NoError(t, err)
		require.NotNil(t, response)
		assert.Equal(t, request.ID, response.ID)
		assert.Nil(t, response.Error)
		assert.NotNil(t, response.Result)
	})

	t.Run("QueryExplain", func(t *testing.T) {
		request := &core.MCPRequest{
			ID:     "mcp_test_2",
			Method: "query/explain",
			Params: map[string]any{
				"query": "统计用户数量",
			},
		}

		response, err := suite.mcpServer.(*IntegrationMCPServer).HandleRequest(ctx, request)
		require.NoError(t, err)
		require.NotNil(t, response)
		assert.Equal(t, request.ID, response.ID)
		assert.Nil(t, response.Error)
	})

	t.Run("SchemaInfo", func(t *testing.T) {
		request := &core.MCPRequest{
			ID:     "mcp_test_3",
			Method: "schema/info",
			Params: map[string]any{
				"table_name": "users",
			},
		}

		response, err := suite.mcpServer.(*IntegrationMCPServer).HandleRequest(ctx, request)
		require.NoError(t, err)
		require.NotNil(t, response)
		assert.Equal(t, request.ID, response.ID)
		assert.Nil(t, response.Error)
	})
}

// TestErrorHandling 测试错误处理
func TestErrorHandling(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") != "true" {
		t.Skip("跳过集成测试，设置 INTEGRATION_TEST=true 启用")
	}

	suite := NewIntegrationTestSuite()
	err := suite.Setup()
	require.NoError(t, err)
	defer suite.Cleanup()

	ctx := context.Background()

	// 加载 Schema
	err = suite.schemaManager.LoadSchema(ctx)
	require.NoError(t, err)

	t.Run("InvalidQuery", func(t *testing.T) {
		request := &core.QueryRequest{
			Query:     "", // 空查询
			UserID:    "test_user",
			RequestID: "error_test_1",
		}

		response, err := suite.queryProcessor.ProcessQuery(ctx, request)
		// 应该返回错误响应而不是抛出异常
		if err == nil {
			assert.False(t, response.Success)
			assert.NotEmpty(t, response.Error)
		}
	})

	t.Run("NonexistentTable", func(t *testing.T) {
		_, err := suite.schemaManager.GetTableInfo("nonexistent_table")
		assert.Error(t, err)
	})

	t.Run("InvalidSessionID", func(t *testing.T) {
		_, err := suite.sessionManager.GetSession(ctx, "invalid_session_id")
		assert.Error(t, err)
	})
}

// TestConcurrentAccess 测试并发访问
func TestConcurrentAccess(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") != "true" {
		t.Skip("跳过集成测试，设置 INTEGRATION_TEST=true 启用")
	}

	suite := NewIntegrationTestSuite()
	err := suite.Setup()
	require.NoError(t, err)
	defer suite.Cleanup()

	ctx := context.Background()

	// 加载 Schema
	err = suite.schemaManager.LoadSchema(ctx)
	require.NoError(t, err)

	// 并发执行查询
	concurrency := 10
	queriesPerWorker := 20

	results := make(chan error, concurrency*queriesPerWorker)

	for i := 0; i < concurrency; i++ {
		go func(workerID int) {
			for j := 0; j < queriesPerWorker; j++ {
				request := &core.QueryRequest{
					Query:     fmt.Sprintf("并发测试查询 worker %d query %d", workerID, j),
					UserID:    fmt.Sprintf("concurrent_user_%d", workerID),
					RequestID: fmt.Sprintf("concurrent_%d_%d", workerID, j),
				}

				_, err := suite.queryProcessor.ProcessQuery(ctx, request)
				results <- err
			}
		}(i)
	}

	// 收集结果
	successCount := 0
	errorCount := 0
	for i := 0; i < concurrency*queriesPerWorker; i++ {
		err := <-results
		if err == nil {
			successCount++
		} else {
			errorCount++
		}
	}

	// 验证并发访问结果
	assert.True(t, successCount > 0, "应该有成功的查询")
	assert.True(t, float64(errorCount)/float64(successCount+errorCount) < 0.1, "错误率应该低于10%")
}

// Mock 实现
type IntegrationSchemaLoader struct {
	db *sql.DB
}

func (m *IntegrationSchemaLoader) LoadSchema(ctx context.Context) (*core.SchemaInfo, error) {
	return &core.SchemaInfo{
		Database: "integration_test_db",
		Tables: []*core.TableInfo{
			{
				Name:    "users",
				Comment: "用户表",
				Columns: []*core.Column{
					{Name: "id", Type: "int", IsPrimaryKey: true, IsAutoIncrement: true},
					{Name: "name", Type: "varchar(100)", Comment: "用户姓名"},
					{Name: "email", Type: "varchar(255)", Comment: "邮箱地址"},
					{Name: "age", Type: "int", Comment: "年龄"},
					{Name: "created_at", Type: "datetime", Comment: "创建时间"},
					{Name: "updated_at", Type: "datetime", Comment: "更新时间"},
				},
				Indexes: []*core.Index{
					{Name: "PRIMARY", Type: "PRIMARY", Columns: []string{"id"}, IsUnique: true},
					{Name: "idx_email", Type: "UNIQUE", Columns: []string{"email"}, IsUnique: true},
					{Name: "idx_name", Type: "INDEX", Columns: []string{"name"}, IsUnique: false},
				},
				RowCount:   1000,
				DataLength: 1024 * 1024,
			},
			{
				Name:    "orders",
				Comment: "订单表",
				Columns: []*core.Column{
					{Name: "id", Type: "int", IsPrimaryKey: true, IsAutoIncrement: true},
					{Name: "user_id", Type: "int", Comment: "用户ID"},
					{Name: "amount", Type: "decimal(10,2)", Comment: "订单金额"},
					{Name: "status", Type: "varchar(20)", Comment: "订单状态"},
					{Name: "created_at", Type: "datetime", Comment: "创建时间"},
				},
				ForeignKeys: []*core.ForeignKey{
					{
						Name:             "fk_orders_user_id",
						Column:           "user_id",
						ReferencedTable:  "users",
						ReferencedColumn: "id",
						OnUpdate:         "CASCADE",
						OnDelete:         "CASCADE",
					},
				},
				RowCount:   5000,
				DataLength: 2 * 1024 * 1024,
			},
		},
		Version:   "1.0",
		UpdatedAt: time.Now(),
	}, nil
}

func (m *IntegrationSchemaLoader) GetTableInfo(ctx context.Context, tableName string) (*core.TableInfo, error) {
	schema, err := m.LoadSchema(ctx)
	if err != nil {
		return nil, err
	}

	for _, table := range schema.Tables {
		if table.Name == tableName {
			return table, nil
		}
	}

	return nil, fmt.Errorf("表 %s 不存在", tableName)
}

func (m *IntegrationSchemaLoader) GetTableStatistics(ctx context.Context, tableName string) (map[string]interface{}, error) {
	stats := map[string]interface{}{
		"users": map[string]interface{}{
			"row_count":   1000,
			"data_length": 1024 * 1024,
			"index_count": 3,
		},
		"orders": map[string]interface{}{
			"row_count":   5000,
			"data_length": 2 * 1024 * 1024,
			"index_count": 2,
		},
	}

	if stat, exists := stats[tableName]; exists {
		return stat.(map[string]interface{}), nil
	}

	return nil, fmt.Errorf("表 %s 的统计信息不存在", tableName)
}

func (m *IntegrationSchemaLoader) ValidateConnection(ctx context.Context) error {
	return m.db.PingContext(ctx)
}

func (m *IntegrationSchemaLoader) Close() error {
	return nil
}

func (m *IntegrationSchemaLoader) GetTableNames(ctx context.Context) ([]string, error) {
	return []string{"users", "orders", "products", "categories"}, nil
}

type IntegrationMetricsCollector struct{}

func (m *IntegrationMetricsCollector) IncrementCounter(name string, labels map[string]string) {}
func (m *IntegrationMetricsCollector) RecordHistogram(name string, value float64, labels map[string]string) {
}
func (m *IntegrationMetricsCollector) SetGauge(name string, value float64, labels map[string]string) {
}

type IntegrationQueryProcessor struct {
	logger         core.Logger
	schemaManager  *schema.Manager
	sessionManager *session.Manager
}

func (m *IntegrationQueryProcessor) ProcessQuery(ctx context.Context, request *core.QueryRequest) (*core.QueryResponse, error) {
	// 验证输入
	if request.Query == "" {
		return &core.QueryResponse{
			Success:   false,
			Error:     "查询不能为空",
			RequestID: request.RequestID,
		}, nil
	}

	// 模拟查询处理时间
	processingTime := time.Duration(10+len(request.Query)) * time.Millisecond
	time.Sleep(processingTime)

	// 生成 SQL
	sql := fmt.Sprintf("SELECT * FROM users WHERE name LIKE '%%%s%%'", request.Query)

	// 模拟查询结果
	var data []map[string]any
	switch {
	case len(request.Query) > 20:
		// 复杂查询
		for i := 0; i < 5; i++ {
			data = append(data, map[string]any{
				"id":         i + 1,
				"name":       fmt.Sprintf("用户%d", i+1),
				"email":      fmt.Sprintf("user%d@example.com", i+1),
				"age":        25 + i,
				"created_at": time.Now().Add(-time.Duration(i) * 24 * time.Hour),
			})
		}
	default:
		// 简单查询
		data = append(data, map[string]any{
			"id":    1,
			"name":  "测试用户",
			"email": "test@example.com",
			"age":   30,
		})
	}

	// 更新会话历史
	if request.SessionID != "" {
		if sessionMemory, err := m.sessionManager.GetSession(ctx, request.SessionID); err == nil {
			history := &core.QueryHistory{
				Query:         request.Query,
				SQL:           sql,
				Results:       data,
				Timestamp:     time.Now(),
				Success:       true,
				ExecutionTime: processingTime,
			}
			sessionMemory.History = append(sessionMemory.History, history)
			m.sessionManager.UpdateSession(ctx, sessionMemory)
		}
	}

	return &core.QueryResponse{
		Success:     true,
		Data:        data,
		SQL:         sql,
		Explanation: fmt.Sprintf("查询包含 '%s' 的用户信息", request.Query),
		Metadata: &core.QueryMetadata{
			ExecutionTime: processingTime,
			RowCount:      len(data),
		},
		RequestID: request.RequestID,
	}, nil
}

func (m *IntegrationQueryProcessor) GetSuggestions(ctx context.Context, partial string) ([]string, error) {
	suggestions := []string{
		"查找所有用户",
		"统计用户数量",
		"查找活跃用户",
		"分析用户年龄分布",
		"查找最近注册的用户",
	}

	// 基于输入过滤建议
	if partial != "" {
		var filtered []string
		for _, suggestion := range suggestions {
			if len(suggestion) > len(partial) && suggestion[:len(partial)] == partial {
				filtered = append(filtered, suggestion)
			}
		}
		if len(filtered) > 0 {
			return filtered, nil
		}
	}

	return suggestions, nil
}

func (m *IntegrationQueryProcessor) ExplainQuery(ctx context.Context, query string) (*core.QueryExplanation, error) {
	return &core.QueryExplanation{
		Intent:      "查询用户信息",
		Tables:      []string{"users"},
		Columns:     []string{"id", "name", "email", "age"},
		Conditions:  []string{fmt.Sprintf("name LIKE '%%%s%%'", query)},
		Operations:  []string{"SELECT"},
		Complexity:  "simple",
		Suggestions: []string{"可以添加索引以提高查询性能"},
	}, nil
}

type IntegrationMCPServer struct {
	queryProcessor core.RAGQueryProcessor
	logger         core.Logger
}

func (s *IntegrationMCPServer) Start(ctx context.Context, addr string) error {
	s.logger.Info("MCP 服务器启动", zap.String("addr", addr))
	return nil
}

func (s *IntegrationMCPServer) Stop(ctx context.Context) error {
	s.logger.Info("MCP 服务器停止")
	return nil
}

func (s *IntegrationMCPServer) RegisterHandler(method string, handler core.MCPHandler) error {
	s.logger.Info("注册 MCP 处理器", zap.String("method", method))
	return nil
}

func (s *IntegrationMCPServer) BroadcastNotification(notification *core.MCPNotification) error {
	s.logger.Info("广播 MCP 通知", zap.String("method", notification.Method))
	return nil
}

func (s *IntegrationMCPServer) HandleRequest(ctx context.Context, request *core.MCPRequest) (*core.MCPResponse, error) {
	switch request.Method {
	case "query/execute":
		return s.handleQueryExecute(ctx, request)
	case "query/explain":
		return s.handleQueryExplain(ctx, request)
	case "query/suggest":
		return s.handleQuerySuggest(ctx, request)
	case "schema/info":
		return s.handleSchemaInfo(ctx, request)
	default:
		return &core.MCPResponse{
			ID: request.ID,
			Error: &core.MCPError{
				Code:    -32601,
				Message: fmt.Sprintf("未知方法: %s", request.Method),
			},
		}, nil
	}
}

func (s *IntegrationMCPServer) handleQueryExecute(ctx context.Context, request *core.MCPRequest) (*core.MCPResponse, error) {
	params, ok := request.Params.(map[string]any)
	if !ok {
		return &core.MCPResponse{
			ID: request.ID,
			Error: &core.MCPError{
				Code:    -32602,
				Message: "无效的参数",
			},
		}, nil
	}

	query, _ := params["query"].(string)
	userID, _ := params["user_id"].(string)
	sessionID, _ := params["session_id"].(string)

	queryRequest := &core.QueryRequest{
		Query:     query,
		UserID:    userID,
		SessionID: sessionID,
		RequestID: request.ID,
	}

	response, err := s.queryProcessor.ProcessQuery(ctx, queryRequest)
	if err != nil {
		return &core.MCPResponse{
			ID: request.ID,
			Error: &core.MCPError{
				Code:    -32603,
				Message: err.Error(),
			},
		}, nil
	}

	return &core.MCPResponse{
		ID:     request.ID,
		Result: response,
	}, nil
}

func (s *IntegrationMCPServer) handleQueryExplain(ctx context.Context, request *core.MCPRequest) (*core.MCPResponse, error) {
	params, ok := request.Params.(map[string]any)
	if !ok {
		return &core.MCPResponse{
			ID: request.ID,
			Error: &core.MCPError{
				Code:    -32602,
				Message: "无效的参数",
			},
		}, nil
	}

	query, _ := params["query"].(string)

	explanation, err := s.queryProcessor.ExplainQuery(ctx, query)
	if err != nil {
		return &core.MCPResponse{
			ID: request.ID,
			Error: &core.MCPError{
				Code:    -32603,
				Message: err.Error(),
			},
		}, nil
	}

	return &core.MCPResponse{
		ID:     request.ID,
		Result: explanation,
	}, nil
}

func (s *IntegrationMCPServer) handleQuerySuggest(ctx context.Context, request *core.MCPRequest) (*core.MCPResponse, error) {
	params, ok := request.Params.(map[string]any)
	if !ok {
		return &core.MCPResponse{
			ID: request.ID,
			Error: &core.MCPError{
				Code:    -32602,
				Message: "无效的参数",
			},
		}, nil
	}

	partial, _ := params["partial"].(string)

	suggestions, err := s.queryProcessor.GetSuggestions(ctx, partial)
	if err != nil {
		return &core.MCPResponse{
			ID: request.ID,
			Error: &core.MCPError{
				Code:    -32603,
				Message: err.Error(),
			},
		}, nil
	}

	return &core.MCPResponse{
		ID:     request.ID,
		Result: suggestions,
	}, nil
}

func (s *IntegrationMCPServer) handleSchemaInfo(ctx context.Context, request *core.MCPRequest) (*core.MCPResponse, error) {
	// 简化实现，返回模拟的 schema 信息
	schemaInfo := map[string]any{
		"database": "integration_test_db",
		"tables": []map[string]any{
			{
				"name":    "users",
				"comment": "用户表",
				"columns": []map[string]any{
					{"name": "id", "type": "int", "is_primary_key": true},
					{"name": "name", "type": "varchar(100)", "comment": "用户姓名"},
					{"name": "email", "type": "varchar(255)", "comment": "邮箱地址"},
				},
			},
		},
	}

	return &core.MCPResponse{
		ID:     request.ID,
		Result: schemaInfo,
	}, nil
}
