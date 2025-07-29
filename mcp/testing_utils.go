package mcp

import (
	"context"

	"pumppill/rag/core"

	"github.com/stretchr/testify/mock"
)

// TestMockLogger 测试用模拟日志记录器
type TestMockLogger struct {
	mock.Mock
}

func (m *TestMockLogger) Debug(msg string, fields ...interface{}) {
	m.Called(msg)
}

func (m *TestMockLogger) Info(msg string, fields ...interface{}) {
	m.Called(msg)
}

func (m *TestMockLogger) Warn(msg string, fields ...interface{}) {
	m.Called(msg)
}

func (m *TestMockLogger) Error(msg string, fields ...interface{}) {
	m.Called(msg)
}

func (m *TestMockLogger) Fatal(msg string, fields ...interface{}) {
	m.Called(msg)
}

// TestMockMetricsCollector 测试用模拟指标收集器
type TestMockMetricsCollector struct {
	mock.Mock
}

func (m *TestMockMetricsCollector) IncrementCounter(name string, labels map[string]string) {
	m.Called(name, labels)
}

func (m *TestMockMetricsCollector) RecordHistogram(name string, value float64, labels map[string]string) {
	m.Called(name, value, labels)
}

func (m *TestMockMetricsCollector) SetGauge(name string, value float64, labels map[string]string) {
	m.Called(name, value, labels)
}

// TestMockHandler 测试用模拟处理器
type TestMockHandler struct {
	mock.Mock
	result any
	err    error
}

func (h *TestMockHandler) Handle(ctx context.Context, request *core.MCPRequest) (*core.MCPResponse, error) {
	args := h.Called(ctx, request)
	return args.Get(0).(*core.MCPResponse), args.Error(1)
}

// NewTestMockHandler 创建测试用模拟处理器
func NewTestMockHandler(result any, err error) *TestMockHandler {
	return &TestMockHandler{
		result: result,
		err:    err,
	}
}

// SimpleMockLogger 简单的模拟日志记录器（不使用 testify/mock）
type SimpleMockLogger struct{}

func (l *SimpleMockLogger) Debug(msg string, fields ...interface{}) {}
func (l *SimpleMockLogger) Info(msg string, fields ...interface{})  {}
func (l *SimpleMockLogger) Warn(msg string, fields ...interface{})  {}
func (l *SimpleMockLogger) Error(msg string, fields ...interface{}) {}
func (l *SimpleMockLogger) Fatal(msg string, fields ...interface{}) {}

// SimpleMockMetricsCollector 简单的模拟指标收集器（不使用 testify/mock）
type SimpleMockMetricsCollector struct{}

func (m *SimpleMockMetricsCollector) IncrementCounter(name string, labels map[string]string) {}
func (m *SimpleMockMetricsCollector) RecordHistogram(name string, value float64, labels map[string]string) {
}
func (m *SimpleMockMetricsCollector) SetGauge(name string, value float64, labels map[string]string) {}

// SimpleMockHandler 简单的模拟处理器（不使用 testify/mock）
type SimpleMockHandler struct {
	result any
	err    error
}

func (h *SimpleMockHandler) Handle(ctx context.Context, request *core.MCPRequest) (*core.MCPResponse, error) {
	if h.err != nil {
		return &core.MCPResponse{
			ID: request.ID,
			Error: &core.MCPError{
				Code:    500,
				Message: h.err.Error(),
			},
		}, h.err
	}

	return &core.MCPResponse{
		ID:     request.ID,
		Result: h.result,
	}, nil
}
