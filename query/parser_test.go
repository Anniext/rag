package query

import (
	"context"
	"testing"

	"pumppill/rag/core"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockSchemaManager 模拟 SchemaManager
type MockSchemaManager struct {
	mock.Mock
}

func (m *MockSchemaManager) LoadSchema(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockSchemaManager) GetTableInfo(tableName string) (*core.TableInfo, error) {
	args := m.Called(tableName)
	return args.Get(0).(*core.TableInfo), args.Error(1)
}

func (m *MockSchemaManager) GetRelationships(tableName string) ([]*core.Relationship, error) {
	args := m.Called(tableName)
	return args.Get(0).([]*core.Relationship), args.Error(1)
}

func (m *MockSchemaManager) FindSimilarTables(query string) ([]*core.TableInfo, error) {
	args := m.Called(query)
	return args.Get(0).([]*core.TableInfo), args.Error(1)
}

func (m *MockSchemaManager) RefreshSchema(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// MockLogger 模拟 Logger
type MockLogger struct {
	mock.Mock
}

func (m *MockLogger) Debug(msg string, fields ...interface{}) {
	// 简化实现，避免 mock 调用问题
}

func (m *MockLogger) Info(msg string, fields ...interface{}) {
	// 简化实现
}

func (m *MockLogger) Warn(msg string, fields ...interface{}) {
	// 简化实现
}

func (m *MockLogger) Error(msg string, fields ...interface{}) {
	// 简化实现
}

func (m *MockLogger) Fatal(msg string, fields ...interface{}) {
	// 简化实现
}

func TestNewQueryParser(t *testing.T) {
	mockSchemaManager := &MockSchemaManager{}
	mockLogger := &MockLogger{}

	parser := NewQueryParser(mockSchemaManager, mockLogger)

	assert.NotNil(t, parser)
	assert.Equal(t, mockSchemaManager, parser.schemaManager)
	assert.Equal(t, mockLogger, parser.logger)
}

func TestPreprocessQuery(t *testing.T) {
	mockSchemaManager := &MockSchemaManager{}
	mockLogger := &MockLogger{}
	parser := NewQueryParser(mockSchemaManager, mockLogger)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "基本预处理",
			input:    "  显示 用户 信息  ",
			expected: "查询 用户 信息",
		},
		{
			name:     "中文转换",
			input:    "统计用户数量",
			expected: "计数用户计数",
		},
		{
			name:     "操作符转换",
			input:    "查找年龄大于25的用户",
			expected: "查找年龄>25的用户",
		},
		{
			name:     "多空格处理",
			input:    "查询    用户     信息",
			expected: "查询 用户 信息",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.preprocessQuery(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIdentifyIntent(t *testing.T) {
	mockSchemaManager := &MockSchemaManager{}
	mockLogger := &MockLogger{}
	parser := NewQueryParser(mockSchemaManager, mockLogger)

	tests := []struct {
		name     string
		query    string
		expected QueryIntent
	}{
		{
			name:     "查询意图",
			query:    "查询用户信息",
			expected: IntentSelect,
		},
		{
			name:     "插入意图",
			query:    "插入新用户",
			expected: IntentInsert,
		},
		{
			name:     "更新意图",
			query:    "更新用户信息",
			expected: IntentUpdate,
		},
		{
			name:     "删除意图",
			query:    "删除用户",
			expected: IntentDelete,
		},
		{
			name:     "计数意图",
			query:    "统计用户数量",
			expected: IntentCount,
		},
		{
			name:     "聚合意图",
			query:    "按部门分组统计",
			expected: IntentAggregate,
		},
		{
			name:     "默认查询意图",
			query:    "用户信息",
			expected: IntentSelect,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.identifyIntent(tt.query)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractTables(t *testing.T) {
	mockSchemaManager := &MockSchemaManager{}
	mockLogger := &MockLogger{}
	parser := NewQueryParser(mockSchemaManager, mockLogger)

	ctx := context.Background()

	tests := []struct {
		name     string
		query    string
		expected []string
		mockFunc func()
	}{
		{
			name:     "直接表名匹配",
			query:    "从 users 表查询",
			expected: []string{"users"},
			mockFunc: func() {},
		},
		{
			name:     "英文表名匹配",
			query:    "select from users",
			expected: []string{"users"},
			mockFunc: func() {},
		},
		{
			name:     "相似性匹配",
			query:    "查询用户信息",
			expected: []string{"users"},
			mockFunc: func() {
				mockSchemaManager.On("FindSimilarTables", "查询用户信息").Return([]*core.TableInfo{
					{Name: "users"},
				}, nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockFunc()
			result, err := parser.extractTables(ctx, tt.query)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
			mockSchemaManager.AssertExpectations(t)
		})
	}
}

func TestExtractConditions(t *testing.T) {
	mockSchemaManager := &MockSchemaManager{}
	mockLogger := &MockLogger{}
	parser := NewQueryParser(mockSchemaManager, mockLogger)

	tests := []struct {
		name     string
		query    string
		expected []QueryCondition
	}{
		{
			name:  "等于条件",
			query: "name = 'john'",
			expected: []QueryCondition{
				{
					Column:   "name",
					Operator: "=",
					Value:    "john",
					Type:     "where",
				},
			},
		},
		{
			name:  "大于条件",
			query: "age > 25",
			expected: []QueryCondition{
				{
					Column:   "age",
					Operator: ">",
					Value:    "25",
					Type:     "where",
				},
			},
		},
		{
			name:  "LIKE条件",
			query: "name like 'john%'",
			expected: []QueryCondition{
				{
					Column:   "name",
					Operator: "LIKE",
					Value:    "john%",
					Type:     "where",
				},
			},
		},
		{
			name:  "中文包含条件",
			query: "姓名 包含 '张'",
			expected: []QueryCondition{
				{
					Column:   "姓名",
					Operator: "LIKE",
					Value:    "张",
					Type:     "where",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.extractConditions(tt.query)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractOperations(t *testing.T) {
	mockSchemaManager := &MockSchemaManager{}
	mockLogger := &MockLogger{}
	parser := NewQueryParser(mockSchemaManager, mockLogger)

	tests := []struct {
		name     string
		query    string
		expected []QueryOperation
	}{
		{
			name:  "排序操作",
			query: "按 name 排序",
			expected: []QueryOperation{
				{
					Type: "order_by",
					Parameters: map[string]interface{}{
						"column": "name",
					},
				},
			},
		},
		{
			name:  "降序排序",
			query: "按 age desc 排序",
			expected: []QueryOperation{
				{
					Type: "order_by",
					Parameters: map[string]interface{}{
						"column":    "age",
						"direction": "DESC",
					},
				},
			},
		},
		{
			name:  "限制操作",
			query: "限制 10",
			expected: []QueryOperation{
				{
					Type: "limit",
					Parameters: map[string]interface{}{
						"count": "10",
					},
				},
			},
		},
		{
			name:  "分组操作",
			query: "按 department 分组",
			expected: []QueryOperation{
				{
					Type: "group_by",
					Parameters: map[string]interface{}{
						"column": "department",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.extractOperations(tt.query)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEvaluateComplexity(t *testing.T) {
	mockSchemaManager := &MockSchemaManager{}
	mockLogger := &MockLogger{}
	parser := NewQueryParser(mockSchemaManager, mockLogger)

	tests := []struct {
		name       string
		intent     QueryIntent
		tables     []string
		columns    []string
		conditions []QueryCondition
		operations []QueryOperation
		expected   string
	}{
		{
			name:       "简单查询",
			intent:     IntentSelect,
			tables:     []string{"users"},
			columns:    []string{"name"},
			conditions: []QueryCondition{},
			operations: []QueryOperation{},
			expected:   "simple",
		},
		{
			name:       "中等复杂度查询",
			intent:     IntentSelect,
			tables:     []string{"users", "orders"},
			columns:    []string{"name", "email", "order_date"},
			conditions: []QueryCondition{{}, {}},
			operations: []QueryOperation{{}},
			expected:   "medium",
		},
		{
			name:       "复杂查询",
			intent:     IntentAggregate,
			tables:     []string{"users", "orders", "products", "categories"},
			columns:    []string{"name", "email", "order_date", "product_name", "category"},
			conditions: []QueryCondition{{}, {}, {}},
			operations: []QueryOperation{{}, {}},
			expected:   "complex",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.evaluateComplexity(tt.intent, tt.tables, tt.columns, tt.conditions, tt.operations)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCalculateConfidence(t *testing.T) {
	mockSchemaManager := &MockSchemaManager{}
	mockLogger := &MockLogger{}
	parser := NewQueryParser(mockSchemaManager, mockLogger)

	tests := []struct {
		name     string
		intent   QueryIntent
		tables   []string
		columns  []string
		expected float64
	}{
		{
			name:     "高置信度",
			intent:   IntentSelect,
			tables:   []string{"users"},
			columns:  []string{"name", "email"},
			expected: 1.0,
		},
		{
			name:     "中等置信度",
			intent:   IntentSelect,
			tables:   []string{"users"},
			columns:  []string{"*"},
			expected: 0.7,
		},
		{
			name:     "低置信度",
			intent:   IntentUnknown,
			tables:   []string{},
			columns:  []string{},
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.calculateConfidence(tt.intent, tt.tables, tt.columns)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseQuery(t *testing.T) {
	mockSchemaManager := &MockSchemaManager{}
	mockLogger := &MockLogger{}
	parser := NewQueryParser(mockSchemaManager, mockLogger)

	ctx := context.Background()

	// 设置 mock 期望
	mockLogger.On("Debug", mock.Anything, mock.Anything, mock.Anything).Return()
	mockSchemaManager.On("FindSimilarTables", mock.Anything).Return([]*core.TableInfo{
		{Name: "users"},
	}, nil)
	mockSchemaManager.On("GetTableInfo", "users").Return(&core.TableInfo{
		Name: "users",
		Columns: []*core.Column{
			{Name: "id", Type: "int"},
			{Name: "name", Type: "varchar"},
			{Name: "email", Type: "varchar"},
		},
		Indexes: []*core.Index{
			{Name: "idx_id", Columns: []string{"id"}},
		},
	}, nil)

	query := "查询用户姓名和邮箱，年龄大于25，按姓名排序"

	result, err := parser.ParseQuery(ctx, query)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, IntentSelect, result.Intent)
	assert.Contains(t, result.Tables, "users")
	assert.Greater(t, result.Confidence, 0.0)
	assert.NotEmpty(t, result.Suggestions)

	mockSchemaManager.AssertExpectations(t)
}

func TestValidateQuery(t *testing.T) {
	mockSchemaManager := &MockSchemaManager{}
	mockLogger := &MockLogger{}
	parser := NewQueryParser(mockSchemaManager, mockLogger)

	ctx := context.Background()

	// 设置 mock 期望
	mockSchemaManager.On("GetTableInfo", "users").Return(&core.TableInfo{
		Name: "users",
		Columns: []*core.Column{
			{Name: "id", Type: "int"},
			{Name: "name", Type: "varchar"},
		},
	}, nil)

	tests := []struct {
		name      string
		parsed    *ParsedQuery
		expectErr bool
	}{
		{
			name: "有效查询",
			parsed: &ParsedQuery{
				Intent:  IntentSelect,
				Tables:  []string{"users"},
				Columns: []string{"name"},
			},
			expectErr: false,
		},
		{
			name: "删除操作缺少条件",
			parsed: &ParsedQuery{
				Intent:     IntentDelete,
				Tables:     []string{"users"},
				Columns:    []string{"*"},
				Conditions: []QueryCondition{},
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := parser.ValidateQuery(ctx, tt.parsed)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetQueryContext(t *testing.T) {
	mockSchemaManager := &MockSchemaManager{}
	mockLogger := &MockLogger{}
	parser := NewQueryParser(mockSchemaManager, mockLogger)

	ctx := context.Background()

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

	query := "查询用户信息"

	context, err := parser.GetQueryContext(ctx, query)

	assert.NoError(t, err)
	assert.NotNil(t, context)
	assert.Contains(t, context, "timestamp")
	assert.Contains(t, context, "query_length")
	assert.Contains(t, context, "intent")
	assert.Contains(t, context, "tables")
	assert.Contains(t, context, "table_infos")

	mockSchemaManager.AssertExpectations(t)
}
