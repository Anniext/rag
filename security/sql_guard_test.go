package security

import (
	"context"
	"strings"
	"testing"

	"pumppill/rag/core"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestSQLGuard_ValidateSQL(t *testing.T) {
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsCollector{}

	// 设置 mock 期望
	mockLogger.On("Warn", mock.AnythingOfType("string"), mock.Anything).Return()
	mockMetrics.On("RecordHistogram", mock.AnythingOfType("string"), mock.AnythingOfType("float64"), mock.AnythingOfType("map[string]string")).Return()
	mockMetrics.On("IncrementCounter", mock.AnythingOfType("string"), mock.AnythingOfType("map[string]string")).Return()

	sqlGuard := NewSQLGuard(mockLogger, mockMetrics)

	t.Run("ValidateSQL_ValidSelect", func(t *testing.T) {
		sql := "SELECT id, name FROM users WHERE status = 'active'"
		err := sqlGuard.ValidateSQL(context.Background(), sql)
		assert.NoError(t, err)
	})

	t.Run("ValidateSQL_EmptySQL", func(t *testing.T) {
		err := sqlGuard.ValidateSQL(context.Background(), "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "SQL 语句不能为空")
	})

	t.Run("ValidateSQL_TooLong", func(t *testing.T) {
		longSQL := "SELECT * FROM users WHERE " + string(make([]byte, 10001))
		err := sqlGuard.ValidateSQL(context.Background(), longSQL)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "长度超过限制")
	})

	t.Run("ValidateSQL_SQLInjection_Union", func(t *testing.T) {
		sql := "SELECT * FROM users WHERE id = 1 UNION SELECT password FROM admin"
		err := sqlGuard.ValidateSQL(context.Background(), sql)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "危险模式")
	})

	t.Run("ValidateSQL_SQLInjection_Or1Equals1", func(t *testing.T) {
		sql := "SELECT * FROM users WHERE id = 1 OR 1=1"
		err := sqlGuard.ValidateSQL(context.Background(), sql)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "危险模式")
	})

	t.Run("ValidateSQL_DropTable", func(t *testing.T) {
		sql := "DROP TABLE users"
		err := sqlGuard.ValidateSQL(context.Background(), sql)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "危险模式")
	})

	t.Run("ValidateSQL_Insert", func(t *testing.T) {
		sql := "INSERT INTO users (name) VALUES ('test')"
		err := sqlGuard.ValidateSQL(context.Background(), sql)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "危险模式")
	})

	t.Run("ValidateSQL_Update", func(t *testing.T) {
		sql := "UPDATE users SET name = 'test' WHERE id = 1"
		err := sqlGuard.ValidateSQL(context.Background(), sql)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "危险模式")
	})

	t.Run("ValidateSQL_Delete", func(t *testing.T) {
		sql := "DELETE FROM users WHERE id = 1"
		err := sqlGuard.ValidateSQL(context.Background(), sql)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "危险模式")
	})

	t.Run("ValidateSQL_Comment", func(t *testing.T) {
		sql := "SELECT * FROM users -- WHERE id = 1"
		err := sqlGuard.ValidateSQL(context.Background(), sql)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "危险模式")
	})

	t.Run("ValidateSQL_MultilineComment", func(t *testing.T) {
		sql := "SELECT * FROM users /* comment */ WHERE id = 1"
		err := sqlGuard.ValidateSQL(context.Background(), sql)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "危险模式")
	})
}

func TestSQLGuard_CheckQueryPermission(t *testing.T) {
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsCollector{}

	// 设置 mock 期望
	mockLogger.On("Warn", mock.AnythingOfType("string"), mock.Anything).Return()

	sqlGuard := NewSQLGuard(mockLogger, mockMetrics)

	t.Run("CheckQueryPermission_NilUser", func(t *testing.T) {
		sql := "SELECT * FROM users"
		tables := []string{"users"}
		err := sqlGuard.CheckQueryPermission(context.Background(), nil, sql, tables)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "用户信息不能为空")
	})

	t.Run("CheckQueryPermission_HasPermission", func(t *testing.T) {
		user := &core.UserInfo{
			ID:          "123",
			Username:    "testuser",
			Permissions: []string{"table:users:select"},
		}
		sql := "SELECT * FROM users"
		tables := []string{"users"}
		err := sqlGuard.CheckQueryPermission(context.Background(), user, sql, tables)
		assert.NoError(t, err)
	})

	t.Run("CheckQueryPermission_WildcardPermission", func(t *testing.T) {
		user := &core.UserInfo{
			ID:          "123",
			Username:    "testuser",
			Permissions: []string{"*"},
		}
		sql := "SELECT * FROM users"
		tables := []string{"users"}
		err := sqlGuard.CheckQueryPermission(context.Background(), user, sql, tables)
		assert.NoError(t, err)
	})

	t.Run("CheckQueryPermission_TableWildcard", func(t *testing.T) {
		user := &core.UserInfo{
			ID:          "123",
			Username:    "testuser",
			Permissions: []string{"table:users:*"},
		}
		sql := "SELECT * FROM users"
		tables := []string{"users"}
		err := sqlGuard.CheckQueryPermission(context.Background(), user, sql, tables)
		assert.NoError(t, err)
	})

	t.Run("CheckQueryPermission_NoPermission", func(t *testing.T) {
		user := &core.UserInfo{
			ID:          "123",
			Username:    "testuser",
			Permissions: []string{"table:other:select"},
		}
		sql := "SELECT * FROM users"
		tables := []string{"users"}
		err := sqlGuard.CheckQueryPermission(context.Background(), user, sql, tables)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "没有权限")
	})
}

func TestSQLGuard_ExtractSQLOperation(t *testing.T) {
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsCollector{}

	sqlGuard := NewSQLGuard(mockLogger, mockMetrics)

	tests := []struct {
		name     string
		sql      string
		expected string
	}{
		{
			name:     "Select",
			sql:      "SELECT * FROM users",
			expected: "SELECT",
		},
		{
			name:     "Insert",
			sql:      "INSERT INTO users (name) VALUES ('test')",
			expected: "INSERT",
		},
		{
			name:     "Update",
			sql:      "UPDATE users SET name = 'test'",
			expected: "UPDATE",
		},
		{
			name:     "Delete",
			sql:      "DELETE FROM users",
			expected: "DELETE",
		},
		{
			name:     "Show",
			sql:      "SHOW TABLES",
			expected: "SHOW",
		},
		{
			name:     "Describe",
			sql:      "DESCRIBE users",
			expected: "DESCRIBE",
		},
		{
			name:     "WithWhitespace",
			sql:      "   SELECT * FROM users   ",
			expected: "SELECT",
		},
		{
			name:     "WithComment",
			sql:      "-- comment\nSELECT * FROM users",
			expected: "SELECT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sqlGuard.extractSQLOperation(tt.sql)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSQLGuard_CheckParenthesesBalance(t *testing.T) {
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsCollector{}

	sqlGuard := NewSQLGuard(mockLogger, mockMetrics)

	tests := []struct {
		name      string
		sql       string
		shouldErr bool
	}{
		{
			name:      "Balanced",
			sql:       "SELECT * FROM users WHERE (id = 1 AND (status = 'active'))",
			shouldErr: false,
		},
		{
			name:      "Unbalanced_Missing_Close",
			sql:       "SELECT * FROM users WHERE (id = 1",
			shouldErr: true,
		},
		{
			name:      "Unbalanced_Extra_Close",
			sql:       "SELECT * FROM users WHERE id = 1)",
			shouldErr: true,
		},
		{
			name:      "No_Parentheses",
			sql:       "SELECT * FROM users WHERE id = 1",
			shouldErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sqlGuard.checkParenthesesBalance(tt.sql)
			if tt.shouldErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSQLGuard_CheckQuotesBalance(t *testing.T) {
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsCollector{}

	sqlGuard := NewSQLGuard(mockLogger, mockMetrics)

	tests := []struct {
		name      string
		sql       string
		shouldErr bool
	}{
		{
			name:      "Balanced_Single",
			sql:       "SELECT * FROM users WHERE name = 'test'",
			shouldErr: false,
		},
		{
			name:      "Balanced_Double",
			sql:       `SELECT * FROM users WHERE name = "test"`,
			shouldErr: false,
		},
		{
			name:      "Balanced_Back",
			sql:       "SELECT * FROM `users` WHERE name = 'test'",
			shouldErr: false,
		},
		{
			name:      "Unbalanced_Single",
			sql:       "SELECT * FROM users WHERE name = 'test",
			shouldErr: true,
		},
		{
			name:      "Unbalanced_Double",
			sql:       `SELECT * FROM users WHERE name = "test`,
			shouldErr: true,
		},
		{
			name:      "Unbalanced_Back",
			sql:       "SELECT * FROM `users WHERE name = 'test'",
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sqlGuard.checkQuotesBalance(tt.sql)
			if tt.shouldErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSQLGuard_SanitizeSQL(t *testing.T) {
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsCollector{}

	sqlGuard := NewSQLGuard(mockLogger, mockMetrics)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Remove_Extra_Spaces",
			input:    "SELECT   *    FROM    users",
			expected: "SELECT * FROM users",
		},
		{
			name:     "Remove_Comments",
			input:    "SELECT * FROM users -- comment",
			expected: "SELECT * FROM users",
		},
		{
			name:     "Remove_Multiline_Comments",
			input:    "SELECT * FROM users /* comment */",
			expected: "SELECT * FROM users",
		},
		{
			name:     "Trim_Whitespace",
			input:    "   SELECT * FROM users   ",
			expected: "SELECT * FROM users",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sqlGuard.SanitizeSQL(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSQLGuard_Configuration(t *testing.T) {
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsCollector{}

	sqlGuard := NewSQLGuard(mockLogger, mockMetrics)

	t.Run("SetMaxQueryLength", func(t *testing.T) {
		sqlGuard.SetMaxQueryLength(5000)
		assert.Equal(t, 5000, sqlGuard.maxQueryLength)
	})

	t.Run("SetStrictMode", func(t *testing.T) {
		sqlGuard.SetStrictMode(false)
		assert.False(t, sqlGuard.enableStrictMode)
	})

	t.Run("AddAllowedOperation", func(t *testing.T) {
		originalCount := len(sqlGuard.allowedOperations)
		sqlGuard.AddAllowedOperation("INSERT")
		assert.Equal(t, originalCount+1, len(sqlGuard.allowedOperations))
		assert.Contains(t, sqlGuard.allowedOperations, "INSERT")
	})

	t.Run("RemoveAllowedOperation", func(t *testing.T) {
		sqlGuard.AddAllowedOperation("DELETE")
		originalCount := len(sqlGuard.allowedOperations)
		sqlGuard.RemoveAllowedOperation("DELETE")
		assert.Equal(t, originalCount-1, len(sqlGuard.allowedOperations))
		assert.NotContains(t, sqlGuard.allowedOperations, "DELETE")
	})
}
func TestSQLGuard_ValidateTables(t *testing.T) {
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsCollector{}

	// 设置 mock 期望
	mockLogger.On("Warn", mock.AnythingOfType("string"), mock.Anything).Return()

	sqlGuard := NewSQLGuard(mockLogger, mockMetrics)

	t.Run("ValidateTables_AllowedTable", func(t *testing.T) {
		sqlGuard.AddAllowedTable("users")
		tables := []string{"users"}
		err := sqlGuard.ValidateTables(context.Background(), tables)
		assert.NoError(t, err)
	})

	t.Run("ValidateTables_BlockedTable", func(t *testing.T) {
		tables := []string{"mysql.user"}
		err := sqlGuard.ValidateTables(context.Background(), tables)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "禁止访问表")
	})

	t.Run("ValidateTables_UnauthorizedTable", func(t *testing.T) {
		sqlGuard.AddAllowedTable("users")
		tables := []string{"admin_users"}
		err := sqlGuard.ValidateTables(context.Background(), tables)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "未授权访问表")
	})
}

func TestSQLGuard_CheckQueryComplexity(t *testing.T) {
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsCollector{}

	sqlGuard := NewSQLGuard(mockLogger, mockMetrics)

	t.Run("CheckQueryComplexity_TooManyJoins", func(t *testing.T) {
		sql := `SELECT * FROM users u1 
				JOIN profiles p1 ON u1.id = p1.user_id 
				JOIN addresses a1 ON u1.id = a1.user_id 
				JOIN orders o1 ON u1.id = o1.user_id 
				JOIN products pr1 ON o1.product_id = pr1.id 
				JOIN categories c1 ON pr1.category_id = c1.id 
				JOIN suppliers s1 ON pr1.supplier_id = s1.id`

		err := sqlGuard.CheckQueryComplexity(context.Background(), sql)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "过多的JOIN操作")
	})

	t.Run("CheckQueryComplexity_AcceptableJoins", func(t *testing.T) {
		sql := `SELECT * FROM users u 
				JOIN profiles p ON u.id = p.user_id 
				JOIN addresses a ON u.id = a.user_id`

		err := sqlGuard.CheckQueryComplexity(context.Background(), sql)
		assert.NoError(t, err)
	})

	t.Run("CheckQueryComplexity_TooDeepSubquery", func(t *testing.T) {
		sql := `SELECT * FROM users WHERE id IN (
					SELECT user_id FROM orders WHERE product_id IN (
						SELECT id FROM products WHERE category_id IN (
							SELECT id FROM categories WHERE parent_id IN (
								SELECT id FROM categories WHERE name = 'root'
							)
						)
					)
				)`

		err := sqlGuard.CheckQueryComplexity(context.Background(), sql)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "子查询嵌套过深")
	})

	t.Run("CheckQueryComplexity_AcceptableSubquery", func(t *testing.T) {
		sql := `SELECT * FROM users WHERE id IN (
					SELECT user_id FROM orders WHERE status = 'active'
				)`

		err := sqlGuard.CheckQueryComplexity(context.Background(), sql)
		assert.NoError(t, err)
	})
}

func TestSQLGuard_ValidateParameterizedQuery(t *testing.T) {
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsCollector{}

	sqlGuard := NewSQLGuard(mockLogger, mockMetrics)

	t.Run("ValidateParameterizedQuery_Success", func(t *testing.T) {
		sql := "SELECT * FROM users WHERE id = ? AND name = ?"
		params := []any{123, "testuser"}
		err := sqlGuard.ValidateParameterizedQuery(context.Background(), sql, params)
		assert.NoError(t, err)
	})

	t.Run("ValidateParameterizedQuery_ParameterCountMismatch", func(t *testing.T) {
		sql := "SELECT * FROM users WHERE id = ? AND name = ?"
		params := []any{123}
		err := sqlGuard.ValidateParameterizedQuery(context.Background(), sql, params)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "参数数量不匹配")
	})

	t.Run("ValidateParameterizedQuery_DangerousParameter", func(t *testing.T) {
		sql := "SELECT * FROM users WHERE name = ?"
		params := []any{"test' OR 1=1 --"}
		err := sqlGuard.ValidateParameterizedQuery(context.Background(), sql, params)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "参数包含危险模式")
	})

	t.Run("ValidateParameterizedQuery_SafeTypes", func(t *testing.T) {
		sql := "SELECT * FROM users WHERE id = ? AND age = ? AND active = ? AND score = ?"
		params := []any{123, 25, true, 95.5}
		err := sqlGuard.ValidateParameterizedQuery(context.Background(), sql, params)
		assert.NoError(t, err)
	})
}

func TestSQLGuard_ExtractTables(t *testing.T) {
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsCollector{}

	sqlGuard := NewSQLGuard(mockLogger, mockMetrics)

	tests := []struct {
		name     string
		sql      string
		expected []string
	}{
		{
			name:     "Simple SELECT",
			sql:      "SELECT * FROM users",
			expected: []string{"users"},
		},
		{
			name:     "SELECT with JOIN",
			sql:      "SELECT * FROM users u JOIN profiles p ON u.id = p.user_id",
			expected: []string{"users", "profiles"},
		},
		{
			name:     "Multiple JOINs",
			sql:      "SELECT * FROM users u LEFT JOIN profiles p ON u.id = p.user_id RIGHT JOIN addresses a ON u.id = a.user_id",
			expected: []string{"users", "profiles", "addresses"},
		},
		{
			name:     "Schema qualified table",
			sql:      "SELECT * FROM mydb.users",
			expected: []string{"mydb.users"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sqlGuard.ExtractTables(tt.sql)
			assert.ElementsMatch(t, tt.expected, result)
		})
	}
}

func TestSQLGuard_TableManagement(t *testing.T) {
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsCollector{}

	sqlGuard := NewSQLGuard(mockLogger, mockMetrics)

	t.Run("AddAndRemoveAllowedTable", func(t *testing.T) {
		sqlGuard.AddAllowedTable("test_table")
		tables := sqlGuard.GetAllowedTables()
		assert.Contains(t, tables, "test_table")

		sqlGuard.RemoveAllowedTable("test_table")
		tables = sqlGuard.GetAllowedTables()
		assert.NotContains(t, tables, "test_table")
	})

	t.Run("AddAndRemoveBlockedTable", func(t *testing.T) {
		sqlGuard.AddBlockedTable("dangerous_table")
		tables := sqlGuard.GetBlockedTables()
		assert.Contains(t, tables, "dangerous_table")

		sqlGuard.RemoveBlockedTable("dangerous_table")
		tables = sqlGuard.GetBlockedTables()
		assert.NotContains(t, tables, "dangerous_table")
	})

	t.Run("SetMaxJoinCount", func(t *testing.T) {
		sqlGuard.SetMaxJoinCount(10)
		assert.Equal(t, 10, sqlGuard.maxJoinCount)
	})

	t.Run("SetMaxSubqueryDepth", func(t *testing.T) {
		sqlGuard.SetMaxSubqueryDepth(5)
		assert.Equal(t, 5, sqlGuard.maxSubqueryDepth)
	})
}

func TestSQLGuard_ValidateWithTables(t *testing.T) {
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsCollector{}

	// 设置 mock 期望
	mockLogger.On("Warn", mock.AnythingOfType("string"), mock.Anything).Return()
	mockMetrics.On("RecordHistogram", mock.AnythingOfType("string"), mock.AnythingOfType("float64"), mock.AnythingOfType("map[string]string")).Return()
	mockMetrics.On("IncrementCounter", mock.AnythingOfType("string"), mock.AnythingOfType("map[string]string")).Return()

	sqlGuard := NewSQLGuard(mockLogger, mockMetrics)
	sqlGuard.AddAllowedTable("users")
	sqlGuard.AddAllowedTable("profiles")

	t.Run("ValidateWithTables_Success", func(t *testing.T) {
		sql := "SELECT * FROM users WHERE status = 'active'"
		err := sqlGuard.ValidateWithTables(context.Background(), sql)
		assert.NoError(t, err)
	})

	t.Run("ValidateWithTables_BlockedTable", func(t *testing.T) {
		sql := "SELECT * FROM mysql.user"
		err := sqlGuard.ValidateWithTables(context.Background(), sql)
		assert.Error(t, err)
		// 可能被危险模式或表访问控制拦截
		assert.True(t, strings.Contains(err.Error(), "禁止访问表") || strings.Contains(err.Error(), "危险模式"))
	})

	t.Run("ValidateWithTables_ComplexQuery", func(t *testing.T) {
		// 添加所有需要的表到允许列表
		sqlGuard.AddAllowedTable("addresses")
		sqlGuard.AddAllowedTable("orders")
		sqlGuard.AddAllowedTable("products")
		sqlGuard.AddAllowedTable("categories")
		sqlGuard.AddAllowedTable("suppliers")

		sql := `SELECT * FROM users u1 
				JOIN profiles p1 ON u1.id = p1.user_id 
				JOIN addresses a1 ON u1.id = a1.user_id 
				JOIN orders o1 ON u1.id = o1.user_id 
				JOIN products pr1 ON o1.product_id = pr1.id 
				JOIN categories c1 ON pr1.category_id = c1.id 
				JOIN suppliers s1 ON pr1.supplier_id = s1.id`

		err := sqlGuard.ValidateWithTables(context.Background(), sql)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "过多的JOIN操作")
	})
}
