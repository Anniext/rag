package core

import (
	"errors"
	"fmt"
	"time"
)

// ErrorType 错误类型枚举
// ErrorType 表示错误类型的字符串枚举，用于区分不同的错误来源和类别
type ErrorType string

// 错误类型枚举常量定义
const (
	ErrorTypeValidation ErrorType = "validation" // 校验错误，如参数不合法、格式错误等
	ErrorTypeDatabase   ErrorType = "database"   // 数据库相关错误，如连接失败、查询异常等
	ErrorTypeLLM        ErrorType = "llm"        // 大模型（LLM）相关错误，如请求超时、响应无效等
	ErrorTypeMCP        ErrorType = "mcp"        // MCP（消息控制协议）相关错误
	ErrorTypeTimeout    ErrorType = "timeout"    // 超时错误，如请求或操作超时
	ErrorTypePermission ErrorType = "permission" // 权限错误，如未授权、权限不足等
	ErrorTypeInternal   ErrorType = "internal"   // 内部服务器错误
	ErrorTypeNotFound   ErrorType = "not_found"  // 资源未找到错误
	ErrorTypeConflict   ErrorType = "conflict"   // 资源冲突错误，如重复、已存在等
	ErrorTypeRateLimit  ErrorType = "rate_limit" // 频率限制错误，如请求过快
	ErrorTypeSchema     ErrorType = "schema"     // Schema 相关错误，如未加载、无效等
	ErrorTypeCache      ErrorType = "cache"      // 缓存相关错误，如连接失败、键不存在等
	ErrorTypeAuth       ErrorType = "auth"       // 认证相关错误，如令牌无效、过期等
)

// RAGError 表示 RAG 系统中的错误结构体，包含错误的详细信息
type RAGError struct {
	Type      ErrorType      `json:"type"`                 // 错误类型，用于区分错误来源（如校验、数据库等）
	Code      string         `json:"code"`                 // 错误码，唯一标识具体错误
	Message   string         `json:"message"`              // 错误信息，描述错误内容
	Details   map[string]any `json:"details,omitempty"`    // 错误详情，包含额外的上下文信息（可选）
	Cause     error          `json:"-"`                    // 原始错误对象，用于错误链追踪（不序列化）
	Timestamp time.Time      `json:"timestamp"`            // 错误发生时间
	RequestID string         `json:"request_id,omitempty"` // 请求 ID，便于追踪（可选）
	UserID    string         `json:"user_id,omitempty"`    // 用户 ID，关联用户（可选）
}

// Error 实现 error 接口
func (e *RAGError) Error() string {
	return fmt.Sprintf("[%s:%s] %s", e.Type, e.Code, e.Message)
}

// Unwrap 支持错误链
func (e *RAGError) Unwrap() error {
	return e.Cause
}

// NewRAGError 创建新的 RAG 错误
func NewRAGError(errorType ErrorType, code, message string) *RAGError {
	return &RAGError{
		Type:      errorType,
		Code:      code,
		Message:   message,
		Timestamp: time.Now(),
	}
}

// WithDetails 添加错误详情
func (e *RAGError) WithDetails(details map[string]any) *RAGError {
	e.Details = details
	return e
}

// WithCause 添加原因错误
func (e *RAGError) WithCause(cause error) *RAGError {
	e.Cause = cause
	return e
}

// WithRequestID 添加请求ID
func (e *RAGError) WithRequestID(requestID string) *RAGError {
	e.RequestID = requestID
	return e
}

// WithUserID 添加用户ID
func (e *RAGError) WithUserID(userID string) *RAGError {
	e.UserID = userID
	return e
}

// 预定义的错误变量
var (
	// 验证错误
	ErrInvalidQuery     = NewRAGError(ErrorTypeValidation, "INVALID_QUERY", "查询语句无效")
	ErrMissingParameter = NewRAGError(ErrorTypeValidation, "MISSING_PARAMETER", "缺少必需参数")
	ErrInvalidParameter = NewRAGError(ErrorTypeValidation, "INVALID_PARAMETER", "参数值无效")
	ErrQueryTooLong     = NewRAGError(ErrorTypeValidation, "QUERY_TOO_LONG", "查询语句过长")
	ErrInvalidFormat    = NewRAGError(ErrorTypeValidation, "INVALID_FORMAT", "格式无效")

	// 数据库错误
	ErrDatabaseConnection = NewRAGError(ErrorTypeDatabase, "DB_CONNECTION_FAILED", "数据库连接失败")
	ErrDatabaseQuery      = NewRAGError(ErrorTypeDatabase, "DB_QUERY_FAILED", "数据库查询失败")
	ErrDatabaseTimeout    = NewRAGError(ErrorTypeDatabase, "DB_TIMEOUT", "数据库查询超时")
	ErrTableNotFound      = NewRAGError(ErrorTypeDatabase, "TABLE_NOT_FOUND", "表不存在")
	ErrColumnNotFound     = NewRAGError(ErrorTypeDatabase, "COLUMN_NOT_FOUND", "列不存在")
	ErrInvalidSQL         = NewRAGError(ErrorTypeDatabase, "INVALID_SQL", "SQL 语句无效")

	// LLM 错误
	ErrLLMConnection      = NewRAGError(ErrorTypeLLM, "LLM_CONNECTION_FAILED", "LLM 连接失败")
	ErrLLMTimeout         = NewRAGError(ErrorTypeLLM, "LLM_TIMEOUT", "LLM 请求超时")
	ErrLLMRateLimit       = NewRAGError(ErrorTypeLLM, "LLM_RATE_LIMIT", "LLM 请求频率超限")
	ErrLLMInvalidResponse = NewRAGError(ErrorTypeLLM, "LLM_INVALID_RESPONSE", "LLM 响应无效")
	ErrLLMQuotaExceeded   = NewRAGError(ErrorTypeLLM, "LLM_QUOTA_EXCEEDED", "LLM 配额已用完")

	// MCP 错误
	ErrMCPInvalidMessage   = NewRAGError(ErrorTypeMCP, "MCP_INVALID_MESSAGE", "MCP 消息格式无效")
	ErrMCPMethodNotFound   = NewRAGError(ErrorTypeMCP, "MCP_METHOD_NOT_FOUND", "MCP 方法不存在")
	ErrMCPConnectionClosed = NewRAGError(ErrorTypeMCP, "MCP_CONNECTION_CLOSED", "MCP 连接已关闭")
	ErrMCPTimeout          = NewRAGError(ErrorTypeMCP, "MCP_TIMEOUT", "MCP 请求超时")

	// 权限错误
	ErrUnauthorized = NewRAGError(ErrorTypePermission, "UNAUTHORIZED", "未授权访问")
	ErrForbidden    = NewRAGError(ErrorTypePermission, "FORBIDDEN", "权限不足")
	ErrInvalidToken = NewRAGError(ErrorTypePermission, "INVALID_TOKEN", "令牌无效")
	ErrTokenExpired = NewRAGError(ErrorTypePermission, "TOKEN_EXPIRED", "令牌已过期")

	// 资源错误
	ErrResourceNotFound = NewRAGError(ErrorTypeNotFound, "RESOURCE_NOT_FOUND", "资源不存在")
	ErrSessionNotFound  = NewRAGError(ErrorTypeNotFound, "SESSION_NOT_FOUND", "会话不存在")
	ErrUserNotFound     = NewRAGError(ErrorTypeNotFound, "USER_NOT_FOUND", "用户不存在")

	// 冲突错误
	ErrResourceConflict = NewRAGError(ErrorTypeConflict, "RESOURCE_CONFLICT", "资源冲突")
	ErrSessionExists    = NewRAGError(ErrorTypeConflict, "SESSION_EXISTS", "会话已存在")

	// 超时错误
	ErrRequestTimeout = NewRAGError(ErrorTypeTimeout, "REQUEST_TIMEOUT", "请求超时")
	ErrQueryTimeout   = NewRAGError(ErrorTypeTimeout, "QUERY_TIMEOUT", "查询超时")

	// 频率限制错误
	ErrRateLimitExceeded = NewRAGError(ErrorTypeRateLimit, "RATE_LIMIT_EXCEEDED", "请求频率超限")

	// Schema 错误
	ErrSchemaNotLoaded     = NewRAGError(ErrorTypeSchema, "SCHEMA_NOT_LOADED", "Schema 未加载")
	ErrSchemaInvalid       = NewRAGError(ErrorTypeSchema, "SCHEMA_INVALID", "Schema 无效")
	ErrSchemaRefreshFailed = NewRAGError(ErrorTypeSchema, "SCHEMA_REFRESH_FAILED", "Schema 刷新失败")

	// 缓存错误
	ErrCacheConnection  = NewRAGError(ErrorTypeCache, "CACHE_CONNECTION_FAILED", "缓存连接失败")
	ErrCacheKeyNotFound = NewRAGError(ErrorTypeCache, "CACHE_KEY_NOT_FOUND", "缓存键不存在")
	ErrCacheTimeout     = NewRAGError(ErrorTypeCache, "CACHE_TIMEOUT", "缓存操作超时")

	// 内部错误
	ErrInternalServer     = NewRAGError(ErrorTypeInternal, "INTERNAL_SERVER_ERROR", "内部服务器错误")
	ErrConfigurationError = NewRAGError(ErrorTypeInternal, "CONFIGURATION_ERROR", "配置错误")
	ErrServiceUnavailable = NewRAGError(ErrorTypeInternal, "SERVICE_UNAVAILABLE", "服务不可用")
)

// IsRAGError 检查是否为 RAG 错误
func IsRAGError(err error) bool {
	var RAGError *RAGError
	ok := errors.As(err, &RAGError)
	return ok
}

// GetRAGError 获取 RAG 错误
func GetRAGError(err error) *RAGError {
	var ragErr *RAGError
	if errors.As(err, &ragErr) {
		return ragErr
	}
	return nil
}

// WrapError 包装普通错误为 RAG 错误
func WrapError(err error, errorType ErrorType, code, message string) *RAGError {
	return &RAGError{
		Type:      errorType,
		Code:      code,
		Message:   message,
		Cause:     err,
		Timestamp: time.Now(),
	}
}

// ErrorHandler 错误处理器结构
type ErrorHandler struct {
	logger  Logger
	metrics MetricsCollector
}

// NewErrorHandler 创建错误处理器
func NewErrorHandler(logger Logger, metrics MetricsCollector) *ErrorHandler {
	return &ErrorHandler{
		logger:  logger,
		metrics: metrics,
	}
}

// HandleError 处理错误
func (h *ErrorHandler) HandleError(err error, requestID string) *MCPResponse {
	var ragErr *RAGError
	ok := errors.As(err, &ragErr)
	if !ok {
		ragErr = &RAGError{
			Type:      ErrorTypeInternal,
			Code:      "INTERNAL_ERROR",
			Message:   "内部服务器错误",
			Cause:     err,
			Timestamp: time.Now(),
			RequestID: requestID,
		}
	}

	// 记录错误日志
	h.logger.Error("RAG error occurred",
		"type", ragErr.Type,
		"code", ragErr.Code,
		"message", ragErr.Message,
		"request_id", ragErr.RequestID,
		"user_id", ragErr.UserID,
		"details", ragErr.Details,
	)

	// 更新错误指标
	if h.metrics != nil {
		h.metrics.IncrementCounter("rag_errors_total", map[string]string{
			"type": string(ragErr.Type),
			"code": ragErr.Code,
		})
	}

	// 返回用户友好的错误响应
	return &MCPResponse{
		ID: requestID,
		Error: &MCPError{
			Code:    h.getHTTPStatusCode(ragErr.Type),
			Message: ragErr.Message,
			Data:    ragErr.Details,
		},
	}
}

// getHTTPStatusCode 根据错误类型获取 HTTP 状态码
func (h *ErrorHandler) getHTTPStatusCode(errorType ErrorType) int {
	switch errorType {
	case ErrorTypeValidation:
		return 400
	case ErrorTypeAuth, ErrorTypePermission:
		return 401
	case ErrorTypeNotFound:
		return 404
	case ErrorTypeConflict:
		return 409
	case ErrorTypeRateLimit:
		return 429
	case ErrorTypeTimeout:
		return 408
	default:
		return 500
	}
}
