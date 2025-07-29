package core

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestVersionConstants 测试版本常量
func TestVersionConstants(t *testing.T) {
	assert.NotEmpty(t, Version, "版本号不能为空")
	assert.NotEmpty(t, BuildTime, "构建时间不能为空")
	assert.NotEmpty(t, Description, "描述不能为空")

	// 验证版本号格式（简单的语义版本检查）
	assert.Regexp(t, `^\d+\.\d+\.\d+`, Version, "版本号应该符合语义版本格式")
}

// TestDefaultConfigValues 测试默认配置值
func TestDefaultConfigValues(t *testing.T) {
	// 服务器配置
	assert.Equal(t, "0.0.0.0", DefaultServerHost)
	assert.Equal(t, 8080, DefaultServerPort)
	assert.Equal(t, 8081, DefaultMCPPort)
	assert.Equal(t, 30*time.Second, DefaultReadTimeout)
	assert.Equal(t, 30*time.Second, DefaultWriteTimeout)
	assert.Equal(t, 60*time.Second, DefaultRequestTimeout)
	assert.Equal(t, 30*time.Second, DefaultQueryTimeout)
	assert.Equal(t, 30*time.Second, DefaultLLMTimeout)

	// 数据库配置
	assert.Equal(t, 100, DefaultMaxOpenConns)
	assert.Equal(t, 10, DefaultMaxIdleConns)
	assert.Equal(t, time.Hour, DefaultConnMaxLifetime)
	assert.Equal(t, 1000, DefaultQueryLimit)
	assert.Equal(t, 10000, DefaultMaxQueryLength)

	// 缓存配置
	assert.Equal(t, time.Hour, DefaultSchemaTTL)
	assert.Equal(t, 10*time.Minute, DefaultQueryResultTTL)
	assert.Equal(t, 24*time.Hour, DefaultSessionTTL)
	assert.Equal(t, "100MB", DefaultMaxCacheSize)
	assert.Equal(t, "rag:", DefaultCacheKeyPrefix)

	// LLM 配置
	assert.Equal(t, "openai", DefaultLLMProvider)
	assert.Equal(t, "gpt-4", DefaultLLMModel)
	assert.Equal(t, 0.1, DefaultTemperature)
	assert.Equal(t, 2048, DefaultMaxTokens)
	assert.Equal(t, 3, DefaultMaxRetries)
	assert.Equal(t, time.Second, DefaultRetryDelay)

	// MCP 配置
	assert.Equal(t, "1.0", MCPVersion)
	assert.Equal(t, 1000, DefaultMaxConnections)
	assert.Equal(t, 30*time.Second, DefaultMessageTimeout)
	assert.Equal(t, 30*time.Second, DefaultHeartbeatInterval)

	// 日志配置
	assert.Equal(t, "info", DefaultLogLevel)
	assert.Equal(t, "json", DefaultLogFormat)
	assert.Equal(t, "stdout", DefaultLogOutput)
	assert.Equal(t, 100, DefaultLogMaxSize)
	assert.Equal(t, 3, DefaultLogMaxBackups)
	assert.Equal(t, 7, DefaultLogMaxAge)

	// 安全配置
	assert.Equal(t, 24*time.Hour, DefaultTokenExpiry)
	assert.Equal(t, 7*24*time.Hour, DefaultRefreshExpiry)
	assert.Equal(t, 8, MinPasswordLength)
	assert.Equal(t, 5, MaxLoginAttempts)
	assert.Equal(t, 15*time.Minute, LockoutDuration)

	// 限流配置
	assert.Equal(t, 100, DefaultRateLimit)
	assert.Equal(t, 10, DefaultBurstLimit)
	assert.Equal(t, time.Minute, DefaultRateLimitWindow)

	// 监控配置
	assert.Equal(t, 10*time.Second, DefaultMetricsInterval)
	assert.Equal(t, "/health", DefaultHealthCheckPath)
	assert.Equal(t, "/metrics", DefaultMetricsPath)
	assert.Equal(t, "/ready", DefaultReadinessPath)
	assert.Equal(t, "/live", DefaultLivenessPath)
}

// TestErrorCodes 测试错误码
func TestErrorCodes(t *testing.T) {
	// 验证错误码范围
	assert.Equal(t, 0, CodeSuccess)

	// 通用错误码 (1000-1999)
	assert.True(t, CodeInternalError >= 1000 && CodeInternalError < 2000)
	assert.True(t, CodeInvalidRequest >= 1000 && CodeInvalidRequest < 2000)
	assert.True(t, CodeUnauthorized >= 1000 && CodeUnauthorized < 2000)
	assert.True(t, CodeForbidden >= 1000 && CodeForbidden < 2000)
	assert.True(t, CodeNotFound >= 1000 && CodeNotFound < 2000)
	assert.True(t, CodeTimeout >= 1000 && CodeTimeout < 2000)
	assert.True(t, CodeRateLimit >= 1000 && CodeRateLimit < 2000)

	// 验证错误码 (2000-2999)
	assert.True(t, CodeValidationError >= 2000 && CodeValidationError < 3000)
	assert.True(t, CodeMissingParameter >= 2000 && CodeMissingParameter < 3000)
	assert.True(t, CodeInvalidParameter >= 2000 && CodeInvalidParameter < 3000)
	assert.True(t, CodeInvalidFormat >= 2000 && CodeInvalidFormat < 3000)

	// 数据库错误码 (3000-3999)
	assert.True(t, CodeDatabaseError >= 3000 && CodeDatabaseError < 4000)
	assert.True(t, CodeConnectionFailed >= 3000 && CodeConnectionFailed < 4000)
	assert.True(t, CodeQueryFailed >= 3000 && CodeQueryFailed < 4000)
	assert.True(t, CodeTableNotFound >= 3000 && CodeTableNotFound < 4000)
	assert.True(t, CodeColumnNotFound >= 3000 && CodeColumnNotFound < 4000)
	assert.True(t, CodeInvalidSQL >= 3000 && CodeInvalidSQL < 4000)

	// LLM 错误码 (4000-4999)
	assert.True(t, CodeLLMError >= 4000 && CodeLLMError < 5000)
	assert.True(t, CodeLLMConnectionFailed >= 4000 && CodeLLMConnectionFailed < 5000)
	assert.True(t, CodeLLMTimeout >= 4000 && CodeLLMTimeout < 5000)
	assert.True(t, CodeLLMRateLimit >= 4000 && CodeLLMRateLimit < 5000)
	assert.True(t, CodeLLMQuotaExceeded >= 4000 && CodeLLMQuotaExceeded < 5000)

	// MCP 错误码 (5000-5999)
	assert.True(t, CodeMCPError >= 5000 && CodeMCPError < 6000)
	assert.True(t, CodeMCPInvalidMessage >= 5000 && CodeMCPInvalidMessage < 6000)
	assert.True(t, CodeMCPMethodNotFound >= 5000 && CodeMCPMethodNotFound < 6000)
	assert.True(t, CodeMCPConnectionClosed >= 5000 && CodeMCPConnectionClosed < 6000)

	// 缓存错误码 (6000-6999)
	assert.True(t, CodeCacheError >= 6000 && CodeCacheError < 7000)
	assert.True(t, CodeCacheConnectionFailed >= 6000 && CodeCacheConnectionFailed < 7000)
	assert.True(t, CodeCacheKeyNotFound >= 6000 && CodeCacheKeyNotFound < 7000)
	assert.True(t, CodeCacheTimeout >= 6000 && CodeCacheTimeout < 7000)
}

// TestHTTPStatusCodes 测试 HTTP 状态码映射
func TestHTTPStatusCodes(t *testing.T) {
	// 验证所有错误码都有对应的 HTTP 状态码
	expectedMappings := map[int]int{
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

	for errorCode, expectedHTTPCode := range expectedMappings {
		actualHTTPCode, exists := HTTPStatusCodes[errorCode]
		assert.True(t, exists, "错误码 %d 没有对应的 HTTP 状态码", errorCode)
		assert.Equal(t, expectedHTTPCode, actualHTTPCode, "错误码 %d 的 HTTP 状态码不匹配", errorCode)
	}

	// 验证 HTTP 状态码的有效性
	validHTTPCodes := []int{200, 400, 401, 403, 404, 408, 429, 500}
	for _, httpCode := range HTTPStatusCodes {
		assert.Contains(t, validHTTPCodes, httpCode, "HTTP 状态码 %d 不在有效范围内", httpCode)
	}
}

// TestSupportedTypes 测试支持的类型列表
func TestSupportedTypes(t *testing.T) {
	// 支持的数据库类型
	expectedDatabases := []string{"mysql", "postgresql", "sqlite"}
	assert.ElementsMatch(t, expectedDatabases, SupportedDatabases)
	assert.NotEmpty(t, SupportedDatabases, "支持的数据库类型不能为空")

	// 支持的 LLM 提供商
	expectedProviders := []string{"openai", "anthropic", "local", "azure"}
	assert.ElementsMatch(t, expectedProviders, SupportedLLMProviders)
	assert.NotEmpty(t, SupportedLLMProviders, "支持的 LLM 提供商不能为空")

	// 支持的缓存类型
	expectedCacheTypes := []string{"redis", "memory", "hybrid"}
	assert.ElementsMatch(t, expectedCacheTypes, SupportedCacheTypes)
	assert.NotEmpty(t, SupportedCacheTypes, "支持的缓存类型不能为空")

	// 支持的日志格式
	expectedLogFormats := []string{"json", "text", "console"}
	assert.ElementsMatch(t, expectedLogFormats, SupportedLogFormats)
	assert.NotEmpty(t, SupportedLogFormats, "支持的日志格式不能为空")

	// 支持的日志级别
	expectedLogLevels := []string{"debug", "info", "warn", "error", "fatal"}
	assert.ElementsMatch(t, expectedLogLevels, SupportedLogLevels)
	assert.NotEmpty(t, SupportedLogLevels, "支持的日志级别不能为空")
}

// TestMCPMethods 测试 MCP 方法名称
func TestMCPMethods(t *testing.T) {
	methods := []string{
		MCPMethodQuery,
		MCPMethodExplain,
		MCPMethodSuggestions,
		MCPMethodSchema,
		MCPMethodHealth,
		MCPMethodVersion,
	}

	for _, method := range methods {
		assert.NotEmpty(t, method, "MCP 方法名称不能为空")
		assert.NotContains(t, method, " ", "MCP 方法名称不能包含空格")
	}

	// 验证具体的方法名称
	assert.Equal(t, "query", MCPMethodQuery)
	assert.Equal(t, "explain", MCPMethodExplain)
	assert.Equal(t, "suggestions", MCPMethodSuggestions)
	assert.Equal(t, "schema", MCPMethodSchema)
	assert.Equal(t, "health", MCPMethodHealth)
	assert.Equal(t, "version", MCPMethodVersion)
}

// TestQueryTypes 测试查询类型
func TestQueryTypes(t *testing.T) {
	queryTypes := []string{
		QueryTypeSelect,
		QueryTypeInsert,
		QueryTypeUpdate,
		QueryTypeDelete,
		QueryTypeCreate,
		QueryTypeDrop,
		QueryTypeAlter,
	}

	for _, queryType := range queryTypes {
		assert.NotEmpty(t, queryType, "查询类型不能为空")
		assert.Equal(t, queryType, strings.ToLower(queryType), "查询类型应该是小写")
	}

	// 验证具体的查询类型
	assert.Equal(t, "select", QueryTypeSelect)
	assert.Equal(t, "insert", QueryTypeInsert)
	assert.Equal(t, "update", QueryTypeUpdate)
	assert.Equal(t, "delete", QueryTypeDelete)
	assert.Equal(t, "create", QueryTypeCreate)
	assert.Equal(t, "drop", QueryTypeDrop)
	assert.Equal(t, "alter", QueryTypeAlter)
}

// TestOutputFormats 测试输出格式
func TestOutputFormats(t *testing.T) {
	formats := []string{
		FormatJSON,
		FormatCSV,
		FormatTable,
		FormatMarkdown,
	}

	for _, format := range formats {
		assert.NotEmpty(t, format, "输出格式不能为空")
		assert.Equal(t, format, strings.ToLower(format), "输出格式应该是小写")
	}

	// 验证具体的输出格式
	assert.Equal(t, "json", FormatJSON)
	assert.Equal(t, "csv", FormatCSV)
	assert.Equal(t, "table", FormatTable)
	assert.Equal(t, "markdown", FormatMarkdown)
}

// TestComplexityLevels 测试复杂度级别
func TestComplexityLevels(t *testing.T) {
	complexities := []string{
		ComplexitySimple,
		ComplexityMedium,
		ComplexityComplex,
	}

	for _, complexity := range complexities {
		assert.NotEmpty(t, complexity, "复杂度级别不能为空")
		assert.Equal(t, complexity, strings.ToLower(complexity), "复杂度级别应该是小写")
	}

	// 验证具体的复杂度级别
	assert.Equal(t, "simple", ComplexitySimple)
	assert.Equal(t, "medium", ComplexityMedium)
	assert.Equal(t, "complex", ComplexityComplex)
}

// TestPermissionsAndRoles 测试权限和角色
func TestPermissionsAndRoles(t *testing.T) {
	// 权限级别
	permissions := []string{
		PermissionRead,
		PermissionWrite,
		PermissionAdmin,
	}

	for _, permission := range permissions {
		assert.NotEmpty(t, permission, "权限级别不能为空")
		assert.Equal(t, permission, strings.ToLower(permission), "权限级别应该是小写")
	}

	// 用户角色
	roles := []string{
		RoleUser,
		RoleAnalyst,
		RoleAdmin,
		RoleSuperAdmin,
	}

	for _, role := range roles {
		assert.NotEmpty(t, role, "用户角色不能为空")
	}

	// 验证具体的权限和角色
	assert.Equal(t, "read", PermissionRead)
	assert.Equal(t, "write", PermissionWrite)
	assert.Equal(t, "admin", PermissionAdmin)

	assert.Equal(t, "user", RoleUser)
	assert.Equal(t, "analyst", RoleAnalyst)
	assert.Equal(t, "admin", RoleAdmin)
	assert.Equal(t, "super_admin", RoleSuperAdmin)
}

// TestEnvironments 测试环境类型
func TestEnvironments(t *testing.T) {
	environments := []string{
		EnvDevelopment,
		EnvStaging,
		EnvProduction,
	}

	for _, env := range environments {
		assert.NotEmpty(t, env, "环境类型不能为空")
		assert.Equal(t, env, strings.ToLower(env), "环境类型应该是小写")
	}

	// 验证具体的环境类型
	assert.Equal(t, "development", EnvDevelopment)
	assert.Equal(t, "staging", EnvStaging)
	assert.Equal(t, "production", EnvProduction)
}

// TestMetricNames 测试指标名称
func TestMetricNames(t *testing.T) {
	metrics := []string{
		MetricRequestsTotal,
		MetricRequestDuration,
		MetricErrorsTotal,
		MetricLLMRequestsTotal,
		MetricLLMTokensTotal,
		MetricDBQueriesTotal,
		MetricCacheHitsTotal,
		MetricCacheMissesTotal,
	}

	for _, metric := range metrics {
		assert.NotEmpty(t, metric, "指标名称不能为空")
		assert.True(t, strings.HasPrefix(metric, "rag_"), "指标名称应该以 'rag_' 开头")
		assert.NotContains(t, metric, " ", "指标名称不能包含空格")
	}

	// 验证具体的指标名称
	assert.Equal(t, "rag_requests_total", MetricRequestsTotal)
	assert.Equal(t, "rag_request_duration_seconds", MetricRequestDuration)
	assert.Equal(t, "rag_errors_total", MetricErrorsTotal)
	assert.Equal(t, "rag_llm_requests_total", MetricLLMRequestsTotal)
	assert.Equal(t, "rag_llm_tokens_total", MetricLLMTokensTotal)
	assert.Equal(t, "rag_db_queries_total", MetricDBQueriesTotal)
	assert.Equal(t, "rag_cache_hits_total", MetricCacheHitsTotal)
	assert.Equal(t, "rag_cache_misses_total", MetricCacheMissesTotal)
}

// TestContextKeys 测试上下文键
func TestContextKeys(t *testing.T) {
	contextKeys := []string{
		ContextKeyRequestID,
		ContextKeyUserID,
		ContextKeySessionID,
		ContextKeyStartTime,
		ContextKeyLogger,
		ContextKeyTracer,
	}

	for _, key := range contextKeys {
		assert.NotEmpty(t, key, "上下文键不能为空")
		assert.NotContains(t, key, " ", "上下文键不能包含空格")
	}

	// 验证具体的上下文键
	assert.Equal(t, "request_id", ContextKeyRequestID)
	assert.Equal(t, "user_id", ContextKeyUserID)
	assert.Equal(t, "session_id", ContextKeySessionID)
	assert.Equal(t, "start_time", ContextKeyStartTime)
	assert.Equal(t, "logger", ContextKeyLogger)
	assert.Equal(t, "tracer", ContextKeyTracer)
}

// TestTimeoutValues 测试超时值的合理性
func TestTimeoutValues(t *testing.T) {
	// 验证超时值都是正数
	timeouts := []time.Duration{
		DefaultReadTimeout,
		DefaultWriteTimeout,
		DefaultRequestTimeout,
		DefaultQueryTimeout,
		DefaultLLMTimeout,
		DefaultMessageTimeout,
		DefaultHeartbeatInterval,
	}

	for _, timeout := range timeouts {
		assert.Positive(t, timeout, "超时值应该是正数")
		assert.True(t, timeout >= time.Second, "超时值应该至少为1秒")
		assert.True(t, timeout <= 10*time.Minute, "超时值不应该超过10分钟")
	}
}

// TestCacheValues 测试缓存相关值的合理性
func TestCacheValues(t *testing.T) {
	// 验证 TTL 值都是正数
	ttls := []time.Duration{
		DefaultSchemaTTL,
		DefaultQueryResultTTL,
		DefaultSessionTTL,
		DefaultTokenExpiry,
		DefaultRefreshExpiry,
	}

	for _, ttl := range ttls {
		assert.Positive(t, ttl, "TTL 值应该是正数")
	}

	// 验证缓存键前缀
	assert.True(t, strings.HasSuffix(DefaultCacheKeyPrefix, ":"), "缓存键前缀应该以冒号结尾")
	assert.NotContains(t, DefaultCacheKeyPrefix, " ", "缓存键前缀不能包含空格")
}

// TestNumericLimits 测试数值限制的合理性
func TestNumericLimits(t *testing.T) {
	// 验证连接数限制
	assert.Positive(t, DefaultMaxOpenConns, "最大打开连接数应该是正数")
	assert.Positive(t, DefaultMaxIdleConns, "最大空闲连接数应该是正数")
	assert.True(t, DefaultMaxIdleConns <= DefaultMaxOpenConns, "最大空闲连接数不应该超过最大打开连接数")

	// 验证查询限制
	assert.Positive(t, DefaultQueryLimit, "查询限制应该是正数")
	assert.Positive(t, DefaultMaxQueryLength, "最大查询长度应该是正数")

	// 验证 LLM 参数
	assert.True(t, DefaultTemperature >= 0 && DefaultTemperature <= 2, "温度值应该在0-2之间")
	assert.Positive(t, DefaultMaxTokens, "最大 token 数应该是正数")
	assert.Positive(t, DefaultMaxRetries, "最大重试次数应该是正数")

	// 验证安全参数
	assert.True(t, MinPasswordLength >= 8, "最小密码长度应该至少为8")
	assert.Positive(t, MaxLoginAttempts, "最大登录尝试次数应该是正数")

	// 验证限流参数
	assert.Positive(t, DefaultRateLimit, "默认限流值应该是正数")
	assert.Positive(t, DefaultBurstLimit, "默认突发限制应该是正数")
}

// 基准测试
func BenchmarkHTTPStatusCodeLookup(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = HTTPStatusCodes[CodeInternalError]
	}
}

func BenchmarkSupportedDatabasesCheck(b *testing.B) {
	database := "mysql"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, db := range SupportedDatabases {
			if db == database {
				break
			}
		}
	}
}
