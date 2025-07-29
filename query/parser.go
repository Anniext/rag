package query

import (
	"context"
	"fmt"
	"github.com/Anniext/rag/core"
	"regexp"
	"strings"
	"time"
)

// QueryParser 查询解析器结构体
type QueryParser struct {
	schemaManager core.SchemaManager
	logger        core.Logger
}

// NewQueryParser 创建新的查询解析器
func NewQueryParser(schemaManager core.SchemaManager, logger core.Logger) *QueryParser {
	return &QueryParser{
		schemaManager: schemaManager,
		logger:        logger,
	}
}

// QueryIntent 查询意图枚举
type QueryIntent string

const (
	IntentSelect    QueryIntent = "select"    // 查询数据
	IntentInsert    QueryIntent = "insert"    // 插入数据
	IntentUpdate    QueryIntent = "update"    // 更新数据
	IntentDelete    QueryIntent = "delete"    // 删除数据
	IntentAnalyze   QueryIntent = "analyze"   // 数据分析
	IntentCount     QueryIntent = "count"     // 计数查询
	IntentAggregate QueryIntent = "aggregate" // 聚合查询
	IntentUnknown   QueryIntent = "unknown"   // 未知意图
)

// ParsedQuery 解析后的查询结构
type ParsedQuery struct {
	Intent      QueryIntent            `json:"intent"`
	Tables      []string               `json:"tables"`
	Columns     []string               `json:"columns"`
	Conditions  []QueryCondition       `json:"conditions"`
	Operations  []QueryOperation       `json:"operations"`
	Parameters  map[string]interface{} `json:"parameters"`
	Complexity  string                 `json:"complexity"`
	Confidence  float64                `json:"confidence"`
	Suggestions []string               `json:"suggestions"`
}

// QueryCondition 查询条件结构
type QueryCondition struct {
	Column   string      `json:"column"`
	Operator string      `json:"operator"`
	Value    interface{} `json:"value"`
	Type     string      `json:"type"` // where, having, join
}

// QueryOperation 查询操作结构
type QueryOperation struct {
	Type       string                 `json:"type"` // join, group_by, order_by, limit
	Parameters map[string]interface{} `json:"parameters"`
}

// ParseQuery 解析自然语言查询
func (p *QueryParser) ParseQuery(ctx context.Context, query string) (*ParsedQuery, error) {
	p.logger.Debug("开始解析查询", "query", query)

	// 预处理查询文本
	normalizedQuery := p.preprocessQuery(query)

	// 识别查询意图
	intent := p.identifyIntent(normalizedQuery)

	// 提取表名
	tables, err := p.extractTables(ctx, normalizedQuery)
	if err != nil {
		p.logger.Error("提取表名失败", "error", err)
		return nil, fmt.Errorf("提取表名失败: %w", err)
	}

	// 提取列名
	columns, err := p.extractColumns(ctx, normalizedQuery, tables)
	if err != nil {
		p.logger.Error("提取列名失败", "error", err)
		return nil, fmt.Errorf("提取列名失败: %w", err)
	}

	// 提取查询条件
	conditions := p.extractConditions(normalizedQuery)

	// 提取查询操作
	operations := p.extractOperations(normalizedQuery)

	// 提取参数
	parameters := p.extractParameters(normalizedQuery)

	// 评估查询复杂度
	complexity := p.evaluateComplexity(intent, tables, columns, conditions, operations)

	// 计算置信度
	confidence := p.calculateConfidence(intent, tables, columns)

	// 生成建议
	suggestions := p.generateSuggestions(ctx, intent, tables, columns, conditions)

	parsed := &ParsedQuery{
		Intent:      intent,
		Tables:      tables,
		Columns:     columns,
		Conditions:  conditions,
		Operations:  operations,
		Parameters:  parameters,
		Complexity:  complexity,
		Confidence:  confidence,
		Suggestions: suggestions,
	}

	p.logger.Debug("查询解析完成", "parsed", parsed)
	return parsed, nil
}

// preprocessQuery 预处理查询文本
func (p *QueryParser) preprocessQuery(query string) string {
	// 转换为小写
	normalized := strings.ToLower(strings.TrimSpace(query))

	// 移除多余的空格
	spaceRegex := regexp.MustCompile(`\s+`)
	normalized = spaceRegex.ReplaceAllString(normalized, " ")

	// 标准化常见的查询词汇
	replacements := map[string]string{
		"显示":  "查询",
		"展示":  "查询",
		"获取":  "查询",
		"找到":  "查询",
		"搜索":  "查询",
		"列出":  "查询",
		"统计":  "计数",
		"数量":  "计数",
		"总数":  "计数",
		"求和":  "聚合",
		"平均":  "聚合",
		"最大":  "聚合",
		"最小":  "聚合",
		"分组":  "聚合",
		"按照":  "按",
		"根据":  "按",
		"排序":  "排序",
		"升序":  "asc",
		"降序":  "desc",
		"大于":  ">",
		"小于":  "<",
		"等于":  "=",
		"不等于": "!=",
		"包含":  "like",
		"匹配":  "like",
	}

	for old, new := range replacements {
		normalized = strings.ReplaceAll(normalized, old, new)
	}

	return normalized
}

// identifyIntent 识别查询意图
func (p *QueryParser) identifyIntent(query string) QueryIntent {
	// 优先级更高的聚合关键词模式
	aggregatePatterns := []string{
		"分组.*统计",
		"按.*分组",
		"group.*by",
	}

	// 检查聚合模式
	for _, pattern := range aggregatePatterns {
		matched, _ := regexp.MatchString(pattern, query)
		if matched {
			return IntentAggregate
		}
	}

	// 查询意图关键词映射
	intentKeywords := map[QueryIntent][]string{
		IntentSelect:    {"查询", "select", "显示", "展示", "获取", "找", "搜索", "列出"},
		IntentInsert:    {"插入", "insert", "添加", "新增", "创建"},
		IntentUpdate:    {"更新", "update", "修改", "改变", "设置"},
		IntentDelete:    {"删除", "delete", "移除", "清除"},
		IntentCount:     {"计数", "count", "数量", "总数", "多少"},
		IntentAnalyze:   {"分析", "analyze", "统计分析", "数据分析"},
		IntentAggregate: {"聚合", "aggregate", "求和", "平均", "最大", "最小"},
	}

	// 计算每种意图的匹配分数
	scores := make(map[QueryIntent]int)

	for intent, keywords := range intentKeywords {
		for _, keyword := range keywords {
			if strings.Contains(query, keyword) {
				scores[intent]++
			}
		}
	}

	// 找到得分最高的意图
	maxScore := 0
	bestIntent := IntentUnknown

	for intent, score := range scores {
		if score > maxScore {
			maxScore = score
			bestIntent = intent
		}
	}

	// 如果没有明确的意图匹配，默认为查询
	if bestIntent == IntentUnknown && maxScore == 0 {
		bestIntent = IntentSelect
	}

	return bestIntent
}

// extractTables 提取表名
func (p *QueryParser) extractTables(ctx context.Context, query string) ([]string, error) {
	var tables []string

	// 直接表名匹配模式
	tablePatterns := []string{
		`从\s+(\w+)`,
		`表\s+(\w+)`,
		`in\s+(\w+)`,
		`from\s+(\w+)`,
		`table\s+(\w+)`,
	}

	for _, pattern := range tablePatterns {
		regex := regexp.MustCompile(pattern)
		matches := regex.FindAllStringSubmatch(query, -1)
		for _, match := range matches {
			if len(match) > 1 {
				tableName := strings.TrimSpace(match[1])
				if !contains(tables, tableName) {
					tables = append(tables, tableName)
				}
			}
		}
	}

	// 如果没有直接匹配到表名，尝试通过相似性匹配
	if len(tables) == 0 {
		similarTables, err := p.findSimilarTables(ctx, query)
		if err != nil {
			return nil, err
		}
		tables = append(tables, similarTables...)
	}

	return tables, nil
}

// findSimilarTables 通过相似性查找表名
func (p *QueryParser) findSimilarTables(ctx context.Context, query string) ([]string, error) {
	// 使用 schema manager 的相似性搜索功能
	similarTables, err := p.schemaManager.FindSimilarTables(query)
	if err != nil {
		return nil, err
	}

	var tableNames []string
	for _, table := range similarTables {
		tableNames = append(tableNames, table.Name)
	}

	return tableNames, nil
}

// extractColumns 提取列名
func (p *QueryParser) extractColumns(ctx context.Context, query string, tables []string) ([]string, error) {
	var columns []string

	// 列名匹配模式
	columnPatterns := []string{
		`选择\s+(\w+)`,
		`查询\s+(\w+)`,
		`select\s+(\w+)`,
		`字段\s+(\w+)`,
		`列\s+(\w+)`,
	}

	for _, pattern := range columnPatterns {
		regex := regexp.MustCompile(pattern)
		matches := regex.FindAllStringSubmatch(query, -1)
		for _, match := range matches {
			if len(match) > 1 {
				columnName := strings.TrimSpace(match[1])
				if !contains(columns, columnName) {
					columns = append(columns, columnName)
				}
			}
		}
	}

	// 如果没有明确指定列名，尝试从表结构中推断
	if len(columns) == 0 && len(tables) > 0 {
		inferredColumns, err := p.inferColumns(ctx, query, tables)
		if err != nil {
			return nil, err
		}
		columns = append(columns, inferredColumns...)
	}

	// 如果仍然没有列名，默认为 *
	if len(columns) == 0 {
		columns = append(columns, "*")
	}

	return columns, nil
}

// inferColumns 从表结构推断列名
func (p *QueryParser) inferColumns(ctx context.Context, query string, tables []string) ([]string, error) {
	var columns []string

	for _, tableName := range tables {
		tableInfo, err := p.schemaManager.GetTableInfo(tableName)
		if err != nil {
			continue
		}

		// 检查查询中是否提到了特定的列名
		for _, column := range tableInfo.Columns {
			if strings.Contains(query, column.Name) ||
				strings.Contains(query, column.Comment) {
				if !contains(columns, column.Name) {
					columns = append(columns, column.Name)
				}
			}
		}
	}

	return columns, nil
}

// extractConditions 提取查询条件
func (p *QueryParser) extractConditions(query string) []QueryCondition {
	var conditions []QueryCondition

	// 条件匹配模式 (支持中文字符)
	conditionPatterns := []struct {
		pattern  string
		operator string
	}{
		{`([^\s]+)\s*=\s*['"]?([^'"]+)['"]?`, "="},
		{`([^\s]+)\s*>\s*(\d+)`, ">"},
		{`([^\s]+)\s*<\s*(\d+)`, "<"},
		{`([^\s]+)\s*>=\s*(\d+)`, ">="},
		{`([^\s]+)\s*<=\s*(\d+)`, "<="},
		{`([^\s]+)\s*!=\s*['"]?([^'"]+)['"]?`, "!="},
		{`([^\s]+)\s*like\s*['"]?([^'"]+)['"]?`, "LIKE"},
		{`([^\s]+)\s*包含\s*['"]?([^'"]+)['"]?`, "LIKE"},
	}

	for _, cp := range conditionPatterns {
		regex := regexp.MustCompile(cp.pattern)
		matches := regex.FindAllStringSubmatch(query, -1)
		for _, match := range matches {
			if len(match) > 2 {
				condition := QueryCondition{
					Column:   strings.TrimSpace(match[1]),
					Operator: cp.operator,
					Value:    strings.TrimSpace(match[2]),
					Type:     "where",
				}
				conditions = append(conditions, condition)
			}
		}
	}

	return conditions
}

// extractOperations 提取查询操作
func (p *QueryParser) extractOperations(query string) []QueryOperation {
	var operations []QueryOperation

	// 排序操作
	if strings.Contains(query, "排序") || strings.Contains(query, "order") {
		orderRegex := regexp.MustCompile(`按\s*(\w+)\s*(asc|desc|升序|降序)?`)
		matches := orderRegex.FindAllStringSubmatch(query, -1)
		for _, match := range matches {
			params := map[string]interface{}{
				"column": match[1],
			}
			if len(match) > 2 && match[2] != "" {
				direction := match[2]
				if direction == "降序" || direction == "desc" {
					params["direction"] = "DESC"
				} else {
					params["direction"] = "ASC"
				}
			}
			operations = append(operations, QueryOperation{
				Type:       "order_by",
				Parameters: params,
			})
		}
	}

	// 限制操作
	limitRegex := regexp.MustCompile(`限制\s*(\d+)|limit\s*(\d+)|前\s*(\d+)`)
	matches := limitRegex.FindAllStringSubmatch(query, -1)
	for _, match := range matches {
		var limit string
		for i := 1; i < len(match); i++ {
			if match[i] != "" {
				limit = match[i]
				break
			}
		}
		if limit != "" {
			operations = append(operations, QueryOperation{
				Type: "limit",
				Parameters: map[string]interface{}{
					"count": limit,
				},
			})
		}
	}

	// 分组操作
	if strings.Contains(query, "分组") || strings.Contains(query, "group") {
		groupRegex := regexp.MustCompile(`按\s*(\w+)\s*分组|group\s*by\s*(\w+)`)
		matches := groupRegex.FindAllStringSubmatch(query, -1)
		for _, match := range matches {
			var column string
			for i := 1; i < len(match); i++ {
				if match[i] != "" {
					column = match[i]
					break
				}
			}
			if column != "" {
				operations = append(operations, QueryOperation{
					Type: "group_by",
					Parameters: map[string]interface{}{
						"column": column,
					},
				})
			}
		}
	}

	return operations
}

// extractParameters 提取查询参数
func (p *QueryParser) extractParameters(query string) map[string]interface{} {
	parameters := make(map[string]interface{})

	// 提取数字参数
	numberRegex := regexp.MustCompile(`(\d+)`)
	numbers := numberRegex.FindAllString(query, -1)
	if len(numbers) > 0 {
		parameters["numbers"] = numbers
	}

	// 提取日期参数
	dateRegex := regexp.MustCompile(`(\d{4}-\d{2}-\d{2})`)
	dates := dateRegex.FindAllString(query, -1)
	if len(dates) > 0 {
		parameters["dates"] = dates
	}

	// 提取字符串参数
	stringRegex := regexp.MustCompile(`['"]([^'"]+)['"]`)
	matches := stringRegex.FindAllStringSubmatch(query, -1)
	var strings []string
	for _, match := range matches {
		if len(match) > 1 {
			strings = append(strings, match[1])
		}
	}
	if len(strings) > 0 {
		parameters["strings"] = strings
	}

	return parameters
}

// evaluateComplexity 评估查询复杂度
func (p *QueryParser) evaluateComplexity(intent QueryIntent, tables []string, columns []string, conditions []QueryCondition, operations []QueryOperation) string {
	score := 0

	// 基于表数量
	if len(tables) > 3 {
		score += 3
	} else if len(tables) > 1 {
		score += 2
	} else {
		score += 1
	}

	// 基于列数量
	if len(columns) > 10 {
		score += 2
	} else if len(columns) > 5 {
		score += 1
	}

	// 基于条件数量
	score += len(conditions)

	// 基于操作数量
	score += len(operations)

	// 基于意图类型
	switch intent {
	case IntentAggregate, IntentAnalyze:
		score += 2
	case IntentUpdate, IntentDelete:
		score += 1
	}

	if score >= 8 {
		return "complex"
	} else if score >= 4 {
		return "medium"
	} else {
		return "simple"
	}
}

// calculateConfidence 计算置信度
func (p *QueryParser) calculateConfidence(intent QueryIntent, tables []string, columns []string) float64 {
	confidence := 0.0

	// 基于意图识别的置信度
	if intent != IntentUnknown {
		confidence += 0.3
	}

	// 基于表名识别的置信度
	if len(tables) > 0 {
		confidence += 0.4
	}

	// 基于列名识别的置信度
	if len(columns) > 0 && columns[0] != "*" {
		confidence += 0.3
	}

	return confidence
}

// generateSuggestions 生成查询建议
func (p *QueryParser) generateSuggestions(ctx context.Context, intent QueryIntent, tables []string, columns []string, conditions []QueryCondition) []string {
	var suggestions []string

	// 基于意图的建议
	switch intent {
	case IntentSelect:
		if len(conditions) == 0 {
			suggestions = append(suggestions, "建议添加查询条件以提高查询效率")
		}
		if len(tables) > 1 {
			suggestions = append(suggestions, "多表查询建议使用适当的连接条件")
		}
	case IntentAggregate:
		suggestions = append(suggestions, "聚合查询建议使用索引字段进行分组")
	case IntentUpdate, IntentDelete:
		if len(conditions) == 0 {
			suggestions = append(suggestions, "修改或删除操作必须包含WHERE条件")
		}
	}

	// 基于表结构的建议
	for _, tableName := range tables {
		tableInfo, err := p.schemaManager.GetTableInfo(tableName)
		if err != nil {
			continue
		}

		// 检查是否使用了索引字段
		hasIndexedCondition := false
		for _, condition := range conditions {
			for _, index := range tableInfo.Indexes {
				if contains(index.Columns, condition.Column) {
					hasIndexedCondition = true
					break
				}
			}
		}

		if !hasIndexedCondition && len(conditions) > 0 {
			suggestions = append(suggestions, fmt.Sprintf("建议在表 %s 的查询条件中使用索引字段", tableName))
		}
	}

	return suggestions
}

// contains 检查字符串切片是否包含指定字符串
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// GetQueryContext 获取查询上下文信息
func (p *QueryParser) GetQueryContext(ctx context.Context, query string) (map[string]interface{}, error) {
	context := make(map[string]interface{})

	// 添加时间戳
	context["timestamp"] = time.Now()

	// 添加查询长度
	context["query_length"] = len(query)

	// 添加查询类型
	intent := p.identifyIntent(p.preprocessQuery(query))
	context["intent"] = string(intent)

	// 提取表信息
	tables, err := p.extractTables(ctx, p.preprocessQuery(query))
	if err != nil {
		return nil, err
	}
	context["tables"] = tables

	// 添加表的详细信息
	var tableInfos []*core.TableInfo
	for _, tableName := range tables {
		tableInfo, err := p.schemaManager.GetTableInfo(tableName)
		if err != nil {
			p.logger.Warn("获取表信息失败", "table", tableName, "error", err)
			continue
		}
		tableInfos = append(tableInfos, tableInfo)
	}
	context["table_infos"] = tableInfos

	return context, nil
}

// ValidateQuery 验证查询的有效性
func (p *QueryParser) ValidateQuery(ctx context.Context, parsed *ParsedQuery) error {
	// 验证表是否存在
	for _, tableName := range parsed.Tables {
		_, err := p.schemaManager.GetTableInfo(tableName)
		if err != nil {
			return fmt.Errorf("表 %s 不存在: %w", tableName, err)
		}
	}

	// 验证列是否存在
	for _, tableName := range parsed.Tables {
		tableInfo, err := p.schemaManager.GetTableInfo(tableName)
		if err != nil {
			continue
		}

		for _, columnName := range parsed.Columns {
			if columnName == "*" {
				continue
			}

			found := false
			for _, column := range tableInfo.Columns {
				if column.Name == columnName {
					found = true
					break
				}
			}

			if !found {
				return fmt.Errorf("表 %s 中不存在列 %s", tableName, columnName)
			}
		}
	}

	// 验证危险操作
	if parsed.Intent == IntentDelete || parsed.Intent == IntentUpdate {
		if len(parsed.Conditions) == 0 {
			return fmt.Errorf("删除或更新操作必须包含WHERE条件")
		}
	}

	return nil
}
