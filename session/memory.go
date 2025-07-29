package session

import (
	"context"
	"fmt"
	"github.com/Anniext/rag/core"
	"sort"
	"strings"
	"time"
)

// MemoryManager 对话上下文管理器
type MemoryManager struct {
	sessionManager core.SessionManager
	logger         core.Logger
	metrics        core.MetricsCollector

	// 配置参数
	maxHistorySize      int           // 最大历史记录数量
	contextWindow       time.Duration // 上下文窗口时间
	similarityThreshold float64       // 相似度阈值
}

// NewMemoryManager 创建新的内存管理器
func NewMemoryManager(
	sessionManager core.SessionManager,
	logger core.Logger,
	metrics core.MetricsCollector,
	maxHistorySize int,
	contextWindow time.Duration,
	similarityThreshold float64,
) *MemoryManager {
	return &MemoryManager{
		sessionManager:      sessionManager,
		logger:              logger,
		metrics:             metrics,
		maxHistorySize:      maxHistorySize,
		contextWindow:       contextWindow,
		similarityThreshold: similarityThreshold,
	}
}

// AddQueryHistory 添加查询历史记录
func (m *MemoryManager) AddQueryHistory(ctx context.Context, sessionID string, history *core.QueryHistory) error {
	if sessionID == "" {
		return fmt.Errorf("session ID cannot be empty")
	}

	if history == nil {
		return fmt.Errorf("history cannot be nil")
	}

	// 获取会话
	session, err := m.sessionManager.GetSession(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	// 添加历史记录
	session.History = append(session.History, history)

	// 限制历史记录数量
	if len(session.History) > m.maxHistorySize {
		// 保留最新的记录
		session.History = session.History[len(session.History)-m.maxHistorySize:]
	}

	// 更新会话
	if err := m.sessionManager.UpdateSession(ctx, session); err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}

	m.logger.Debug("Query history added", "session_id", sessionID, "query", history.Query)
	m.metrics.IncrementCounter("query_history_added", map[string]string{
		"session_id": sessionID,
		"success":    fmt.Sprintf("%t", history.Success),
	})

	return nil
}

// GetQueryHistory 获取查询历史记录
func (m *MemoryManager) GetQueryHistory(ctx context.Context, sessionID string, limit int) ([]*core.QueryHistory, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("session ID cannot be empty")
	}

	// 获取会话
	session, err := m.sessionManager.GetSession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	history := session.History
	if history == nil {
		return []*core.QueryHistory{}, nil
	}

	// 按时间排序（最新的在前）
	sort.Slice(history, func(i, j int) bool {
		return history[i].Timestamp.After(history[j].Timestamp)
	})

	// 限制返回数量
	if limit > 0 && len(history) > limit {
		history = history[:limit]
	}

	m.logger.Debug("Query history retrieved", "session_id", sessionID, "count", len(history))
	return history, nil
}

// GetRecentHistory 获取最近的查询历史（在上下文窗口内）
func (m *MemoryManager) GetRecentHistory(ctx context.Context, sessionID string) ([]*core.QueryHistory, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("session ID cannot be empty")
	}

	// 获取会话
	session, err := m.sessionManager.GetSession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	if session.History == nil {
		return []*core.QueryHistory{}, nil
	}

	// 过滤出上下文窗口内的历史记录
	cutoffTime := time.Now().Add(-m.contextWindow)
	var recentHistory []*core.QueryHistory

	for _, h := range session.History {
		if h.Timestamp.After(cutoffTime) {
			recentHistory = append(recentHistory, h)
		}
	}

	// 按时间排序（最新的在前）
	sort.Slice(recentHistory, func(i, j int) bool {
		return recentHistory[i].Timestamp.After(recentHistory[j].Timestamp)
	})

	m.logger.Debug("Recent history retrieved", "session_id", sessionID, "count", len(recentHistory))
	return recentHistory, nil
}

// FindSimilarQueries 查找相似的查询
func (m *MemoryManager) FindSimilarQueries(ctx context.Context, sessionID string, query string, limit int) ([]*core.QueryHistory, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("session ID cannot be empty")
	}

	if query == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}

	// 获取会话
	session, err := m.sessionManager.GetSession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	if session.History == nil {
		return []*core.QueryHistory{}, nil
	}

	// 计算相似度并过滤
	type scoredHistory struct {
		history *core.QueryHistory
		score   float64
	}

	var candidates []scoredHistory
	queryLower := strings.ToLower(strings.TrimSpace(query))

	for _, h := range session.History {
		if h.Success { // 只考虑成功的查询
			score := m.calculateSimilarity(queryLower, strings.ToLower(strings.TrimSpace(h.Query)))
			if score >= m.similarityThreshold {
				candidates = append(candidates, scoredHistory{
					history: h,
					score:   score,
				})
			}
		}
	}

	// 按相似度排序（高分在前）
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	// 限制返回数量
	if limit > 0 && len(candidates) > limit {
		candidates = candidates[:limit]
	}

	// 提取历史记录
	var similarQueries []*core.QueryHistory
	for _, c := range candidates {
		similarQueries = append(similarQueries, c.history)
	}

	m.logger.Debug("Similar queries found", "session_id", sessionID, "query", query, "count", len(similarQueries))
	m.metrics.IncrementCounter("similar_queries_found", map[string]string{
		"session_id": sessionID,
		"count":      fmt.Sprintf("%d", len(similarQueries)),
	})

	return similarQueries, nil
}

// GetContextualSuggestions 获取基于上下文的查询建议
func (m *MemoryManager) GetContextualSuggestions(ctx context.Context, sessionID string, currentQuery string) ([]string, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("session ID cannot be empty")
	}

	// 获取最近的历史记录
	recentHistory, err := m.GetRecentHistory(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent history: %w", err)
	}

	// 获取相似查询
	var similarQueries []*core.QueryHistory
	if currentQuery != "" {
		similarQueries, err = m.FindSimilarQueries(ctx, sessionID, currentQuery, 5)
		if err != nil {
			m.logger.Warn("Failed to find similar queries", "error", err)
		}
	}

	suggestions := m.generateSuggestions(recentHistory, similarQueries, currentQuery)

	m.logger.Debug("Contextual suggestions generated", "session_id", sessionID, "count", len(suggestions))
	m.metrics.IncrementCounter("contextual_suggestions_generated", map[string]string{
		"session_id": sessionID,
		"count":      fmt.Sprintf("%d", len(suggestions)),
	})

	return suggestions, nil
}

// UpdateContext 更新会话上下文
func (m *MemoryManager) UpdateContext(ctx context.Context, sessionID string, key string, value any) error {
	if sessionID == "" {
		return fmt.Errorf("session ID cannot be empty")
	}

	if key == "" {
		return fmt.Errorf("context key cannot be empty")
	}

	// 获取会话
	session, err := m.sessionManager.GetSession(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	// 更新上下文
	if session.Context == nil {
		session.Context = make(map[string]any)
	}
	session.Context[key] = value

	// 更新会话
	if err := m.sessionManager.UpdateSession(ctx, session); err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}

	m.logger.Debug("Context updated", "session_id", sessionID, "key", key)
	m.metrics.IncrementCounter("context_updated", map[string]string{
		"session_id": sessionID,
		"key":        key,
	})

	return nil
}

// GetContext 获取会话上下文
func (m *MemoryManager) GetContext(ctx context.Context, sessionID string) (map[string]any, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("session ID cannot be empty")
	}

	// 获取会话
	session, err := m.sessionManager.GetSession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	if session.Context == nil {
		return make(map[string]any), nil
	}

	// 返回上下文的副本
	context := make(map[string]any, len(session.Context))
	for k, v := range session.Context {
		context[k] = v
	}

	m.logger.Debug("Context retrieved", "session_id", sessionID, "keys", len(context))
	return context, nil
}

// ClearContext 清空会话上下文
func (m *MemoryManager) ClearContext(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return fmt.Errorf("session ID cannot be empty")
	}

	// 获取会话
	session, err := m.sessionManager.GetSession(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	// 清空上下文
	session.Context = make(map[string]any)

	// 更新会话
	if err := m.sessionManager.UpdateSession(ctx, session); err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}

	m.logger.Debug("Context cleared", "session_id", sessionID)
	m.metrics.IncrementCounter("context_cleared", map[string]string{
		"session_id": sessionID,
	})

	return nil
}

// GetQueryPatterns 分析查询模式
func (m *MemoryManager) GetQueryPatterns(ctx context.Context, sessionID string) (*QueryPatterns, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("session ID cannot be empty")
	}

	// 获取会话
	session, err := m.sessionManager.GetSession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	if len(session.History) == 0 {
		return &QueryPatterns{}, nil
	}

	patterns := m.analyzeQueryPatterns(session.History)

	m.logger.Debug("Query patterns analyzed", "session_id", sessionID)
	return patterns, nil
}

// OptimizeQuery 基于历史记录优化查询
func (m *MemoryManager) OptimizeQuery(ctx context.Context, sessionID string, query string) (*QueryOptimization, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("session ID cannot be empty")
	}

	if query == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}

	// 获取相似查询
	similarQueries, err := m.FindSimilarQueries(ctx, sessionID, query, 3)
	if err != nil {
		return nil, fmt.Errorf("failed to find similar queries: %w", err)
	}

	// 获取最近历史
	recentHistory, err := m.GetRecentHistory(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent history: %w", err)
	}

	optimization := m.generateOptimization(query, similarQueries, recentHistory)

	m.logger.Debug("Query optimization generated", "session_id", sessionID, "query", query)
	m.metrics.IncrementCounter("query_optimization_generated", map[string]string{
		"session_id": sessionID,
	})

	return optimization, nil
}

// 内部方法

// calculateSimilarity 计算两个查询的相似度
func (m *MemoryManager) calculateSimilarity(query1, query2 string) float64 {
	if query1 == query2 {
		return 1.0
	}

	// 简单的基于词汇重叠的相似度计算
	words1 := strings.Fields(query1)
	words2 := strings.Fields(query2)

	if len(words1) == 0 || len(words2) == 0 {
		return 0.0
	}

	// 计算交集
	wordSet1 := make(map[string]bool)
	for _, word := range words1 {
		wordSet1[word] = true
	}

	intersection := 0
	for _, word := range words2 {
		if wordSet1[word] {
			intersection++
		}
	}

	// 计算 Jaccard 相似度
	union := len(words1) + len(words2) - intersection
	if union == 0 {
		return 0.0
	}

	return float64(intersection) / float64(union)
}

// generateSuggestions 生成查询建议
func (m *MemoryManager) generateSuggestions(recentHistory, similarQueries []*core.QueryHistory, currentQuery string) []string {
	suggestions := make(map[string]bool) // 使用 map 去重

	// 基于最近历史的建议
	for _, h := range recentHistory {
		if h.Success && h.Query != currentQuery {
			// 提取查询中的关键词作为建议
			words := strings.Fields(strings.ToLower(h.Query))
			for _, word := range words {
				if len(word) > 3 && !isCommonWord(word) {
					suggestions[fmt.Sprintf("查询包含 '%s' 的数据", word)] = true
				}
			}
		}
	}

	// 基于相似查询的建议
	for _, h := range similarQueries {
		if h.Query != currentQuery {
			suggestions[fmt.Sprintf("类似查询: %s", h.Query)] = true
		}
	}

	// 通用建议
	if len(recentHistory) > 0 {
		suggestions["查看最近的查询结果"] = true
		suggestions["重新执行上一个查询"] = true
	}

	// 转换为切片
	var result []string
	for suggestion := range suggestions {
		result = append(result, suggestion)
	}

	// 限制建议数量
	if len(result) > 10 {
		result = result[:10]
	}

	return result
}

// analyzeQueryPatterns 分析查询模式
func (m *MemoryManager) analyzeQueryPatterns(history []*core.QueryHistory) *QueryPatterns {
	patterns := &QueryPatterns{
		TotalQueries:      len(history),
		SuccessfulQueries: 0,
		FailedQueries:     0,
		CommonKeywords:    make(map[string]int),
		QueryTypes:        make(map[string]int),
		TimePatterns:      make(map[string]int),
	}

	wordCount := make(map[string]int)

	for _, h := range history {
		if h.Success {
			patterns.SuccessfulQueries++
		} else {
			patterns.FailedQueries++
		}

		// 分析关键词
		words := strings.Fields(strings.ToLower(h.Query))
		for _, word := range words {
			if len(word) > 2 && !isCommonWord(word) {
				wordCount[word]++
			}
		}

		// 分析查询类型（简单分类）
		queryType := classifyQuery(h.Query)
		patterns.QueryTypes[queryType]++

		// 分析时间模式
		hour := h.Timestamp.Hour()
		timeSlot := getTimeSlot(hour)
		patterns.TimePatterns[timeSlot]++
	}

	// 提取最常见的关键词
	for word, count := range wordCount {
		if count >= 2 { // 至少出现2次
			patterns.CommonKeywords[word] = count
		}
	}

	return patterns
}

// generateOptimization 生成查询优化建议
func (m *MemoryManager) generateOptimization(query string, similarQueries, recentHistory []*core.QueryHistory) *QueryOptimization {
	optimization := &QueryOptimization{
		OriginalQuery: query,
		Suggestions:   []string{},
		Confidence:    0.0,
	}

	// 基于相似查询的优化
	if len(similarQueries) > 0 {
		optimization.Suggestions = append(optimization.Suggestions, "基于历史查询，建议使用更具体的条件")
		optimization.Confidence += 0.3
	}

	// 基于最近历史的优化
	if len(recentHistory) > 0 {
		lastQuery := recentHistory[0]
		if !lastQuery.Success {
			optimization.Suggestions = append(optimization.Suggestions, "上一个查询失败，建议检查语法")
			optimization.Confidence += 0.2
		}
	}

	// 通用优化建议
	if strings.Contains(strings.ToLower(query), "select *") {
		optimization.Suggestions = append(optimization.Suggestions, "建议指定具体的列名而不是使用 SELECT *")
		optimization.Confidence += 0.1
	}

	if !strings.Contains(strings.ToLower(query), "limit") {
		optimization.Suggestions = append(optimization.Suggestions, "建议添加 LIMIT 子句以限制结果数量")
		optimization.Confidence += 0.1
	}

	// 限制置信度在 0-1 之间
	if optimization.Confidence > 1.0 {
		optimization.Confidence = 1.0
	}

	return optimization
}

// 辅助函数

// isCommonWord 检查是否为常见词
func isCommonWord(word string) bool {
	commonWords := map[string]bool{
		"the": true, "a": true, "an": true, "and": true, "or": true, "but": true,
		"in": true, "on": true, "at": true, "to": true, "for": true, "of": true,
		"with": true, "by": true, "up": true, "about": true, "into": true,
		"through": true, "during": true, "before": true, "after": true, "above": true,
		"below": true, "between": true, "among": true, "is": true, "are": true, "was": true,
		"were": true, "be": true, "been": true, "being": true, "have": true, "has": true,
		"had": true, "do": true, "does": true, "did": true, "will": true, "would": true,
		"could": true, "should": true, "may": true, "might": true, "must": true, "can": true,
		"select": true, "from": true, "where": true, "order": true, "group": true, "having": true,
		"limit": true, "offset": true, "join": true, "inner": true, "left": true, "right": true,
		"outer": true, "union": true, "all": true, "distinct": true, "as": true, "like": true,
		"not": true, "null": true, "true": true, "false": true,
	}
	return commonWords[word]
}

// classifyQuery 简单的查询分类
func classifyQuery(query string) string {
	queryLower := strings.ToLower(query)

	if strings.Contains(queryLower, "select") {
		return "查询"
	} else if strings.Contains(queryLower, "insert") {
		return "插入"
	} else if strings.Contains(queryLower, "update") {
		return "更新"
	} else if strings.Contains(queryLower, "delete") {
		return "删除"
	} else if strings.Contains(queryLower, "create") {
		return "创建"
	} else if strings.Contains(queryLower, "drop") {
		return "删除"
	} else if strings.Contains(queryLower, "alter") {
		return "修改"
	}

	return "其他"
}

// getTimeSlot 获取时间段
func getTimeSlot(hour int) string {
	if hour >= 6 && hour < 12 {
		return "上午"
	} else if hour >= 12 && hour < 18 {
		return "下午"
	} else if hour >= 18 && hour < 22 {
		return "晚上"
	} else {
		return "深夜"
	}
}

// QueryPatterns 查询模式分析结果
type QueryPatterns struct {
	TotalQueries      int            `json:"total_queries"`
	SuccessfulQueries int            `json:"successful_queries"`
	FailedQueries     int            `json:"failed_queries"`
	CommonKeywords    map[string]int `json:"common_keywords"`
	QueryTypes        map[string]int `json:"query_types"`
	TimePatterns      map[string]int `json:"time_patterns"`
}

// QueryOptimization 查询优化建议
type QueryOptimization struct {
	OriginalQuery string   `json:"original_query"`
	Suggestions   []string `json:"suggestions"`
	Confidence    float64  `json:"confidence"`
}
