package langchain

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/tmc/langchaingo/llms"
	"go.uber.org/zap"

	"pumppill/rag/core"
)

// baseTool 基础工具实现，包含工具的通用属性和方法
type baseTool struct {
	name             string            // 工具名称
	description      string            // 工具描述
	llmClient        LLMClient         // LLM 客户端，用于调用大语言模型
	templateRenderer *TemplateRenderer // 模板渲染器，用于生成提示词
	logger           *zap.Logger       // 日志记录器
}

// Name 获取工具名称
func (t *baseTool) Name() string {
	return t.name
}

// Description 获取工具描述
func (t *baseTool) Description() string {
	return t.description
}

// sqlGeneratorTool SQL 生成工具，继承 baseTool
type sqlGeneratorTool struct {
	baseTool
}

// NewSQLGeneratorTool 创建 SQL 生成工具实例
func NewSQLGeneratorTool(llmClient LLMClient, templateRenderer *TemplateRenderer, logger *zap.Logger) core.Tool {
	return &sqlGeneratorTool{
		baseTool: baseTool{
			name:             "sql_generator",
			description:      "将自然语言查询转换为 SQL 语句",
			llmClient:        llmClient,
			templateRenderer: templateRenderer,
			logger:           logger,
		},
	}
}

// Execute 执行 SQL 生成逻辑
func (t *sqlGeneratorTool) Execute(ctx context.Context, input string) (string, error) {
	t.logger.Debug("Executing SQL generator tool", zap.String("input", input))

	// 解析输入，提取查询和 schema 信息
	query, schema, err := t.parseInput(input)
	if err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}

	// 渲染 SQL 生成的提示词模板
	prompt, err := t.templateRenderer.RenderSQLGeneration(ctx, query, schema)
	if err != nil {
		return "", fmt.Errorf("failed to render template: %w", err)
	}

	// 构造 LLM 消息内容
	messages := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeHuman, prompt),
	}

	// 调用 LLM 生成 SQL
	response, err := t.llmClient.GenerateContent(ctx, messages)
	if err != nil {
		return "", fmt.Errorf("LLM generation failed: %w", err)
	}

	// 检查 LLM 响应
	if len(response.Choices) == 0 {
		return "", fmt.Errorf("no response from LLM")
	}

	sql := response.Choices[0].Content

	// 清理 SQL 格式，去除多余内容
	sql = t.cleanSQL(sql)

	t.logger.Debug("SQL generated successfully", zap.String("sql", sql))

	return sql, nil
}

// parseInput 解析输入字符串，提取 Query 和 Schema 信息
func (t *sqlGeneratorTool) parseInput(input string) (string, *core.SchemaInfo, error) {
	lines := strings.Split(input, "\n")
	var query string
	var schemaStr string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Query:") {
			query = strings.TrimSpace(strings.TrimPrefix(line, "Query:"))
		} else if strings.HasPrefix(line, "Schema:") {
			schemaStr = strings.TrimSpace(strings.TrimPrefix(line, "Schema:"))
		}
	}

	if query == "" {
		return "", nil, fmt.Errorf("query not found in input")
	}

	// 简单的 schema 解析（实际项目中可扩展为更复杂的解析）
	var schema *core.SchemaInfo
	if schemaStr != "" && schemaStr != "<nil>" {
		schema = &core.SchemaInfo{
			Database: "default",
			Tables:   []*core.TableInfo{}, // 这里只做简化处理
		}
	}

	return query, schema, nil
}

// cleanSQL 清理 SQL 字符串，去除代码块标记和多余前缀
func (t *sqlGeneratorTool) cleanSQL(sql string) string {
	// 移除代码块标记
	sql = strings.ReplaceAll(sql, "```sql", "")
	sql = strings.ReplaceAll(sql, "```", "")

	// 移除多余的空白字符
	sql = strings.TrimSpace(sql)

	// 移除 "SQL:" 前缀
	if strings.HasPrefix(sql, "SQL:") {
		sql = strings.TrimSpace(strings.TrimPrefix(sql, "SQL:"))
	}

	return sql
}

// queryExplanationTool 查询解释工具，继承 baseTool
type queryExplanationTool struct {
	baseTool
}

// NewQueryExplanationTool 创建查询解释工具实例
func NewQueryExplanationTool(llmClient LLMClient, templateRenderer *TemplateRenderer, logger *zap.Logger) core.Tool {
	return &queryExplanationTool{
		baseTool: baseTool{
			name:             "query_explanation",
			description:      "解释 SQL 查询的含义和执行逻辑",
			llmClient:        llmClient,
			templateRenderer: templateRenderer,
			logger:           logger,
		},
	}
}

// Execute 执行查询解释逻辑
func (t *queryExplanationTool) Execute(ctx context.Context, input string) (string, error) {
	t.logger.Debug("Executing query explanation tool", zap.String("input", input))

	// 解析输入，提取 SQL 和结果信息
	sql, results, err := t.parseInput(input)
	if err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}

	// 渲染查询解释的提示词模板
	prompt, err := t.templateRenderer.RenderQueryExplanation(ctx, sql, results)
	if err != nil {
		return "", fmt.Errorf("failed to render template: %w", err)
	}

	// 构造 LLM 消息内容
	messages := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeHuman, prompt),
	}

	// 调用 LLM 生成查询解释
	response, err := t.llmClient.GenerateContent(ctx, messages)
	if err != nil {
		return "", fmt.Errorf("LLM generation failed: %w", err)
	}

	// 检查 LLM 响应
	if len(response.Choices) == 0 {
		return "", fmt.Errorf("no response from LLM")
	}

	explanation := strings.TrimSpace(response.Choices[0].Content)

	t.logger.Debug("Query explanation generated successfully")

	return explanation, nil
}

// parseInput 解析输入字符串，提取 SQL 和结果信息
func (t *queryExplanationTool) parseInput(input string) (string, interface{}, error) {
	lines := strings.Split(input, "\n")
	var sql string
	var results interface{}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "SQL:") {
			sql = strings.TrimSpace(strings.TrimPrefix(line, "SQL:"))
		} else if strings.HasPrefix(line, "Results:") {
			resultsStr := strings.TrimSpace(strings.TrimPrefix(line, "Results:"))
			if resultsStr != "" && resultsStr != "<nil>" {
				results = resultsStr
			}
		}
	}

	if sql == "" {
		return "", nil, fmt.Errorf("SQL not found in input")
	}

	return sql, results, nil
}

// errorAnalysisTool 错误分析工具，继承 baseTool
type errorAnalysisTool struct {
	baseTool
}

// NewErrorAnalysisTool 创建错误分析工具实例
func NewErrorAnalysisTool(llmClient LLMClient, templateRenderer *TemplateRenderer, logger *zap.Logger) core.Tool {
	return &errorAnalysisTool{
		baseTool: baseTool{
			name:             "error_analysis",
			description:      "分析 SQL 查询错误并提供修复建议",
			llmClient:        llmClient,
			templateRenderer: templateRenderer,
			logger:           logger,
		},
	}
}

// Execute 执行错误分析逻辑
func (t *errorAnalysisTool) Execute(ctx context.Context, input string) (string, error) {
	t.logger.Debug("Executing error analysis tool", zap.String("input", input))

	// 解析输入，提取 Query、SQL 和错误信息
	query, sql, errorMsg, err := t.parseInput(input)
	if err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}

	// 渲染错误分析的提示词模板
	prompt, err := t.templateRenderer.RenderErrorAnalysis(ctx, query, sql, errorMsg)
	if err != nil {
		return "", fmt.Errorf("failed to render template: %w", err)
	}

	// 构造 LLM 消息内容
	messages := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeHuman, prompt),
	}

	// 调用 LLM 生成错误分析
	response, err := t.llmClient.GenerateContent(ctx, messages)
	if err != nil {
		return "", fmt.Errorf("LLM generation failed: %w", err)
	}

	// 检查 LLM 响应
	if len(response.Choices) == 0 {
		return "", fmt.Errorf("no response from LLM")
	}

	analysis := strings.TrimSpace(response.Choices[0].Content)

	t.logger.Debug("Error analysis generated successfully")

	return analysis, nil
}

// parseInput 解析输入字符串，提取 Query、SQL 和错误信息
func (t *errorAnalysisTool) parseInput(input string) (string, string, string, error) {
	lines := strings.Split(input, "\n")
	var query, sql, errorMsg string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Query:") {
			query = strings.TrimSpace(strings.TrimPrefix(line, "Query:"))
		} else if strings.HasPrefix(line, "SQL:") {
			sql = strings.TrimSpace(strings.TrimPrefix(line, "SQL:"))
		} else if strings.HasPrefix(line, "Error:") {
			errorMsg = strings.TrimSpace(strings.TrimPrefix(line, "Error:"))
		}
	}

	if query == "" {
		return "", "", "", fmt.Errorf("query not found in input")
	}
	if sql == "" {
		return "", "", "", fmt.Errorf("SQL not found in input")
	}
	if errorMsg == "" {
		return "", "", "", fmt.Errorf("error message not found in input")
	}

	return query, sql, errorMsg, nil
}

// suggestionTool 建议工具，继承 baseTool
type suggestionTool struct {
	baseTool
}

// NewSuggestionTool 创建建议工具实例
func NewSuggestionTool(llmClient LLMClient, templateRenderer *TemplateRenderer, logger *zap.Logger) core.Tool {
	return &suggestionTool{
		baseTool: baseTool{
			name:             "suggestion",
			description:      "基于部分输入提供查询建议",
			llmClient:        llmClient,
			templateRenderer: templateRenderer,
			logger:           logger,
		},
	}
}

// Execute 执行建议生成逻辑
func (t *suggestionTool) Execute(ctx context.Context, input string) (string, error) {
	t.logger.Debug("Executing suggestion tool", zap.String("input", input))

	// 解析输入，提取部分查询和 schema 信息
	partialQuery, schema, err := t.parseInput(input)
	if err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}

	// 渲染建议生成的提示词模板
	prompt, err := t.templateRenderer.RenderSuggestion(ctx, partialQuery, schema)
	if err != nil {
		return "", fmt.Errorf("failed to render template: %w", err)
	}

	// 构造 LLM 消息内容
	messages := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeHuman, prompt),
	}

	// 调用 LLM 生成建议
	response, err := t.llmClient.GenerateContent(ctx, messages)
	if err != nil {
		return "", fmt.Errorf("LLM generation failed: %w", err)
	}

	// 检查 LLM 响应
	if len(response.Choices) == 0 {
		return "", fmt.Errorf("no response from LLM")
	}

	suggestions := strings.TrimSpace(response.Choices[0].Content)

	t.logger.Debug("Suggestions generated successfully")

	return suggestions, nil
}

// parseInput 解析输入字符串，提取 PartialQuery 和 Schema 信息
func (t *suggestionTool) parseInput(input string) (string, *core.SchemaInfo, error) {
	lines := strings.Split(input, "\n")
	var partialQuery string
	var schemaStr string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "PartialQuery:") {
			partialQuery = strings.TrimSpace(strings.TrimPrefix(line, "PartialQuery:"))
		} else if strings.HasPrefix(line, "Schema:") {
			schemaStr = strings.TrimSpace(strings.TrimPrefix(line, "Schema:"))
		}
	}

	if partialQuery == "" {
		return "", nil, fmt.Errorf("partial query not found in input")
	}

	// 简单的 schema 解析
	var schema *core.SchemaInfo
	if schemaStr != "" && schemaStr != "<nil>" {
		schema = &core.SchemaInfo{
			Database: "default",
			Tables:   []*core.TableInfo{},
		}
	}

	return partialQuery, schema, nil
}

// optimizationTool 优化工具，继承 baseTool
type optimizationTool struct {
	baseTool
}

// NewOptimizationTool 创建优化工具实例
func NewOptimizationTool(llmClient LLMClient, templateRenderer *TemplateRenderer, logger *zap.Logger) core.Tool {
	return &optimizationTool{
		baseTool: baseTool{
			name:             "optimization",
			description:      "分析 SQL 查询性能并提供优化建议",
			llmClient:        llmClient,
			templateRenderer: templateRenderer,
			logger:           logger,
		},
	}
}

// Execute 执行优化分析逻辑
func (t *optimizationTool) Execute(ctx context.Context, input string) (string, error) {
	t.logger.Debug("Executing optimization tool", zap.String("input", input))

	// 解析输入，提取 SQL、执行计划和性能指标
	sql, executionPlan, metrics, err := t.parseInput(input)
	if err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}

	// 构建变量映射，用于模板渲染
	variables := map[string]interface{}{
		"SQL":           sql,
		"ExecutionPlan": executionPlan,
		"Metrics":       metrics,
	}

	// 获取最新的优化模板版本
	version, err := t.templateRenderer.manager.GetLatestVersion(TemplateTypeOptimization)
	if err != nil {
		return "", fmt.Errorf("failed to get optimization template: %w", err)
	}

	// 渲染优化分析的提示词模板
	prompt, err := t.templateRenderer.manager.RenderTemplate(TemplateTypeOptimization, version, variables)
	if err != nil {
		return "", fmt.Errorf("failed to render template: %w", err)
	}

	// 构造 LLM 消息内容
	messages := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeHuman, prompt),
	}

	// 调用 LLM 生成优化建议
	response, err := t.llmClient.GenerateContent(ctx, messages)
	if err != nil {
		return "", fmt.Errorf("LLM generation failed: %w", err)
	}

	// 检查 LLM 响应
	if len(response.Choices) == 0 {
		return "", fmt.Errorf("no response from LLM")
	}

	optimization := strings.TrimSpace(response.Choices[0].Content)

	t.logger.Debug("Optimization analysis generated successfully")

	return optimization, nil
}

// parseInput 解析输入字符串，提取 SQL、执行计划和性能指标
func (t *optimizationTool) parseInput(input string) (string, interface{}, interface{}, error) {
	lines := strings.Split(input, "\n")
	var sql string
	var executionPlan, metrics interface{}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "SQL:") {
			sql = strings.TrimSpace(strings.TrimPrefix(line, "SQL:"))
		} else if strings.HasPrefix(line, "ExecutionPlan:") {
			planStr := strings.TrimSpace(strings.TrimPrefix(line, "ExecutionPlan:"))
			if planStr != "" && planStr != "<nil>" {
				executionPlan = planStr
			}
		} else if strings.HasPrefix(line, "Metrics:") {
			metricsStr := strings.TrimSpace(strings.TrimPrefix(line, "Metrics:"))
			if metricsStr != "" && metricsStr != "<nil>" {
				metrics = metricsStr
			}
		}
	}

	if sql == "" {
		return "", nil, nil, fmt.Errorf("SQL not found in input")
	}

	return sql, executionPlan, metrics, nil
}

// MockTool 模拟工具（用于测试），可自定义响应和错误
type MockTool struct {
	name        string // 工具名称
	description string // 工具描述
	response    string // 响应内容
	shouldError bool   // 是否模拟错误
	errorMsg    string // 错误信息
	callCount   int    // 调用次数
}

// NewMockTool 创建模拟工具实例
func NewMockTool(name, description string) *MockTool {
	return &MockTool{
		name:        name,
		description: description,
		response:    fmt.Sprintf("Mock response from %s", name),
	}
}

// Name 获取工具名称
func (t *MockTool) Name() string {
	return t.name
}

// Description 获取工具描述
func (t *MockTool) Description() string {
	return t.description
}

// Execute 执行模拟工具逻辑
func (t *MockTool) Execute(ctx context.Context, input string) (string, error) {
	t.callCount++

	if t.shouldError {
		return "", fmt.Errorf("%s", t.errorMsg)
	}

	return fmt.Sprintf("%s (input: %s, call: %d)", t.response, input, t.callCount), nil
}

// SetResponse 设置模拟响应内容
func (t *MockTool) SetResponse(response string) {
	t.response = response
}

// SetError 设置模拟错误状态和信息
func (t *MockTool) SetError(shouldError bool, errorMsg string) {
	t.shouldError = shouldError
	t.errorMsg = errorMsg
}

// GetCallCount 获取模拟工具被调用的次数
func (t *MockTool) GetCallCount() int {
	return t.callCount
}

// Reset 重置模拟工具状态
func (t *MockTool) Reset() {
	t.callCount = 0
	t.shouldError = false
	t.errorMsg = ""
}

// ToolRegistry 工具注册表，管理所有工具的注册、注销和查询
type ToolRegistry struct {
	tools  map[string]core.Tool // 工具映射表，key 为工具名称
	logger *zap.Logger          // 日志记录器
	mutex  sync.RWMutex         // 读写锁，保证并发安全
}

// NewToolRegistry 创建工具注册表实例
func NewToolRegistry(logger *zap.Logger) *ToolRegistry {
	return &ToolRegistry{
		tools:  make(map[string]core.Tool),
		logger: logger,
	}
}

// RegisterTool 注册新工具到注册表
func (r *ToolRegistry) RegisterTool(tool core.Tool) error {
	if tool == nil {
		return fmt.Errorf("tool cannot be nil")
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	name := tool.Name()
	if name == "" {
		return fmt.Errorf("tool name cannot be empty")
	}

	if _, exists := r.tools[name]; exists {
		return fmt.Errorf("tool with name '%s' already exists", name)
	}

	r.tools[name] = tool

	r.logger.Info("Tool registered",
		zap.String("name", name),
		zap.String("description", tool.Description()),
	)

	return nil
}

// GetTool 根据名称获取工具实例
func (r *ToolRegistry) GetTool(name string) (core.Tool, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	tool, exists := r.tools[name]
	if !exists {
		return nil, fmt.Errorf("tool '%s' not found", name)
	}

	return tool, nil
}

// ListTools 列出所有已注册的工具
func (r *ToolRegistry) ListTools() []core.Tool {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	tools := make([]core.Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}

	return tools
}

// UnregisterTool 注销指定名称的工具
func (r *ToolRegistry) UnregisterTool(name string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if _, exists := r.tools[name]; !exists {
		return fmt.Errorf("tool '%s' not found", name)
	}

	delete(r.tools, name)

	r.logger.Info("Tool unregistered", zap.String("name", name))

	return nil
}

// GetToolCount 获取已注册工具的数量
func (r *ToolRegistry) GetToolCount() int {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	return len(r.tools)
}

// ToolExecutor 工具执行器，负责调用工具并记录执行日志
type ToolExecutor struct {
	registry *ToolRegistry // 工具注册表
	logger   *zap.Logger   // 日志记录器
}

// NewToolExecutor 创建工具执行器实例
func NewToolExecutor(registry *ToolRegistry, logger *zap.Logger) *ToolExecutor {
	return &ToolExecutor{
		registry: registry,
		logger:   logger,
	}
}

// ExecuteTool 执行指定工具并返回结果
func (e *ToolExecutor) ExecuteTool(ctx context.Context, toolName, input string) (string, error) {
	startTime := time.Now()

	tool, err := e.registry.GetTool(toolName)
	if err != nil {
		return "", fmt.Errorf("failed to get tool: %w", err)
	}

	result, err := tool.Execute(ctx, input)

	executionTime := time.Since(startTime)

	if err != nil {
		e.logger.Error("Tool execution failed",
			zap.String("tool", toolName),
			zap.String("input", input),
			zap.Duration("execution_time", executionTime),
			zap.Error(err),
		)
		return "", fmt.Errorf("tool execution failed: %w", err)
	}

	e.logger.Debug("Tool executed successfully",
		zap.String("tool", toolName),
		zap.Duration("execution_time", executionTime),
	)

	return result, nil
}

// ExecuteToolWithTimeout 执行工具并设置超时时间
func (e *ToolExecutor) ExecuteToolWithTimeout(ctx context.Context, toolName, input string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return e.ExecuteTool(ctx, toolName, input)
}

// BatchExecuteTools 批量执行多个工具请求
func (e *ToolExecutor) BatchExecuteTools(ctx context.Context, requests []ToolRequest) ([]ToolResult, error) {
	results := make([]ToolResult, len(requests))

	for i, request := range requests {
		result, err := e.ExecuteTool(ctx, request.ToolName, request.Input)

		results[i] = ToolResult{
			ToolName: request.ToolName,
			Input:    request.Input,
			Output:   result,
			Error:    err,
		}
	}

	return results, nil
}

// ToolRequest 工具请求结构体，包含工具名称和输入内容
type ToolRequest struct {
	ToolName string `json:"tool_name"` // 工具名称
	Input    string `json:"input"`     // 输入内容
}

// ToolResult 工具执行结果结构体，包含工具名称、输入、输出和错误信息
type ToolResult struct {
	ToolName string `json:"tool_name"`       // 工具名称
	Input    string `json:"input"`           // 输入内容
	Output   string `json:"output"`          // 输出内容
	Error    error  `json:"error,omitempty"` // 错误信息（可选）
}
