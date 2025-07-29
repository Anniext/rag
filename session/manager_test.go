package session

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"pumppill/rag/core"
)

// MockCacheManager 模拟缓存管理器
type MockCacheManager struct {
	mock.Mock
}

func (m *MockCacheManager) Get(ctx context.Context, key string) (interface{}, error) {
	args := m.Called(ctx, key)
	return args.Get(0), args.Error(1)
}

func (m *MockCacheManager) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	args := m.Called(ctx, key, value, ttl)
	return args.Error(0)
}

func (m *MockCacheManager) Delete(ctx context.Context, key string) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

func (m *MockCacheManager) Clear(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// MockLogger 模拟日志记录器
type MockLogger struct {
	mock.Mock
}

func (m *MockLogger) Debug(msg string, fields ...interface{}) {
	m.Called(msg, fields)
}

func (m *MockLogger) Info(msg string, fields ...interface{}) {
	m.Called(msg, fields)
}

func (m *MockLogger) Warn(msg string, fields ...interface{}) {
	m.Called(msg, fields)
}

func (m *MockLogger) Error(msg string, fields ...interface{}) {
	m.Called(msg, fields)
}

func (m *MockLogger) Fatal(msg string, fields ...interface{}) {
	m.Called(msg, fields)
}

// MockMetricsCollector 模拟指标收集器
type MockMetricsCollector struct {
	mock.Mock
}

func (m *MockMetricsCollector) IncrementCounter(name string, labels map[string]string) {
	m.Called(name, labels)
}

func (m *MockMetricsCollector) RecordHistogram(name string, value float64, labels map[string]string) {
	m.Called(name, value, labels)
}

func (m *MockMetricsCollector) SetGauge(name string, value float64, labels map[string]string) {
	m.Called(name, value, labels)
}

// setupMocks 设置通用的模拟期望
func setupMocks(cache *MockCacheManager, logger *MockLogger, metrics *MockMetricsCollector, ttl time.Duration) {
	// 通用的日志期望
	logger.On("Info", mock.AnythingOfType("string"), mock.Anything).Return()
	logger.On("Debug", mock.AnythingOfType("string"), mock.Anything).Return()
	logger.On("Warn", mock.AnythingOfType("string"), mock.Anything).Return()
	logger.On("Error", mock.AnythingOfType("string"), mock.Anything).Return()

	// 通用的缓存期望
	cache.On("Set", mock.Anything, mock.AnythingOfType("string"), mock.Anything, ttl).Return(nil)
	cache.On("Get", mock.Anything, mock.AnythingOfType("string")).Return(nil, assert.AnError)
	cache.On("Delete", mock.Anything, mock.AnythingOfType("string")).Return(nil)

	// 通用的指标期望
	metrics.On("IncrementCounter", mock.AnythingOfType("string"), mock.Anything).Return()
	metrics.On("SetGauge", mock.AnythingOfType("string"), mock.AnythingOfType("float64"), mock.Anything).Return()
}

func TestNewManager(t *testing.T) {
	cache := &MockCacheManager{}
	logger := &MockLogger{}
	metrics := &MockMetricsCollector{}

	setupMocks(cache, logger, metrics, time.Hour)

	manager := NewManager(time.Hour, cache, logger, metrics)

	assert.NotNil(t, manager)
	assert.Equal(t, time.Hour, manager.ttl)
	assert.Equal(t, cache, manager.cache)
	assert.Equal(t, logger, manager.logger)
	assert.Equal(t, metrics, manager.metrics)
	assert.NotNil(t, manager.sessions)
	assert.NotNil(t, manager.cleanupTicker)
	assert.NotNil(t, manager.stopCleanup)

	// 清理
	manager.Stop()
}

func TestCreateSession(t *testing.T) {
	cache := &MockCacheManager{}
	logger := &MockLogger{}
	metrics := &MockMetricsCollector{}

	setupMocks(cache, logger, metrics, time.Hour)

	manager := NewManager(time.Hour, cache, logger, metrics)
	defer manager.Stop()

	ctx := context.Background()
	userID := "test-user-123"

	session, err := manager.CreateSession(ctx, userID)

	assert.NoError(t, err)
	assert.NotNil(t, session)
	assert.NotEmpty(t, session.SessionID)
	assert.Equal(t, userID, session.UserID)
	assert.NotNil(t, session.History)
	assert.NotNil(t, session.Context)
	assert.NotNil(t, session.Preferences)
	assert.False(t, session.CreatedAt.IsZero())
	assert.False(t, session.LastAccessed.IsZero())

	// 验证默认偏好设置
	assert.Equal(t, "zh-CN", session.Preferences.Language)
	assert.Equal(t, "2006-01-02", session.Preferences.DateFormat)
	assert.Equal(t, "Asia/Shanghai", session.Preferences.TimeZone)
	assert.Equal(t, 20, session.Preferences.DefaultLimit)
	assert.False(t, session.Preferences.ExplainQueries)
	assert.True(t, session.Preferences.OptimizeQueries)

	// 验证会话已存储在内存中
	assert.Equal(t, 1, manager.GetSessionCount())
}

func TestGetSession(t *testing.T) {
	cache := &MockCacheManager{}
	logger := &MockLogger{}
	metrics := &MockMetricsCollector{}

	setupMocks(cache, logger, metrics, time.Hour)

	manager := NewManager(time.Hour, cache, logger, metrics)
	defer manager.Stop()

	ctx := context.Background()
	userID := "test-user-123"

	// 创建会话
	originalSession, err := manager.CreateSession(ctx, userID)
	assert.NoError(t, err)

	// 获取会话
	retrievedSession, err := manager.GetSession(ctx, originalSession.SessionID)
	assert.NoError(t, err)
	assert.NotNil(t, retrievedSession)
	assert.Equal(t, originalSession.SessionID, retrievedSession.SessionID)
	assert.Equal(t, originalSession.UserID, retrievedSession.UserID)

	// 验证最后访问时间已更新
	assert.True(t, retrievedSession.LastAccessed.After(originalSession.LastAccessed) ||
		retrievedSession.LastAccessed.Equal(originalSession.LastAccessed))
}

func TestGetSessionNotFound(t *testing.T) {
	cache := &MockCacheManager{}
	logger := &MockLogger{}
	metrics := &MockMetricsCollector{}

	setupMocks(cache, logger, metrics, time.Hour)

	manager := NewManager(time.Hour, cache, logger, metrics)
	defer manager.Stop()

	ctx := context.Background()

	session, err := manager.GetSession(ctx, "non-existent")
	assert.Error(t, err)
	assert.Nil(t, session)
	assert.Contains(t, err.Error(), "session not found")
}

func TestGetSessionEmpty(t *testing.T) {
	cache := &MockCacheManager{}
	logger := &MockLogger{}
	metrics := &MockMetricsCollector{}

	setupMocks(cache, logger, metrics, time.Hour)

	manager := NewManager(time.Hour, cache, logger, metrics)
	defer manager.Stop()

	ctx := context.Background()

	session, err := manager.GetSession(ctx, "")
	assert.Error(t, err)
	assert.Nil(t, session)
	assert.Contains(t, err.Error(), "session ID cannot be empty")
}

func TestUpdateSession(t *testing.T) {
	cache := &MockCacheManager{}
	logger := &MockLogger{}
	metrics := &MockMetricsCollector{}

	setupMocks(cache, logger, metrics, time.Hour)

	manager := NewManager(time.Hour, cache, logger, metrics)
	defer manager.Stop()

	ctx := context.Background()
	userID := "test-user-123"

	// 创建会话
	session, err := manager.CreateSession(ctx, userID)
	assert.NoError(t, err)

	// 修改会话
	session.Context["test_key"] = "test_value"
	originalLastAccessed := session.LastAccessed

	// 等待一小段时间确保时间戳不同
	time.Sleep(10 * time.Millisecond)

	// 更新会话
	err = manager.UpdateSession(ctx, session)
	assert.NoError(t, err)

	// 验证最后访问时间已更新
	assert.True(t, session.LastAccessed.After(originalLastAccessed))

	// 验证更新已保存
	retrievedSession, err := manager.GetSession(ctx, session.SessionID)
	assert.NoError(t, err)
	assert.Equal(t, "test_value", retrievedSession.Context["test_key"])
}

func TestUpdateSessionInvalid(t *testing.T) {
	cache := &MockCacheManager{}
	logger := &MockLogger{}
	metrics := &MockMetricsCollector{}

	setupMocks(cache, logger, metrics, time.Hour)

	manager := NewManager(time.Hour, cache, logger, metrics)
	defer manager.Stop()

	ctx := context.Background()

	// 测试 nil 会话
	err := manager.UpdateSession(ctx, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid session")

	// 测试空会话 ID
	session := &core.SessionMemory{
		SessionID: "",
		UserID:    "test-user",
	}
	err = manager.UpdateSession(ctx, session)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid session")
}

func TestDeleteSession(t *testing.T) {
	cache := &MockCacheManager{}
	logger := &MockLogger{}
	metrics := &MockMetricsCollector{}

	setupMocks(cache, logger, metrics, time.Hour)

	manager := NewManager(time.Hour, cache, logger, metrics)
	defer manager.Stop()

	ctx := context.Background()
	userID := "test-user-123"

	// 创建会话
	session, err := manager.CreateSession(ctx, userID)
	assert.NoError(t, err)
	assert.Equal(t, 1, manager.GetSessionCount())

	// 删除会话
	err = manager.DeleteSession(ctx, session.SessionID)
	assert.NoError(t, err)
	assert.Equal(t, 0, manager.GetSessionCount())

	// 验证会话已删除
	_, err = manager.GetSession(ctx, session.SessionID)
	assert.Error(t, err)
}

func TestDeleteSessionEmpty(t *testing.T) {
	cache := &MockCacheManager{}
	logger := &MockLogger{}
	metrics := &MockMetricsCollector{}

	setupMocks(cache, logger, metrics, time.Hour)

	manager := NewManager(time.Hour, cache, logger, metrics)
	defer manager.Stop()

	ctx := context.Background()

	err := manager.DeleteSession(ctx, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "session ID cannot be empty")
}

func TestListUserSessions(t *testing.T) {
	cache := &MockCacheManager{}
	logger := &MockLogger{}
	metrics := &MockMetricsCollector{}

	setupMocks(cache, logger, metrics, time.Hour)

	manager := NewManager(time.Hour, cache, logger, metrics)
	defer manager.Stop()

	ctx := context.Background()
	userID1 := "test-user-1"
	userID2 := "test-user-2"

	// 为用户1创建两个会话
	session1, err := manager.CreateSession(ctx, userID1)
	assert.NoError(t, err)
	session2, err := manager.CreateSession(ctx, userID1)
	assert.NoError(t, err)

	// 为用户2创建一个会话
	session3, err := manager.CreateSession(ctx, userID2)
	assert.NoError(t, err)

	// 列出用户1的会话
	user1Sessions, err := manager.ListUserSessions(ctx, userID1)
	assert.NoError(t, err)
	assert.Len(t, user1Sessions, 2)

	sessionIDs := make([]string, len(user1Sessions))
	for i, s := range user1Sessions {
		sessionIDs[i] = s.SessionID
	}
	assert.Contains(t, sessionIDs, session1.SessionID)
	assert.Contains(t, sessionIDs, session2.SessionID)

	// 列出用户2的会话
	user2Sessions, err := manager.ListUserSessions(ctx, userID2)
	assert.NoError(t, err)
	assert.Len(t, user2Sessions, 1)
	assert.Equal(t, session3.SessionID, user2Sessions[0].SessionID)
}

func TestListUserSessionsEmpty(t *testing.T) {
	cache := &MockCacheManager{}
	logger := &MockLogger{}
	metrics := &MockMetricsCollector{}

	setupMocks(cache, logger, metrics, time.Hour)

	manager := NewManager(time.Hour, cache, logger, metrics)
	defer manager.Stop()

	ctx := context.Background()

	sessions, err := manager.ListUserSessions(ctx, "")
	assert.Error(t, err)
	assert.Nil(t, sessions)
	assert.Contains(t, err.Error(), "user ID cannot be empty")
}

func TestCleanupExpiredSessions(t *testing.T) {
	cache := &MockCacheManager{}
	logger := &MockLogger{}
	metrics := &MockMetricsCollector{}

	// 使用很短的 TTL 进行测试
	setupMocks(cache, logger, metrics, 100*time.Millisecond)

	manager := NewManager(100*time.Millisecond, cache, logger, metrics)
	defer manager.Stop()

	ctx := context.Background()
	userID := "test-user-123"

	// 创建会话
	session, err := manager.CreateSession(ctx, userID)
	assert.NoError(t, err)
	assert.Equal(t, 1, manager.GetSessionCount())

	// 等待会话过期
	time.Sleep(150 * time.Millisecond)

	// 清理过期会话
	err = manager.CleanupExpiredSessions(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 0, manager.GetSessionCount())

	// 验证会话已被清理
	_, err = manager.GetSession(ctx, session.SessionID)
	assert.Error(t, err)
}

func TestGetSessionStats(t *testing.T) {
	cache := &MockCacheManager{}
	logger := &MockLogger{}
	metrics := &MockMetricsCollector{}

	setupMocks(cache, logger, metrics, time.Hour)

	manager := NewManager(time.Hour, cache, logger, metrics)
	defer manager.Stop()

	ctx := context.Background()
	userID1 := "test-user-1"
	userID2 := "test-user-2"

	// 为用户1创建两个会话
	_, err := manager.CreateSession(ctx, userID1)
	assert.NoError(t, err)
	_, err = manager.CreateSession(ctx, userID1)
	assert.NoError(t, err)

	// 为用户2创建一个会话
	_, err = manager.CreateSession(ctx, userID2)
	assert.NoError(t, err)

	// 获取统计信息
	stats := manager.GetSessionStats()

	assert.Equal(t, 3, stats["total_sessions"])
	assert.Equal(t, 3600, stats["ttl_seconds"])

	sessionsByUser, ok := stats["sessions_by_user"].(map[string]int)
	assert.True(t, ok)
	assert.Equal(t, 2, sessionsByUser[userID1])
	assert.Equal(t, 1, sessionsByUser[userID2])
}

func TestSessionExpiry(t *testing.T) {
	cache := &MockCacheManager{}
	logger := &MockLogger{}
	metrics := &MockMetricsCollector{}

	// 使用很短的 TTL 进行测试，但不设置自动清理
	setupMocks(cache, logger, metrics, 100*time.Millisecond)

	// 创建管理器但立即停止清理协程
	manager := NewManager(100*time.Millisecond, cache, logger, metrics)
	manager.cleanupTicker.Stop() // 停止自动清理
	defer manager.Stop()

	ctx := context.Background()
	userID := "test-user-123"

	// 创建会话
	session, err := manager.CreateSession(ctx, userID)
	assert.NoError(t, err)
	assert.Equal(t, 1, manager.GetSessionCount())

	// 等待会话过期
	time.Sleep(150 * time.Millisecond)

	// 尝试获取过期会话，此时会话仍在内存中但已过期
	_, err = manager.GetSession(ctx, session.SessionID)
	assert.Error(t, err)
	// 会话过期后，应该返回 "session expired" 错误
	assert.Contains(t, err.Error(), "session expired")

	// 验证会话已从内存中删除（因为 GetSession 检测到过期并删除了它）
	assert.Equal(t, 0, manager.GetSessionCount())
}
