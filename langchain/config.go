// 本文件实现了大语言模���（LLM）相关的配置结构体和默认配置方法。
// 主要功能：
// 1. 定义 LLM 客户端、连接池、限流、Token 使用统计等配置结构体，支持 YAML 配置和类型安全访问。
// 2. 提供默认配置生成方法，便于快速初始化和测试。
// 3. 提供配置校验方法，保证各项参数合法性，防止错误配置导致系统异常。
// 4. 所有字段和方法均有详细中文注释，便于理解和维护。

package langchain

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
	"go.uber.org/zap"

	"pumppill/rag/core"
)

// LLMClientConfig LLM 客户端配置，定义与大语言模型交互的参数
// 支���多种模型、API 密钥、采样温度、最大 token 数、超时、重试等
// 可通过 YAML 配置文件加载
// Provider: 服务商（如 openai、anthropic、local、mock）
// Model: 模型名称（如 gpt-3.5-turbo、gpt-4）
// APIKey: API 密钥
// BaseURL: 自定义 API 地址
// Temperature: 采样温度，影响生成内容的多样性
// MaxTokens: 最大生成 token 数
// Timeout: 请求超时时间
// RetryCount: 请求失败时的重试次数
// Options: 其他自定义选项
type LLMClientConfig struct {
	Provider    string            `yaml:"provider"`    // openai, anthropic, local, etc.
	Model       string            `yaml:"model"`       // gpt-3.5-turbo, gpt-4, etc.
	APIKey      string            `yaml:"api_key"`     // API 密钥
	BaseURL     string            `yaml:"base_url"`    // 自定义 API 基础 URL
	Temperature float32           `yaml:"temperature"` // 采样温度 0.0-2.0
	MaxTokens   int               `yaml:"max_tokens"`  // 最大 token 数
	Timeout     time.Duration     `yaml:"timeout"`     // 请求超时时间
	RetryCount  int               `yaml:"retry_count"` // 重试次数
	Options     map[string]string `yaml:"options"`     // 其他选项
}

// ConnectionPoolConfig 连接池配置，定义连接池参数
// MaxConnections: 最大连接数
// MaxIdleTime: 最大空闲时间
// ConnectionTimeout: 获取连接超时时间
// HealthCheckPeriod: 健康检查周期
type ConnectionPoolConfig struct {
	MaxConnections    int           `yaml:"max_connections"`     // 最大连接数
	MaxIdleTime       time.Duration `yaml:"max_idle_time"`       // 最大空闲时间
	ConnectionTimeout time.Duration `yaml:"connection_timeout"`  // 连接超时时间
	HealthCheckPeriod time.Duration `yaml:"health_check_period"` // 健康检查周期
}

// RateLimitConfig 限流配置，定义请求和 token 的限流参数
// RequestsPerMinute: 每分钟最大请求数
// TokensPerMinute: 每分钟最大 token 数
// BurstSize: 突发请求大小
// WindowSize: 时间窗口大小
type RateLimitConfig struct {
	RequestsPerMinute int           `yaml:"requests_per_minute"` // 每分钟请求数限制
	TokensPerMinute   int           `yaml:"tokens_per_minute"`   // 每分钟 token 数限制
	BurstSize         int           `yaml:"burst_size"`          // 突发请求大小
	WindowSize        time.Duration `yaml:"window_size"`         // 时间窗口大小
}

// TokenUsageConfig Token 使用统计配置，定义 token 跟踪和报告参数
// EnableTracking: 是否启用 token 跟踪
// ReportInterval: 统计报告间隔
// MaxHistory: 最大历史记录数
type TokenUsageConfig struct {
	EnableTracking bool          `yaml:"enable_tracking"` // 是否启用 token 使用跟踪
	ReportInterval time.Duration `yaml:"report_interval"` // 统计报告间隔
	MaxHistory     int           `yaml:"max_history"`     // 最大历史记录数
}

// LLMManagerConfig LLM 管理器配置，聚合所有 LLM 相关配置
// Client: 客户端配置
// Pool: 连接池配置
// RateLimit: 限流配置
// TokenUsage: Token 使用统计配置
// EnableMetrics: 是否启用指标收集
// LogLevel: 日志级别
type LLMManagerConfig struct {
	Client        *LLMClientConfig      `yaml:"client"`
	Pool          *ConnectionPoolConfig `yaml:"pool"`
	RateLimit     *RateLimitConfig      `yaml:"rate_limit"`
	TokenUsage    *TokenUsageConfig     `yaml:"token_usage"`
	EnableMetrics bool                  `yaml:"enable_metrics"` // 是否启用指标收集
	LogLevel      string                `yaml:"log_level"`      // 日志级别
}

// DefaultLLMManagerConfig 返回默认的 LLM 管理器配置，便于快速初始化和测试
func DefaultLLMManagerConfig() *LLMManagerConfig {
	return &LLMManagerConfig{
		Client: &LLMClientConfig{
			Provider:    "openai",
			Model:       "gpt-3.5-turbo",
			Temperature: 0.1,
			MaxTokens:   2048,
			Timeout:     30 * time.Second,
			RetryCount:  3,
			Options:     make(map[string]string),
		},
		Pool: &ConnectionPoolConfig{
			MaxConnections:    10,
			MaxIdleTime:       5 * time.Minute,
			ConnectionTimeout: 10 * time.Second,
			HealthCheckPeriod: 1 * time.Minute,
		},
		RateLimit: &RateLimitConfig{
			RequestsPerMinute: 60,
			TokensPerMinute:   100000,
			BurstSize:         10,
			WindowSize:        1 * time.Minute,
		},
		TokenUsage: &TokenUsageConfig{
			EnableTracking: true,
			ReportInterval: 5 * time.Minute,
			MaxHistory:     1000,
		},
		EnableMetrics: true,
		LogLevel:      "info",
	}
}

// ValidateConfig 验证配置，检查各项参数是否合法，防止错误配置导致系统异常
func (c *LLMManagerConfig) ValidateConfig() error {
	if c.Client == nil {
		return fmt.Errorf("client config is required")
	}

	if c.Client.Provider == "" {
		return fmt.Errorf("provider is required")
	}

	if c.Client.Model == "" {
		return fmt.Errorf("model is required")
	}

	if c.Client.APIKey == "" && c.Client.Provider != "local" && c.Client.Provider != "mock" {
		return fmt.Errorf("api_key is required for provider %s", c.Client.Provider)
	}

	if c.Client.Temperature < 0 || c.Client.Temperature > 2 {
		return fmt.Errorf("temperature must be between 0 and 2")
	}

	if c.Client.MaxTokens <= 0 {
		return fmt.Errorf("max_tokens must be positive")
	}

	if c.Pool != nil {
		if c.Pool.MaxConnections <= 0 {
			return fmt.Errorf("max_connections must be positive")
		}
	}

	if c.RateLimit != nil {
		if c.RateLimit.RequestsPerMinute <= 0 {
			return fmt.Errorf("requests_per_minute must be positive")
		}
		if c.RateLimit.TokensPerMinute <= 0 {
			return fmt.Errorf("tokens_per_minute must be positive")
		}
	}

	return nil
}

// LLMConnection LLM 连接结构
type LLMConnection struct {
	ID           string
	LLM          llms.Model
	CreatedAt    time.Time
	LastUsedAt   time.Time
	IsHealthy    bool
	RequestCount int64
	TokenCount   int64
	mutex        sync.RWMutex
}

// UpdateUsage 更新使用统计
func (c *LLMConnection) UpdateUsage(tokenUsage *core.TokenUsage) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.LastUsedAt = time.Now()
	c.RequestCount++
	if tokenUsage != nil {
		c.TokenCount += int64(tokenUsage.TotalTokens)
	}
}

// GetStats 获取连接统计信息
func (c *LLMConnection) GetStats() map[string]interface{} {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return map[string]interface{}{
		"id":            c.ID,
		"created_at":    c.CreatedAt,
		"last_used_at":  c.LastUsedAt,
		"is_healthy":    c.IsHealthy,
		"request_count": c.RequestCount,
		"token_count":   c.TokenCount,
		"idle_time":     time.Since(c.LastUsedAt),
	}
}

// IsIdle 检查连接是否空闲
func (c *LLMConnection) IsIdle(maxIdleTime time.Duration) bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return time.Since(c.LastUsedAt) > maxIdleTime
}

// HealthCheck 健康检查
func (c *LLMConnection) HealthCheck(ctx context.Context) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// 发送简单的测试请求来检查连接健康状态
	_, err := c.LLM.GenerateContent(ctx, []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeHuman, "ping"),
	}, llms.WithMaxTokens(1))

	c.IsHealthy = (err == nil)
	return err
}

// createLLMClient 创建 LLM 客户端
func createLLMClient(config *LLMClientConfig) (llms.Model, error) {
	switch config.Provider {
	case "openai":
		opts := []openai.Option{
			openai.WithModel(config.Model),
			openai.WithToken(config.APIKey),
		}

		if config.BaseURL != "" {
			opts = append(opts, openai.WithBaseURL(config.BaseURL))
		}

		return openai.New(opts...)

	case "mock", "local":
		// 对于测试和本���开发，返回一个简单的模拟实现
		return &mockLLM{}, nil

	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s", config.Provider)
	}
}

// ConfigManager 配置管理器
type ConfigManager struct {
	config *LLMManagerConfig
	logger *zap.Logger
	mutex  sync.RWMutex
}

// NewConfigManager 创建配置管理器
func NewConfigManager(config *LLMManagerConfig, logger *zap.Logger) (*ConfigManager, error) {
	if config == nil {
		config = DefaultLLMManagerConfig()
	}

	if err := config.ValidateConfig(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &ConfigManager{
		config: config,
		logger: logger,
	}, nil
}

// GetConfig 获取配置
func (cm *ConfigManager) GetConfig() *LLMManagerConfig {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	// 返回配置的深拷贝以避免并发修改
	configCopy := *cm.config
	return &configCopy
}

// UpdateConfig 更新配置
func (cm *ConfigManager) UpdateConfig(newConfig *LLMManagerConfig) error {
	if err := newConfig.ValidateConfig(); err != nil {
		return fmt.Errorf("invalid new config: %w", err)
	}

	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	cm.config = newConfig
	cm.logger.Info("LLM configuration updated")

	return nil
}

// GetClientConfig 获取客户端配置
func (cm *ConfigManager) GetClientConfig() *LLMClientConfig {
	config := cm.GetConfig()
	return config.Client
}

// GetPoolConfig 获取连接池配置
func (cm *ConfigManager) GetPoolConfig() *ConnectionPoolConfig {
	config := cm.GetConfig()
	return config.Pool
}

// GetRateLimitConfig 获取限流配置
func (cm *ConfigManager) GetRateLimitConfig() *RateLimitConfig {
	config := cm.GetConfig()
	return config.RateLimit
}

// GetTokenUsageConfig 获取 Token 使用配置
func (cm *ConfigManager) GetTokenUsageConfig() *TokenUsageConfig {
	config := cm.GetConfig()
	return config.TokenUsage
}

// mockLLM 模拟 LLM 实现（用于测试）
type mockLLM struct{}

// GenerateContent 生成内容（模拟实现）
func (m *mockLLM) GenerateContent(ctx context.Context, messages []llms.MessageContent, options ...llms.CallOption) (*llms.ContentResponse, error) {
	return &llms.ContentResponse{
		Choices: []*llms.ContentChoice{
			{
				Content: "Mock LLM response",
			},
		},
	}, nil
}

// Call 调用（模拟实现）
func (m *mockLLM) Call(ctx context.Context, prompt string, options ...llms.CallOption) (string, error) {
	return "Mock LLM response", nil
}
