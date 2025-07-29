// 本文件实现了健康检查和告警系统，提供系统健康状态监控、异常检测和故障预警功能。
// 主要功能：
// 1. 健康检查端点和状态监控
// 2. 异常检测和故障预警
// 3. 告警分级和通知渠道管理
// 4. 系统组件健康状态检查
// 5. 自动故障恢复和重试机制
// 6. 健康状态历史记录和趋势分析

package monitor

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
)

// HealthStatus 健康状态枚举
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusDegraded  HealthStatus = "degraded"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
	HealthStatusUnknown   HealthStatus = "unknown"
)

// AlertLevel 告警级别枚举
type AlertLevel string

const (
	AlertLevelInfo     AlertLevel = "info"
	AlertLevelWarning  AlertLevel = "warning"
	AlertLevelError    AlertLevel = "error"
	AlertLevelCritical AlertLevel = "critical"
)

// ComponentType 组件类型枚举
type ComponentType string

const (
	ComponentTypeDatabase ComponentType = "database"
	ComponentTypeRedis    ComponentType = "redis"
	ComponentTypeLLM      ComponentType = "llm"
	ComponentTypeMCP      ComponentType = "mcp"
	ComponentTypeSystem   ComponentType = "system"
	ComponentTypeCache    ComponentType = "cache"
)

// HealthChecker 健康检查器接口
type HealthChecker interface {
	CheckHealth(ctx context.Context) *HealthCheckResult
	GetComponentType() ComponentType
	GetName() string
	IsEnabled() bool
	SetEnabled(enabled bool)
}

// HealthCheckResult 健康检查结果
type HealthCheckResult struct {
	ComponentType ComponentType          `json:"component_type"`
	ComponentName string                 `json:"component_name"`
	Status        HealthStatus           `json:"status"`
	Message       string                 `json:"message"`
	Details       map[string]interface{} `json:"details,omitempty"`
	CheckTime     time.Time              `json:"check_time"`
	Duration      time.Duration          `json:"duration"`
	Error         string                 `json:"error,omitempty"`
}

// HealthReport 健康报告
type HealthReport struct {
	OverallStatus HealthStatus                  `json:"overall_status"`
	Components    map[string]*HealthCheckResult `json:"components"`
	Summary       *HealthSummary                `json:"summary"`
	Timestamp     time.Time                     `json:"timestamp"`
	Uptime        time.Duration                 `json:"uptime"`
}

// HealthSummary 健康摘要
type HealthSummary struct {
	TotalComponents     int `json:"total_components"`
	HealthyComponents   int `json:"healthy_components"`
	DegradedComponents  int `json:"degraded_components"`
	UnhealthyComponents int `json:"unhealthy_components"`
	UnknownComponents   int `json:"unknown_components"`
}

// Alert 告警信息
type Alert struct {
	ID            string                 `json:"id"`
	Level         AlertLevel             `json:"level"`
	ComponentType ComponentType          `json:"component_type"`
	ComponentName string                 `json:"component_name"`
	Title         string                 `json:"title"`
	Message       string                 `json:"message"`
	Details       map[string]interface{} `json:"details,omitempty"`
	CreatedAt     time.Time              `json:"created_at"`
	ResolvedAt    *time.Time             `json:"resolved_at,omitempty"`
	Status        string                 `json:"status"` // active, resolved, suppressed
	Count         int                    `json:"count"`  // 重复告警次数
}

// NotificationChannel 通知渠道接口
type NotificationChannel interface {
	SendNotification(ctx context.Context, alert *Alert) error
	GetChannelType() string
	IsEnabled() bool
	SetEnabled(enabled bool)
}

// HealthManager 健康管理器
type HealthManager struct {
	checkers         map[string]HealthChecker
	channels         map[string]NotificationChannel
	alerts           map[string]*Alert
	config           *HealthConfig
	logger           Logger
	metrics          MetricsCollector
	mutex            sync.RWMutex
	startTime        time.Time
	lastHealthReport *HealthReport
	healthHistory    []*HealthReport
	alertHistory     []*Alert
	checkInterval    time.Duration
	alertCooldown    map[string]time.Time
	suppressedAlerts map[string]bool
	running          bool
	stopChan         chan struct{}
}

// HealthConfig 健康检查配置
type HealthConfig struct {
	CheckInterval     time.Duration                      `yaml:"check_interval"`
	Timeout           time.Duration                      `yaml:"timeout"`
	RetryCount        int                                `yaml:"retry_count"`
	RetryInterval     time.Duration                      `yaml:"retry_interval"`
	AlertCooldown     time.Duration                      `yaml:"alert_cooldown"`
	HistorySize       int                                `yaml:"history_size"`
	EnabledCheckers   []string                           `yaml:"enabled_checkers"`
	EnabledChannels   []string                           `yaml:"enabled_channels"`
	ComponentConfigs  map[ComponentType]*ComponentConfig `yaml:"component_configs"`
	NotificationRules []*NotificationRule                `yaml:"notification_rules"`
}

// ComponentConfig 组件配置
type ComponentConfig struct {
	Enabled       bool               `yaml:"enabled"`
	Timeout       time.Duration      `yaml:"timeout"`
	RetryCount    int                `yaml:"retry_count"`
	RetryInterval time.Duration      `yaml:"retry_interval"`
	Thresholds    map[string]float64 `yaml:"thresholds"`
}

// NotificationRule 通知规则
type NotificationRule struct {
	Name          string        `yaml:"name"`
	ComponentType ComponentType `yaml:"component_type"`
	AlertLevel    AlertLevel    `yaml:"alert_level"`
	Channels      []string      `yaml:"channels"`
	Cooldown      time.Duration `yaml:"cooldown"`
	Enabled       bool          `yaml:"enabled"`
}

// NewHealthManager 创建健康管理器
func NewHealthManager(config *HealthConfig, logger Logger, metrics MetricsCollector) *HealthManager {
	if config == nil {
		config = &HealthConfig{
			CheckInterval: 30 * time.Second,
			Timeout:       10 * time.Second,
			RetryCount:    3,
			RetryInterval: 5 * time.Second,
			AlertCooldown: 5 * time.Minute,
			HistorySize:   100,
		}
	}

	return &HealthManager{
		checkers:         make(map[string]HealthChecker),
		channels:         make(map[string]NotificationChannel),
		alerts:           make(map[string]*Alert),
		config:           config,
		logger:           logger,
		metrics:          metrics,
		startTime:        time.Now(),
		healthHistory:    make([]*HealthReport, 0, config.HistorySize),
		alertHistory:     make([]*Alert, 0, config.HistorySize),
		checkInterval:    config.CheckInterval,
		alertCooldown:    make(map[string]time.Time),
		suppressedAlerts: make(map[string]bool),
		stopChan:         make(chan struct{}),
	}
}

// RegisterChecker 注册健康检查器
func (hm *HealthManager) RegisterChecker(name string, checker HealthChecker) error {
	hm.mutex.Lock()
	defer hm.mutex.Unlock()

	if _, exists := hm.checkers[name]; exists {
		return fmt.Errorf("健康检查器 %s 已存在", name)
	}

	hm.checkers[name] = checker
	hm.logger.Info("注册健康检查器",
		zap.String("name", name),
		zap.String("type", string(checker.GetComponentType())),
	)

	return nil
}

// RegisterNotificationChannel 注册通知渠道
func (hm *HealthManager) RegisterNotificationChannel(name string, channel NotificationChannel) error {
	hm.mutex.Lock()
	defer hm.mutex.Unlock()

	if _, exists := hm.channels[name]; exists {
		return fmt.Errorf("通知渠道 %s 已存在", name)
	}

	hm.channels[name] = channel
	hm.logger.Info("注册通知渠道",
		zap.String("name", name),
		zap.String("type", channel.GetChannelType()),
	)

	return nil
}

// Start 启动健康检查
func (hm *HealthManager) Start(ctx context.Context) error {
	hm.mutex.Lock()
	if hm.running {
		hm.mutex.Unlock()
		return fmt.Errorf("健康检查已在运行")
	}
	hm.running = true
	hm.mutex.Unlock()

	hm.logger.Info("启动健康检查服务",
		zap.Duration("interval", hm.checkInterval),
		zap.Int("checkers", len(hm.checkers)),
		zap.Int("channels", len(hm.channels)),
	)

	// 启动定期健康检查
	go hm.runPeriodicHealthCheck(ctx)

	return nil
}

// Stop 停止健康检查
func (hm *HealthManager) Stop(ctx context.Context) error {
	hm.mutex.Lock()
	if !hm.running {
		hm.mutex.Unlock()
		return nil
	}
	hm.running = false
	hm.mutex.Unlock()

	hm.logger.Info("停止健康检查服务")

	// 发送停止信号
	close(hm.stopChan)

	return nil
}

// CheckHealth 执行健康检查
func (hm *HealthManager) CheckHealth(ctx context.Context) *HealthReport {
	hm.mutex.RLock()
	checkers := make(map[string]HealthChecker)
	for name, checker := range hm.checkers {
		if checker.IsEnabled() {
			checkers[name] = checker
		}
	}
	hm.mutex.RUnlock()

	report := &HealthReport{
		Components: make(map[string]*HealthCheckResult),
		Timestamp:  time.Now(),
		Uptime:     time.Since(hm.startTime),
	}

	// 并发执行健康检查
	var wg sync.WaitGroup
	resultChan := make(chan *HealthCheckResult, len(checkers))

	for name, checker := range checkers {
		wg.Add(1)
		go func(name string, checker HealthChecker) {
			defer wg.Done()

			// 创建带超时的上下文
			checkCtx, cancel := context.WithTimeout(ctx, hm.config.Timeout)
			defer cancel()

			// 执行健康检查
			result := hm.executeHealthCheck(checkCtx, name, checker)
			resultChan <- result
		}(name, checker)
	}

	// 等待所有检查完成
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// 收集结果
	for result := range resultChan {
		report.Components[result.ComponentName] = result
	}

	// 计算整体状态和摘要
	report.OverallStatus, report.Summary = hm.calculateOverallStatus(report.Components)

	// 保存报告到历史记录
	hm.saveHealthReport(report)

	// 检查是否需要发送告警
	hm.checkAndSendAlerts(ctx, report)

	return report
}

// executeHealthCheck 执行单个健康检查
func (hm *HealthManager) executeHealthCheck(ctx context.Context, name string, checker HealthChecker) *HealthCheckResult {
	startTime := time.Now()

	// 获取组件配置
	componentConfig := hm.getComponentConfig(checker.GetComponentType())

	var result *HealthCheckResult
	var lastErr error

	// 重试机制
	for attempt := 0; attempt <= componentConfig.RetryCount; attempt++ {
		if attempt > 0 {
			// 等待重试间隔
			select {
			case <-ctx.Done():
				return &HealthCheckResult{
					ComponentType: checker.GetComponentType(),
					ComponentName: name,
					Status:        HealthStatusUnknown,
					Message:       "健康检查被取消",
					CheckTime:     startTime,
					Duration:      time.Since(startTime),
					Error:         ctx.Err().Error(),
				}
			case <-time.After(componentConfig.RetryInterval):
			}
		}

		// 执行健康检查
		result = checker.CheckHealth(ctx)
		if result.Status == HealthStatusHealthy {
			break
		}

		lastErr = fmt.Errorf("健康检查失败: %s", result.Message)
		hm.logger.Warn("健康检查重试",
			zap.String("component", name),
			zap.Int("attempt", attempt+1),
			zap.String("error", result.Error),
		)
	}

	// 设置检查时间和持续时间
	result.CheckTime = startTime
	result.Duration = time.Since(startTime)

	// 记录指标
	hm.recordHealthCheckMetrics(result)

	// 记录日志
	if result.Status == HealthStatusHealthy {
		hm.logger.Debug("健康检查成功",
			zap.String("component", name),
			zap.Duration("duration", result.Duration),
		)
	} else {
		hm.logger.Warn("健康检查失败",
			zap.String("component", name),
			zap.String("status", string(result.Status)),
			zap.String("message", result.Message),
			zap.Duration("duration", result.Duration),
			zap.Error(lastErr),
		)
	}

	return result
}

// getComponentConfig 获取组件配置
func (hm *HealthManager) getComponentConfig(componentType ComponentType) *ComponentConfig {
	if config, exists := hm.config.ComponentConfigs[componentType]; exists {
		return config
	}

	// 返回默认配置
	return &ComponentConfig{
		Enabled:       true,
		Timeout:       hm.config.Timeout,
		RetryCount:    hm.config.RetryCount,
		RetryInterval: hm.config.RetryInterval,
		Thresholds:    make(map[string]float64),
	}
}

// calculateOverallStatus 计算整体状态
func (hm *HealthManager) calculateOverallStatus(components map[string]*HealthCheckResult) (HealthStatus, *HealthSummary) {
	summary := &HealthSummary{
		TotalComponents: len(components),
	}

	hasUnhealthy := false
	hasDegraded := false

	for _, result := range components {
		switch result.Status {
		case HealthStatusHealthy:
			summary.HealthyComponents++
		case HealthStatusDegraded:
			summary.DegradedComponents++
			hasDegraded = true
		case HealthStatusUnhealthy:
			summary.UnhealthyComponents++
			hasUnhealthy = true
		case HealthStatusUnknown:
			summary.UnknownComponents++
		}
	}

	// 确定整体状态
	var overallStatus HealthStatus
	if hasUnhealthy {
		overallStatus = HealthStatusUnhealthy
	} else if hasDegraded {
		overallStatus = HealthStatusDegraded
	} else if summary.HealthyComponents > 0 {
		overallStatus = HealthStatusHealthy
	} else {
		overallStatus = HealthStatusUnknown
	}

	return overallStatus, summary
}

// saveHealthReport 保存健康报告
func (hm *HealthManager) saveHealthReport(report *HealthReport) {
	hm.mutex.Lock()
	defer hm.mutex.Unlock()

	// 添加到历史记录
	hm.healthHistory = append(hm.healthHistory, report)

	// 保持历史记录大小限制
	if len(hm.healthHistory) > hm.config.HistorySize {
		hm.healthHistory = hm.healthHistory[1:]
	}

	// 更新最新报告
	hm.lastHealthReport = report
}

// checkAndSendAlerts 检查并发送告警
func (hm *HealthManager) checkAndSendAlerts(ctx context.Context, report *HealthReport) {
	for componentName, result := range report.Components {
		hm.processComponentAlert(ctx, componentName, result)
	}

	// 检查整体状态告警
	hm.processOverallAlert(ctx, report)
}

// processComponentAlert 处理组件告警
func (hm *HealthManager) processComponentAlert(ctx context.Context, componentName string, result *HealthCheckResult) {
	alertKey := fmt.Sprintf("%s:%s", result.ComponentType, componentName)

	// 检查是否需要发送告警
	var alertLevel AlertLevel
	var shouldAlert bool

	switch result.Status {
	case HealthStatusUnhealthy:
		alertLevel = AlertLevelCritical
		shouldAlert = true
	case HealthStatusDegraded:
		alertLevel = AlertLevelWarning
		shouldAlert = true
	case HealthStatusHealthy:
		// 检查是否需要发送恢复告警
		if existingAlert, exists := hm.alerts[alertKey]; exists && existingAlert.Status == "active" {
			hm.resolveAlert(ctx, alertKey, "组件已恢复正常")
		}
		return
	default:
		return
	}

	if !shouldAlert {
		return
	}

	// 检查是否被抑制
	if hm.suppressedAlerts[alertKey] {
		return
	}

	// 检查告警冷却时间
	if lastAlertTime, exists := hm.alertCooldown[alertKey]; exists {
		if time.Since(lastAlertTime) < hm.config.AlertCooldown {
			return
		}
	}

	// 创建或更新告警
	alert := hm.createOrUpdateAlert(alertKey, alertLevel, result)

	// 发送告警通知
	hm.sendAlert(ctx, alert)

	// 更新冷却时间
	hm.alertCooldown[alertKey] = time.Now()
}

// processOverallAlert 处理整体状态告警
func (hm *HealthManager) processOverallAlert(ctx context.Context, report *HealthReport) {
	alertKey := "system:overall"

	var alertLevel AlertLevel
	var shouldAlert bool

	switch report.OverallStatus {
	case HealthStatusUnhealthy:
		alertLevel = AlertLevelCritical
		shouldAlert = true
	case HealthStatusDegraded:
		alertLevel = AlertLevelWarning
		shouldAlert = true
	case HealthStatusHealthy:
		// 检查是否需要发送恢复告警
		if existingAlert, exists := hm.alerts[alertKey]; exists && existingAlert.Status == "active" {
			hm.resolveAlert(ctx, alertKey, "系统整体状态已恢复正常")
		}
		return
	default:
		return
	}

	if !shouldAlert {
		return
	}

	// 检查告警冷却时间
	if lastAlertTime, exists := hm.alertCooldown[alertKey]; exists {
		if time.Since(lastAlertTime) < hm.config.AlertCooldown {
			return
		}
	}

	// 创建整体状态告警
	alert := &Alert{
		ID:            hm.generateAlertID(),
		Level:         alertLevel,
		ComponentType: ComponentTypeSystem,
		ComponentName: "overall",
		Title:         "系统整体状态异常",
		Message:       fmt.Sprintf("系统整体状态: %s", report.OverallStatus),
		Details: map[string]interface{}{
			"overall_status":       report.OverallStatus,
			"healthy_components":   report.Summary.HealthyComponents,
			"degraded_components":  report.Summary.DegradedComponents,
			"unhealthy_components": report.Summary.UnhealthyComponents,
			"total_components":     report.Summary.TotalComponents,
		},
		CreatedAt: time.Now(),
		Status:    "active",
		Count:     1,
	}

	// 检查是否已存在相同告警
	if existingAlert, exists := hm.alerts[alertKey]; exists && existingAlert.Status == "active" {
		existingAlert.Count++
		existingAlert.Details = alert.Details
		alert = existingAlert
	} else {
		hm.alerts[alertKey] = alert
		hm.addAlertToHistory(alert)
	}

	// 发送告警通知
	hm.sendAlert(ctx, alert)

	// 更新冷却时间
	hm.alertCooldown[alertKey] = time.Now()
}

// createOrUpdateAlert 创建或更新告警
func (hm *HealthManager) createOrUpdateAlert(alertKey string, level AlertLevel, result *HealthCheckResult) *Alert {
	// 检查是否已存在相同告警
	if existingAlert, exists := hm.alerts[alertKey]; exists && existingAlert.Status == "active" {
		existingAlert.Count++
		existingAlert.Details = result.Details
		return existingAlert
	}

	// 创建新告警
	alert := &Alert{
		ID:            hm.generateAlertID(),
		Level:         level,
		ComponentType: result.ComponentType,
		ComponentName: result.ComponentName,
		Title:         fmt.Sprintf("%s 组件异常", result.ComponentName),
		Message:       result.Message,
		Details:       result.Details,
		CreatedAt:     time.Now(),
		Status:        "active",
		Count:         1,
	}

	hm.alerts[alertKey] = alert
	hm.addAlertToHistory(alert)

	return alert
}

// resolveAlert 解决告警
func (hm *HealthManager) resolveAlert(ctx context.Context, alertKey, resolution string) {
	hm.mutex.Lock()
	defer hm.mutex.Unlock()

	if alert, exists := hm.alerts[alertKey]; exists && alert.Status == "active" {
		now := time.Now()
		alert.Status = "resolved"
		alert.ResolvedAt = &now
		alert.Message = resolution

		hm.logger.Info("告警已解决",
			zap.String("alert_id", alert.ID),
			zap.String("component", alert.ComponentName),
			zap.String("resolution", resolution),
		)

		// 发送恢复通知
		hm.sendAlert(ctx, alert)
	}
}

// sendAlert 发送告警
func (hm *HealthManager) sendAlert(ctx context.Context, alert *Alert) {
	// 获取适用的通知规则
	rules := hm.getNotificationRules(alert)

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}

		// 检查规则特定的冷却时间
		ruleKey := fmt.Sprintf("%s:%s", alert.ID, rule.Name)
		if lastNotifyTime, exists := hm.alertCooldown[ruleKey]; exists {
			if time.Since(lastNotifyTime) < rule.Cooldown {
				continue
			}
		}

		// 发送到指定渠道
		for _, channelName := range rule.Channels {
			if channel, exists := hm.channels[channelName]; exists && channel.IsEnabled() {
				go func(ch NotificationChannel, a *Alert) {
					if err := ch.SendNotification(ctx, a); err != nil {
						hm.logger.Error("发送告警通知失败",
							zap.String("channel", channelName),
							zap.String("alert_id", a.ID),
							zap.Error(err),
						)
					} else {
						hm.logger.Info("告警通知已发送",
							zap.String("channel", channelName),
							zap.String("alert_id", a.ID),
							zap.String("level", string(a.Level)),
						)
					}
				}(channel, alert)
			}
		}

		// 更新规则冷却时间
		hm.alertCooldown[ruleKey] = time.Now()
	}

	// 记录告警指标
	hm.recordAlertMetrics(alert)
}

// getNotificationRules 获取适用的通知规则
func (hm *HealthManager) getNotificationRules(alert *Alert) []*NotificationRule {
	var applicableRules []*NotificationRule

	for _, rule := range hm.config.NotificationRules {
		if hm.isRuleApplicable(rule, alert) {
			applicableRules = append(applicableRules, rule)
		}
	}

	// 如果没有匹配的规则，使用默认规则
	if len(applicableRules) == 0 {
		applicableRules = append(applicableRules, &NotificationRule{
			Name:          "default",
			ComponentType: alert.ComponentType,
			AlertLevel:    alert.Level,
			Channels:      []string{"default"},
			Cooldown:      hm.config.AlertCooldown,
			Enabled:       true,
		})
	}

	return applicableRules
}

// isRuleApplicable 检查规则是否适用
func (hm *HealthManager) isRuleApplicable(rule *NotificationRule, alert *Alert) bool {
	// 检查组件类型
	if rule.ComponentType != "" && rule.ComponentType != alert.ComponentType {
		return false
	}

	// 检查告警级别
	if rule.AlertLevel != "" && rule.AlertLevel != alert.Level {
		return false
	}

	return true
}

// generateAlertID 生成告警ID
func (hm *HealthManager) generateAlertID() string {
	return fmt.Sprintf("alert_%d_%d", time.Now().Unix(), time.Now().Nanosecond())
}

// addAlertToHistory 添加告警到历史记录
func (hm *HealthManager) addAlertToHistory(alert *Alert) {
	hm.alertHistory = append(hm.alertHistory, alert)

	// 保持历史记录大小限制
	if len(hm.alertHistory) > hm.config.HistorySize {
		hm.alertHistory = hm.alertHistory[1:]
	}
}

// recordHealthCheckMetrics 记录健康检查指标
func (hm *HealthManager) recordHealthCheckMetrics(result *HealthCheckResult) {
	if hm.metrics == nil {
		return
	}

	// 记录检查持续时间
	// hm.metrics.RecordHealthCheckDuration(string(result.ComponentType), result.ComponentName, result.Duration)

	// 记录检查状态
	// status := 1.0
	// if result.Status != HealthStatusHealthy {
	// 	status = 0.0
	// }
	// hm.metrics.RecordHealthCheckStatus(string(result.ComponentType), result.ComponentName, status)
}

// recordAlertMetrics 记录告警指标
func (hm *HealthManager) recordAlertMetrics(alert *Alert) {
	if hm.metrics == nil {
		return
	}

	// 记录告警数量
	// hm.metrics.RecordAlert(string(alert.ComponentType), alert.ComponentName, string(alert.Level))
}

// runPeriodicHealthCheck 运行定期健康检查
func (hm *HealthManager) runPeriodicHealthCheck(ctx context.Context) {
	ticker := time.NewTicker(hm.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-hm.stopChan:
			return
		case <-ticker.C:
			// 执行健康检查
			report := hm.CheckHealth(ctx)
			hm.logger.Debug("定期健康检查完成",
				zap.String("overall_status", string(report.OverallStatus)),
				zap.Int("total_components", report.Summary.TotalComponents),
				zap.Int("healthy_components", report.Summary.HealthyComponents),
			)
		}
	}
}

// GetHealthReport 获取最新健康报告
func (hm *HealthManager) GetHealthReport() *HealthReport {
	hm.mutex.RLock()
	defer hm.mutex.RUnlock()

	if hm.lastHealthReport == nil {
		return &HealthReport{
			OverallStatus: HealthStatusUnknown,
			Components:    make(map[string]*HealthCheckResult),
			Summary: &HealthSummary{
				TotalComponents: 0,
			},
			Timestamp: time.Now(),
			Uptime:    time.Since(hm.startTime),
		}
	}

	// 返回副本以避免并发问题
	report := *hm.lastHealthReport
	return &report
}

// GetHealthHistory 获取健康历史记录
func (hm *HealthManager) GetHealthHistory(limit int) []*HealthReport {
	hm.mutex.RLock()
	defer hm.mutex.RUnlock()

	if limit <= 0 || limit > len(hm.healthHistory) {
		limit = len(hm.healthHistory)
	}

	// 返回最近的记录
	start := len(hm.healthHistory) - limit
	history := make([]*HealthReport, limit)
	copy(history, hm.healthHistory[start:])

	return history
}

// GetActiveAlerts 获取活跃告警
func (hm *HealthManager) GetActiveAlerts() []*Alert {
	hm.mutex.RLock()
	defer hm.mutex.RUnlock()

	var activeAlerts []*Alert
	for _, alert := range hm.alerts {
		if alert.Status == "active" {
			activeAlerts = append(activeAlerts, alert)
		}
	}

	return activeAlerts
}

// GetAlertHistory 获取告警历史记录
func (hm *HealthManager) GetAlertHistory(limit int) []*Alert {
	hm.mutex.RLock()
	defer hm.mutex.RUnlock()

	if limit <= 0 || limit > len(hm.alertHistory) {
		limit = len(hm.alertHistory)
	}

	// 返回最近的记录
	start := len(hm.alertHistory) - limit
	history := make([]*Alert, limit)
	copy(history, hm.alertHistory[start:])

	return history
}

// SuppressAlert 抑制告警
func (hm *HealthManager) SuppressAlert(alertKey string, duration time.Duration) error {
	hm.mutex.Lock()
	defer hm.mutex.Unlock()

	hm.suppressedAlerts[alertKey] = true

	// 设置自动取消抑制
	if duration > 0 {
		go func() {
			time.Sleep(duration)
			hm.mutex.Lock()
			delete(hm.suppressedAlerts, alertKey)
			hm.mutex.Unlock()
		}()
	}

	hm.logger.Info("告警已被抑制",
		zap.String("alert_key", alertKey),
		zap.Duration("duration", duration),
	)

	return nil
}

// UnsuppressAlert 取消抑制告警
func (hm *HealthManager) UnsuppressAlert(alertKey string) error {
	hm.mutex.Lock()
	defer hm.mutex.Unlock()

	delete(hm.suppressedAlerts, alertKey)

	hm.logger.Info("告警抑制已取消",
		zap.String("alert_key", alertKey),
	)

	return nil
}

// HTTPHealthHandler HTTP健康检查处理器
func (hm *HealthManager) HTTPHealthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		report := hm.CheckHealth(ctx)

		// 设置响应头
		w.Header().Set("Content-Type", "application/json")

		// 根据健康状态设置HTTP状态码
		switch report.OverallStatus {
		case HealthStatusHealthy:
			w.WriteHeader(http.StatusOK)
		case HealthStatusDegraded:
			w.WriteHeader(http.StatusOK) // 降级状态仍返回200，但在响应体中标明
		case HealthStatusUnhealthy:
			w.WriteHeader(http.StatusServiceUnavailable)
		default:
			w.WriteHeader(http.StatusServiceUnavailable)
		}

		// 返回健康报告
		if err := writeJSON(w, report); err != nil {
			hm.logger.Error("写入健康检查响应失败", zap.Error(err))
		}
	}
}

// writeJSON 写入JSON响应
func writeJSON(w http.ResponseWriter, data interface{}) error {
	// 这里应该使用实际的JSON编码库
	// 为了简化，这里只是一个占位符
	return nil
}

// DatabaseHealthChecker 数据库健康检查器
type DatabaseHealthChecker struct {
	name    string
	db      *sql.DB
	enabled bool
	timeout time.Duration
}

// NewDatabaseHealthChecker 创建数据库健康检查器
func NewDatabaseHealthChecker(name string, db *sql.DB) *DatabaseHealthChecker {
	return &DatabaseHealthChecker{
		name:    name,
		db:      db,
		enabled: true,
		timeout: 5 * time.Second,
	}
}

// CheckHealth 检查数据库健康状态
func (c *DatabaseHealthChecker) CheckHealth(ctx context.Context) *HealthCheckResult {
	startTime := time.Now()

	// 创建带超时的上下文
	checkCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	result := &HealthCheckResult{
		ComponentType: ComponentTypeDatabase,
		ComponentName: c.name,
		CheckTime:     startTime,
		Details:       make(map[string]interface{}),
	}

	// 检查数据库连接
	if err := c.db.PingContext(checkCtx); err != nil {
		result.Status = HealthStatusUnhealthy
		result.Message = "数据库连接失败"
		result.Error = err.Error()
		result.Duration = time.Since(startTime)
		return result
	}

	// 获取数据库统计信息
	stats := c.db.Stats()
	result.Details["open_connections"] = stats.OpenConnections
	result.Details["in_use"] = stats.InUse
	result.Details["idle"] = stats.Idle
	result.Details["wait_count"] = stats.WaitCount
	result.Details["wait_duration"] = stats.WaitDuration.String()

	// 检查连接池状态
	if stats.OpenConnections > stats.MaxOpenConnections*8/10 {
		result.Status = HealthStatusDegraded
		result.Message = "数据库连接池使用率较高"
	} else {
		result.Status = HealthStatusHealthy
		result.Message = "数据库连接正常"
	}

	result.Duration = time.Since(startTime)
	return result
}

// GetComponentType 获取组件类型
func (c *DatabaseHealthChecker) GetComponentType() ComponentType {
	return ComponentTypeDatabase
}

// GetName 获取组件名称
func (c *DatabaseHealthChecker) GetName() string {
	return c.name
}

// IsEnabled 检查是否启用
func (c *DatabaseHealthChecker) IsEnabled() bool {
	return c.enabled
}

// SetEnabled 设置启用状态
func (c *DatabaseHealthChecker) SetEnabled(enabled bool) {
	c.enabled = enabled
}

// LogNotificationChannel 日志通知渠道
type LogNotificationChannel struct {
	name    string
	logger  Logger
	enabled bool
}

// NewLogNotificationChannel 创建日志通知渠道
func NewLogNotificationChannel(name string, logger Logger) *LogNotificationChannel {
	return &LogNotificationChannel{
		name:    name,
		logger:  logger,
		enabled: true,
	}
}

// SendNotification 发送通知
func (c *LogNotificationChannel) SendNotification(ctx context.Context, alert *Alert) error {
	switch alert.Level {
	case AlertLevelInfo:
		c.logger.Info(alert.Title,
			zap.String("alert_id", alert.ID),
			zap.String("level", string(alert.Level)),
			zap.String("component_type", string(alert.ComponentType)),
			zap.String("component_name", alert.ComponentName),
			zap.String("message", alert.Message),
			zap.Any("details", alert.Details),
			zap.String("status", alert.Status),
			zap.Int("count", alert.Count),
			zap.Time("created_at", alert.CreatedAt),
		)
	case AlertLevelWarning:
		c.logger.Warn(alert.Title,
			zap.String("alert_id", alert.ID),
			zap.String("level", string(alert.Level)),
			zap.String("component_type", string(alert.ComponentType)),
			zap.String("component_name", alert.ComponentName),
			zap.String("message", alert.Message),
			zap.Any("details", alert.Details),
			zap.String("status", alert.Status),
			zap.Int("count", alert.Count),
			zap.Time("created_at", alert.CreatedAt),
		)
	case AlertLevelError, AlertLevelCritical:
		c.logger.Error(alert.Title,
			zap.String("alert_id", alert.ID),
			zap.String("level", string(alert.Level)),
			zap.String("component_type", string(alert.ComponentType)),
			zap.String("component_name", alert.ComponentName),
			zap.String("message", alert.Message),
			zap.Any("details", alert.Details),
			zap.String("status", alert.Status),
			zap.Int("count", alert.Count),
			zap.Time("created_at", alert.CreatedAt),
		)
	}

	return nil
}

// GetChannelType 获取渠道类型
func (c *LogNotificationChannel) GetChannelType() string {
	return "log"
}

// IsEnabled 检查是否启用
func (c *LogNotificationChannel) IsEnabled() bool {
	return c.enabled
}

// SetEnabled 设置启用状态
func (c *LogNotificationChannel) SetEnabled(enabled bool) {
	c.enabled = enabled
}
