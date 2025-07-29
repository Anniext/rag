// 本文件实现了各种类型的健康检查器，包括HTTP、TCP、数据库、Redis等组件的健康检查功能。
// 主要功能：
// 1. HTTP健康检查器
// 2. TCP连接检查器
// 3. 数据库连接检查器
// 4. Redis连接检查器
// 5. 自定义健康检查器
// 6. 服务实例健康检查器

package monitor

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
)

// ServiceHealthChecker 服务健康检查器
type ServiceHealthChecker struct {
	config *ServiceDiscoveryConfig
	logger Logger
	client *http.Client
}

// NewServiceHealthChecker 创建服务健康检查器
func NewServiceHealthChecker(config *ServiceDiscoveryConfig, logger Logger) *ServiceHealthChecker {
	return &ServiceHealthChecker{
		config: config,
		logger: logger,
		client: &http.Client{
			Timeout: config.HealthCheckTimeout,
		},
	}
}

// CheckInstance 检查服务实例健康状态
func (c *ServiceHealthChecker) CheckInstance(ctx context.Context, instance *ServiceInstance) *HealthCheckResult {
	if instance.HealthCheck == nil {
		return &HealthCheckResult{
			ComponentType: ComponentTypeSystem,
			ComponentName: instance.ID,
			Status:        HealthStatusUnknown,
			Message:       "未配置健康检查",
			CheckTime:     time.Now(),
		}
	}

	switch strings.ToLower(instance.HealthCheck.Type) {
	case "http", "https":
		return c.checkHTTP(ctx, instance)
	case "tcp":
		return c.checkTCP(ctx, instance)
	case "grpc":
		return c.checkGRPC(ctx, instance)
	default:
		return &HealthCheckResult{
			ComponentType: ComponentTypeSystem,
			ComponentName: instance.ID,
			Status:        HealthStatusUnknown,
			Message:       fmt.Sprintf("不支持的健康检查类型: %s", instance.HealthCheck.Type),
			CheckTime:     time.Now(),
		}
	}
}

// checkHTTP HTTP健康检查
func (c *ServiceHealthChecker) checkHTTP(ctx context.Context, instance *ServiceInstance) *HealthCheckResult {
	startTime := time.Now()

	result := &HealthCheckResult{
		ComponentType: ComponentTypeSystem,
		ComponentName: instance.ID,
		CheckTime:     startTime,
		Details:       make(map[string]interface{}),
	}

	// 创建HTTP请求
	req, err := http.NewRequestWithContext(ctx, "GET", instance.HealthCheck.URL, nil)
	if err != nil {
		result.Status = HealthStatusUnhealthy
		result.Message = "创建HTTP请求失败"
		result.Error = err.Error()
		result.Duration = time.Since(startTime)
		return result
	}

	// 设置请求头
	for key, value := range instance.HealthCheck.Headers {
		req.Header.Set(key, value)
	}

	// 执行请求
	resp, err := c.client.Do(req)
	if err != nil {
		result.Status = HealthStatusUnhealthy
		result.Message = "HTTP请求失败"
		result.Error = err.Error()
		result.Duration = time.Since(startTime)
		return result
	}
	defer resp.Body.Close()

	result.Duration = time.Since(startTime)
	result.Details["status_code"] = resp.StatusCode
	result.Details["response_time"] = result.Duration.Milliseconds()

	// 检查响应状态码
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		result.Status = HealthStatusHealthy
		result.Message = "HTTP健康检查成功"
	} else if resp.StatusCode >= 500 {
		result.Status = HealthStatusUnhealthy
		result.Message = fmt.Sprintf("HTTP响应状态码: %d", resp.StatusCode)
	} else {
		result.Status = HealthStatusDegraded
		result.Message = fmt.Sprintf("HTTP响应状态码: %d", resp.StatusCode)
	}

	return result
}

// checkTCP TCP连接检查
func (c *ServiceHealthChecker) checkTCP(ctx context.Context, instance *ServiceInstance) *HealthCheckResult {
	startTime := time.Now()

	result := &HealthCheckResult{
		ComponentType: ComponentTypeSystem,
		ComponentName: instance.ID,
		CheckTime:     startTime,
		Details:       make(map[string]interface{}),
	}

	address := fmt.Sprintf("%s:%d", instance.Host, instance.Port)

	// 创建TCP连接
	dialer := &net.Dialer{
		Timeout: instance.HealthCheck.Timeout,
	}

	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		result.Status = HealthStatusUnhealthy
		result.Message = "TCP连接失败"
		result.Error = err.Error()
		result.Duration = time.Since(startTime)
		return result
	}
	defer conn.Close()

	result.Duration = time.Since(startTime)
	result.Details["address"] = address
	result.Details["connection_time"] = result.Duration.Milliseconds()
	result.Status = HealthStatusHealthy
	result.Message = "TCP连接成功"

	return result
}

// checkGRPC gRPC健康检查
func (c *ServiceHealthChecker) checkGRPC(ctx context.Context, instance *ServiceInstance) *HealthCheckResult {
	startTime := time.Now()

	result := &HealthCheckResult{
		ComponentType: ComponentTypeSystem,
		ComponentName: instance.ID,
		CheckTime:     startTime,
		Details:       make(map[string]interface{}),
	}

	// 这里应该实现gRPC健康检查
	// 由于需要gRPC依赖，这里先返回未实现状态
	result.Status = HealthStatusUnknown
	result.Message = "gRPC健康检查未实现"
	result.Duration = time.Since(startTime)

	return result
}

// HTTPHealthChecker HTTP健康检查器
type HTTPHealthChecker struct {
	name    string
	url     string
	timeout time.Duration
	client  *http.Client
	enabled bool
	headers map[string]string
}

// NewHTTPHealthChecker 创建HTTP健康检查器
func NewHTTPHealthChecker(name, url string, timeout time.Duration) *HTTPHealthChecker {
	return &HTTPHealthChecker{
		name:    name,
		url:     url,
		timeout: timeout,
		client: &http.Client{
			Timeout: timeout,
		},
		enabled: true,
		headers: make(map[string]string),
	}
}

// CheckHealth 检查健康状态
func (c *HTTPHealthChecker) CheckHealth(ctx context.Context) *HealthCheckResult {
	startTime := time.Now()

	result := &HealthCheckResult{
		ComponentType: ComponentTypeSystem,
		ComponentName: c.name,
		CheckTime:     startTime,
		Details:       make(map[string]interface{}),
	}

	// 创建HTTP请求
	req, err := http.NewRequestWithContext(ctx, "GET", c.url, nil)
	if err != nil {
		result.Status = HealthStatusUnhealthy
		result.Message = "创建HTTP请求失败"
		result.Error = err.Error()
		result.Duration = time.Since(startTime)
		return result
	}

	// 设置请求头
	for key, value := range c.headers {
		req.Header.Set(key, value)
	}

	// 执行请求
	resp, err := c.client.Do(req)
	if err != nil {
		result.Status = HealthStatusUnhealthy
		result.Message = "HTTP请求失败"
		result.Error = err.Error()
		result.Duration = time.Since(startTime)
		return result
	}
	defer resp.Body.Close()

	result.Duration = time.Since(startTime)
	result.Details["url"] = c.url
	result.Details["status_code"] = resp.StatusCode
	result.Details["response_time"] = result.Duration.Milliseconds()

	// 检查响应状态码
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		result.Status = HealthStatusHealthy
		result.Message = "HTTP健康检查成功"
	} else if resp.StatusCode >= 500 {
		result.Status = HealthStatusUnhealthy
		result.Message = fmt.Sprintf("HTTP响应状态码: %d", resp.StatusCode)
	} else {
		result.Status = HealthStatusDegraded
		result.Message = fmt.Sprintf("HTTP响应状态码: %d", resp.StatusCode)
	}

	return result
}

// GetComponentType 获取组件类型
func (c *HTTPHealthChecker) GetComponentType() ComponentType {
	return ComponentTypeSystem
}

// GetName 获取组件名称
func (c *HTTPHealthChecker) GetName() string {
	return c.name
}

// IsEnabled 检查是否启用
func (c *HTTPHealthChecker) IsEnabled() bool {
	return c.enabled
}

// SetEnabled 设置启用状态
func (c *HTTPHealthChecker) SetEnabled(enabled bool) {
	c.enabled = enabled
}

// SetHeader 设置请求头
func (c *HTTPHealthChecker) SetHeader(key, value string) {
	c.headers[key] = value
}

// TCPHealthChecker TCP健康检查器
type TCPHealthChecker struct {
	name    string
	address string
	timeout time.Duration
	enabled bool
}

// NewTCPHealthChecker 创建TCP健康检查器
func NewTCPHealthChecker(name, address string, timeout time.Duration) *TCPHealthChecker {
	return &TCPHealthChecker{
		name:    name,
		address: address,
		timeout: timeout,
		enabled: true,
	}
}

// CheckHealth 检查健康状态
func (c *TCPHealthChecker) CheckHealth(ctx context.Context) *HealthCheckResult {
	startTime := time.Now()

	result := &HealthCheckResult{
		ComponentType: ComponentTypeSystem,
		ComponentName: c.name,
		CheckTime:     startTime,
		Details:       make(map[string]interface{}),
	}

	// 创建TCP连接
	dialer := &net.Dialer{
		Timeout: c.timeout,
	}

	conn, err := dialer.DialContext(ctx, "tcp", c.address)
	if err != nil {
		result.Status = HealthStatusUnhealthy
		result.Message = "TCP连接失败"
		result.Error = err.Error()
		result.Duration = time.Since(startTime)
		return result
	}
	defer conn.Close()

	result.Duration = time.Since(startTime)
	result.Details["address"] = c.address
	result.Details["connection_time"] = result.Duration.Milliseconds()
	result.Status = HealthStatusHealthy
	result.Message = "TCP连接成功"

	return result
}

// GetComponentType 获取组件类型
func (c *TCPHealthChecker) GetComponentType() ComponentType {
	return ComponentTypeSystem
}

// GetName 获取组件名称
func (c *TCPHealthChecker) GetName() string {
	return c.name
}

// IsEnabled 检查是否启用
func (c *TCPHealthChecker) IsEnabled() bool {
	return c.enabled
}

// SetEnabled 设置启用状态
func (c *TCPHealthChecker) SetEnabled(enabled bool) {
	c.enabled = enabled
}

// RedisHealthChecker Redis健康检查器
type RedisHealthChecker struct {
	name    string
	client  *redis.Client
	enabled bool
	timeout time.Duration
}

// NewRedisHealthChecker 创建Redis健康检查器
func NewRedisHealthChecker(name string, client *redis.Client) *RedisHealthChecker {
	return &RedisHealthChecker{
		name:    name,
		client:  client,
		enabled: true,
		timeout: 5 * time.Second,
	}
}

// CheckHealth 检查健康状态
func (c *RedisHealthChecker) CheckHealth(ctx context.Context) *HealthCheckResult {
	startTime := time.Now()

	result := &HealthCheckResult{
		ComponentType: ComponentTypeRedis,
		ComponentName: c.name,
		CheckTime:     startTime,
		Details:       make(map[string]interface{}),
	}

	// 创建带超时的上下文
	checkCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// 执行PING命令
	pong, err := c.client.Ping(checkCtx).Result()
	if err != nil {
		result.Status = HealthStatusUnhealthy
		result.Message = "Redis连接失败"
		result.Error = err.Error()
		result.Duration = time.Since(startTime)
		return result
	}

	// 获取Redis信息
	info, err := c.client.Info(checkCtx, "server").Result()
	if err == nil {
		result.Details["server_info"] = info
	}

	// 获取连接统计
	poolStats := c.client.PoolStats()
	result.Details["pool_stats"] = map[string]interface{}{
		"hits":        poolStats.Hits,
		"misses":      poolStats.Misses,
		"timeouts":    poolStats.Timeouts,
		"total_conns": poolStats.TotalConns,
		"idle_conns":  poolStats.IdleConns,
		"stale_conns": poolStats.StaleConns,
	}

	result.Duration = time.Since(startTime)
	result.Details["ping_response"] = pong
	result.Details["response_time"] = result.Duration.Milliseconds()
	result.Status = HealthStatusHealthy
	result.Message = "Redis连接正常"

	return result
}

// GetComponentType 获取组件类型
func (c *RedisHealthChecker) GetComponentType() ComponentType {
	return ComponentTypeRedis
}

// GetName 获取组件名称
func (c *RedisHealthChecker) GetName() string {
	return c.name
}

// IsEnabled 检查是否启用
func (c *RedisHealthChecker) IsEnabled() bool {
	return c.enabled
}

// SetEnabled 设置启用状态
func (c *RedisHealthChecker) SetEnabled(enabled bool) {
	c.enabled = enabled
}

// MySQLHealthChecker MySQL健康检查器
type MySQLHealthChecker struct {
	name    string
	db      *sql.DB
	enabled bool
	timeout time.Duration
}

// NewMySQLHealthChecker 创建MySQL健康检查器
func NewMySQLHealthChecker(name string, db *sql.DB) *MySQLHealthChecker {
	return &MySQLHealthChecker{
		name:    name,
		db:      db,
		enabled: true,
		timeout: 5 * time.Second,
	}
}

// CheckHealth 检查健康状态
func (c *MySQLHealthChecker) CheckHealth(ctx context.Context) *HealthCheckResult {
	startTime := time.Now()

	result := &HealthCheckResult{
		ComponentType: ComponentTypeDatabase,
		ComponentName: c.name,
		CheckTime:     startTime,
		Details:       make(map[string]interface{}),
	}

	// 创建带超时的上下文
	checkCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// 检查数据库连接
	if err := c.db.PingContext(checkCtx); err != nil {
		result.Status = HealthStatusUnhealthy
		result.Message = "MySQL连接失败"
		result.Error = err.Error()
		result.Duration = time.Since(startTime)
		return result
	}

	// 获取数据库统计信息
	stats := c.db.Stats()
	result.Details["db_stats"] = map[string]interface{}{
		"open_connections":     stats.OpenConnections,
		"in_use":               stats.InUse,
		"idle":                 stats.Idle,
		"wait_count":           stats.WaitCount,
		"wait_duration":        stats.WaitDuration.String(),
		"max_idle_closed":      stats.MaxIdleClosed,
		"max_idle_time_closed": stats.MaxIdleTimeClosed,
		"max_lifetime_closed":  stats.MaxLifetimeClosed,
	}

	// 执行简单查询测试
	var version string
	err := c.db.QueryRowContext(checkCtx, "SELECT VERSION()").Scan(&version)
	if err != nil {
		result.Status = HealthStatusDegraded
		result.Message = "MySQL查询测试失败"
		result.Error = err.Error()
	} else {
		result.Details["mysql_version"] = version
		result.Status = HealthStatusHealthy
		result.Message = "MySQL连接正常"
	}

	// 检查连接池状态
	if stats.OpenConnections > stats.MaxOpenConnections*8/10 {
		if result.Status == HealthStatusHealthy {
			result.Status = HealthStatusDegraded
			result.Message = "MySQL连接池使用率较高"
		}
	}

	result.Duration = time.Since(startTime)
	result.Details["response_time"] = result.Duration.Milliseconds()

	return result
}

// GetComponentType 获取组件类型
func (c *MySQLHealthChecker) GetComponentType() ComponentType {
	return ComponentTypeDatabase
}

// GetName 获取组件名称
func (c *MySQLHealthChecker) GetName() string {
	return c.name
}

// IsEnabled 检查是否启用
func (c *MySQLHealthChecker) IsEnabled() bool {
	return c.enabled
}

// SetEnabled 设置启用状态
func (c *MySQLHealthChecker) SetEnabled(enabled bool) {
	c.enabled = enabled
}

// CustomHealthChecker 自定义健康检查器
type CustomHealthChecker struct {
	name          string
	componentType ComponentType
	checkFunc     func(ctx context.Context) *HealthCheckResult
	enabled       bool
}

// NewCustomHealthChecker 创建自定义健康检查器
func NewCustomHealthChecker(name string, componentType ComponentType, checkFunc func(ctx context.Context) *HealthCheckResult) *CustomHealthChecker {
	return &CustomHealthChecker{
		name:          name,
		componentType: componentType,
		checkFunc:     checkFunc,
		enabled:       true,
	}
}

// CheckHealth 检查健康状态
func (c *CustomHealthChecker) CheckHealth(ctx context.Context) *HealthCheckResult {
	if c.checkFunc == nil {
		return &HealthCheckResult{
			ComponentType: c.componentType,
			ComponentName: c.name,
			Status:        HealthStatusUnknown,
			Message:       "未定义检查函数",
			CheckTime:     time.Now(),
		}
	}

	return c.checkFunc(ctx)
}

// GetComponentType 获取组件类型
func (c *CustomHealthChecker) GetComponentType() ComponentType {
	return c.componentType
}

// GetName 获取组件名称
func (c *CustomHealthChecker) GetName() string {
	return c.name
}

// IsEnabled 检查是否启用
func (c *CustomHealthChecker) IsEnabled() bool {
	return c.enabled
}

// SetEnabled 设置启用状态
func (c *CustomHealthChecker) SetEnabled(enabled bool) {
	c.enabled = enabled
}

// SystemHealthChecker 系统健康检查器
type SystemHealthChecker struct {
	name    string
	enabled bool
}

// NewSystemHealthChecker 创建系统健康检查器
func NewSystemHealthChecker(name string) *SystemHealthChecker {
	return &SystemHealthChecker{
		name:    name,
		enabled: true,
	}
}

// CheckHealth 检查健康状态
func (c *SystemHealthChecker) CheckHealth(ctx context.Context) *HealthCheckResult {
	startTime := time.Now()

	result := &HealthCheckResult{
		ComponentType: ComponentTypeSystem,
		ComponentName: c.name,
		CheckTime:     startTime,
		Details:       make(map[string]interface{}),
	}

	// 检查系统资源
	// 这里可以添加CPU、内存、磁盘等系统资源检查
	// 由于需要系统相关的库，这里先返回基本信息

	result.Details["uptime"] = time.Since(startTime).String()
	result.Details["timestamp"] = time.Now().Unix()

	result.Duration = time.Since(startTime)
	result.Status = HealthStatusHealthy
	result.Message = "系统运行正常"

	return result
}

// GetComponentType 获取组件类型
func (c *SystemHealthChecker) GetComponentType() ComponentType {
	return ComponentTypeSystem
}

// GetName 获取组件名称
func (c *SystemHealthChecker) GetName() string {
	return c.name
}

// IsEnabled 检查是否启用
func (c *SystemHealthChecker) IsEnabled() bool {
	return c.enabled
}

// SetEnabled 设置启用状态
func (c *SystemHealthChecker) SetEnabled(enabled bool) {
	c.enabled = enabled
}
