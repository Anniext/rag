package schema

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockLogger 模拟日志记录器
type MockLogger struct{}

func (l *MockLogger) Debug(msg string, fields ...interface{}) {}
func (l *MockLogger) Info(msg string, fields ...interface{})  {}
func (l *MockLogger) Warn(msg string, fields ...interface{})  {}
func (l *MockLogger) Error(msg string, fields ...interface{}) {}
func (l *MockLogger) Fatal(msg string, fields ...interface{}) {}

func TestNewMySQLSchemaLoader(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// 模拟 Ping 成功
	mock.ExpectPing()

	// 创建加载器（使用已存在的数据库连接）
	loader := &MySQLSchemaLoader{
		db:       db,
		database: "test_db",
		logger:   &MockLogger{},
	}

	assert.NotNil(t, loader)
	assert.Equal(t, "test_db", loader.database)

	// 验证所有期望的调用都被执行
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMySQLSchemaLoader_getDatabaseVersion(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	loader := &MySQLSchemaLoader{
		db:       db,
		database: "test_db",
		logger:   &MockLogger{},
	}

	// 模拟版本查询
	mock.ExpectQuery("SELECT VERSION\\(\\)").
		WillReturnRows(sqlmock.NewRows([]string{"VERSION()"}).
			AddRow("8.0.25"))

	ctx := context.Background()
	version, err := loader.getDatabaseVersion(ctx)

	assert.NoError(t, err)
	assert.Equal(t, "8.0.25", version)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMySQLSchemaLoader_loadTables(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	loader := &MySQLSchemaLoader{
		db:       db,
		database: "test_db",
		logger:   &MockLogger{},
	}

	// 模拟表查询
	tableRows := sqlmock.NewRows([]string{
		"TABLE_NAME", "ENGINE", "TABLE_COMMENT", "TABLE_ROWS",
		"DATA_LENGTH", "CREATE_TIME", "UPDATE_TIME",
	}).AddRow(
		"users", "InnoDB", "用户表", 100,
		16384, time.Now(), time.Now(),
	)

	mock.ExpectQuery("SELECT.*FROM information_schema.TABLES").
		WithArgs("test_db").
		WillReturnRows(tableRows)

	// 模拟列查询
	columnRows := sqlmock.NewRows([]string{
		"COLUMN_NAME", "DATA_TYPE", "IS_NULLABLE", "COLUMN_DEFAULT",
		"COLUMN_COMMENT", "COLUMN_KEY", "EXTRA", "CHARACTER_SET_NAME",
		"COLLATION_NAME", "ORDINAL_POSITION",
	}).AddRow(
		"id", "bigint", "NO", nil,
		"用户ID", "PRI", "auto_increment", nil,
		nil, 1,
	).AddRow(
		"username", "varchar(50)", "NO", nil,
		"用户名", "", "", "utf8mb4",
		"utf8mb4_unicode_ci", 2,
	)

	mock.ExpectQuery("SELECT.*FROM information_schema.COLUMNS").
		WithArgs("test_db", "users").
		WillReturnRows(columnRows)

	// 模拟索引查询
	indexRows := sqlmock.NewRows([]string{
		"INDEX_NAME", "NON_UNIQUE", "COLUMN_NAME", "INDEX_TYPE",
	}).AddRow(
		"PRIMARY", 0, "id", "BTREE",
	).AddRow(
		"idx_username", 0, "username", "BTREE",
	)

	mock.ExpectQuery("SELECT.*FROM information_schema.STATISTICS").
		WithArgs("test_db", "users").
		WillReturnRows(indexRows)

	// 模拟外键查询
	fkRows := sqlmock.NewRows([]string{
		"CONSTRAINT_NAME", "COLUMN_NAME", "REFERENCED_TABLE_NAME",
		"REFERENCED_COLUMN_NAME", "UPDATE_RULE", "DELETE_RULE",
	})

	mock.ExpectQuery("SELECT.*FROM information_schema.KEY_COLUMN_USAGE").
		WithArgs("test_db", "users").
		WillReturnRows(fkRows)

	ctx := context.Background()
	tables, err := loader.loadTables(ctx)

	assert.NoError(t, err)
	assert.Len(t, tables, 1)

	table := tables[0]
	assert.Equal(t, "users", table.Name)
	assert.Equal(t, "InnoDB", table.Engine)
	assert.Equal(t, "用户表", table.Comment)
	assert.Equal(t, int64(100), table.RowCount)
	assert.Equal(t, int64(16384), table.DataLength)

	// 验证列信息
	assert.Len(t, table.Columns, 2)

	idColumn := table.Columns[0]
	assert.Equal(t, "id", idColumn.Name)
	assert.Equal(t, "bigint", idColumn.Type)
	assert.False(t, idColumn.Nullable)
	assert.True(t, idColumn.IsPrimaryKey)
	assert.True(t, idColumn.IsAutoIncrement)
	assert.Equal(t, 1, idColumn.Position)

	usernameColumn := table.Columns[1]
	assert.Equal(t, "username", usernameColumn.Name)
	assert.Equal(t, "varchar(50)", usernameColumn.Type)
	assert.False(t, usernameColumn.Nullable)
	assert.False(t, usernameColumn.IsPrimaryKey)
	assert.False(t, usernameColumn.IsAutoIncrement)
	assert.Equal(t, "utf8mb4", usernameColumn.CharacterSet)
	assert.Equal(t, "utf8mb4_unicode_ci", usernameColumn.Collation)
	assert.Equal(t, 2, usernameColumn.Position)

	// 验证索引信息
	assert.Len(t, table.Indexes, 2)

	primaryIndex := table.Indexes[0]
	assert.Equal(t, "PRIMARY", primaryIndex.Name)
	assert.Equal(t, "PRIMARY", primaryIndex.Type)
	assert.True(t, primaryIndex.IsUnique)
	assert.Equal(t, []string{"id"}, primaryIndex.Columns)

	usernameIndex := table.Indexes[1]
	assert.Equal(t, "idx_username", usernameIndex.Name)
	assert.Equal(t, "INDEX", usernameIndex.Type)
	assert.True(t, usernameIndex.IsUnique)
	assert.Equal(t, []string{"username"}, usernameIndex.Columns)

	// 验证外键信息
	assert.Len(t, table.ForeignKeys, 0)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMySQLSchemaLoader_loadViews(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	loader := &MySQLSchemaLoader{
		db:       db,
		database: "test_db",
		logger:   &MockLogger{},
	}

	// 模拟视图查询
	viewRows := sqlmock.NewRows([]string{
		"TABLE_NAME", "VIEW_DEFINITION",
	}).AddRow(
		"user_stats", "SELECT COUNT(*) as total FROM users",
	)

	mock.ExpectQuery("SELECT.*FROM information_schema.VIEWS").
		WithArgs("test_db").
		WillReturnRows(viewRows)

	ctx := context.Background()
	views, err := loader.loadViews(ctx)

	assert.NoError(t, err)
	assert.Len(t, views, 1)

	view := views[0]
	assert.Equal(t, "user_stats", view.Name)
	assert.Equal(t, "SELECT COUNT(*) as total FROM users", view.Definition)
	assert.NotZero(t, view.CreatedAt)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMySQLSchemaLoader_GetTableInfo(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	loader := &MySQLSchemaLoader{
		db:       db,
		database: "test_db",
		logger:   &MockLogger{},
	}

	// 模拟表基本信息查询
	tableRow := sqlmock.NewRows([]string{
		"TABLE_NAME", "ENGINE", "TABLE_COMMENT", "TABLE_ROWS",
		"DATA_LENGTH", "CREATE_TIME", "UPDATE_TIME",
	}).AddRow(
		"users", "InnoDB", "用户表", 100,
		16384, time.Now(), time.Now(),
	)

	mock.ExpectQuery("SELECT.*FROM information_schema.TABLES").
		WithArgs("test_db", "users").
		WillReturnRows(tableRow)

	// 模拟列查询
	columnRows := sqlmock.NewRows([]string{
		"COLUMN_NAME", "DATA_TYPE", "IS_NULLABLE", "COLUMN_DEFAULT",
		"COLUMN_COMMENT", "COLUMN_KEY", "EXTRA", "CHARACTER_SET_NAME",
		"COLLATION_NAME", "ORDINAL_POSITION",
	}).AddRow(
		"id", "bigint", "NO", nil,
		"用户ID", "PRI", "auto_increment", nil,
		nil, 1,
	)

	mock.ExpectQuery("SELECT.*FROM information_schema.COLUMNS").
		WithArgs("test_db", "users").
		WillReturnRows(columnRows)

	// 模拟索引查询
	indexRows := sqlmock.NewRows([]string{
		"INDEX_NAME", "NON_UNIQUE", "COLUMN_NAME", "INDEX_TYPE",
	}).AddRow(
		"PRIMARY", 0, "id", "BTREE",
	)

	mock.ExpectQuery("SELECT.*FROM information_schema.STATISTICS").
		WithArgs("test_db", "users").
		WillReturnRows(indexRows)

	// 模拟外键查询
	fkRows := sqlmock.NewRows([]string{
		"CONSTRAINT_NAME", "COLUMN_NAME", "REFERENCED_TABLE_NAME",
		"REFERENCED_COLUMN_NAME", "UPDATE_RULE", "DELETE_RULE",
	})

	mock.ExpectQuery("SELECT.*FROM information_schema.KEY_COLUMN_USAGE").
		WithArgs("test_db", "users").
		WillReturnRows(fkRows)

	ctx := context.Background()
	table, err := loader.GetTableInfo(ctx, "users")

	assert.NoError(t, err)
	assert.NotNil(t, table)
	assert.Equal(t, "users", table.Name)
	assert.Equal(t, "InnoDB", table.Engine)
	assert.Equal(t, "用户表", table.Comment)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMySQLSchemaLoader_GetTableInfo_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	loader := &MySQLSchemaLoader{
		db:       db,
		database: "test_db",
		logger:   &MockLogger{},
	}

	// 模拟表不存在
	mock.ExpectQuery("SELECT.*FROM information_schema.TABLES").
		WithArgs("test_db", "nonexistent").
		WillReturnError(sql.ErrNoRows)

	ctx := context.Background()
	table, err := loader.GetTableInfo(ctx, "nonexistent")

	assert.Error(t, err)
	assert.Nil(t, table)
	assert.Contains(t, err.Error(), "表 nonexistent 不存在")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMySQLSchemaLoader_GetTableNames(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	loader := &MySQLSchemaLoader{
		db:       db,
		database: "test_db",
		logger:   &MockLogger{},
	}

	// 模拟表名查询
	tableRows := sqlmock.NewRows([]string{"TABLE_NAME"}).
		AddRow("users").
		AddRow("posts").
		AddRow("comments")

	mock.ExpectQuery("SELECT TABLE_NAME FROM information_schema.TABLES").
		WithArgs("test_db").
		WillReturnRows(tableRows)

	ctx := context.Background()
	tableNames, err := loader.GetTableNames(ctx)

	assert.NoError(t, err)
	assert.Equal(t, []string{"users", "posts", "comments"}, tableNames)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMySQLSchemaLoader_GetTableStatistics(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	loader := &MySQLSchemaLoader{
		db:       db,
		database: "test_db",
		logger:   &MockLogger{},
	}

	// 模拟行数查询
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM `users`").
		WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(100))

	// 模拟大小信息查询
	sizeRows := sqlmock.NewRows([]string{
		"DATA_LENGTH", "INDEX_LENGTH", "DATA_FREE",
	}).AddRow(16384, 8192, 0)

	mock.ExpectQuery("SELECT.*FROM information_schema.TABLES").
		WithArgs("test_db", "users").
		WillReturnRows(sizeRows)

	ctx := context.Background()
	stats, err := loader.GetTableStatistics(ctx, "users")

	assert.NoError(t, err)
	assert.Equal(t, int64(100), stats["row_count"])
	assert.Equal(t, int64(16384), stats["data_length"])
	assert.Equal(t, int64(8192), stats["index_length"])
	assert.Equal(t, int64(0), stats["data_free"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMySQLSchemaLoader_ValidateConnection(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	loader := &MySQLSchemaLoader{
		db:       db,
		database: "test_db",
		logger:   &MockLogger{},
	}

	// 模拟 Ping 成功
	mock.ExpectPing()

	ctx := context.Background()
	err = loader.ValidateConnection(ctx)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMySQLSchemaLoader_getIndexType(t *testing.T) {
	loader := &MySQLSchemaLoader{
		logger: &MockLogger{},
	}

	tests := []struct {
		indexName string
		indexType string
		expected  string
	}{
		{"PRIMARY", "BTREE", "PRIMARY"},
		{"idx_name", "FULLTEXT", "FULLTEXT"},
		{"idx_name", "BTREE", "INDEX"},
		{"unique_idx", "BTREE", "INDEX"},
	}

	for _, test := range tests {
		result := loader.getIndexType(test.indexName, test.indexType)
		assert.Equal(t, test.expected, result)
	}
}

func TestMySQLSchemaLoader_LoadSchema(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	loader := &MySQLSchemaLoader{
		db:       db,
		database: "test_db",
		logger:   &MockLogger{},
	}

	// 模拟版本查询
	mock.ExpectQuery("SELECT VERSION\\(\\)").
		WillReturnRows(sqlmock.NewRows([]string{"VERSION()"}).
			AddRow("8.0.25"))

	// 模拟表查询
	tableRows := sqlmock.NewRows([]string{
		"TABLE_NAME", "ENGINE", "TABLE_COMMENT", "TABLE_ROWS",
		"DATA_LENGTH", "CREATE_TIME", "UPDATE_TIME",
	}).AddRow(
		"users", "InnoDB", "用户表", 100,
		16384, time.Now(), time.Now(),
	)

	mock.ExpectQuery("SELECT.*FROM information_schema.TABLES").
		WithArgs("test_db").
		WillReturnRows(tableRows)

	// 模拟列查询
	columnRows := sqlmock.NewRows([]string{
		"COLUMN_NAME", "DATA_TYPE", "IS_NULLABLE", "COLUMN_DEFAULT",
		"COLUMN_COMMENT", "COLUMN_KEY", "EXTRA", "CHARACTER_SET_NAME",
		"COLLATION_NAME", "ORDINAL_POSITION",
	}).AddRow(
		"id", "bigint", "NO", nil,
		"用户ID", "PRI", "auto_increment", nil,
		nil, 1,
	)

	mock.ExpectQuery("SELECT.*FROM information_schema.COLUMNS").
		WithArgs("test_db", "users").
		WillReturnRows(columnRows)

	// 模拟索引查询
	indexRows := sqlmock.NewRows([]string{
		"INDEX_NAME", "NON_UNIQUE", "COLUMN_NAME", "INDEX_TYPE",
	}).AddRow(
		"PRIMARY", 0, "id", "BTREE",
	)

	mock.ExpectQuery("SELECT.*FROM information_schema.STATISTICS").
		WithArgs("test_db", "users").
		WillReturnRows(indexRows)

	// 模拟外键查询
	fkRows := sqlmock.NewRows([]string{
		"CONSTRAINT_NAME", "COLUMN_NAME", "REFERENCED_TABLE_NAME",
		"REFERENCED_COLUMN_NAME", "UPDATE_RULE", "DELETE_RULE",
	})

	mock.ExpectQuery("SELECT.*FROM information_schema.KEY_COLUMN_USAGE").
		WithArgs("test_db", "users").
		WillReturnRows(fkRows)

	// 模拟视图查询
	viewRows := sqlmock.NewRows([]string{
		"TABLE_NAME", "VIEW_DEFINITION",
	}).AddRow(
		"user_stats", "SELECT COUNT(*) as total FROM users",
	)

	mock.ExpectQuery("SELECT.*FROM information_schema.VIEWS").
		WithArgs("test_db").
		WillReturnRows(viewRows)

	ctx := context.Background()
	schema, err := loader.LoadSchema(ctx)

	assert.NoError(t, err)
	assert.NotNil(t, schema)
	assert.Equal(t, "test_db", schema.Database)
	assert.Equal(t, "8.0.25", schema.Version)
	assert.Len(t, schema.Tables, 1)
	assert.Len(t, schema.Views, 1)
	assert.NotZero(t, schema.UpdatedAt)

	assert.NoError(t, mock.ExpectationsWereMet())
}
