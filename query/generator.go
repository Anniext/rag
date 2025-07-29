package query

import (
	"context"
	"fmt"
	"github.com/Anniext/rag/core"
	"regexp"
	"strings"
)

// SQLGenerator SQL 生成引擎
type SQLGenerator struct {
	schemaManager core.SchemaManager
	logger        core.Logger
}

// NewSQLGenerator 创建新的 SQL 生成器
func NewSQLGenerator(schemaManager core.SchemaManager, logger core.Logger) *SQLGenerator {
	return &SQLGenerator{
		schemaManager: schemaManager,
		logger:        logger,
	}
}

// GeneratedSQL 生成的 SQL 结构
type GeneratedSQL struct {
	SQL         string                 `json:"sql"`
	Parameters  []interface{}          `json:"parameters"`
	Explanation string                 `json:"explanation"`
	Warnings    []string               `json:"warnings"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// GenerateSQL 基于解析后的查询生成 SQL
func (g *SQLGenerator) GenerateSQL(ctx context.Context, parsed *ParsedQuery) (*GeneratedSQL, error) {
	g.logger.Debug("开始生成 SQL", "parsed", parsed)

	var sql string
	var parameters []interface{}
	var warnings []string
	var err error

	switch parsed.Intent {
	case IntentSelect:
		sql, parameters, warnings, err = g.generateSelectSQL(ctx, parsed)
	case IntentInsert:
		sql, parameters, warnings, err = g.generateInsertSQL(ctx, parsed)
	case IntentUpdate:
		sql, parameters, warnings, err = g.generateUpdateSQL(ctx, parsed)
	case IntentDelete:
		sql, parameters, warnings, err = g.generateDeleteSQL(ctx, parsed)
	case IntentCount:
		sql, parameters, warnings, err = g.generateCountSQL(ctx, parsed)
	case IntentAggregate:
		sql, parameters, warnings, err = g.generateAggregateSQL(ctx, parsed)
	default:
		return nil, fmt.Errorf("不支持的查询意图: %s", parsed.Intent)
	}

	if err != nil {
		return nil, fmt.Errorf("生成 SQL 失败: %w", err)
	}

	// 验证生成的 SQL
	if err := g.validateSQL(sql); err != nil {
		return nil, fmt.Errorf("SQL 验证失败: %w", err)
	}

	// 生成解释
	explanation := g.generateExplanation(parsed, sql)

	// 生成元数据
	metadata := map[string]interface{}{
		"intent":     string(parsed.Intent),
		"tables":     parsed.Tables,
		"columns":    parsed.Columns,
		"complexity": parsed.Complexity,
		"confidence": parsed.Confidence,
	}

	result := &GeneratedSQL{
		SQL:         sql,
		Parameters:  parameters,
		Explanation: explanation,
		Warnings:    warnings,
		Metadata:    metadata,
	}

	g.logger.Debug("SQL 生成完成", "result", result)
	return result, nil
}

// generateSelectSQL 生成 SELECT 语句
func (g *SQLGenerator) generateSelectSQL(ctx context.Context, parsed *ParsedQuery) (string, []interface{}, []string, error) {
	warnings := make([]string, 0)
	parameters := make([]interface{}, 0)

	// 构建 SELECT 子句
	selectClause := g.buildSelectClause(parsed.Columns)

	// 构建 FROM 子句
	fromClause, joinWarnings, err := g.buildFromClause(ctx, parsed.Tables)
	if err != nil {
		return "", nil, nil, err
	}
	warnings = append(warnings, joinWarnings...)

	// 构建 WHERE 子句
	whereClause, whereParams, whereWarnings := g.buildWhereClause(parsed.Conditions)
	parameters = append(parameters, whereParams...)
	warnings = append(warnings, whereWarnings...)

	// 构建其他子句
	var clauses []string
	clauses = append(clauses, selectClause)
	clauses = append(clauses, fromClause)

	if whereClause != "" {
		clauses = append(clauses, whereClause)
	}

	// 处理操作
	for _, op := range parsed.Operations {
		switch op.Type {
		case "group_by":
			if column, ok := op.Parameters["column"].(string); ok {
				clauses = append(clauses, fmt.Sprintf("GROUP BY %s", g.escapeIdentifier(column)))
			}
		case "order_by":
			if column, ok := op.Parameters["column"].(string); ok {
				direction := "ASC"
				if dir, ok := op.Parameters["direction"].(string); ok {
					direction = dir
				}
				clauses = append(clauses, fmt.Sprintf("ORDER BY %s %s", g.escapeIdentifier(column), direction))
			}
		case "limit":
			if count, ok := op.Parameters["count"].(string); ok {
				clauses = append(clauses, fmt.Sprintf("LIMIT %s", count))
			}
		}
	}

	sql := strings.Join(clauses, " ")
	return sql, parameters, warnings, nil
}

// generateInsertSQL 生成 INSERT 语句
func (g *SQLGenerator) generateInsertSQL(ctx context.Context, parsed *ParsedQuery) (string, []interface{}, []string, error) {
	warnings := make([]string, 0)
	parameters := make([]interface{}, 0)

	if len(parsed.Tables) != 1 {
		return "", nil, nil, fmt.Errorf("INSERT 语句只能操作一个表")
	}

	tableName := parsed.Tables[0]

	// 获取表信息
	tableInfo, err := g.schemaManager.GetTableInfo(tableName)
	if err != nil {
		return "", nil, nil, fmt.Errorf("获取表信息失败: %w", err)
	}

	// 构建列名和值
	var columns []string
	var placeholders []string

	// 从参数中提取值
	if values, ok := parsed.Parameters["values"].(map[string]interface{}); ok {
		for column, value := range values {
			// 验证列是否存在
			if g.columnExists(tableInfo, column) {
				columns = append(columns, g.escapeIdentifier(column))
				placeholders = append(placeholders, "?")
				parameters = append(parameters, value)
			} else {
				warnings = append(warnings, fmt.Sprintf("列 %s 在表 %s 中不存在", column, tableName))
			}
		}
	}

	if len(columns) == 0 {
		return "", nil, nil, fmt.Errorf("没有有效的插入列")
	}

	sql := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		g.escapeIdentifier(tableName),
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "))

	return sql, parameters, warnings, nil
}

// generateUpdateSQL 生成 UPDATE 语句
func (g *SQLGenerator) generateUpdateSQL(ctx context.Context, parsed *ParsedQuery) (string, []interface{}, []string, error) {
	warnings := make([]string, 0)
	parameters := make([]interface{}, 0)

	if len(parsed.Tables) != 1 {
		return "", nil, nil, fmt.Errorf("UPDATE 语句只能操作一个表")
	}

	if len(parsed.Conditions) == 0 {
		return "", nil, nil, fmt.Errorf("UPDATE 语句必须包含 WHERE 条件")
	}

	tableName := parsed.Tables[0]

	// 获取表信息
	tableInfo, err := g.schemaManager.GetTableInfo(tableName)
	if err != nil {
		return "", nil, nil, fmt.Errorf("获取表信息失败: %w", err)
	}

	// 构建 SET 子句
	var setClauses []string
	if values, ok := parsed.Parameters["values"].(map[string]interface{}); ok {
		for column, value := range values {
			if g.columnExists(tableInfo, column) {
				setClauses = append(setClauses, fmt.Sprintf("%s = ?", g.escapeIdentifier(column)))
				parameters = append(parameters, value)
			} else {
				warnings = append(warnings, fmt.Sprintf("列 %s 在表 %s 中不存在", column, tableName))
			}
		}
	}

	if len(setClauses) == 0 {
		return "", nil, nil, fmt.Errorf("没有有效的更新列")
	}

	// 构建 WHERE 子句
	whereClause, whereParams, whereWarnings := g.buildWhereClause(parsed.Conditions)
	parameters = append(parameters, whereParams...)
	warnings = append(warnings, whereWarnings...)

	sql := fmt.Sprintf("UPDATE %s SET %s %s",
		g.escapeIdentifier(tableName),
		strings.Join(setClauses, ", "),
		whereClause)

	return sql, parameters, warnings, nil
}

// generateDeleteSQL 生成 DELETE 语句
func (g *SQLGenerator) generateDeleteSQL(ctx context.Context, parsed *ParsedQuery) (string, []interface{}, []string, error) {
	warnings := make([]string, 0)
	parameters := make([]interface{}, 0)

	if len(parsed.Tables) != 1 {
		return "", nil, nil, fmt.Errorf("DELETE 语句只能操作一个表")
	}

	if len(parsed.Conditions) == 0 {
		return "", nil, nil, fmt.Errorf("DELETE 语句必须包含 WHERE 条件")
	}

	tableName := parsed.Tables[0]

	// 构建 WHERE 子句
	whereClause, whereParams, whereWarnings := g.buildWhereClause(parsed.Conditions)
	parameters = append(parameters, whereParams...)
	warnings = append(warnings, whereWarnings...)

	sql := fmt.Sprintf("DELETE FROM %s %s",
		g.escapeIdentifier(tableName),
		whereClause)

	return sql, parameters, warnings, nil
}

// generateCountSQL 生成 COUNT 语句
func (g *SQLGenerator) generateCountSQL(ctx context.Context, parsed *ParsedQuery) (string, []interface{}, []string, error) {
	warnings := make([]string, 0)
	parameters := make([]interface{}, 0)

	// 构建 FROM 子句
	fromClause, joinWarnings, err := g.buildFromClause(ctx, parsed.Tables)
	if err != nil {
		return "", nil, nil, err
	}
	warnings = append(warnings, joinWarnings...)

	// 构建 WHERE 子句
	whereClause, whereParams, whereWarnings := g.buildWhereClause(parsed.Conditions)
	parameters = append(parameters, whereParams...)
	warnings = append(warnings, whereWarnings...)

	var clauses []string
	clauses = append(clauses, "SELECT COUNT(*)")
	clauses = append(clauses, fromClause)

	if whereClause != "" {
		clauses = append(clauses, whereClause)
	}

	sql := strings.Join(clauses, " ")
	return sql, parameters, warnings, nil
}

// generateAggregateSQL 生成聚合查询语句
func (g *SQLGenerator) generateAggregateSQL(ctx context.Context, parsed *ParsedQuery) (string, []interface{}, []string, error) {
	warnings := make([]string, 0)
	parameters := make([]interface{}, 0)

	// 构建聚合 SELECT 子句
	selectClause := g.buildAggregateSelectClause(parsed)

	// 构建 FROM 子句
	fromClause, joinWarnings, err := g.buildFromClause(ctx, parsed.Tables)
	if err != nil {
		return "", nil, nil, err
	}
	warnings = append(warnings, joinWarnings...)

	// 构建 WHERE 子句
	whereClause, whereParams, whereWarnings := g.buildWhereClause(parsed.Conditions)
	parameters = append(parameters, whereParams...)
	warnings = append(warnings, whereWarnings...)

	var clauses []string
	clauses = append(clauses, selectClause)
	clauses = append(clauses, fromClause)

	if whereClause != "" {
		clauses = append(clauses, whereClause)
	}

	// 处理 GROUP BY
	var groupByColumns []string
	for _, op := range parsed.Operations {
		if op.Type == "group_by" {
			if column, ok := op.Parameters["column"].(string); ok {
				groupByColumns = append(groupByColumns, g.escapeIdentifier(column))
			}
		}
	}

	if len(groupByColumns) > 0 {
		clauses = append(clauses, fmt.Sprintf("GROUP BY %s", strings.Join(groupByColumns, ", ")))
	}

	// 处理其他操作
	for _, op := range parsed.Operations {
		switch op.Type {
		case "order_by":
			if column, ok := op.Parameters["column"].(string); ok {
				direction := "ASC"
				if dir, ok := op.Parameters["direction"].(string); ok {
					direction = dir
				}
				clauses = append(clauses, fmt.Sprintf("ORDER BY %s %s", g.escapeIdentifier(column), direction))
			}
		case "limit":
			if count, ok := op.Parameters["count"].(string); ok {
				clauses = append(clauses, fmt.Sprintf("LIMIT %s", count))
			}
		}
	}

	sql := strings.Join(clauses, " ")
	return sql, parameters, warnings, nil
}

// buildSelectClause 构建 SELECT 子句
func (g *SQLGenerator) buildSelectClause(columns []string) string {
	if len(columns) == 0 || (len(columns) == 1 && columns[0] == "*") {
		return "SELECT *"
	}

	var escapedColumns []string
	for _, column := range columns {
		escapedColumns = append(escapedColumns, g.escapeIdentifier(column))
	}

	return fmt.Sprintf("SELECT %s", strings.Join(escapedColumns, ", "))
}

// buildAggregateSelectClause 构建聚合 SELECT 子句
func (g *SQLGenerator) buildAggregateSelectClause(parsed *ParsedQuery) string {
	var selectParts []string

	// 添加分组列
	for _, op := range parsed.Operations {
		if op.Type == "group_by" {
			if column, ok := op.Parameters["column"].(string); ok {
				selectParts = append(selectParts, g.escapeIdentifier(column))
			}
		}
	}

	// 添加聚合函数
	if len(parsed.Columns) > 0 && parsed.Columns[0] != "*" {
		for _, column := range parsed.Columns {
			// 检查是否包含聚合函数关键词
			if strings.Contains(column, "count") || strings.Contains(column, "计数") {
				selectParts = append(selectParts, "COUNT(*)")
			} else if strings.Contains(column, "sum") || strings.Contains(column, "求和") {
				selectParts = append(selectParts, fmt.Sprintf("SUM(%s)", g.escapeIdentifier(column)))
			} else if strings.Contains(column, "avg") || strings.Contains(column, "平均") {
				selectParts = append(selectParts, fmt.Sprintf("AVG(%s)", g.escapeIdentifier(column)))
			} else if strings.Contains(column, "max") || strings.Contains(column, "最大") {
				selectParts = append(selectParts, fmt.Sprintf("MAX(%s)", g.escapeIdentifier(column)))
			} else if strings.Contains(column, "min") || strings.Contains(column, "最小") {
				selectParts = append(selectParts, fmt.Sprintf("MIN(%s)", g.escapeIdentifier(column)))
			} else {
				selectParts = append(selectParts, g.escapeIdentifier(column))
			}
		}
	} else {
		selectParts = append(selectParts, "COUNT(*)")
	}

	if len(selectParts) == 0 {
		return "SELECT COUNT(*)"
	}

	return fmt.Sprintf("SELECT %s", strings.Join(selectParts, ", "))
}

// buildFromClause 构建 FROM 子句
func (g *SQLGenerator) buildFromClause(ctx context.Context, tables []string) (string, []string, error) {
	warnings := make([]string, 0)

	if len(tables) == 0 {
		return "", nil, fmt.Errorf("没有指定表名")
	}

	if len(tables) == 1 {
		return fmt.Sprintf("FROM %s", g.escapeIdentifier(tables[0])), warnings, nil
	}

	// 多表查询，尝试自动生成 JOIN
	mainTable := tables[0]
	fromClause := fmt.Sprintf("FROM %s", g.escapeIdentifier(mainTable))

	for i := 1; i < len(tables); i++ {
		joinTable := tables[i]
		joinCondition, err := g.findJoinCondition(ctx, mainTable, joinTable)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("无法找到表 %s 和 %s 之间的连接条件", mainTable, joinTable))
			// 使用 CROSS JOIN 作为后备方案
			fromClause += fmt.Sprintf(" CROSS JOIN %s", g.escapeIdentifier(joinTable))
		} else {
			fromClause += fmt.Sprintf(" INNER JOIN %s ON %s", g.escapeIdentifier(joinTable), joinCondition)
		}
	}

	return fromClause, warnings, nil
}

// findJoinCondition 查找表之间的连接条件
func (g *SQLGenerator) findJoinCondition(ctx context.Context, table1, table2 string) (string, error) {
	// 获取表关系
	relationships, err := g.schemaManager.GetRelationships(table1)
	if err != nil {
		return "", err
	}

	// 查找直接关系
	for _, rel := range relationships {
		if rel.ToTable == table2 {
			return fmt.Sprintf("%s.%s = %s.%s",
				g.escapeIdentifier(table1), g.escapeIdentifier(rel.FromColumn),
				g.escapeIdentifier(table2), g.escapeIdentifier(rel.ToColumn)), nil
		}
	}

	// 查找反向关系
	relationships2, err := g.schemaManager.GetRelationships(table2)
	if err != nil {
		return "", err
	}

	for _, rel := range relationships2 {
		if rel.ToTable == table1 {
			return fmt.Sprintf("%s.%s = %s.%s",
				g.escapeIdentifier(table2), g.escapeIdentifier(rel.FromColumn),
				g.escapeIdentifier(table1), g.escapeIdentifier(rel.ToColumn)), nil
		}
	}

	// 尝试通过常见的命名约定推断关系
	table1Info, err1 := g.schemaManager.GetTableInfo(table1)
	table2Info, err2 := g.schemaManager.GetTableInfo(table2)

	if err1 == nil && err2 == nil {
		// 查找外键关系
		for _, fk := range table1Info.ForeignKeys {
			if fk.ReferencedTable == table2 {
				return fmt.Sprintf("%s.%s = %s.%s",
					g.escapeIdentifier(table1), g.escapeIdentifier(fk.Column),
					g.escapeIdentifier(table2), g.escapeIdentifier(fk.ReferencedColumn)), nil
			}
		}

		for _, fk := range table2Info.ForeignKeys {
			if fk.ReferencedTable == table1 {
				return fmt.Sprintf("%s.%s = %s.%s",
					g.escapeIdentifier(table2), g.escapeIdentifier(fk.Column),
					g.escapeIdentifier(table1), g.escapeIdentifier(fk.ReferencedColumn)), nil
			}
		}

		// 尝试通过 ID 字段推断
		if g.columnExists(table1Info, "id") && g.columnExists(table2Info, table1+"_id") {
			return fmt.Sprintf("%s.id = %s.%s_id",
				g.escapeIdentifier(table1),
				g.escapeIdentifier(table2),
				table1), nil
		}

		if g.columnExists(table2Info, "id") && g.columnExists(table1Info, table2+"_id") {
			return fmt.Sprintf("%s.id = %s.%s_id",
				g.escapeIdentifier(table2),
				g.escapeIdentifier(table1),
				table2), nil
		}
	}

	return "", fmt.Errorf("无法找到表 %s 和 %s 之间的连接条件", table1, table2)
}

// buildWhereClause 构建 WHERE 子句
func (g *SQLGenerator) buildWhereClause(conditions []QueryCondition) (string, []interface{}, []string) {
	warnings := make([]string, 0)
	parameters := make([]interface{}, 0)

	if len(conditions) == 0 {
		return "", parameters, warnings
	}

	var whereParts []string
	for _, condition := range conditions {
		if condition.Type != "where" {
			continue
		}

		escapedColumn := g.escapeIdentifier(condition.Column)

		switch condition.Operator {
		case "=", "!=", ">", "<", ">=", "<=":
			whereParts = append(whereParts, fmt.Sprintf("%s %s ?", escapedColumn, condition.Operator))
			parameters = append(parameters, condition.Value)
		case "LIKE":
			whereParts = append(whereParts, fmt.Sprintf("%s LIKE ?", escapedColumn))
			// 为 LIKE 操作添加通配符
			value := fmt.Sprintf("%%%v%%", condition.Value)
			parameters = append(parameters, value)
		default:
			warnings = append(warnings, fmt.Sprintf("不支持的操作符: %s", condition.Operator))
		}
	}

	if len(whereParts) == 0 {
		return "", parameters, warnings
	}

	whereClause := fmt.Sprintf("WHERE %s", strings.Join(whereParts, " AND "))
	return whereClause, parameters, warnings
}

// columnExists 检查列是否存在于表中
func (g *SQLGenerator) columnExists(tableInfo *core.TableInfo, columnName string) bool {
	for _, column := range tableInfo.Columns {
		if column.Name == columnName {
			return true
		}
	}
	return false
}

// escapeIdentifier 转义标识符
func (g *SQLGenerator) escapeIdentifier(identifier string) string {
	// 移除可能的危险字符
	identifier = strings.ReplaceAll(identifier, "`", "")
	identifier = strings.ReplaceAll(identifier, "'", "")
	identifier = strings.ReplaceAll(identifier, "\"", "")
	identifier = strings.ReplaceAll(identifier, ";", "")

	// 使用反引号包围标识符
	return fmt.Sprintf("`%s`", identifier)
}

// validateSQL 验证 SQL 语句的安全性
func (g *SQLGenerator) validateSQL(sql string) error {
	// 检查危险的 SQL 关键词
	dangerousKeywords := []string{
		"DROP", "TRUNCATE", "ALTER", "CREATE", "GRANT", "REVOKE",
		"EXEC", "EXECUTE", "UNION", "SCRIPT", "SHUTDOWN",
	}

	upperSQL := strings.ToUpper(sql)
	for _, keyword := range dangerousKeywords {
		if strings.Contains(upperSQL, keyword) {
			return fmt.Errorf("SQL 包含危险关键词: %s", keyword)
		}
	}

	// 检查 SQL 注入模式
	injectionPatterns := []string{
		`--`,                      // SQL 注释
		`/\*`,                     // 多行注释开始
		`\*/`,                     // 多行注释结束
		`';`,                      // 语句终止符
		`\bOR\b\s+\d+\s*=\s*\d+`,  // OR 注入 (如 OR 1=1)
		`\bAND\b\s+\d+\s*=\s*\d+`, // AND 注入 (如 AND 1=1)
	}

	for _, pattern := range injectionPatterns {
		matched, _ := regexp.MatchString(pattern, upperSQL)
		if matched {
			return fmt.Errorf("SQL 可能包含注入攻击模式: %s", pattern)
		}
	}

	return nil
}

// generateExplanation 生成 SQL 解释
func (g *SQLGenerator) generateExplanation(parsed *ParsedQuery, sql string) string {
	var parts []string

	switch parsed.Intent {
	case IntentSelect:
		parts = append(parts, "查询数据")
	case IntentInsert:
		parts = append(parts, "插入数据")
	case IntentUpdate:
		parts = append(parts, "更新数据")
	case IntentDelete:
		parts = append(parts, "删除数据")
	case IntentCount:
		parts = append(parts, "统计数量")
	case IntentAggregate:
		parts = append(parts, "聚合分析")
	}

	if len(parsed.Tables) > 0 {
		parts = append(parts, fmt.Sprintf("涉及表: %s", strings.Join(parsed.Tables, ", ")))
	}

	if len(parsed.Columns) > 0 && parsed.Columns[0] != "*" {
		parts = append(parts, fmt.Sprintf("查询字段: %s", strings.Join(parsed.Columns, ", ")))
	}

	if len(parsed.Conditions) > 0 {
		parts = append(parts, fmt.Sprintf("包含 %d 个查询条件", len(parsed.Conditions)))
	}

	if len(parsed.Operations) > 0 {
		var operations []string
		for _, op := range parsed.Operations {
			switch op.Type {
			case "order_by":
				operations = append(operations, "排序")
			case "group_by":
				operations = append(operations, "分组")
			case "limit":
				operations = append(operations, "限制结果数量")
			}
		}
		if len(operations) > 0 {
			parts = append(parts, fmt.Sprintf("包含操作: %s", strings.Join(operations, ", ")))
		}
	}

	return strings.Join(parts, "，")
}

// OptimizeSQL 优化 SQL 语句
func (g *SQLGenerator) OptimizeSQL(ctx context.Context, sql string, tables []string) (string, []string, error) {
	var suggestions []string
	optimizedSQL := sql

	// 检查是否使用了索引
	for _, tableName := range tables {
		tableInfo, err := g.schemaManager.GetTableInfo(tableName)
		if err != nil {
			continue
		}

		// 检查 WHERE 子句中是否使用了索引字段
		whereRegex := regexp.MustCompile(`WHERE\s+(.+?)(?:\s+ORDER|\s+GROUP|\s+LIMIT|$)`)
		matches := whereRegex.FindStringSubmatch(strings.ToUpper(sql))
		if len(matches) > 1 {
			whereClause := matches[1]

			hasIndexedCondition := false
			for _, index := range tableInfo.Indexes {
				for _, column := range index.Columns {
					if strings.Contains(strings.ToUpper(whereClause), strings.ToUpper(column)) {
						hasIndexedCondition = true
						break
					}
				}
				if hasIndexedCondition {
					break
				}
			}

			if !hasIndexedCondition {
				suggestions = append(suggestions, fmt.Sprintf("建议在表 %s 的查询条件中使用索引字段", tableName))
			}
		}
	}

	// 检查是否可以添加 LIMIT
	if !strings.Contains(strings.ToUpper(sql), "LIMIT") && strings.HasPrefix(strings.ToUpper(sql), "SELECT") {
		suggestions = append(suggestions, "建议添加 LIMIT 子句以提高查询性能")
	}

	// 检查是否使用了 SELECT *
	if strings.Contains(strings.ToUpper(sql), "SELECT *") {
		suggestions = append(suggestions, "建议明确指定需要的列名而不是使用 SELECT *")
	}

	return optimizedSQL, suggestions, nil
}
