package mcp

import (
	"context"
	"testing"
	"time"

	"pumppill/rag/core"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockEventSubscriber 模拟事件订阅者
type MockEventSubscriber struct {
	mock.Mock
	events []EventType
}

func (m *MockEventSubscriber) OnEvent(event *Event) {
	m.Called(event)
}

func (m *MockEventSubscriber) GetSubscribedEvents() []EventType {
	return m.events
}

func TestNewEventBus(t *testing.T) {
	mockLogger := &MockLogger{}
	eventBus := NewEventBus(mockLogger)

	assert.NotNil(t, eventBus)
	assert.NotNil(t, eventBus.subscribers)
}

func TestEventBus_Subscribe(t *testing.T) {
	mockLogger := &MockLogger{}
	mockLogger.On("Info", mock.Anything).Maybe().Return()

	eventBus := NewEventBus(mockLogger)
	subscriber := &MockEventSubscriber{
		events: []EventType{EventConnectionOpened},
	}

	eventBus.Subscribe(EventConnectionOpened, subscriber)

	// 验证订阅者已添加
	subscribers := eventBus.subscribers[EventConnectionOpened]
	assert.Len(t, subscribers, 1)
	assert.Equal(t, subscriber, subscribers[0])

	mockLogger.AssertExpectations(t)
}

func TestEventBus_Publish(t *testing.T) {
	mockLogger := &MockLogger{}
	mockLogger.On("Info", mock.Anything).Maybe().Return()

	eventBus := NewEventBus(mockLogger)
	subscriber := &MockEventSubscriber{
		events: []EventType{EventConnectionOpened},
	}

	event := &Event{
		Type:      EventConnectionOpened,
		Timestamp: time.Now(),
	}

	subscriber.On("OnEvent", event).Return()
	eventBus.Subscribe(EventConnectionOpened, subscriber)

	// 发布事件
	eventBus.Publish(event)

	// 等待异步处理完成
	time.Sleep(10 * time.Millisecond)

	subscriber.AssertExpectations(t)
	mockLogger.AssertExpectations(t)
}

func TestNewStatusReporter(t *testing.T) {
	mockLogger := &MockLogger{}
	mockLogger.On("Info", mock.Anything).Maybe().Return()

	mockMetrics := &MockMetricsCollector{}
	mockMetrics.On("IncrementCounter", mock.Anything, mock.Anything).Maybe().Return()
	mockMetrics.On("RecordHistogram", mock.Anything, mock.Anything, mock.Anything).Maybe().Return()
	mockMetrics.On("SetGauge", mock.Anything, mock.Anything, mock.Anything).Maybe().Return()

	// 创建一个简单的服务器实例用于测试
	config := &core.MCPConfig{
		Host:           "localhost",
		Port:           8080,
		Timeout:        30 * time.Second,
		MaxConnections: 100,
	}
	server := NewMCPServer(config, mockLogger, mockMetrics, nil)

	reporter := NewStatusReporter(server, mockLogger, mockMetrics)

	assert.NotNil(t, reporter)
	assert.Equal(t, server, reporter.server)
	assert.Equal(t, mockLogger, reporter.logger)
	assert.Equal(t, mockMetrics, reporter.metrics)

	mockLogger.AssertExpectations(t)
	mockMetrics.AssertExpectations(t)
}

func TestStatusReporter_OnEvent(t *testing.T) {
	mockLogger := &MockLogger{}
	mockLogger.On("Info", mock.Anything).Maybe().Return()
	mockLogger.On("Error", mock.Anything).Maybe().Return()

	mockMetrics := &MockMetricsCollector{}
	mockMetrics.On("IncrementCounter", mock.Anything, mock.Anything).Return()

	config := &core.MCPConfig{
		Host:           "localhost",
		Port:           8080,
		Timeout:        30 * time.Second,
		MaxConnections: 100,
	}
	server := NewMCPServer(config, mockLogger, mockMetrics, nil)
	reporter := NewStatusReporter(server, mockLogger, mockMetrics)

	// 测试不同类型的事件
	events := []*Event{
		{Type: EventConnectionOpened, Timestamp: time.Now(), ConnectionID: "conn1"},
		{Type: EventConnectionClosed, Timestamp: time.Now(), ConnectionID: "conn1"},
		{Type: EventRequestReceived, Timestamp: time.Now()},
		{Type: EventRequestProcessed, Timestamp: time.Now()},
		{Type: EventNotificationSent, Timestamp: time.Now()},
		{Type: EventError, Timestamp: time.Now(), Data: map[string]interface{}{"error": "test error"}},
	}

	for _, event := range events {
		reporter.OnEvent(event)
	}

	mockLogger.AssertExpectations(t)
	mockMetrics.AssertExpectations(t)
}

func TestNewMCPServer(t *testing.T) {
	mockLogger := &MockLogger{}
	mockLogger.On("Info", mock.Anything).Maybe().Return()

	mockMetrics := &MockMetricsCollector{}
	mockMetrics.On("IncrementCounter", mock.Anything, mock.Anything).Maybe().Return()
	mockMetrics.On("RecordHistogram", mock.Anything, mock.Anything, mock.Anything).Maybe().Return()
	mockMetrics.On("SetGauge", mock.Anything, mock.Anything, mock.Anything).Maybe().Return()

	mockSecurity := &MockSecurityManager{}

	config := &core.MCPConfig{
		Host:           "localhost",
		Port:           8080,
		Timeout:        30 * time.Second,
		MaxConnections: 100,
	}

	server := NewMCPServer(config, mockLogger, mockMetrics, mockSecurity)

	assert.NotNil(t, server)
	assert.Equal(t, config, server.config)
	assert.NotNil(t, server.connectionManager)
	assert.NotNil(t, server.requestProcessor)
	assert.NotNil(t, server.asyncProcessor)
	assert.NotNil(t, server.handlerRegistry)
	assert.NotNil(t, server.eventBus)
	assert.NotNil(t, server.statusReporter)
	assert.NotNil(t, server.progressNotifier)
	assert.False(t, server.IsRunning())

	mockLogger.AssertExpectations(t)
	mockMetrics.AssertExpectations(t)
}

func TestMCPServer_RegisterHandler(t *testing.T) {
	mockLogger := &MockLogger{}
	mockLogger.On("Info", mock.Anything).Maybe().Return()

	mockMetrics := &MockMetricsCollector{}
	mockMetrics.On("IncrementCounter", mock.Anything, mock.Anything).Maybe().Return()
	mockMetrics.On("RecordHistogram", mock.Anything, mock.Anything, mock.Anything).Maybe().Return()
	mockMetrics.On("SetGauge", mock.Anything, mock.Anything, mock.Anything).Maybe().Return()

	config := &core.MCPConfig{
		Host:           "localhost",
		Port:           8080,
		Timeout:        30 * time.Second,
		MaxConnections: 100,
	}

	server := NewMCPServer(config, mockLogger, mockMetrics, nil)
	mockHandler := &MockMCPHandler{}

	err := server.RegisterHandler("test.method", mockHandler)
	assert.NoError(t, err)

	// 验证处理器已注册
	methods := server.GetHandlerMethods()
	assert.Contains(t, methods, "test.method")

	mockLogger.AssertExpectations(t)
	mockMetrics.AssertExpectations(t)
}

func TestMCPServer_BroadcastNotification(t *testing.T) {
	mockLogger := &MockLogger{}
	mockLogger.On("Info", mock.Anything).Maybe().Return()
	mockLogger.On("Error", mock.Anything).Maybe().Return()

	mockMetrics := &MockMetricsCollector{}
	mockMetrics.On("IncrementCounter", mock.Anything, mock.Anything).Maybe().Return()
	mockMetrics.On("RecordHistogram", mock.Anything, mock.Anything, mock.Anything).Maybe().Return()
	mockMetrics.On("SetGauge", mock.Anything, mock.Anything, mock.Anything).Maybe().Return()

	config := &core.MCPConfig{
		Host:           "localhost",
		Port:           8080,
		Timeout:        30 * time.Second,
		MaxConnections: 100,
	}

	server := NewMCPServer(config, mockLogger, mockMetrics, nil)

	notification := &core.MCPNotification{
		Method: "test.notification",
		Params: map[string]interface{}{"key": "value"},
	}

	err := server.BroadcastNotification(notification)
	assert.NoError(t, err)

	mockLogger.AssertExpectations(t)
	mockMetrics.AssertExpectations(t)
}

func TestMCPServer_StartStop(t *testing.T) {
	mockLogger := &MockLogger{}
	mockLogger.On("Info", mock.Anything).Maybe().Return()
	mockLogger.On("Error", mock.Anything).Maybe().Return()

	mockMetrics := &MockMetricsCollector{}
	mockMetrics.On("IncrementCounter", mock.Anything, mock.Anything).Maybe().Return()
	mockMetrics.On("RecordHistogram", mock.Anything, mock.Anything, mock.Anything).Maybe().Return()
	mockMetrics.On("SetGauge", mock.Anything, mock.Anything, mock.Anything).Maybe().Return()

	config := &core.MCPConfig{
		Host:           "localhost",
		Port:           0, // 使用随机端口
		Timeout:        30 * time.Second,
		MaxConnections: 100,
	}

	server := NewMCPServer(config, mockLogger, mockMetrics, nil)

	// 测试启动
	err := server.Start(context.Background(), "localhost:0")
	assert.NoError(t, err)
	assert.True(t, server.IsRunning())

	// 等待一小段时间确保服务器启动
	time.Sleep(10 * time.Millisecond)

	// 测试重复启动
	err = server.Start(context.Background(), "localhost:0")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already running")

	// 测试停止
	err = server.Stop(context.Background())
	assert.NoError(t, err)
	assert.False(t, server.IsRunning())

	// 测试重复停止
	err = server.Stop(context.Background())
	assert.NoError(t, err)

	mockLogger.AssertExpectations(t)
	mockMetrics.AssertExpectations(t)
}

func TestMCPServer_GetConnectionStats(t *testing.T) {
	mockLogger := &MockLogger{}
	mockLogger.On("Info", mock.Anything).Maybe().Return()

	mockMetrics := &MockMetricsCollector{}
	mockMetrics.On("IncrementCounter", mock.Anything, mock.Anything).Maybe().Return()
	mockMetrics.On("RecordHistogram", mock.Anything, mock.Anything, mock.Anything).Maybe().Return()
	mockMetrics.On("SetGauge", mock.Anything, mock.Anything, mock.Anything).Maybe().Return()

	config := &core.MCPConfig{
		Host:           "localhost",
		Port:           8080,
		Timeout:        30 * time.Second,
		MaxConnections: 100,
	}

	server := NewMCPServer(config, mockLogger, mockMetrics, nil)

	stats := server.GetConnectionStats()
	assert.NotNil(t, stats)
	assert.Contains(t, stats, "total_connections")
	assert.Contains(t, stats, "active_connections")
	assert.Contains(t, stats, "connection_details")

	mockLogger.AssertExpectations(t)
	mockMetrics.AssertExpectations(t)
}

func TestMCPServer_EventSubscription(t *testing.T) {
	mockLogger := &MockLogger{}
	mockLogger.On("Info", mock.Anything).Maybe().Return()

	mockMetrics := &MockMetricsCollector{}
	mockMetrics.On("IncrementCounter", mock.Anything, mock.Anything).Maybe().Return()
	mockMetrics.On("RecordHistogram", mock.Anything, mock.Anything, mock.Anything).Maybe().Return()
	mockMetrics.On("SetGauge", mock.Anything, mock.Anything, mock.Anything).Maybe().Return()

	config := &core.MCPConfig{
		Host:           "localhost",
		Port:           8080,
		Timeout:        30 * time.Second,
		MaxConnections: 100,
	}

	server := NewMCPServer(config, mockLogger, mockMetrics, nil)
	subscriber := &MockEventSubscriber{
		events: []EventType{EventConnectionOpened},
	}

	// 测试订阅
	server.SubscribeToEvents(EventConnectionOpened, subscriber)

	// 测试取消订阅
	server.UnsubscribeFromEvents(EventConnectionOpened, subscriber)

	mockLogger.AssertExpectations(t)
	mockMetrics.AssertExpectations(t)
}
