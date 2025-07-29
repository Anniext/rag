package core

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewRAGError 测试创建 RAG 错误
func TestNewRAGError(t *testing.T) {
	err := NewRAGError(ErrorTypeValidation, "TEST_ERROR", "测试错误")

	assert.Equal(t, ErrorTypeValidation, err.Type)
	assert.Equal(t, "TEST_ERROR", err.Code)
	assert.Equal(t, "测试错误", err.Message)
	assert.NotZero(t, err.Timestamp)
	assert.Nil(t, err.Details)
	assert.Nil(t, err.Cause)
	assert.Empty(t, err.RequestID)
	assert.Empty(t, err.UserID)
}

// TestRAGErrorError 测试错误接口实现
func TestRAGErrorError(t *testing.T) {
	err := NewRAGError(ErrorTypeDatabase, "DB_ERROR", "数据库错误")
	expected := "[database:DB_ERROR] 数据库错误"
	assert.Equal(t, expected, err.Error())
}

// TestRAGErrorChaining 测试错误链式调用
func TestRAGErrorChaining(t *testing.T) {
	originalErr := errors.New("原始错误")
	details := map[string]any{
		"field": "value",
		"count": 42,
	}

	err := NewRAGError(ErrorTypeValidation, "CHAIN_ERROR", "链式错误").
		WithDetails(details).
		WithCause(originalErr).
		WithRequestID("req_123").
		WithUserID("user_456")

	assert.Equal(t, ErrorTypeValidation, err.Type)
	assert.Equal(t, "CHAIN_ERROR", err.Code)
	assert.Equal(t, "链式错误", err.Message)
	assert.Equal(t, details, err.Details)
	assert.Equal(t, originalErr, err.Cause)
	assert.Equal(t, "req_123", err.RequestID)
	assert.Equal(t, "user_456", err.UserID)
}

// TestRAGErrorUnwrap 测试错误解包
func TestRAGErrorUnwrap(t *testing.T) {
	originalErr := errors.New("原始错误")
	ragErr := NewRAGError(ErrorTypeInternal, "WRAP_ERROR", "包装错误").
		WithCause(originalErr)

	unwrapped := ragErr.Unwrap()
	assert.Equal(t, originalErr, unwrapped)

	// 测试没有原因错误的情况
	ragErr2 := NewRAGError(ErrorTypeValidation, "NO_CAUSE", "无原因错误")
	assert.Nil(t, ragErr2.Unwrap())
}

// TestIsRAGError 测试 RAG 错误检查
func TestIsRAGError(t *testing.T) {
	// 测试 RAG 错误
	ragErr := NewRAGError(ErrorTypeValidation, "TEST_ERROR", "测试错误")
	assert.True(t, IsRAGError(ragErr))

	// 测试普通错误
	normalErr := errors.New("普通错误")
	assert.False(t, IsRAGError(normalErr))

	// 测试包装的 RAG 错误
	wrappedErr := errors.New("包装错误")
	ragErr2 := NewRAGError(ErrorTypeInternal, "WRAPPED", "包装的RAG错误").WithCause(wrappedErr)
	assert.True(t, IsRAGError(ragErr2))
}

// TestGetRAGError 测试获取 RAG 错误
func TestGetRAGError(t *testing.T) {
	// 测试 RAG 错误
	originalRAGErr := NewRAGError(ErrorTypeDatabase, "DB_ERROR", "数据库错误")
	retrievedErr := GetRAGError(originalRAGErr)
	require.NotNil(t, retrievedErr)
	assert.Equal(t, originalRAGErr, retrievedErr)

	// 测试普通错误
	normalErr := errors.New("普通错误")
	retrievedErr = GetRAGError(normalErr)
	assert.Nil(t, retrievedErr)

	// 测试 nil 错误
	retrievedErr = GetRAGError(nil)
	assert.Nil(t, retrievedErr)
}

// TestWrapError 测试错误包装
func TestWrapError(t *testing.T) {
	originalErr := errors.New("原始错误")
	wrappedErr := WrapError(originalErr, ErrorTypeLLM, "LLM_WRAP", "包装的LLM错误")

	assert.Equal(t, ErrorTypeLLM, wrappedErr.Type)
	assert.Equal(t, "LLM_WRAP", wrappedErr.Code)
	assert.Equal(t, "包装的LLM错误", wrappedErr.Message)
	assert.Equal(t, originalErr, wrappedErr.Cause)
	assert.NotZero(t, wrappedErr.Timestamp)

	// 测试错误链
	assert.True(t, errors.Is(wrappedErr, originalErr))
}

// TestPredefinedErrors 测试预定义错误
func TestPredefinedErrors(t *testing.T) {
	testCases := []struct {
		err          *RAGError
		expectedType ErrorType
		expectedCode string
	}{
		{ErrInvalidQuery, ErrorTypeValidation, "INVALID_QUERY"},
		{ErrDatabaseConnection, ErrorTypeDatabase, "DB_CONNECTION_FAILED"},
		{ErrLLMTimeout, ErrorTypeLLM, "LLM_TIMEOUT"},
		{ErrMCPInvalidMessage, ErrorTypeMCP, "MCP_INVALID_MESSAGE"},
		{ErrUnauthorized, ErrorTypePermission, "UNAUTHORIZED"},
		{ErrResourceNotFound, ErrorTypeNotFound, "RESOURCE_NOT_FOUND"},
		{ErrResourceConflict, ErrorTypeConflict, "RESOURCE_CONFLICT"},
		{ErrRequestTimeout, ErrorTypeTimeout, "REQUEST_TIMEOUT"},
		{ErrRateLimitExceeded, ErrorTypeRateLimit, "RATE_LIMIT_EXCEEDED"},
		{ErrSchemaNotLoaded, ErrorTypeSchema, "SCHEMA_NOT_LOADED"},
		{ErrCacheConnection, ErrorTypeCache, "CACHE_CONNECTION_FAILED"},
		{ErrInternalServer, ErrorTypeInternal, "INTERNAL_SERVER_ERROR"},
	}

	for _, tc := range testCases {
		assert.Equal(t, tc.expectedType, tc.err.Type, "错误类型不匹配: %s", tc.err.Code)
		assert.Equal(t, tc.expectedCode, tc.err.Code, "错误码不匹配: %s", tc.err.Code)
		assert.NotEmpty(t, tc.err.Message, "错误消息不能为空: %s", tc.err.Code)
		assert.NotZero(t, tc.err.Timestamp, "时间戳不能为零: %s", tc.err.Code)
	}
}

// TestErrorHandler 测试错误处理器
func TestErrorHandler(t *testing.T) {
	// 创建 mock logger 和 metrics
	mockLogger := &MockLoggerExtended{}
	mockMetrics := &MockMetricsCollector{}

	handler := NewErrorHandler(mockLogger, mockMetrics)
	assert.NotNil(t, handler)

	// 测试处理 RAG 错误
	ragErr := NewRAGError(ErrorTypeValidation, "TEST_ERROR", "测试错误").
		WithRequestID("req_123").
		WithUserID("user_456").
		WithDetails(map[string]any{"field": "value"})

	response := handler.HandleError(ragErr, "req_123")

	// 验证响应
	require.NotNil(t, response)
	assert.Equal(t, "req_123", response.ID)
	require.NotNil(t, response.Error)
	assert.Equal(t, 400, response.Error.Code) // 验证错误对应 HTTP 状态码
	assert.Equal(t, "测试错误", response.Error.Message)
	assert.Equal(t, ragErr.Details, response.Error.Data)

	// 验证日志记录
	assert.True(t, mockLogger.ErrorCalled)
	assert.Contains(t, mockLogger.LastErrorMsg, "RAG error occurred")

	// 验证指标记录
	assert.True(t, mockMetrics.IncrementCounterCalled)
	assert.Equal(t, "rag_errors_total", mockMetrics.LastCounterName)
	assert.Equal(t, "validation", mockMetrics.LastCounterLabels["type"])
	assert.Equal(t, "TEST_ERROR", mockMetrics.LastCounterLabels["code"])
}

// TestErrorHandlerWithNormalError 测试处理普通错误
func TestErrorHandlerWithNormalError(t *testing.T) {
	mockLogger := &MockLoggerExtended{}
	mockMetrics := &MockMetricsCollector{}

	handler := NewErrorHandler(mockLogger, mockMetrics)

	// 测试处理普通错误
	normalErr := errors.New("普通错误")
	response := handler.HandleError(normalErr, "req_456")

	// 验证响应
	require.NotNil(t, response)
	assert.Equal(t, "req_456", response.ID)
	require.NotNil(t, response.Error)
	assert.Equal(t, 500, response.Error.Code) // 内部错误
	assert.Equal(t, "内部服务器错误", response.Error.Message)

	// 验证日志记录
	assert.True(t, mockLogger.ErrorCalled)

	// 验证指标记录
	assert.True(t, mockMetrics.IncrementCounterCalled)
	assert.Equal(t, "internal", mockMetrics.LastCounterLabels["type"])
	assert.Equal(t, "INTERNAL_ERROR", mockMetrics.LastCounterLabels["code"])
}

// TestGetHTTPStatusCode 测试 HTTP 状态码映射
func TestGetHTTPStatusCode(t *testing.T) {
	handler := &ErrorHandler{}

	testCases := []struct {
		errorType    ErrorType
		expectedCode int
	}{
		{ErrorTypeValidation, 400},
		{ErrorTypeAuth, 401},
		{ErrorTypePermission, 401},
		{ErrorTypeNotFound, 404},
		{ErrorTypeConflict, 409},
		{ErrorTypeTimeout, 408},
		{ErrorTypeRateLimit, 429},
		{ErrorTypeDatabase, 500},
		{ErrorTypeLLM, 500},
		{ErrorTypeMCP, 500},
		{ErrorTypeInternal, 500},
		{ErrorTypeSchema, 500},
		{ErrorTypeCache, 500},
	}

	for _, tc := range testCases {
		code := handler.getHTTPStatusCode(tc.errorType)
		assert.Equal(t, tc.expectedCode, code, "错误类型 %s 的状态码不匹配", tc.errorType)
	}
}

// TestErrorHandlerWithNilMetrics 测试没有指标收集器的情况
func TestErrorHandlerWithNilMetrics(t *testing.T) {
	mockLogger := &MockLoggerExtended{}
	handler := NewErrorHandler(mockLogger, nil)

	ragErr := NewRAGError(ErrorTypeValidation, "TEST_ERROR", "测试错误")
	response := handler.HandleError(ragErr, "req_123")

	// 应该正常工作，不会 panic
	require.NotNil(t, response)
	assert.Equal(t, "req_123", response.ID)
}

// TestErrorTimestamp 测试错误时间戳
func TestErrorTimestamp(t *testing.T) {
	before := time.Now()
	err := NewRAGError(ErrorTypeValidation, "TIME_TEST", "时间测试")
	after := time.Now()

	assert.True(t, err.Timestamp.After(before) || err.Timestamp.Equal(before))
	assert.True(t, err.Timestamp.Before(after) || err.Timestamp.Equal(after))
}

// TestErrorSerialization 测试错误序列化
func TestErrorSerialization(t *testing.T) {
	err := NewRAGError(ErrorTypeDatabase, "SERIALIZE_TEST", "序列化测试").
		WithDetails(map[string]any{
			"table": "users",
			"count": 100,
		}).
		WithRequestID("req_serialize").
		WithUserID("user_serialize")

	// 验证 JSON 标签
	assert.Equal(t, ErrorTypeDatabase, err.Type)
	assert.Equal(t, "SERIALIZE_TEST", err.Code)
	assert.Equal(t, "序列化测试", err.Message)
	assert.NotNil(t, err.Details)
	assert.Equal(t, "req_serialize", err.RequestID)
	assert.Equal(t, "user_serialize", err.UserID)
	assert.NotZero(t, err.Timestamp)

	// Cause 字段不应该被序列化（json:"-" 标签）
	originalErr := errors.New("原始错误")
	err.WithCause(originalErr)
	assert.Equal(t, originalErr, err.Cause)
}

// MockMetricsCollector 用于测试的 mock 指标收集器
type MockMetricsCollector struct {
	IncrementCounterCalled bool
	RecordHistogramCalled  bool
	SetGaugeCalled         bool

	LastCounterName   string
	LastCounterLabels map[string]string

	LastHistogramName   string
	LastHistogramValue  float64
	LastHistogramLabels map[string]string

	LastGaugeName   string
	LastGaugeValue  float64
	LastGaugeLabels map[string]string
}

func (m *MockMetricsCollector) IncrementCounter(name string, labels map[string]string) {
	m.IncrementCounterCalled = true
	m.LastCounterName = name
	m.LastCounterLabels = labels
}

func (m *MockMetricsCollector) RecordHistogram(name string, value float64, labels map[string]string) {
	m.RecordHistogramCalled = true
	m.LastHistogramName = name
	m.LastHistogramValue = value
	m.LastHistogramLabels = labels
}

func (m *MockMetricsCollector) SetGauge(name string, value float64, labels map[string]string) {
	m.SetGaugeCalled = true
	m.LastGaugeName = name
	m.LastGaugeValue = value
	m.LastGaugeLabels = labels
}

// 扩展 MockLogger 以支持错误处理测试
type MockLoggerExtended struct {
	MockLogger
	ErrorCalled  bool
	LastErrorMsg string
	LastFields   []any
}

func (m *MockLoggerExtended) Error(msg string, fields ...any) {
	m.ErrorCalled = true
	m.LastErrorMsg = msg
	m.LastFields = fields
	if m.ErrorFunc != nil {
		m.ErrorFunc(msg, fields...)
	}
}

// 基准测试
func BenchmarkNewRAGError(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewRAGError(ErrorTypeValidation, "BENCH_ERROR", "基准测试错误")
	}
}

func BenchmarkRAGErrorChaining(b *testing.B) {
	originalErr := errors.New("原始错误")
	details := map[string]any{"field": "value"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewRAGError(ErrorTypeValidation, "CHAIN_ERROR", "链式错误").
			WithDetails(details).
			WithCause(originalErr).
			WithRequestID("req_123").
			WithUserID("user_456")
	}
}

func BenchmarkIsRAGError(b *testing.B) {
	ragErr := NewRAGError(ErrorTypeValidation, "BENCH_ERROR", "基准测试错误")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		IsRAGError(ragErr)
	}
}

func BenchmarkErrorHandling(b *testing.B) {
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsCollector{}
	handler := NewErrorHandler(mockLogger, mockMetrics)

	ragErr := NewRAGError(ErrorTypeValidation, "BENCH_ERROR", "基准测试错误")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		handler.HandleError(ragErr, "req_bench")
	}
}
