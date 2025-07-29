package monitor

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

// MockHealthChecker 模拟健康检查器
type MockHealthChecker struct {
	name          string
	componentType ComponentType
	enabled       bool
	status        HealthStatus
	message       string
	checkDelay    time.Duration
	shouldError   bool
}

func NewMockHealthChecker(name string, componentType ComponentType) *MockHealthChecker {
	return &MockHealthChecker{
		name:          name,
		componentType: componentType,
		enabled:       true,
		status:        HealthStatusHealthy,
		message:       "模拟组件正常",
		checkDelay:    0,
		shouldError:   false,
	}
}

func (m *MockHealthChecker) CheckHealth(ctx context.Context) *HealthCheckResult {
	if m.checkDelay > 0 {
		time.Sleep(m.checkDelay)
	}

	result := &HealthCheckResult{
		ComponentType: m.componentType,
		ComponentName: m.name,
		Status:        m.status,
		Message:       m.message,
		CheckTime:     time.Now(),
		Duration:      m.checkDelay,
		Details:       make(map[string]interface{}),
	}

	if m.shouldError {
		result.Error = "模拟错误"
	}

	return result
}

func (m *MockHealthChecker) GetComponentType() ComponentType {
	return m.componentType
}

func (m *MockHealthChecker) GetName() string {
	return m.name
}

func (m *MockHealthChecker) IsEnabled() bool {
	return m.enabled
}

func (m *MockHealthChecker) SetEnabled(enabled bool) {
	m.enabled = enabled
}

// MockNotificationChannel 模拟通知渠道
type MockNotificationChannel struct {
	name          string
	channelType   string
	enabled       bool
	notifications []*Alert
}

func NewMockNotificationChannel(name, channelType string) *MockNotificationChannel {
	return &MockNotificationChannel{
		name:          name,
		channelType:   channelType,
		enabled:       true,
		notifications: make([]*Alert, 0),
	}
}

func (m *MockNotificationChannel) SendNotification(ctx context.Context, alert *Alert) error {
	m.notifications = append(m.notifications, alert)
	return nil
}

func (m *MockNotificationChannel) GetChannelType() string {
	return m.channelType
}

func (m *MockNotificationChannel) IsEnabled() bool {
	return m.enabled
}

func (m *MockNotificationChannel) SetEnabled(enabled bool) {
	m.enabled = enabled
}

func (m *MockNotificationChannel) GetNotifications() []*Alert {
	return m.notifications
}

func (m *MockNotificationChannel) ClearNotifications() {
	m.notifications = make([]*Alert, 0)
}

// TestHealthManager_RegisterChecker 测试注册健康检查器
func TestHealthManager_RegisterChecker(t *testing.T) {
	zapLogger := zaptest.NewLogger(t)
	logger := &zapLoggerWrapper{logger: zapLogger}
	config := &HealthConfig{
		CheckInterval: 30 * time.Second,
		Timeout:       10 * time.Second,
		RetryCount:    3,
		RetryInterval: 5 * time.Second,
		AlertCooldown: 5 * time.Minute,
		HistorySize:   100,
	}

	hm := NewHealthManager(config, logger, nil)

	// 测试注册健康检查器
	checker := NewMockHealthChecker("test-db", ComponentTypeDatabase)
	err := hm.RegisterChecker("test-db", checker)
	if err != nil {
		t.Fatalf("注册健康检查器失败: %v", err)
	}

	// 测试重复注册
	err = hm.RegisterChecker("test-db", checker)
	if err == nil {
		t.Fatal("重复注册应该返回错误")
	}

	// 验证检查器已注册
	hm.mutex.RLock()
	if _, exists := hm.checkers["test-db"]; !exists {
		t.Fatal("健康检查器未正确注册")
	}
	hm.mutex.RUnlock()
}

// TestHealthManager_RegisterNotificationChannel 测试注册通知渠道
func TestHealthManager_RegisterNotificationChannel(t *testing.T) {
	zapLogger := zaptest.NewLogger(t)
	logger := &zapLoggerWrapper{logger: zapLogger}
	config := &HealthConfig{}

	hm := NewHealthManager(config, logger, nil)

	// 测试注册通知渠道
	channel := NewMockNotificationChannel("test-channel", "mock")
	err := hm.RegisterNotificationChannel("test-channel", channel)
	if err != nil {
		t.Fatalf("注册通知渠道失败: %v", err)
	}

	// 测试重复注册
	err = hm.RegisterNotificationChannel("test-channel", channel)
	if err == nil {
		t.Fatal("重复注册应该返回错误")
	}

	// 验证渠道已注册
	hm.mutex.RLock()
	if _, exists := hm.channels["test-channel"]; !exists {
		t.Fatal("通知渠道未正确注册")
	}
	hm.mutex.RUnlock()
}

// TestHealthManager_CheckHealth 测试健康检查
func TestHealthManager_CheckHealth(t *testing.T) {
	zapLogger := zaptest.NewLogger(t)
	logger := &zapLoggerWrapper{logger: zapLogger}
	config := &HealthConfig{
		Timeout:    5 * time.Second,
		RetryCount: 1,
	}

	hm := NewHealthManager(config, logger, nil)

	// 注册多个健康检查器
	healthyChecker := NewMockHealthChecker("healthy-service", ComponentTypeSystem)
	healthyChecker.status = HealthStatusHealthy

	degradedChecker := NewMockHealthChecker("degraded-service", ComponentTypeCache)
	degradedChecker.status = HealthStatusDegraded
	degradedChecker.message = "服务性能下降"

	unhealthyChecker := NewMockHealthChecker("unhealthy-service", ComponentTypeDatabase)
	unhealthyChecker.status = HealthStatusUnhealthy
	unhealthyChecker.message = "服务不可用"

	hm.RegisterChecker("healthy", healthyChecker)
	hm.RegisterChecker("degraded", degradedChecker)
	hm.RegisterChecker("unhealthy", unhealthyChecker)

	// 执行健康检查
	ctx := context.Background()
	report := hm.CheckHealth(ctx)

	// 验证报告
	if report == nil {
		t.Fatal("健康报告不应为空")
	}

	if report.OverallStatus != HealthStatusUnhealthy {
		t.Errorf("期望整体状态为 %s，实际为 %s", HealthStatusUnhealthy, report.OverallStatus)
	}

	if len(report.Components) != 3 {
		t.Errorf("期望3个组件，实际为 %d", len(report.Components))
	}

	// 验证摘要
	if report.Summary.TotalComponents != 3 {
		t.Errorf("期望总组件数为3，实际为 %d", report.Summary.TotalComponents)
	}

	if report.Summary.HealthyComponents != 1 {
		t.Errorf("期望健康组件数为1，实际为 %d", report.Summary.HealthyComponents)
	}

	if report.Summary.DegradedComponents != 1 {
		t.Errorf("期望降级组件数为1，实际为 %d", report.Summary.DegradedComponents)
	}

	if report.Summary.UnhealthyComponents != 1 {
		t.Errorf("期望不健康组件数为1，实际为 %d", report.Summary.UnhealthyComponents)
	}
}

// TestHealthManager_AlertGeneration 测试告警生成
func TestHealthManager_AlertGeneration(t *testing.T) {
	zapLogger := zaptest.NewLogger(t)
	logger := &zapLoggerWrapper{logger: zapLogger}
	config := &HealthConfig{
		AlertCooldown: 1 * time.Second, // 短冷却时间用于测试
		NotificationRules: []*NotificationRule{
			{
				Name:          "critical-alerts",
				ComponentType: ComponentTypeDatabase,
				AlertLevel:    AlertLevelCritical,
				Channels:      []string{"test-channel"},
				Cooldown:      1 * time.Second,
				Enabled:       true,
			},
		},
	}

	hm := NewHealthManager(config, logger, nil)

	// 注册通知渠道
	channel := NewMockNotificationChannel("test-channel", "mock")
	hm.RegisterNotificationChannel("test-channel", channel)

	// 注册不健康的检查器
	unhealthyChecker := NewMockHealthChecker("failing-db", ComponentTypeDatabase)
	unhealthyChecker.status = HealthStatusUnhealthy
	unhealthyChecker.message = "数据库连接失败"

	hm.RegisterChecker("failing-db", unhealthyChecker)

	// 执行健康检查
	ctx := context.Background()
	hm.CheckHealth(ctx)

	// 等待告警处理
	time.Sleep(100 * time.Millisecond)

	// 验证告警生成
	activeAlerts := hm.GetActiveAlerts()
	if len(activeAlerts) == 0 {
		t.Fatal("应该生成告警")
	}

	alert := activeAlerts[0]
	if alert.Level != AlertLevelCritical {
		t.Errorf("期望告警级别为 %s，实际为 %s", AlertLevelCritical, alert.Level)
	}

	if alert.ComponentType != ComponentTypeDatabase {
		t.Errorf("期望组件类型为 %s，实际为 %s", ComponentTypeDatabase, alert.ComponentType)
	}

	// 验证通知发送
	notifications := channel.GetNotifications()
	if len(notifications) == 0 {
		t.Fatal("应该发送通知")
	}

	// 测试告警恢复
	unhealthyChecker.status = HealthStatusHealthy
	unhealthyChecker.message = "数据库连接正常"

	// 等待冷却时间
	time.Sleep(1100 * time.Millisecond)

	// 再次执行健康检查
	hm.CheckHealth(ctx)

	// 等待告警处理
	time.Sleep(100 * time.Millisecond)

	// 验证告警已解决
	activeAlerts = hm.GetActiveAlerts()
	if len(activeAlerts) != 0 {
		t.Errorf("告警应该已解决，但仍有 %d 个活跃告警", len(activeAlerts))
	}
}

// TestHealthManager_StartStop 测试启动停止
func TestHealthManager_StartStop(t *testing.T) {
	zapLogger := zaptest.NewLogger(t)
	logger := &zapLoggerWrapper{logger: zapLogger}
	config := &HealthConfig{
		CheckInterval: 100 * time.Millisecond, // 短间隔用于测试
	}

	hm := NewHealthManager(config, logger, nil)

	// 注册健康检查器
	checker := NewMockHealthChecker("test-service", ComponentTypeSystem)
	hm.RegisterChecker("test-service", checker)

	ctx := context.Background()

	// 测试启动
	err := hm.Start(ctx)
	if err != nil {
		t.Fatalf("启动健康管理器失败: %v", err)
	}

	// 验证运行状态
	if !hm.running {
		t.Fatal("健康管理器应该处于运行状态")
	}

	// 等待几次检查
	time.Sleep(300 * time.Millisecond)

	// 验证有健康报告生成
	report := hm.GetHealthReport()
	if report == nil {
		t.Fatal("应该有健康报告")
	}

	// 测试停止
	err = hm.Stop(ctx)
	if err != nil {
		t.Fatalf("停止健康管理器失败: %v", err)
	}

	// 验证停止状态
	if hm.running {
		t.Fatal("健康管理器应该已停止")
	}
}

// TestHealthManager_HealthHistory 测试健康历史记录
func TestHealthManager_HealthHistory(t *testing.T) {
	zapLogger := zaptest.NewLogger(t)
	logger := &zapLoggerWrapper{logger: zapLogger}
	config := &HealthConfig{
		HistorySize: 5, // 小的历史记录大小用于测试
	}

	hm := NewHealthManager(config, logger, nil)

	// 注册健康检查器
	checker := NewMockHealthChecker("test-service", ComponentTypeSystem)
	hm.RegisterChecker("test-service", checker)

	ctx := context.Background()

	// 执行多次健康检查
	for i := 0; i < 10; i++ {
		hm.CheckHealth(ctx)
		time.Sleep(10 * time.Millisecond)
	}

	// 验证历史记录大小限制
	history := hm.GetHealthHistory(0)
	if len(history) > config.HistorySize {
		t.Errorf("历史记录大小应该限制在 %d，实际为 %d", config.HistorySize, len(history))
	}

	// 验证获取指定数量的历史记录
	limitedHistory := hm.GetHealthHistory(3)
	if len(limitedHistory) != 3 {
		t.Errorf("期望获取3条历史记录，实际为 %d", len(limitedHistory))
	}
}

// TestHealthManager_AlertSuppression 测试告警抑制
func TestHealthManager_AlertSuppression(t *testing.T) {
	zapLogger := zaptest.NewLogger(t)
	logger := &zapLoggerWrapper{logger: zapLogger}
	config := &HealthConfig{
		AlertCooldown: 100 * time.Millisecond,
		NotificationRules: []*NotificationRule{
			{
				Name:          "system-alerts",
				ComponentType: ComponentTypeSystem,
				AlertLevel:    AlertLevelCritical,
				Channels:      []string{"test-channel"},
				Cooldown:      100 * time.Millisecond,
				Enabled:       true,
			},
		},
	}

	hm := NewHealthManager(config, logger, nil)

	// 注册通知渠道
	channel := NewMockNotificationChannel("test-channel", "mock")
	hm.RegisterNotificationChannel("test-channel", channel)

	// 注册不健康的检查器
	unhealthyChecker := NewMockHealthChecker("failing-service", ComponentTypeSystem)
	unhealthyChecker.status = HealthStatusUnhealthy
	unhealthyChecker.message = "服务失败"

	hm.RegisterChecker("failing-service", unhealthyChecker)

	ctx := context.Background()

	// 先执行一次健康检查，看看会生成什么告警
	hm.CheckHealth(ctx)
	time.Sleep(200 * time.Millisecond)

	// 获取生成的告警
	activeAlerts := hm.GetActiveAlerts()
	if len(activeAlerts) == 0 {
		t.Fatal("应该生成告警")
	}

	// 清除通知记录
	channel.ClearNotifications()

	// 抑制告警 - 使用实际的告警键格式
	alertKey := fmt.Sprintf("%s:%s", unhealthyChecker.GetComponentType(), unhealthyChecker.GetName())
	err := hm.SuppressAlert(alertKey, 1*time.Second)
	if err != nil {
		t.Fatalf("抑制告警失败: %v", err)
	}

	// 再次执行健康检查
	hm.CheckHealth(ctx)

	// 等待处理
	time.Sleep(200 * time.Millisecond)

	// 验证没有发送新的组件告警通知（可能还有整体状态告警）
	notifications := channel.GetNotifications()
	componentAlerts := 0
	for _, notification := range notifications {
		if notification.ComponentName == "failing-service" {
			componentAlerts++
		}
	}
	if componentAlerts != 0 {
		t.Errorf("抑制期间不应发送组件告警通知，但发送了 %d 个", componentAlerts)
	}

	// 等待抑制期结束
	time.Sleep(1100 * time.Millisecond)

	// 再次执行健康检查
	hm.CheckHealth(ctx)

	// 等待处理
	time.Sleep(200 * time.Millisecond)

	// 验证现在发送了组件告警通知
	notifications = channel.GetNotifications()
	componentAlerts = 0
	for _, notification := range notifications {
		if notification.ComponentName == "failing-service" {
			componentAlerts++
		}
	}
	if componentAlerts == 0 {
		t.Fatal("抑制期结束后应该发送组件告警通知")
	}
}

// TestDatabaseHealthChecker 测试数据库健康检查器
func TestDatabaseHealthChecker(t *testing.T) {
	// 注意：这个测试需要实际的数据库连接，在实际环境中可能需要使用测试数据库
	// 这里只是展示测试结构

	t.Skip("跳过数据库健康检查器测试，需要实际数据库连接")

	// 创建模拟数据库连接
	// db, err := sql.Open("mysql", "test_connection_string")
	// if err != nil {
	// 	t.Fatalf("创建数据库连接失败: %v", err)
	// }
	// defer db.Close()

	// checker := NewDatabaseHealthChecker("test-db", db)

	// ctx := context.Background()
	// result := checker.CheckHealth(ctx)

	// if result == nil {
	// 	t.Fatal("健康检查结果不应为空")
	// }

	// if result.ComponentType != ComponentTypeDatabase {
	// 	t.Errorf("期望组件类型为 %s，实际为 %s", ComponentTypeDatabase, result.ComponentType)
	// }
}

// TestLogNotificationChannel 测试日志通知渠道
func TestLogNotificationChannel(t *testing.T) {
	zapLogger := zaptest.NewLogger(t)
	logger := &zapLoggerWrapper{logger: zapLogger}
	channel := NewLogNotificationChannel("test-log", logger)

	// 创建测试告警
	alert := &Alert{
		ID:            "test-alert-1",
		Level:         AlertLevelWarning,
		ComponentType: ComponentTypeSystem,
		ComponentName: "test-component",
		Title:         "测试告警",
		Message:       "这是一个测试告警",
		Details:       map[string]interface{}{"test": "value"},
		CreatedAt:     time.Now(),
		Status:        "active",
		Count:         1,
	}

	ctx := context.Background()

	// 发送通知
	err := channel.SendNotification(ctx, alert)
	if err != nil {
		t.Fatalf("发送日志通知失败: %v", err)
	}

	// 验证渠道属性
	if channel.GetChannelType() != "log" {
		t.Errorf("期望渠道类型为 'log'，实际为 '%s'", channel.GetChannelType())
	}

	if !channel.IsEnabled() {
		t.Error("渠道应该默认启用")
	}

	// 测试禁用渠道
	channel.SetEnabled(false)
	if channel.IsEnabled() {
		t.Error("渠道应该已禁用")
	}
}

// BenchmarkHealthManager_CheckHealth 健康检查性能基准测试
func BenchmarkHealthManager_CheckHealth(b *testing.B) {
	zapLogger := zap.NewNop()
	logger := &zapLoggerWrapper{logger: zapLogger}
	config := &HealthConfig{
		Timeout:    1 * time.Second,
		RetryCount: 1,
	}

	hm := NewHealthManager(config, logger, nil)

	// 注册多个健康检查器
	for i := 0; i < 10; i++ {
		checker := NewMockHealthChecker(
			fmt.Sprintf("service-%d", i),
			ComponentTypeSystem,
		)
		hm.RegisterChecker(fmt.Sprintf("service-%d", i), checker)
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hm.CheckHealth(ctx)
	}
}

// TestHealthManager_ConcurrentAccess 测试并发访问
func TestHealthManager_ConcurrentAccess(t *testing.T) {
	zapLogger := zaptest.NewLogger(t)
	logger := &zapLoggerWrapper{logger: zapLogger}
	config := &HealthConfig{
		CheckInterval: 10 * time.Millisecond,
	}

	hm := NewHealthManager(config, logger, nil)

	// 注册健康检查器
	checker := NewMockHealthChecker("concurrent-service", ComponentTypeSystem)
	hm.RegisterChecker("concurrent-service", checker)

	ctx := context.Background()

	// 启动健康管理器
	err := hm.Start(ctx)
	if err != nil {
		t.Fatalf("启动健康管理器失败: %v", err)
	}
	defer hm.Stop(ctx)

	// 并发访问健康报告
	const numGoroutines = 10
	const numIterations = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < numIterations; j++ {
				// 获取健康报告
				report := hm.GetHealthReport()
				if report == nil {
					t.Errorf("健康报告不应为空")
					return
				}

				// 获取历史记录
				history := hm.GetHealthHistory(5)
				if history == nil {
					t.Errorf("历史记录不应为空")
					return
				}

				// 获取活跃告警
				alerts := hm.GetActiveAlerts()
				_ = alerts // 使用变量避免未使用警告
			}
		}()
	}

	wg.Wait()
}
