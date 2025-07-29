package cache

import (
	"context"
	"fmt"
	"github.com/Anniext/rag/core"
	"regexp"
	"strings"
	"time"
)

// QueryOptimizer SQL 查询性能分析和优化器
type QueryOptimizer struct {
	logger  core.Logger
	metrics core.MetricsCollector
	config  *OptimizerConfig
}

// OptimizerConfig 优化器配置
type OptimizerConfig struct {
	SlowQueryThreshold    time.Duration `yaml:"slow_query_threshold" json:"slow_query_threshold"`
	EnableQueryPlan       bool          `yaml:"enable_query_plan" json:"enable_query_plan"`
	EnableIndexSuggestion bool          `yaml:"enable_index_suggestion" json:"enable_index_suggestion"`
	MaxQueryLength        int           `yaml:"max_query_length" json:"max_query_length"`
	EnableQueryRewrite    bool          `yaml:"enable_query_rewrite" json:"enable_query_rewrite"`
}

// DefaultOptimizerConfig 默认优化器配置
func DefaultOptimizerConfig() *OptimizerConfig {
	return &OptimizerConfig{
		SlowQueryThreshold:    time.Second,
		EnableQueryPlan:       true,
		EnableIndexSuggestion: true,
		MaxQueryLength:        10000,
		EnableQueryRewrite:    true,
	}
}

// QueryAnalysis 查询分析结果
type QueryAnalysis struct {
	SQL              string                   `json:"sql"`
	QueryType        string                   `json:"query_type"`
	Tables           []string                 `json:"tables"`
	Columns          []string                 `json:"columns"`
	HasJoin          bool                     `json:"has_join"`
	HasSubquery      bool                     `json:"has_subquery"`
	HasAggregate     bool                     `json:"has_aggregate"`
	HasOrderBy       bool                     `json:"has_order_by"`
	HasGroupBy       bool                     `json:"has_group_by"`
	HasLimit         bool                     `json:"has_limit"`
	EstimatedRows    int64                    `json:"estimated_rows"`
	ExecutionTime    time.Duration            `json:"execution_time"`
	Issues           []QueryIssue             `json:"issues"`
	Suggestions      []OptimizationSuggestion `json:"suggestions"`
	QueryPlan        *QueryPlan               `json:"query_plan,omitempty"`
	Complexity       QueryComplexity          `json:"complexity"`
	IndexUsage       []IndexUsage             `json:"index_usage"`
	PerformanceScore float64                  `json:"performance_score"`
}

// QueryIssue 查询问题
type QueryIssue struct {
	Type       string `json:"type"`
	Severity   string `json:"severity"`
	Message    string `json:"message"`
	Line       int    `json:"line,omitempty"`
	Column     int    `json:"column,omitempty"`
	Suggestion string `json:"suggestion,omitempty"`
}

// OptimizationSuggestion 优化建议
type OptimizationSuggestion struct {
	Type        string  `json:"type"`
	Priority    string  `json:"priority"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Example     string  `json:"example,omitempty"`
	Impact      string  `json:"impact"`
	Effort      string  `json:"effort"`
	Score       float64 `json:"score"`
}

// QueryPlan 查询计划
type QueryPlan struct {
	Steps         []PlanStep `json:"steps"`
	TotalCost     float64    `json:"total_cost"`
	EstimatedRows int64      `json:"estimated_rows"`
	Warnings      []string   `json:"warnings"`
}

// PlanStep 查询计划步骤
type PlanStep struct {
	ID        int     `json:"id"`
	Operation string  `json:"operation"`
	Table     string  `json:"table,omitempty"`
	Index     string  `json:"index,omitempty"`
	Condition string  `json:"condition,omitempty"`
	Rows      int64   `json:"rows"`
	Cost      float64 `json:"cost"`
	Time      string  `json:"time,omitempty"`
}

// QueryComplexity 查询复杂度
type QueryComplexity struct {
	Score         float64  `json:"score"`
	Level         string   `json:"level"` // LOW, MEDIUM, HIGH, VERY_HIGH
	Factors       []string `json:"factors"`
	TableCount    int      `json:"table_count"`
	JoinCount     int      `json:"join_count"`
	SubqueryCount int      `json:"subquery_count"`
}

// IndexUsage 索引使用情况
type IndexUsage struct {
	Table       string  `json:"table"`
	Index       string  `json:"index"`
	Used        bool    `json:"used"`
	Selectivity float64 `json:"selectivity"`
	Cardinality int64   `json:"cardinality"`
}

// SlowQueryAlert 慢查询告警
type SlowQueryAlert struct {
	ID            string         `json:"id"`
	SQL           string         `json:"sql"`
	ExecutionTime time.Duration  `json:"execution_time"`
	Timestamp     time.Time      `json:"timestamp"`
	UserID        string         `json:"user_id,omitempty"`
	SessionID     string         `json:"session_id,omitempty"`
	Database      string         `json:"database"`
	Analysis      *QueryAnalysis `json:"analysis"`
	Severity      string         `json:"severity"`
}

// NewQueryOptimizer 创建查询优化器
func NewQueryOptimizer(config *OptimizerConfig, logger core.Logger, metrics core.MetricsCollector) *QueryOptimizer {
	if config == nil {
		config = DefaultOptimizerConfig()
	}

	return &QueryOptimizer{
		config:  config,
		logger:  logger,
		metrics: metrics,
	}
}

// AnalyzeQuery 分析查询
func (o *QueryOptimizer) AnalyzeQuery(ctx context.Context, sql string, executionTime time.Duration) (*QueryAnalysis, error) {
	start := time.Now()
	defer func() {
		if o.metrics != nil {
			o.metrics.RecordHistogram("query_analysis_duration_seconds",
				time.Since(start).Seconds(),
				map[string]string{"operation": "analyze"})
		}
	}()

	// 基本验证
	if len(sql) > o.config.MaxQueryLength {
		return nil, fmt.Errorf("query too long: %d characters (max: %d)",
			len(sql), o.config.MaxQueryLength)
	}

	analysis := &QueryAnalysis{
		SQL:           sql,
		ExecutionTime: executionTime,
		Issues:        []QueryIssue{},
		Suggestions:   []OptimizationSuggestion{},
		IndexUsage:    []IndexUsage{},
	}

	// 解析查询类型
	analysis.QueryType = o.detectQueryType(sql)

	// 提取表和列
	analysis.Tables = o.extractTables(sql)
	analysis.Columns = o.extractColumns(sql)

	// 分析查询特征
	analysis.HasJoin = o.hasJoin(sql)
	analysis.HasSubquery = o.hasSubquery(sql)
	analysis.HasAggregate = o.hasAggregate(sql)
	analysis.HasOrderBy = o.hasOrderBy(sql)
	analysis.HasGroupBy = o.hasGroupBy(sql)
	analysis.HasLimit = o.hasLimit(sql)

	// 计算复杂度
	analysis.Complexity = o.calculateComplexity(analysis)

	// 检测问题
	analysis.Issues = o.detectIssues(analysis)

	// 生成优化建议
	analysis.Suggestions = o.generateSuggestions(analysis)

	// 计算性能分数
	analysis.PerformanceScore = o.calculatePerformanceScore(analysis)

	// 记录指标
	if o.metrics != nil {
		o.metrics.IncrementCounter("queries_analyzed_total",
			map[string]string{"type": analysis.QueryType, "complexity": analysis.Complexity.Level})

		if executionTime > o.config.SlowQueryThreshold {
			o.metrics.IncrementCounter("slow_queries_total",
				map[string]string{"type": analysis.QueryType})
		}
	}

	return analysis, nil
}

// CheckSlowQuery 检查是否为慢查询
func (o *QueryOptimizer) CheckSlowQuery(ctx context.Context, sql string, executionTime time.Duration, userID, sessionID string) *SlowQueryAlert {
	if executionTime <= o.config.SlowQueryThreshold {
		return nil
	}

	// 分析查询
	analysis, err := o.AnalyzeQuery(ctx, sql, executionTime)
	if err != nil {
		o.logger.Error("Failed to analyze slow query", "sql", sql, "error", err)
		analysis = &QueryAnalysis{
			SQL:           sql,
			ExecutionTime: executionTime,
			Issues:        []QueryIssue{},
			Suggestions:   []OptimizationSuggestion{},
		}
	}

	// 确定严重程度
	severity := o.determineSeverity(executionTime)

	alert := &SlowQueryAlert{
		ID:            fmt.Sprintf("slow_%d", time.Now().UnixNano()),
		SQL:           sql,
		ExecutionTime: executionTime,
		Timestamp:     time.Now(),
		UserID:        userID,
		SessionID:     sessionID,
		Analysis:      analysis,
		Severity:      severity,
	}

	// 记录慢查询日志
	o.logger.Warn("Slow query detected",
		"id", alert.ID,
		"execution_time", executionTime,
		"threshold", o.config.SlowQueryThreshold,
		"severity", severity,
		"user_id", userID,
		"session_id", sessionID,
		"sql", sql)

	// 记录指标
	if o.metrics != nil {
		o.metrics.IncrementCounter("slow_queries_detected_total",
			map[string]string{"severity": severity})
		o.metrics.RecordHistogram("slow_query_duration_seconds",
			executionTime.Seconds(),
			map[string]string{"severity": severity})
	}

	return alert
}

// OptimizeQuery 优化查询
func (o *QueryOptimizer) OptimizeQuery(ctx context.Context, sql string) (string, []OptimizationSuggestion, error) {
	if !o.config.EnableQueryRewrite {
		return sql, nil, nil
	}

	analysis, err := o.AnalyzeQuery(ctx, sql, 0)
	if err != nil {
		return sql, nil, err
	}

	optimizedSQL := sql
	appliedSuggestions := []OptimizationSuggestion{}

	// 应用自动优化规则
	for _, suggestion := range analysis.Suggestions {
		if (suggestion.Priority == "HIGH" || suggestion.Priority == "LOW") && o.canAutoApply(suggestion.Type) {
			newSQL, applied := o.applySuggestion(optimizedSQL, suggestion)
			if applied {
				optimizedSQL = newSQL
				appliedSuggestions = append(appliedSuggestions, suggestion)
			}
		}
	}

	return optimizedSQL, appliedSuggestions, nil
}

// detectQueryType 检测查询类型
func (o *QueryOptimizer) detectQueryType(sql string) string {
	sql = strings.ToUpper(strings.TrimSpace(sql))

	if strings.HasPrefix(sql, "SELECT") {
		return "SELECT"
	} else if strings.HasPrefix(sql, "INSERT") {
		return "INSERT"
	} else if strings.HasPrefix(sql, "UPDATE") {
		return "UPDATE"
	} else if strings.HasPrefix(sql, "DELETE") {
		return "DELETE"
	} else if strings.HasPrefix(sql, "CREATE") {
		return "CREATE"
	} else if strings.HasPrefix(sql, "ALTER") {
		return "ALTER"
	} else if strings.HasPrefix(sql, "DROP") {
		return "DROP"
	}

	return "UNKNOWN"
}

// extractTables 提取表名
func (o *QueryOptimizer) extractTables(sql string) []string {
	// 简化的表名提取逻辑
	tables := []string{}

	// 匹配 FROM 和 JOIN 后的表名
	patterns := []string{
		`(?i)\bFROM\s+(\w+)`,
		`(?i)\bJOIN\s+(\w+)`,
		`(?i)\bINTO\s+(\w+)`,
		`(?i)\bUPDATE\s+(\w+)`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllStringSubmatch(sql, -1)
		for _, match := range matches {
			if len(match) > 1 {
				table := strings.ToLower(match[1])
				if !contains(tables, table) {
					tables = append(tables, table)
				}
			}
		}
	}

	return tables
}

// extractColumns 提取列名
func (o *QueryOptimizer) extractColumns(sql string) []string {
	columns := []string{}

	// 简化的列名提取逻辑
	// 匹配 SELECT 后的列名
	re := regexp.MustCompile(`(?i)SELECT\s+(.*?)\s+FROM`)
	matches := re.FindStringSubmatch(sql)
	if len(matches) > 1 {
		columnsPart := matches[1]
		if strings.TrimSpace(columnsPart) != "*" {
			cols := strings.Split(columnsPart, ",")
			for _, col := range cols {
				col = strings.TrimSpace(col)
				// 移除别名
				if idx := strings.Index(col, " AS "); idx != -1 {
					col = col[:idx]
				}
				if idx := strings.Index(col, " "); idx != -1 {
					col = col[:idx]
				}
				if col != "" && !contains(columns, col) {
					columns = append(columns, col)
				}
			}
		}
	}

	return columns
}

// hasJoin 检查是否有 JOIN
func (o *QueryOptimizer) hasJoin(sql string) bool {
	return regexp.MustCompile(`(?i)\bJOIN\b`).MatchString(sql)
}

// hasSubquery 检查是否有子查询
func (o *QueryOptimizer) hasSubquery(sql string) bool {
	// 简单检查括号内的 SELECT
	return regexp.MustCompile(`(?i)\(\s*SELECT\b`).MatchString(sql)
}

// hasAggregate 检查是否有聚合函数
func (o *QueryOptimizer) hasAggregate(sql string) bool {
	aggregates := []string{"COUNT", "SUM", "AVG", "MAX", "MIN", "GROUP_CONCAT"}
	for _, agg := range aggregates {
		if regexp.MustCompile(`(?i)\b` + agg + `\s*\(`).MatchString(sql) {
			return true
		}
	}
	return false
}

// hasOrderBy 检查是否有 ORDER BY
func (o *QueryOptimizer) hasOrderBy(sql string) bool {
	return regexp.MustCompile(`(?i)\bORDER\s+BY\b`).MatchString(sql)
}

// hasGroupBy 检查是否有 GROUP BY
func (o *QueryOptimizer) hasGroupBy(sql string) bool {
	return regexp.MustCompile(`(?i)\bGROUP\s+BY\b`).MatchString(sql)
}

// hasLimit 检查是否有 LIMIT
func (o *QueryOptimizer) hasLimit(sql string) bool {
	return regexp.MustCompile(`(?i)\bLIMIT\b`).MatchString(sql)
}

// calculateComplexity 计算查询复杂度
func (o *QueryOptimizer) calculateComplexity(analysis *QueryAnalysis) QueryComplexity {
	score := 0.0
	factors := []string{}

	// 表数量影响
	tableCount := len(analysis.Tables)
	score += float64(tableCount) * 1.0
	if tableCount > 1 {
		factors = append(factors, fmt.Sprintf("%d tables", tableCount))
	}

	// JOIN 影响
	joinCount := 0
	if analysis.HasJoin {
		joinCount = tableCount - 1 // 简化计算
		score += float64(joinCount) * 2.0
		factors = append(factors, fmt.Sprintf("%d joins", joinCount))
	}

	// 子查询影响
	subqueryCount := 0
	if analysis.HasSubquery {
		subqueryCount = 1 // 简化计算
		score += 3.0
		factors = append(factors, "subquery")
	}

	// 聚合函数影响
	if analysis.HasAggregate {
		score += 1.5
		factors = append(factors, "aggregate")
	}

	// GROUP BY 影响
	if analysis.HasGroupBy {
		score += 1.0
		factors = append(factors, "group by")
	}

	// ORDER BY 影响
	if analysis.HasOrderBy {
		score += 0.5
		factors = append(factors, "order by")
	}

	// 确定复杂度级别
	var level string
	if score <= 2.0 {
		level = "LOW"
	} else if score <= 5.0 {
		level = "MEDIUM"
	} else if score <= 10.0 {
		level = "HIGH"
	} else {
		level = "VERY_HIGH"
	}

	return QueryComplexity{
		Score:         score,
		Level:         level,
		Factors:       factors,
		TableCount:    tableCount,
		JoinCount:     joinCount,
		SubqueryCount: subqueryCount,
	}
}

// detectIssues 检测查询问题
func (o *QueryOptimizer) detectIssues(analysis *QueryAnalysis) []QueryIssue {
	issues := []QueryIssue{}

	// 检查 SELECT *
	if strings.Contains(strings.ToUpper(analysis.SQL), "SELECT *") {
		issues = append(issues, QueryIssue{
			Type:       "SELECT_ALL",
			Severity:   "MEDIUM",
			Message:    "使用 SELECT * 可能影响性能",
			Suggestion: "明确指定需要的列名",
		})
	}

	// 检查缺少 LIMIT
	if analysis.QueryType == "SELECT" && !analysis.HasLimit {
		issues = append(issues, QueryIssue{
			Type:       "MISSING_LIMIT",
			Severity:   "LOW",
			Message:    "查询没有 LIMIT 子句",
			Suggestion: "添加 LIMIT 子句限制返回行数",
		})
	}

	// 检查复杂度过高
	if analysis.Complexity.Score > 10.0 {
		issues = append(issues, QueryIssue{
			Type:       "HIGH_COMPLEXITY",
			Severity:   "HIGH",
			Message:    "查询复杂度过高",
			Suggestion: "考虑拆分为多个简单查询或优化 JOIN 条件",
		})
	}

	// 检查可能的笛卡尔积
	if analysis.HasJoin && len(analysis.Tables) > 2 {
		issues = append(issues, QueryIssue{
			Type:       "POTENTIAL_CARTESIAN",
			Severity:   "HIGH",
			Message:    "多表 JOIN 可能产生笛卡尔积",
			Suggestion: "确保所有 JOIN 都有适当的连接条件",
		})
	}

	return issues
}

// generateSuggestions 生成优化建议
func (o *QueryOptimizer) generateSuggestions(analysis *QueryAnalysis) []OptimizationSuggestion {
	suggestions := []OptimizationSuggestion{}

	// 基于问题生成建议
	for _, issue := range analysis.Issues {
		switch issue.Type {
		case "SELECT_ALL":
			suggestions = append(suggestions, OptimizationSuggestion{
				Type:        "COLUMN_SELECTION",
				Priority:    "MEDIUM",
				Title:       "优化列选择",
				Description: "明确指定需要的列而不是使用 SELECT *",
				Impact:      "减少网络传输和内存使用",
				Effort:      "LOW",
				Score:       3.0,
			})
		case "MISSING_LIMIT":
			suggestions = append(suggestions, OptimizationSuggestion{
				Type:        "ADD_LIMIT",
				Priority:    "LOW",
				Title:       "添加 LIMIT 子句",
				Description: "限制返回的行数以提高性能",
				Example:     "SELECT ... LIMIT 100",
				Impact:      "减少内存使用和响应时间",
				Effort:      "LOW",
				Score:       2.0,
			})
		case "HIGH_COMPLEXITY":
			suggestions = append(suggestions, OptimizationSuggestion{
				Type:        "REDUCE_COMPLEXITY",
				Priority:    "HIGH",
				Title:       "降低查询复杂度",
				Description: "考虑拆分复杂查询或优化 JOIN 条件",
				Impact:      "显著提高查询性能",
				Effort:      "HIGH",
				Score:       8.0,
			})
		}
	}

	// 基于查询特征生成建议
	if analysis.HasOrderBy && !analysis.HasLimit {
		suggestions = append(suggestions, OptimizationSuggestion{
			Type:        "INDEX_SUGGESTION",
			Priority:    "MEDIUM",
			Title:       "考虑添加索引",
			Description: "为 ORDER BY 列添加索引可以提高排序性能",
			Impact:      "提高排序性能",
			Effort:      "MEDIUM",
			Score:       4.0,
		})
	}

	return suggestions
}

// calculatePerformanceScore 计算性能分数
func (o *QueryOptimizer) calculatePerformanceScore(analysis *QueryAnalysis) float64 {
	score := 100.0 // 基础分数

	// 根据复杂度扣分
	score -= analysis.Complexity.Score * 5.0

	// 根据问题扣分
	for _, issue := range analysis.Issues {
		switch issue.Severity {
		case "HIGH":
			score -= 20.0
		case "MEDIUM":
			score -= 10.0
		case "LOW":
			score -= 5.0
		}
	}

	// 确保分数在 0-100 范围内
	if score < 0 {
		score = 0
	}

	return score
}

// determineSeverity 确定慢查询严重程度
func (o *QueryOptimizer) determineSeverity(executionTime time.Duration) string {
	threshold := o.config.SlowQueryThreshold

	if executionTime > threshold*10 {
		return "CRITICAL"
	} else if executionTime > threshold*5 {
		return "HIGH"
	} else if executionTime > threshold*2 {
		return "MEDIUM"
	}

	return "LOW"
}

// canAutoApply 检查是否可以自动应用优化
func (o *QueryOptimizer) canAutoApply(suggestionType string) bool {
	// 只有安全的优化才能自动应用
	safeTypes := []string{"ADD_LIMIT"}
	return contains(safeTypes, suggestionType)
}

// applySuggestion 应用优化建议
func (o *QueryOptimizer) applySuggestion(sql string, suggestion OptimizationSuggestion) (string, bool) {
	switch suggestion.Type {
	case "ADD_LIMIT":
		if !regexp.MustCompile(`(?i)\bLIMIT\b`).MatchString(sql) {
			return sql + " LIMIT 1000", true
		}
	}

	return sql, false
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
