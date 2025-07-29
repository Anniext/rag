package security

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestAuditLogger_LogEvent(t *testing.T) {
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsCollector{}
	mockCache := &MockCacheManager{}

	// 设置 mock 期望
	mockLogger.On("Info", mock.AnythingOfType("string"), mock.Anything).Return()
	mockMetrics.On("IncrementCounter", mock.AnythingOfType("string"), mock.AnythingOfType("map[string]string")).Return()
	mockMetrics.On("RecordHistogram", mock.AnythingOfType("string"), mock.AnythingOfType("float64"), mock.AnythingOfType("map[string]string")).Return()
	mockCache.On("Get", mock.Anything, mock.AnythingOfType("string")).Return(nil, assert.AnError)
	mockCache.On("Set", mock.Anything, mock.AnythingOfType("string"), mock.Anything, mock.AnythingOfType("time.Duration")).Return(nil)

	auditLogger := NewAuditLogger(mockLogger, mockMetrics, mockCache)

	t.Run("LogEvent_Success", func(t *testing.T) {
		event := &AuditEvent{
			EventType: AuditEventTypeLogin,
			UserID:    "123",
			Username:  "testuser",
			Resource:  "auth",
			Action:    "login",
			Result:    AuditResultSuccess,
			IPAddress: "192.168.1.1",
			Duration:  100 * time.Millisecond,
		}

		auditLogger.LogEvent(context.Background(), event)

		// 验证事件ID和时间戳被设置
		assert.NotEmpty(t, event.ID)
		assert.False(t, event.Timestamp.IsZero())

		// 验证 mock 调用
		mockLogger.AssertCalled(t, "Info", "审计事件", mock.Anything)
		mockMetrics.AssertCalled(t, "IncrementCounter", "audit_events_total", mock.AnythingOfType("map[string]string"))
		mockMetrics.AssertCalled(t, "RecordHistogram", "audit_event_duration", mock.AnythingOfType("float64"), mock.AnythingOfType("map[string]string"))
	})

	t.Run("LogEvent_SecurityViolation", func(t *testing.T) {
		event := &AuditEvent{
			EventType: AuditEventTypeSecurityViolation,
			UserID:    "123",
			Username:  "testuser",
			Resource:  "security",
			Action:    "sql_injection",
			Result:    AuditResultDenied,
			IPAddress: "192.168.1.1",
		}

		auditLogger.LogEvent(context.Background(), event)

		// 验证安全违规事件被正确处理
		mockMetrics.AssertCalled(t, "IncrementCounter", "security_violations_total", mock.AnythingOfType("map[string]string"))
	})
}

func TestAuditLogger_LogLogin(t *testing.T) {
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsCollector{}
	mockCache := &MockCacheManager{}

	// 设置 mock 期望
	mockLogger.On("Info", mock.AnythingOfType("string"), mock.Anything).Return()
	mockMetrics.On("IncrementCounter", mock.AnythingOfType("string"), mock.AnythingOfType("map[string]string")).Return()
	mockCache.On("Get", mock.Anything, mock.AnythingOfType("string")).Return(nil, assert.AnError)
	mockCache.On("Set", mock.Anything, mock.AnythingOfType("string"), mock.Anything, mock.AnythingOfType("time.Duration")).Return(nil)

	auditLogger := NewAuditLogger(mockLogger, mockMetrics, mockCache)

	t.Run("LogLogin_Success", func(t *testing.T) {
		auditLogger.LogLogin(context.Background(), "123", "testuser", "192.168.1.1", "Mozilla/5.0", true, nil)

		mockLogger.AssertCalled(t, "Info", "审计事件", mock.Anything)
		mockMetrics.AssertCalled(t, "IncrementCounter", "audit_events_total", mock.AnythingOfType("map[string]string"))
	})

	t.Run("LogLogin_Failure", func(t *testing.T) {
		err := assert.AnError
		auditLogger.LogLogin(context.Background(), "123", "testuser", "192.168.1.1", "Mozilla/5.0", false, err)

		mockLogger.AssertCalled(t, "Info", "审计事件", mock.Anything)
		mockMetrics.AssertCalled(t, "IncrementCounter", "audit_events_total", mock.AnythingOfType("map[string]string"))
	})
}

func TestAuditLogger_LogTokenValidation(t *testing.T) {
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsCollector{}
	mockCache := &MockCacheManager{}

	// 设置 mock 期望
	mockLogger.On("Info", mock.AnythingOfType("string"), mock.Anything).Return()
	mockMetrics.On("IncrementCounter", mock.AnythingOfType("string"), mock.AnythingOfType("map[string]string")).Return()
	mockCache.On("Get", mock.Anything, mock.AnythingOfType("string")).Return(nil, assert.AnError)
	mockCache.On("Set", mock.Anything, mock.AnythingOfType("string"), mock.Anything, mock.AnythingOfType("time.Duration")).Return(nil)

	auditLogger := NewAuditLogger(mockLogger, mockMetrics, mockCache)

	t.Run("LogTokenValidation_Success", func(t *testing.T) {
		auditLogger.LogTokenValidation(context.Background(), "123", "testuser", "192.168.1.1", true, nil)

		mockLogger.AssertCalled(t, "Info", "审计事件", mock.Anything)
		mockMetrics.AssertCalled(t, "IncrementCounter", "audit_events_total", mock.AnythingOfType("map[string]string"))
	})

	t.Run("LogTokenValidation_Failure", func(t *testing.T) {
		err := assert.AnError
		auditLogger.LogTokenValidation(context.Background(), "123", "testuser", "192.168.1.1", false, err)

		mockLogger.AssertCalled(t, "Info", "审计事件", mock.Anything)
		mockMetrics.AssertCalled(t, "IncrementCounter", "audit_events_total", mock.AnythingOfType("map[string]string"))
	})
}

func TestAuditLogger_LogPermissionCheck(t *testing.T) {
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsCollector{}
	mockCache := &MockCacheManager{}

	// 设置 mock 期望
	mockLogger.On("Info", mock.AnythingOfType("string"), mock.Anything).Return()
	mockMetrics.On("IncrementCounter", mock.AnythingOfType("string"), mock.AnythingOfType("map[string]string")).Return()
	mockCache.On("Get", mock.Anything, mock.AnythingOfType("string")).Return(nil, assert.AnError)
	mockCache.On("Set", mock.Anything, mock.AnythingOfType("string"), mock.Anything, mock.AnythingOfType("time.Duration")).Return(nil)

	auditLogger := NewAuditLogger(mockLogger, mockMetrics, mockCache)

	t.Run("LogPermissionCheck_Success", func(t *testing.T) {
		auditLogger.LogPermissionCheck(context.Background(), "123", "testuser", "users", "read", "192.168.1.1", true, nil)

		mockLogger.AssertCalled(t, "Info", "审计事件", mock.Anything)
		mockMetrics.AssertCalled(t, "IncrementCounter", "audit_events_total", mock.AnythingOfType("map[string]string"))
	})

	t.Run("LogPermissionCheck_Denied", func(t *testing.T) {
		err := assert.AnError
		auditLogger.LogPermissionCheck(context.Background(), "123", "testuser", "users", "write", "192.168.1.1", false, err)

		mockLogger.AssertCalled(t, "Info", "审计事件", mock.Anything)
		mockMetrics.AssertCalled(t, "IncrementCounter", "audit_events_total", mock.AnythingOfType("map[string]string"))
	})
}

func TestAuditLogger_LogDataAccess(t *testing.T) {
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsCollector{}
	mockCache := &MockCacheManager{}

	// 设置 mock 期望
	mockLogger.On("Info", mock.AnythingOfType("string"), mock.Anything).Return()
	mockMetrics.On("IncrementCounter", mock.AnythingOfType("string"), mock.AnythingOfType("map[string]string")).Return()
	mockMetrics.On("RecordHistogram", mock.AnythingOfType("string"), mock.AnythingOfType("float64"), mock.AnythingOfType("map[string]string")).Return()
	mockCache.On("Get", mock.Anything, mock.AnythingOfType("string")).Return(nil, assert.AnError)
	mockCache.On("Set", mock.Anything, mock.AnythingOfType("string"), mock.Anything, mock.AnythingOfType("time.Duration")).Return(nil)

	auditLogger := NewAuditLogger(mockLogger, mockMetrics, mockCache)

	t.Run("LogDataAccess", func(t *testing.T) {
		auditLogger.LogDataAccess(context.Background(), "123", "testuser", "users", "SELECT", "192.168.1.1", 10, 50*time.Millisecond)

		mockLogger.AssertCalled(t, "Info", "审计事件", mock.Anything)
		mockMetrics.AssertCalled(t, "IncrementCounter", "audit_events_total", mock.AnythingOfType("map[string]string"))
		mockMetrics.AssertCalled(t, "RecordHistogram", "audit_event_duration", mock.AnythingOfType("float64"), mock.AnythingOfType("map[string]string"))
	})
}

func TestAuditLogger_LogQueryExecution(t *testing.T) {
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsCollector{}
	mockCache := &MockCacheManager{}

	// 设置 mock 期望
	mockLogger.On("Info", mock.AnythingOfType("string"), mock.Anything).Return()
	mockMetrics.On("IncrementCounter", mock.AnythingOfType("string"), mock.AnythingOfType("map[string]string")).Return()
	mockMetrics.On("RecordHistogram", mock.AnythingOfType("string"), mock.AnythingOfType("float64"), mock.AnythingOfType("map[string]string")).Return()
	mockCache.On("Get", mock.Anything, mock.AnythingOfType("string")).Return(nil, assert.AnError)
	mockCache.On("Set", mock.Anything, mock.AnythingOfType("string"), mock.Anything, mock.AnythingOfType("time.Duration")).Return(nil)

	auditLogger := NewAuditLogger(mockLogger, mockMetrics, mockCache)

	t.Run("LogQueryExecution_Success", func(t *testing.T) {
		sql := "SELECT * FROM users WHERE status = 'active'"
		auditLogger.LogQueryExecution(context.Background(), "123", "testuser", sql, "192.168.1.1", true, 100*time.Millisecond, 5, nil)

		mockLogger.AssertCalled(t, "Info", "审计事件", mock.Anything)
		mockMetrics.AssertCalled(t, "IncrementCounter", "audit_events_total", mock.AnythingOfType("map[string]string"))
		mockMetrics.AssertCalled(t, "RecordHistogram", "audit_event_duration", mock.AnythingOfType("float64"), mock.AnythingOfType("map[string]string"))
	})

	t.Run("LogQueryExecution_Failure", func(t *testing.T) {
		sql := "SELECT * FROM invalid_table"
		err := assert.AnError
		auditLogger.LogQueryExecution(context.Background(), "123", "testuser", sql, "192.168.1.1", false, 10*time.Millisecond, 0, err)

		mockLogger.AssertCalled(t, "Info", "审计事件", mock.Anything)
		mockMetrics.AssertCalled(t, "IncrementCounter", "audit_events_total", mock.AnythingOfType("map[string]string"))
	})
}

func TestAuditLogger_LogSecurityViolation(t *testing.T) {
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsCollector{}
	mockCache := &MockCacheManager{}

	// 设置 mock 期望
	mockLogger.On("Info", mock.AnythingOfType("string"), mock.Anything).Return()
	mockLogger.On("Error", mock.AnythingOfType("string"), mock.Anything).Return()
	mockMetrics.On("IncrementCounter", mock.AnythingOfType("string"), mock.AnythingOfType("map[string]string")).Return()
	mockCache.On("Get", mock.Anything, mock.AnythingOfType("string")).Return(nil, assert.AnError)
	mockCache.On("Set", mock.Anything, mock.AnythingOfType("string"), mock.Anything, mock.AnythingOfType("time.Duration")).Return(nil)

	auditLogger := NewAuditLogger(mockLogger, mockMetrics, mockCache)

	t.Run("LogSecurityViolation", func(t *testing.T) {
		details := map[string]any{
			"sql":     "SELECT * FROM users WHERE 1=1",
			"pattern": "or 1=1",
		}
		auditLogger.LogSecurityViolation(context.Background(), "123", "testuser", "sql_injection", "检测到SQL注入尝试", "192.168.1.1", details)

		mockLogger.AssertCalled(t, "Info", "审计事件", mock.Anything)
		mockMetrics.AssertCalled(t, "IncrementCounter", "audit_events_total", mock.AnythingOfType("map[string]string"))
		mockMetrics.AssertCalled(t, "IncrementCounter", "security_violations_total", mock.AnythingOfType("map[string]string"))
	})
}

func TestAuditLogger_GetRecentEvents(t *testing.T) {
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsCollector{}
	mockCache := &MockCacheManager{}

	auditLogger := NewAuditLogger(mockLogger, mockMetrics, mockCache)

	t.Run("GetRecentEvents_FromCache", func(t *testing.T) {
		events := []*AuditEvent{
			{
				ID:        "event1",
				EventType: AuditEventTypeLogin,
				UserID:    "123",
				Username:  "testuser",
			},
			{
				ID:        "event2",
				EventType: AuditEventTypeDataAccess,
				UserID:    "123",
				Username:  "testuser",
			},
		}

		mockCache.On("Get", mock.Anything, "recent_audit_events:123").Return(events, nil)

		result, err := auditLogger.GetRecentEvents(context.Background(), "123", 10)
		assert.NoError(t, err)
		assert.Equal(t, events, result)
	})

	t.Run("GetRecentEvents_NotInCache", func(t *testing.T) {
		mockCache.On("Get", mock.Anything, "recent_audit_events:456").Return(nil, assert.AnError)

		result, err := auditLogger.GetRecentEvents(context.Background(), "456", 10)
		assert.NoError(t, err)
		assert.Empty(t, result)
	})
}

func TestAuditLogger_GenerateEventID(t *testing.T) {
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsCollector{}
	mockCache := &MockCacheManager{}

	auditLogger := NewAuditLogger(mockLogger, mockMetrics, mockCache)

	t.Run("GenerateEventID", func(t *testing.T) {
		id1 := auditLogger.generateEventID()
		time.Sleep(1 * time.Microsecond) // 确保时间戳不同
		id2 := auditLogger.generateEventID()

		assert.NotEmpty(t, id1)
		assert.NotEmpty(t, id2)
		assert.NotEqual(t, id1, id2)
		assert.Contains(t, id1, "audit_")
		assert.Contains(t, id2, "audit_")
	})
}

func TestAuditLogger_ExportEvents(t *testing.T) {
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsCollector{}
	mockCache := &MockCacheManager{}

	auditLogger := NewAuditLogger(mockLogger, mockMetrics, mockCache)

	t.Run("ExportEvents", func(t *testing.T) {
		startTime := time.Now().Add(-24 * time.Hour)
		endTime := time.Now()
		eventTypes := []string{AuditEventTypeLogin, AuditEventTypeDataAccess}

		data, err := auditLogger.ExportEvents(context.Background(), startTime, endTime, eventTypes)
		assert.NoError(t, err)
		assert.NotEmpty(t, data)

		// 验证返回的是有效的JSON
		assert.Contains(t, string(data), "export_time")
		assert.Contains(t, string(data), "start_time")
		assert.Contains(t, string(data), "end_time")
		assert.Contains(t, string(data), "event_types")
	})
}

func TestAuditLogger_GetAuditStatistics(t *testing.T) {
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsCollector{}
	mockCache := &MockCacheManager{}

	auditLogger := NewAuditLogger(mockLogger, mockMetrics, mockCache)

	t.Run("GetAuditStatistics", func(t *testing.T) {
		startTime := time.Now().Add(-24 * time.Hour)
		endTime := time.Now()

		stats, err := auditLogger.GetAuditStatistics(context.Background(), startTime, endTime)
		assert.NoError(t, err)
		assert.NotNil(t, stats)

		// 验证统计信息包含必要的字段
		assert.Contains(t, stats, "total_events")
		assert.Contains(t, stats, "success_events")
		assert.Contains(t, stats, "failure_events")
		assert.Contains(t, stats, "security_violations")
		assert.Contains(t, stats, "unique_users")
	})
}
