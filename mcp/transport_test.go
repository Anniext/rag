package mcp

import (
	"net"
	"testing"
	"time"

	"pumppill/rag/core"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockLogger 模拟日志记录器
type MockLogger struct {
	mock.Mock
}

func (m *MockLogger) Debug(msg string, fields ...interface{}) {
	// 简化 mock 调用，只传递消息
	m.Called(msg)
}

func (m *MockLogger) Info(msg string, fields ...interface{}) {
	// 简化 mock 调用，只传递消息
	m.Called(msg)
}

func (m *MockLogger) Warn(msg string, fields ...interface{}) {
	// 简化 mock 调用，只传递消息
	m.Called(msg)
}

func (m *MockLogger) Error(msg string, fields ...interface{}) {
	// 简化 mock 调用，只传递消息
	m.Called(msg)
}

func (m *MockLogger) Fatal(msg string, fields ...interface{}) {
	// 简化 mock 调用，只传递消息
	m.Called(msg)
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

// MockConn 模拟网络连接
type MockConn struct {
	mock.Mock
	readData  []byte
	writeData []byte
}

func (m *MockConn) Read(b []byte) (int, error) {
	args := m.Called(b)
	if len(m.readData) > 0 {
		n := copy(b, m.readData)
		m.readData = m.readData[n:]
		return n, args.Error(1)
	}
	return args.Int(0), args.Error(1)
}

func (m *MockConn) Write(b []byte) (int, error) {
	args := m.Called(b)
	m.writeData = append(m.writeData, b...)
	return args.Int(0), args.Error(1)
}

func (m *MockConn) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockConn) LocalAddr() net.Addr {
	args := m.Called()
	return args.Get(0).(net.Addr)
}

func (m *MockConn) RemoteAddr() net.Addr {
	args := m.Called()
	return args.Get(0).(net.Addr)
}

func (m *MockConn) SetDeadline(t time.Time) error {
	args := m.Called(t)
	return args.Error(0)
}

func (m *MockConn) SetReadDeadline(t time.Time) error {
	args := m.Called(t)
	return args.Error(0)
}

func (m *MockConn) SetWriteDeadline(t time.Time) error {
	args := m.Called(t)
	return args.Error(0)
}

func TestConnectionState_String(t *testing.T) {
	tests := []struct {
		state    ConnectionState
		expected string
	}{
		{StateDisconnected, "disconnected"},
		{StateConnecting, "connecting"},
		{StateConnected, "connected"},
		{StateReconnecting, "reconnecting"},
		{StateClosed, "closed"},
		{ConnectionState(999), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.state.String())
		})
	}
}

func TestNewConnection(t *testing.T) {
	mockConn := &MockConn{}
	mockLogger := &MockLogger{}
	mockLogger.On("Info", mock.Anything).Maybe().Return()
	mockLogger.On("Error", mock.Anything).Maybe().Return()

	conn := NewConnection("test-id", mockConn, mockLogger)

	assert.Equal(t, "test-id", conn.ID())
	assert.Equal(t, StateConnected, conn.State())
	assert.True(t, conn.IsConnected())
	assert.NotNil(t, conn.sendChan)
	assert.NotNil(t, conn.receiveChan)
	assert.NotNil(t, conn.closeChan)
}

func TestConnection_SetState(t *testing.T) {
	mockConn := &MockConn{}
	mockLogger := &MockLogger{}
	mockLogger.On("Info", mock.Anything).Maybe().Return()
	mockLogger.On("Error", mock.Anything).Maybe().Return()

	conn := NewConnection("test-id", mockConn, mockLogger)

	conn.SetState(StateDisconnected)
	assert.Equal(t, StateDisconnected, conn.State())
	assert.False(t, conn.IsConnected())

	conn.SetState(StateConnected)
	assert.Equal(t, StateConnected, conn.State())
	assert.True(t, conn.IsConnected())
}

func TestConnection_Send(t *testing.T) {
	mockConn := &MockConn{}
	mockLogger := &MockLogger{}
	mockLogger.On("Info", mock.Anything).Maybe().Return()
	mockLogger.On("Error", mock.Anything).Maybe().Return()

	conn := NewConnection("test-id", mockConn, mockLogger)

	// 测试正常发送
	data := []byte("test data")
	err := conn.Send(data)
	assert.NoError(t, err)

	// 验证数据在发送通道中
	select {
	case receivedData := <-conn.sendChan:
		assert.Equal(t, data, receivedData)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("data not received in send channel")
	}

	// 测试断开连接时发送
	conn.SetState(StateDisconnected)
	err = conn.Send(data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "is not connected")
}

func TestConnection_MetadataOperations(t *testing.T) {
	mockConn := &MockConn{}
	mockLogger := &MockLogger{}
	mockLogger.On("Info", mock.Anything).Maybe().Return()
	mockLogger.On("Error", mock.Anything).Maybe().Return()

	conn := NewConnection("test-id", mockConn, mockLogger)

	// 测试设置和获取元数据
	conn.SetMetadata("key1", "value1")
	conn.SetMetadata("key2", 123)

	value1, exists1 := conn.GetMetadata("key1")
	assert.True(t, exists1)
	assert.Equal(t, "value1", value1)

	value2, exists2 := conn.GetMetadata("key2")
	assert.True(t, exists2)
	assert.Equal(t, 123, value2)

	// 测试不存在的键
	_, exists3 := conn.GetMetadata("nonexistent")
	assert.False(t, exists3)
}

func TestConnection_HeartbeatOperations(t *testing.T) {
	mockConn := &MockConn{}
	mockLogger := &MockLogger{}
	mockLogger.On("Info", mock.Anything).Maybe().Return()
	mockLogger.On("Error", mock.Anything).Maybe().Return()

	conn := NewConnection("test-id", mockConn, mockLogger)

	// 测试初始状态
	assert.True(t, conn.IsAlive(time.Minute))

	// 更新 ping 和 pong
	conn.UpdatePing()
	conn.UpdatePong()

	// 测试超时检查
	time.Sleep(10 * time.Millisecond)
	assert.True(t, conn.IsAlive(time.Minute))
	assert.False(t, conn.IsAlive(time.Nanosecond))
}

func TestConnection_Close(t *testing.T) {
	mockConn := &MockConn{}
	mockConn.On("Close").Return(nil)
	mockLogger := &MockLogger{}
	mockLogger.On("Info", mock.Anything).Maybe().Return()
	mockLogger.On("Error", mock.Anything).Maybe().Return()

	conn := NewConnection("test-id", mockConn, mockLogger)

	err := conn.Close()
	assert.NoError(t, err)
	assert.Equal(t, StateClosed, conn.State())

	mockConn.AssertExpectations(t)
}

func TestNewConnectionPool(t *testing.T) {
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsCollector{}

	pool := NewConnectionPool(10, mockLogger, mockMetrics)

	assert.NotNil(t, pool)
	assert.Equal(t, 10, pool.maxSize)
	assert.Equal(t, 0, pool.Size())
}

func TestConnectionPool_Add(t *testing.T) {
	mockLogger := &MockLogger{}
	mockLogger.On("Info", mock.Anything).Maybe().Return()
	mockMetrics := &MockMetricsCollector{}
	mockMetrics.On("SetGauge", "mcp_connections_active", mock.Anything, mock.Anything).Return()

	pool := NewConnectionPool(2, mockLogger, mockMetrics)

	mockConn1 := &MockConn{}
	conn1 := NewConnection("conn1", mockConn1, mockLogger)

	// 测试添加连接
	err := pool.Add(conn1)
	assert.NoError(t, err)
	assert.Equal(t, 1, pool.Size())

	mockConn2 := &MockConn{}
	conn2 := NewConnection("conn2", mockConn2, mockLogger)

	err = pool.Add(conn2)
	assert.NoError(t, err)
	assert.Equal(t, 2, pool.Size())

	// 测试池满时添加连接
	mockConn3 := &MockConn{}
	conn3 := NewConnection("conn3", mockConn3, mockLogger)

	err = pool.Add(conn3)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connection pool is full")
	assert.Equal(t, 2, pool.Size())

	mockLogger.AssertExpectations(t)
	mockMetrics.AssertExpectations(t)
}

func TestRoundRobinBalancer_SelectConnection(t *testing.T) {
	balancer := NewRoundRobinBalancer()

	// 测试空连接列表
	conn := balancer.SelectConnection([]*Connection{})
	assert.Nil(t, conn)

	// 创建测试连接
	mockLogger := &MockLogger{}
	mockLogger.On("Info", mock.Anything).Maybe().Return()
	mockLogger.On("Error", mock.Anything).Maybe().Return()

	mockConn1 := &MockConn{}
	conn1 := NewConnection("conn1", mockConn1, mockLogger)
	mockConn2 := &MockConn{}
	conn2 := NewConnection("conn2", mockConn2, mockLogger)
	mockConn3 := &MockConn{}
	conn3 := NewConnection("conn3", mockConn3, mockLogger)

	connections := []*Connection{conn1, conn2, conn3}

	// 测试轮询选择
	selected1 := balancer.SelectConnection(connections)
	selected2 := balancer.SelectConnection(connections)
	selected3 := balancer.SelectConnection(connections)
	selected4 := balancer.SelectConnection(connections) // 应该回到第一个

	assert.Equal(t, conn2, selected1) // 因为 counter 从 1 开始
	assert.Equal(t, conn3, selected2)
	assert.Equal(t, conn1, selected3)
	assert.Equal(t, conn2, selected4)

	mockLogger.AssertExpectations(t)
}

func TestNewConnectionManager(t *testing.T) {
	config := &core.MCPConfig{
		Host:           "localhost",
		Port:           8080,
		Timeout:        30 * time.Second,
		MaxConnections: 100,
	}

	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsCollector{}

	manager := NewConnectionManager(config, mockLogger, mockMetrics)

	assert.NotNil(t, manager)
	assert.NotNil(t, manager.pool)
	assert.NotNil(t, manager.balancer)
	assert.Equal(t, config, manager.config)
}

func TestConnectionManager_StartStop(t *testing.T) {
	config := &core.MCPConfig{
		Host:           "localhost",
		Port:           8080,
		Timeout:        30 * time.Second,
		MaxConnections: 100,
	}

	mockLogger := &MockLogger{}
	mockLogger.On("Info", mock.Anything).Maybe().Return()
	mockLogger.On("Error", mock.Anything).Maybe().Return()
	mockMetrics := &MockMetricsCollector{}

	manager := NewConnectionManager(config, mockLogger, mockMetrics)

	// 启动管理器
	manager.Start()

	// 等待一小段时间确保 goroutines 启动
	time.Sleep(10 * time.Millisecond)

	// 停止管理器
	manager.Stop()

	mockLogger.AssertExpectations(t)
}

func TestConnectionManager_AddRemoveConnection(t *testing.T) {
	config := &core.MCPConfig{
		Host:           "localhost",
		Port:           8080,
		Timeout:        30 * time.Second,
		MaxConnections: 100,
	}

	mockLogger := &MockLogger{}
	mockLogger.On("Info", mock.Anything).Maybe().Return()
	mockLogger.On("Error", mock.Anything).Maybe().Return()
	mockMetrics := &MockMetricsCollector{}
	mockMetrics.On("IncrementCounter", "mcp_connections_created", mock.Anything).Return()
	mockMetrics.On("IncrementCounter", "mcp_connections_removed", mock.Anything).Return()
	mockMetrics.On("SetGauge", "mcp_connections_active", mock.Anything, mock.Anything).Return()

	manager := NewConnectionManager(config, mockLogger, mockMetrics)

	mockConn := &MockConn{}
	mockConn.On("SetReadDeadline", mock.Anything).Return(nil)
	mockConn.On("SetWriteDeadline", mock.Anything).Return(nil)
	mockConn.On("Read", mock.Anything).Return(0, net.ErrClosed)
	mockConn.On("Write", mock.Anything).Return(0, nil)
	mockConn.On("Close").Return(nil)

	// 添加连接
	conn, err := manager.AddConnection(mockConn)
	assert.NoError(t, err)
	assert.NotNil(t, conn)

	// 验证连接已添加
	retrievedConn, exists := manager.GetConnection(conn.ID())
	assert.True(t, exists)
	assert.Equal(t, conn, retrievedConn)

	// 移除连接
	manager.RemoveConnection(conn.ID())

	// 验证连接已移除
	_, exists = manager.GetConnection(conn.ID())
	assert.False(t, exists)

	mockLogger.AssertExpectations(t)
	mockMetrics.AssertExpectations(t)
}

func TestConnectionManager_GetConnectionStats(t *testing.T) {
	config := &core.MCPConfig{
		Host:           "localhost",
		Port:           8080,
		Timeout:        30 * time.Second,
		MaxConnections: 100,
	}

	mockLogger := &MockLogger{}
	mockLogger.On("Info", mock.Anything).Maybe().Return()
	mockLogger.On("Error", mock.Anything).Maybe().Return()
	mockMetrics := &MockMetricsCollector{}
	mockMetrics.On("IncrementCounter", "mcp_connections_created", mock.Anything).Return()
	mockMetrics.On("SetGauge", "mcp_connections_active", mock.Anything, mock.Anything).Return()

	manager := NewConnectionManager(config, mockLogger, mockMetrics)

	// 测试空统计
	stats := manager.GetConnectionStats()
	assert.Equal(t, 0, stats["total_connections"])
	assert.Equal(t, 0, stats["active_connections"])
	assert.Len(t, stats["connection_details"], 0)

	// 添加连接
	mockConn := &MockConn{}
	mockConn.On("SetReadDeadline", mock.Anything).Return(nil)
	mockConn.On("SetWriteDeadline", mock.Anything).Return(nil)
	mockConn.On("Read", mock.Anything).Return(0, net.ErrClosed)
	mockConn.On("Write", mock.Anything).Return(0, nil)
	mockConn.On("Close").Return(nil)

	conn, err := manager.AddConnection(mockConn)
	assert.NoError(t, err)

	// 测试有连接的统计
	stats = manager.GetConnectionStats()
	assert.Equal(t, 1, stats["total_connections"])
	assert.Equal(t, 1, stats["active_connections"])
	assert.Len(t, stats["connection_details"], 1)

	details := stats["connection_details"].([]map[string]interface{})
	assert.Equal(t, conn.ID(), details[0]["id"])
	assert.Equal(t, "connected", details[0]["state"])

	mockLogger.AssertExpectations(t)
	mockMetrics.AssertExpectations(t)
}
