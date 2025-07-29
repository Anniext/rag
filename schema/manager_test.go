package schema

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"pumppill/rag/core"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockCacheManager 模拟缓存管理器
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

// MockSchemaLoader 模拟 Schema 加载器
type MockSchemaLoader struct {
	mock.Mock
}

func (m *MockSchemaLoader) LoadSchema(ctx context.Context) (*core.SchemaInfo, error) {
	args := m.Called(ctx)
	return args.Get(0).(*core.SchemaInfo), args.Error(1)
}

func (m *MockSchemaLoader) GetTableInfo(ctx context.Context, tableName string) (*core.TableInfo, error) {
	args := m.Called(ctx, tableName)
	return args.Get(0).(*core.TableInfo), args.Error(1)
}

func (m *MockSchemaLoader) GetTableStatistics(ctx context.Context, tableName string) (map[string]interface{}, error) {
	args := m.Called(ctx, tableName)
	return args.Get(0).(map[string]interface{}), args.Error(1)
}

func (m *MockSchemaLoader) ValidateConnection(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockSchemaLoader) Close() error {
	args := m.Called()
	return args.Error(0)
}

// testLoaderWrapper 测试加载器包装器，用于适配 MySQLSchemaLoader 接口
type testLoaderWrapper struct {
	mock *MockSchemaLoader
}

func (w *testLoaderWrapper) LoadSchema(ctx context.Context) (*core.SchemaInfo, error) {
	return w.mock.LoadSchema(ctx)
}

func (w *testLoaderWrapper) GetTableInfo(ctx context.Context, tableName string) (*core.TableInfo, error) {
	return w.mock.GetTableInfo(ctx, tableName)
}

func (w *testLoaderWrapper) GetTableStatistics(ctx context.Context, tableName string) (map[string]interface{}, error) {
	return w.mock.GetTableStatistics(ctx, tableName)
}

func (w *testLoaderWrapper) ValidateConnection(ctx context.Context) error {
	return w.mock.ValidateConnection(ctx)
}

func (w *testLoaderWrapper) Close() error {
	return w.mock.Close()
}

func (w *testLoaderWrapper) GetTableNames(ctx context.Context) ([]string, error) {
	// 简单实现，返回测试数据
	return []string{"users", "posts"}, nil
}

// 创建一个简单的测试加载器，不依赖数据库
type simpleTestLoader struct {
	schema *core.SchemaInfo
}

func (l *simpleTestLoader) LoadSchema(ctx context.Context) (*core.SchemaInfo, error) {
	return l.schema, nil
}

func (l *simpleTestLoader) GetTableInfo(ctx context.Context, tableName string) (*core.TableInfo, error) {
	for _, table := range l.schema.Tables {
		if strings.ToLower(table.Name) == strings.ToLower(tableName) {
			return table, nil
		}
	}
	return nil, fmt.Errorf("表 %s 不存在", tableName)
}

func (l *simpleTestLoader) GetTableStatistics(ctx context.Context, tableName string) (map[string]interface{}, error) {
	return map[string]interface{}{
		"row_count":   100,
		"data_length": 16384,
	}, nil
}

func (l *simpleTestLoader) ValidateConnection(ctx context.Context) error {
	return nil
}

func (l *simpleTestLoader) Close() error {
	return nil
}

func (l *simpleTestLoader) GetTableNames(ctx context.Context) ([]string, error) {
	names := make([]string, len(l.schema.Tables))
	for i, table := range l.schema.Tables {
		names[i] = table.Name
	}
	return names, nil
}

func createTestSchema() *core.SchemaInfo {
	return &core.SchemaInfo{
		Database:  "test_db",
		Version:   "8.0.25",
		UpdatedAt: time.Now(),
		Tables: []*core.TableInfo{
			{
				Name:    "users",
				Comment: "用户表",
				Engine:  "InnoDB",
				Columns: []*core.Column{
					{
						Name:         "id",
						Type:         "bigint",
						IsPrimaryKey: true,
						Comment:      "用户ID",
						Position:     1,
					},
					{
						Name:     "username",
						Type:     "varchar(50)",
						Comment:  "用户名",
						Position: 2,
					},
					{
						Name:     "email",
						Type:     "varchar(100)",
						Comment:  "邮箱",
						Position: 3,
					},
				},
				Indexes: []*core.Index{
					{
						Name:     "PRIMARY",
						Type:     "PRIMARY",
						Columns:  []string{"id"},
						IsUnique: true,
					},
					{
						Name:     "idx_username",
						Type:     "INDEX",
						Columns:  []string{"username"},
						IsUnique: true,
					},
				},
				ForeignKeys: []*core.ForeignKey{},
			},
			{
				Name:    "posts",
				Comment: "文章表",
				Engine:  "InnoDB",
				Columns: []*core.Column{
					{
						Name:         "id",
						Type:         "bigint",
						IsPrimaryKey: true,
						Comment:      "文章ID",
						Position:     1,
					},
					{
						Name:     "user_id",
						Type:     "bigint",
						Comment:  "用户ID",
						Position: 2,
					},
					{
						Name:     "title",
						Type:     "varchar(200)",
						Comment:  "标题",
						Position: 3,
					},
				},
				Indexes: []*core.Index{
					{
						Name:     "PRIMARY",
						Type:     "PRIMARY",
						Columns:  []string{"id"},
						IsUnique: true,
					},
					{
						Name:     "idx_user_id",
						Type:     "INDEX",
						Columns:  []string{"user_id"},
						IsUnique: false,
					},
				},
				ForeignKeys: []*core.ForeignKey{
					{
						Name:             "fk_posts_user_id",
						Column:           "user_id",
						ReferencedTable:  "users",
						ReferencedColumn: "id",
						OnUpdate:         "CASCADE",
						OnDelete:         "CASCADE",
					},
				},
			},
		},
		Views: []*core.ViewInfo{
			{
				Name:       "user_stats",
				Definition: "SELECT COUNT(*) as total FROM users",
				Comment:    "用户统计",
				CreatedAt:  time.Now(),
			},
		},
	}
}

func TestNewManager(t *testing.T) {
	loader := &MySQLSchemaLoader{}
	cache := &MockCacheManager{}
	logger := &MockLogger{}
	config := &core.Config{
		Database: &core.DatabaseConfig{
			Database: "test_db",
		},
	}

	opts := &ManagerOptions{
		AutoRefresh:     false, // 禁用自动刷新以便测试
		RefreshInterval: time.Hour,
		CacheEnabled:    true,
		CacheTTL:        time.Hour,
	}

	manager := NewManager(loader, cache, logger, config, opts)

	assert.NotNil(t, manager)
	assert.Equal(t, loader, manager.loader)
	assert.Equal(t, cache, manager.cache)
	assert.Equal(t, logger, manager.logger)
	assert.Equal(t, config, manager.config)
	assert.False(t, manager.autoRefresh)
	assert.Equal(t, time.Hour, manager.refreshInterval)
	assert.True(t, manager.cacheEnabled)
	assert.Equal(t, time.Hour, manager.cacheTTL)
}

func TestManager_LoadSchema_FromDatabase(t *testing.T) {
	// 创建一个模拟加载器
	mockLoader := &MockSchemaLoader{}
	cache := &MockCacheManager{}
	logger := &MockLogger{}
	config := &core.Config{
		Database: &core.DatabaseConfig{
			Database: "test_db",
		},
	}

	opts := &ManagerOptions{
		AutoRefresh:  false,
		CacheEnabled: false, // 禁用缓存
	}

	// 创建一个包装器来适配接口
	loaderWrapper := &testLoaderWrapper{mockLoader}
	manager := NewManager(loaderWrapper, cache, logger, config, opts)

	testSchema := createTestSchema()

	// 模拟从数据库加载
	mockLoader.On("LoadSchema", mock.Anything).Return(testSchema, nil)

	ctx := context.Background()
	err := manager.LoadSchema(ctx)

	assert.NoError(t, err)
	assert.NotNil(t, manager.schema)
	assert.Equal(t, "test_db", manager.schema.Database)
	assert.Len(t, manager.tableMap, 2)

	mockLoader.AssertExpectations(t)
}

func TestManager_LoadSchema_FromCache(t *testing.T) {
	loader := &MySQLSchemaLoader{}
	cache := &MockCacheManager{}
	logger := &MockLogger{}
	config := &core.Config{
		Database: &core.DatabaseConfig{
			Database: "test_db",
		},
	}

	opts := &ManagerOptions{
		AutoRefresh:  false,
		CacheEnabled: true,
	}

	manager := NewManager(loader, cache, logger, config, opts)

	testSchema := createTestSchema()

	// 模拟从缓存加载成功
	cache.On("Get", mock.Anything, "schema:test_db").Return(testSchema, nil)

	ctx := context.Background()
	err := manager.LoadSchema(ctx)

	assert.NoError(t, err)
	assert.NotNil(t, manager.schema)
	assert.Equal(t, "test_db", manager.schema.Database)
	assert.Len(t, manager.tableMap, 2)

	cache.AssertExpectations(t)
}

func TestManager_GetTableInfo(t *testing.T) {
	manager := createManagerWithTestData()

	// 测试获取存在的表
	table, err := manager.GetTableInfo("users")
	assert.NoError(t, err)
	assert.NotNil(t, table)
	assert.Equal(t, "users", table.Name)
	assert.Equal(t, "用户表", table.Comment)
	assert.Len(t, table.Columns, 3)

	// 测试获取不存在的表
	table, err = manager.GetTableInfo("nonexistent")
	assert.Error(t, err)
	assert.Nil(t, table)
	assert.Contains(t, err.Error(), "表 nonexistent 不存在")

	// 测试大小写不敏感
	table, err = manager.GetTableInfo("USERS")
	assert.NoError(t, err)
	assert.NotNil(t, table)
	assert.Equal(t, "users", table.Name)
}

func TestManager_GetRelationships(t *testing.T) {
	manager := createManagerWithTestData()

	// 测试获取 posts 表的关系（有外键）
	relationships, err := manager.GetRelationships("posts")
	assert.NoError(t, err)
	assert.Len(t, relationships, 1)

	rel := relationships[0]
	assert.Equal(t, "one_to_many", rel.Type)
	assert.Equal(t, "posts", rel.FromTable)
	assert.Equal(t, "user_id", rel.FromColumn)
	assert.Equal(t, "users", rel.ToTable)
	assert.Equal(t, "id", rel.ToColumn)

	// 测试获取 users 表的关系（被其他表引用）
	relationships, err = manager.GetRelationships("users")
	assert.NoError(t, err)
	assert.Len(t, relationships, 1)

	rel = relationships[0]
	assert.Equal(t, "many_to_one", rel.Type)
	assert.Equal(t, "posts", rel.FromTable)
	assert.Equal(t, "user_id", rel.FromColumn)
	assert.Equal(t, "users", rel.ToTable)
	assert.Equal(t, "id", rel.ToColumn)
}

func TestManager_FindSimilarTables(t *testing.T) {
	manager := createManagerWithTestData()

	// 测试精确匹配
	tables, err := manager.FindSimilarTables("users")
	assert.NoError(t, err)
	assert.Len(t, tables, 1)
	assert.Equal(t, "users", tables[0].Name)

	// 测试前缀匹配
	tables, err = manager.FindSimilarTables("user")
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(tables), 1)
	// 应该包含 users 表
	found := false
	for _, table := range tables {
		if table.Name == "users" {
			found = true
			break
		}
	}
	assert.True(t, found)

	// 测试包含匹配
	tables, err = manager.FindSimilarTables("post")
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(tables), 1)
	// 应该包含 posts 表
	found = false
	for _, table := range tables {
		if table.Name == "posts" {
			found = true
			break
		}
	}
	assert.True(t, found)

	// 测试注释匹配
	tables, err = manager.FindSimilarTables("用户")
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(tables), 1)
	// 应该包含 users 表
	found = false
	for _, table := range tables {
		if table.Name == "users" {
			found = true
			break
		}
	}
	assert.True(t, found)

	// 测试列名匹配
	tables, err = manager.FindSimilarTables("username")
	assert.NoError(t, err)
	assert.Len(t, tables, 1)
	assert.Equal(t, "users", tables[0].Name)

	// 测试空查询
	tables, err = manager.FindSimilarTables("")
	assert.Error(t, err)
	assert.Nil(t, tables)

	// 测试无匹配结果
	tables, err = manager.FindSimilarTables("nonexistent")
	assert.NoError(t, err)
	assert.Len(t, tables, 0)
}

func TestManager_RefreshSchema(t *testing.T) {
	// 创建模拟加载器
	mockLoader := &MockSchemaLoader{}
	cache := &MockCacheManager{}
	logger := &MockLogger{}
	config := &core.Config{
		Database: &core.DatabaseConfig{
			Database: "test_db",
		},
	}

	opts := &ManagerOptions{
		AutoRefresh:  false,
		CacheEnabled: true,
	}

	// 创建一个包装器来适配接口
	loaderWrapper := &testLoaderWrapper{mockLoader}
	manager := NewManager(loaderWrapper, cache, logger, config, opts)

	testSchema := createTestSchema()

	// 模拟从数据库重新加载
	mockLoader.On("LoadSchema", mock.Anything).Return(testSchema, nil)
	cache.On("Set", mock.Anything, "schema:test_db", mock.Anything, mock.Anything).Return(nil)

	ctx := context.Background()
	err := manager.RefreshSchema(ctx)

	assert.NoError(t, err)
	assert.NotNil(t, manager.schema)
	assert.Equal(t, "test_db", manager.schema.Database)

	mockLoader.AssertExpectations(t)
	cache.AssertExpectations(t)
}

func TestManager_GetSchema(t *testing.T) {
	manager := createManagerWithTestData()

	schema := manager.GetSchema()
	assert.NotNil(t, schema)
	assert.Equal(t, "test_db", schema.Database)
	assert.Equal(t, "8.0.25", schema.Version)
	assert.Len(t, schema.Tables, 2)
	assert.Len(t, schema.Views, 1)

	// 验证返回的是副本，修改不会影响原始数据
	schema.Database = "modified"
	originalSchema := manager.GetSchema()
	assert.Equal(t, "test_db", originalSchema.Database)
}

func TestManager_GetTableNames(t *testing.T) {
	manager := createManagerWithTestData()

	names := manager.GetTableNames()
	assert.Len(t, names, 2)
	assert.Contains(t, names, "users")
	assert.Contains(t, names, "posts")
}

func TestManager_IsTableExists(t *testing.T) {
	manager := createManagerWithTestData()

	assert.True(t, manager.IsTableExists("users"))
	assert.True(t, manager.IsTableExists("USERS")) // 大小写不敏感
	assert.True(t, manager.IsTableExists("posts"))
	assert.False(t, manager.IsTableExists("nonexistent"))
}

func TestManager_SearchColumns(t *testing.T) {
	manager := createManagerWithTestData()

	// 测试按列名搜索
	results, err := manager.SearchColumns("id")
	assert.NoError(t, err)
	assert.Len(t, results, 2) // users.id 和 posts.id
	assert.Contains(t, results, "users")
	assert.Contains(t, results, "posts")

	// 测试按列注释搜索
	results, err = manager.SearchColumns("用户")
	assert.NoError(t, err)
	assert.Len(t, results, 2) // users.id (用户ID) 和 posts.user_id (用户ID)

	// 测试按列类型搜索
	results, err = manager.SearchColumns("varchar")
	assert.NoError(t, err)
	assert.Len(t, results, 2) // users 表有 username 和 email，posts 表有 title

	// 测试空查询
	results, err = manager.SearchColumns("")
	assert.Error(t, err)
	assert.Nil(t, results)

	// 测试无匹配结果
	results, err = manager.SearchColumns("nonexistent")
	assert.NoError(t, err)
	assert.Len(t, results, 0)
}

func TestManager_GetDatabaseInfo(t *testing.T) {
	manager := createManagerWithTestData()

	info := manager.GetDatabaseInfo()
	assert.Equal(t, "test_db", info["database"])
	assert.Equal(t, "8.0.25", info["version"])
	assert.Equal(t, 2, info["table_count"])
	assert.Equal(t, 1, info["view_count"])
	assert.False(t, info["cache_enabled"].(bool)) // 在 createManagerWithTestData 中禁用了缓存
	assert.False(t, info["auto_refresh"].(bool))
}

func TestManager_copyTableInfo(t *testing.T) {
	manager := &Manager{}

	original := &core.TableInfo{
		Name:    "test_table",
		Comment: "测试表",
		Columns: []*core.Column{
			{Name: "id", Type: "bigint", IsPrimaryKey: true},
		},
		Indexes: []*core.Index{
			{Name: "PRIMARY", Type: "PRIMARY", Columns: []string{"id"}},
		},
		ForeignKeys: []*core.ForeignKey{
			{Name: "fk_test", Column: "ref_id", ReferencedTable: "ref_table"},
		},
	}

	copy := manager.copyTableInfo(original)

	// 验证复制的内容正确
	assert.Equal(t, original.Name, copy.Name)
	assert.Equal(t, original.Comment, copy.Comment)
	assert.Len(t, copy.Columns, 1)
	assert.Len(t, copy.Indexes, 1)
	assert.Len(t, copy.ForeignKeys, 1)

	// 验证是深拷贝，修改副本不影响原始数据
	copy.Name = "modified"
	copy.Columns[0].Name = "modified"
	copy.Indexes[0].Name = "modified"
	copy.ForeignKeys[0].Name = "modified"

	assert.Equal(t, "test_table", original.Name)
	assert.Equal(t, "id", original.Columns[0].Name)
	assert.Equal(t, "PRIMARY", original.Indexes[0].Name)
	assert.Equal(t, "fk_test", original.ForeignKeys[0].Name)

	// 测试 nil 输入
	nilCopy := manager.copyTableInfo(nil)
	assert.Nil(t, nilCopy)
}

// createManagerWithTestData 创建包含测试数据的管理器
func createManagerWithTestData() *Manager {
	testSchema := createTestSchema()
	loader := &simpleTestLoader{schema: testSchema}
	cache := &MockCacheManager{}
	logger := &MockLogger{}
	config := &core.Config{
		Database: &core.DatabaseConfig{
			Database: "test_db",
		},
	}

	opts := &ManagerOptions{
		AutoRefresh:  false,
		CacheEnabled: false, // 禁用缓存以避免 mock 调用
		CacheTTL:     time.Hour,
	}

	manager := NewManager(loader, cache, logger, config, opts)

	// 直接设置测试数据
	manager.updateInMemoryCache(testSchema)

	return manager
}
