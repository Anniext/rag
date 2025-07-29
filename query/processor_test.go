package query

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"pumppill/rag/core"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockCacheManager 模拟 CacheManager
type MockCacheManager struct {
	mock.Mock
}

func (m *MockCacheManager) Get(ctx context.Context, key string) (interface{}, error) {
	args := m.Called(ctx, key)
	return args.Get(0), args.Error(1)
}

func (m *MockCacheManager) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	args := m.Called(ctx, key, value, ttl)
	return args.Error(0)
}

func (m *MockCacheManager) Delete(ctx context.Context, key string) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

func (m *MockCacheManager) Clear(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// MockLangChainManager 模拟 LangChainManager
type MockLangChainManager struct {
	mock.Mock
}

func (m *MockLangChainManager) CreateChain(ctx context.Context, config *core.ChainConfig) (core.Chain, error) {
	args := m.Called(ctx, config)
	return args.Get(0).(core.Chain), args.Error(1)
}

func (m *MockLangChainManager) ExecuteChain(ctx context.Context, chain core.Chain, input *core.ChainInput) (*core.ChainOutput, error) {
	args := m.Called(ctx, chain, input)
	return args.Get(0).(*core.ChainOutput), args.Error(1)
}

func (m *MockLangChainManager) GetMemory(sessionID string) (core.Memory, error) {
	args := m.Called(sessionID)
	return args.Get(0).(core.Memory), args.Error(1)
}

func (m *MockLangChainManager) UpdateMemory(sessionID string, memory core.Memory) error {
	args := m.Called(sessionID, memory)
	return args.Error(0)
}

// MockChain 模拟 Chain
type MockChain struct {
	mock.Mock
}

func (m *MockChain) Run(ctx context.Context, input *core.ChainInput) (*core.ChainOutput, error) {
	args := m.Called(ctx, input)
	return args.Get(0).(*core.ChainOutput), args.Error(1)
}

func (m *MockChain) GetTools() []core.Tool {
	args := m.Called()
	return args.Get(0).([]core.Tool)
}

func (m *MockChain) AddTool(tool core.Tool) error {
	args := m.Called(tool)
	return args.Error(0)
}

func TestNewRAGQueryProcessor(t *testing.T) {
	mockParser := &QueryParser{}
	mockGenerator := &SQLGenerator{}
	mockExecutor := &QueryExecutor{}
	mockCacheManager := &MockCacheManager{}
	mockLangChain := &MockLangChainManager{}
	mockLogger := &MockLogger{}

	processor := NewRAGQueryProcessor(
		mockParser,
		mockGenerator,
		mockExecutor,
		mockCacheManager,
		mockLangChain,
		mockLogger,
		nil,
	)

	assert.NotNil(t, processor)
	assert.Equal(t, mockParser, processor.parser)
	assert.Equal(t, mockGenerator, processor.generator)
	assert.Equal(t, mockExecutor, processor.executor)
	assert.Equal(t, mockCacheManager, processor.cacheManager)
	assert.Equal(t, mockLangChain, processor.langChain)
	assert.Equal(t, mockLogger, processor.logger)
	assert.NotNil(t, processor.config)
	assert.True(t, processor.config.EnableCache)
	assert.Equal(t, 5*time.Minute, processor.config.CacheTTL)
}

func TestNewRAGQueryProcessorWithConfig(t *testing.T) {
	mockParser := &QueryParser{}
	mockGenerator := &SQLGenerator{}
	mockExecutor := &QueryExecutor{}
	mockCacheManager := &MockCacheManager{}
	mockLangChain := &MockLangChainManager{}
	mockLogger := &MockLogger{}

	config := &ProcessorConfig{
		EnableCache:        false,
		CacheTTL:           10 * time.Minute,
		EnableExplanation:  false,
		EnableOptimization: false,
		MaxRetries:         5,
		RetryDelay:         2 * time.Second,
	}

	processor := NewRAGQueryProcessor(
		mockParser,
		mockGenerator,
		mockExecutor,
		mockCacheManager,
		mockLangChain,
		mockLogger,
		config,
	)

	assert.NotNil(t, processor)
	assert.Equal(t, config, processor.config)
}

func TestGenerateCacheKey(t *testing.T) {
	processor := &RAGQueryProcessor{}

	tests := []struct {
		name     string
		request  *core.QueryRequest
		expected string
	}{
		{
			name: "基本请求",
			request: &core.QueryRequest{
				Query:  "SELECT * FROM users",
				UserID: "user123",
			},
			expected: "query:SELECT * FROM users:user:user123",
		},
		{
			name: "带选项的请求",
			request: &core.QueryRequest{
				Query:  "SELECT * FROM users",
				UserID: "user123",
				Options: &core.QueryOptions{
					Limit:  10,
					Offset: 20,
					Format: "json",
				},
			},
			expected: "query:SELECT * FROM users:user:user123:limit:10:offset:20:format:json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.generateCacheKey(tt.request)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetUserInfo(t *testing.T) {
	processor := &RAGQueryProcessor{}

	tests := []struct {
		name     string
		ctx      context.Context
		request  *core.QueryRequest
		expected *core.UserInfo
	}{
		{
			name: "从上下文获取用户信息",
			ctx: context.WithValue(context.Background(), "user", &core.UserInfo{
				ID:       "user123",
				Username: "testuser",
				Roles:    []string{"admin"},
			}),
			request: &core.QueryRequest{UserID: "user456"},
			expected: &core.UserInfo{
				ID:       "user123",
				Username: "testuser",
				Roles:    []string{"admin"},
			},
		},
		{
			name:    "从请求构建用户信息",
			ctx:     context.Background(),
			request: &core.QueryRequest{UserID: "user456"},
			expected: &core.UserInfo{
				ID:       "user456",
				Username: "user456",
				Roles:    []string{"user"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.getUserInfo(tt.ctx, tt.request)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatConditions(t *testing.T) {
	processor := &RAGQueryProcessor{}

	conditions := []QueryCondition{
		{Column: "age", Operator: ">", Value: 25},
		{Column: "name", Operator: "=", Value: "John"},
		{Column: "status", Operator: "LIKE", Value: "active%"},
	}

	expected := []string{
		"age > 25",
		"name = John",
		"status LIKE active%",
	}

	result := processor.formatConditions(conditions)
	assert.Equal(t, expected, result)
}

func TestFormatOperations(t *testing.T) {
	processor := &RAGQueryProcessor{}

	operations := []QueryOperation{
		{
			Type: "order_by",
			Parameters: map[string]interface{}{
				"column":    "name",
				"direction": "ASC",
			},
		},
		{
			Type: "group_by",
			Parameters: map[string]interface{}{
				"column": "department",
			},
		},
		{
			Type: "limit",
			Parameters: map[string]interface{}{
				"count": "10",
			},
		},
	}

	expected := []string{
		"按 name ASC 排序",
		"按 department 分组",
		"限制 10 条结果",
	}

	result := processor.formatOperations(operations)
	assert.Equal(t, expected, result)
}

func TestGenerateBasicSuggestions(t *testing.T) {
	processor := &RAGQueryProcessor{}

	tests := []struct {
		name     string
		partial  string
		expected []string
	}{
		{
			name:    "查询开头",
			partial: "查询",
			expected: []string{
				"查询用户信息",
				"查询订单数据",
				"查询产品列表",
				"查询销售统计",
			},
		},
		{
			name:    "统计开头",
			partial: "统计",
			expected: []string{
				"统计用户数量",
				"统计订单总数",
				"统计销售额",
			},
		},
		{
			name:    "分析开头",
			partial: "分析",
			expected: []string{
				"分析用户行为",
				"分析销售趋势",
				"分析产品性能",
			},
		},
		{
			name:    "其他输入",
			partial: "其他",
			expected: []string{
				"查询用户信息",
				"统计订单数量",
				"分析销售数据",
				"查询产品列表",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.generateBasicSuggestions(tt.partial)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGenerateSuggestions(t *testing.T) {
	processor := &RAGQueryProcessor{}

	tests := []struct {
		name     string
		parsed   *ParsedQuery
		result   *ExecutionResult
		expected []string
	}{
		{
			name: "复杂查询建议",
			parsed: &ParsedQuery{
				Complexity:  "complex",
				Suggestions: []string{"使用索引"},
			},
			result: &ExecutionResult{
				RowCount:      100,
				ExecutionTime: 6 * time.Second,
			},
			expected: []string{
				"复杂查询可能影响性能，建议添加适当的索引",
				"查询执行时间较长，建议优化查询条件",
				"使用索引",
			},
		},
		{
			name: "无结果建议",
			parsed: &ParsedQuery{
				Complexity:  "simple",
				Suggestions: []string{},
			},
			result: &ExecutionResult{
				RowCount:      0,
				ExecutionTime: 1 * time.Second,
			},
			expected: []string{
				"查询未返回结果，请检查查询条件",
			},
		},
		{
			name: "大结果集建议",
			parsed: &ParsedQuery{
				Complexity:  "medium",
				Suggestions: []string{},
			},
			result: &ExecutionResult{
				RowCount:      15000,
				ExecutionTime: 2 * time.Second,
			},
			expected: []string{
				"结果集较大，建议使用分页查询",
				"结果集过大，建议添加更具体的查询条件",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.generateSuggestions(context.Background(), tt.parsed, tt.result)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCreateErrorResponse(t *testing.T) {
	mockLogger := &MockLogger{}
	processor := &RAGQueryProcessor{
		logger: mockLogger,
	}

	request := &core.QueryRequest{
		Query:     "SELECT * FROM users",
		RequestID: "req123",
	}

	err := assert.AnError
	response := processor.createErrorResponse(request, err)

	assert.False(t, response.Success)
	assert.Equal(t, err.Error(), response.Error)
	assert.Equal(t, "req123", response.RequestID)
	assert.Empty(t, response.Data)
	assert.NotNil(t, response.Metadata)
	assert.False(t, response.Metadata.CacheHit)
	assert.Equal(t, 0, response.Metadata.RowCount)
}

func TestGetSuggestions(t *testing.T) {
	mockLangChain := &MockLangChainManager{}
	mockLogger := &MockLogger{}
	processor := &RAGQueryProcessor{
		langChain: mockLangChain,
		logger:    mockLogger,
	}

	ctx := context.Background()
	partial := "查询用户"

	// 测试基础建议（LangChain 失败时的回退）
	mockChain := &MockChain{}
	mockLangChain.On("CreateChain", ctx, mock.AnythingOfType("*core.ChainConfig")).Return(mockChain, assert.AnError)

	suggestions, err := processor.GetSuggestions(ctx, partial)

	assert.NoError(t, err)
	assert.NotEmpty(t, suggestions)
	assert.Contains(t, suggestions, "查询用户信息")

	mockLangChain.AssertExpectations(t)
}

func TestExplainQuery(t *testing.T) {
	// 创建模拟对象
	mockSchemaManager := &MockSchemaManager{}
	mockLogger := &MockLogger{}

	parser := NewQueryParser(mockSchemaManager, mockLogger)
	generator := NewSQLGenerator(mockSchemaManager, mockLogger)

	processor := &RAGQueryProcessor{
		parser:    parser,
		generator: generator,
		logger:    mockLogger,
	}

	ctx := context.Background()
	query := "查询用户信息"

	// 设置 mock 期望
	mockSchemaManager.On("FindSimilarTables", mock.Anything).Return([]*core.TableInfo{
		{Name: "users"},
	}, nil)
	mockSchemaManager.On("GetTableInfo", "users").Return(&core.TableInfo{
		Name: "users",
		Columns: []*core.Column{
			{Name: "id", Type: "int"},
			{Name: "name", Type: "varchar"},
		},
	}, nil)
	mockSchemaManager.On("GetRelationships", "users").Return([]*core.Relationship{}, nil).Maybe()

	explanation, err := processor.ExplainQuery(ctx, query)

	assert.NoError(t, err)
	assert.NotNil(t, explanation)
	assert.Equal(t, "select", explanation.Intent)
	assert.Contains(t, explanation.Tables, "users")

	mockSchemaManager.AssertExpectations(t)
}

func TestGetProcessorStats(t *testing.T) {
	// 创建一个模拟的数据库连接
	mockDB := &sql.DB{}
	mockSecurityManager := &MockSecurityManager{}
	mockLogger := &MockLogger{}

	mockExecutor := NewQueryExecutor(mockDB, mockSecurityManager, mockLogger, nil)
	processor := &RAGQueryProcessor{
		executor: mockExecutor,
		config: &ProcessorConfig{
			EnableCache:        true,
			CacheTTL:           5 * time.Minute,
			EnableExplanation:  true,
			EnableOptimization: true,
			MaxRetries:         3,
			RetryDelay:         1 * time.Second,
		},
	}

	ctx := context.Background()
	stats, err := processor.GetProcessorStats(ctx)

	assert.NoError(t, err)
	assert.NotNil(t, stats)
	assert.Contains(t, stats, "config")

	config := stats["config"].(map[string]any)
	assert.Equal(t, true, config["enable_cache"])
	assert.Equal(t, "5m0s", config["cache_ttl"])
	assert.Equal(t, true, config["enable_explanation"])
	assert.Equal(t, true, config["enable_optimization"])
	assert.Equal(t, 3, config["max_retries"])
	assert.Equal(t, "1s", config["retry_delay"])
}
