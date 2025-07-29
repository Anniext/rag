package session

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"pumppill/rag/core"
)

// MockSessionManager 模拟会话管理器
type MockSessionManager struct {
	mock.Mock
}

func (m *MockSessionManager) CreateSession(ctx context.Context, userID string) (*core.SessionMemory, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).(*core.SessionMemory), args.Error(1)
}

func (m *MockSessionManager) GetSession(ctx context.Context, sessionID string) (*core.SessionMemory, error) {
	args := m.Called(ctx, sessionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*core.SessionMemory), args.Error(1)
}

func (m *MockSessionManager) UpdateSession(ctx context.Context, session *core.SessionMemory) error {
	args := m.Called(ctx, session)
	return args.Error(0)
}

func (m *MockSessionManager) DeleteSession(ctx context.Context, sessionID string) error {
	args := m.Called(ctx, sessionID)
	return args.Error(0)
}

func (m *MockSessionManager) CleanupExpiredSessions(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockSessionManager) ListUserSessions(ctx context.Context, userID string) ([]*core.SessionMemory, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).([]*core.SessionMemory), args.Error(1)
}

func createTestSession(sessionID string) *core.SessionMemory {
	return &core.SessionMemory{
		SessionID:    sessionID,
		UserID:       "test-user",
		History:      []*core.QueryHistory{},
		Context:      make(map[string]any),
		Preferences:  &core.UserPreferences{},
		CreatedAt:    time.Now(),
		LastAccessed: time.Now(),
	}
}

func TestNewMemoryManager(t *testing.T) {
	sessionManager := &MockSessionManager{}
	logger := &MockLogger{}
	metrics := &MockMetricsCollector{}

	memoryManager := NewMemoryManager(
		sessionManager,
		logger,
		metrics,
		100,
		time.Hour,
		0.5,
	)

	assert.NotNil(t, memoryManager)
	assert.Equal(t, sessionManager, memoryManager.sessionManager)
	assert.Equal(t, logger, memoryManager.logger)
	assert.Equal(t, metrics, memoryManager.metrics)
	assert.Equal(t, 100, memoryManager.maxHistorySize)
	assert.Equal(t, time.Hour, memoryManager.contextWindow)
	assert.Equal(t, 0.5, memoryManager.similarityThreshold)
}

func TestAddQueryHistory(t *testing.T) {
	sessionManager := &MockSessionManager{}
	logger := &MockLogger{}
	metrics := &MockMetricsCollector{}

	session := createTestSession("test-session")
	history := &core.QueryHistory{
		Query:         "SELECT * FROM users",
		SQL:           "SELECT * FROM users",
		Success:       true,
		Timestamp:     time.Now(),
		ExecutionTime: 100 * time.Millisecond,
	}

	// 设置模拟期望
	sessionManager.On("GetSession", mock.Anything, "test-session").Return(session, nil)
	sessionManager.On("UpdateSession", mock.Anything, session).Return(nil)
	logger.On("Debug", mock.AnythingOfType("string"), mock.Anything).Return()
	metrics.On("IncrementCounter", "query_history_added", mock.AnythingOfType("map[string]string")).Return()

	memoryManager := NewMemoryManager(sessionManager, logger, metrics, 100, time.Hour, 0.5)

	ctx := context.Background()
	err := memoryManager.AddQueryHistory(ctx, "test-session", history)

	assert.NoError(t, err)
	assert.Len(t, session.History, 1)
	assert.Equal(t, history, session.History[0])

	// 验证模拟调用
	sessionManager.AssertExpectations(t)
	logger.AssertExpectations(t)
	metrics.AssertExpectations(t)
}

func TestAddQueryHistoryMaxSize(t *testing.T) {
	sessionManager := &MockSessionManager{}
	logger := &MockLogger{}
	metrics := &MockMetricsCollector{}

	session := createTestSession("test-session")

	// 预先添加一些历史记录
	for i := 0; i < 5; i++ {
		session.History = append(session.History, &core.QueryHistory{
			Query:     "old query",
			Timestamp: time.Now().Add(-time.Duration(i) * time.Minute),
		})
	}

	newHistory := &core.QueryHistory{
		Query:         "new query",
		SQL:           "SELECT * FROM users",
		Success:       true,
		Timestamp:     time.Now(),
		ExecutionTime: 100 * time.Millisecond,
	}

	// 设置模拟期望
	sessionManager.On("GetSession", mock.Anything, "test-session").Return(session, nil)
	sessionManager.On("UpdateSession", mock.Anything, session).Return(nil)
	logger.On("Debug", mock.AnythingOfType("string"), mock.Anything).Return()
	metrics.On("IncrementCounter", "query_history_added", mock.AnythingOfType("map[string]string")).Return()

	// 设置最大历史记录数为 3
	memoryManager := NewMemoryManager(sessionManager, logger, metrics, 3, time.Hour, 0.5)

	ctx := context.Background()
	err := memoryManager.AddQueryHistory(ctx, "test-session", newHistory)

	assert.NoError(t, err)
	assert.Len(t, session.History, 3)                                    // 应该被限制为 3 条记录
	assert.Equal(t, newHistory, session.History[len(session.History)-1]) // 新记录应该在最后

	// 验证模拟调用
	sessionManager.AssertExpectations(t)
	logger.AssertExpectations(t)
	metrics.AssertExpectations(t)
}

func TestAddQueryHistoryInvalidInput(t *testing.T) {
	sessionManager := &MockSessionManager{}
	logger := &MockLogger{}
	metrics := &MockMetricsCollector{}

	memoryManager := NewMemoryManager(sessionManager, logger, metrics, 100, time.Hour, 0.5)
	ctx := context.Background()

	// 测试空会话 ID
	err := memoryManager.AddQueryHistory(ctx, "", &core.QueryHistory{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "session ID cannot be empty")

	// 测试 nil 历史记录
	err = memoryManager.AddQueryHistory(ctx, "test-session", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "history cannot be nil")
}

func TestGetQueryHistory(t *testing.T) {
	sessionManager := &MockSessionManager{}
	logger := &MockLogger{}
	metrics := &MockMetricsCollector{}

	session := createTestSession("test-session")

	// 添加一些历史记录
	now := time.Now()
	session.History = []*core.QueryHistory{
		{Query: "query1", Timestamp: now.Add(-3 * time.Minute)},
		{Query: "query2", Timestamp: now.Add(-2 * time.Minute)},
		{Query: "query3", Timestamp: now.Add(-1 * time.Minute)},
	}

	// 设置模拟期望
	sessionManager.On("GetSession", mock.Anything, "test-session").Return(session, nil)
	logger.On("Debug", mock.AnythingOfType("string"), mock.Anything).Return()

	memoryManager := NewMemoryManager(sessionManager, logger, metrics, 100, time.Hour, 0.5)

	ctx := context.Background()
	history, err := memoryManager.GetQueryHistory(ctx, "test-session", 0)

	assert.NoError(t, err)
	assert.Len(t, history, 3)
	// 应该按时间倒序排列（最新的在前）
	assert.Equal(t, "query3", history[0].Query)
	assert.Equal(t, "query2", history[1].Query)
	assert.Equal(t, "query1", history[2].Query)

	// 验证模拟调用
	sessionManager.AssertExpectations(t)
	logger.AssertExpectations(t)
}

func TestGetQueryHistoryWithLimit(t *testing.T) {
	sessionManager := &MockSessionManager{}
	logger := &MockLogger{}
	metrics := &MockMetricsCollector{}

	session := createTestSession("test-session")

	// 添加一些历史记录
	now := time.Now()
	session.History = []*core.QueryHistory{
		{Query: "query1", Timestamp: now.Add(-3 * time.Minute)},
		{Query: "query2", Timestamp: now.Add(-2 * time.Minute)},
		{Query: "query3", Timestamp: now.Add(-1 * time.Minute)},
	}

	// 设置模拟期望
	sessionManager.On("GetSession", mock.Anything, "test-session").Return(session, nil)
	logger.On("Debug", mock.AnythingOfType("string"), mock.Anything).Return()

	memoryManager := NewMemoryManager(sessionManager, logger, metrics, 100, time.Hour, 0.5)

	ctx := context.Background()
	history, err := memoryManager.GetQueryHistory(ctx, "test-session", 2)

	assert.NoError(t, err)
	assert.Len(t, history, 2)
	// 应该返回最新的 2 条记录
	assert.Equal(t, "query3", history[0].Query)
	assert.Equal(t, "query2", history[1].Query)

	// 验证模拟调用
	sessionManager.AssertExpectations(t)
	logger.AssertExpectations(t)
}

func TestGetRecentHistory(t *testing.T) {
	sessionManager := &MockSessionManager{}
	logger := &MockLogger{}
	metrics := &MockMetricsCollector{}

	session := createTestSession("test-session")

	// 添加一些历史记录，有些在上下文窗口内，有些在外
	now := time.Now()
	session.History = []*core.QueryHistory{
		{Query: "old_query", Timestamp: now.Add(-2 * time.Hour)},        // 超出上下文窗口
		{Query: "recent_query1", Timestamp: now.Add(-30 * time.Minute)}, // 在上下文窗口内
		{Query: "recent_query2", Timestamp: now.Add(-10 * time.Minute)}, // 在上下文窗口内
	}

	// 设置模拟期望
	sessionManager.On("GetSession", mock.Anything, "test-session").Return(session, nil)
	logger.On("Debug", mock.AnythingOfType("string"), mock.Anything).Return()

	// 设置上下文窗口为 1 小时
	memoryManager := NewMemoryManager(sessionManager, logger, metrics, 100, time.Hour, 0.5)

	ctx := context.Background()
	recentHistory, err := memoryManager.GetRecentHistory(ctx, "test-session")

	assert.NoError(t, err)
	assert.Len(t, recentHistory, 2)
	// 应该按时间倒序排列（最新的在前）
	assert.Equal(t, "recent_query2", recentHistory[0].Query)
	assert.Equal(t, "recent_query1", recentHistory[1].Query)

	// 验证模拟调用
	sessionManager.AssertExpectations(t)
	logger.AssertExpectations(t)
}

func TestFindSimilarQueries(t *testing.T) {
	sessionManager := &MockSessionManager{}
	logger := &MockLogger{}
	metrics := &MockMetricsCollector{}

	session := createTestSession("test-session")

	// 添加一些历史记录
	session.History = []*core.QueryHistory{
		{Query: "SELECT name FROM users", Success: true},
		{Query: "SELECT email FROM users", Success: true},
		{Query: "SELECT * FROM products", Success: true},
		{Query: "failed query", Success: false}, // 失败的查询应该被忽略
	}

	// 设置模拟期望
	sessionManager.On("GetSession", mock.Anything, "test-session").Return(session, nil)
	logger.On("Debug", mock.AnythingOfType("string"), mock.Anything).Return()
	metrics.On("IncrementCounter", "similar_queries_found", mock.AnythingOfType("map[string]string")).Return()

	// 使用更高的相似度阈值，确保只找到真正相似的查询
	memoryManager := NewMemoryManager(sessionManager, logger, metrics, 100, time.Hour, 0.5)

	ctx := context.Background()
	similarQueries, err := memoryManager.FindSimilarQueries(ctx, "test-session", "SELECT id FROM users", 10)

	assert.NoError(t, err)
	// 应该找到包含 "users" 的相似查询
	assert.GreaterOrEqual(t, len(similarQueries), 1)

	// 验证找到的查询都是成功的
	for _, query := range similarQueries {
		assert.True(t, query.Success) // 只应该返回成功的查询
	}

	// 验证模拟调用
	sessionManager.AssertExpectations(t)
	logger.AssertExpectations(t)
	metrics.AssertExpectations(t)
}

func TestUpdateContext(t *testing.T) {
	sessionManager := &MockSessionManager{}
	logger := &MockLogger{}
	metrics := &MockMetricsCollector{}

	session := createTestSession("test-session")

	// 设置模拟期望
	sessionManager.On("GetSession", mock.Anything, "test-session").Return(session, nil)
	sessionManager.On("UpdateSession", mock.Anything, session).Return(nil)
	logger.On("Debug", mock.AnythingOfType("string"), mock.Anything).Return()
	metrics.On("IncrementCounter", "context_updated", mock.AnythingOfType("map[string]string")).Return()

	memoryManager := NewMemoryManager(sessionManager, logger, metrics, 100, time.Hour, 0.5)

	ctx := context.Background()
	err := memoryManager.UpdateContext(ctx, "test-session", "current_table", "users")

	assert.NoError(t, err)
	assert.Equal(t, "users", session.Context["current_table"])

	// 验证模拟调用
	sessionManager.AssertExpectations(t)
	logger.AssertExpectations(t)
	metrics.AssertExpectations(t)
}

func TestGetContext(t *testing.T) {
	sessionManager := &MockSessionManager{}
	logger := &MockLogger{}
	metrics := &MockMetricsCollector{}

	session := createTestSession("test-session")
	session.Context["current_table"] = "users"
	session.Context["last_query"] = "SELECT * FROM users"

	// 设置模拟期望
	sessionManager.On("GetSession", mock.Anything, "test-session").Return(session, nil)
	logger.On("Debug", mock.AnythingOfType("string"), mock.Anything).Return()

	memoryManager := NewMemoryManager(sessionManager, logger, metrics, 100, time.Hour, 0.5)

	ctx := context.Background()
	context, err := memoryManager.GetContext(ctx, "test-session")

	assert.NoError(t, err)
	assert.Len(t, context, 2)
	assert.Equal(t, "users", context["current_table"])
	assert.Equal(t, "SELECT * FROM users", context["last_query"])

	// 验证返回的是副本，修改不会影响原始上下文
	context["new_key"] = "new_value"
	assert.NotContains(t, session.Context, "new_key")

	// 验证模拟调用
	sessionManager.AssertExpectations(t)
	logger.AssertExpectations(t)
}

func TestClearContext(t *testing.T) {
	sessionManager := &MockSessionManager{}
	logger := &MockLogger{}
	metrics := &MockMetricsCollector{}

	session := createTestSession("test-session")
	session.Context["current_table"] = "users"
	session.Context["last_query"] = "SELECT * FROM users"

	// 设置模拟期望
	sessionManager.On("GetSession", mock.Anything, "test-session").Return(session, nil)
	sessionManager.On("UpdateSession", mock.Anything, session).Return(nil)
	logger.On("Debug", mock.AnythingOfType("string"), mock.Anything).Return()
	metrics.On("IncrementCounter", "context_cleared", mock.AnythingOfType("map[string]string")).Return()

	memoryManager := NewMemoryManager(sessionManager, logger, metrics, 100, time.Hour, 0.5)

	ctx := context.Background()
	err := memoryManager.ClearContext(ctx, "test-session")

	assert.NoError(t, err)
	assert.Len(t, session.Context, 0)

	// 验证模拟调用
	sessionManager.AssertExpectations(t)
	logger.AssertExpectations(t)
	metrics.AssertExpectations(t)
}

func TestGetContextualSuggestions(t *testing.T) {
	sessionManager := &MockSessionManager{}
	logger := &MockLogger{}
	metrics := &MockMetricsCollector{}

	session := createTestSession("test-session")

	// 添加一些最近的历史记录
	now := time.Now()
	session.History = []*core.QueryHistory{
		{Query: "SELECT name FROM users WHERE age > 18", Success: true, Timestamp: now.Add(-10 * time.Minute)},
		{Query: "SELECT email FROM users", Success: true, Timestamp: now.Add(-5 * time.Minute)},
	}

	// 设置模拟期望
	sessionManager.On("GetSession", mock.Anything, "test-session").Return(session, nil)
	logger.On("Debug", mock.AnythingOfType("string"), mock.Anything).Return()
	metrics.On("IncrementCounter", "similar_queries_found", mock.AnythingOfType("map[string]string")).Return()
	metrics.On("IncrementCounter", "contextual_suggestions_generated", mock.AnythingOfType("map[string]string")).Return()

	memoryManager := NewMemoryManager(sessionManager, logger, metrics, 100, time.Hour, 0.3)

	ctx := context.Background()
	suggestions, err := memoryManager.GetContextualSuggestions(ctx, "test-session", "SELECT * FROM users")

	assert.NoError(t, err)
	assert.NotEmpty(t, suggestions)

	// 验证模拟调用
	sessionManager.AssertExpectations(t)
	logger.AssertExpectations(t)
	metrics.AssertExpectations(t)
}

func TestGetQueryPatterns(t *testing.T) {
	sessionManager := &MockSessionManager{}
	logger := &MockLogger{}
	metrics := &MockMetricsCollector{}

	session := createTestSession("test-session")

	// 添加一些历史记录
	session.History = []*core.QueryHistory{
		{Query: "SELECT name FROM users", Success: true, Timestamp: time.Now().Add(-1 * time.Hour)},
		{Query: "SELECT email FROM users", Success: true, Timestamp: time.Now().Add(-2 * time.Hour)},
		{Query: "INSERT INTO users VALUES (...)", Success: false, Timestamp: time.Now().Add(-3 * time.Hour)},
	}

	// 设置模拟期望
	sessionManager.On("GetSession", mock.Anything, "test-session").Return(session, nil)
	logger.On("Debug", mock.AnythingOfType("string"), mock.Anything).Return()

	memoryManager := NewMemoryManager(sessionManager, logger, metrics, 100, time.Hour, 0.5)

	ctx := context.Background()
	patterns, err := memoryManager.GetQueryPatterns(ctx, "test-session")

	assert.NoError(t, err)
	assert.NotNil(t, patterns)
	assert.Equal(t, 3, patterns.TotalQueries)
	assert.Equal(t, 2, patterns.SuccessfulQueries)
	assert.Equal(t, 1, patterns.FailedQueries)
	assert.NotEmpty(t, patterns.QueryTypes)
	assert.NotEmpty(t, patterns.TimePatterns)

	// 验证模拟调用
	sessionManager.AssertExpectations(t)
	logger.AssertExpectations(t)
}

func TestOptimizeQuery(t *testing.T) {
	sessionManager := &MockSessionManager{}
	logger := &MockLogger{}
	metrics := &MockMetricsCollector{}

	session := createTestSession("test-session")

	// 添加一些历史记录
	now := time.Now()
	session.History = []*core.QueryHistory{
		{Query: "SELECT name FROM users WHERE age > 18", Success: true, Timestamp: now.Add(-10 * time.Minute)},
		{Query: "SELECT email FROM users", Success: true, Timestamp: now.Add(-5 * time.Minute)},
	}

	// 设置模拟期望
	sessionManager.On("GetSession", mock.Anything, "test-session").Return(session, nil)
	logger.On("Debug", mock.AnythingOfType("string"), mock.Anything).Return()
	metrics.On("IncrementCounter", "similar_queries_found", mock.AnythingOfType("map[string]string")).Return()
	metrics.On("IncrementCounter", "query_optimization_generated", mock.AnythingOfType("map[string]string")).Return()

	memoryManager := NewMemoryManager(sessionManager, logger, metrics, 100, time.Hour, 0.3)

	ctx := context.Background()
	optimization, err := memoryManager.OptimizeQuery(ctx, "test-session", "SELECT * FROM users")

	assert.NoError(t, err)
	assert.NotNil(t, optimization)
	assert.Equal(t, "SELECT * FROM users", optimization.OriginalQuery)
	assert.NotEmpty(t, optimization.Suggestions)
	assert.GreaterOrEqual(t, optimization.Confidence, 0.0)
	assert.LessOrEqual(t, optimization.Confidence, 1.0)

	// 验证模拟调用
	sessionManager.AssertExpectations(t)
	logger.AssertExpectations(t)
	metrics.AssertExpectations(t)
}

func TestCalculateSimilarity(t *testing.T) {
	sessionManager := &MockSessionManager{}
	logger := &MockLogger{}
	metrics := &MockMetricsCollector{}

	memoryManager := NewMemoryManager(sessionManager, logger, metrics, 100, time.Hour, 0.5)

	// 测试完全相同的查询
	similarity := memoryManager.calculateSimilarity("select name from users", "select name from users")
	assert.Equal(t, 1.0, similarity)

	// 测试部分相似的查询
	similarity = memoryManager.calculateSimilarity("select name from users", "select email from users")
	assert.Greater(t, similarity, 0.0)
	assert.Less(t, similarity, 1.0)

	// 测试完全不同的查询
	similarity = memoryManager.calculateSimilarity("select name from users", "insert into products")
	assert.GreaterOrEqual(t, similarity, 0.0)

	// 测试空查询
	similarity = memoryManager.calculateSimilarity("", "select name from users")
	assert.Equal(t, 0.0, similarity)
}

func TestClassifyQuery(t *testing.T) {
	assert.Equal(t, "查询", classifyQuery("SELECT * FROM users"))
	assert.Equal(t, "插入", classifyQuery("INSERT INTO users VALUES (...)"))
	assert.Equal(t, "更新", classifyQuery("UPDATE users SET name = 'John'"))
	assert.Equal(t, "删除", classifyQuery("DELETE FROM users WHERE id = 1"))
	assert.Equal(t, "创建", classifyQuery("CREATE TABLE users (...)"))
	assert.Equal(t, "删除", classifyQuery("DROP TABLE users"))
	assert.Equal(t, "修改", classifyQuery("ALTER TABLE users ADD COLUMN age INT"))
	assert.Equal(t, "其他", classifyQuery("SHOW TABLES"))
}

func TestGetTimeSlot(t *testing.T) {
	assert.Equal(t, "上午", getTimeSlot(9))
	assert.Equal(t, "下午", getTimeSlot(15))
	assert.Equal(t, "晚上", getTimeSlot(20))
	assert.Equal(t, "深夜", getTimeSlot(2))
}

func TestIsCommonWord(t *testing.T) {
	assert.True(t, isCommonWord("select"))
	assert.True(t, isCommonWord("from"))
	assert.True(t, isCommonWord("where"))
	assert.False(t, isCommonWord("users"))
	assert.False(t, isCommonWord("products"))
}
