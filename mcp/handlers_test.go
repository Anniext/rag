package mcp

import (
	"context"
	"testing"
	"time"

	"pumppill/rag/core"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockMCPHandler 模拟 MCP 处理器
type MockMCPHandler struct {
	mock.Mock
}

func (m *MockMCPHandler) Handle(ctx context.Context, request *core.MCPRequest) (*core.MCPResponse, error) {
	args := m.Called(ctx, request)
	return args.Get(0).(*core.MCPResponse), args.Error(1)
}

// MockSecurityManager 模拟安全管理器
type MockSecurityManager struct {
	mock.Mock
}

func (m *MockSecurityManager) ValidateToken(ctx context.Context, token string) (*core.UserInfo, error) {
	args := m.Called(ctx, token)
	return args.Get(0).(*core.UserInfo), args.Error(1)
}

func (m *MockSecurityManager) CheckPermission(ctx context.Context, user *core.UserInfo, resource string, action string) error {
	args := m.Called(ctx, user, resource, action)
	return args.Error(0)
}

func (m *MockSecurityManager) ValidateSQL(ctx context.Context, sql string) error {
	args := m.Called(ctx, sql)
	return args.Error(0)
}

func TestNewHandlerRegistry(t *testing.T) {
	mockLogger := &MockLogger{}
	registry := NewHandlerRegistry(mockLogger)

	assert.NotNil(t, registry)
	assert.NotNil(t, registry.handlers)
	assert.Equal(t, 0, len(registry.handlers))
}

func TestHandlerRegistry_Register(t *testing.T) {
	mockLogger := &MockLogger{}
	mockLogger.On("Info", mock.Anything).Maybe().Return()

	registry := NewHandlerRegistry(mockLogger)
	mockHandler := &MockMCPHandler{}

	// 测试注册处理器
	err := registry.Register("test.method", mockHandler)
	assert.NoError(t, err)

	// 测试重复注册
	err = registry.Register("test.method", mockHandler)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")

	mockLogger.AssertExpectations(t)
}

func TestHandlerRegistry_Get(t *testing.T) {
	mockLogger := &MockLogger{}
	mockLogger.On("Info", mock.Anything).Maybe().Return()

	registry := NewHandlerRegistry(mockLogger)
	mockHandler := &MockMCPHandler{}

	// 注册处理器
	registry.Register("test.method", mockHandler)

	// 测试获取存在的处理器
	handler, exists := registry.Get("test.method")
	assert.True(t, exists)
	assert.Equal(t, mockHandler, handler)

	// 测试获取不存在的处理器
	_, exists = registry.Get("nonexistent.method")
	assert.False(t, exists)

	mockLogger.AssertExpectations(t)
}

func TestHandlerRegistry_Unregister(t *testing.T) {
	mockLogger := &MockLogger{}
	mockLogger.On("Info", mock.Anything).Maybe().Return()

	registry := NewHandlerRegistry(mockLogger)
	mockHandler := &MockMCPHandler{}

	// 注册处理器
	registry.Register("test.method", mockHandler)

	// 验证处理器存在
	_, exists := registry.Get("test.method")
	assert.True(t, exists)

	// 注销处理器
	registry.Unregister("test.method")

	// 验证处理器已被移除
	_, exists = registry.Get("test.method")
	assert.False(t, exists)

	mockLogger.AssertExpectations(t)
}

func TestHandlerRegistry_GetAll(t *testing.T) {
	mockLogger := &MockLogger{}
	mockLogger.On("Info", mock.Anything).Maybe().Return()

	registry := NewHandlerRegistry(mockLogger)
	mockHandler1 := &MockMCPHandler{}
	mockHandler2 := &MockMCPHandler{}

	// 注册多个处理器
	registry.Register("method1", mockHandler1)
	registry.Register("method2", mockHandler2)

	// 获取所有方法
	methods := registry.GetAll()
	assert.Len(t, methods, 2)
	assert.Contains(t, methods, "method1")
	assert.Contains(t, methods, "method2")

	mockLogger.AssertExpectations(t)
}

func TestDefaultRequestValidator_Validate(t *testing.T) {
	mockLogger := &MockLogger{}
	validator := NewDefaultRequestValidator(mockLogger)
	ctx := context.Background()

	// 测试有效请求
	validRequest := &core.MCPRequest{
		ID:     "test-id",
		Method: "test.method",
		Params: map[string]interface{}{"key": "value"},
	}
	err := validator.Validate(ctx, validRequest)
	assert.NoError(t, err)

	// 测试 nil 请求
	err = validator.Validate(ctx, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be nil")

	// 测试空 ID
	invalidRequest := &core.MCPRequest{
		ID:     "",
		Method: "test.method",
	}
	err = validator.Validate(ctx, invalidRequest)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ID cannot be empty")

	// 测试空方法
	invalidRequest = &core.MCPRequest{
		ID:     "test-id",
		Method: "",
	}
	err = validator.Validate(ctx, invalidRequest)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "method cannot be empty")

	// 测试方法名过长
	longMethod := make([]byte, 101)
	for i := range longMethod {
		longMethod[i] = 'a'
	}
	invalidRequest = &core.MCPRequest{
		ID:     "test-id",
		Method: string(longMethod),
	}
	err = validator.Validate(ctx, invalidRequest)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "method name too long")
}

func TestTokenBucketRateLimiter_Allow(t *testing.T) {
	mockLogger := &MockLogger{}
	limiter := NewTokenBucketRateLimiter(10, 10, time.Second, mockLogger)
	ctx := context.Background()

	// 测试初始允许
	allowed, err := limiter.Allow(ctx, "test-key")
	assert.NoError(t, err)
	assert.True(t, allowed)

	// 消耗所有令牌
	for i := 0; i < 9; i++ {
		allowed, err = limiter.Allow(ctx, "test-key")
		assert.NoError(t, err)
		assert.True(t, allowed)
	}

	// 应该被限制
	allowed, err = limiter.Allow(ctx, "test-key")
	assert.NoError(t, err)
	assert.False(t, allowed)

	// 测试不同的键
	allowed, err = limiter.Allow(ctx, "different-key")
	assert.NoError(t, err)
	assert.True(t, allowed)
}

func TestTokenBucketRateLimiter_Reset(t *testing.T) {
	mockLogger := &MockLogger{}
	limiter := NewTokenBucketRateLimiter(1, 1, time.Second, mockLogger)
	ctx := context.Background()

	// 消耗令牌
	allowed, err := limiter.Allow(ctx, "test-key")
	assert.NoError(t, err)
	assert.True(t, allowed)

	// 应该被限制
	allowed, err = limiter.Allow(ctx, "test-key")
	assert.NoError(t, err)
	assert.False(t, allowed)

	// 重置限流器
	err = limiter.Reset(ctx, "test-key")
	assert.NoError(t, err)

	// 应该再次允许
	allowed, err = limiter.Allow(ctx, "test-key")
	assert.NoError(t, err)
	assert.True(t, allowed)
}

func TestRequestProcessor_ProcessRequest(t *testing.T) {
	mockLogger := &MockLogger{}
	mockLogger.On("Info", mock.Anything).Maybe().Return()
	mockLogger.On("Error", mock.Anything).Maybe().Return()
	mockLogger.On("Warn", mock.Anything).Maybe().Return()

	mockMetrics := &MockMetricsCollector{}
	mockMetrics.On("IncrementCounter", mock.Anything, mock.Anything).Maybe().Return()
	mockMetrics.On("RecordHistogram", mock.Anything, mock.Anything, mock.Anything).Maybe().Return()

	registry := NewHandlerRegistry(mockLogger)
	validator := NewDefaultRequestValidator(mockLogger)
	rateLimiter := NewTokenBucketRateLimiter(10, 10, time.Second, mockLogger)

	processor := NewRequestProcessor(
		registry,
		validator,
		rateLimiter,
		nil, // 不使用安全管理器
		mockMetrics,
		mockLogger,
		30*time.Second,
	)

	// 注册测试处理器
	mockHandler := &MockMCPHandler{}
	expectedResponse := &core.MCPResponse{
		ID:     "test-id",
		Result: "test result",
	}
	mockHandler.On("Handle", mock.Anything, mock.Anything).Return(expectedResponse, nil)
	registry.Register("test.method", mockHandler)

	// 测试成功处理
	request := &core.MCPRequest{
		ID:     "test-id",
		Method: "test.method",
		Params: map[string]interface{}{"key": "value"},
	}

	response := processor.ProcessRequest(context.Background(), request, "conn-1")
	assert.NotNil(t, response)
	assert.Equal(t, "test-id", response.ID)
	assert.Equal(t, "test result", response.Result)
	assert.Nil(t, response.Error)

	mockHandler.AssertExpectations(t)
	mockLogger.AssertExpectations(t)
	mockMetrics.AssertExpectations(t)
}

func TestRequestProcessor_ProcessRequest_ValidationError(t *testing.T) {
	mockLogger := &MockLogger{}
	mockLogger.On("Info", mock.Anything).Maybe().Return()
	mockLogger.On("Error", mock.Anything).Maybe().Return()

	mockMetrics := &MockMetricsCollector{}
	mockMetrics.On("IncrementCounter", mock.Anything, mock.Anything).Maybe().Return()

	registry := NewHandlerRegistry(mockLogger)
	validator := NewDefaultRequestValidator(mockLogger)

	processor := NewRequestProcessor(
		registry,
		validator,
		nil, // 不使用限流器
		nil, // 不使用安全管理器
		mockMetrics,
		mockLogger,
		30*time.Second,
	)

	// 测试无效请求
	invalidRequest := &core.MCPRequest{
		ID:     "", // 空 ID
		Method: "test.method",
	}

	response := processor.ProcessRequest(context.Background(), invalidRequest, "conn-1")
	assert.NotNil(t, response)
	assert.Equal(t, "", response.ID)
	assert.NotNil(t, response.Error)
	assert.Equal(t, 400, response.Error.Code)
	assert.Equal(t, "Invalid request", response.Error.Message)

	mockLogger.AssertExpectations(t)
	mockMetrics.AssertExpectations(t)
}

func TestRequestProcessor_ProcessRequest_HandlerNotFound(t *testing.T) {
	mockLogger := &MockLogger{}
	mockLogger.On("Info", mock.Anything).Maybe().Return()
	mockLogger.On("Warn", mock.Anything).Maybe().Return()

	mockMetrics := &MockMetricsCollector{}
	mockMetrics.On("IncrementCounter", mock.Anything, mock.Anything).Maybe().Return()

	registry := NewHandlerRegistry(mockLogger)
	validator := NewDefaultRequestValidator(mockLogger)

	processor := NewRequestProcessor(
		registry,
		validator,
		nil, // 不使用限流器
		nil, // 不使用安全管理器
		mockMetrics,
		mockLogger,
		30*time.Second,
	)

	// 测试不存在的方法
	request := &core.MCPRequest{
		ID:     "test-id",
		Method: "nonexistent.method",
	}

	response := processor.ProcessRequest(context.Background(), request, "conn-1")
	assert.NotNil(t, response)
	assert.Equal(t, "test-id", response.ID)
	assert.NotNil(t, response.Error)
	assert.Equal(t, 404, response.Error.Code)
	assert.Equal(t, "Method not found", response.Error.Message)

	mockLogger.AssertExpectations(t)
	mockMetrics.AssertExpectations(t)
}

func TestAsyncRequestProcessor(t *testing.T) {
	mockLogger := &MockLogger{}
	mockLogger.On("Info", mock.Anything).Maybe().Return()
	mockLogger.On("Error", mock.Anything).Maybe().Return()

	mockMetrics := &MockMetricsCollector{}
	mockMetrics.On("IncrementCounter", mock.Anything, mock.Anything).Maybe().Return()
	mockMetrics.On("RecordHistogram", mock.Anything, mock.Anything, mock.Anything).Maybe().Return()

	registry := NewHandlerRegistry(mockLogger)
	validator := NewDefaultRequestValidator(mockLogger)

	processor := NewRequestProcessor(
		registry,
		validator,
		nil, // 不使用限流器
		nil, // 不使用安全管理器
		mockMetrics,
		mockLogger,
		30*time.Second,
	)

	// 注册测试处理器
	mockHandler := &MockMCPHandler{}
	expectedResponse := &core.MCPResponse{
		ID:     "test-id",
		Result: "test result",
	}
	mockHandler.On("Handle", mock.Anything, mock.Anything).Return(expectedResponse, nil)
	registry.Register("test.method", mockHandler)

	// 创建异步处理器
	asyncProcessor := NewAsyncRequestProcessor(processor, 2, mockLogger)
	asyncProcessor.Start()
	defer asyncProcessor.Stop()

	// 测试异步处理
	request := &core.MCPRequest{
		ID:     "test-id",
		Method: "test.method",
		Params: map[string]interface{}{"key": "value"},
	}

	responseChan := asyncProcessor.ProcessAsync(request, "conn-1")

	select {
	case response := <-responseChan:
		assert.NotNil(t, response)
		assert.Equal(t, "test-id", response.ID)
		assert.Equal(t, "test result", response.Result)
		assert.Nil(t, response.Error)
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for response")
	}

	mockHandler.AssertExpectations(t)
	mockLogger.AssertExpectations(t)
	mockMetrics.AssertExpectations(t)
}

func TestMiddlewareChain(t *testing.T) {
	mockLogger := &MockLogger{}
	mockLogger.On("Info", mock.Anything).Maybe().Return()
	mockLogger.On("Error", mock.Anything).Maybe().Return()

	mockMetrics := &MockMetricsCollector{}
	mockMetrics.On("IncrementCounter", mock.Anything, mock.Anything).Maybe().Return()
	mockMetrics.On("RecordHistogram", mock.Anything, mock.Anything, mock.Anything).Maybe().Return()

	// 创建基础处理器
	mockHandler := &MockMCPHandler{}
	expectedResponse := &core.MCPResponse{
		ID:     "test-id",
		Result: "test result",
	}
	mockHandler.On("Handle", mock.Anything, mock.Anything).Return(expectedResponse, nil)

	// 创建中间件链
	chain := NewMiddlewareChain(
		LoggingMiddleware(mockLogger),
		MetricsMiddleware(mockMetrics),
		RecoveryMiddleware(mockLogger),
	)

	// 应用中间件
	wrappedHandler := chain.Apply(mockHandler)

	// 测试处理请求
	request := &core.MCPRequest{
		ID:     "test-id",
		Method: "test.method",
		Params: map[string]interface{}{"key": "value"},
	}

	response, err := wrappedHandler.Handle(context.Background(), request)
	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, "test-id", response.ID)
	assert.Equal(t, "test result", response.Result)

	mockHandler.AssertExpectations(t)
	mockLogger.AssertExpectations(t)
	mockMetrics.AssertExpectations(t)
}

func TestRecoveryMiddleware_Panic(t *testing.T) {
	mockLogger := &MockLogger{}
	mockLogger.On("Error", mock.Anything).Maybe().Return()

	// 创建会 panic 的处理器
	panicHandler := &MockMCPHandler{}
	panicHandler.On("Handle", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		panic("test panic")
	}).Return((*core.MCPResponse)(nil), nil)

	// 应用恢复中间件
	wrappedHandler := RecoveryMiddleware(mockLogger)(panicHandler)

	// 测试处理请求
	request := &core.MCPRequest{
		ID:     "test-id",
		Method: "test.method",
	}

	response, err := wrappedHandler.Handle(context.Background(), request)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "handler panic")
	assert.NotNil(t, response)
	assert.Equal(t, "test-id", response.ID)
	assert.NotNil(t, response.Error)
	assert.Equal(t, 500, response.Error.Code)

	panicHandler.AssertExpectations(t)
	mockLogger.AssertExpectations(t)
}
