package query

import (
	"context"
	"testing"

	"pumppill/rag/core"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestNewSQLGenerator(t *testing.T) {
	mockSchemaManager := &MockSchemaManager{}
	mockLogger := &MockLogger{}

	generator := NewSQLGenerator(mockSchemaManager, mockLogger)

	assert.NotNil(t, generator)
	assert.Equal(t, mockSchemaManager, generator.schemaManager)
	assert.Equal(t, mockLogger, generator.logger)
}

func TestGenerateSelectSQL(t *testing.T) {
	mockSchemaManager := &MockSchemaManager{}
	mockLogger := &MockLogger{}
	generator := NewSQLGenerator(mockSchemaManager, mockLogger)

	ctx := context.Background()

	// 设置 mock 期望
	mockSchemaManager.On("GetRelationships", "users").Return([]*core.Relationship{}, nil)

	tests := []struct {
		name     string
		parsed   *ParsedQuery
		expected string
	}{
		{
			name: "简单查询",
			parsed: &ParsedQuery{
				Intent:  IntentSelect,
				Tables:  []string{"users"},
				Columns: []string{"name", "email"},
				Conditions: []QueryCondition{
					{Column: "age", Operator: ">", Value: "25", Type: "where"},
				},
			},
			expected: "SELECT `name`, `email` FROM `users` WHERE `age` > ?",
		},
		{
			name: "查询所有列",
			parsed: &ParsedQuery{
				Intent:  IntentSelect,
				Tables:  []string{"users"},
				Columns: []string{"*"},
			},
			expected: "SELECT * FROM `users`",
		},
		{
			name: "带排序的查询",
			parsed: &ParsedQuery{
				Intent:  IntentSelect,
				Tables:  []string{"users"},
				Columns: []string{"name"},
				Operations: []QueryOperation{
					{
						Type: "order_by",
						Parameters: map[string]interface{}{
							"column":    "name",
							"direction": "ASC",
						},
					},
				},
			},
			expected: "SELECT `name` FROM `users` ORDER BY `name` ASC",
		},
		{
			name: "带限制的查询",
			parsed: &ParsedQuery{
				Intent:  IntentSelect,
				Tables:  []string{"users"},
				Columns: []string{"name"},
				Operations: []QueryOperation{
					{
						Type: "limit",
						Parameters: map[string]interface{}{
							"count": "10",
						},
					},
				},
			},
			expected: "SELECT `name` FROM `users` LIMIT 10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, params, warnings, err := generator.generateSelectSQL(ctx, tt.parsed)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, sql)
			assert.NotNil(t, params)
			assert.IsType(t, []string{}, warnings)
		})
	}
}

func TestGenerateCountSQL(t *testing.T) {
	mockSchemaManager := &MockSchemaManager{}
	mockLogger := &MockLogger{}
	generator := NewSQLGenerator(mockSchemaManager, mockLogger)

	ctx := context.Background()

	// 设置 mock 期望
	mockSchemaManager.On("GetRelationships", "users").Return([]*core.Relationship{}, nil)

	parsed := &ParsedQuery{
		Intent: IntentCount,
		Tables: []string{"users"},
		Conditions: []QueryCondition{
			{Column: "status", Operator: "=", Value: "active", Type: "where"},
		},
	}

	sql, params, warnings, err := generator.generateCountSQL(ctx, parsed)

	assert.NoError(t, err)
	assert.Equal(t, "SELECT COUNT(*) FROM `users` WHERE `status` = ?", sql)
	assert.Equal(t, []interface{}{"active"}, params)
	assert.IsType(t, []string{}, warnings)
}

func TestGenerateAggregateSQL(t *testing.T) {
	mockSchemaManager := &MockSchemaManager{}
	mockLogger := &MockLogger{}
	generator := NewSQLGenerator(mockSchemaManager, mockLogger)

	ctx := context.Background()

	// 设置 mock 期望
	mockSchemaManager.On("GetRelationships", "orders").Return([]*core.Relationship{}, nil)

	parsed := &ParsedQuery{
		Intent:  IntentAggregate,
		Tables:  []string{"orders"},
		Columns: []string{"sum_amount"},
		Operations: []QueryOperation{
			{
				Type: "group_by",
				Parameters: map[string]interface{}{
					"column": "customer_id",
				},
			},
		},
	}

	sql, params, warnings, err := generator.generateAggregateSQL(ctx, parsed)

	assert.NoError(t, err)
	assert.Contains(t, sql, "SELECT `customer_id`")
	assert.Contains(t, sql, "FROM `orders`")
	assert.Contains(t, sql, "GROUP BY `customer_id`")
	assert.NotNil(t, params)
	assert.IsType(t, []string{}, warnings)
}

func TestGenerateInsertSQL(t *testing.T) {
	mockSchemaManager := &MockSchemaManager{}
	mockLogger := &MockLogger{}
	generator := NewSQLGenerator(mockSchemaManager, mockLogger)

	ctx := context.Background()

	// 设置 mock 期望
	mockSchemaManager.On("GetTableInfo", "users").Return(&core.TableInfo{
		Name: "users",
		Columns: []*core.Column{
			{Name: "name", Type: "varchar"},
			{Name: "email", Type: "varchar"},
			{Name: "age", Type: "int"},
		},
	}, nil)

	parsed := &ParsedQuery{
		Intent: IntentInsert,
		Tables: []string{"users"},
		Parameters: map[string]interface{}{
			"values": map[string]interface{}{
				"name":  "John Doe",
				"email": "john@example.com",
				"age":   30,
			},
		},
	}

	sql, params, warnings, err := generator.generateInsertSQL(ctx, parsed)

	assert.NoError(t, err)
	assert.Contains(t, sql, "INSERT INTO `users`")
	assert.Contains(t, sql, "VALUES")
	assert.Len(t, params, 3)
	assert.IsType(t, []string{}, warnings)
}

func TestGenerateUpdateSQL(t *testing.T) {
	mockSchemaManager := &MockSchemaManager{}
	mockLogger := &MockLogger{}
	generator := NewSQLGenerator(mockSchemaManager, mockLogger)

	ctx := context.Background()

	// 设置 mock 期望
	mockSchemaManager.On("GetTableInfo", "users").Return(&core.TableInfo{
		Name: "users",
		Columns: []*core.Column{
			{Name: "name", Type: "varchar"},
			{Name: "email", Type: "varchar"},
			{Name: "age", Type: "int"},
		},
	}, nil)

	parsed := &ParsedQuery{
		Intent: IntentUpdate,
		Tables: []string{"users"},
		Conditions: []QueryCondition{
			{Column: "id", Operator: "=", Value: "1", Type: "where"},
		},
		Parameters: map[string]interface{}{
			"values": map[string]interface{}{
				"name": "Jane Doe",
				"age":  25,
			},
		},
	}

	sql, params, warnings, err := generator.generateUpdateSQL(ctx, parsed)

	assert.NoError(t, err)
	assert.Contains(t, sql, "UPDATE `users` SET")
	assert.Contains(t, sql, "WHERE `id` = ?")
	assert.Len(t, params, 3) // 2 for SET + 1 for WHERE
	assert.IsType(t, []string{}, warnings)
}

func TestGenerateDeleteSQL(t *testing.T) {
	mockSchemaManager := &MockSchemaManager{}
	mockLogger := &MockLogger{}
	generator := NewSQLGenerator(mockSchemaManager, mockLogger)

	ctx := context.Background()

	parsed := &ParsedQuery{
		Intent: IntentDelete,
		Tables: []string{"users"},
		Conditions: []QueryCondition{
			{Column: "id", Operator: "=", Value: "1", Type: "where"},
		},
	}

	sql, params, warnings, err := generator.generateDeleteSQL(ctx, parsed)

	assert.NoError(t, err)
	assert.Equal(t, "DELETE FROM `users` WHERE `id` = ?", sql)
	assert.Equal(t, []interface{}{"1"}, params)
	assert.IsType(t, []string{}, warnings)
}

func TestBuildWhereClause(t *testing.T) {
	mockSchemaManager := &MockSchemaManager{}
	mockLogger := &MockLogger{}
	generator := NewSQLGenerator(mockSchemaManager, mockLogger)

	tests := []struct {
		name       string
		conditions []QueryCondition
		expected   string
		paramCount int
	}{
		{
			name:       "空条件",
			conditions: []QueryCondition{},
			expected:   "",
			paramCount: 0,
		},
		{
			name: "单个等于条件",
			conditions: []QueryCondition{
				{Column: "name", Operator: "=", Value: "John", Type: "where"},
			},
			expected:   "WHERE `name` = ?",
			paramCount: 1,
		},
		{
			name: "多个条件",
			conditions: []QueryCondition{
				{Column: "age", Operator: ">", Value: "25", Type: "where"},
				{Column: "status", Operator: "=", Value: "active", Type: "where"},
			},
			expected:   "WHERE `age` > ? AND `status` = ?",
			paramCount: 2,
		},
		{
			name: "LIKE 条件",
			conditions: []QueryCondition{
				{Column: "name", Operator: "LIKE", Value: "John", Type: "where"},
			},
			expected:   "WHERE `name` LIKE ?",
			paramCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			whereClause, params, warnings := generator.buildWhereClause(tt.conditions)
			assert.Equal(t, tt.expected, whereClause)
			assert.Len(t, params, tt.paramCount)
			assert.IsType(t, []string{}, warnings)
		})
	}
}

func TestEscapeIdentifier(t *testing.T) {
	mockSchemaManager := &MockSchemaManager{}
	mockLogger := &MockLogger{}
	generator := NewSQLGenerator(mockSchemaManager, mockLogger)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "普通标识符",
			input:    "users",
			expected: "`users`",
		},
		{
			name:     "包含危险字符",
			input:    "user`s",
			expected: "`users`",
		},
		{
			name:     "包含分号",
			input:    "users;DROP TABLE",
			expected: "`usersDROP TABLE`",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generator.escapeIdentifier(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidateSQL(t *testing.T) {
	mockSchemaManager := &MockSchemaManager{}
	mockLogger := &MockLogger{}
	generator := NewSQLGenerator(mockSchemaManager, mockLogger)

	tests := []struct {
		name      string
		sql       string
		expectErr bool
	}{
		{
			name:      "安全的 SELECT 语句",
			sql:       "SELECT name FROM users WHERE age > 25",
			expectErr: false,
		},
		{
			name:      "包含 DROP 关键词",
			sql:       "SELECT name FROM users; DROP TABLE users;",
			expectErr: true,
		},
		{
			name:      "包含注释",
			sql:       "SELECT name FROM users -- comment",
			expectErr: true,
		},
		{
			name:      "可能的 OR 注入",
			sql:       "SELECT name FROM users WHERE id = 1 OR 1=1",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := generator.validateSQL(tt.sql)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGenerateSQL(t *testing.T) {
	mockSchemaManager := &MockSchemaManager{}
	mockLogger := &MockLogger{}
	generator := NewSQLGenerator(mockSchemaManager, mockLogger)

	ctx := context.Background()

	// 设置 mock 期望
	mockLogger.On("Debug", mock.Anything, mock.Anything, mock.Anything).Return().Maybe()
	mockSchemaManager.On("GetRelationships", mock.Anything).Return([]*core.Relationship{}, nil).Maybe()

	parsed := &ParsedQuery{
		Intent:  IntentSelect,
		Tables:  []string{"users"},
		Columns: []string{"name", "email"},
		Conditions: []QueryCondition{
			{Column: "age", Operator: ">", Value: "25", Type: "where"},
		},
		Complexity: "simple",
		Confidence: 0.9,
	}

	result, err := generator.GenerateSQL(ctx, parsed)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.SQL)
	assert.NotNil(t, result.Parameters)
	assert.NotEmpty(t, result.Explanation)
	assert.IsType(t, []string{}, result.Warnings)
	assert.NotNil(t, result.Metadata)

	// 验证元数据
	assert.Equal(t, "select", result.Metadata["intent"])
	assert.Equal(t, []string{"users"}, result.Metadata["tables"])
	assert.Equal(t, []string{"name", "email"}, result.Metadata["columns"])
	assert.Equal(t, "simple", result.Metadata["complexity"])
	assert.Equal(t, 0.9, result.Metadata["confidence"])

	mockSchemaManager.AssertExpectations(t)
}

func TestOptimizeSQL(t *testing.T) {
	mockSchemaManager := &MockSchemaManager{}
	mockLogger := &MockLogger{}
	generator := NewSQLGenerator(mockSchemaManager, mockLogger)

	ctx := context.Background()

	// 设置 mock 期望
	mockSchemaManager.On("GetTableInfo", "users").Return(&core.TableInfo{
		Name: "users",
		Columns: []*core.Column{
			{Name: "id", Type: "int"},
			{Name: "name", Type: "varchar"},
		},
		Indexes: []*core.Index{
			{Name: "idx_id", Columns: []string{"id"}},
		},
	}, nil)

	tests := []struct {
		name              string
		sql               string
		tables            []string
		expectSuggestions bool
	}{
		{
			name:              "使用索引字段的查询",
			sql:               "SELECT name FROM users WHERE id = 1",
			tables:            []string{"users"},
			expectSuggestions: false,
		},
		{
			name:              "未使用索引字段的查询",
			sql:               "SELECT name FROM users WHERE name = 'John'",
			tables:            []string{"users"},
			expectSuggestions: true,
		},
		{
			name:              "使用 SELECT * 的查询",
			sql:               "SELECT * FROM users",
			tables:            []string{"users"},
			expectSuggestions: true,
		},
		{
			name:              "没有 LIMIT 的查询",
			sql:               "SELECT name FROM users WHERE id > 100",
			tables:            []string{"users"},
			expectSuggestions: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			optimizedSQL, suggestions, err := generator.OptimizeSQL(ctx, tt.sql, tt.tables)
			assert.NoError(t, err)
			assert.NotEmpty(t, optimizedSQL)

			if tt.expectSuggestions {
				assert.NotEmpty(t, suggestions)
			}
		})
	}
}

func TestFindJoinCondition(t *testing.T) {
	mockSchemaManager := &MockSchemaManager{}
	mockLogger := &MockLogger{}
	generator := NewSQLGenerator(mockSchemaManager, mockLogger)

	ctx := context.Background()

	// 设置 mock 期望 - 直接关系
	mockSchemaManager.On("GetRelationships", "users").Return([]*core.Relationship{
		{
			FromTable:  "users",
			FromColumn: "id",
			ToTable:    "orders",
			ToColumn:   "user_id",
		},
	}, nil)

	condition, err := generator.findJoinCondition(ctx, "users", "orders")

	assert.NoError(t, err)
	assert.Equal(t, "`users`.`id` = `orders`.`user_id`", condition)

	mockSchemaManager.AssertExpectations(t)
}

func TestColumnExists(t *testing.T) {
	mockSchemaManager := &MockSchemaManager{}
	mockLogger := &MockLogger{}
	generator := NewSQLGenerator(mockSchemaManager, mockLogger)

	tableInfo := &core.TableInfo{
		Columns: []*core.Column{
			{Name: "id", Type: "int"},
			{Name: "name", Type: "varchar"},
			{Name: "email", Type: "varchar"},
		},
	}

	tests := []struct {
		name       string
		columnName string
		expected   bool
	}{
		{
			name:       "存在的列",
			columnName: "name",
			expected:   true,
		},
		{
			name:       "不存在的列",
			columnName: "age",
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generator.columnExists(tableInfo, tt.columnName)
			assert.Equal(t, tt.expected, result)
		})
	}
}
