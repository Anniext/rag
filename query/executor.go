package query

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/Anniext/rag/core"
	"strings"
	"time"
)

// QueryExecutor 查询执行器
type QueryExecutor struct {
	db              *sql.DB
	securityManager core.SecurityManager
	logger          core.Logger
	config          *ExecutorConfig
}

// ExecutorConfig 执行器配置
type ExecutorConfig struct {
	QueryTimeout     time.Duration `json:"query_timeout"`
	MaxRows          int           `json:"max_rows"`
	EnablePagination bool          `json:"enable_pagination"`
	DefaultPageSize  int           `json:"default_page_size"`
	MaxPageSize      int           `json:"max_page_size"`
}

// NewQueryExecutor 创建新的查询执行器
func NewQueryExecutor(db *sql.DB, securityManager core.SecurityManager, logger core.Logger, config *ExecutorConfig) *QueryExecutor {
	if config == nil {
		config = &ExecutorConfig{
			QueryTimeout:     30 * time.Second,
			MaxRows:          10000,
			EnablePagination: true,
			DefaultPageSize:  20,
			MaxPageSize:      1000,
		}
	}

	return &QueryExecutor{
		db:              db,
		securityManager: securityManager,
		logger:          logger,
		config:          config,
	}
}

// ExecutionResult 执行结果
type ExecutionResult struct {
	Data          []map[string]interface{} `json:"data"`
	RowCount      int                      `json:"row_count"`
	AffectedRows  int64                    `json:"affected_rows"`
	ExecutionTime time.Duration            `json:"execution_time"`
	QueryPlan     string                   `json:"query_plan,omitempty"`
	Warnings      []string                 `json:"warnings"`
	Pagination    *PaginationInfo          `json:"pagination,omitempty"`
}

// PaginationInfo 分页信息
type PaginationInfo struct {
	Page       int  `json:"page"`
	PageSize   int  `json:"page_size"`
	TotalRows  int  `json:"total_rows"`
	TotalPages int  `json:"total_pages"`
	HasNext    bool `json:"has_next"`
	HasPrev    bool `json:"has_prev"`
}

// ExecuteQuery 执行查询
func (e *QueryExecutor) ExecuteQuery(ctx context.Context, generatedSQL *GeneratedSQL, userInfo *core.UserInfo, options *core.QueryOptions) (*ExecutionResult, error) {
	e.logger.Debug("开始执行查询", "sql", generatedSQL.SQL, "user", userInfo.ID)

	// 权限检查
	if err := e.checkPermissions(ctx, generatedSQL.SQL, userInfo); err != nil {
		return nil, fmt.Errorf("权限检查失败: %w", err)
	}

	// 创建带超时的上下文
	queryCtx, cancel := context.WithTimeout(ctx, e.config.QueryTimeout)
	defer cancel()

	// 记录开始时间
	startTime := time.Now()

	// 处理分页
	sql, params, pagination, err := e.handlePagination(generatedSQL.SQL, generatedSQL.Parameters, options)
	if err != nil {
		return nil, fmt.Errorf("处理分页失败: %w", err)
	}

	// 执行查询
	result, err := e.executeSQL(queryCtx, sql, params)
	if err != nil {
		return nil, fmt.Errorf("执行 SQL 失败: %w", err)
	}

	// 计算执行时间
	executionTime := time.Since(startTime)

	// 格式化结果
	formattedResult := &ExecutionResult{
		Data:          result.Data,
		RowCount:      result.RowCount,
		AffectedRows:  result.AffectedRows,
		ExecutionTime: executionTime,
		Warnings:      generatedSQL.Warnings,
		Pagination:    pagination,
	}

	// 获取查询计划（如果需要）
	if options != nil && options.Explain {
		queryPlan, err := e.getQueryPlan(queryCtx, generatedSQL.SQL, generatedSQL.Parameters)
		if err != nil {
			e.logger.Warn("获取查询计划失败", "error", err)
		} else {
			formattedResult.QueryPlan = queryPlan
		}
	}

	e.logger.Debug("查询执行完成",
		"rows", result.RowCount,
		"execution_time", executionTime,
		"user", userInfo.ID)

	return formattedResult, nil
}

// checkPermissions 检查查询权限
func (e *QueryExecutor) checkPermissions(ctx context.Context, sql string, userInfo *core.UserInfo) error {
	// 验证 SQL 安全性
	if err := e.securityManager.ValidateSQL(ctx, sql); err != nil {
		return fmt.Errorf("SQL 安全验证失败: %w", err)
	}

	// 提取表名进行权限检查
	tables := e.extractTablesFromSQL(sql)
	for _, table := range tables {
		// 检查表的读取权限
		if err := e.securityManager.CheckPermission(ctx, userInfo, table, "read"); err != nil {
			return fmt.Errorf("用户 %s 没有表 %s 的读取权限: %w", userInfo.Username, table, err)
		}
	}

	// 检查写操作权限
	if e.isWriteOperation(sql) {
		for _, table := range tables {
			if err := e.securityManager.CheckPermission(ctx, userInfo, table, "write"); err != nil {
				return fmt.Errorf("用户 %s 没有表 %s 的写入权限: %w", userInfo.Username, table, err)
			}
		}
	}

	return nil
}

// extractTablesFromSQL 从 SQL 中提取表名
func (e *QueryExecutor) extractTablesFromSQL(sql string) []string {
	var tables []string
	upperSQL := strings.ToUpper(sql)

	// 简单的表名提取逻辑
	// 这里可以使用更复杂的 SQL 解析器
	patterns := []string{
		"FROM ",
		"JOIN ",
		"UPDATE ",
		"INSERT INTO ",
		"DELETE FROM ",
	}

	for _, pattern := range patterns {
		if idx := strings.Index(upperSQL, pattern); idx != -1 {
			remaining := sql[idx+len(pattern):]
			// 提取表名（简化版本）
			parts := strings.Fields(remaining)
			if len(parts) > 0 {
				tableName := strings.Trim(parts[0], "`\"'")
				if !contains(tables, tableName) {
					tables = append(tables, tableName)
				}
			}
		}
	}

	return tables
}

// isWriteOperation 检查是否为写操作
func (e *QueryExecutor) isWriteOperation(sql string) bool {
	upperSQL := strings.ToUpper(strings.TrimSpace(sql))
	writeKeywords := []string{"INSERT", "UPDATE", "DELETE", "CREATE", "DROP", "ALTER", "TRUNCATE"}

	for _, keyword := range writeKeywords {
		if strings.HasPrefix(upperSQL, keyword) {
			return true
		}
	}

	return false
}

// handlePagination 处理分页
func (e *QueryExecutor) handlePagination(sql string, params []interface{}, options *core.QueryOptions) (string, []interface{}, *PaginationInfo, error) {
	if !e.config.EnablePagination || options == nil {
		return sql, params, nil, nil
	}

	// 获取分页参数
	page := 1
	pageSize := e.config.DefaultPageSize

	if options.Offset > 0 && options.Limit > 0 {
		page = (options.Offset / options.Limit) + 1
		pageSize = options.Limit
	} else if options.Limit > 0 {
		pageSize = options.Limit
	}

	// 限制页面大小
	if pageSize > e.config.MaxPageSize {
		pageSize = e.config.MaxPageSize
	}

	// 计算偏移量
	offset := (page - 1) * pageSize

	// 检查是否已经包含 LIMIT 子句
	upperSQL := strings.ToUpper(sql)
	if strings.Contains(upperSQL, "LIMIT") {
		// 如果已经有 LIMIT，直接返回
		return sql, params, nil, nil
	}

	// 添加 LIMIT 和 OFFSET
	paginatedSQL := fmt.Sprintf("%s LIMIT %d OFFSET %d", sql, pageSize, offset)

	// 获取总行数（用于分页信息）
	totalRows, err := e.getTotalRows(sql, params)
	if err != nil {
		e.logger.Warn("获取总行数失败", "error", err)
		totalRows = 0
	}

	// 计算分页信息
	totalPages := (totalRows + pageSize - 1) / pageSize
	pagination := &PaginationInfo{
		Page:       page,
		PageSize:   pageSize,
		TotalRows:  totalRows,
		TotalPages: totalPages,
		HasNext:    page < totalPages,
		HasPrev:    page > 1,
	}

	return paginatedSQL, params, pagination, nil
}

// getTotalRows 获取查询的总行数
func (e *QueryExecutor) getTotalRows(sql string, params []interface{}) (int, error) {
	// 构建 COUNT 查询
	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM (%s) AS count_query", sql)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var count int
	err := e.db.QueryRowContext(ctx, countSQL, params...).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// executeSQL 执行 SQL 查询
func (e *QueryExecutor) executeSQL(ctx context.Context, sql string, params []interface{}) (*ExecutionResult, error) {
	upperSQL := strings.ToUpper(strings.TrimSpace(sql))

	if strings.HasPrefix(upperSQL, "SELECT") {
		return e.executeSelectQuery(ctx, sql, params)
	} else {
		return e.executeWriteQuery(ctx, sql, params)
	}
}

// executeSelectQuery 执行 SELECT 查询
func (e *QueryExecutor) executeSelectQuery(ctx context.Context, sql string, params []interface{}) (*ExecutionResult, error) {
	rows, err := e.db.QueryContext(ctx, sql, params...)
	if err != nil {
		return nil, fmt.Errorf("执行查询失败: %w", err)
	}
	defer rows.Close()

	// 获取列信息
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("获取列信息失败: %w", err)
	}

	// 获取列类型
	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		return nil, fmt.Errorf("获取列类型失败: %w", err)
	}

	var data []map[string]interface{}
	rowCount := 0

	for rows.Next() {
		// 检查行数限制
		if rowCount >= e.config.MaxRows {
			e.logger.Warn("查询结果超过最大行数限制", "max_rows", e.config.MaxRows)
			break
		}

		// 创建扫描目标
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		// 扫描行数据
		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("扫描行数据失败: %w", err)
		}

		// 转换数据类型
		row := make(map[string]interface{})
		for i, col := range columns {
			row[col] = e.convertValue(values[i], columnTypes[i])
		}

		data = append(data, row)
		rowCount++
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历结果集失败: %w", err)
	}

	return &ExecutionResult{
		Data:     data,
		RowCount: rowCount,
	}, nil
}

// executeWriteQuery 执行写操作查询
func (e *QueryExecutor) executeWriteQuery(ctx context.Context, sql string, params []interface{}) (*ExecutionResult, error) {
	result, err := e.db.ExecContext(ctx, sql, params...)
	if err != nil {
		return nil, fmt.Errorf("执行写操作失败: %w", err)
	}

	affectedRows, err := result.RowsAffected()
	if err != nil {
		e.logger.Warn("获取影响行数失败", "error", err)
		affectedRows = 0
	}

	return &ExecutionResult{
		Data:         []map[string]interface{}{},
		RowCount:     0,
		AffectedRows: affectedRows,
	}, nil
}

// convertValue 转换数据库值到适当的 Go 类型
func (e *QueryExecutor) convertValue(value interface{}, columnType *sql.ColumnType) interface{} {
	if value == nil {
		return nil
	}

	switch v := value.(type) {
	case []byte:
		// 处理字节数组
		return string(v)
	case time.Time:
		// 格式化时间
		return v.Format("2006-01-02 15:04:05")
	default:
		return value
	}
}

// getQueryPlan 获取查询执行计划
func (e *QueryExecutor) getQueryPlan(ctx context.Context, sql string, params []interface{}) (string, error) {
	explainSQL := fmt.Sprintf("EXPLAIN %s", sql)

	rows, err := e.db.QueryContext(ctx, explainSQL, params...)
	if err != nil {
		return "", fmt.Errorf("获取查询计划失败: %w", err)
	}
	defer rows.Close()

	var planLines []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			return "", fmt.Errorf("扫描查询计划失败: %w", err)
		}
		planLines = append(planLines, line)
	}

	return strings.Join(planLines, "\n"), nil
}

// FormatResult 格式化查询结果
func (e *QueryExecutor) FormatResult(result *ExecutionResult, format string) (interface{}, error) {
	switch strings.ToLower(format) {
	case "json":
		return result, nil
	case "csv":
		return e.formatAsCSV(result)
	case "table":
		return e.formatAsTable(result)
	default:
		return result, nil
	}
}

// formatAsCSV 格式化为 CSV
func (e *QueryExecutor) formatAsCSV(result *ExecutionResult) (string, error) {
	if len(result.Data) == 0 {
		return "", nil
	}

	var lines []string

	// 获取列名
	var columns []string
	for col := range result.Data[0] {
		columns = append(columns, col)
	}
	lines = append(lines, strings.Join(columns, ","))

	// 添加数据行
	for _, row := range result.Data {
		var values []string
		for _, col := range columns {
			value := fmt.Sprintf("%v", row[col])
			// 简单的 CSV 转义
			if strings.Contains(value, ",") || strings.Contains(value, "\"") {
				value = fmt.Sprintf("\"%s\"", strings.ReplaceAll(value, "\"", "\"\""))
			}
			values = append(values, value)
		}
		lines = append(lines, strings.Join(values, ","))
	}

	return strings.Join(lines, "\n"), nil
}

// formatAsTable 格式化为表格
func (e *QueryExecutor) formatAsTable(result *ExecutionResult) (string, error) {
	if len(result.Data) == 0 {
		return "No data", nil
	}

	// 获取列名
	var columns []string
	for col := range result.Data[0] {
		columns = append(columns, col)
	}

	// 计算列宽
	colWidths := make(map[string]int)
	for _, col := range columns {
		colWidths[col] = len(col)
	}

	for _, row := range result.Data {
		for _, col := range columns {
			value := fmt.Sprintf("%v", row[col])
			if len(value) > colWidths[col] {
				colWidths[col] = len(value)
			}
		}
	}

	var lines []string

	// 表头
	var headerParts []string
	var separatorParts []string
	for _, col := range columns {
		width := colWidths[col]
		headerParts = append(headerParts, fmt.Sprintf("%-*s", width, col))
		separatorParts = append(separatorParts, strings.Repeat("-", width))
	}
	lines = append(lines, "| "+strings.Join(headerParts, " | ")+" |")
	lines = append(lines, "|"+strings.Join(separatorParts, "|")+"|")

	// 数据行
	for _, row := range result.Data {
		var rowParts []string
		for _, col := range columns {
			width := colWidths[col]
			value := fmt.Sprintf("%v", row[col])
			rowParts = append(rowParts, fmt.Sprintf("%-*s", width, value))
		}
		lines = append(lines, "| "+strings.Join(rowParts, " | ")+" |")
	}

	return strings.Join(lines, "\n"), nil
}

// GetExecutionStats 获取执行统计信息
func (e *QueryExecutor) GetExecutionStats(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// 获取数据库连接统计
	dbStats := e.db.Stats()
	stats["db_stats"] = map[string]interface{}{
		"open_connections":    dbStats.OpenConnections,
		"in_use":              dbStats.InUse,
		"idle":                dbStats.Idle,
		"wait_count":          dbStats.WaitCount,
		"wait_duration":       dbStats.WaitDuration.String(),
		"max_idle_closed":     dbStats.MaxIdleClosed,
		"max_lifetime_closed": dbStats.MaxLifetimeClosed,
	}

	// 获取配置信息
	stats["config"] = map[string]interface{}{
		"query_timeout":     e.config.QueryTimeout.String(),
		"max_rows":          e.config.MaxRows,
		"enable_pagination": e.config.EnablePagination,
		"default_page_size": e.config.DefaultPageSize,
		"max_page_size":     e.config.MaxPageSize,
	}

	return stats, nil
}

// Close 关闭执行器
func (e *QueryExecutor) Close() error {
	if e.db != nil {
		return e.db.Close()
	}
	return nil
}
