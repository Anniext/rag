// 本文件实现了 LangChain 的链式构建器和执行器，支持 SQL 生成、查询解释、错误分析、建议生成、查询优化等多种链式任务。
// 主要功能：
// 1. ChainBuilder：链式配置构建器，支持链类型、LLM 配置、内存配置、自定义选项等参数设置。
// 2. ChainExecutor：链执行器，负责链的创建和执行，封装了多种链式任务的执行方法。
// 3. 支持 SQL 生成、查询解释、错误分析、建议生成、查询优化等典型场景。
// 4. 每个方法均有详细注释，便于理解和维护。
// 5. 结构体字段和关键逻辑均有中文说明。

package langchain

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"go.uber.org/zap"

	"pumppill/rag/core"
)

// baseChain 基础 Chain 实现，作为所有链的通用父类，封装了链的通用属性和方法
type baseChain struct {
	chainType        string            // 链类型（如 sql_generation、query_explanation 等）
	llmClient        LLMClient         // LLM 客户端，用于与大语言模型交互
	templateRenderer *TemplateRenderer // 模板渲染器，用于生成提示词或请求内容
	logger           *zap.Logger       // 日志记录器
	config           *core.ChainConfig // 链的配置参数
	tools            []core.Tool       // 工具列表，每个链可持有多个工具
	mutex            sync.RWMutex      // 读写锁，保护 tools 并发安全
}

// GetTools 获取工具列表，返回当前链持有的所有工具的副本，保证并发安全
func (c *baseChain) GetTools() []core.Tool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	// 返回副本，避免外部修改原始工具列表
	tools := make([]core.Tool, len(c.tools))
	copy(tools, c.tools)

	return tools
}

// AddTool 添加工具到链中，若工具已存在则返回错误，保证工具唯一性
func (c *baseChain) AddTool(tool core.Tool) error {
	if tool == nil {
		return fmt.Errorf("tool cannot be nil")
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	// 检查是否已存在同名工具，避免重复添加
	for _, existingTool := range c.tools {
		if existingTool.Name() == tool.Name() {
			return fmt.Errorf("tool with name '%s' already exists", tool.Name())
		}
	}

	c.tools = append(c.tools, tool)

	c.logger.Debug("Tool added to chain",
		zap.String("chain_type", c.chainType),
		zap.String("tool_name", tool.Name()),
	)

	return nil
}

// sqlGenerationChain SQL 生成 Chain，负责根据自然语言查询生成 SQL 语句
type sqlGenerationChain struct {
	baseChain
}

// Run 执行 SQL 生成 Chain，根据输入的查询和 schema 生成 SQL 语句
func (c *sqlGenerationChain) Run(ctx context.Context, input *core.ChainInput) (*core.ChainOutput, error) {
	if input == nil {
		return nil, fmt.Errorf("input is required")
	}

	if input.Query == "" {
		return nil, fmt.Errorf("query is required")
	}

	c.logger.Debug("Running SQL generation chain", zap.String("query", input.Query))

	// 查找 SQL 生成工具
	var sqlTool core.Tool
	for _, tool := range c.GetTools() {
		if tool.Name() == "sql_generator" {
			sqlTool = tool
			break
		}
	}

	if sqlTool == nil {
		return nil, fmt.Errorf("SQL generator tool not found")
	}

	// 准备工具输入，将查询和 schema 拼接为字符串
	toolInput := fmt.Sprintf("Query: %s", input.Query)
	if input.Schema != nil {
		toolInput += fmt.Sprintf("\nSchema: %v", input.Schema)
	}

	// 执行工具，生成 SQL
	result, err := sqlTool.Execute(ctx, toolInput)
	if err != nil {
		return nil, fmt.Errorf("SQL generation failed: %w", err)
	}

	// 解析结果，去除首尾空格
	sql := strings.TrimSpace(result)

	// 构造输出结果，包含生成的 SQL 和原始查询
	output := &core.ChainOutput{
		Result: map[string]interface{}{
			"sql":   sql,
			"query": input.Query,
		},
		Metadata: map[string]interface{}{
			"chain_type": "sql_generation",
			"tool_used":  "sql_generator",
		},
	}

	c.logger.Debug("SQL generation chain completed", zap.String("sql", sql))

	return output, nil
}

// queryExplanationChain 查询解释 Chain，负责对 SQL 查询进行解释说明
type queryExplanationChain struct {
	baseChain
}

// Run 执行查询解释 Chain，根据 SQL 和结果生成解释说明
func (c *queryExplanationChain) Run(ctx context.Context, input *core.ChainInput) (*core.ChainOutput, error) {
	if input == nil {
		return nil, fmt.Errorf("input is required")
	}

	// 从变量中获取 SQL
	sql, ok := input.Variables["sql"].(string)
	if !ok || sql == "" {
		return nil, fmt.Errorf("sql is required in variables")
	}

	c.logger.Debug("Running query explanation chain", zap.String("sql", sql))

	// 查找查询解释工具
	var explanationTool core.Tool
	for _, tool := range c.GetTools() {
		if tool.Name() == "query_explanation" {
			explanationTool = tool
			break
		}
	}

	if explanationTool == nil {
		return nil, fmt.Errorf("query explanation tool not found")
	}

	// 准备工具输入，将 SQL 和结果拼接为字符串
	toolInput := fmt.Sprintf("SQL: %s", sql)
	if results, ok := input.Variables["results"]; ok {
		toolInput += fmt.Sprintf("\nResults: %v", results)
	}

	// 执行工具，生成解释说明
	result, err := explanationTool.Execute(ctx, toolInput)
	if err != nil {
		return nil, fmt.Errorf("query explanation failed: %w", err)
	}

	explanation := strings.TrimSpace(result)

	// 构造输出结果，包含解释说明和 SQL
	output := &core.ChainOutput{
		Result: map[string]interface{}{
			"explanation": explanation,
			"sql":         sql,
		},
		Metadata: map[string]interface{}{
			"chain_type": "query_explanation",
			"tool_used":  "query_explanation",
		},
	}

	c.logger.Debug("Query explanation chain completed")

	return output, nil
}

// errorAnalysisChain 错误分析 Chain，负责分析 SQL 执行错误原因
type errorAnalysisChain struct {
	baseChain
}

// Run 执行错误分析 Chain，根据查询、SQL 和错误信息分析原因
func (c *errorAnalysisChain) Run(ctx context.Context, input *core.ChainInput) (*core.ChainOutput, error) {
	if input == nil {
		return nil, fmt.Errorf("input is required")
	}

	// 获取查询、SQL 和错误信息
	query, ok := input.Variables["query"].(string)
	if !ok || query == "" {
		return nil, fmt.Errorf("query is required in variables")
	}

	sql, ok := input.Variables["sql"].(string)
	if !ok || sql == "" {
		return nil, fmt.Errorf("sql is required in variables")
	}

	errorMsg, ok := input.Variables["error"].(string)
	if !ok || errorMsg == "" {
		return nil, fmt.Errorf("error is required in variables")
	}

	c.logger.Debug("Running error analysis chain",
		zap.String("query", query),
		zap.String("sql", sql),
		zap.String("error", errorMsg),
	)

	// 查找错误分析工具
	var errorTool core.Tool
	for _, tool := range c.GetTools() {
		if tool.Name() == "error_analysis" {
			errorTool = tool
			break
		}
	}

	if errorTool == nil {
		return nil, fmt.Errorf("error analysis tool not found")
	}

	// 准备工具输入，将查询、SQL 和错误信息拼接为字符串
	toolInput := fmt.Sprintf("Query: %s\nSQL: %s\nError: %s", query, sql, errorMsg)

	// 执行工具，分析错误原因
	result, err := errorTool.Execute(ctx, toolInput)
	if err != nil {
		return nil, fmt.Errorf("error analysis failed: %w", err)
	}

	analysis := strings.TrimSpace(result)

	// 构造输出结果，包含分析结果、查询、SQL 和错误信息
	output := &core.ChainOutput{
		Result: map[string]interface{}{
			"analysis": analysis,
			"query":    query,
			"sql":      sql,
			"error":    errorMsg,
		},
		Metadata: map[string]interface{}{
			"chain_type": "error_analysis",
			"tool_used":  "error_analysis",
		},
	}

	c.logger.Debug("Error analysis chain completed")

	return output, nil
}

// suggestionChain 建议 Chain，负责根据部分查询和 schema 生成补全建议
type suggestionChain struct {
	baseChain
}

// Run 执行建议 Chain，根据部分查询和 schema 生成补全建议
func (c *suggestionChain) Run(ctx context.Context, input *core.ChainInput) (*core.ChainOutput, error) {
	if input == nil {
		return nil, fmt.Errorf("input is required")
	}

	// 获取部分查询
	partialQuery, ok := input.Variables["partial_query"].(string)
	if !ok || partialQuery == "" {
		return nil, fmt.Errorf("partial_query is required in variables")
	}

	c.logger.Debug("Running suggestion chain", zap.String("partial_query", partialQuery))

	// 查找建议工具
	var suggestionTool core.Tool
	for _, tool := range c.GetTools() {
		if tool.Name() == "suggestion" {
			suggestionTool = tool
			break
		}
	}

	if suggestionTool == nil {
		return nil, fmt.Errorf("suggestion tool not found")
	}

	// 准备工具输入，将部分查询和 schema 拼接为字符串
	toolInput := fmt.Sprintf("PartialQuery: %s", partialQuery)
	if input.Schema != nil {
		toolInput += fmt.Sprintf("\nSchema: %v", input.Schema)
	}

	// 执行工具，���成建议
	result, err := suggestionTool.Execute(ctx, toolInput)
	if err != nil {
		return nil, fmt.Errorf("suggestion generation failed: %w", err)
	}

	suggestions := strings.TrimSpace(result)

	// 解析建议，按行分割并去除空行和前缀
	suggestionList := strings.Split(suggestions, "\n")
	var parsedSuggestions []string
	for _, suggestion := range suggestionList {
		suggestion = strings.TrimSpace(suggestion)
		if suggestion != "" && !strings.HasPrefix(suggestion, "建议:") {
			parsedSuggestions = append(parsedSuggestions, suggestion)
		}
	}

	// 构造输出结果，包含建议列表和部分查询
	output := &core.ChainOutput{
		Result: map[string]interface{}{
			"suggestions":   parsedSuggestions,
			"partial_query": partialQuery,
		},
		Metadata: map[string]interface{}{
			"chain_type": "suggestion",
			"tool_used":  "suggestion",
		},
	}

	c.logger.Debug("Suggestion chain completed", zap.Int("suggestion_count", len(parsedSuggestions)))

	return output, nil
}

// optimizationChain 优化 Chain，负责对 SQL 查询进行优化分析
type optimizationChain struct {
	baseChain
}

// Run 执行优化 Chain，根据 SQL、执行计划和指标生成优化建议
func (c *optimizationChain) Run(ctx context.Context, input *core.ChainInput) (*core.ChainOutput, error) {
	if input == nil {
		return nil, fmt.Errorf("input is required")
	}

	// 获取 SQL
	sql, ok := input.Variables["sql"].(string)
	if !ok || sql == "" {
		return nil, fmt.Errorf("sql is required in variables")
	}

	c.logger.Debug("Running optimization chain", zap.String("sql", sql))

	// 查找优化工具
	var optimizationTool core.Tool
	for _, tool := range c.GetTools() {
		if tool.Name() == "optimization" {
			optimizationTool = tool
			break
		}
	}

	if optimizationTool == nil {
		return nil, fmt.Errorf("optimization tool not found")
	}

	// 准备工具输入，将 SQL、执行计划和指标拼接为字符串
	toolInput := fmt.Sprintf("SQL: %s", sql)
	if executionPlan, ok := input.Variables["execution_plan"]; ok {
		toolInput += fmt.Sprintf("\nExecutionPlan: %v", executionPlan)
	}
	if metrics, ok := input.Variables["metrics"]; ok {
		toolInput += fmt.Sprintf("\nMetrics: %v", metrics)
	}

	// 执行工具，生成优化建议
	result, err := optimizationTool.Execute(ctx, toolInput)
	if err != nil {
		return nil, fmt.Errorf("optimization analysis failed: %w", err)
	}

	optimization := strings.TrimSpace(result)

	// 构造输出结果，包含优化建议和 SQL
	output := &core.ChainOutput{
		Result: map[string]interface{}{
			"optimization": optimization,
			"sql":          sql,
		},
		Metadata: map[string]interface{}{
			"chain_type": "optimization",
			"tool_used":  "optimization",
		},
	}

	c.logger.Debug("Optimization chain completed")

	return output, nil
}

// MockChain 模拟 Chain（用于测试），可模拟不同类型链的输出
type MockChain struct {
	chainType string       // 链类型
	tools     []core.Tool  // 工具列表
	mutex     sync.RWMutex // 读写锁，保护 tools 并发安全
}

// NewMockChain 创建模拟 Chain，指定链类型
func NewMockChain(chainType string) *MockChain {
	return &MockChain{
		chainType: chainType,
		tools:     make([]core.Tool, 0),
	}
}

// Run 执行模拟 Chain，根据链类型返回不同的模拟输出
func (c *MockChain) Run(ctx context.Context, input *core.ChainInput) (*core.ChainOutput, error) {
	if input == nil {
		return nil, fmt.Errorf("input is required")
	}

	// 模拟不同类型的输出
	var result interface{}

	switch c.chainType {
	case "sql_generation":
		result = map[string]interface{}{
			"sql":   "SELECT * FROM users WHERE name = 'test'",
			"query": input.Query,
		}
	case "query_explanation":
		result = map[string]interface{}{
			"explanation": "This query selects all users with name 'test'",
			"sql":         input.Variables["sql"],
		}
	case "error_analysis":
		result = map[string]interface{}{
			"analysis": "The error is caused by missing table",
			"query":    input.Variables["query"],
			"sql":      input.Variables["sql"],
			"error":    input.Variables["error"],
		}
	case "suggestion":
		result = map[string]interface{}{
			"suggestions": []string{
				"SELECT * FROM users",
				"SELECT name FROM users",
				"SELECT id, name FROM users",
			},
			"partial_query": input.Variables["partial_query"],
		}
	case "optimization":
		result = map[string]interface{}{
			"optimization": "Add index on name column for better performance",
			"sql":          input.Variables["sql"],
		}
	default:
		result = map[string]interface{}{
			"message": fmt.Sprintf("Mock result for %s chain", c.chainType),
		}
	}

	return &core.ChainOutput{
		Result: result,
		Metadata: map[string]interface{}{
			"chain_type": c.chainType,
			"mock":       true,
		},
	}, nil
}

// GetTools 获取工具列表，返回副本，保证并发安全
func (c *MockChain) GetTools() []core.Tool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	tools := make([]core.Tool, len(c.tools))
	copy(tools, c.tools)

	return tools
}

// AddTool 添加工具到模拟链中
func (c *MockChain) AddTool(tool core.Tool) error {
	if tool == nil {
		return fmt.Errorf("tool cannot be nil")
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.tools = append(c.tools, tool)
	return nil
}

// ChainBuilder Chain 构建器，负责链配置的构建
// manager：链管理器，负责链的生命周期管理
// logger：日志记录器
// 提供多种链式任务的执行方法
// 典型任务包括 SQL 生成、查询解释、错误分析、建议生成、查询优化等
// 每个方法均有详细参数说明和错误处理
// 便于扩展和维护
type ChainBuilder struct {
	config *core.ChainConfig // 链配置
}

// NewChainBuilder 创建 Chain 构建器，初始化配置
func NewChainBuilder() *ChainBuilder {
	return &ChainBuilder{
		config: &core.ChainConfig{
			Options: make(map[string]interface{}),
		},
	}
}

// WithType 设置链类型
func (b *ChainBuilder) WithType(chainType string) *ChainBuilder {
	b.config.Type = chainType // 设置链类型
	return b
}

// WithLLMConfig 设置 LLM 配置
func (b *ChainBuilder) WithLLMConfig(llmConfig *core.LLMConfig) *ChainBuilder {
	b.config.LLMConfig = llmConfig // 设置 LLM 配置
	return b
}

// WithTools 设置工具列表
func (b *ChainBuilder) WithTools(tools ...string) *ChainBuilder {
	b.config.Tools = tools // 设置工具列表
	return b
}

// WithMemory 设置内存配置
func (b *ChainBuilder) WithMemory(memoryConfig *core.MemoryConfig) *ChainBuilder {
	b.config.Memory = memoryConfig // 设置内存配置
	return b
}

// WithOption 设置自定义选项
func (b *ChainBuilder) WithOption(key string, value interface{}) *ChainBuilder {
	b.config.Options[key] = value // 设置自定义选项
	return b
}

// Build 构建链配置，返回配置对象
func (b *ChainBuilder) Build() *core.ChainConfig {
	return b.config // 返回链配置
}

// ChainExecutor Chain 执行器，负责链的创建和执行
// manager：链管理器，负责链的生命周期管理
// logger：日志记录器
// 提供多种链式任务的执行方法
// 典型任务包括 SQL 生成、查询解释、错误分析、建议生成、查询优化等
// 每个方法均有详细参数说明和错误处理
// 便于扩展和维护
type ChainExecutor struct {
	manager core.LangChainManager // 链管理器
	logger  *zap.Logger           // 日志记录器
}

// NewChainExecutor 创建 Chain 执行器
func NewChainExecutor(manager core.LangChainManager, logger *zap.Logger) *ChainExecutor {
	return &ChainExecutor{
		manager: manager,
		logger:  logger,
	}
}

// ExecuteSQLGeneration 执行 SQL 生成链，生成 SQL 语句
func (e *ChainExecutor) ExecuteSQLGeneration(ctx context.Context, query string, schema *core.SchemaInfo) (*core.ChainOutput, error) {
	config := NewChainBuilder().
		WithType("sql_generation").
		WithLLMConfig(&core.LLMConfig{
			Provider: "mock",
			Model:    "gpt-3.5-turbo",
		}).
		Build()

	chain, err := e.manager.CreateChain(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create SQL generation chain: %w", err)
	}

	input := &core.ChainInput{
		Query:  query,
		Schema: schema,
	}

	return e.manager.ExecuteChain(ctx, chain, input)
}

// ExecuteQueryExplanation 执行查询解释链，生成 SQL 解释说明
func (e *ChainExecutor) ExecuteQueryExplanation(ctx context.Context, sql string, results interface{}) (*core.ChainOutput, error) {
	config := NewChainBuilder().
		WithType("query_explanation").
		WithLLMConfig(&core.LLMConfig{
			Provider: "mock",
			Model:    "gpt-3.5-turbo",
		}).
		Build()

	chain, err := e.manager.CreateChain(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create query explanation chain: %w", err)
	}

	input := &core.ChainInput{
		Variables: map[string]interface{}{
			"sql":     sql,
			"results": results,
		},
	}

	return e.manager.ExecuteChain(ctx, chain, input)
}

// ExecuteErrorAnalysis 执行错误分析链，分析 SQL 执行错误
func (e *ChainExecutor) ExecuteErrorAnalysis(ctx context.Context, query, sql, errorMsg string) (*core.ChainOutput, error) {
	config := NewChainBuilder().
		WithType("error_analysis").
		WithLLMConfig(&core.LLMConfig{
			Provider: "mock",
			Model:    "gpt-3.5-turbo",
		}).
		Build()

	chain, err := e.manager.CreateChain(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create error analysis chain: %w", err)
	}

	input := &core.ChainInput{
		Variables: map[string]interface{}{
			"query": query,
			"sql":   sql,
			"error": errorMsg,
		},
	}

	return e.manager.ExecuteChain(ctx, chain, input)
}

// ExecuteSuggestion 执行建议生成链，生成查询补全建议
func (e *ChainExecutor) ExecuteSuggestion(ctx context.Context, partialQuery string, schema *core.SchemaInfo) (*core.ChainOutput, error) {
	config := NewChainBuilder().
		WithType("suggestion").
		WithLLMConfig(&core.LLMConfig{
			Provider: "mock",
			Model:    "gpt-3.5-turbo",
		}).
		Build()

	chain, err := e.manager.CreateChain(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create suggestion chain: %w", err)
	}

	input := &core.ChainInput{
		Schema: schema,
		Variables: map[string]interface{}{
			"partial_query": partialQuery,
		},
	}

	return e.manager.ExecuteChain(ctx, chain, input)
}

// ExecuteOptimization 执行查询优化链，生成 SQL 优化建议
func (e *ChainExecutor) ExecuteOptimization(ctx context.Context, sql string, executionPlan, metrics interface{}) (*core.ChainOutput, error) {
	config := NewChainBuilder().
		WithType("optimization").
		WithLLMConfig(&core.LLMConfig{
			Provider: "mock",
			Model:    "gpt-3.5-turbo",
		}).
		Build()

	chain, err := e.manager.CreateChain(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create optimization chain: %w", err)
	}

	input := &core.ChainInput{
		Variables: map[string]interface{}{
			"sql":            sql,
			"execution_plan": executionPlan,
			"metrics":        metrics,
		},
	}

	return e.manager.ExecuteChain(ctx, chain, input)
}
