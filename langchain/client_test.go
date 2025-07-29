package langchain

import (
	"context"
	"testing"
	"time"

	"github.com/tmc/langchaingo/llms"
	"go.uber.org/zap"

	"pumppill/rag/core"
)

func TestLLMClientBuilder(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	// 创建测试配置
	config := DefaultLLMManagerConfig()
	config.Client.Provider = "mock" // 使用 mock 提供商避免需要 API key

	// 测试构建器
	builder := NewLLMClientBuilder()
	client, err := builder.
		WithConfig(config).
		WithLogger(logger).
		Build()

	if err != nil {
		t.Fatalf("Failed to build LLM client: %v", err)
	}

	if client == nil {
		t.Fatal("Client should not be nil")
	}

	// 测试获取统计信息
	stats := client.GetStats()
	if stats == nil {
		t.Fatal("Stats should not be nil")
	}

	// 清理
	client.Close()
}

func TestMockLLMClient(t *testing.T) {
	mock := NewMockLLMClient()

	// 添加模拟响应
	response := &llms.ContentResponse{
		Choices: []*llms.ContentChoice{
			{
				Content: "Test response",
			},
		},
	}
	mock.AddResponse(response)

	// 测试生成内容
	ctx := context.Background()
	messages := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeHuman, "test message"),
	}

	result, err := mock.GenerateContent(ctx, messages)
	if err != nil {
		t.Fatalf("GenerateContent failed: %v", err)
	}

	if result == nil {
		t.Fatal("Result should not be nil")
	}

	if len(result.Choices) == 0 {
		t.Fatal("Result should have choices")
	}

	if result.Choices[0].Content != "Test response" {
		t.Errorf("Expected 'Test response', got '%s'", result.Choices[0].Content)
	}

	// 测试错误情况
	mock.SetError(true, "test error")
	_, err = mock.GenerateContent(ctx, messages)
	if err == nil {
		t.Fatal("Expected error but got none")
	}

	if err.Error() != "test error" {
		t.Errorf("Expected 'test error', got '%s'", err.Error())
	}
}

func TestConfigManager(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	// 测试默认配置
	config := DefaultLLMManagerConfig()
	if config == nil {
		t.Fatal("Default config should not be nil")
	}

	// 修改配置以通过验证
	config.Client.Provider = "mock"

	// 测试配置验证
	err := config.ValidateConfig()
	if err != nil {
		t.Fatalf("Default config should be valid: %v", err)
	}

	// 测试配置管理器
	manager, err := NewConfigManager(config, logger)
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}

	// 测试获取配置
	retrievedConfig := manager.GetConfig()
	if retrievedConfig == nil {
		t.Fatal("Retrieved config should not be nil")
	}

	if retrievedConfig.Client.Provider != config.Client.Provider {
		t.Errorf("Expected provider '%s', got '%s'", config.Client.Provider, retrievedConfig.Client.Provider)
	}

	// 测试更新配置
	newConfig := DefaultLLMManagerConfig()
	newConfig.Client.Provider = "mock" // 使用 mock 提供商
	newConfig.Client.Model = "gpt-4"

	err = manager.UpdateConfig(newConfig)
	if err != nil {
		t.Fatalf("Failed to update config: %v", err)
	}

	updatedConfig := manager.GetConfig()
	if updatedConfig.Client.Model != "gpt-4" {
		t.Errorf("Expected model 'gpt-4', got '%s'", updatedConfig.Client.Model)
	}
}

func TestTokenTracker(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	config := &TokenUsageConfig{
		EnableTracking: true,
		ReportInterval: 1 * time.Second,
		MaxHistory:     10,
	}

	tracker := NewTokenTracker(config, logger)

	// 测试记录使用情况
	usage1 := &core.TokenUsage{
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
	}

	tracker.RecordUsage(usage1)

	// 测试获取使用情况
	totalUsage := tracker.GetUsage()
	if totalUsage.TotalTokens != 150 {
		t.Errorf("Expected total tokens 150, got %d", totalUsage.TotalTokens)
	}

	// 测试历史记录
	history := tracker.GetHistory()
	if len(history) != 1 {
		t.Errorf("Expected 1 history record, got %d", len(history))
	}

	// 记录更多使用情况
	usage2 := &core.TokenUsage{
		PromptTokens:     200,
		CompletionTokens: 100,
		TotalTokens:      300,
	}

	tracker.RecordUsage(usage2)

	// 验证累计统计
	totalUsage = tracker.GetUsage()
	if totalUsage.TotalTokens != 450 {
		t.Errorf("Expected total tokens 450, got %d", totalUsage.TotalTokens)
	}

	// 测试重置
	tracker.Reset()
	totalUsage = tracker.GetUsage()
	if totalUsage.TotalTokens != 0 {
		t.Errorf("Expected total tokens 0 after reset, got %d", totalUsage.TotalTokens)
	}
}

func TestRateLimiter(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	config := &RateLimitConfig{
		RequestsPerMinute: 60,
		TokensPerMinute:   10000, // 增加 token 限制
		BurstSize:         10,
		WindowSize:        1 * time.Minute,
	}

	limiter, err := NewRateLimiter(config, logger)
	if err != nil {
		t.Fatalf("Failed to create rate limiter: %v", err)
	}

	ctx := context.Background()

	// 测试允许请求
	err = limiter.Allow(ctx, 100)
	if err != nil {
		t.Fatalf("First request should be allowed: %v", err)
	}

	// 测试统计信息
	stats := limiter.GetStats()
	if stats == nil {
		t.Fatal("Stats should not be nil")
	}

	totalRequests, ok := stats["total_requests"].(int64)
	if !ok || totalRequests != 1 {
		t.Errorf("Expected 1 total request, got %v", stats["total_requests"])
	}
}

func TestMockRateLimiter(t *testing.T) {
	mock := NewMockRateLimiter()
	ctx := context.Background()

	// 测试允许请求
	err := mock.Allow(ctx, 100)
	if err != nil {
		t.Fatalf("Mock limiter should allow by default: %v", err)
	}

	// 测试阻塞请求
	mock.SetBlocking(true, "rate limit exceeded")
	err = mock.Allow(ctx, 100)
	if err == nil {
		t.Fatal("Expected error but got none")
	}

	if err.Error() != "rate limit exceeded" {
		t.Errorf("Expected 'rate limit exceeded', got '%s'", err.Error())
	}

	// 测试统计信息
	stats := mock.GetStats()
	if stats["allow_count"].(int64) != 1 {
		t.Errorf("Expected allow_count 1, got %v", stats["allow_count"])
	}

	if stats["block_count"].(int64) != 1 {
		t.Errorf("Expected block_count 1, got %v", stats["block_count"])
	}
}

func TestConnectionPoolStats(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	config := DefaultLLMManagerConfig()
	config.Client.Provider = "mock" // 使用 mock 提供商
	config.Pool.MaxConnections = 5

	configManager, err := NewConfigManager(config, logger)
	if err != nil {
		t.Fatalf("Failed to create config manager: %v", err)
	}

	pool, err := NewConnectionPool(configManager, logger)
	if err != nil {
		t.Fatalf("Failed to create connection pool: %v", err)
	}
	defer pool.Close()

	// 测试统计信息
	stats := pool.GetStats()
	if stats == nil {
		t.Fatal("Stats should not be nil")
	}

	// 验证统计信息包含预期字段
	expectedFields := []string{
		"active_connections",
		"total_connections",
		"healthy_connections",
		"created_count",
		"borrowed_count",
		"returned_count",
		"error_count",
		"pool_utilization",
	}

	for _, field := range expectedFields {
		if _, exists := stats[field]; !exists {
			t.Errorf("Expected field '%s' in stats", field)
		}
	}
}

// 基准测试
func BenchmarkTokenTracker(b *testing.B) {
	logger, _ := zap.NewDevelopment()
	config := &TokenUsageConfig{
		EnableTracking: true,
		MaxHistory:     1000,
	}

	tracker := NewTokenTracker(config, logger)

	usage := &core.TokenUsage{
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		tracker.RecordUsage(usage)
	}
}

func BenchmarkRateLimiter(b *testing.B) {
	logger, _ := zap.NewDevelopment()
	config := &RateLimitConfig{
		RequestsPerMinute: 10000,
		TokensPerMinute:   100000,
		BurstSize:         1000,
		WindowSize:        1 * time.Minute,
	}

	limiter, _ := NewRateLimiter(config, logger)
	ctx := context.Background()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		limiter.Allow(ctx, 100)
	}
}
