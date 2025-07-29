// 本文件实现了大语言模型（LLM）限流器，支持请求速率和令牌桶双重限流。
// 主要功能：
// 1. 支持按请求数和 Token 数进行限流，防止超载和资源滥用。
// 2. 采用令牌桶算法，定时补充令牌，保证限流的平滑性和高效性。
// 3. 提供限流器统计信息，包括总请求数、被限流请求数、Token 消耗等。
// 4. 所有结构体字段和方法均有详细中文注释，便于理解和维护。

package langchain

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// rateLimiterImpl 限流器实现，支持请求速率和令牌桶双重限流
type rateLimiterImpl struct {
	config *RateLimitConfig // 限流配置
	logger *zap.Logger      // 日志记录器

	// 请求限流相关
	requestTokens     int       // 当前可用请求令牌数
	requestLastRefill time.Time // 上次请求令牌补充时间

	// Token 限流相关
	tokenBucket     int       // 当前可用 Token 数
	tokenLastRefill time.Time // 上次 Token 补充时间

	// 统计信息
	totalRequests   int64 // 总请求数
	blockedRequests int64 // 被限流的请求数
	totalTokens     int64 // 总消耗 Token 数
	blockedTokens   int64 // 被限流的 Token 数

	mutex sync.Mutex // 互斥锁，保证并发安全
}

// NewRateLimiter 创建限流器实例
func NewRateLimiter(config *RateLimitConfig, logger *zap.Logger) (RateLimiter, error) {
	if config == nil {
		return nil, fmt.Errorf("rate limit config is required")
	}

	limiter := &rateLimiterImpl{
		config:            config,
		logger:            logger,
		requestTokens:     config.BurstSize,            // 初始化为最大突发请求数
		requestLastRefill: time.Now(),                  // 初始化补充时间
		tokenBucket:       config.TokensPerMinute / 60, // 初始化 Token 桶大小
		tokenLastRefill:   time.Now(),                  // 初始化补充时间
	}

	logger.Info("Rate limiter created",
		zap.Int("requests_per_minute", config.RequestsPerMinute),
		zap.Int("tokens_per_minute", config.TokensPerMinute),
		zap.Int("burst_size", config.BurstSize),
	)

	return limiter, nil
}

// Allow 检查是否允许请求，支持请求速率和 Token 双重限流
func (r *rateLimiterImpl) Allow(ctx context.Context, tokens int) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	now := time.Now()

	// 补充请求令牌
	r.refillRequestTokens(now)

	// 补充 Token 令牌
	r.refillTokenBucket(now)

	// 检查请求限流
	if r.requestTokens <= 0 {
		r.blockedRequests++
		r.logger.Warn("Request rate limit exceeded",
			zap.Int("available_tokens", r.requestTokens),
			zap.Int("requests_per_minute", r.config.RequestsPerMinute),
		)
		return fmt.Errorf("request rate limit exceeded: %d requests per minute", r.config.RequestsPerMinute)
	}

	// 检查 Token 限流
	if r.tokenBucket < tokens {
		r.blockedTokens += int64(tokens)
		r.logger.Warn("Token rate limit exceeded",
			zap.Int("requested_tokens", tokens),
			zap.Int("available_tokens", r.tokenBucket),
			zap.Int("tokens_per_minute", r.config.TokensPerMinute),
		)
		return fmt.Errorf("token rate limit exceeded: requested %d tokens, available %d", tokens, r.tokenBucket)
	}

	// 消耗令牌
	r.requestTokens--
	r.tokenBucket -= tokens

	// 更新统计
	r.totalRequests++
	r.totalTokens += int64(tokens)

	return nil
}

// refillRequestTokens 补充请求令牌，根据时间间隔动态补充
func (r *rateLimiterImpl) refillRequestTokens(now time.Time) {
	elapsed := now.Sub(r.requestLastRefill)
	if elapsed < time.Second {
		return
	}

	// 计算应该补充的令牌数
	tokensToAdd := int(elapsed.Seconds()) * r.config.RequestsPerMinute / 60
	if tokensToAdd > 0 {
		r.requestTokens += tokensToAdd
		if r.requestTokens > r.config.BurstSize {
			r.requestTokens = r.config.BurstSize // 限制最大突发请求数
		}
		r.requestLastRefill = now
	}
}

// refillTokenBucket 补充 Token 令牌桶，根据时间间隔动态补充
func (r *rateLimiterImpl) refillTokenBucket(now time.Time) {
	elapsed := now.Sub(r.tokenLastRefill)
	if elapsed < time.Second {
		return
	}

	// 计算应该补充的 token 数
	tokensToAdd := int(elapsed.Seconds()) * r.config.TokensPerMinute / 60
	if tokensToAdd > 0 {
		r.tokenBucket += tokensToAdd

		// 限制最大 token 数（避免无限累积）
		maxTokens := r.config.TokensPerMinute / 6 // 10秒的 token 量
		if r.tokenBucket > maxTokens {
			r.tokenBucket = maxTokens
		}

		r.tokenLastRefill = now
	}
}

// GetStats 获取限流器统计信息，返回当前状态和配置
func (r *rateLimiterImpl) GetStats() map[string]interface{} {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	now := time.Now()
	r.refillRequestTokens(now)
	r.refillTokenBucket(now)

	var requestBlockRate, tokenBlockRate float64
	if r.totalRequests > 0 {
		requestBlockRate = float64(r.blockedRequests) / float64(r.totalRequests)
	}
	if r.totalTokens > 0 {
		tokenBlockRate = float64(r.blockedTokens) / float64(r.totalTokens)
	}

	return map[string]interface{}{
		"total_requests":           r.totalRequests,   // 总请求数
		"blocked_requests":         r.blockedRequests, // 被限流的请求数
		"request_block_rate":       requestBlockRate,  // 请求被限流比例
		"total_tokens":             r.totalTokens,     // 总消耗 Token 数
		"blocked_tokens":           r.blockedTokens,   // 被限流的 Token 数
		"token_block_rate":         tokenBlockRate,    // Token 被限流比例
		"available_request_tokens": r.requestTokens,   // 当前可用请求令牌
		"available_token_bucket":   r.tokenBucket,     // 当前可用 Token
		"config": map[string]interface{}{ // 配置信息
			"requests_per_minute": r.config.RequestsPerMinute,
			"tokens_per_minute":   r.config.TokensPerMinute,
			"burst_size":          r.config.BurstSize,
		},
	}
}

// AdaptiveRateLimiter 自适应限流器，根据性能指标动态调整限流速率
type AdaptiveRateLimiter struct {
	baseLimiter RateLimiter      // 基础限流器
	config      *RateLimitConfig // 限流配置
	logger      *zap.Logger      // 日志记录器

	// 自适应参数
	currentRequestRate int       // 当前请求速率
	currentTokenRate   int       // 当前 Token 速率
	errorRate          float64   // 错误率
	lastAdjustment     time.Time // 上次调整时间

	// 性能监控
	avgLatency  time.Duration // 平均延迟
	successRate float64       // 成功率

	mutex sync.RWMutex // 读写锁，保证并发安全
}

// NewAdaptiveRateLimiter 创建自适应限流器实例
func NewAdaptiveRateLimiter(config *RateLimitConfig, logger *zap.Logger) (RateLimiter, error) {
	baseLimiter, err := NewRateLimiter(config, logger)
	if err != nil {
		return nil, err
	}

	adaptive := &AdaptiveRateLimiter{
		baseLimiter:        baseLimiter,
		config:             config,
		logger:             logger,
		currentRequestRate: config.RequestsPerMinute, // 初始化为配置值
		currentTokenRate:   config.TokensPerMinute,   // 初��化为配置值
		lastAdjustment:     time.Now(),
	}

	// 启动自适应调整协程
	go adaptive.startAdaptiveAdjustment()

	return adaptive, nil
}

// Allow 检查是否允许请求（自适应版本，实际调用基础限流器）
func (a *AdaptiveRateLimiter) Allow(ctx context.Context, tokens int) error {
	return a.baseLimiter.Allow(ctx, tokens)
}

// GetStats 获取统计信息，包含自适应参数
func (a *AdaptiveRateLimiter) GetStats() map[string]interface{} {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	baseStats := a.baseLimiter.GetStats()
	baseStats["adaptive"] = map[string]interface{}{
		"current_request_rate": a.currentRequestRate,   // 当前请求速率
		"current_token_rate":   a.currentTokenRate,     // 当前 Token 速率
		"error_rate":           a.errorRate,            // 错误率
		"avg_latency":          a.avgLatency.Seconds(), // 平均延迟（���）
		"success_rate":         a.successRate,          // 成功率
		"last_adjustment":      a.lastAdjustment,       // 上次调整时间
	}

	return baseStats
}

// UpdateMetrics 更新性能指标（延迟和成功率），用于自适应调整
func (a *AdaptiveRateLimiter) UpdateMetrics(latency time.Duration, success bool) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	// 更新平均延迟（简单移动平均）
	if a.avgLatency == 0 {
		a.avgLatency = latency
	} else {
		a.avgLatency = (a.avgLatency + latency) / 2
	}

	// 更新成功率（简单移动平均）
	if success {
		a.successRate = (a.successRate + 1.0) / 2
	} else {
		a.successRate = a.successRate / 2
		a.errorRate = (a.errorRate + 1.0) / 2
	}
}

// startAdaptiveAdjustment 启动自适应调整协程，定期调整限流速率
func (a *AdaptiveRateLimiter) startAdaptiveAdjustment() {
	ticker := time.NewTicker(30 * time.Second) // 每30秒调整一次
	defer ticker.Stop()

	for range ticker.C {
		a.adjustRates()
	}
}

// adjustRates 根据性能指标动态调整限流速率
func (a *AdaptiveRateLimiter) adjustRates() {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	now := time.Now()
	if now.Sub(a.lastAdjustment) < 30*time.Second {
		return
	}

	// 根据错误率和延迟调整限流率
	if a.errorRate > 0.1 || a.avgLatency > 5*time.Second {
		// 错误率高或延迟高，降低限流率
		a.currentRequestRate = int(float64(a.currentRequestRate) * 0.8)
		a.currentTokenRate = int(float64(a.currentTokenRate) * 0.8)

		a.logger.Info("Decreasing rate limits due to high error rate or latency",
			zap.Float64("error_rate", a.errorRate),
			zap.Duration("avg_latency", a.avgLatency),
			zap.Int("new_request_rate", a.currentRequestRate),
			zap.Int("new_token_rate", a.currentTokenRate),
		)

	} else if a.successRate > 0.95 && a.avgLatency < 1*time.Second {
		// 成功率高且延迟低，可以适当提高限流率
		maxRequestRate := int(float64(a.config.RequestsPerMinute) * 1.2)
		maxTokenRate := int(float64(a.config.TokensPerMinute) * 1.2)

		if a.currentRequestRate < maxRequestRate {
			a.currentRequestRate = int(float64(a.currentRequestRate) * 1.1)
			if a.currentRequestRate > maxRequestRate {
				a.currentRequestRate = maxRequestRate
			}
		}

		if a.currentTokenRate < maxTokenRate {
			a.currentTokenRate = int(float64(a.currentTokenRate) * 1.1)
			if a.currentTokenRate > maxTokenRate {
				a.currentTokenRate = maxTokenRate
			}
		}

		a.logger.Debug("Increasing rate limits due to good performance",
			zap.Float64("success_rate", a.successRate),
			zap.Duration("avg_latency", a.avgLatency),
			zap.Int("new_request_rate", a.currentRequestRate),
			zap.Int("new_token_rate", a.currentTokenRate),
		)
	}

	// 确保限流速率不低于最小值
	minRequestRate := a.config.RequestsPerMinute / 4
	minTokenRate := a.config.TokensPerMinute / 4

	if a.currentRequestRate < minRequestRate {
		a.currentRequestRate = minRequestRate
	}
	if a.currentTokenRate < minTokenRate {
		a.currentTokenRate = minTokenRate
	}

	a.lastAdjustment = now

	// 重置统计（避免历史数据影响）
	a.errorRate = 0
	a.successRate = 0.5 // 重置为中性值
}

// MockRateLimiter 模拟限流器（用于测试场景）
type MockRateLimiter struct {
	shouldBlock bool       // 是否阻塞请求
	blockReason string     // 阻塞原因
	allowCount  int64      // 允许请求计数
	blockCount  int64      // 阻塞请求计数
	mutex       sync.Mutex // 互斥锁
}

// NewMockRateLimiter 创建模拟限流器实例
func NewMockRateLimiter() *MockRateLimiter {
	return &MockRateLimiter{}
}

// SetBlocking 设置是否阻塞请求及原因
func (m *MockRateLimiter) SetBlocking(shouldBlock bool, reason string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.shouldBlock = shouldBlock
	m.blockReason = reason
}

// Allow 检查是否允许请求（模拟实现）
func (m *MockRateLimiter) Allow(ctx context.Context, tokens int) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.shouldBlock {
		m.blockCount++
		return fmt.Errorf("%s", m.blockReason)
	}

	m.allowCount++
	return nil
}

// GetStats 获取统计信息（模拟实现）
func (m *MockRateLimiter) GetStats() map[string]interface{} {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	return map[string]interface{}{
		"type":         "mock",        // 类型标识
		"allow_count":  m.allowCount,  // 允许请求计数
		"block_count":  m.blockCount,  // 阻塞请求计数
		"should_block": m.shouldBlock, // 当前是否阻塞
	}
}
