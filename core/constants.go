package core

import "time"

// 系统常量定义

// 版本信息
const (
	Version     = "1.0.0"
	BuildTime   = "2025-01-20"
	Description = "RAG (Retrieval-Augmented Generation) System for MySQL Database Queries"
)

// 默认配置值
const (
	DefaultServerHost     = "0.0.0.0"
	DefaultServerPort     = 8080
	DefaultMCPPort        = 8081
	DefaultReadTimeout    = 30 * time.Second
	DefaultWriteTimeout   = 30 * time.Second
	DefaultRequestTimeout = 60 * time.Second
	DefaultQueryTimeout   = 30 * time.Second
	DefaultLLMTimeout     = 30 * time.Second
)

// 数据库相关常量
const (
	DefaultMaxOpenConns    = 100
	DefaultMaxIdleConns    = 10
	DefaultConnMaxLifetime = time.Hour
	DefaultQueryLimit      = 1000
	DefaultMaxQueryLength  = 10000
)

// 缓存相关常量
const (
	DefaultSchemaTTL      = time.Hour
	DefaultQueryResultTTL = 10 * time.Minute
	DefaultSessionTTL     = 24 * time.Hour
	DefaultMaxCacheSize   = "100MB"
	DefaultCacheKeyPrefix = "rag:"
)

// LLM 相关常量
const (
	DefaultLLMProvider = "openai"
	DefaultLLMModel    = "gpt-4"
	DefaultTemperature = 0.1
	DefaultMaxTokens   = 2048
	DefaultMaxRetries  = 3
	DefaultRetryDelay  = time.Second
)

// MCP 协议常量
const (
	MCPVersion               = "1.0"
	DefaultMaxConnections    = 1000
	DefaultMessageTimeout    = 30 * time.Second
	DefaultHeartbeatInterval = 30 * time.Second
)

// 日志相关常量
const (
	DefaultLogLevel      = "info"
	DefaultLogFormat     = "json"
	DefaultLogOutput     = "stdout"
	DefaultLogMaxSize    = 100 // MB
	DefaultLogMaxBackups = 3
	DefaultLogMaxAge     = 7 // days
)

// 安全相关常量
const (
	DefaultTokenExpiry   = 24 * time.Hour
	DefaultRefreshExpiry = 7 * 24 * time.Hour
	MinPasswordLength    = 8
	MaxLoginAttempts     = 5
	LockoutDuration      = 15 * time.Minute
)

// 限流相关常量
const (
	DefaultRateLimit       = 100 // requests per minute
	DefaultBurstLimit      = 10
	DefaultRateLimitWindow = time.Minute
)

// 监控相关常量
const (
	DefaultMetricsInterval = 10 * time.Second
	DefaultHealthCheckPath = "/health"
	DefaultMetricsPath     = "/metrics"
	DefaultReadinessPath   = "/ready"
	DefaultLivenessPath    = "/live"
)

// 错误码常量
const (
	//  通用错误码
	CodeSuccess        = 0
	CodeInternalError  = 1000
	CodeInvalidRequest = 1001
	CodeUnauthorized   = 1002
	CodeForbidden      = 1003
	CodeNotFound       = 1004
	CodeTimeout        = 1005
	CodeRateLimit      = 1006

	// 验证错误码
	CodeValidationError  = 2000
	CodeMissingParameter = 2001
	CodeInvalidParameter = 2002
	CodeInvalidFormat    = 2003

	// 数据库错误码
	CodeDatabaseError    = 3000
	CodeConnectionFailed = 3001
	CodeQueryFailed      = 3002
	CodeTableNotFound    = 3003
	CodeColumnNotFound   = 3004
	CodeInvalidSQL       = 3005

	// LLM 错误码
	CodeLLMError            = 4000
	CodeLLMConnectionFailed = 4001
	CodeLLMTimeout          = 4002
	CodeLLMRateLimit        = 4003
	CodeLLMQuotaExceeded    = 4004

	// MCP 错误码
	CodeMCPError            = 5000
	CodeMCPInvalidMessage   = 5001
	CodeMCPMethodNotFound   = 5002
	CodeMCPConnectionClosed = 5003

	// 缓存错误码
	CodeCacheError            = 6000
	CodeCacheConnectionFailed = 6001
	CodeCacheKeyNotFound      = 6002
	CodeCacheTimeout          = 6003
)

// HTTP 状态码映射
var HTTPStatusCodes = map[int]int{
	CodeSuccess:               200,
	CodeInternalError:         500,
	CodeInvalidRequest:        400,
	CodeUnauthorized:          401,
	CodeForbidden:             403,
	CodeNotFound:              404,
	CodeTimeout:               408,
	CodeRateLimit:             429,
	CodeValidationError:       400,
	CodeMissingParameter:      400,
	CodeInvalidParameter:      400,
	CodeInvalidFormat:         400,
	CodeDatabaseError:         500,
	CodeConnectionFailed:      500,
	CodeQueryFailed:           500,
	CodeTableNotFound:         404,
	CodeColumnNotFound:        404,
	CodeInvalidSQL:            400,
	CodeLLMError:              500,
	CodeLLMConnectionFailed:   500,
	CodeLLMTimeout:            408,
	CodeLLMRateLimit:          429,
	CodeLLMQuotaExceeded:      429,
	CodeMCPError:              500,
	CodeMCPInvalidMessage:     400,
	CodeMCPMethodNotFound:     404,
	CodeMCPConnectionClosed:   500,
	CodeCacheError:            500,
	CodeCacheConnectionFailed: 500,
	CodeCacheKeyNotFound:      404,
	CodeCacheTimeout:          408,
}

// 支持的数据库类型
var SupportedDatabases = []string{
	"mysql",
	"postgresql",
	"sqlite",
}

// 支持的 LLM 提供商
var SupportedLLMProviders = []string{
	"openai",
	"anthropic",
	"local",
	"azure",
}

// 支持的缓存类型
var SupportedCacheTypes = []string{
	"redis",
	"memory",
	"hybrid",
}

// 支持的日志格式
var SupportedLogFormats = []string{
	"json",
	"text",
	"console",
}

// 支持的日志级别
var SupportedLogLevels = []string{
	"debug",
	"info",
	"warn",
	"error",
	"fatal",
}

// MCP 方法名称
const (
	MCPMethodQuery       = "query"
	MCPMethodExplain     = "explain"
	MCPMethodSuggestions = "suggestions"
	MCPMethodSchema      = "schema"
	MCPMethodHealth      = "health"
	MCPMethodVersion     = "version"
)

// 查询类型
const (
	QueryTypeSelect = "select"
	QueryTypeInsert = "insert"
	QueryTypeUpdate = "update"
	QueryTypeDelete = "delete"
	QueryTypeCreate = "create"
	QueryTypeDrop   = "drop"
	QueryTypeAlter  = "alter"
)

// 输出格式
const (
	FormatJSON     = "json"
	FormatCSV      = "csv"
	FormatTable    = "table"
	FormatMarkdown = "markdown"
)

// 复杂度级别
const (
	ComplexitySimple  = "simple"
	ComplexityMedium  = "medium"
	ComplexityComplex = "complex"
)

// 权限级别
const (
	PermissionRead  = "read"
	PermissionWrite = "write"
	PermissionAdmin = "admin"
)

// 用户角色
const (
	RoleUser       = "user"
	RoleAnalyst    = "analyst"
	RoleAdmin      = "admin"
	RoleSuperAdmin = "super_admin"
)

// 环境类型
const (
	EnvDevelopment = "development"
	EnvStaging     = "staging"
	EnvProduction  = "production"
)

// 指标名称
const (
	MetricRequestsTotal    = "rag_requests_total"
	MetricRequestDuration  = "rag_request_duration_seconds"
	MetricErrorsTotal      = "rag_errors_total"
	MetricLLMRequestsTotal = "rag_llm_requests_total"
	MetricLLMTokensTotal   = "rag_llm_tokens_total"
	MetricDBQueriesTotal   = "rag_db_queries_total"
	MetricCacheHitsTotal   = "rag_cache_hits_total"
	MetricCacheMissesTotal = "rag_cache_misses_total"
)

// 上下文键
const (
	ContextKeyRequestID = "request_id"
	ContextKeyUserID    = "user_id"
	ContextKeySessionID = "session_id"
	ContextKeyStartTime = "start_time"
	ContextKeyLogger    = "logger"
	ContextKeyTracer    = "tracer"
)
