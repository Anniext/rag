package langchain

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/tmc/langchaingo/llms"
	"go.uber.org/zap"
)

func TestSQLGeneratorTool(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockClient := NewMockLLMClient()
	templateManager := NewTemplateManager(logger)
	templateRenderer := NewTemplateRenderer(templateManager, logger)

	// 设置模拟响应
	mockClient.AddResponse(&llms.ContentResponse{
		Choices: []*llms.ContentChoice{
			{
				Content: "SELECT * FROM users WHERE name = 'test'",
			},
		},
	})

	tool := NewSQLGeneratorTool(mockClient, templateRenderer, logger)

	// 测试工具基本信息
	if tool.Name() != "sql_generator" {
		t.Errorf("Expected name 'sql_generator', got '%s'", tool.Name())
	}

	if tool.Description() == "" {
		t.Error("Tool description should not be empty")
	}

	// 测试执行
	ctx := context.Background()
	input := "Query: 查找所有用户\nSchema: CREATE TABLE users (id INT, name VARCHAR(100))"

	result, err := tool.Execute(ctx, input)
	if err != nil {
		t.Fatalf("Tool execution failed: %v", err)
	}

	if result == "" {
		t.Fatal("Tool result should not be empty")
	}

	// 验证结果包含 SQL
	if !contains(result, "SELECT") {
		t.Error("Result should contain SQL statement")
	}
}

func TestQueryExplanationTool(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockClient := NewMockLLMClient()
	templateManager := NewTemplateManager(logger)
	templateRenderer := NewTemplateRenderer(templateManager, logger)

	// 设置模拟响应
	mockClient.AddResponse(&llms.ContentResponse{
		Choices: []*llms.ContentChoice{
			{
				Content: "这个查询从用户表中选择所有记录",
			},
		},
	})

	tool := NewQueryExplanationTool(mockClient, templateRenderer, logger)

	// 测试执行
	ctx := context.Background()
	input := "SQL: SELECT * FROM users\nResults: [{'id': 1, 'name': 'test'}]"

	result, err := tool.Execute(ctx, input)
	if err != nil {
		t.Fatalf("Tool execution failed: %v", err)
	}

	if result == "" {
		t.Fatal("Tool result should not be empty")
	}

	// 验证结果包含解释
	if !contains(result, "查询") {
		t.Error("Result should contain query explanation")
	}
}

func TestErrorAnalysisTool(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockClient := NewMockLLMClient()
	templateManager := NewTemplateManager(logger)
	templateRenderer := NewTemplateRenderer(templateManager, logger)

	// 设置模拟响应
	mockClient.AddResponse(&llms.ContentResponse{
		Choices: []*llms.ContentChoice{
			{
				Content: "错误原因：表名不存在。建议：检查表名拼写。",
			},
		},
	})

	tool := NewErrorAnalysisTool(mockClient, templateRenderer, logger)

	// 测试执行
	ctx := context.Background()
	input := "Query: 查找用户\nSQL: SELECT * FROM user\nError: Table 'user' doesn't exist"

	result, err := tool.Execute(ctx, input)
	if err != nil {
		t.Fatalf("Tool execution failed: %v", err)
	}

	if result == "" {
		t.Fatal("Tool result should not be empty")
	}

	// 验证结果包含错误分析
	if !contains(result, "错误") {
		t.Error("Result should contain error analysis")
	}
}

func TestSuggestionTool(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockClient := NewMockLLMClient()
	templateManager := NewTemplateManager(logger)
	templateRenderer := NewTemplateRenderer(templateManager, logger)

	// 设置模拟响应
	mockClient.AddResponse(&llms.ContentResponse{
		Choices: []*llms.ContentChoice{
			{
				Content: "1. SELECT * FROM users\n2. SELECT name FROM users\n3. SELECT id, name FROM users",
			},
		},
	})

	tool := NewSuggestionTool(mockClient, templateRenderer, logger)

	// 测试执行
	ctx := context.Background()
	input := "PartialQuery: 查找用户\nSchema: CREATE TABLE users (id INT, name VARCHAR(100))"

	result, err := tool.Execute(ctx, input)
	if err != nil {
		t.Fatalf("Tool execution failed: %v", err)
	}

	if result == "" {
		t.Fatal("Tool result should not be empty")
	}

	// 验证结果包含建议
	if !contains(result, "SELECT") {
		t.Error("Result should contain SQL suggestions")
	}
}

func TestOptimizationTool(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockClient := NewMockLLMClient()
	templateManager := NewTemplateManager(logger)
	templateRenderer := NewTemplateRenderer(templateManager, logger)

	// 设置模拟响应
	mockClient.AddResponse(&llms.ContentResponse{
		Choices: []*llms.ContentChoice{
			{
				Content: "建议在 name 列上添加索引以提高查询性能",
			},
		},
	})

	tool := NewOptimizationTool(mockClient, templateRenderer, logger)

	// 测试执行
	ctx := context.Background()
	input := "SQL: SELECT * FROM users WHERE name = 'test'\nExecutionPlan: Full table scan\nMetrics: 1000ms"

	result, err := tool.Execute(ctx, input)
	if err != nil {
		t.Fatalf("Tool execution failed: %v", err)
	}

	if result == "" {
		t.Fatal("Tool result should not be empty")
	}

	// 验证结果包含优化建议
	if !contains(result, "索引") || !contains(result, "性能") {
		t.Error("Result should contain optimization suggestions")
	}
}

func TestMockTool(t *testing.T) {
	tool := NewMockTool("test_tool", "Test tool description")

	// 测试基本信息
	if tool.Name() != "test_tool" {
		t.Errorf("Expected name 'test_tool', got '%s'", tool.Name())
	}

	if tool.Description() != "Test tool description" {
		t.Errorf("Expected description 'Test tool description', got '%s'", tool.Description())
	}

	// 测试正常执行
	ctx := context.Background()
	result, err := tool.Execute(ctx, "test input")
	if err != nil {
		t.Fatalf("Tool execution failed: %v", err)
	}

	if !contains(result, "Mock response from test_tool") {
		t.Error("Result should contain mock response")
	}

	if !contains(result, "test input") {
		t.Error("Result should contain input")
	}

	if tool.GetCallCount() != 1 {
		t.Errorf("Expected call count 1, got %d", tool.GetCallCount())
	}

	// 测试错误情况
	tool.SetError(true, "test error")
	_, err = tool.Execute(ctx, "test input")
	if err == nil {
		t.Fatal("Expected error but got none")
	}

	if err.Error() != "test error" {
		t.Errorf("Expected error 'test error', got '%s'", err.Error())
	}

	// 测试重置
	tool.Reset()
	if tool.GetCallCount() != 0 {
		t.Errorf("Expected call count 0 after reset, got %d", tool.GetCallCount())
	}

	// 测试自定义响应
	tool.SetResponse("custom response")
	result, err = tool.Execute(ctx, "test")
	if err != nil {
		t.Fatalf("Tool execution failed: %v", err)
	}

	if !contains(result, "custom response") {
		t.Error("Result should contain custom response")
	}
}

func TestToolRegistry(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	registry := NewToolRegistry(logger)

	// 测试注册工具
	tool1 := NewMockTool("tool1", "Tool 1")
	tool2 := NewMockTool("tool2", "Tool 2")

	err := registry.RegisterTool(tool1)
	if err != nil {
		t.Fatalf("Failed to register tool1: %v", err)
	}

	err = registry.RegisterTool(tool2)
	if err != nil {
		t.Fatalf("Failed to register tool2: %v", err)
	}

	// 测试重复注册
	err = registry.RegisterTool(tool1)
	if err == nil {
		t.Fatal("Expected error for duplicate tool registration")
	}

	// 测试获取工具
	retrievedTool, err := registry.GetTool("tool1")
	if err != nil {
		t.Fatalf("Failed to get tool1: %v", err)
	}

	if retrievedTool.Name() != "tool1" {
		t.Errorf("Expected tool name 'tool1', got '%s'", retrievedTool.Name())
	}

	// 测试获取不存在的工具
	_, err = registry.GetTool("nonexistent")
	if err == nil {
		t.Fatal("Expected error for nonexistent tool")
	}

	// 测试列出工具
	tools := registry.ListTools()
	if len(tools) != 2 {
		t.Errorf("Expected 2 tools, got %d", len(tools))
	}

	// 测试工具数量
	count := registry.GetToolCount()
	if count != 2 {
		t.Errorf("Expected tool count 2, got %d", count)
	}

	// 测试注销工具
	err = registry.UnregisterTool("tool1")
	if err != nil {
		t.Fatalf("Failed to unregister tool1: %v", err)
	}

	count = registry.GetToolCount()
	if count != 1 {
		t.Errorf("Expected tool count 1 after unregistration, got %d", count)
	}

	// 测试注销不存在的工具
	err = registry.UnregisterTool("nonexistent")
	if err == nil {
		t.Fatal("Expected error for unregistering nonexistent tool")
	}
}

func TestToolExecutor(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	registry := NewToolRegistry(logger)
	executor := NewToolExecutor(registry, logger)

	// 注册测试工具
	tool := NewMockTool("test_tool", "Test tool")
	tool.SetResponse("test response")

	err := registry.RegisterTool(tool)
	if err != nil {
		t.Fatalf("Failed to register tool: %v", err)
	}

	// 测试执行工具
	ctx := context.Background()
	result, err := executor.ExecuteTool(ctx, "test_tool", "test input")
	if err != nil {
		t.Fatalf("Tool execution failed: %v", err)
	}

	if !contains(result, "test response") {
		t.Error("Result should contain test response")
	}

	// 测试执行不存在的工具
	_, err = executor.ExecuteTool(ctx, "nonexistent", "input")
	if err == nil {
		t.Fatal("Expected error for nonexistent tool")
	}

	// 测试带超时的执行
	result, err = executor.ExecuteToolWithTimeout(ctx, "test_tool", "test input", 5*time.Second)
	if err != nil {
		t.Fatalf("Tool execution with timeout failed: %v", err)
	}

	if !contains(result, "test response") {
		t.Error("Result should contain test response")
	}

	// 测试批量执行
	requests := []ToolRequest{
		{ToolName: "test_tool", Input: "input1"},
		{ToolName: "test_tool", Input: "input2"},
	}

	results, err := executor.BatchExecuteTools(ctx, requests)
	if err != nil {
		t.Fatalf("Batch execution failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}

	for i, result := range results {
		if result.Error != nil {
			t.Errorf("Result %d should not have error: %v", i, result.Error)
		}

		if !contains(result.Output, "test response") {
			t.Errorf("Result %d should contain test response", i)
		}
	}
}

func TestToolInputParsing(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockClient := NewMockLLMClient()
	templateManager := NewTemplateManager(logger)
	templateRenderer := NewTemplateRenderer(templateManager, logger)

	// 测试 SQL 生成工具的输入解析
	sqlTool := NewSQLGeneratorTool(mockClient, templateRenderer, logger).(*sqlGeneratorTool)

	input := "Query: 查找用户\nSchema: CREATE TABLE users (id INT, name VARCHAR(100))"
	query, schema, err := sqlTool.parseInput(input)
	if err != nil {
		t.Fatalf("Failed to parse input: %v", err)
	}

	if query != "查找用户" {
		t.Errorf("Expected query '查找用户', got '%s'", query)
	}

	if schema == nil {
		t.Error("Schema should not be nil")
	}

	// 测试查询解释工具的输入解析
	explanationTool := NewQueryExplanationTool(mockClient, templateRenderer, logger).(*queryExplanationTool)

	input = "SQL: SELECT * FROM users\nResults: test results"
	sql, results, err := explanationTool.parseInput(input)
	if err != nil {
		t.Fatalf("Failed to parse input: %v", err)
	}

	if sql != "SELECT * FROM users" {
		t.Errorf("Expected SQL 'SELECT * FROM users', got '%s'", sql)
	}

	if results != "test results" {
		t.Errorf("Expected results 'test results', got '%v'", results)
	}

	// 测试错误分析工具的输入解析
	errorTool := NewErrorAnalysisTool(mockClient, templateRenderer, logger).(*errorAnalysisTool)

	input = "Query: 查找用户\nSQL: SELECT * FROM user\nError: Table doesn't exist"
	query, sql, errorMsg, err := errorTool.parseInput(input)
	if err != nil {
		t.Fatalf("Failed to parse input: %v", err)
	}

	if query != "查找用户" {
		t.Errorf("Expected query '查找用户', got '%s'", query)
	}

	if sql != "SELECT * FROM user" {
		t.Errorf("Expected SQL 'SELECT * FROM user', got '%s'", sql)
	}

	if errorMsg != "Table doesn't exist" {
		t.Errorf("Expected error 'Table doesn't exist', got '%s'", errorMsg)
	}
}

func TestSQLCleaning(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockClient := NewMockLLMClient()
	templateManager := NewTemplateManager(logger)
	templateRenderer := NewTemplateRenderer(templateManager, logger)

	sqlTool := NewSQLGeneratorTool(mockClient, templateRenderer, logger).(*sqlGeneratorTool)

	testCases := []struct {
		input    string
		expected string
	}{
		{
			input:    "```sql\nSELECT * FROM users;\n```",
			expected: "SELECT * FROM users;",
		},
		{
			input:    "SQL: SELECT * FROM users;",
			expected: "SELECT * FROM users;",
		},
		{
			input:    "  SELECT * FROM users;  ",
			expected: "SELECT * FROM users;",
		},
		{
			input:    "```\nSELECT * FROM users;\n```",
			expected: "SELECT * FROM users;",
		},
	}

	for i, tc := range testCases {
		result := sqlTool.cleanSQL(tc.input)
		if result != tc.expected {
			t.Errorf("Test case %d: expected '%s', got '%s'", i, tc.expected, result)
		}
	}
}

// 基准测试
func BenchmarkToolExecution(b *testing.B) {
	logger, _ := zap.NewDevelopment()
	registry := NewToolRegistry(logger)
	executor := NewToolExecutor(registry, logger)

	tool := NewMockTool("bench_tool", "Benchmark tool")
	registry.RegisterTool(tool)

	ctx := context.Background()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := executor.ExecuteTool(ctx, "bench_tool", "benchmark input")
		if err != nil {
			b.Fatalf("Tool execution failed: %v", err)
		}
	}
}

func BenchmarkToolRegistry(b *testing.B) {
	logger, _ := zap.NewDevelopment()
	registry := NewToolRegistry(logger)

	// 预注册一些工具
	for i := 0; i < 100; i++ {
		tool := NewMockTool(fmt.Sprintf("tool_%d", i), "Test tool")
		registry.RegisterTool(tool)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := registry.GetTool("tool_50")
		if err != nil {
			b.Fatalf("Failed to get tool: %v", err)
		}
	}
}
