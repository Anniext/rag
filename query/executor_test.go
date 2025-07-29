package query

import (
	"context"
	"testing"
	"time"

	"pumppill/rag/core"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockSecurityManager 模拟 SecurityManager
type MockSecurityManager struct {
	mock.Mock
}

func (m *MockSecurityManager) ValidateToken(ctx context.Context, token string) (*core.UserInfo, error) {
	args := m.Called(ctx, token)
	return args.Get(0).(*core.UserInfo), args.Error(1)
}

func (m *MockSecurityManager) CheckPermission(ctx context.Context, user *core.UserInfo, resource string, action string) error {
	args := m.Called(ctx, user, resource, action)
	return args.Error(0)
}

func (m *MockSecurityManager) ValidateSQL(ctx context.Context, sql string) error {
	args := m.Called(ctx, sql)
	return args.Error(0)
}

func TestNewQueryExecutor(t *testing.T) {
	db, _, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	mockSecurityManager := &MockSecurityManager{}
	mockLogger := &MockLogger{}

	executor := NewQueryExecutor(db, mockSecurityManager, mockLogger, nil)

	assert.NotNil(t, executor)
	assert.Equal(t, db, executor.db)
	assert.Equal(t, mockSecurityManager, executor.securityManager)
	assert.Equal(t, mockLogger, executor.logger)
	assert.NotNil(t, executor.config)
	assert.Equal(t, 30*time.Second, executor.config.QueryTimeout)
	assert.Equal(t, 10000, executor.config.MaxRows)
	assert.True(t, executor.config.EnablePagination)
}

func TestNewQueryExecutorWithConfig(t *testing.T) {
	db, _, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	mockSecurityManager := &MockSecurityManager{}
	mockLogger := &MockLogger{}

	config := &ExecutorConfig{
		QueryTimeout:     60 * time.Second,
		MaxRows:          5000,
		EnablePagination: false,
		DefaultPageSize:  50,
		MaxPageSize:      500,
	}

	executor := NewQueryExecutor(db, mockSecurityManager, mockLogger, config)

	assert.NotNil(t, executor)
	assert.Equal(t, config, executor.config)
}

func TestExecuteSelectQuery(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	mockSecurityManager := &MockSecurityManager{}
	mockLogger := &MockLogger{}
	executor := NewQueryExecutor(db, mockSecurityManager, mockLogger, nil)

	ctx := context.Background()

	// 设置 mock 期望
	rows := sqlmock.NewRows([]string{"id", "name", "email"}).
		AddRow(1, "John Doe", "john@example.com").
		AddRow(2, "Jane Smith", "jane@example.com")

	mock.ExpectQuery("SELECT (.+) FROM users").WillReturnRows(rows)

	sql := "SELECT id, name, email FROM users"
	params := []interface{}{}

	result, err := executor.executeSelectQuery(ctx, sql, params)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 2, result.RowCount)
	assert.Len(t, result.Data, 2)
	assert.Equal(t, "John Doe", result.Data[0]["name"])
	assert.Equal(t, "jane@example.com", result.Data[1]["email"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestExecuteWriteQuery(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	mockSecurityManager := &MockSecurityManager{}
	mockLogger := &MockLogger{}
	executor := NewQueryExecutor(db, mockSecurityManager, mockLogger, nil)

	ctx := context.Background()

	// 设置 mock 期望
	mock.ExpectExec("INSERT INTO users").
		WithArgs("John Doe", "john@example.com").
		WillReturnResult(sqlmock.NewResult(1, 1))

	sql := "INSERT INTO users (name, email) VALUES (?, ?)"
	params := []interface{}{"John Doe", "john@example.com"}

	result, err := executor.executeWriteQuery(ctx, sql, params)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, int64(1), result.AffectedRows)
	assert.Equal(t, 0, result.RowCount)
	assert.Empty(t, result.Data)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCheckPermissions(t *testing.T) {
	db, _, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	mockSecurityManager := &MockSecurityManager{}
	mockLogger := &MockLogger{}
	executor := NewQueryExecutor(db, mockSecurityManager, mockLogger, nil)

	ctx := context.Background()
	userInfo := &core.UserInfo{
		ID:       "user1",
		Username: "testuser",
		Roles:    []string{"user"},
	}

	tests := []struct {
		name      string
		sql       string
		setupMock func()
		expectErr bool
	}{
		{
			name: "有效的 SELECT 查询",
			sql:  "SELECT * FROM users",
			setupMock: func() {
				mockSecurityManager.On("ValidateSQL", ctx, "SELECT * FROM users").Return(nil)
				mockSecurityManager.On("CheckPermission", ctx, userInfo, "users", "read").Return(nil)
			},
			expectErr: false,
		},
		{
			name: "SQL 验证失败",
			sql:  "SELECT * FROM users; DROP TABLE users;",
			setupMock: func() {
				mockSecurityManager.On("ValidateSQL", ctx, "SELECT * FROM users; DROP TABLE users;").Return(assert.AnError)
			},
			expectErr: true,
		},
		{
			name: "没有读取权限",
			sql:  "SELECT * FROM users",
			setupMock: func() {
				mockSecurityManager.On("ValidateSQL", ctx, "SELECT * FROM users").Return(nil)
				mockSecurityManager.On("CheckPermission", ctx, userInfo, "users", "read").Return(assert.AnError)
			},
			expectErr: true,
		},
		{
			name: "写操作需要写权限",
			sql:  "INSERT INTO users (name) VALUES ('test')",
			setupMock: func() {
				mockSecurityManager.On("ValidateSQL", ctx, "INSERT INTO users (name) VALUES ('test')").Return(nil)
				mockSecurityManager.On("CheckPermission", ctx, userInfo, "users", "read").Return(nil)
				mockSecurityManager.On("CheckPermission", ctx, userInfo, "users", "write").Return(nil)
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 重置 mock
			mockSecurityManager.ExpectedCalls = nil
			tt.setupMock()

			err := executor.checkPermissions(ctx, tt.sql, userInfo)

			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			mockSecurityManager.AssertExpectations(t)
		})
	}
}

func TestExtractTablesFromSQL(t *testing.T) {
	db, _, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	mockSecurityManager := &MockSecurityManager{}
	mockLogger := &MockLogger{}
	executor := NewQueryExecutor(db, mockSecurityManager, mockLogger, nil)

	tests := []struct {
		name     string
		sql      string
		expected []string
	}{
		{
			name:     "简单 SELECT",
			sql:      "SELECT * FROM users",
			expected: []string{"users"},
		},
		{
			name:     "JOIN 查询",
			sql:      "SELECT * FROM users JOIN orders ON users.id = orders.user_id",
			expected: []string{"users", "orders"},
		},
		{
			name:     "INSERT 语句",
			sql:      "INSERT INTO users (name) VALUES ('test')",
			expected: []string{"users"},
		},
		{
			name:     "UPDATE 语句",
			sql:      "UPDATE users SET name = 'test' WHERE id = 1",
			expected: []string{"users"},
		},
		{
			name:     "DELETE 语句",
			sql:      "DELETE FROM users WHERE id = 1",
			expected: []string{"users"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := executor.extractTablesFromSQL(tt.sql)
			assert.ElementsMatch(t, tt.expected, result)
		})
	}
}

func TestIsWriteOperation(t *testing.T) {
	db, _, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	mockSecurityManager := &MockSecurityManager{}
	mockLogger := &MockLogger{}
	executor := NewQueryExecutor(db, mockSecurityManager, mockLogger, nil)

	tests := []struct {
		name     string
		sql      string
		expected bool
	}{
		{
			name:     "SELECT 查询",
			sql:      "SELECT * FROM users",
			expected: false,
		},
		{
			name:     "INSERT 语句",
			sql:      "INSERT INTO users (name) VALUES ('test')",
			expected: true,
		},
		{
			name:     "UPDATE 语句",
			sql:      "UPDATE users SET name = 'test'",
			expected: true,
		},
		{
			name:     "DELETE 语句",
			sql:      "DELETE FROM users WHERE id = 1",
			expected: true,
		},
		{
			name:     "CREATE 语句",
			sql:      "CREATE TABLE test (id INT)",
			expected: true,
		},
		{
			name:     "DROP 语句",
			sql:      "DROP TABLE test",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := executor.isWriteOperation(tt.sql)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHandlePagination(t *testing.T) {
	db, _, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	mockSecurityManager := &MockSecurityManager{}
	mockLogger := &MockLogger{}
	executor := NewQueryExecutor(db, mockSecurityManager, mockLogger, nil)

	tests := []struct {
		name         string
		sql          string
		params       []interface{}
		options      *core.QueryOptions
		expectedSQL  string
		expectedPage int
	}{
		{
			name:        "无分页选项",
			sql:         "SELECT * FROM users",
			params:      []interface{}{},
			options:     nil,
			expectedSQL: "SELECT * FROM users",
		},
		{
			name:         "使用 Limit",
			sql:          "SELECT * FROM users",
			params:       []interface{}{},
			options:      &core.QueryOptions{Limit: 10},
			expectedSQL:  "SELECT * FROM users LIMIT 10 OFFSET 0",
			expectedPage: 1,
		},
		{
			name:         "使用 Limit 和 Offset",
			sql:          "SELECT * FROM users",
			params:       []interface{}{},
			options:      &core.QueryOptions{Limit: 10, Offset: 20},
			expectedSQL:  "SELECT * FROM users LIMIT 10 OFFSET 20",
			expectedPage: 3,
		},
		{
			name:        "已有 LIMIT 子句",
			sql:         "SELECT * FROM users LIMIT 5",
			params:      []interface{}{},
			options:     &core.QueryOptions{Limit: 10},
			expectedSQL: "SELECT * FROM users LIMIT 5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resultSQL, resultParams, pagination, err := executor.handlePagination(tt.sql, tt.params, tt.options)

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedSQL, resultSQL)
			assert.Equal(t, tt.params, resultParams)

			if tt.expectedPage > 0 {
				assert.NotNil(t, pagination)
				assert.Equal(t, tt.expectedPage, pagination.Page)
			}
		})
	}
}

func TestConvertValue(t *testing.T) {
	db, _, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	mockSecurityManager := &MockSecurityManager{}
	mockLogger := &MockLogger{}
	executor := NewQueryExecutor(db, mockSecurityManager, mockLogger, nil)

	tests := []struct {
		name     string
		value    interface{}
		expected interface{}
	}{
		{
			name:     "nil 值",
			value:    nil,
			expected: nil,
		},
		{
			name:     "字节数组",
			value:    []byte("test"),
			expected: "test",
		},
		{
			name:     "时间值",
			value:    time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
			expected: "2023-01-01 12:00:00",
		},
		{
			name:     "字符串值",
			value:    "test",
			expected: "test",
		},
		{
			name:     "整数值",
			value:    123,
			expected: 123,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := executor.convertValue(tt.value, nil)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatAsCSV(t *testing.T) {
	db, _, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	mockSecurityManager := &MockSecurityManager{}
	mockLogger := &MockLogger{}
	executor := NewQueryExecutor(db, mockSecurityManager, mockLogger, nil)

	result := &ExecutionResult{
		Data: []map[string]interface{}{
			{"id": 1, "name": "John Doe", "email": "john@example.com"},
			{"id": 2, "name": "Jane Smith", "email": "jane@example.com"},
		},
		RowCount: 2,
	}

	csv, err := executor.formatAsCSV(result)

	assert.NoError(t, err)
	// CSV 列顺序可能不同，只检查包含关键内容
	assert.Contains(t, csv, "id")
	assert.Contains(t, csv, "name")
	assert.Contains(t, csv, "email")
	assert.Contains(t, csv, "John Doe")
	assert.Contains(t, csv, "jane@example.com")
}

func TestFormatAsTable(t *testing.T) {
	db, _, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	mockSecurityManager := &MockSecurityManager{}
	mockLogger := &MockLogger{}
	executor := NewQueryExecutor(db, mockSecurityManager, mockLogger, nil)

	result := &ExecutionResult{
		Data: []map[string]interface{}{
			{"id": 1, "name": "John"},
			{"id": 2, "name": "Jane"},
		},
		RowCount: 2,
	}

	table, err := executor.formatAsTable(result)

	assert.NoError(t, err)
	assert.Contains(t, table, "|")
	assert.Contains(t, table, "id")
	assert.Contains(t, table, "name")
	assert.Contains(t, table, "John")
	assert.Contains(t, table, "Jane")
}

func TestGetExecutionStats(t *testing.T) {
	db, _, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	mockSecurityManager := &MockSecurityManager{}
	mockLogger := &MockLogger{}
	executor := NewQueryExecutor(db, mockSecurityManager, mockLogger, nil)

	ctx := context.Background()
	stats, err := executor.GetExecutionStats(ctx)

	assert.NoError(t, err)
	assert.NotNil(t, stats)
	assert.Contains(t, stats, "db_stats")
	assert.Contains(t, stats, "config")

	dbStats := stats["db_stats"].(map[string]interface{})
	assert.Contains(t, dbStats, "open_connections")
	assert.Contains(t, dbStats, "in_use")

	config := stats["config"].(map[string]interface{})
	assert.Contains(t, config, "query_timeout")
	assert.Contains(t, config, "max_rows")
}

func TestExecuteQuery(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	mockSecurityManager := &MockSecurityManager{}
	mockLogger := &MockLogger{}
	executor := NewQueryExecutor(db, mockSecurityManager, mockLogger, nil)

	ctx := context.Background()
	userInfo := &core.UserInfo{
		ID:       "user1",
		Username: "testuser",
	}

	generatedSQL := &GeneratedSQL{
		SQL:        "SELECT id, name FROM users WHERE id = ?",
		Parameters: []interface{}{1},
		Warnings:   []string{},
	}

	options := &core.QueryOptions{
		Limit: 10,
	}

	// 设置 mock 期望
	mockSecurityManager.On("ValidateSQL", ctx, generatedSQL.SQL).Return(nil)
	mockSecurityManager.On("CheckPermission", ctx, userInfo, "users", "read").Return(nil)

	// Mock 总行数查询
	countRows := sqlmock.NewRows([]string{"count"}).AddRow(5)
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM").WillReturnRows(countRows)

	// Mock 主查询
	rows := sqlmock.NewRows([]string{"id", "name"}).
		AddRow(1, "John Doe")
	mock.ExpectQuery("SELECT id, name FROM users").WillReturnRows(rows)

	result, err := executor.ExecuteQuery(ctx, generatedSQL, userInfo, options)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 1, result.RowCount)
	assert.Len(t, result.Data, 1)
	assert.Equal(t, "John Doe", result.Data[0]["name"])
	assert.NotNil(t, result.Pagination)
	assert.Greater(t, result.ExecutionTime, time.Duration(0))

	assert.NoError(t, mock.ExpectationsWereMet())
	mockSecurityManager.AssertExpectations(t)
}
