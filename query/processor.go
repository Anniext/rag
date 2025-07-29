package query

import (
	"context"
	"fmt"
	"strings"
	"time"

	"pumppill/rag/core"
)

// RAGQueryProcessor RAG 查询处理器的完整实现
type RAGQueryProcessor struct {
	parser       *QueryParser
	generator    *SQLGenerator
	executor     *QueryExecutor
	cacheManager core.CacheManager
	langChain    core.LangChainManager
	logger       core.Logger
	config       *ProcessorConfig
}

// ProcessorConfig 处理器配置
type ProcessorConfig struct {
	EnableCache        bool          `json:"enable_cache"`
	CacheTTL           time.Duration `json:"cache_ttl"`
	EnableExplanation  bool          `json:"enable_explanation"`
	EnableOptimization bool          `json:"enable_optimization"`
	MaxRetries         int           `json:"max_retries"`
	RetryDelay         time.Duration `json:"retry_delay"`
}

// NewRAGQueryProcessor 创建新的 RAG 查询处理器
func NewRAGQueryProcessor(
	parser *QueryParser,
	generator *SQLGenerator,
	executor *QueryExecutor,
	cacheManager core.CacheManager,
	langChain core.LangChainManager,
	logger core.Logger,
	config *ProcessorConfig,
) *RAGQueryProcessor {
	if config == nil {
		config = &ProcessorConfig{
			EnableCache:        true,
			CacheTTL:           5 * time.Minute,
			EnableExplanation:  true,
			EnableOptimization: true,
			MaxRetries:         3,
			RetryDelay:         1 * time.Second,
		}
	}

	return &RAGQueryProcessor{
		parser:       parser,
		generator:    generator,
		executor:     executor,
		cacheManager: cacheManager,
		langChain:    langChain,
		logger:       logger,
		config:       config,
	}
}

// ProcessQuery 处理查询请求
func (p *RAGQueryProcessor) ProcessQuery(ctx context.Context, request *core.QueryRequest) (*core.QueryResponse, error) {
	p.logger.Info("开始处理查询请求", "query", request.Query, "user", request.UserID, "request_id", request.RequestID)

	// 检查缓存
	if p.config.EnableCache {
		if cached, err := p.getCachedResult(ctx, request); err == nil && cached != nil {
			p.logger.Debug("返回缓存结果", "request_id", request.RequestID)
			return cached, nil
		}
	}

	// 解析查询
	parsed, err := p.parser.ParseQuery(ctx, request.Query)
	if err != nil {
		return p.createErrorResponse(request, fmt.Errorf("查询解析失败: %w", err)), nil
	}

	// 验证解析结果
	if err := p.parser.ValidateQuery(ctx, parsed); err != nil {
		return p.createErrorResponse(request, fmt.Errorf("查询验证失败: %w", err)), nil
	}

	// 生成 SQL
	generatedSQL, err := p.generator.GenerateSQL(ctx, parsed)
	if err != nil {
		return p.createErrorResponse(request, fmt.Errorf("SQL 生成失败: %w", err)), nil
	}

	// 优化 SQL（如果启用）
	if p.config.EnableOptimization {
		optimizedSQL, suggestions, err := p.generator.OptimizeSQL(ctx, generatedSQL.SQL, parsed.Tables)
		if err != nil {
			p.logger.Warn("SQL 优化失败", "error", err)
		} else {
			generatedSQL.SQL = optimizedSQL
			generatedSQL.Warnings = append(generatedSQL.Warnings, suggestions...)
		}
	}

	// 获取用户信息（从上下文或请求中）
	userInfo := p.getUserInfo(ctx, request)

	// 执行查询（带重试机制）
	var result *ExecutionResult
	for i := 0; i < p.config.MaxRetries; i++ {
		result, err = p.executor.ExecuteQuery(ctx, generatedSQL, userInfo, request.Options)
		if err == nil {
			break
		}

		if i < p.config.MaxRetries-1 {
			p.logger.Warn("查询执行失败，准备重试", "attempt", i+1, "error", err)
			time.Sleep(p.config.RetryDelay)
		}
	}

	if err != nil {
		return p.createErrorResponse(request, fmt.Errorf("查询执行失败: %w", err)), nil
	}

	// 生成自然语言解释
	var explanation string
	if p.config.EnableExplanation {
		explanation, err = p.generateExplanation(ctx, request.Query, parsed, generatedSQL, result)
		if err != nil {
			p.logger.Warn("生成解释失败", "error", err)
			explanation = generatedSQL.Explanation
		}
	}

	// 构建响应
	response := &core.QueryResponse{
		Success:     true,
		Data:        result.Data,
		SQL:         generatedSQL.SQL,
		Explanation: explanation,
		Suggestions: p.generateSuggestions(ctx, parsed, result),
		Metadata: &core.QueryMetadata{
			ExecutionTime: result.ExecutionTime,
			RowCount:      result.RowCount,
			AffectedRows:  int(result.AffectedRows),
			QueryPlan:     result.QueryPlan,
			Warnings:      append(generatedSQL.Warnings, result.Warnings...),
			CacheHit:      false,
		},
		RequestID: request.RequestID,
	}

	// 缓存结果
	if p.config.EnableCache && result.RowCount > 0 {
		if err := p.cacheResult(ctx, request, response); err != nil {
			p.logger.Warn("缓存结果失败", "error", err)
		}
	}

	p.logger.Info("查询处理完成",
		"request_id", request.RequestID,
		"rows", result.RowCount,
		"execution_time", result.ExecutionTime)

	return response, nil
}

// GetSuggestions 获取查询建议
func (p *RAGQueryProcessor) GetSuggestions(ctx context.Context, partial string) ([]string, error) {
	p.logger.Debug("获取查询建议", "partial", partial)

	// 使用 LangChain 生成智能建议
	if p.langChain != nil {
		suggestions, err := p.generateLangChainSuggestions(ctx, partial)
		if err == nil && len(suggestions) > 0 {
			return suggestions, nil
		}
		p.logger.Warn("LangChain 建议生成失败", "error", err)
	}

	// 回退到基础建议
	return p.generateBasicSuggestions(partial), nil
}

// ExplainQuery 解释查询
func (p *RAGQueryProcessor) ExplainQuery(ctx context.Context, query string) (*core.QueryExplanation, error) {
	p.logger.Debug("解释查询", "query", query)

	// 解析查询
	parsed, err := p.parser.ParseQuery(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("查询解析失败: %w", err)
	}

	// 生成 SQL
	_, err = p.generator.GenerateSQL(ctx, parsed)
	if err != nil {
		return nil, fmt.Errorf("SQL 生成失败: %w", err)
	}

	// 构建解释
	explanation := &core.QueryExplanation{
		Intent:      string(parsed.Intent),
		Tables:      parsed.Tables,
		Columns:     parsed.Columns,
		Conditions:  p.formatConditions(parsed.Conditions),
		Operations:  p.formatOperations(parsed.Operations),
		Complexity:  parsed.Complexity,
		Suggestions: parsed.Suggestions,
	}

	return explanation, nil
}

// getCachedResult 获取缓存结果
func (p *RAGQueryProcessor) getCachedResult(ctx context.Context, request *core.QueryRequest) (*core.QueryResponse, error) {
	cacheKey := p.generateCacheKey(request)

	cached, err := p.cacheManager.Get(ctx, cacheKey)
	if err != nil {
		return nil, err
	}

	if response, ok := cached.(*core.QueryResponse); ok {
		// 标记为缓存命中
		if response.Metadata != nil {
			response.Metadata.CacheHit = true
		}
		return response, nil
	}

	return nil, fmt.Errorf("缓存数据类型错误")
}

// cacheResult 缓存查询结果
func (p *RAGQueryProcessor) cacheResult(ctx context.Context, request *core.QueryRequest, response *core.QueryResponse) error {
	cacheKey := p.generateCacheKey(request)
	return p.cacheManager.Set(ctx, cacheKey, response, p.config.CacheTTL)
}

// generateCacheKey 生成缓存键
func (p *RAGQueryProcessor) generateCacheKey(request *core.QueryRequest) string {
	// 基于查询内容、用户ID和选项生成缓存键
	key := fmt.Sprintf("query:%s:user:%s", request.Query, request.UserID)

	if request.Options != nil {
		key += fmt.Sprintf(":limit:%d:offset:%d:format:%s",
			request.Options.Limit,
			request.Options.Offset,
			request.Options.Format)
	}

	return key
}

// getUserInfo 获取用户信息
func (p *RAGQueryProcessor) getUserInfo(ctx context.Context, request *core.QueryRequest) *core.UserInfo {
	// 从上下文中获取用户信息
	if userInfo, ok := ctx.Value("user").(*core.UserInfo); ok {
		return userInfo
	}

	// 从请求中构建基础用户信息
	return &core.UserInfo{
		ID:       request.UserID,
		Username: request.UserID,
		Roles:    []string{"user"},
	}
}

// generateExplanation 生成自然语言解释
func (p *RAGQueryProcessor) generateExplanation(ctx context.Context, originalQuery string, parsed *ParsedQuery, generatedSQL *GeneratedSQL, result *ExecutionResult) (string, error) {
	if p.langChain == nil {
		return generatedSQL.Explanation, nil
	}

	// 使用 LangChain 生成更详细的解释
	chainConfig := &core.ChainConfig{
		Type: "query_explanation",
		LLMConfig: &core.LLMConfig{
			Provider:    "openai",
			Model:       "gpt-3.5-turbo",
			Temperature: 0.3,
			MaxTokens:   500,
		},
	}

	chain, err := p.langChain.CreateChain(ctx, chainConfig)
	if err != nil {
		return generatedSQL.Explanation, err
	}

	input := &core.ChainInput{
		Query: originalQuery,
		Context: map[string]any{
			"parsed_query":  parsed,
			"generated_sql": generatedSQL,
			"result_summary": map[string]any{
				"row_count":      result.RowCount,
				"execution_time": result.ExecutionTime.String(),
				"affected_rows":  result.AffectedRows,
			},
		},
	}

	output, err := p.langChain.ExecuteChain(ctx, chain, input)
	if err != nil {
		return generatedSQL.Explanation, err
	}

	if explanation, ok := output.Result.(string); ok {
		return explanation, nil
	}

	return generatedSQL.Explanation, nil
}

// generateSuggestions 生成查询建议
func (p *RAGQueryProcessor) generateSuggestions(ctx context.Context, parsed *ParsedQuery, result *ExecutionResult) []string {
	var suggestions []string

	// 基于查询复杂度的建议
	switch parsed.Complexity {
	case "complex":
		suggestions = append(suggestions, "复杂查询可能影响性能，建议添加适当的索引")
		if result.ExecutionTime > 5*time.Second {
			suggestions = append(suggestions, "查询执行时间较长，建议优化查询条件")
		}
	case "medium":
		if result.RowCount > 1000 {
			suggestions = append(suggestions, "结果集较大，建议使用分页查询")
		}
	}

	// 基于结果的建议
	if result.RowCount == 0 {
		suggestions = append(suggestions, "查询未返回结果，请检查查询条件")
	} else if result.RowCount > 10000 {
		suggestions = append(suggestions, "结果集过大，建议添加更具体的查询条件")
	}

	// 基于执行时间的建议
	if result.ExecutionTime > 10*time.Second {
		suggestions = append(suggestions, "查询执行时间过长，建议优化查询或添加索引")
	}

	// 合并解析器的建议
	suggestions = append(suggestions, parsed.Suggestions...)

	return suggestions
}

// generateLangChainSuggestions 使用 LangChain 生成智能建议
func (p *RAGQueryProcessor) generateLangChainSuggestions(ctx context.Context, partial string) ([]string, error) {
	chainConfig := &core.ChainConfig{
		Type: "query_suggestions",
		LLMConfig: &core.LLMConfig{
			Provider:    "openai",
			Model:       "gpt-3.5-turbo",
			Temperature: 0.7,
			MaxTokens:   200,
		},
	}

	chain, err := p.langChain.CreateChain(ctx, chainConfig)
	if err != nil {
		return nil, err
	}

	input := &core.ChainInput{
		Query: partial,
		Context: map[string]any{
			"type": "autocomplete",
		},
	}

	output, err := p.langChain.ExecuteChain(ctx, chain, input)
	if err != nil {
		return nil, err
	}

	if suggestions, ok := output.Result.([]string); ok {
		return suggestions, nil
	}

	return nil, fmt.Errorf("无效的建议格式")
}

// generateBasicSuggestions 生成基础建议
func (p *RAGQueryProcessor) generateBasicSuggestions(partial string) []string {
	partial = strings.ToLower(strings.TrimSpace(partial))

	var suggestions []string

	// 基于部分输入的建议
	if strings.HasPrefix(partial, "查询") || strings.HasPrefix(partial, "select") {
		suggestions = append(suggestions,
			"查询用户信息",
			"查询订单数据",
			"查询产品列表",
			"查询销售统计")
	} else if strings.HasPrefix(partial, "统计") || strings.HasPrefix(partial, "count") {
		suggestions = append(suggestions,
			"统计用户数量",
			"统计订单总数",
			"统计销售额")
	} else if strings.HasPrefix(partial, "分析") || strings.HasPrefix(partial, "analyze") {
		suggestions = append(suggestions,
			"分析用户行为",
			"分析销售趋势",
			"分析产品性能")
	} else {
		// 默认建议
		suggestions = append(suggestions,
			"查询用户信息",
			"统计订单数量",
			"分析销售数据",
			"查询产品列表")
	}

	return suggestions
}

// formatConditions 格式化查询条件
func (p *RAGQueryProcessor) formatConditions(conditions []QueryCondition) []string {
	var formatted []string

	for _, condition := range conditions {
		formatted = append(formatted,
			fmt.Sprintf("%s %s %v", condition.Column, condition.Operator, condition.Value))
	}

	return formatted
}

// formatOperations 格式化查询操作
func (p *RAGQueryProcessor) formatOperations(operations []QueryOperation) []string {
	var formatted []string

	for _, operation := range operations {
		switch operation.Type {
		case "order_by":
			if column, ok := operation.Parameters["column"].(string); ok {
				direction := "ASC"
				if dir, ok := operation.Parameters["direction"].(string); ok {
					direction = dir
				}
				formatted = append(formatted, fmt.Sprintf("按 %s %s 排序", column, direction))
			}
		case "group_by":
			if column, ok := operation.Parameters["column"].(string); ok {
				formatted = append(formatted, fmt.Sprintf("按 %s 分组", column))
			}
		case "limit":
			if count, ok := operation.Parameters["count"].(string); ok {
				formatted = append(formatted, fmt.Sprintf("限制 %s 条结果", count))
			}
		}
	}

	return formatted
}

// createErrorResponse 创建错误响应
func (p *RAGQueryProcessor) createErrorResponse(request *core.QueryRequest, err error) *core.QueryResponse {
	p.logger.Error("查询处理失败", "error", err, "request_id", request.RequestID)

	return &core.QueryResponse{
		Success:   false,
		Data:      []map[string]any{},
		Error:     err.Error(),
		RequestID: request.RequestID,
		Metadata: &core.QueryMetadata{
			ExecutionTime: 0,
			RowCount:      0,
			CacheHit:      false,
		},
	}
}

// GetProcessorStats 获取处理器统计信息
func (p *RAGQueryProcessor) GetProcessorStats(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// 获取执行器统计
	if executorStats, err := p.executor.GetExecutionStats(ctx); err == nil {
		stats["executor"] = executorStats
	}

	// 获取缓存统计（如果支持）
	// 这里可以添加缓存统计信息

	// 添加配置信息
	stats["config"] = map[string]interface{}{
		"enable_cache":        p.config.EnableCache,
		"cache_ttl":           p.config.CacheTTL.String(),
		"enable_explanation":  p.config.EnableExplanation,
		"enable_optimization": p.config.EnableOptimization,
		"max_retries":         p.config.MaxRetries,
		"retry_delay":         p.config.RetryDelay.String(),
	}

	return stats, nil
}

// Close 关闭处理器
func (p *RAGQueryProcessor) Close() error {
	if p.executor != nil {
		return p.executor.Close()
	}
	return nil
}
