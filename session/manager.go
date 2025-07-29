package session

import (
	"context"
	"fmt"
	"github.com/Anniext/rag/core"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager 会话管理器实现
type Manager struct {
	sessions map[string]*core.SessionMemory
	mutex    sync.RWMutex
	ttl      time.Duration
	cache    core.CacheManager
	logger   core.Logger
	metrics  core.MetricsCollector

	// 清理任务相关
	cleanupTicker *time.Ticker
	stopCleanup   chan struct{}
}

// NewManager 创建新的会话管理器
func NewManager(ttl time.Duration, cache core.CacheManager, logger core.Logger, metrics core.MetricsCollector) *Manager {
	manager := &Manager{
		sessions:      make(map[string]*core.SessionMemory),
		ttl:           ttl,
		cache:         cache,
		logger:        logger,
		metrics:       metrics,
		cleanupTicker: time.NewTicker(ttl / 4), // 每1/4 TTL时间清理一次
		stopCleanup:   make(chan struct{}),
	}

	// 启动清理协程
	go manager.startCleanupRoutine()

	return manager
}

// CreateSession 创建新会话
func (m *Manager) CreateSession(ctx context.Context, userID string) (*core.SessionMemory, error) {
	sessionID := uuid.New().String()
	now := time.Now()

	session := &core.SessionMemory{
		SessionID:    sessionID,
		UserID:       userID,
		History:      make([]*core.QueryHistory, 0),
		Context:      make(map[string]any),
		Preferences:  m.getDefaultPreferences(),
		CreatedAt:    now,
		LastAccessed: now,
	}

	m.mutex.Lock()
	m.sessions[sessionID] = session
	m.mutex.Unlock()

	// 持久化到缓存
	if m.cache != nil {
		if err := m.cache.Set(ctx, m.getSessionKey(sessionID), session, m.ttl); err != nil {
			m.logger.Warn("Failed to persist session to cache", "session_id", sessionID, "error", err)
		}
	}

	m.logger.Info("Session created", "session_id", sessionID, "user_id", userID)
	m.metrics.IncrementCounter("session_created", map[string]string{
		"user_id": userID,
	})

	return session, nil
}

// GetSession 获取会话
func (m *Manager) GetSession(ctx context.Context, sessionID string) (*core.SessionMemory, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("session ID cannot be empty")
	}

	m.mutex.RLock()
	session, exists := m.sessions[sessionID]
	m.mutex.RUnlock()

	if exists {
		// 检查会话是否过期
		if time.Since(session.LastAccessed) > m.ttl {
			m.logger.Info("Session expired", "session_id", sessionID)
			m.deleteSessionInternal(sessionID)
			m.metrics.IncrementCounter("session_expired", map[string]string{
				"session_id": sessionID,
			})
			return nil, fmt.Errorf("session expired")
		}

		// 更新最后访问时间
		session.LastAccessed = time.Now()
		return session, nil
	}

	// 尝试从缓存恢复
	if m.cache != nil {
		cached, err := m.cache.Get(ctx, m.getSessionKey(sessionID))
		if err == nil && cached != nil {
			if session, ok := cached.(*core.SessionMemory); ok {
				// 检查缓存中的会话是否过期
				if time.Since(session.LastAccessed) <= m.ttl {
					session.LastAccessed = time.Now()

					// 恢复到内存
					m.mutex.Lock()
					m.sessions[sessionID] = session
					m.mutex.Unlock()

					m.logger.Info("Session restored from cache", "session_id", sessionID)
					m.metrics.IncrementCounter("session_restored", map[string]string{
						"session_id": sessionID,
					})
					return session, nil
				}
			}
		}
	}

	m.metrics.IncrementCounter("session_not_found", map[string]string{
		"session_id": sessionID,
	})
	return nil, fmt.Errorf("session not found")
}

// UpdateSession 更新会话
func (m *Manager) UpdateSession(ctx context.Context, session *core.SessionMemory) error {
	if session == nil || session.SessionID == "" {
		return fmt.Errorf("invalid session")
	}

	session.LastAccessed = time.Now()

	m.mutex.Lock()
	m.sessions[session.SessionID] = session
	m.mutex.Unlock()

	// 持久化到缓存
	if m.cache != nil {
		if err := m.cache.Set(ctx, m.getSessionKey(session.SessionID), session, m.ttl); err != nil {
			m.logger.Warn("Failed to update session in cache", "session_id", session.SessionID, "error", err)
		}
	}

	m.logger.Debug("Session updated", "session_id", session.SessionID)
	m.metrics.IncrementCounter("session_updated", map[string]string{
		"session_id": session.SessionID,
	})

	return nil
}

// DeleteSession 删除会话
func (m *Manager) DeleteSession(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return fmt.Errorf("session ID cannot be empty")
	}

	m.deleteSessionInternal(sessionID)

	// 从缓存删除
	if m.cache != nil {
		if err := m.cache.Delete(ctx, m.getSessionKey(sessionID)); err != nil {
			m.logger.Warn("Failed to delete session from cache", "session_id", sessionID, "error", err)
		}
	}

	m.logger.Info("Session deleted", "session_id", sessionID)
	m.metrics.IncrementCounter("session_deleted", map[string]string{
		"session_id": sessionID,
	})

	return nil
}

// CleanupExpiredSessions 清理过期会话
func (m *Manager) CleanupExpiredSessions(ctx context.Context) error {
	now := time.Now()
	expiredSessions := make([]string, 0)

	m.mutex.RLock()
	for sessionID, session := range m.sessions {
		if now.Sub(session.LastAccessed) > m.ttl {
			expiredSessions = append(expiredSessions, sessionID)
		}
	}
	m.mutex.RUnlock()

	// 删除过期会话
	for _, sessionID := range expiredSessions {
		m.deleteSessionInternal(sessionID)

		// 从缓存删除
		if m.cache != nil {
			if err := m.cache.Delete(ctx, m.getSessionKey(sessionID)); err != nil {
				m.logger.Warn("Failed to delete expired session from cache", "session_id", sessionID, "error", err)
			}
		}
	}

	if len(expiredSessions) > 0 {
		m.logger.Info("Cleaned up expired sessions", "count", len(expiredSessions))
		m.metrics.SetGauge("expired_sessions_cleaned", float64(len(expiredSessions)), nil)
	}

	// 更新活跃会话数量指标
	m.mutex.RLock()
	activeCount := len(m.sessions)
	m.mutex.RUnlock()

	m.metrics.SetGauge("active_sessions", float64(activeCount), nil)

	return nil
}

// ListUserSessions 列出用户的所有会话
func (m *Manager) ListUserSessions(ctx context.Context, userID string) ([]*core.SessionMemory, error) {
	if userID == "" {
		return nil, fmt.Errorf("user ID cannot be empty")
	}

	var userSessions []*core.SessionMemory

	m.mutex.RLock()
	for _, session := range m.sessions {
		if session.UserID == userID {
			// 检查会话是否过期
			if time.Since(session.LastAccessed) <= m.ttl {
				userSessions = append(userSessions, session)
			}
		}
	}
	m.mutex.RUnlock()

	m.logger.Debug("Listed user sessions", "user_id", userID, "count", len(userSessions))
	return userSessions, nil
}

// Stop 停止会话管理器
func (m *Manager) Stop() {
	if m.cleanupTicker != nil {
		m.cleanupTicker.Stop()
	}

	close(m.stopCleanup)

	m.logger.Info("Session manager stopped")
}

// GetSessionCount 获取当前活跃会话数量
func (m *Manager) GetSessionCount() int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return len(m.sessions)
}

// GetSessionStats 获取会话统计信息
func (m *Manager) GetSessionStats() map[string]interface{} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	stats := map[string]interface{}{
		"total_sessions": len(m.sessions),
		"ttl_seconds":    int(m.ttl.Seconds()),
	}

	// 按用户统计会话数量
	userSessions := make(map[string]int)
	for _, session := range m.sessions {
		userSessions[session.UserID]++
	}
	stats["sessions_by_user"] = userSessions

	return stats
}

// 内部方法

// deleteSessionInternal 内部删除会话方法
func (m *Manager) deleteSessionInternal(sessionID string) {
	m.mutex.Lock()
	delete(m.sessions, sessionID)
	m.mutex.Unlock()
}

// getSessionKey 获取会话在缓存中的键
func (m *Manager) getSessionKey(sessionID string) string {
	return fmt.Sprintf("session:%s", sessionID)
}

// getDefaultPreferences 获取默认用户偏好设置
func (m *Manager) getDefaultPreferences() *core.UserPreferences {
	return &core.UserPreferences{
		Language:        "zh-CN",
		DateFormat:      "2006-01-02",
		TimeZone:        "Asia/Shanghai",
		DefaultLimit:    20,
		ExplainQueries:  false,
		OptimizeQueries: true,
	}
}

// startCleanupRoutine 启动清理协程
func (m *Manager) startCleanupRoutine() {
	for {
		select {
		case <-m.cleanupTicker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			if err := m.CleanupExpiredSessions(ctx); err != nil {
				m.logger.Error("Failed to cleanup expired sessions", "error", err)
			}
			cancel()
		case <-m.stopCleanup:
			return
		}
	}
}
