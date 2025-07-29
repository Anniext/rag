// 本文件实现了大语言模型（LLM）相关的核心接口，包括 LLM 客户端、连接池、限流器等。
// 主要功能：
// 1. LLMClient：定义与大语言模型交互的客户端接口，支持内容生成、用量统计、状态查询和资源释放。
// 2. ConnectionPool：定义连接池接口，支持连接的获取、归还、关闭和统计。
// 3. RateLimiter：定义限流器接口，支持令牌限流、状态查询。
// 所有接口均有详细方法注释，便于理解和扩展。

package langchain

import (
	"context"
	"fmt"
	"github.com/Anniext/rag/core"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tmc/langchaingo/llms"
	"go.uber.org/zap"
)

// LLMClient LLM 客户端接口，定义了大语言模型客户端的基本操作方法
type LLMClient interface {
	// GenerateContent 生成内容，根据输入的消息和可选参数调用 LLM，返回生成的内容响应和错误信息
	GenerateContent(ctx context.Context, messages []llms.MessageContent, options ...llms.CallOption) (*llms.ContentResponse, error)
	// GetUsage 获取当前 Token 使用统计信息
	GetUsage() *core.TokenUsage
	// GetStats 获取客户���的统计信息（如请求数、成功率等）
	GetStats() map[string]interface{}
	// Close 关闭客户端，释放相关资源
	Close() error
}

// ConnectionPool 连接池接口
type ConnectionPool interface {
	// GetConnection 获取可用连接
	GetConnection(ctx context.Context) (*LLMConnection, error)
	// ReturnConnection 归还连接到连接池
	ReturnConnection(conn *LLMConnection)
	// Close 关闭连接池，释放相关资源
	Close() error
	// GetStats 获取连接池的统计信息
	GetStats() map[string]interface{}
}

// RateLimiter 限流器接口
type RateLimiter interface {
	// Allow 检查并允许一定数量的令牌通过，支持限流
	Allow(ctx context.Context, tokens int) error
	// GetStats 获取限流器的统计信息
	GetStats() map[string]interface{}
}

// TokenTracker Token 使用跟踪器接口
type TokenTracker interface {
	// RecordUsage 记录一次 Token 使用情况
	RecordUsage(usage *core.TokenUsage)
	// GetUsage 获取当前 Token 使用统计信息
	GetUsage() *core.TokenUsage
	// GetHistory 获取 Token 使用历史记录
	GetHistory() []*core.TokenUsage
	// Reset 重置 Token 使用统计信息
	Reset()
}

// llmClientImpl LLM 客户端实现
type llmClientImpl struct {
	config       *ConfigManager
	pool         ConnectionPool
	rateLimiter  RateLimiter
	tokenTracker TokenTracker
	logger       *zap.Logger
	metrics      core.MetricsCollector

	// 统计信息
	requestCount int64
	successCount int64
	errorCount   int64
	totalLatency int64

	mutex sync.RWMutex
}

// NewLLMClient 创建 LLM 客户端
func NewLLMClient(config *LLMManagerConfig, logger *zap.Logger, metrics core.MetricsCollector) (LLMClient, error) {
	configManager, err := NewConfigManager(config, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create config manager: %w", err)
	}

	// 创建连接池
	pool, err := NewConnectionPool(configManager, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// 创建限流器
	rateLimiter, err := NewRateLimiter(configManager.GetRateLimitConfig(), logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create rate limiter: %w", err)
	}

	// 创建 Token 跟踪器
	tokenTracker := NewTokenTracker(configManager.GetTokenUsageConfig(), logger)

	client := &llmClientImpl{
		config:       configManager,
		pool:         pool,
		rateLimiter:  rateLimiter,
		tokenTracker: tokenTracker,
		logger:       logger,
		metrics:      metrics,
	}

	return client, nil
}

// GenerateContent 生成内容
func (c *llmClientImpl) GenerateContent(ctx context.Context, messages []llms.MessageContent, options ...llms.CallOption) (*llms.ContentResponse, error) {
	startTime := time.Now()
	atomic.AddInt64(&c.requestCount, 1)

	// 记录指标
	if c.metrics != nil {
		c.metrics.IncrementCounter("llm_requests_total", map[string]string{
			"provider": c.config.GetClientConfig().Provider,
			"model":    c.config.GetClientConfig().Model,
		})
	}

	// 估算 token 数量用于限流
	estimatedTokens := c.estimateTokens(messages)

	// 检查限流
	if err := c.rateLimiter.Allow(ctx, estimatedTokens); err != nil {
		atomic.AddInt64(&c.errorCount, 1)
		c.logger.Warn("Rate limit exceeded", zap.Error(err))

		if c.metrics != nil {
			c.metrics.IncrementCounter("llm_rate_limit_errors_total", map[string]string{
				"provider": c.config.GetClientConfig().Provider,
			})
		}

		return nil, fmt.Errorf("rate limit exceeded: %w", err)
	}

	// 获取连接
	conn, err := c.pool.GetConnection(ctx)
	if err != nil {
		atomic.AddInt64(&c.errorCount, 1)
		c.logger.Error("Failed to get connection", zap.Error(err))
		return nil, fmt.Errorf("failed to get connection: %w", err)
	}
	defer c.pool.ReturnConnection(conn)

	// 执行请求
	response, err := conn.LLM.GenerateContent(ctx, messages, options...)

	// 计算延迟
	latency := time.Since(startTime)
	atomic.AddInt64(&c.totalLatency, latency.Nanoseconds())

	if err != nil {
		atomic.AddInt64(&c.errorCount, 1)
		c.logger.Error("LLM request failed",
			zap.Error(err),
			zap.Duration("latency", latency),
		)

		if c.metrics != nil {
			c.metrics.IncrementCounter("llm_errors_total", map[string]string{
				"provider": c.config.GetClientConfig().Provider,
				"error":    "generation_failed",
			})
		}

		return nil, fmt.Errorf("LLM generation failed: %w", err)
	}

	atomic.AddInt64(&c.successCount, 1)

	// 记录 Token 使用情况
	if response != nil {
		tokenUsage := c.extractTokenUsage(response)
		if tokenUsage != nil {
			c.tokenTracker.RecordUsage(tokenUsage)
			conn.UpdateUsage(tokenUsage)
		}
	}

	// 记录成功指标
	if c.metrics != nil {
		c.metrics.IncrementCounter("llm_requests_success_total", map[string]string{
			"provider": c.config.GetClientConfig().Provider,
		})
		c.metrics.RecordHistogram("llm_request_duration_seconds", latency.Seconds(), map[string]string{
			"provider": c.config.GetClientConfig().Provider,
		})
	}

	c.logger.Debug("LLM request completed",
		zap.Duration("latency", latency),
		zap.Int("estimated_tokens", estimatedTokens),
	)

	return response, nil
}

// GetUsage 获取 Token 使用统计
func (c *llmClientImpl) GetUsage() *core.TokenUsage {
	return c.tokenTracker.GetUsage()
}

// GetStats 获取客户端统计信息
func (c *llmClientImpl) GetStats() map[string]interface{} {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	requestCount := atomic.LoadInt64(&c.requestCount)
	successCount := atomic.LoadInt64(&c.successCount)
	errorCount := atomic.LoadInt64(&c.errorCount)
	totalLatency := atomic.LoadInt64(&c.totalLatency)

	var avgLatency float64
	if requestCount > 0 {
		avgLatency = float64(totalLatency) / float64(requestCount) / 1e9 // 转换为秒
	}

	var successRate float64
	if requestCount > 0 {
		successRate = float64(successCount) / float64(requestCount)
	}

	stats := map[string]interface{}{
		"request_count":    requestCount,
		"success_count":    successCount,
		"error_count":      errorCount,
		"success_rate":     successRate,
		"avg_latency":      avgLatency,
		"token_usage":      c.tokenTracker.GetUsage(),
		"pool_stats":       c.pool.GetStats(),
		"rate_limit_stats": c.rateLimiter.GetStats(),
	}

	return stats
}

// Close 关闭客户端
func (c *llmClientImpl) Close() error {
	c.logger.Info("Closing LLM client")

	if err := c.pool.Close(); err != nil {
		c.logger.Error("Failed to close connection pool", zap.Error(err))
		return err
	}

	return nil
}

// estimateTokens 估算消息的 token 数量
func (c *llmClientImpl) estimateTokens(messages []llms.MessageContent) int {
	totalChars := 0
	for _, msg := range messages {
		for _, part := range msg.Parts {
			if textPart, ok := part.(llms.TextContent); ok {
				totalChars += len(textPart.Text)
			}
		}
	}

	// 粗略估算：平均 4 个字符 = 1 个 token
	return totalChars / 4
}

// extractTokenUsage 从响应中提取 Token 使用信息
func (c *llmClientImpl) extractTokenUsage(response *llms.ContentResponse) *core.TokenUsage {
	if response == nil {
		return nil
	}

	// 尝试从响应中提取 token 使用信息
	// 注意：这里需要根据实际的 langchaingo 库 API 进行调整
	usage := &core.TokenUsage{
		PromptTokens:     0, // 需要从响应中获取
		CompletionTokens: 0, // 需要从响应中获取
		TotalTokens:      0, // 需要从响应中获取
	}

	// 如果无法从响应中获取准确的 token 数量，进行估算
	if len(response.Choices) > 0 {
		content := response.Choices[0].Content
		usage.CompletionTokens = len(content) / 4 // 粗略估算
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}

	return usage
}

// LLMClientBuilder 客户端构建器
type LLMClientBuilder struct {
	config  *LLMManagerConfig
	logger  *zap.Logger
	metrics core.MetricsCollector
}

// NewLLMClientBuilder 创建客户端构建器
func NewLLMClientBuilder() *LLMClientBuilder {
	return &LLMClientBuilder{
		config: DefaultLLMManagerConfig(),
	}
}

// WithConfig 设置配置
func (b *LLMClientBuilder) WithConfig(config *LLMManagerConfig) *LLMClientBuilder {
	b.config = config
	return b
}

// WithLogger 设置日志器
func (b *LLMClientBuilder) WithLogger(logger *zap.Logger) *LLMClientBuilder {
	b.logger = logger
	return b
}

// WithMetrics 设置指标收集器
func (b *LLMClientBuilder) WithMetrics(metrics core.MetricsCollector) *LLMClientBuilder {
	b.metrics = metrics
	return b
}

// Build 构建客户端
func (b *LLMClientBuilder) Build() (LLMClient, error) {
	if b.logger == nil {
		// 创建默认日志器
		logger, err := zap.NewProduction()
		if err != nil {
			return nil, fmt.Errorf("failed to create default logger: %w", err)
		}
		b.logger = logger
	}

	return NewLLMClient(b.config, b.logger, b.metrics)
}

// MockLLMClient 模拟 LLM 客户端（用于测试）
type MockLLMClient struct {
	responses    []*llms.ContentResponse
	responseIdx  int
	usage        *core.TokenUsage
	shouldError  bool
	errorMessage string
	mutex        sync.Mutex
}

// NewMockLLMClient 创建模拟客户端
func NewMockLLMClient() *MockLLMClient {
	return &MockLLMClient{
		responses: make([]*llms.ContentResponse, 0),
		usage: &core.TokenUsage{
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
		},
	}
}

// AddResponse 添加模拟响应
func (m *MockLLMClient) AddResponse(response *llms.ContentResponse) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.responses = append(m.responses, response)
}

// SetError 设置错误
func (m *MockLLMClient) SetError(shouldError bool, message string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.shouldError = shouldError
	m.errorMessage = message
}

// GenerateContent 生成内容（模拟实现）
func (m *MockLLMClient) GenerateContent(ctx context.Context, messages []llms.MessageContent, options ...llms.CallOption) (*llms.ContentResponse, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.shouldError {
		return nil, fmt.Errorf("%s", m.errorMessage)
	}

	if len(m.responses) == 0 {
		return &llms.ContentResponse{
			Choices: []*llms.ContentChoice{
				{
					Content: "Mock response",
				},
			},
		}, nil
	}

	response := m.responses[m.responseIdx%len(m.responses)]
	m.responseIdx++

	return response, nil
}

// GetUsage 获取使用统计
func (m *MockLLMClient) GetUsage() *core.TokenUsage {
	return m.usage
}

// GetStats 获取统计信息
func (m *MockLLMClient) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"type":          "mock",
		"request_count": m.responseIdx,
		"usage":         m.usage,
	}
}

// Close 关闭客户端
func (m *MockLLMClient) Close() error {
	return nil
}
