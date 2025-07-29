// 本文件实现了 MySQL 数据库 Schema 加载器，负责自动化加载数据库结构信息，包括表、列、索引、外键、视图等。
// 主要功能：
// 1. 连接 MySQL 数据库并管理连接池。
// 2. 加载数据库版本、表结构、列属性、索引、外键、视图等详细信息。
// 3. 支持单表信息、表名列表、表统计信息的查询。
// 4. 提供连接验证与关闭方法。
// 5. 结构体和方法均有详细注释，便于理解和维护。

package schema

import (
	"context"      // 上下文管理
	"database/sql" // 数据库操作
	"errors"
	"fmt" // 格式化输出
	"github.com/Anniext/rag/core"
	"strings" // 字符串处理
	"time"    // 时间处理

	_ "github.com/go-sql-driver/mysql" // MySQL 驱动
)

// MySQLSchemaLoader MySQL Schema 加载器，负责加载数据库结构信息。
type MySQLSchemaLoader struct {
	db       *sql.DB     // 数据库连接
	database string      // 当前数据库名
	logger   core.Logger // 日志记录器
}

// NewMySQLSchemaLoader 创建 MySQL Schema 加载器。
// 参数：dsn 数据库连接串，database 数据库名，logger 日志记录器。
// 返回：MySQLSchemaLoader 实例或错误。
func NewMySQLSchemaLoader(dsn string, database string, logger core.Logger) (*MySQLSchemaLoader, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}

	// 测试连接
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("数据库连接测试失败: %w", err)
	}

	// 设置连接池参数
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)

	return &MySQLSchemaLoader{
		db:       db,
		database: database,
		logger:   logger,
	}, nil
}

// LoadSchema 加载完整的数据库 Schema
func (l *MySQLSchemaLoader) LoadSchema(ctx context.Context) (*core.SchemaInfo, error) {
	l.logger.Info("开始加载数据库 Schema", "database", l.database)

	schema := &core.SchemaInfo{
		Database:  l.database,
		UpdatedAt: time.Now(),
	}

	// 获取数据库版本
	version, err := l.getDatabaseVersion(ctx)
	if err != nil {
		l.logger.Warn("获取数据库版本失败", "error", err)
		version = "unknown"
	}
	schema.Version = version

	// 加载表信息
	tables, err := l.loadTables(ctx)
	if err != nil {
		return nil, fmt.Errorf("加载表信息失败: %w", err)
	}
	schema.Tables = tables

	// 加载视图信息
	views, err := l.loadViews(ctx)
	if err != nil {
		return nil, fmt.Errorf("加载视图信息失败: %w", err)
	}
	schema.Views = views

	l.logger.Info("数据库 Schema 加载完成",
		"tables", len(tables),
		"views", len(views))

	return schema, nil
}

// getDatabaseVersion 获取数据库版本
func (l *MySQLSchemaLoader) getDatabaseVersion(ctx context.Context) (string, error) {
	var version string
	query := "SELECT VERSION()"

	err := l.db.QueryRowContext(ctx, query).Scan(&version)
	if err != nil {
		return "", err
	}

	return version, nil
}

// loadTables 加载所有表信息
func (l *MySQLSchemaLoader) loadTables(ctx context.Context) ([]*core.TableInfo, error) {
	// 查询表基本信息
	query := `
		SELECT 
			TABLE_NAME,
			ENGINE,
			TABLE_COMMENT,
			TABLE_ROWS,
			DATA_LENGTH,
			CREATE_TIME,
			UPDATE_TIME
		FROM information_schema.TABLES 
		WHERE TABLE_SCHEMA = ? AND TABLE_TYPE = 'BASE TABLE'
		ORDER BY TABLE_NAME
	`

	rows, err := l.db.QueryContext(ctx, query, l.database)
	if err != nil {
		return nil, fmt.Errorf("查询表信息失败: %w", err)
	}
	defer rows.Close()

	var tables []*core.TableInfo
	for rows.Next() {
		table := &core.TableInfo{}
		var createTime, updateTime sql.NullTime
		var rowCount sql.NullInt64
		var dataLength sql.NullInt64

		err := rows.Scan(
			&table.Name,
			&table.Engine,
			&table.Comment,
			&rowCount,
			&dataLength,
			&createTime,
			&updateTime,
		)
		if err != nil {
			l.logger.Error("扫描表信息失败", "error", err)
			continue
		}

		// 处理可空字段
		if rowCount.Valid {
			table.RowCount = rowCount.Int64
		}
		if dataLength.Valid {
			table.DataLength = dataLength.Int64
		}
		if createTime.Valid {
			table.CreatedAt = createTime.Time
		}
		if updateTime.Valid {
			table.UpdatedAt = updateTime.Time
		}

		// 加载表的详细信息
		if err := l.loadTableDetails(ctx, table); err != nil {
			l.logger.Error("加载表详细信息失败", "table", table.Name, "error", err)
			continue
		}

		tables = append(tables, table)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历表信息失败: %w", err)
	}

	return tables, nil
}

// loadTableDetails 加载表的详细信息（列、索引、外键）
func (l *MySQLSchemaLoader) loadTableDetails(ctx context.Context, table *core.TableInfo) error {
	// 加载列信息
	columns, err := l.loadColumns(ctx, table.Name)
	if err != nil {
		return fmt.Errorf("加载列信息失败: %w", err)
	}
	table.Columns = columns

	// 加载索引信息
	indexes, err := l.loadIndexes(ctx, table.Name)
	if err != nil {
		return fmt.Errorf("加载索引信息失败: %w", err)
	}
	table.Indexes = indexes

	// 加载外键信息
	foreignKeys, err := l.loadForeignKeys(ctx, table.Name)
	if err != nil {
		return fmt.Errorf("加载外键信息失败: %w", err)
	}
	table.ForeignKeys = foreignKeys

	return nil
}

// loadColumns 加载表的列信息
func (l *MySQLSchemaLoader) loadColumns(ctx context.Context, tableName string) ([]*core.Column, error) {
	query := `
		SELECT 
			COLUMN_NAME,
			DATA_TYPE,
			IS_NULLABLE,
			COLUMN_DEFAULT,
			COLUMN_COMMENT,
			COLUMN_KEY,
			EXTRA,
			CHARACTER_SET_NAME,
			COLLATION_NAME,
			ORDINAL_POSITION
		FROM information_schema.COLUMNS 
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
		ORDER BY ORDINAL_POSITION
	`

	rows, err := l.db.QueryContext(ctx, query, l.database, tableName)
	if err != nil {
		return nil, fmt.Errorf("查询列信息失败: %w", err)
	}
	defer rows.Close()

	var columns []*core.Column
	for rows.Next() {
		column := &core.Column{}
		var nullable, columnKey, extra string
		var defaultValue, characterSet, collation sql.NullString

		err := rows.Scan(
			&column.Name,
			&column.Type,
			&nullable,
			&defaultValue,
			&column.Comment,
			&columnKey,
			&extra,
			&characterSet,
			&collation,
			&column.Position,
		)
		if err != nil {
			return nil, fmt.Errorf("扫描列信息失败: %w", err)
		}

		// 处理字段属性
		column.Nullable = nullable == "YES"
		column.IsPrimaryKey = columnKey == "PRI"
		column.IsAutoIncrement = strings.Contains(extra, "auto_increment")

		if defaultValue.Valid {
			column.DefaultValue = defaultValue.String
		}
		if characterSet.Valid {
			column.CharacterSet = characterSet.String
		}
		if collation.Valid {
			column.Collation = collation.String
		}

		columns = append(columns, column)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历列信息失败: %w", err)
	}

	return columns, nil
}

// loadIndexes 加载表的索引信息
func (l *MySQLSchemaLoader) loadIndexes(ctx context.Context, tableName string) ([]*core.Index, error) {
	query := `
		SELECT 
			INDEX_NAME,
			NON_UNIQUE,
			COLUMN_NAME,
			INDEX_TYPE
		FROM information_schema.STATISTICS 
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
		ORDER BY INDEX_NAME, SEQ_IN_INDEX
	`

	rows, err := l.db.QueryContext(ctx, query, l.database, tableName)
	if err != nil {
		return nil, fmt.Errorf("查询索引信息失败: %w", err)
	}
	defer rows.Close()

	indexMap := make(map[string]*core.Index)
	for rows.Next() {
		var indexName, columnName, indexType string
		var nonUnique int

		err := rows.Scan(&indexName, &nonUnique, &columnName, &indexType)
		if err != nil {
			return nil, fmt.Errorf("扫描索引信息失败: %w", err)
		}

		index, exists := indexMap[indexName]
		if !exists {
			index = &core.Index{
				Name:     indexName,
				Type:     l.getIndexType(indexName, indexType),
				IsUnique: nonUnique == 0,
				Columns:  []string{},
			}
			indexMap[indexName] = index
		}

		index.Columns = append(index.Columns, columnName)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历索引信息失败: %w", err)
	}

	// 转换为切片
	var indexes []*core.Index
	for _, index := range indexMap {
		indexes = append(indexes, index)
	}

	return indexes, nil
}

// getIndexType 获取索引类型
func (l *MySQLSchemaLoader) getIndexType(indexName, indexType string) string {
	if indexName == "PRIMARY" {
		return "PRIMARY"
	}
	if indexType == "FULLTEXT" {
		return "FULLTEXT"
	}
	return "INDEX"
}

// loadForeignKeys 加载表的外键信息
func (l *MySQLSchemaLoader) loadForeignKeys(ctx context.Context, tableName string) ([]*core.ForeignKey, error) {
	query := `
		SELECT 
			CONSTRAINT_NAME,
			COLUMN_NAME,
			REFERENCED_TABLE_NAME,
			REFERENCED_COLUMN_NAME,
			UPDATE_RULE,
			DELETE_RULE
		FROM information_schema.KEY_COLUMN_USAGE kcu
		JOIN information_schema.REFERENTIAL_CONSTRAINTS rc 
			ON kcu.CONSTRAINT_NAME = rc.CONSTRAINT_NAME 
			AND kcu.CONSTRAINT_SCHEMA = rc.CONSTRAINT_SCHEMA
		WHERE kcu.TABLE_SCHEMA = ? AND kcu.TABLE_NAME = ?
		ORDER BY kcu.CONSTRAINT_NAME
	`

	rows, err := l.db.QueryContext(ctx, query, l.database, tableName)
	if err != nil {
		return nil, fmt.Errorf("查询外键信息失败: %w", err)
	}
	defer rows.Close()

	var foreignKeys []*core.ForeignKey
	for rows.Next() {
		fk := &core.ForeignKey{}

		err := rows.Scan(
			&fk.Name,
			&fk.Column,
			&fk.ReferencedTable,
			&fk.ReferencedColumn,
			&fk.OnUpdate,
			&fk.OnDelete,
		)
		if err != nil {
			return nil, fmt.Errorf("扫描外键信息失败: %w", err)
		}

		foreignKeys = append(foreignKeys, fk)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历外键信息失败: %w", err)
	}

	return foreignKeys, nil
}

// loadViews 加载视图信息
func (l *MySQLSchemaLoader) loadViews(ctx context.Context) ([]*core.ViewInfo, error) {
	query := `
		SELECT 
			TABLE_NAME,
			VIEW_DEFINITION
		FROM information_schema.VIEWS 
		WHERE TABLE_SCHEMA = ?
		ORDER BY TABLE_NAME
	`

	rows, err := l.db.QueryContext(ctx, query, l.database)
	if err != nil {
		return nil, fmt.Errorf("查询视图信息失败: %w", err)
	}
	defer rows.Close()

	var views []*core.ViewInfo
	for rows.Next() {
		view := &core.ViewInfo{
			CreatedAt: time.Now(), // MySQL 视图没有创建时间，使用当前时间
		}

		err := rows.Scan(&view.Name, &view.Definition)
		if err != nil {
			return nil, fmt.Errorf("扫描视图信息失败: %w", err)
		}

		views = append(views, view)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历视图信息失败: %w", err)
	}

	return views, nil
}

// GetTableInfo 获取单���表的信息
func (l *MySQLSchemaLoader) GetTableInfo(ctx context.Context, tableName string) (*core.TableInfo, error) {
	// 查询表基本信息
	query := `
		SELECT 
			TABLE_NAME,
			ENGINE,
			TABLE_COMMENT,
			TABLE_ROWS,
			DATA_LENGTH,
			CREATE_TIME,
			UPDATE_TIME
		FROM information_schema.TABLES 
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? AND TABLE_TYPE = 'BASE TABLE'
	`

	table := &core.TableInfo{}
	var createTime, updateTime sql.NullTime
	var rowCount sql.NullInt64
	var dataLength sql.NullInt64

	err := l.db.QueryRowContext(ctx, query, l.database, tableName).Scan(
		&table.Name,
		&table.Engine,
		&table.Comment,
		&rowCount,
		&dataLength,
		&createTime,
		&updateTime,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("表 %s 不存在", tableName)
		}
		return nil, fmt.Errorf("查询表信息失败: %w", err)
	}

	// 处理可空字段
	if rowCount.Valid {
		table.RowCount = rowCount.Int64
	}
	if dataLength.Valid {
		table.DataLength = dataLength.Int64
	}
	if createTime.Valid {
		table.CreatedAt = createTime.Time
	}
	if updateTime.Valid {
		table.UpdatedAt = updateTime.Time
	}

	// 加载表的详细信息
	if err := l.loadTableDetails(ctx, table); err != nil {
		return nil, fmt.Errorf("加载表详细信息失败: %w", err)
	}

	return table, nil
}

// ValidateConnection 验证数据库连接
func (l *MySQLSchemaLoader) ValidateConnection(ctx context.Context) error {
	return l.db.PingContext(ctx)
}

// Close 关闭数据库连接
func (l *MySQLSchemaLoader) Close() error {
	if l.db != nil {
		return l.db.Close()
	}
	return nil
}

// GetTableNames 获取所有表名
func (l *MySQLSchemaLoader) GetTableNames(ctx context.Context) ([]string, error) {
	query := `
		SELECT TABLE_NAME 
		FROM information_schema.TABLES 
		WHERE TABLE_SCHEMA = ? AND TABLE_TYPE = 'BASE TABLE'
		ORDER BY TABLE_NAME
	`

	rows, err := l.db.QueryContext(ctx, query, l.database)
	if err != nil {
		return nil, fmt.Errorf("查询表名失败: %w", err)
	}
	defer rows.Close()

	var tableNames []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, fmt.Errorf("扫描表名失败: %w", err)
		}
		tableNames = append(tableNames, tableName)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历表名失败: %w", err)
	}

	return tableNames, nil
}

// GetTableStatistics 获取表统计信息
func (l *MySQLSchemaLoader) GetTableStatistics(ctx context.Context, tableName string) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// 获取表行数
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM `%s`", tableName)
	var rowCount int64
	if err := l.db.QueryRowContext(ctx, countQuery).Scan(&rowCount); err != nil {
		l.logger.Warn("获取表行数失败", "table", tableName, "error", err)
		rowCount = 0
	}
	stats["row_count"] = rowCount

	// 获取表大小信息
	sizeQuery := `
		SELECT 
			DATA_LENGTH,
			INDEX_LENGTH,
			DATA_FREE
		FROM information_schema.TABLES 
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
	`

	var dataLength, indexLength, dataFree sql.NullInt64
	err := l.db.QueryRowContext(ctx, sizeQuery, l.database, tableName).Scan(
		&dataLength, &indexLength, &dataFree)
	if err != nil {
		l.logger.Warn("获取表大小信息失败", "table", tableName, "error", err)
	} else {
		if dataLength.Valid {
			stats["data_length"] = dataLength.Int64
		}
		if indexLength.Valid {
			stats["index_length"] = indexLength.Int64
		}
		if dataFree.Valid {
			stats["data_free"] = dataFree.Int64
		}
	}

	return stats, nil
}
