// 本文件实现了安全管理器，集成认证授权、SQL 防护和数据访问控制功能。
// 主要功能：
// 1. 统一的安全管理接口
// 2. 集成各个安全组件
// 3. 提供安全策略配置
// 4. 安全事件监控和告警

package security

import (
	"context"
	"fmt"
	"github.com/Anniext/rag/core"
	"strings"
	"time"
)

// Manager 安全管理器，实现 SecurityManager 接口
type Manager struct {
	authManager      *AuthManager          // 认证授权管理器
	sqlGuard         *SQLGuard             // SQL 安全防护器
	accessController *AccessController     // 数据访问控制器
	auditLogger      *AuditLogger          // 审计日志记录器
	logger           core.Logger           // 日志记录器
	metrics          core.MetricsCollector // 指标收集器
	config           *core.SecurityConfig  // 安全配置
}

// NewManager 创建安全管理器
func NewManager(config *core.SecurityConfig, cache core.CacheManager, logger core.Logger, metrics core.MetricsCollector) *Manager {
	return &Manager{
		authManager:      NewAuthManager(config, cache, logger, metrics),
		sqlGuard:         NewSQLGuard(logger, metrics),
		accessController: NewAccessController(logger, metrics),
		auditLogger:      NewAuditLogger(logger, metrics, cache),
		logger:           logger,
		metrics:          metrics,
		config:           config,
	}
}

// ValidateToken 验证 JWT Token 并返回用户信息
func (m *Manager) ValidateToken(ctx context.Context, token string) (*core.UserInfo, error) {
	// 记录安全操作
	startTime := time.Now()
	defer func() {
		m.metrics.RecordHistogram("security_manager_validate_token_duration",
			float64(time.Since(startTime).Milliseconds()),
			map[string]string{"operation": "validate_token"})
	}()

	userInfo, err := m.authManager.ValidateToken(ctx, token)

	// 记录审计日志
	if userInfo != nil {
		m.auditLogger.LogTokenValidation(ctx, userInfo.ID, userInfo.Username, m.getIPFromContext(ctx), err == nil, err)
	} else {
		m.auditLogger.LogTokenValidation(ctx, "", "", m.getIPFromContext(ctx), false, err)
	}

	if err != nil {
		m.logger.Warn("Token 验证失败", "error", err.Error())
		m.metrics.IncrementCounter("security_manager_token_validation_errors",
			map[string]string{"error": "validation_failed"})
		return nil, err
	}

	m.logger.Debug("Token 验证成功", "user_id", userInfo.ID, "username", userInfo.Username)
	return userInfo, nil
}

// CheckPermission 检查用户权限
func (m *Manager) CheckPermission(ctx context.Context, user *core.UserInfo, resource string, action string) error {
	// 记录权限检查
	startTime := time.Now()
	defer func() {
		m.metrics.RecordHistogram("security_manager_check_permission_duration",
			float64(time.Since(startTime).Milliseconds()),
			map[string]string{"resource": resource, "action": action})
	}()

	err := m.authManager.CheckPermission(ctx, user, resource, action)

	// 记录审计日志
	var userID, username string
	if user != nil {
		userID = user.ID
		username = user.Username
	}
	m.auditLogger.LogPermissionCheck(ctx, userID, username, resource, action, m.getIPFromContext(ctx), err == nil, err)

	if err != nil {
		m.logger.Warn("权限检查失败",
			"user_id", user.ID,
			"username", user.Username,
			"resource", resource,
			"action", action,
			"error", err.Error())
		m.metrics.IncrementCounter("security_manager_permission_check_errors",
			map[string]string{"resource": resource, "action": action, "error": "permission_denied"})
		return err
	}

	m.logger.Debug("权限检查通过",
		"user_id", user.ID,
		"username", user.Username,
		"resource", resource,
		"action", action)
	return nil
}

// ValidateSQL 验证 SQL 语句安全性
func (m *Manager) ValidateSQL(ctx context.Context, sql string) error {
	// 记录 SQL 验证
	startTime := time.Now()
	defer func() {
		m.metrics.RecordHistogram("security_manager_validate_sql_duration",
			float64(time.Since(startTime).Milliseconds()),
			map[string]string{"operation": "validate_sql"})
	}()

	err := m.sqlGuard.ValidateSQL(ctx, sql)

	// 从上下文中获取用户信息（如果有的话）
	var userID, username string
	if user := m.getUserFromContext(ctx); user != nil {
		userID = user.ID
		username = user.Username
	}

	// 记录审计日志
	m.auditLogger.LogSQLValidation(ctx, userID, username, sql, m.getIPFromContext(ctx), err == nil, err)

	// 如果验证失败且是安全违规，记录安全违规事件
	if err != nil {
		m.logger.Warn("SQL 安全验证失败", "sql", sql, "error", err.Error())
		m.metrics.IncrementCounter("security_manager_sql_validation_errors",
			map[string]string{"error": "validation_failed"})

		// 记录安全违规
		if userID != "" {
			details := map[string]any{
				"sql":   sql,
				"error": err.Error(),
			}
			m.auditLogger.LogSecurityViolation(ctx, userID, username, "sql_validation_failed", err.Error(), m.getIPFromContext(ctx), details)
		}

		return err
	}

	m.logger.Debug("SQL 安全验证通过", "sql", sql)
	return nil
}

// ValidateQueryAccess 验证查询访问权限（综合检查）
func (m *Manager) ValidateQueryAccess(ctx context.Context, user *core.UserInfo, sql string, tables []string) error {
	// 记录查询访问验证
	startTime := time.Now()
	defer func() {
		m.metrics.RecordHistogram("security_manager_validate_query_access_duration",
			float64(time.Since(startTime).Milliseconds()),
			map[string]string{"operation": "validate_query_access"})
	}()

	// 1. 验证 SQL 安全性
	if err := m.ValidateSQL(ctx, sql); err != nil {
		return fmt.Errorf("SQL 安全验证失败: %w", err)
	}

	// 2. 检查表访问权限
	for _, table := range tables {
		if err := m.accessController.CheckTableAccess(ctx, user, table, "SELECT"); err != nil {
			return fmt.Errorf("表访问权限检查失败: %w", err)
		}
	}

	// 3. 检查查询权限
	if err := m.sqlGuard.CheckQueryPermission(ctx, user, sql, tables); err != nil {
		return fmt.Errorf("查询权限检查失败: %w", err)
	}

	// 4. 检查 RAG 查询权限
	if err := m.CheckPermission(ctx, user, "rag", "query"); err != nil {
		return fmt.Errorf("RAG 查询权限检查失败: %w", err)
	}

	m.logger.Info("查询访问验证通过",
		"user_id", user.ID,
		"username", user.Username,
		"tables", strings.Join(tables, ","))

	return nil
}

// FilterQueryResult 过滤查询结果
func (m *Manager) FilterQueryResult(ctx context.Context, user *core.UserInfo, tableName string, data []map[string]any) ([]map[string]any, error) {
	// 记录结果过滤
	startTime := time.Now()
	originalCount := len(data)
	defer func() {
		m.metrics.RecordHistogram("security_manager_filter_result_duration",
			float64(time.Since(startTime).Milliseconds()),
			map[string]string{"table": tableName})
	}()

	filteredData, err := m.accessController.FilterData(ctx, user, tableName, data)
	if err != nil {
		m.logger.Error("查询结果过滤失败", "error", err.Error())
		return data, err
	}

	filteredCount := len(filteredData)
	m.logger.Debug("查询结果过滤完成",
		"user_id", user.ID,
		"table", tableName,
		"original_count", originalCount,
		"filtered_count", filteredCount)

	// 记录过滤统计
	m.metrics.SetGauge("security_manager_filtered_rows",
		float64(originalCount-filteredCount),
		map[string]string{"table": tableName, "user_id": user.ID})

	return filteredData, nil
}

// LogSecurityEvent 记录安全事件
func (m *Manager) LogSecurityEvent(ctx context.Context, eventType string, user *core.UserInfo, details map[string]interface{}) {
	event := map[string]interface{}{
		"event_type": eventType,
		"timestamp":  time.Now().Format(time.RFC3339),
		"details":    details,
	}

	if user != nil {
		event["user_id"] = user.ID
		event["username"] = user.Username
		event["user_roles"] = user.Roles
	}

	m.logger.Info("安全事件记录", "event", event)

	// 记录安全事件指标
	m.metrics.IncrementCounter("security_events",
		map[string]string{"event_type": eventType})
}

// CheckAPIAccess 检查 API 访问权限
func (m *Manager) CheckAPIAccess(ctx context.Context, user *core.UserInfo, apiPath string, method string) error {
	return m.authManager.ValidateAPIAccess(ctx, user, apiPath, method)
}

// IsAdmin 检查用户是否为管理员
func (m *Manager) IsAdmin(user *core.UserInfo) bool {
	return m.authManager.IsAdmin(user)
}

// HasRole 检查用户是否具有指定角色
func (m *Manager) HasRole(user *core.UserInfo, role string) bool {
	return m.authManager.HasRole(user, role)
}

// HasPermission 检查用户是否具有指定权限
func (m *Manager) HasPermission(user *core.UserInfo, permission string) bool {
	return m.authManager.HasPermission(user, permission)
}

// RefreshUserCache 刷新用户缓存
func (m *Manager) RefreshUserCache(ctx context.Context, userID string) error {
	return m.authManager.RefreshUserCache(ctx, userID)
}

// GetSecurityConfig 获取安全配置
func (m *Manager) GetSecurityConfig() *core.SecurityConfig {
	return m.config
}

// UpdateSecurityConfig 更新安全配置
func (m *Manager) UpdateSecurityConfig(config *core.SecurityConfig) {
	m.config = config
	// 这里可以更新各个组件的配置
}

// GetSecurityMetrics 获取安全指标
func (m *Manager) GetSecurityMetrics(ctx context.Context) map[string]interface{} {
	return map[string]interface{}{
		"auth_enabled":           true,
		"rbac_enabled":           m.config.EnableRBAC,
		"sql_guard_enabled":      true,
		"access_control_enabled": true,
		"token_expiry":           m.config.TokenExpiry.String(),
	}
}

// ValidateSessionAccess 验证会话访问权限
func (m *Manager) ValidateSessionAccess(ctx context.Context, user *core.UserInfo, sessionID string) error {
	// 检查用户是否有权限访问指定会话
	if err := m.CheckPermission(ctx, user, "session", "access"); err != nil {
		return err
	}

	// 这里可以添加更多的会话访问控制逻辑
	// 例如：检查会话是否属于当前用户

	return nil
}

// SanitizeSQL 清理 SQL 语句
func (m *Manager) SanitizeSQL(sql string) string {
	return m.sqlGuard.SanitizeSQL(sql)
}

// AddTablePermission 添加表权限配置
func (m *Manager) AddTablePermission(permission TablePermission) {
	m.accessController.AddTablePermission(permission)
}

// RemoveTablePermission 移除表权限配置
func (m *Manager) RemoveTablePermission(tableName string) {
	m.accessController.RemoveTablePermission(tableName)
}

// GetTablePermissions 获取表权限配置
func (m *Manager) GetTablePermissions() map[string]TablePermission {
	return m.accessController.GetTablePermissions()
}

// SetSQLGuardConfig 设置 SQL 防护配置
func (m *Manager) SetSQLGuardConfig(maxQueryLength int, strictMode bool, allowedOperations []string) {
	m.sqlGuard.SetMaxQueryLength(maxQueryLength)
	m.sqlGuard.SetStrictMode(strictMode)

	// 清空现有允许的操作
	m.sqlGuard.allowedOperations = []string{}

	// 添加新的允许操作
	for _, op := range allowedOperations {
		m.sqlGuard.AddAllowedOperation(op)
	}
}

// CheckFieldAccess 检查字段访问权限
func (m *Manager) CheckFieldAccess(ctx context.Context, user *core.UserInfo, tableName string, fieldName string) bool {
	return m.accessController.CheckFieldAccess(ctx, user, tableName, fieldName)
}

// LogDataAccess 记录数据访问日志
func (m *Manager) LogDataAccess(ctx context.Context, user *core.UserInfo, tableName string, operation string, rowCount int) {
	m.accessController.LogDataAccess(ctx, user, tableName, operation, rowCount)

	// 同时记录到审计日志
	if user != nil {
		m.auditLogger.LogDataAccess(ctx, user.ID, user.Username, tableName, operation, m.getIPFromContext(ctx), rowCount, 0)
	}
}

// ValidateSchemaAccess 验证 Schema 访问权限
func (m *Manager) ValidateSchemaAccess(ctx context.Context, user *core.UserInfo, operation string) error {
	return m.CheckPermission(ctx, user, "schema", operation)
}

// ValidateMCPAccess 验证 MCP 访问权限
func (m *Manager) ValidateMCPAccess(ctx context.Context, user *core.UserInfo, method string) error {
	return m.CheckPermission(ctx, user, "mcp", method)
}

// getIPFromContext 从上下文中获取IP地址
func (m *Manager) getIPFromContext(ctx context.Context) string {
	// 尝试从上下文中获取IP地址
	if ip, ok := ctx.Value("client_ip").(string); ok {
		return ip
	}
	if ip, ok := ctx.Value("remote_addr").(string); ok {
		return ip
	}
	return "unknown"
}

// getUserFromContext 从上下文中获取用户信息
func (m *Manager) getUserFromContext(ctx context.Context) *core.UserInfo {
	if user, ok := ctx.Value("user").(*core.UserInfo); ok {
		return user
	}
	return nil
}

// GetAuditLogger 获取审计日志记录器
func (m *Manager) GetAuditLogger() *AuditLogger {
	return m.auditLogger
}

// LogQueryExecution 记录查询执行日志
func (m *Manager) LogQueryExecution(ctx context.Context, user *core.UserInfo, sql string, success bool, duration time.Duration, rowCount int, err error) {
	if user != nil {
		m.auditLogger.LogQueryExecution(ctx, user.ID, user.Username, sql, m.getIPFromContext(ctx), success, duration, rowCount, err)
	}
}

// LogSecurityViolation 记录安全违规事件
func (m *Manager) LogSecurityViolation(ctx context.Context, user *core.UserInfo, violationType, description string, details map[string]any) {
	var userID, username string
	if user != nil {
		userID = user.ID
		username = user.Username
	}
	m.auditLogger.LogSecurityViolation(ctx, userID, username, violationType, description, m.getIPFromContext(ctx), details)
}

// GetAuditEvents 获取审计事件
func (m *Manager) GetAuditEvents(ctx context.Context, userID string, limit int) ([]*AuditEvent, error) {
	return m.auditLogger.GetRecentEvents(ctx, userID, limit)
}

// ExportAuditEvents 导出审计事件
func (m *Manager) ExportAuditEvents(ctx context.Context, startTime, endTime time.Time, eventTypes []string) ([]byte, error) {
	return m.auditLogger.ExportEvents(ctx, startTime, endTime, eventTypes)
}

// GetAuditStatistics 获取审计统计信息
func (m *Manager) GetAuditStatistics(ctx context.Context, startTime, endTime time.Time) (map[string]any, error) {
	return m.auditLogger.GetAuditStatistics(ctx, startTime, endTime)
}
