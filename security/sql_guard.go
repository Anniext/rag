// 本文件实现了 SQL 注入防护机制，包括 SQL 语句的安全检查、危险操作检测和查询权限控制。
// 主要功能：
// 1. SQL 语句安全检查和过滤
// 2. 危险操作检测和阻止
// 3. 查询权限细粒度控制
// 4. 参数化查询验证

package security

import (
	"context"
	"fmt"
	"github.com/Anniext/rag/core"
	"regexp"
	"strings"
	"time"
)

// SQLGuard SQL 安全防护器
type SQLGuard struct {
	logger            core.Logger           // 日志记录器
	metrics           core.MetricsCollector // 指标收集器
	dangerousPatterns []*regexp.Regexp      // 危险模式正则表达式
	allowedOperations []string              // 允许的操作类型
	maxQueryLength    int                   // 最大查询长度
	enableStrictMode  bool                  // 是否启用严格模式
	allowedTables     map[string]bool       // 允许访问的表
	blockedTables     map[string]bool       // 禁止访问的表
	maxJoinCount      int                   // 最大JOIN数量
	maxSubqueryDepth  int                   // 最大子查询深度
}

// NewSQLGuard 创建 SQL 安全防护器
func NewSQLGuard(logger core.Logger, metrics core.MetricsCollector) *SQLGuard {
	guard := &SQLGuard{
		logger:            logger,
		metrics:           metrics,
		maxQueryLength:    10000, // 默认最大查询长度
		enableStrictMode:  true,  // 默认启用严格模式
		allowedOperations: []string{"SELECT", "SHOW", "DESCRIBE", "EXPLAIN"},
		allowedTables:     make(map[string]bool),
		blockedTables:     make(map[string]bool),
		maxJoinCount:      5, // 默认最大JOIN数量
		maxSubqueryDepth:  3, // 默认最大子查询深度
	}

	// 初始化危险模式
	guard.initDangerousPatterns()

	// 初始化默认的禁止访问表
	guard.initBlockedTables()

	return guard
}

// initDangerousPatterns 初始化危险 SQL 模式
func (sg *SQLGuard) initDangerousPatterns() {
	dangerousPatterns := []string{
		// SQL 注入常见模式
		`(?i)\b(union\s+select)\b`,             // UNION SELECT 注入
		`(?i)\b(or\s+1\s*=\s*1)\b`,             // OR 1=1 注入
		`(?i)\b(and\s+1\s*=\s*1)\b`,            // AND 1=1 注入
		`(?i)\b(or\s+true)\b`,                  // OR TRUE 注入
		`(?i)\b(and\s+false)\b`,                // AND FALSE 注入
		`(?i)\b(drop\s+table)\b`,               // DROP TABLE
		`(?i)\b(drop\s+database)\b`,            // DROP DATABASE
		`(?i)\b(truncate\s+table)\b`,           // TRUNCATE TABLE
		`(?i)\b(delete\s+from)\b`,              // DELETE FROM
		`(?i)\b(insert\s+into)\b`,              // INSERT INTO
		`(?i)\b(update\s+\w+\s+set)\b`,         // UPDATE SET
		`(?i)\b(alter\s+table)\b`,              // ALTER TABLE
		`(?i)\b(create\s+table)\b`,             // CREATE TABLE
		`(?i)\b(create\s+database)\b`,          // CREATE DATABASE
		`(?i)\b(grant\s+\w+)\b`,                // GRANT 权限
		`(?i)\b(revoke\s+\w+)\b`,               // REVOKE 权限
		`(?i)\b(load_file)\b`,                  // LOAD_FILE 函数
		`(?i)\b(into\s+outfile)\b`,             // INTO OUTFILE
		`(?i)\b(into\s+dumpfile)\b`,            // INTO DUMPFILE
		`(?i)\b(benchmark)\b`,                  // BENCHMARK 函数
		`(?i)\b(sleep)\b`,                      // SLEEP 函数
		`(?i)\b(waitfor\s+delay)\b`,            // WAITFOR DELAY
		`(?i)\b(exec\s*\()\b`,                  // EXEC 执行
		`(?i)\b(execute\s*\()\b`,               // EXECUTE 执行
		`(?i)\b(sp_executesql)\b`,              // SQL Server 动态执行
		`(?i)\b(xp_cmdshell)\b`,                // SQL Server 命令执行
		`(?i)\b(information_schema)\b`,         // 信息模式访问
		`(?i)\b(mysql\.user)\b`,                // MySQL 用户表
		`(?i)\b(pg_user)\b`,                    // PostgreSQL 用户表
		`(?i)\b(sysobjects)\b`,                 // SQL Server 系统对象
		`(?i)\b(syscolumns)\b`,                 // SQL Server 系统列
		`(?i)--`,                               // SQL 注释
		`(?i)/\*.*\*/`,                         // 多行注释
		`(?i)\b(char\s*\(\s*\d+\s*\))\b`,       // CHAR 函数注入
		`(?i)\b(ascii\s*\(\s*substr)\b`,        // ASCII SUBSTR 注入
		`(?i)\b(hex\s*\()\b`,                   // HEX 函数
		`(?i)\b(unhex\s*\()\b`,                 // UNHEX 函数
		`(?i)\b(concat\s*\(.*select)\b`,        // CONCAT SELECT 注入
		`(?i)\b(group_concat\s*\(.*select)\b`,  // GROUP_CONCAT SELECT 注入
		`(?i)\b(if\s*\(.*select)\b`,            // IF SELECT 注入
		`(?i)\b(case\s+when.*select)\b`,        // CASE WHEN SELECT 注入
		`(?i)\b(extractvalue\s*\()\b`,          // EXTRACTVALUE 函数
		`(?i)\b(updatexml\s*\()\b`,             // UPDATEXML 函数
		`(?i)\b(geometrycollection\s*\()\b`,    // GEOMETRYCOLLECTION 函数
		`(?i)\b(polygon\s*\()\b`,               // POLYGON 函数
		`(?i)\b(multipoint\s*\()\b`,            // MULTIPOINT 函数
		`(?i)\b(linestring\s*\()\b`,            // LINESTRING 函数
		`(?i)\b(exp\s*\()\b`,                   // EXP 函数注入
		`(?i)\b(floor\s*\(.*rand)\b`,           // FLOOR RAND 注入
		`(?i)\b(count\s*\(\s*\*\s*\).*from)\b`, // COUNT(*) FROM 注入
		`(?i)\b(having\s+\d+\s*=\s*\d+)\b`,     // HAVING 注入
		`(?i)\b(order\s+by\s+\d+)\b`,           // ORDER BY 数字注入
		`(?i)\b(limit\s+\d+\s*,\s*\d+)\b`,      // LIMIT 注入检测
		`(?i)\b(procedure\s+analyse)\b`,        // PROCEDURE ANALYSE
		`(?i)\b(into\s+@)\b`,                   // INTO 变量注入
		`(?i)@\w+`,                             // 变量注入
		`(?i)\$\w+`,                            // 变量注入
		`(?i)%\w+%`,                            // 通配符注入
	}

	sg.dangerousPatterns = make([]*regexp.Regexp, 0, len(dangerousPatterns))
	for _, pattern := range dangerousPatterns {
		if regex, err := regexp.Compile(pattern); err == nil {
			sg.dangerousPatterns = append(sg.dangerousPatterns, regex)
		}
	}
}

// ValidateSQL 验证 SQL 语句的安全性
func (sg *SQLGuard) ValidateSQL(ctx context.Context, sql string) error {
	// 记录验证开始时间
	startTime := time.Now()
	defer func() {
		sg.metrics.RecordHistogram("sql_guard_validation_duration",
			float64(time.Since(startTime).Milliseconds()),
			map[string]string{"operation": "validate_sql"})
	}()

	// 基本检查
	if err := sg.basicValidation(sql); err != nil {
		sg.metrics.IncrementCounter("sql_guard_validation_errors",
			map[string]string{"error": "basic_validation", "reason": err.Error()})
		return err
	}

	// 危险模式检查
	if err := sg.checkDangerousPatterns(sql); err != nil {
		sg.metrics.IncrementCounter("sql_guard_validation_errors",
			map[string]string{"error": "dangerous_pattern", "reason": err.Error()})
		return err
	}

	// 操作类型检查
	if err := sg.checkAllowedOperations(sql); err != nil {
		sg.metrics.IncrementCounter("sql_guard_validation_errors",
			map[string]string{"error": "operation_not_allowed", "reason": err.Error()})
		return err
	}

	// 结构检查
	if err := sg.checkSQLStructure(sql); err != nil {
		sg.metrics.IncrementCounter("sql_guard_validation_errors",
			map[string]string{"error": "structure_check", "reason": err.Error()})
		return err
	}

	sg.metrics.IncrementCounter("sql_guard_validation_success",
		map[string]string{"operation": "validate_sql"})

	return nil
}

// basicValidation 基本验证
func (sg *SQLGuard) basicValidation(sql string) error {
	// 检查是否为空
	if strings.TrimSpace(sql) == "" {
		return fmt.Errorf("SQL 语句不能为空")
	}

	// 检查长度
	if len(sql) > sg.maxQueryLength {
		return fmt.Errorf("SQL 语句长度超过限制 (%d 字符)", sg.maxQueryLength)
	}

	// 检查是否包含空字节
	if strings.Contains(sql, "\x00") {
		return fmt.Errorf("SQL 语句包含非法字符")
	}

	return nil
}

// checkDangerousPatterns 检查危险模式
func (sg *SQLGuard) checkDangerousPatterns(sql string) error {
	for _, pattern := range sg.dangerousPatterns {
		if pattern.MatchString(sql) {
			match := pattern.FindString(sql)
			sg.logger.Warn("检测到危险 SQL 模式",
				"pattern", pattern.String(),
				"match", match,
				"sql", sql)
			return fmt.Errorf("SQL 语句包含危险模式: %s", match)
		}
	}
	return nil
}

// checkAllowedOperations 检查允许的操作类型
func (sg *SQLGuard) checkAllowedOperations(sql string) error {
	// 提取 SQL 操作类型
	operation := sg.extractSQLOperation(sql)
	if operation == "" {
		return fmt.Errorf("无法识别 SQL 操作类型")
	}

	// 检查是否在允许列表中
	for _, allowed := range sg.allowedOperations {
		if strings.EqualFold(operation, allowed) {
			return nil
		}
	}

	return fmt.Errorf("不允许的 SQL 操作类型: %s", operation)
}

// extractSQLOperation 提取 SQL 操作类型
func (sg *SQLGuard) extractSQLOperation(sql string) string {
	// 移除前导空白和注释
	sql = strings.TrimSpace(sql)
	sql = sg.removeComments(sql)

	// 提取第一个单词作为操作类型
	words := strings.Fields(sql)
	if len(words) > 0 {
		return strings.ToUpper(words[0])
	}

	return ""
}

// removeComments 移除 SQL 注释
func (sg *SQLGuard) removeComments(sql string) string {
	// 移除单行注释 --
	lines := strings.Split(sql, "\n")
	var cleanLines []string
	for _, line := range lines {
		if idx := strings.Index(line, "--"); idx != -1 {
			line = line[:idx]
		}
		cleanLines = append(cleanLines, line)
	}
	sql = strings.Join(cleanLines, "\n")

	// 移除多行注释 /* */
	commentRegex := regexp.MustCompile(`/\*.*?\*/`)
	sql = commentRegex.ReplaceAllString(sql, "")

	return sql
}

// checkSQLStructure 检查 SQL 结构
func (sg *SQLGuard) checkSQLStructure(sql string) error {
	// 检查括号匹配
	if err := sg.checkParenthesesBalance(sql); err != nil {
		return err
	}

	// 检查引号匹配
	if err := sg.checkQuotesBalance(sql); err != nil {
		return err
	}

	// 检查是否有多个语句（防止语句注入）
	if sg.enableStrictMode {
		if err := sg.checkMultipleStatements(sql); err != nil {
			return err
		}
	}

	return nil
}

// checkParenthesesBalance 检查括号平衡
func (sg *SQLGuard) checkParenthesesBalance(sql string) error {
	count := 0
	for _, char := range sql {
		switch char {
		case '(':
			count++
		case ')':
			count--
			if count < 0 {
				return fmt.Errorf("SQL 语句括号不匹配")
			}
		}
	}

	if count != 0 {
		return fmt.Errorf("SQL 语句括号不匹配")
	}

	return nil
}

// checkQuotesBalance 检查引号平衡
func (sg *SQLGuard) checkQuotesBalance(sql string) error {
	singleQuoteCount := 0
	doubleQuoteCount := 0
	backQuoteCount := 0

	inSingleQuote := false
	inDoubleQuote := false
	inBackQuote := false

	for i, char := range sql {
		switch char {
		case '\'':
			if !inDoubleQuote && !inBackQuote {
				if i > 0 && sql[i-1] == '\\' {
					continue // 转义字符
				}
				inSingleQuote = !inSingleQuote
				singleQuoteCount++
			}
		case '"':
			if !inSingleQuote && !inBackQuote {
				if i > 0 && sql[i-1] == '\\' {
					continue // 转义字符
				}
				inDoubleQuote = !inDoubleQuote
				doubleQuoteCount++
			}
		case '`':
			if !inSingleQuote && !inDoubleQuote {
				inBackQuote = !inBackQuote
				backQuoteCount++
			}
		}
	}

	if singleQuoteCount%2 != 0 {
		return fmt.Errorf("SQL 语句单引号不匹配")
	}

	if doubleQuoteCount%2 != 0 {
		return fmt.Errorf("SQL 语句双引号不匹配")
	}

	if backQuoteCount%2 != 0 {
		return fmt.Errorf("SQL 语句反引号不匹配")
	}

	return nil
}

// checkMultipleStatements 检查多语句
func (sg *SQLGuard) checkMultipleStatements(sql string) error {
	// 移除字符串内容，避免误判
	cleanSQL := sg.removeStringLiterals(sql)

	// 检查分号数量
	semicolonCount := strings.Count(cleanSQL, ";")

	// 允许最后一个分号
	cleanSQL = strings.TrimSpace(cleanSQL)
	if strings.HasSuffix(cleanSQL, ";") {
		semicolonCount--
	}

	if semicolonCount > 0 {
		return fmt.Errorf("不允许执行多个 SQL 语句")
	}

	return nil
}

// removeStringLiterals 移除字符串字面量
func (sg *SQLGuard) removeStringLiterals(sql string) string {
	var result strings.Builder
	inSingleQuote := false
	inDoubleQuote := false
	inBackQuote := false

	for i, char := range sql {
		switch char {
		case '\'':
			if !inDoubleQuote && !inBackQuote {
				if i > 0 && sql[i-1] == '\\' {
					continue // 转义字符
				}
				inSingleQuote = !inSingleQuote
			}
			if !inSingleQuote {
				result.WriteRune(char)
			}
		case '"':
			if !inSingleQuote && !inBackQuote {
				if i > 0 && sql[i-1] == '\\' {
					continue // 转义字符
				}
				inDoubleQuote = !inDoubleQuote
			}
			if !inDoubleQuote {
				result.WriteRune(char)
			}
		case '`':
			if !inSingleQuote && !inDoubleQuote {
				inBackQuote = !inBackQuote
			}
			if !inBackQuote {
				result.WriteRune(char)
			}
		default:
			if !inSingleQuote && !inDoubleQuote && !inBackQuote {
				result.WriteRune(char)
			}
		}
	}

	return result.String()
}

// CheckQueryPermission 检查查询权限
func (sg *SQLGuard) CheckQueryPermission(ctx context.Context, user *core.UserInfo, sql string, tables []string) error {
	if user == nil {
		return fmt.Errorf("用户信息不能为空")
	}

	// 提取 SQL 操作类型
	operation := sg.extractSQLOperation(sql)

	// 检查每个表的权限
	for _, table := range tables {
		resource := fmt.Sprintf("table:%s", table)
		action := strings.ToLower(operation)

		// 检查用户是否有该表的操作权限
		hasPermission := false
		for _, perm := range user.Permissions {
			if perm == "*" ||
				perm == fmt.Sprintf("%s:%s", resource, action) ||
				perm == fmt.Sprintf("%s:*", resource) ||
				perm == fmt.Sprintf("table:*:%s", action) {
				hasPermission = true
				break
			}
		}

		if !hasPermission {
			sg.logger.Warn("用户缺少表操作权限",
				"user_id", user.ID,
				"table", table,
				"operation", operation)
			return fmt.Errorf("用户 %s 没有权限对表 %s 执行 %s 操作", user.Username, table, operation)
		}
	}

	return nil
}

// SanitizeSQL 清理 SQL 语句
func (sg *SQLGuard) SanitizeSQL(sql string) string {
	// 移除注释
	sql = sg.removeComments(sql)

	// 移除多余的空白字符
	sql = regexp.MustCompile(`\s+`).ReplaceAllString(strings.TrimSpace(sql), " ")

	return sql
}

// SetMaxQueryLength 设置最大查询长度
func (sg *SQLGuard) SetMaxQueryLength(length int) {
	sg.maxQueryLength = length
}

// SetStrictMode 设置严格模式
func (sg *SQLGuard) SetStrictMode(enabled bool) {
	sg.enableStrictMode = enabled
}

// AddAllowedOperation 添加允许的操作类型
func (sg *SQLGuard) AddAllowedOperation(operation string) {
	operation = strings.ToUpper(operation)
	for _, existing := range sg.allowedOperations {
		if existing == operation {
			return // 已存在
		}
	}
	sg.allowedOperations = append(sg.allowedOperations, operation)
}

// RemoveAllowedOperation 移除允许的操作类型
func (sg *SQLGuard) RemoveAllowedOperation(operation string) {
	operation = strings.ToUpper(operation)
	for i, existing := range sg.allowedOperations {
		if existing == operation {
			sg.allowedOperations = append(sg.allowedOperations[:i], sg.allowedOperations[i+1:]...)
			break
		}
	}
}

// initBlockedTables 初始化默认的禁止访问表
func (sg *SQLGuard) initBlockedTables() {
	// 系统敏感表
	systemTables := []string{
		"mysql.user",
		"mysql.db",
		"mysql.tables_priv",
		"mysql.columns_priv",
		"information_schema.user_privileges",
		"information_schema.schema_privileges",
		"information_schema.table_privileges",
		"information_schema.column_privileges",
		"performance_schema.users",
		"sys.user_summary",
		"sys.user_summary_by_file_io",
		"sys.user_summary_by_statement_type",
	}

	for _, table := range systemTables {
		sg.blockedTables[table] = true
	}
}

// ValidateTables 验证查询中涉及的表是否允许访问
func (sg *SQLGuard) ValidateTables(ctx context.Context, tables []string) error {
	for _, table := range tables {
		// 检查是否在禁止列表中
		if sg.blockedTables[table] {
			sg.logger.Warn("尝试访问禁止的表", "table", table)
			return fmt.Errorf("禁止访问表: %s", table)
		}

		// 如果设置了允许列表，检查表是否在允许列表中
		if len(sg.allowedTables) > 0 && !sg.allowedTables[table] {
			sg.logger.Warn("尝试访问未授权的表", "table", table)
			return fmt.Errorf("未授权访问表: %s", table)
		}
	}

	return nil
}

// CheckQueryComplexity 检查查询复杂度
func (sg *SQLGuard) CheckQueryComplexity(ctx context.Context, sql string) error {
	// 检查JOIN数量
	joinCount := sg.countJoins(sql)
	if joinCount > sg.maxJoinCount {
		return fmt.Errorf("查询包含过多的JOIN操作 (%d > %d)", joinCount, sg.maxJoinCount)
	}

	// 检查子查询深度
	subqueryDepth := sg.countSubqueryDepth(sql)
	if subqueryDepth > sg.maxSubqueryDepth {
		return fmt.Errorf("子查询嵌套过深 (%d > %d)", subqueryDepth, sg.maxSubqueryDepth)
	}

	return nil
}

// countJoins 计算SQL中的JOIN数量
func (sg *SQLGuard) countJoins(sql string) int {
	joinPattern := regexp.MustCompile(`(?i)\b(inner\s+join|left\s+join|right\s+join|full\s+join|cross\s+join|join)\b`)
	matches := joinPattern.FindAllString(sql, -1)
	return len(matches)
}

// countSubqueryDepth 计算子查询深度
func (sg *SQLGuard) countSubqueryDepth(sql string) int {
	maxDepth := 0
	currentDepth := 0
	inString := false
	stringChar := byte(0)

	for i := 0; i < len(sql); i++ {
		char := sql[i]

		// 处理字符串
		if !inString && (char == '\'' || char == '"') {
			inString = true
			stringChar = char
			continue
		}
		if inString && char == stringChar {
			inString = false
			continue
		}
		if inString {
			continue
		}

		// 检查SELECT关键字
		if i <= len(sql)-6 {
			substr := strings.ToUpper(sql[i : i+6])
			if substr == "SELECT" {
				// 检查前面是否有开括号
				j := i - 1
				for j >= 0 && (sql[j] == ' ' || sql[j] == '\t' || sql[j] == '\n') {
					j--
				}
				if j >= 0 && sql[j] == '(' {
					currentDepth++
					if currentDepth > maxDepth {
						maxDepth = currentDepth
					}
				}
			}
		}

		// 检查括号平衡
		if char == '(' {
			// 这里不增加深度，因为我们在SELECT时已经处理了
		} else if char == ')' {
			if currentDepth > 0 {
				currentDepth--
			}
		}
	}

	return maxDepth
}

// ValidateParameterizedQuery 验证参数化查询
func (sg *SQLGuard) ValidateParameterizedQuery(ctx context.Context, sql string, params []any) error {
	// 检查参数数量
	placeholderCount := strings.Count(sql, "?")
	if placeholderCount != len(params) {
		return fmt.Errorf("参数数量不匹配: 期望 %d 个，实际 %d 个", placeholderCount, len(params))
	}

	// 检查参数类型安全
	for i, param := range params {
		if err := sg.validateParameter(param); err != nil {
			return fmt.Errorf("参数 %d 验证失败: %w", i+1, err)
		}
	}

	return nil
}

// validateParameter 验证单个参数
func (sg *SQLGuard) validateParameter(param any) error {
	if param == nil {
		return nil
	}

	switch v := param.(type) {
	case string:
		// 检查字符串参数是否包含危险内容
		for _, pattern := range sg.dangerousPatterns {
			if pattern.MatchString(v) {
				return fmt.Errorf("参数包含危险模式: %s", v)
			}
		}
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		// 数字类型是安全的
		return nil
	case float32, float64:
		// 浮点数类型是安全的
		return nil
	case bool:
		// 布尔类型是安全的
		return nil
	default:
		// 其他类型需要转换为字符串检查
		str := fmt.Sprintf("%v", v)
		return sg.validateParameter(str)
	}

	return nil
}

// AddAllowedTable 添加允许访问的表
func (sg *SQLGuard) AddAllowedTable(tableName string) {
	sg.allowedTables[tableName] = true
}

// RemoveAllowedTable 移除允许访问的表
func (sg *SQLGuard) RemoveAllowedTable(tableName string) {
	delete(sg.allowedTables, tableName)
}

// AddBlockedTable 添加禁止访问的表
func (sg *SQLGuard) AddBlockedTable(tableName string) {
	sg.blockedTables[tableName] = true
}

// RemoveBlockedTable 移除禁止访问的表
func (sg *SQLGuard) RemoveBlockedTable(tableName string) {
	delete(sg.blockedTables, tableName)
}

// SetMaxJoinCount 设置最大JOIN数量
func (sg *SQLGuard) SetMaxJoinCount(count int) {
	sg.maxJoinCount = count
}

// SetMaxSubqueryDepth 设置最大子查询深度
func (sg *SQLGuard) SetMaxSubqueryDepth(depth int) {
	sg.maxSubqueryDepth = depth
}

// GetAllowedTables 获取允许访问的表列表
func (sg *SQLGuard) GetAllowedTables() []string {
	tables := make([]string, 0, len(sg.allowedTables))
	for table := range sg.allowedTables {
		tables = append(tables, table)
	}
	return tables
}

// GetBlockedTables 获取禁止访问的表列表
func (sg *SQLGuard) GetBlockedTables() []string {
	tables := make([]string, 0, len(sg.blockedTables))
	for table := range sg.blockedTables {
		tables = append(tables, table)
	}
	return tables
}

// ExtractTables 从SQL语句中提取表名
func (sg *SQLGuard) ExtractTables(sql string) []string {
	// 简化的表名提取逻辑
	// 实际实现中应该使用SQL解析器
	tables := make([]string, 0)

	// 移除注释和字符串字面量
	cleanSQL := sg.removeStringLiterals(sql)
	cleanSQL = sg.removeComments(cleanSQL)

	// 查找FROM和JOIN子句中的表名
	fromPattern := regexp.MustCompile(`(?i)\b(?:from|join)\s+([a-zA-Z_][a-zA-Z0-9_]*(?:\.[a-zA-Z_][a-zA-Z0-9_]*)?)\b`)
	matches := fromPattern.FindAllStringSubmatch(cleanSQL, -1)

	for _, match := range matches {
		if len(match) > 1 {
			tableName := strings.TrimSpace(match[1])
			if tableName != "" {
				tables = append(tables, tableName)
			}
		}
	}

	return tables
}

// ValidateWithTables 验证SQL语句并检查表访问权限
func (sg *SQLGuard) ValidateWithTables(ctx context.Context, sql string) error {
	// 基本SQL验证
	if err := sg.ValidateSQL(ctx, sql); err != nil {
		return err
	}

	// 提取表名
	tables := sg.ExtractTables(sql)

	// 验证表访问权限
	if err := sg.ValidateTables(ctx, tables); err != nil {
		return err
	}

	// 检查查询复杂度
	if err := sg.CheckQueryComplexity(ctx, sql); err != nil {
		return err
	}

	return nil
}
