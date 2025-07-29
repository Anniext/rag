// 本文件实现了审计日志功能，记录和追踪用户的操作行为。
// 主要功能：
// 1. 记录用户操作日志
// 2. 数据访问审计
// 3. 安全事件追踪
// 4. 审计日志查询和分析

package security

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"pumppill/rag/core"
)

// AuditLogger 审计日志记录器
type AuditLogger struct {
	logger  core.Logger           // 日志记录器
	metrics core.MetricsCollector // 指标收集器
	cache   core.CacheManager     // 缓存管理器
}

// AuditEvent 审计事件结构体
type AuditEvent struct {
	ID        string         `json:"id"`         // 事件ID
	EventType string         `json:"event_type"` // 事件类型
	UserID    string         `json:"user_id"`    // 用户ID
	Username  string         `json:"username"`   // 用户名
	Resource  string         `json:"resource"`   // 资源
	Action    string         `json:"action"`     // 操作
	Result    string         `json:"result"`     // 结果 (success/failure)
	Details   map[string]any `json:"details"`    // 详细信息
	IPAddress string         `json:"ip_address"` // IP地址
	UserAgent string         `json:"user_agent"` // 用户代理
	Timestamp time.Time      `json:"timestamp"`  // 时间戳
	Duration  time.Duration  `json:"duration"`   // 执行时长
	Error     string         `json:"error"`      // 错误信息
}

// AuditEventType 审计事件类型常量
const (
	AuditEventTypeLogin             = "login"              // 登录
	AuditEventTypeLogout            = "logout"             // 登出
	AuditEventTypeTokenValidate     = "token_validate"     // Token验证
	AuditEventTypePermissionCheck   = "permission_check"   // 权限检查
	AuditEventTypeDataAccess        = "data_access"        // 数据访问
	AuditEventTypeQueryExecute      = "query_execute"      // 查询执行
	AuditEventTypeSQLValidate       = "sql_validate"       // SQL验证
	AuditEventTypeSchemaAccess      = "schema_access"      // Schema访问
	AuditEventTypeSessionCreate     = "session_create"     // 会话创建
	AuditEventTypeSessionDestroy    = "session_destroy"    // 会话销毁
	AuditEventTypeSecurityViolation = "security_violation" // 安全违规
)

// AuditResult 审计结果常量
const (
	AuditResultSuccess = "success" // 成功
	AuditResultFailure = "failure" // 失败
	AuditResultDenied  = "denied"  // 拒绝
)

// NewAuditLogger 创建审计日志记录器
func NewAuditLogger(logger core.Logger, metrics core.MetricsCollector, cache core.CacheManager) *AuditLogger {
	return &AuditLogger{
		logger:  logger,
		metrics: metrics,
		cache:   cache,
	}
}

// LogEvent 记录审计事件
func (al *AuditLogger) LogEvent(ctx context.Context, event *AuditEvent) {
	// 设置事件ID和时间戳
	if event.ID == "" {
		event.ID = al.generateEventID()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// 记录到日志
	al.logger.Info("审计事件",
		"event_id", event.ID,
		"event_type", event.EventType,
		"user_id", event.UserID,
		"username", event.Username,
		"resource", event.Resource,
		"action", event.Action,
		"result", event.Result,
		"ip_address", event.IPAddress,
		"timestamp", event.Timestamp.Format(time.RFC3339),
		"duration", event.Duration.String(),
		"details", event.Details,
		"error", event.Error)

	// 更新指标
	al.metrics.IncrementCounter("audit_events_total",
		map[string]string{
			"event_type": event.EventType,
			"result":     event.Result,
		})

	if event.Duration > 0 {
		al.metrics.RecordHistogram("audit_event_duration",
			float64(event.Duration.Milliseconds()),
			map[string]string{
				"event_type": event.EventType,
			})
	}

	// 缓存最近的审计事件（用于快速查询）
	al.cacheRecentEvent(ctx, event)

	// 检查是否为安全违规事件
	if event.EventType == AuditEventTypeSecurityViolation || event.Result == AuditResultDenied {
		al.handleSecurityViolation(ctx, event)
	}
}

// LogLogin 记录登录事件
func (al *AuditLogger) LogLogin(ctx context.Context, userID, username, ipAddress, userAgent string, success bool, err error) {
	result := AuditResultSuccess
	errorMsg := ""
	if !success {
		result = AuditResultFailure
		if err != nil {
			errorMsg = err.Error()
		}
	}

	event := &AuditEvent{
		EventType: AuditEventTypeLogin,
		UserID:    userID,
		Username:  username,
		Resource:  "auth",
		Action:    "login",
		Result:    result,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Error:     errorMsg,
	}

	al.LogEvent(ctx, event)
}

// LogTokenValidation 记录Token验证事件
func (al *AuditLogger) LogTokenValidation(ctx context.Context, userID, username, ipAddress string, success bool, err error) {
	result := AuditResultSuccess
	errorMsg := ""
	if !success {
		result = AuditResultFailure
		if err != nil {
			errorMsg = err.Error()
		}
	}

	event := &AuditEvent{
		EventType: AuditEventTypeTokenValidate,
		UserID:    userID,
		Username:  username,
		Resource:  "auth",
		Action:    "validate_token",
		Result:    result,
		IPAddress: ipAddress,
		Error:     errorMsg,
	}

	al.LogEvent(ctx, event)
}

// LogPermissionCheck 记录权限检查事件
func (al *AuditLogger) LogPermissionCheck(ctx context.Context, userID, username, resource, action, ipAddress string, success bool, err error) {
	result := AuditResultSuccess
	errorMsg := ""
	if !success {
		result = AuditResultDenied
		if err != nil {
			errorMsg = err.Error()
		}
	}

	event := &AuditEvent{
		EventType: AuditEventTypePermissionCheck,
		UserID:    userID,
		Username:  username,
		Resource:  resource,
		Action:    action,
		Result:    result,
		IPAddress: ipAddress,
		Error:     errorMsg,
	}

	al.LogEvent(ctx, event)
}

// LogDataAccess 记录数据访问事件
func (al *AuditLogger) LogDataAccess(ctx context.Context, userID, username, tableName, operation, ipAddress string, rowCount int, duration time.Duration) {
	event := &AuditEvent{
		EventType: AuditEventTypeDataAccess,
		UserID:    userID,
		Username:  username,
		Resource:  fmt.Sprintf("table:%s", tableName),
		Action:    operation,
		Result:    AuditResultSuccess,
		IPAddress: ipAddress,
		Duration:  duration,
		Details: map[string]any{
			"table_name": tableName,
			"row_count":  rowCount,
		},
	}

	al.LogEvent(ctx, event)
}

// LogQueryExecution 记录查询执行事件
func (al *AuditLogger) LogQueryExecution(ctx context.Context, userID, username, sql, ipAddress string, success bool, duration time.Duration, rowCount int, err error) {
	result := AuditResultSuccess
	errorMsg := ""
	if !success {
		result = AuditResultFailure
		if err != nil {
			errorMsg = err.Error()
		}
	}

	event := &AuditEvent{
		EventType: AuditEventTypeQueryExecute,
		UserID:    userID,
		Username:  username,
		Resource:  "database",
		Action:    "execute_query",
		Result:    result,
		IPAddress: ipAddress,
		Duration:  duration,
		Error:     errorMsg,
		Details: map[string]any{
			"sql":       sql,
			"row_count": rowCount,
		},
	}

	al.LogEvent(ctx, event)
}

// LogSQLValidation 记录SQL验证事件
func (al *AuditLogger) LogSQLValidation(ctx context.Context, userID, username, sql, ipAddress string, success bool, err error) {
	result := AuditResultSuccess
	errorMsg := ""
	if !success {
		result = AuditResultFailure
		if err != nil {
			errorMsg = err.Error()
		}
	}

	event := &AuditEvent{
		EventType: AuditEventTypeSQLValidate,
		UserID:    userID,
		Username:  username,
		Resource:  "sql_guard",
		Action:    "validate_sql",
		Result:    result,
		IPAddress: ipAddress,
		Error:     errorMsg,
		Details: map[string]any{
			"sql": sql,
		},
	}

	al.LogEvent(ctx, event)
}

// LogSecurityViolation 记录安全违规事件
func (al *AuditLogger) LogSecurityViolation(ctx context.Context, userID, username, violationType, description, ipAddress string, details map[string]any) {
	event := &AuditEvent{
		EventType: AuditEventTypeSecurityViolation,
		UserID:    userID,
		Username:  username,
		Resource:  "security",
		Action:    violationType,
		Result:    AuditResultDenied,
		IPAddress: ipAddress,
		Error:     description,
		Details:   details,
	}

	al.LogEvent(ctx, event)
}

// GetRecentEvents 获取最近的审计事件
func (al *AuditLogger) GetRecentEvents(ctx context.Context, userID string, limit int) ([]*AuditEvent, error) {
	cacheKey := fmt.Sprintf("recent_audit_events:%s", userID)

	if cached, err := al.cache.Get(ctx, cacheKey); err == nil {
		if events, ok := cached.([]*AuditEvent); ok {
			if len(events) > limit {
				return events[:limit], nil
			}
			return events, nil
		}
	}

	// 如果缓存中没有，返回空列表
	// 实际实现中应该从持久化存储中查询
	return []*AuditEvent{}, nil
}

// GetEventsByType 根据事件类型获取审计事件
func (al *AuditLogger) GetEventsByType(ctx context.Context, eventType string, limit int) ([]*AuditEvent, error) {
	cacheKey := fmt.Sprintf("audit_events_by_type:%s", eventType)

	if cached, err := al.cache.Get(ctx, cacheKey); err == nil {
		if events, ok := cached.([]*AuditEvent); ok {
			if len(events) > limit {
				return events[:limit], nil
			}
			return events, nil
		}
	}

	// 如果缓存中没有，返回空列表
	return []*AuditEvent{}, nil
}

// generateEventID 生成事件ID
func (al *AuditLogger) generateEventID() string {
	now := time.Now()
	return fmt.Sprintf("audit_%d_%d", now.UnixNano(), now.Nanosecond()%10000)
}

// cacheRecentEvent 缓存最近的事件
func (al *AuditLogger) cacheRecentEvent(ctx context.Context, event *AuditEvent) {
	// 缓存用户最近的事件
	userCacheKey := fmt.Sprintf("recent_audit_events:%s", event.UserID)
	if cached, err := al.cache.Get(ctx, userCacheKey); err == nil {
		if events, ok := cached.([]*AuditEvent); ok {
			// 添加新事件到列表开头
			events = append([]*AuditEvent{event}, events...)
			// 保持最多100个事件
			if len(events) > 100 {
				events = events[:100]
			}
			al.cache.Set(ctx, userCacheKey, events, time.Hour)
		}
	} else {
		// 创建新的事件列表
		events := []*AuditEvent{event}
		al.cache.Set(ctx, userCacheKey, events, time.Hour)
	}

	// 缓存按事件类型分类的事件
	typeCacheKey := fmt.Sprintf("audit_events_by_type:%s", event.EventType)
	if cached, err := al.cache.Get(ctx, typeCacheKey); err == nil {
		if events, ok := cached.([]*AuditEvent); ok {
			events = append([]*AuditEvent{event}, events...)
			if len(events) > 50 {
				events = events[:50]
			}
			al.cache.Set(ctx, typeCacheKey, events, 30*time.Minute)
		}
	} else {
		events := []*AuditEvent{event}
		al.cache.Set(ctx, typeCacheKey, events, 30*time.Minute)
	}
}

// handleSecurityViolation 处理安全违规事件
func (al *AuditLogger) handleSecurityViolation(ctx context.Context, event *AuditEvent) {
	// 记录安全违规指标
	al.metrics.IncrementCounter("security_violations_total",
		map[string]string{
			"user_id":    event.UserID,
			"event_type": event.EventType,
			"action":     event.Action,
		})

	// 检查是否需要触发告警
	al.checkSecurityAlert(ctx, event)
}

// checkSecurityAlert 检查是否需要触发安全告警
func (al *AuditLogger) checkSecurityAlert(ctx context.Context, event *AuditEvent) {
	// 检查用户在短时间内的违规次数
	cacheKey := fmt.Sprintf("security_violations:%s", event.UserID)

	var violationCount int
	if cached, err := al.cache.Get(ctx, cacheKey); err == nil {
		if count, ok := cached.(int); ok {
			violationCount = count
		}
	}

	violationCount++
	al.cache.Set(ctx, cacheKey, violationCount, 15*time.Minute)

	// 如果违规次数超过阈值，记录高级别告警
	if violationCount >= 5 {
		al.logger.Error("检测到频繁安全违规",
			"user_id", event.UserID,
			"username", event.Username,
			"violation_count", violationCount,
			"event_type", event.EventType,
			"ip_address", event.IPAddress)

		// 这里可以集成告警系统，发送通知
		al.metrics.IncrementCounter("security_alerts_total",
			map[string]string{
				"user_id": event.UserID,
				"type":    "frequent_violations",
			})
	}
}

// ExportEvents 导出审计事件（用于合规性报告）
func (al *AuditLogger) ExportEvents(ctx context.Context, startTime, endTime time.Time, eventTypes []string) ([]byte, error) {
	// 这里应该实现从持久化存储中查询和导出事件的逻辑
	// 简化实现：返回JSON格式的事件数据

	exportData := map[string]any{
		"export_time": time.Now().Format(time.RFC3339),
		"start_time":  startTime.Format(time.RFC3339),
		"end_time":    endTime.Format(time.RFC3339),
		"event_types": eventTypes,
		"events":      []*AuditEvent{}, // 实际应该查询数据库
	}

	return json.Marshal(exportData)
}

// GetAuditStatistics 获取审计统计信息
func (al *AuditLogger) GetAuditStatistics(ctx context.Context, startTime, endTime time.Time) (map[string]any, error) {
	// 这里应该实现统计查询逻辑
	// 简化实现：返回基本统计信息

	stats := map[string]any{
		"total_events":        0,
		"success_events":      0,
		"failure_events":      0,
		"security_violations": 0,
		"unique_users":        0,
		"top_resources":       []string{},
		"top_actions":         []string{},
	}

	return stats, nil
}
