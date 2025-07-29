// 本文件实现了 Schema 管理器，负责数据库结构的加载、缓存、查询和统计等功能。
// 主要功能：
// 1. 定义 SchemaLoader 接口，抽象数据库结构加载、表信息获取、统计、连接验证等操作。
// 2. Manager 结构体实现 Schema 管理，集成 loader、缓存、日志、配置等组件。
// 3. 支持内存缓存 Schema 信息和表映射，加速查询。
// 4. 线程安全，支持并发读写。
// 5. 便于扩展多种数据库和缓存实现。

package schema

import (
	"context" // 上下文管理
	"fmt"     // 格式化输出
	"github.com/Anniext/rag/core"
	"strings" // 字符串处理
	"sync"    // 并发锁
	"time"    // 时间处理
)

// SchemaLoader Schema 加载器接口，定义数据库结构相关操作。
type SchemaLoader interface {
	LoadSchema(ctx context.Context) (*core.SchemaInfo, error)                                 // 加载完整 Schema
	GetTableInfo(ctx context.Context, tableName string) (*core.TableInfo, error)              // 获取单表信息
	GetTableStatistics(ctx context.Context, tableName string) (map[string]interface{}, error) // 获取表统计信息
	ValidateConnection(ctx context.Context) error                                             // 验证数据库连接
	Close() error                                                                             // 关闭连接
	GetTableNames(ctx context.Context) ([]string, error)                                      // 获取所有表名
}

// Manager Schema 管理器实现，负责 Schema 的加载、缓存和查询。
type Manager struct {
	loader SchemaLoader      // Schema 加载器
	cache  core.CacheManager // 缓存管理器
	logger core.Logger       // 日志记录器
	config *core.Config      // 配置信息

	// 内存缓存
	schema      *core.SchemaInfo           // 当前 Schema 信息
	tableMap    map[string]*core.TableInfo // 表名到表结构的映射
	lastUpdated time.Time                  // 上次更新时间
	mutex       sync.RWMutex               // 读写锁，保证并发安全

	// 配置选项
	autoRefresh     bool          // 是否自动刷新 Schema
	refreshInterval time.Duration // 刷新间隔
	cacheEnabled    bool          // 是否启用缓存
	cacheTTL        time.Duration // 缓存有效期
}

// ManagerOptions Schema 管理器选项
type ManagerOptions struct {
	AutoRefresh     bool
	RefreshInterval time.Duration
	CacheEnabled    bool
	CacheTTL        time.Duration
}

// NewManager 创建 Schema 管理器
func NewManager(loader SchemaLoader, cache core.CacheManager, logger core.Logger, config *core.Config, opts *ManagerOptions) *Manager {
	if opts == nil {
		opts = &ManagerOptions{
			AutoRefresh:     true,
			RefreshInterval: time.Hour,
			CacheEnabled:    true,
			CacheTTL:        time.Hour,
		}
	}

	manager := &Manager{
		loader:          loader,
		cache:           cache,
		logger:          logger,
		config:          config,
		tableMap:        make(map[string]*core.TableInfo),
		autoRefresh:     opts.AutoRefresh,
		refreshInterval: opts.RefreshInterval,
		cacheEnabled:    opts.CacheEnabled,
		cacheTTL:        opts.CacheTTL,
	}

	// 启动自动刷新
	if manager.autoRefresh {
		go manager.startAutoRefresh()
	}

	return manager
}

// LoadSchema 加载数据库 Schema
func (m *Manager) LoadSchema(ctx context.Context) error {
	m.logger.Info("开始加载数据库 Schema")

	// 尝试从缓存加载
	if m.cacheEnabled {
		if schema, err := m.loadFromCache(ctx); err == nil && schema != nil {
			m.logger.Info("从缓存加载 Schema 成功")
			m.updateInMemoryCache(schema)
			return nil
		}
	}

	// 从数据库加载
	schema, err := m.loader.LoadSchema(ctx)
	if err != nil {
		return fmt.Errorf("从数据库加载 Schema 失败: %w", err)
	}

	// 更新内存缓存
	m.updateInMemoryCache(schema)

	// 保存到缓存
	if m.cacheEnabled {
		if err := m.saveToCache(ctx, schema); err != nil {
			m.logger.Warn("保存 Schema 到缓存失败", "error", err)
		}
	}

	m.logger.Info("数据库 Schema 加载完成",
		"tables", len(schema.Tables),
		"views", len(schema.Views))

	return nil
}

// GetTableInfo 获取表信息
func (m *Manager) GetTableInfo(tableName string) (*core.TableInfo, error) {
	m.mutex.RLock()

	// 检查是否需要刷新
	if m.needsRefresh() {
		m.mutex.RUnlock()
		if err := m.RefreshSchema(context.Background()); err != nil {
			m.logger.Warn("刷新 Schema 失败", "error", err)
		}
		m.mutex.RLock()
	}

	table, exists := m.tableMap[strings.ToLower(tableName)]
	if !exists {
		m.mutex.RUnlock()
		return nil, fmt.Errorf("表 %s 不存在", tableName)
	}

	// 返回表的副本以避免并发修改
	result := m.copyTableInfo(table)
	m.mutex.RUnlock()
	return result, nil
}

// GetRelationships 获取表关系
func (m *Manager) GetRelationships(tableName string) ([]*core.Relationship, error) {
	table, err := m.GetTableInfo(tableName)
	if err != nil {
		return nil, err
	}

	var relationships []*core.Relationship

	// 基于外键构建关系
	for _, fk := range table.ForeignKeys {
		relationship := &core.Relationship{
			Type:            "one_to_many", // 默认为一对多关系
			FromTable:       tableName,
			FromColumn:      fk.Column,
			ToTable:         fk.ReferencedTable,
			ToColumn:        fk.ReferencedColumn,
			RelationshipKey: fk.Name,
		}
		relationships = append(relationships, relationship)
	}

	// 查找反向关系（其他表引用当前表）
	m.mutex.RLock()
	for _, otherTable := range m.tableMap {
		if otherTable.Name == tableName {
			continue
		}

		for _, fk := range otherTable.ForeignKeys {
			if fk.ReferencedTable == tableName {
				relationship := &core.Relationship{
					Type:            "many_to_one", // 反向关系
					FromTable:       otherTable.Name,
					FromColumn:      fk.Column,
					ToTable:         tableName,
					ToColumn:        fk.ReferencedColumn,
					RelationshipKey: fk.Name,
				}
				relationships = append(relationships, relationship)
			}
		}
	}
	m.mutex.RUnlock()

	return relationships, nil
}

// FindSimilarTables 查找相似的表
func (m *Manager) FindSimilarTables(query string) ([]*core.TableInfo, error) {
	m.mutex.RLock()

	if m.needsRefresh() {
		m.mutex.RUnlock()
		if err := m.RefreshSchema(context.Background()); err != nil {
			m.logger.Warn("刷新 Schema 失败", "error", err)
		}
		m.mutex.RLock()
	}

	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		m.mutex.RUnlock()
		return nil, fmt.Errorf("查询字符串不能为空")
	}

	var results []*core.TableInfo
	var exactMatches []*core.TableInfo
	var prefixMatches []*core.TableInfo
	var containsMatches []*core.TableInfo

	for _, table := range m.tableMap {
		tableName := strings.ToLower(table.Name)
		tableComment := strings.ToLower(table.Comment)

		// 精确匹配
		if tableName == query {
			exactMatches = append(exactMatches, m.copyTableInfo(table))
			continue
		}

		// 前缀匹配
		if strings.HasPrefix(tableName, query) {
			prefixMatches = append(prefixMatches, m.copyTableInfo(table))
			continue
		}

		// 包含匹配（表名或注释）
		if strings.Contains(tableName, query) || strings.Contains(tableComment, query) {
			containsMatches = append(containsMatches, m.copyTableInfo(table))
			continue
		}

		// 列名匹配
		for _, column := range table.Columns {
			columnName := strings.ToLower(column.Name)
			columnComment := strings.ToLower(column.Comment)

			if strings.Contains(columnName, query) || strings.Contains(columnComment, query) {
				containsMatches = append(containsMatches, m.copyTableInfo(table))
				break
			}
		}
	}

	// 按匹配优先级排序结果
	results = append(results, exactMatches...)
	results = append(results, prefixMatches...)
	results = append(results, containsMatches...)

	// 限制结果数量
	maxResults := 20
	if len(results) > maxResults {
		results = results[:maxResults]
	}

	m.mutex.RUnlock()
	return results, nil
}

// RefreshSchema 刷新 Schema
func (m *Manager) RefreshSchema(ctx context.Context) error {
	m.logger.Info("开始刷新数据库 Schema")

	// 从数据库重新加载
	schema, err := m.loader.LoadSchema(ctx)
	if err != nil {
		return fmt.Errorf("刷新 Schema 失败: %w", err)
	}

	// 更新内存缓存
	m.updateInMemoryCache(schema)

	// 更新缓存
	if m.cacheEnabled {
		if err := m.saveToCache(ctx, schema); err != nil {
			m.logger.Warn("保存刷新的 Schema 到缓存失败", "error", err)
		}
	}

	m.logger.Info("数据库 Schema 刷新完成")
	return nil
}

// GetSchema 获取完整的 Schema 信息
func (m *Manager) GetSchema() *core.SchemaInfo {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if m.schema == nil {
		return nil
	}

	// 返回 Schema 的副本
	schemaCopy := &core.SchemaInfo{
		Database:  m.schema.Database,
		Version:   m.schema.Version,
		UpdatedAt: m.schema.UpdatedAt,
		Tables:    make([]*core.TableInfo, len(m.schema.Tables)),
		Views:     make([]*core.ViewInfo, len(m.schema.Views)),
	}

	for i, table := range m.schema.Tables {
		schemaCopy.Tables[i] = m.copyTableInfo(table)
	}

	for i, view := range m.schema.Views {
		schemaCopy.Views[i] = &core.ViewInfo{
			Name:       view.Name,
			Definition: view.Definition,
			Comment:    view.Comment,
			CreatedAt:  view.CreatedAt,
		}
	}

	return schemaCopy
}

// GetTableNames 获取所有表名
func (m *Manager) GetTableNames() []string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	names := make([]string, 0, len(m.tableMap))
	for name := range m.tableMap {
		names = append(names, name)
	}

	return names
}

// IsTableExists 检查表是否存在
func (m *Manager) IsTableExists(tableName string) bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	_, exists := m.tableMap[strings.ToLower(tableName)]
	return exists
}

// GetTableStatistics 获取表统计信息
func (m *Manager) GetTableStatistics(ctx context.Context, tableName string) (map[string]interface{}, error) {
	if !m.IsTableExists(tableName) {
		return nil, fmt.Errorf("表 %s 不存在", tableName)
	}

	return m.loader.GetTableStatistics(ctx, tableName)
}

// updateInMemoryCache 更新内存缓存
func (m *Manager) updateInMemoryCache(schema *core.SchemaInfo) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.schema = schema
	m.lastUpdated = time.Now()

	// 重建表映射
	m.tableMap = make(map[string]*core.TableInfo)
	for _, table := range schema.Tables {
		m.tableMap[strings.ToLower(table.Name)] = table
	}
}

// needsRefresh 检查是否需要刷新
func (m *Manager) needsRefresh() bool {
	if m.schema == nil {
		return true
	}

	return time.Since(m.lastUpdated) > m.refreshInterval
}

// copyTableInfo 复制表信息
func (m *Manager) copyTableInfo(table *core.TableInfo) *core.TableInfo {
	if table == nil {
		return nil
	}

	copy := &core.TableInfo{
		Name:        table.Name,
		Comment:     table.Comment,
		Engine:      table.Engine,
		Charset:     table.Charset,
		RowCount:    table.RowCount,
		DataLength:  table.DataLength,
		CreatedAt:   table.CreatedAt,
		UpdatedAt:   table.UpdatedAt,
		Columns:     make([]*core.Column, len(table.Columns)),
		Indexes:     make([]*core.Index, len(table.Indexes)),
		ForeignKeys: make([]*core.ForeignKey, len(table.ForeignKeys)),
	}

	// 复制列信息
	for i, col := range table.Columns {
		copy.Columns[i] = &core.Column{
			Name:            col.Name,
			Type:            col.Type,
			Nullable:        col.Nullable,
			DefaultValue:    col.DefaultValue,
			Comment:         col.Comment,
			IsPrimaryKey:    col.IsPrimaryKey,
			IsAutoIncrement: col.IsAutoIncrement,
			CharacterSet:    col.CharacterSet,
			Collation:       col.Collation,
			Position:        col.Position,
		}
	}

	// 复制索引信息
	for i, idx := range table.Indexes {
		copy.Indexes[i] = &core.Index{
			Name:     idx.Name,
			Type:     idx.Type,
			Columns:  append([]string{}, idx.Columns...),
			IsUnique: idx.IsUnique,
		}
	}

	// 复制外键信息
	for i, fk := range table.ForeignKeys {
		copy.ForeignKeys[i] = &core.ForeignKey{
			Name:             fk.Name,
			Column:           fk.Column,
			ReferencedTable:  fk.ReferencedTable,
			ReferencedColumn: fk.ReferencedColumn,
			OnUpdate:         fk.OnUpdate,
			OnDelete:         fk.OnDelete,
		}
	}

	return copy
}

// loadFromCache 从缓存加载 Schema
func (m *Manager) loadFromCache(ctx context.Context) (*core.SchemaInfo, error) {
	if m.cache == nil {
		return nil, fmt.Errorf("缓存管理器未配置")
	}

	cacheKey := fmt.Sprintf("schema:%s", m.config.Database.Database)

	data, err := m.cache.Get(ctx, cacheKey)
	if err != nil {
		return nil, err
	}

	schema, ok := data.(*core.SchemaInfo)
	if !ok {
		return nil, fmt.Errorf("缓存数据类型错误")
	}

	return schema, nil
}

// saveToCache 保存 Schema 到缓存
func (m *Manager) saveToCache(ctx context.Context, schema *core.SchemaInfo) error {
	if m.cache == nil {
		return fmt.Errorf("缓存管理器未配置")
	}

	cacheKey := fmt.Sprintf("schema:%s", m.config.Database.Database)

	return m.cache.Set(ctx, cacheKey, schema, m.cacheTTL)
}

// startAutoRefresh 启动自动刷新
func (m *Manager) startAutoRefresh() {
	ticker := time.NewTicker(m.refreshInterval)
	defer ticker.Stop()

	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

		if err := m.RefreshSchema(ctx); err != nil {
			m.logger.Error("自动刷新 Schema 失败", "error", err)
		}

		cancel()
	}
}

// Close 关闭管理器
func (m *Manager) Close() error {
	if m.loader != nil {
		return m.loader.Close()
	}
	return nil
}

// ValidateConnection 验证数据库连接
func (m *Manager) ValidateConnection(ctx context.Context) error {
	return m.loader.ValidateConnection(ctx)
}

// GetDatabaseInfo 获取数据库信息
func (m *Manager) GetDatabaseInfo() map[string]interface{} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	info := make(map[string]interface{})

	if m.schema != nil {
		info["database"] = m.schema.Database
		info["version"] = m.schema.Version
		info["table_count"] = len(m.schema.Tables)
		info["view_count"] = len(m.schema.Views)
		info["last_updated"] = m.schema.UpdatedAt
	}

	info["cache_enabled"] = m.cacheEnabled
	info["auto_refresh"] = m.autoRefresh
	info["refresh_interval"] = m.refreshInterval.String()
	info["last_refresh"] = m.lastUpdated

	return info
}

// SearchColumns 搜索列
func (m *Manager) SearchColumns(query string) (map[string][]*core.Column, error) {
	m.mutex.RLock()

	if m.needsRefresh() {
		m.mutex.RUnlock()
		if err := m.RefreshSchema(context.Background()); err != nil {
			m.logger.Warn("刷新 Schema 失败", "error", err)
		}
		m.mutex.RLock()
	}

	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		m.mutex.RUnlock()
		return nil, fmt.Errorf("查询字符串不能为空")
	}

	results := make(map[string][]*core.Column)

	for tableName, table := range m.tableMap {
		var matchedColumns []*core.Column

		for _, column := range table.Columns {
			columnName := strings.ToLower(column.Name)
			columnComment := strings.ToLower(column.Comment)
			columnType := strings.ToLower(column.Type)

			if strings.Contains(columnName, query) ||
				strings.Contains(columnComment, query) ||
				strings.Contains(columnType, query) {

				// 创建列的副本
				columnCopy := &core.Column{
					Name:            column.Name,
					Type:            column.Type,
					Nullable:        column.Nullable,
					DefaultValue:    column.DefaultValue,
					Comment:         column.Comment,
					IsPrimaryKey:    column.IsPrimaryKey,
					IsAutoIncrement: column.IsAutoIncrement,
					CharacterSet:    column.CharacterSet,
					Collation:       column.Collation,
					Position:        column.Position,
				}

				matchedColumns = append(matchedColumns, columnCopy)
			}
		}

		if len(matchedColumns) > 0 {
			results[tableName] = matchedColumns
		}
	}

	m.mutex.RUnlock()
	return results, nil
}
