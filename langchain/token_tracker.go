package langchain

import (
	"go.uber.org/zap"
	"pumppill/rag/core"
	"sync"
	"time"
)

// tokenTrackerImpl 是 Token 使用跟踪器的具体实现，负责统计和管理 Token 的使用情况，包括总量、历史记录和时间窗口统计。
type tokenTrackerImpl struct {
	config *TokenUsageConfig // 跟踪器配置
	logger *zap.Logger       // 日志记录器

	// 当前 Token 使用总量统计
	totalUsage *core.TokenUsage

	// 历史 Token 使用记录
	history    []*TokenUsageRecord
	maxHistory int // 历史记录最大保存数量

	// 时间窗口统计（如 1 分钟、5 分钟等）
	windowStats map[string]*WindowStats

	mutex sync.RWMutex // 读写锁，保证并发安全
}

// TokenUsageRecord 表示一次 Token 使用的详细记录，包括使用量、时间戳和请求 ID。
type TokenUsageRecord struct {
	Usage     *core.TokenUsage `json:"usage"`                // Token 使用详情
	Timestamp time.Time        `json:"timestamp"`            // 记录时间
	RequestID string           `json:"request_id,omitempty"` // 可选，请求唯一标识
}

// WindowStats 表示某个时间窗口内的 Token 使用统计信息。
type WindowStats struct {
	StartTime        time.Time `json:"start_time"`        // 窗口开始时间
	EndTime          time.Time `json:"end_time"`          // 窗口结束时间
	TotalRequests    int       `json:"total_requests"`    // 总请求次数
	TotalTokens      int       `json:"total_tokens"`      // Token 总数
	PromptTokens     int       `json:"prompt_tokens"`     // Prompt Token 数量
	CompletionTokens int       `json:"completion_tokens"` // Completion Token 数量
	AverageTokens    float64   `json:"average_tokens"`    // 平均每次请求 Token 数
}

// NewTokenTracker 创建并初始化一个 Token 跟踪器实例。
func NewTokenTracker(config *TokenUsageConfig, logger *zap.Logger) TokenTracker {
	if config == nil {
		config = &TokenUsageConfig{
			EnableTracking: true,            // 默认启用跟踪
			ReportInterval: 5 * time.Minute, // 默认报告间隔 5 分钟
			MaxHistory:     1000,            // 默认最大历史记录数
		}
	}

	tracker := &tokenTrackerImpl{
		config:      config,
		logger:      logger,
		totalUsage:  &core.TokenUsage{},
		history:     make([]*TokenUsageRecord, 0),
		maxHistory:  config.MaxHistory,
		windowStats: make(map[string]*WindowStats),
	}

	if config.EnableTracking {
		// 启动定期报告协程
		go tracker.startPeriodicReporting()

		// 启动时间窗口统计清理协程
		go tracker.startWindowCleanup()
	}

	logger.Info("Token tracker created",
		zap.Bool("tracking_enabled", config.EnableTracking),
		zap.Duration("report_interval", config.ReportInterval),
		zap.Int("max_history", config.MaxHistory),
	)

	return tracker
}

// RecordUsage 记录一次 Token 使用，并更新统计信息。
func (t *tokenTrackerImpl) RecordUsage(usage *core.TokenUsage) {
	if !t.config.EnableTracking || usage == nil {
		return
	}

	t.mutex.Lock()
	defer t.mutex.Unlock()

	// 累加到总 Token 使用量
	t.totalUsage.PromptTokens += usage.PromptTokens
	t.totalUsage.CompletionTokens += usage.CompletionTokens
	t.totalUsage.TotalTokens += usage.TotalTokens

	// 添加到历史记录
	record := &TokenUsageRecord{
		Usage:     usage,
		Timestamp: time.Now(),
	}
	t.history = append(t.history, record)

	// 超过最大历史记录数时，删除最旧的记录
	if len(t.history) > t.maxHistory {
		copy(t.history, t.history[1:])
		t.history = t.history[:len(t.history)-1]
	}

	// 更新各时间窗口统计
	t.updateWindowStats(usage)

	t.logger.Debug("Token usage recorded",
		zap.Int("prompt_tokens", usage.PromptTokens),
		zap.Int("completion_tokens", usage.CompletionTokens),
		zap.Int("total_tokens", usage.TotalTokens),
	)
}

// GetUsage 获取当前 Token 使用总量（返回副本，避免并发问题）。
func (t *tokenTrackerImpl) GetUsage() *core.TokenUsage {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	return &core.TokenUsage{
		PromptTokens:     t.totalUsage.PromptTokens,
		CompletionTokens: t.totalUsage.CompletionTokens,
		TotalTokens:      t.totalUsage.TotalTokens,
	}
}

// GetHistory 获取所有历史 Token 使用记录（只返回 TokenUsage 部分）。
func (t *tokenTrackerImpl) GetHistory() []*core.TokenUsage {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	history := make([]*core.TokenUsage, len(t.history))
	for i, record := range t.history {
		history[i] = record.Usage
	}

	return history
}

// Reset 重置所有统计信息，包括总量、历史和窗口统计。
func (t *tokenTrackerImpl) Reset() {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	t.totalUsage = &core.TokenUsage{}
	t.history = make([]*TokenUsageRecord, 0)
	t.windowStats = make(map[string]*WindowStats)

	t.logger.Info("Token usage statistics reset")
}

// updateWindowStats 根据当前 Token 使用，更新各时间窗口的统计信息。
func (t *tokenTrackerImpl) updateWindowStats(usage *core.TokenUsage) {
	now := time.Now()

	// 定义需要统计的时间窗口
	windows := map[string]time.Duration{
		"1m":  1 * time.Minute,
		"5m":  5 * time.Minute,
		"15m": 15 * time.Minute,
		"1h":  1 * time.Hour,
		"24h": 24 * time.Hour,
	}

	for windowName, duration := range windows {
		stats, exists := t.windowStats[windowName]
		// 如果窗口不存在或已过期，则新建窗口统计
		if !exists || now.Sub(stats.StartTime) >= duration {
			stats = &WindowStats{
				StartTime: now,
				EndTime:   now.Add(duration),
			}
			t.windowStats[windowName] = stats
		}

		// 累加统计数据
		stats.TotalRequests++
		stats.TotalTokens += usage.TotalTokens
		stats.PromptTokens += usage.PromptTokens
		stats.CompletionTokens += usage.CompletionTokens
		stats.AverageTokens = float64(stats.TotalTokens) / float64(stats.TotalRequests)
	}
}

// GetWindowStats 获取所有时间窗口的统计信息（返回副本）。
func (t *tokenTrackerImpl) GetWindowStats() map[string]*WindowStats {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	result := make(map[string]*WindowStats)
	for k, v := range t.windowStats {
		result[k] = &WindowStats{
			StartTime:        v.StartTime,
			EndTime:          v.EndTime,
			TotalRequests:    v.TotalRequests,
			TotalTokens:      v.TotalTokens,
			PromptTokens:     v.PromptTokens,
			CompletionTokens: v.CompletionTokens,
			AverageTokens:    v.AverageTokens,
		}
	}

	return result
}

// GetDetailedStats 获取详细统计信息，包括总量、历史、平均值、窗口统计和配置。
func (t *tokenTrackerImpl) GetDetailedStats() map[string]interface{} {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	var avgPromptTokens, avgCompletionTokens, avgTotalTokens float64
	historyCount := len(t.history)

	if historyCount > 0 {
		var totalPrompt, totalCompletion, totalTotal int
		for _, record := range t.history {
			totalPrompt += record.Usage.PromptTokens
			totalCompletion += record.Usage.CompletionTokens
			totalTotal += record.Usage.TotalTokens
		}

		avgPromptTokens = float64(totalPrompt) / float64(historyCount)
		avgCompletionTokens = float64(totalCompletion) / float64(historyCount)
		avgTotalTokens = float64(totalTotal) / float64(historyCount)
	}

	return map[string]interface{}{
		"total_usage":   t.totalUsage, // 总 Token 使用量
		"history_count": historyCount, // 历史记录条数
		"averages": map[string]float64{
			"prompt_tokens":     avgPromptTokens,     // 平均 Prompt Token
			"completion_tokens": avgCompletionTokens, // 平均 Completion Token
			"total_tokens":      avgTotalTokens,      // 平均总 Token
		},
		"window_stats": t.windowStats, // 时间窗口统计
		"config": map[string]interface{}{
			"tracking_enabled": t.config.EnableTracking, // 是否启用跟踪
			"report_interval":  t.config.ReportInterval, // 报告间隔
			"max_history":      t.config.MaxHistory,     // 最大历史记录数
		},
	}
}

// startPeriodicReporting 启动定期报告协程，定时输出 Token 使用报告。
func (t *tokenTrackerImpl) startPeriodicReporting() {
	if t.config.ReportInterval <= 0 {
		return
	}

	ticker := time.NewTicker(t.config.ReportInterval)
	defer ticker.Stop()

	for range ticker.C {
		t.generateReport()
	}
}

// generateReport 生成并输出当前 Token 使用报告到日志。
func (t *tokenTrackerImpl) generateReport() {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	windowStats := t.GetWindowStats()

	t.logger.Info("Token usage report",
		zap.Int("total_prompt_tokens", t.totalUsage.PromptTokens),
		zap.Int("total_completion_tokens", t.totalUsage.CompletionTokens),
		zap.Int("total_tokens", t.totalUsage.TotalTokens),
		zap.Int("history_count", len(t.history)),
		zap.Any("window_stats", windowStats),
	)
}

// startWindowCleanup 启动时间窗口统计清理协程，每小时清理一次过期窗口。
func (t *tokenTrackerImpl) startWindowCleanup() {
	ticker := time.NewTicker(1 * time.Hour) // 每小时清理一次
	defer ticker.Stop()

	for range ticker.C {
		t.cleanupExpiredWindows()
	}
}

// cleanupExpiredWindows 清理已过期的时间窗口统计信息。
func (t *tokenTrackerImpl) cleanupExpiredWindows() {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	now := time.Now()
	cleanedCount := 0

	for windowName, stats := range t.windowStats {
		if now.After(stats.EndTime) {
			delete(t.windowStats, windowName)
			cleanedCount++
		}
	}

	if cleanedCount > 0 {
		t.logger.Debug("Cleaned up expired window statistics",
			zap.Int("cleaned_count", cleanedCount),
		)
	}
}

// GetRecentUsage 获取最近 duration 时间内的 Token 使用总量。
func (t *tokenTrackerImpl) GetRecentUsage(duration time.Duration) *core.TokenUsage {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	cutoff := time.Now().Add(-duration)
	usage := &core.TokenUsage{}

	for _, record := range t.history {
		if record.Timestamp.After(cutoff) {
			usage.PromptTokens += record.Usage.PromptTokens
			usage.CompletionTokens += record.Usage.CompletionTokens
			usage.TotalTokens += record.Usage.TotalTokens
		}
	}

	return usage
}

// GetUsageByTimeRange 获取指定时间范围内的 Token 使用记录。
func (t *tokenTrackerImpl) GetUsageByTimeRange(start, end time.Time) []*TokenUsageRecord {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	var result []*TokenUsageRecord

	for _, record := range t.history {
		if record.Timestamp.After(start) && record.Timestamp.Before(end) {
			result = append(result, &TokenUsageRecord{
				Usage:     record.Usage,
				Timestamp: record.Timestamp,
				RequestID: record.RequestID,
			})
		}
	}

	return result
}

// EstimateCost 根据传入的单价，估算当前 Token 使用的总成本。
func (t *tokenTrackerImpl) EstimateCost(pricePerPromptToken, pricePerCompletionToken float64) float64 {
	usage := t.GetUsage()

	promptCost := float64(usage.PromptTokens) * pricePerPromptToken
	completionCost := float64(usage.CompletionTokens) * pricePerCompletionToken

	return promptCost + completionCost
}

// MockTokenTracker 是用于测试的 Token 跟踪器模拟实现。
type MockTokenTracker struct {
	usage   *core.TokenUsage   // 模拟总 Token 使用量
	history []*core.TokenUsage // 模拟历史记录
	mutex   sync.RWMutex       // 并发安全锁
}

// NewMockTokenTracker 创建一个新的模拟 Token 跟踪器。
func NewMockTokenTracker() *MockTokenTracker {
	return &MockTokenTracker{
		usage:   &core.TokenUsage{},
		history: make([]*core.TokenUsage, 0),
	}
}

// RecordUsage 记录一次 Token 使用（模拟实现）。
func (m *MockTokenTracker) RecordUsage(usage *core.TokenUsage) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.usage.PromptTokens += usage.PromptTokens
	m.usage.CompletionTokens += usage.CompletionTokens
	m.usage.TotalTokens += usage.TotalTokens

	m.history = append(m.history, usage)
}

// GetUsage 获取当前 Token 使用总量（模拟实现）。
func (m *MockTokenTracker) GetUsage() *core.TokenUsage {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return &core.TokenUsage{
		PromptTokens:     m.usage.PromptTokens,
		CompletionTokens: m.usage.CompletionTokens,
		TotalTokens:      m.usage.TotalTokens,
	}
}

// GetHistory 获取所有历史 Token 使用记录（模拟实现）。
func (m *MockTokenTracker) GetHistory() []*core.TokenUsage {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	result := make([]*core.TokenUsage, len(m.history))
	copy(result, m.history)
	return result
}

// Reset 重置所有统计信息（模拟实现）。
func (m *MockTokenTracker) Reset() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.usage = &core.TokenUsage{}
	m.history = make([]*core.TokenUsage, 0)
}
