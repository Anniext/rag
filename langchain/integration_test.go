package langchain

import (
	"context"
	"os"
	"testing"
	"time"

	"pumppill/rag/core"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

// LangChainIntegrationTestSuite LangChain 集成测试套件
type LangChainIntegrationTestSuite struct {
	suite.Suite
	ctx             context.Context
	cancel          context.CancelFunc
	logger          *zap.Logger
	manager         *Manager
	templateManager *TemplateManager
	toolRegistry    *ToolRegistry
	client          LLMClient
}

// SetupSuite 设置测试套件
func (suite *LangChainIntegrationTestSuite) SetupSuite() {
	// 检查是否启用 LLM 集成测试
	if os.Getenv("LLM_INTEGRATION_TEST") != "true" {
		suite.T().Skip("跳过 LLM 集成测试，设置 LLM_INTEGRATION_TEST=true 启用")
	}

	suite.ctx, suite.cancel = context.WithTimeout(context.Background(), 2*time.Minute)

	// 创建日志记录器
	var err error
	suite.logger, err = zap.NewDevelopment()
	require.NoError(suite.T(), err)

	// 创建组件
	suite.setupComponents()
}

// TearDownSuite 清理测试套件
func (suite *LangChainIntegrationTestSuite) TearDownSuite() {
	if suite.cancel != nil {
		suite.cancel()
	}

	if suite.logger != nil {
		suite.logger.Sync()
	}
}

// setupComponents 设置组件
func (suite *LangChainIntegrationTestSuite) setupComponents() {
	// 创建模板管理器
	suite.templateManager = NewTemplateManager(suite.logger)

	// 创建工具注册表
	suite.toolRegistry = NewToolRegistry(suite.logger)

	// 创建 LLM 客户端（使用 Mock 客户端进行测试）
	suite.client = NewMockLLMClient()

	// 创建 LangChain 管理器
	config := &Config{
		Provider:    "mock",
		Model:       "test-model",
		Temperature: 0.1,
		MaxTokens:   2048,
		Timeout:     30 * time.Second,
	}
	suite.manager = NewManager(config, suite.logger)
}

// TestTemplateRendering 测试模板渲染
func (suite *LangChainIntegrationTestSuite) TestTemplateRendering() {
	// 测试 SQL 生成模板
	template := suite.templateManager.GetSQLGenerationTemplate()
	assert.NotNil(suite.T(), template)

	variables := map[string]interface{}{
		"Query": "查找所有用户",
		"Schema": map[string]interface{}{
			"tables": []string{"users", "posts"},
		},
	}

	rendered, err := template.Render(variables)
	assert.NoError(suite.T(), err)
	assert.Contains(suite.T(), rendered, "查找所有用户")
	assert.Contains(suite.T(), rendered, "users")

	// 测试查询解释模板
	explanationTemplate := suite.templateManager.GetQueryExplanationTemplate()
	assert.NotNil(suite.T(), explanationTemplate)

	variables = map[string]interface{}{
		"SQL":     "SELECT * FROM users",
		"Results": []map[string]interface{}{{"id": 1, "name": "张三"}},
	}

	rendered, err = explanationTemplate.Render(variables)
	assert.NoError(suite.T(), err)
	assert.Contains(suite.T(), rendered, "SELECT * FROM users")
}

// TestToolRegistration 测试工具注册
func (suite *LangChainIntegrationTestSuite) TestToolRegistration() {
	// 创建测试工具
	sqlTool := NewSQLGeneratorTool(suite.client, nil, suite.logger)
	explanationTool := NewQueryExplanationTool(suite.client, nil, suite.logger)

	// 注册工具
	err := suite.toolRegistry.RegisterTool(sqlTool)
	assert.NoError(suite.T(), err)

	err = suite.toolRegistry.RegisterTool(explanationTool)
	assert.NoError(suite.T(), err)

	// 验证工具已注册
	assert.Equal(suite.T(), 2, suite.toolRegistry.GetToolCount())

	// 获取工具
	retrievedTool, err := suite.toolRegistry.GetTool("sql_generator")
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "sql_generator", retrievedTool.Name())

	// 列出所有工具
	tools := suite.toolRegistry.ListTools()
	assert.Len(suite.T(), tools, 2)
}

// TestSQLGenerationTool 测试 SQL 生成工具
func (suite *LangChainIntegrationTestSuite) TestSQLGenerationTool() {
	// 设置 Mock 客户端响应
	mockClient := suite.client.(*MockLLMClient)
	mockClient.AddResponse(&core.LLMResponse{
		Content: "SELECT * FROM users WHERE age > 25",
		Usage: &core.TokenUsage{
			PromptTokens:     50,
			CompletionTokens: 20,
			TotalTokens:      70,
		},
	})

	// 创建 SQL 生成工具
	tool := NewSQLGeneratorTool(suite.client, nil, suite.logger)

	// 执行工具
	input := "Query: 查找年龄大于25的用户\nSchema: CREATE TABLE users (id INT, name VARCHAR(100), age INT)"
	result, err := tool.Execute(suite.ctx, input)

	assert.NoError(suite.T(), err)
	assert.Contains(suite.T(), result, "SELECT")
	assert.Contains(suite.T(), result, "users")
	assert.Contains(suite.T(), result, "age > 25")
}

// TestQueryExplanationTool 测试查询解释工具
func (suite *LangChainIntegrationTestSuite) TestQueryExplanationTool() {
	// 设置 Mock 客户端响应
	mockClient := suite.client.(*MockLLMClient)
	mockClient.AddResponse(&core.LLMResponse{
		Content: "这个查询从用户表中选择所有年龄大于25岁的用户记录。",
		Usage: &core.TokenUsage{
			PromptTokens:     60,
			CompletionTokens: 30,
			TotalTokens:      90,
		},
	})

	// 创建查询解释工具
	tool := NewQueryExplanationTool(suite.client, nil, suite.logger)

	// 执行工具
	input := "SQL: SELECT * FROM users WHERE age > 25\nResults: [{'id': 1, 'name': '张三', 'age': 30}]"
	result, err := tool.Execute(suite.ctx, input)

	assert.NoError(suite.T(), err)
	assert.Contains(suite.T(), result, "查询")
	assert.Contains(suite.T(), result, "用户")
	assert.Contains(suite.T(), result, "25")
}

// TestErrorAnalysisTool 测试错误分析工具
func (suite *LangChainIntegrationTestSuite) TestErrorAnalysisTool() {
	// 设置 Mock 客户端响应
	mockClient := suite.client.(*MockLLMClient)
	mockClient.AddResponse(&core.LLMResponse{
		Content: "错误原因：表名 'user' 不存在。建议：使用正确的表名 'users'。",
		Usage: &core.TokenUsage{
			PromptTokens:     70,
			CompletionTokens: 25,
			TotalTokens:      95,
		},
	})

	// 创建错误分析工具
	tool := NewErrorAnalysisTool(suite.client, nil, suite.logger)

	// 执行工具
	input := "Query: 查找用户\nSQL: SELECT * FROM user\nError: Table 'user' doesn't exist"
	result, err := tool.Execute(suite.ctx, input)

	assert.NoError(suite.T(), err)
	assert.Contains(suite.T(), result, "错误")
	assert.Contains(suite.T(), result, "表名")
	assert.Contains(suite.T(), result, "建议")
}

// TestChainCreation 测试链创建
func (suite *LangChainIntegrationTestSuite) TestChainCreation() {
	// 创建链配置
	chainConfig := &core.ChainConfig{
		Type: "sql_generation",
		LLMConfig: &core.LLMConfig{
			Provider:    "mock",
			Model:       "test-model",
			Temperature: 0.1,
			MaxTokens:   2048,
		},
		Tools:  []string{"sql_generator"},
		Memory: &core.MemoryConfig{Type: "buffer", Size: 10},
	}

	// 创建链
	chain, err := suite.manager.CreateChain(suite.ctx, chainConfig)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), chain)

	// 验证链的工具
	tools := chain.GetTools()
	assert.Len(suite.T(), tools, 1)
	assert.Equal(suite.T(), "sql_generator", tools[0].Name())
}

// TestChainExecution 测试链执行
func (suite *LangChainIntegrationTestSuite) TestChainExecution() {
	// 设置 Mock 客户端响应
	mockClient := suite.client.(*MockLLMClient)
	mockClient.AddResponse(&core.LLMResponse{
		Content: "SELECT name, age FROM users WHERE age > 25 ORDER BY age DESC",
		Usage: &core.TokenUsage{
			PromptTokens:     80,
			CompletionTokens: 35,
			TotalTokens:      115,
		},
	})

	// 创建链配置
	chainConfig := &core.ChainConfig{
		Type: "sql_generation",
		LLMConfig: &core.LLMConfig{
			Provider:    "mock",
			Model:       "test-model",
			Temperature: 0.1,
		},
		Tools: []string{"sql_generator"},
	}

	// 创建链
	chain, err := suite.manager.CreateChain(suite.ctx, chainConfig)
	require.NoError(suite.T(), err)

	// 执行链
	input := &core.ChainInput{
		Query: "查找年龄大于25的用户，按年龄降序排列",
		Context: map[string]any{
			"schema": "CREATE TABLE users (id INT, name VARCHAR(100), age INT)",
		},
	}

	output, err := suite.manager.ExecuteChain(suite.ctx, chain, input)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), output)
	assert.NotNil(suite.T(), output.Result)
	assert.NotNil(suite.T(), output.Usage)
	assert.True(suite.T(), output.Usage.TotalTokens > 0)
}

// TestMemoryManagement 测试内存管理
func (suite *LangChainIntegrationTestSuite) TestMemoryManagement() {
	sessionID := "test_session_123"

	// 获取内存（应该创建新的）
	memory, err := suite.manager.GetMemory(sessionID)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), memory)

	// 添加历史记录
	history := &core.QueryHistory{
		Query:     "查找用户",
		SQL:       "SELECT * FROM users",
		Success:   true,
		Timestamp: time.Now(),
	}
	memory.AddHistory(history)

	// 设置上下文
	memory.SetContext("last_query", "users")

	// 更新内存
	err = suite.manager.UpdateMemory(sessionID, memory)
	assert.NoError(suite.T(), err)

	// 重新获取内存并验证
	retrievedMemory, err := suite.manager.GetMemory(sessionID)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), retrievedMemory.GetHistory(), 1)
	assert.Equal(suite.T(), "users", retrievedMemory.GetContext()["last_query"])
}

// TestConcurrentToolExecution 测试并发工具执行
func (suite *LangChainIntegrationTestSuite) TestConcurrentToolExecution() {
	const numGoroutines = 5
	const numExecutions = 3

	// 设置 Mock 客户端响应
	mockClient := suite.client.(*MockLLMClient)
	for i := 0; i < numGoroutines*numExecutions; i++ {
		mockClient.AddResponse(&core.LLMResponse{
			Content: "SELECT * FROM users",
			Usage: &core.TokenUsage{
				PromptTokens:     50,
				CompletionTokens: 20,
				TotalTokens:      70,
			},
		})
	}

	// 创建工具
	tool := NewSQLGeneratorTool(suite.client, nil, suite.logger)

	// 并发执行
	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer func() { done <- true }()

			for j := 0; j < numExecutions; j++ {
				input := "Query: 查找用户\nSchema: CREATE TABLE users (id INT, name VARCHAR(100))"
				result, err := tool.Execute(suite.ctx, input)
				assert.NoError(suite.T(), err)
				assert.NotEmpty(suite.T(), result)
			}
		}(i)
	}

	// 等待所有 goroutine 完成
	for i := 0; i < numGoroutines; i++ {
		<-done
	}
}

// TestToolTimeout 测试工具超时
func (suite *LangChainIntegrationTestSuite) TestToolTimeout() {
	// 创建一个会超时的 Mock 客户端
	timeoutClient := &TimeoutMockLLMClient{
		delay: 2 * time.Second,
	}

	// 创建工具
	tool := NewSQLGeneratorTool(timeoutClient, nil, suite.logger)

	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(suite.ctx, 1*time.Second)
	defer cancel()

	// 执行工具（应该超时）
	input := "Query: 查找用户\nSchema: CREATE TABLE users (id INT, name VARCHAR(100))"
	_, err := tool.Execute(ctx, input)
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "context deadline exceeded")
}

// TestErrorHandling 测试错误处理
func (suite *LangChainIntegrationTestSuite) TestErrorHandling() {
	// 创建一个会返回错误的 Mock 客户端
	errorClient := &ErrorMockLLMClient{
		errorMessage: "LLM service unavailable",
	}

	// 创建工具
	tool := NewSQLGeneratorTool(errorClient, nil, suite.logger)

	// 执行工具（应该返回错误）
	input := "Query: 查找用户\nSchema: CREATE TABLE users (id INT, name VARCHAR(100))"
	_, err := tool.Execute(suite.ctx, input)
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "LLM service unavailable")
}

// TestTokenUsageTracking 测试 Token 使用统计
func (suite *LangChainIntegrationTestSuite) TestTokenUsageTracking() {
	// 设置 Mock 客户端响应
	mockClient := suite.client.(*MockLLMClient)
	mockClient.AddResponse(&core.LLMResponse{
		Content: "SELECT * FROM users",
		Usage: &core.TokenUsage{
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
		},
	})

	// 创建工具
	tool := NewSQLGeneratorTool(suite.client, nil, suite.logger)

	// 执行工具
	input := "Query: 查找用户\nSchema: CREATE TABLE users (id INT, name VARCHAR(100))"
	_, err := tool.Execute(suite.ctx, input)
	assert.NoError(suite.T(), err)

	// 验证 Token 使用统计
	// 这里应该检查 Token 使用情况，但需要工具支持返回使用统计
	// 在实际实现中，可以通过工具的元数据或日志来验证
}

// TimeoutMockLLMClient 超时 Mock LLM 客户端
type TimeoutMockLLMClient struct {
	delay time.Duration
}

func (c *TimeoutMockLLMClient) GenerateContent(ctx context.Context, input string) (*core.LLMResponse, error) {
	select {
	case <-time.After(c.delay):
		return &core.LLMResponse{Content: "SELECT * FROM users"}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// ErrorMockLLMClient 错误 Mock LLM 客户端
type ErrorMockLLMClient struct {
	errorMessage string
}

func (c *ErrorMockLLMClient) GenerateContent(ctx context.Context, input string) (*core.LLMResponse, error) {
	return nil, core.NewRAGError(core.ErrorTypeLLM, "LLM_ERROR", c.errorMessage)
}

// TestLangChainIntegration 运行 LangChain 集成测试套件
func TestLangChainIntegration(t *testing.T) {
	suite.Run(t, new(LangChainIntegrationTestSuite))
}

// BenchmarkToolExecution 工具执行基准测试
func BenchmarkToolExecution(b *testing.B) {
	if os.Getenv("LLM_INTEGRATION_TEST") != "true" {
		b.Skip("跳过 LLM 集成基准测试，设置 LLM_INTEGRATION_TEST=true 启用")
	}

	logger, _ := zap.NewDevelopment()
	client := NewMockLLMClient()

	// 设置响应
	mockClient := client.(*MockLLMClient)
	for i := 0; i < b.N; i++ {
		mockClient.AddResponse(&core.LLMResponse{
			Content: "SELECT * FROM users",
			Usage: &core.TokenUsage{
				PromptTokens:     50,
				CompletionTokens: 20,
				TotalTokens:      70,
			},
		})
	}

	tool := NewSQLGeneratorTool(client, nil, logger)
	ctx := context.Background()
	input := "Query: 查找用户\nSchema: CREATE TABLE users (id INT, name VARCHAR(100))"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := tool.Execute(ctx, input)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkChainExecution 链执行基准测试
func BenchmarkChainExecution(b *testing.B) {
	if os.Getenv("LLM_INTEGRATION_TEST") != "true" {
		b.Skip("跳过 LLM 集成基准测试，设置 LLM_INTEGRATION_TEST=true 启用")
	}

	logger, _ := zap.NewDevelopment()
	config := &Config{
		Provider:    "mock",
		Model:       "test-model",
		Temperature: 0.1,
	}
	manager := NewManager(config, logger)

	// 创建链
	chainConfig := &core.ChainConfig{
		Type: "sql_generation",
		LLMConfig: &core.LLMConfig{
			Provider: "mock",
			Model:    "test-model",
		},
		Tools: []string{"sql_generator"},
	}

	ctx := context.Background()
	chain, err := manager.CreateChain(ctx, chainConfig)
	if err != nil {
		b.Fatal(err)
	}

	input := &core.ChainInput{
		Query: "查找用户",
		Context: map[string]any{
			"schema": "CREATE TABLE users (id INT, name VARCHAR(100))",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := manager.ExecuteChain(ctx, chain, input)
		if err != nil {
			b.Fatal(err)
		}
	}
}
