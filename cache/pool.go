package cache

import (
	"context"
	"fmt"
	"github.com/Anniext/rag/core"
	"sync"
	"sync/atomic"
	"time"
)

// ConnectionPool 连接池接口
type ConnectionPool interface {
	Get(ctx context.Context) (Connection, error)
	Put(conn Connection) error
	Close() error
	Stats() *PoolStats
}

// Connection 连接接口
type Connection interface {
	IsValid() bool
	Close() error
	LastUsed() time.Time
	SetLastUsed(time.Time)
}

// PoolConfig 连接池配置
type PoolConfig struct {
	MinConnections      int           `yaml:"min_connections" json:"min_connections"`
	MaxConnections      int           `yaml:"max_connections" json:"max_connections"`
	MaxIdleTime         time.Duration `yaml:"max_idle_time" json:"max_idle_time"`
	ConnectionTimeout   time.Duration `yaml:"connection_timeout" json:"connection_timeout"`
	IdleCheckInterval   time.Duration `yaml:"idle_check_interval" json:"idle_check_interval"`
	MaxLifetime         time.Duration `yaml:"max_lifetime" json:"max_lifetime"`
	HealthCheckEnabled  bool          `yaml:"health_check_enabled" json:"health_check_enabled"`
	HealthCheckInterval time.Duration `yaml:"health_check_interval" json:"health_check_interval"`
}

// DefaultPoolConfig 默认连接池配置
func DefaultPoolConfig() *PoolConfig {
	return &PoolConfig{
		MinConnections:      5,
		MaxConnections:      50,
		MaxIdleTime:         30 * time.Minute,
		ConnectionTimeout:   10 * time.Second,
		IdleCheckInterval:   5 * time.Minute,
		MaxLifetime:         time.Hour,
		HealthCheckEnabled:  true,
		HealthCheckInterval: time.Minute,
	}
}

// PoolStats 连接池统计信息
type PoolStats struct {
	TotalConnections  int32     `json:"total_connections"`
	ActiveConnections int32     `json:"active_connections"`
	IdleConnections   int32     `json:"idle_connections"`
	WaitingRequests   int32     `json:"waiting_requests"`
	CreatedCount      int64     `json:"created_count"`
	ClosedCount       int64     `json:"closed_count"`
	ErrorCount        int64     `json:"error_count"`
	LastActivity      time.Time `json:"last_activity"`
}

// ConnectionFactory 连接工厂函数类型
type ConnectionFactory func(ctx context.Context) (Connection, error)

// Pool 通用连接池实现
type Pool struct {
	config  *PoolConfig
	factory ConnectionFactory
	logger  core.Logger
	metrics core.MetricsCollector

	connections chan Connection
	stats       *PoolStats
	statsMux    sync.RWMutex

	stopCh chan struct{}
	wg     sync.WaitGroup
	closed int32
}

// NewPool 创建新的连接池
func NewPool(config *PoolConfig, factory ConnectionFactory, logger core.Logger, metrics core.MetricsCollector) (*Pool, error) {
	if config == nil {
		config = DefaultPoolConfig()
	}

	if factory == nil {
		return nil, fmt.Errorf("connection factory cannot be nil")
	}

	pool := &Pool{
		config:      config,
		factory:     factory,
		logger:      logger,
		metrics:     metrics,
		connections: make(chan Connection, config.MaxConnections),
		stats: &PoolStats{
			LastActivity: time.Now(),
		},
		stopCh: make(chan struct{}),
	}

	// 预创建最小连接数
	ctx, cancel := context.WithTimeout(context.Background(), config.ConnectionTimeout)
	defer cancel()

	for i := 0; i < config.MinConnections; i++ {
		conn, err := factory(ctx)
		if err != nil {
			pool.Close()
			return nil, fmt.Errorf("failed to create initial connection: %w", err)
		}

		pool.connections <- conn
		atomic.AddInt32(&pool.stats.TotalConnections, 1)
		atomic.AddInt32(&pool.stats.IdleConnections, 1)
		atomic.AddInt64(&pool.stats.CreatedCount, 1)
	}

	// 启动后台任务
	pool.wg.Add(2)
	go pool.idleChecker()
	if config.HealthCheckEnabled {
		go pool.healthChecker()
	}

	logger.Info("Connection pool initialized",
		"min_connections", config.MinConnections,
		"max_connections", config.MaxConnections,
		"initial_connections", config.MinConnections)

	return pool, nil
}

// Get 获取连接
func (p *Pool) Get(ctx context.Context) (Connection, error) {
	if atomic.LoadInt32(&p.closed) == 1 {
		return nil, fmt.Errorf("connection pool is closed")
	}

	atomic.AddInt32(&p.stats.WaitingRequests, 1)
	defer atomic.AddInt32(&p.stats.WaitingRequests, -1)

	// 记录指标
	start := time.Now()
	defer func() {
		if p.metrics != nil {
			p.metrics.RecordHistogram("connection_pool_get_duration_seconds",
				time.Since(start).Seconds(),
				map[string]string{"operation": "get"})
		}
	}()

	// 尝试从池中获取连接
	select {
	case conn := <-p.connections:
		if conn.IsValid() {
			conn.SetLastUsed(time.Now())
			atomic.AddInt32(&p.stats.IdleConnections, -1)
			atomic.AddInt32(&p.stats.ActiveConnections, 1)
			p.updateLastActivity()
			return conn, nil
		}
		// 连接无效，关闭并创建新连接
		conn.Close()
		atomic.AddInt32(&p.stats.TotalConnections, -1)
		atomic.AddInt64(&p.stats.ClosedCount, 1)

	case <-ctx.Done():
		atomic.AddInt64(&p.stats.ErrorCount, 1)
		return nil, ctx.Err()

	default:
		// 池为空，尝试创建新连接
	}

	// 检查是否可以创建新连接
	if atomic.LoadInt32(&p.stats.TotalConnections) >= int32(p.config.MaxConnections) {
		// 等待连接可用
		select {
		case conn := <-p.connections:
			if conn.IsValid() {
				conn.SetLastUsed(time.Now())
				atomic.AddInt32(&p.stats.IdleConnections, -1)
				atomic.AddInt32(&p.stats.ActiveConnections, 1)
				p.updateLastActivity()
				return conn, nil
			}
			// 连接无效，关闭
			conn.Close()
			atomic.AddInt32(&p.stats.TotalConnections, -1)
			atomic.AddInt64(&p.stats.ClosedCount, 1)

		case <-ctx.Done():
			atomic.AddInt64(&p.stats.ErrorCount, 1)
			return nil, ctx.Err()
		}
	}

	// 创建新连接
	conn, err := p.factory(ctx)
	if err != nil {
		atomic.AddInt64(&p.stats.ErrorCount, 1)
		if p.metrics != nil {
			p.metrics.IncrementCounter("connection_pool_errors_total",
				map[string]string{"type": "create_failed"})
		}
		return nil, fmt.Errorf("failed to create connection: %w", err)
	}

	conn.SetLastUsed(time.Now())
	atomic.AddInt32(&p.stats.TotalConnections, 1)
	atomic.AddInt32(&p.stats.ActiveConnections, 1)
	atomic.AddInt64(&p.stats.CreatedCount, 1)
	p.updateLastActivity()

	if p.metrics != nil {
		p.metrics.IncrementCounter("connection_pool_connections_created_total",
			map[string]string{})
	}

	return conn, nil
}

// Put 归还连接
func (p *Pool) Put(conn Connection) error {
	if atomic.LoadInt32(&p.closed) == 1 {
		conn.Close()
		return fmt.Errorf("connection pool is closed")
	}

	if conn == nil {
		return fmt.Errorf("connection cannot be nil")
	}

	// 检查连接是否有效
	if !conn.IsValid() {
		conn.Close()
		atomic.AddInt32(&p.stats.TotalConnections, -1)
		atomic.AddInt32(&p.stats.ActiveConnections, -1)
		atomic.AddInt64(&p.stats.ClosedCount, 1)
		return nil
	}

	// 尝试归还到池中
	select {
	case p.connections <- conn:
		atomic.AddInt32(&p.stats.ActiveConnections, -1)
		atomic.AddInt32(&p.stats.IdleConnections, 1)
		p.updateLastActivity()
		return nil

	default:
		// 池已满，关闭连接
		conn.Close()
		atomic.AddInt32(&p.stats.TotalConnections, -1)
		atomic.AddInt32(&p.stats.ActiveConnections, -1)
		atomic.AddInt64(&p.stats.ClosedCount, 1)
		return nil
	}
}

// Stats 获取连接池统计信息
func (p *Pool) Stats() *PoolStats {
	p.statsMux.RLock()
	defer p.statsMux.RUnlock()

	stats := *p.stats
	return &stats
}

// Close 关闭连接池
func (p *Pool) Close() error {
	if !atomic.CompareAndSwapInt32(&p.closed, 0, 1) {
		return nil // 已经关闭
	}

	close(p.stopCh)
	p.wg.Wait()

	// 关闭所有连接
	close(p.connections)
	for conn := range p.connections {
		conn.Close()
	}

	p.logger.Info("Connection pool closed")
	return nil
}

// idleChecker 空闲连接检查器
func (p *Pool) idleChecker() {
	defer p.wg.Done()

	ticker := time.NewTicker(p.config.IdleCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.cleanupIdleConnections()

		case <-p.stopCh:
			return
		}
	}
}

// healthChecker 健康检查器
func (p *Pool) healthChecker() {
	defer p.wg.Done()

	ticker := time.NewTicker(p.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.performHealthCheck()

		case <-p.stopCh:
			return
		}
	}
}

// cleanupIdleConnections 清理空闲连接
func (p *Pool) cleanupIdleConnections() {
	now := time.Now()
	var toClose []Connection

	// 收集需要关闭的连接
	for {
		select {
		case conn := <-p.connections:
			if !conn.IsValid() ||
				now.Sub(conn.LastUsed()) > p.config.MaxIdleTime ||
				now.Sub(conn.LastUsed()) > p.config.MaxLifetime {
				toClose = append(toClose, conn)
			} else {
				// 连接仍然有效，放回池中
				select {
				case p.connections <- conn:
				default:
					// 池已满，关闭连接
					toClose = append(toClose, conn)
				}
			}

		default:
			// 没有更多连接
			goto cleanup
		}
	}

cleanup:
	// 关闭无效连接
	for _, conn := range toClose {
		conn.Close()
		atomic.AddInt32(&p.stats.TotalConnections, -1)
		atomic.AddInt32(&p.stats.IdleConnections, -1)
		atomic.AddInt64(&p.stats.ClosedCount, 1)
	}

	if len(toClose) > 0 {
		p.logger.Debug("Cleaned up idle connections", "count", len(toClose))
		if p.metrics != nil {
			p.metrics.IncrementCounter("connection_pool_connections_cleaned_total",
				map[string]string{"reason": "idle"})
		}
	}

	// 确保最小连接数
	p.ensureMinConnections()
}

// performHealthCheck 执行健康检查
func (p *Pool) performHealthCheck() {
	stats := p.Stats()

	// 记录连接池指标
	if p.metrics != nil {
		p.metrics.SetGauge("connection_pool_total_connections",
			float64(stats.TotalConnections), map[string]string{})
		p.metrics.SetGauge("connection_pool_active_connections",
			float64(stats.ActiveConnections), map[string]string{})
		p.metrics.SetGauge("connection_pool_idle_connections",
			float64(stats.IdleConnections), map[string]string{})
		p.metrics.SetGauge("connection_pool_waiting_requests",
			float64(stats.WaitingRequests), map[string]string{})
	}

	// 检查连接池健康状态
	if stats.TotalConnections == 0 {
		p.logger.Warn("Connection pool has no connections")
		p.ensureMinConnections()
	}

	if stats.WaitingRequests > int32(p.config.MaxConnections/2) {
		p.logger.Warn("High number of waiting requests",
			"waiting", stats.WaitingRequests,
			"max_connections", p.config.MaxConnections)
	}
}

// ensureMinConnections 确保最小连接数
func (p *Pool) ensureMinConnections() {
	current := atomic.LoadInt32(&p.stats.TotalConnections)
	needed := int32(p.config.MinConnections) - current

	if needed <= 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), p.config.ConnectionTimeout)
	defer cancel()

	for i := int32(0); i < needed; i++ {
		conn, err := p.factory(ctx)
		if err != nil {
			p.logger.Error("Failed to create connection for min pool size", "error", err)
			break
		}

		select {
		case p.connections <- conn:
			atomic.AddInt32(&p.stats.TotalConnections, 1)
			atomic.AddInt32(&p.stats.IdleConnections, 1)
			atomic.AddInt64(&p.stats.CreatedCount, 1)

		default:
			// 池已满
			conn.Close()
			break
		}
	}
}

// updateLastActivity 更新最后活动时间
func (p *Pool) updateLastActivity() {
	p.statsMux.Lock()
	p.stats.LastActivity = time.Now()
	p.statsMux.Unlock()
}

// LoadBalancer 负载均衡器接口
type LoadBalancer interface {
	Next() int
	Update(index int, weight float64)
	Remove(index int)
	Add(weight float64) int
}

// RoundRobinBalancer 轮询负载均衡器
type RoundRobinBalancer struct {
	current int32
	count   int32
	mutex   sync.RWMutex
}

// NewRoundRobinBalancer 创建轮询负载均衡器
func NewRoundRobinBalancer(count int) *RoundRobinBalancer {
	return &RoundRobinBalancer{
		count: int32(count),
	}
}

// Next 获取下一个索引
func (rb *RoundRobinBalancer) Next() int {
	rb.mutex.RLock()
	count := rb.count
	rb.mutex.RUnlock()

	if count == 0 {
		return -1
	}

	next := atomic.AddInt32(&rb.current, 1)
	return int((next - 1) % count)
}

// Update 更新权重（轮询不使用权重）
func (rb *RoundRobinBalancer) Update(index int, weight float64) {
	// 轮询负载均衡器不使用权重
}

// Remove 移除节点
func (rb *RoundRobinBalancer) Remove(index int) {
	rb.mutex.Lock()
	defer rb.mutex.Unlock()

	if rb.count > 0 {
		rb.count--
	}
}

// Add 添加节点
func (rb *RoundRobinBalancer) Add(weight float64) int {
	rb.mutex.Lock()
	defer rb.mutex.Unlock()

	index := int(rb.count)
	rb.count++
	return index
}

// WeightedBalancer 加权负载均衡器
type WeightedBalancer struct {
	weights []float64
	total   float64
	mutex   sync.RWMutex
}

// NewWeightedBalancer 创建加权负载均衡器
func NewWeightedBalancer() *WeightedBalancer {
	return &WeightedBalancer{
		weights: make([]float64, 0),
	}
}

// Next 获取下一个索引
func (wb *WeightedBalancer) Next() int {
	wb.mutex.RLock()
	defer wb.mutex.RUnlock()

	if len(wb.weights) == 0 || wb.total == 0 {
		return -1
	}

	// 简化的加权轮询实现
	// 在实际应用中，可以使用更复杂的算法如平滑加权轮询
	target := time.Now().UnixNano() % int64(wb.total*1000)
	current := float64(0)

	for i, weight := range wb.weights {
		current += weight * 1000
		if float64(target) < current {
			return i
		}
	}

	return len(wb.weights) - 1
}

// Update 更新权重
func (wb *WeightedBalancer) Update(index int, weight float64) {
	wb.mutex.Lock()
	defer wb.mutex.Unlock()

	if index >= 0 && index < len(wb.weights) {
		wb.total -= wb.weights[index]
		wb.weights[index] = weight
		wb.total += weight
	}
}

// Remove 移除节点
func (wb *WeightedBalancer) Remove(index int) {
	wb.mutex.Lock()
	defer wb.mutex.Unlock()

	if index >= 0 && index < len(wb.weights) {
		wb.total -= wb.weights[index]
		wb.weights = append(wb.weights[:index], wb.weights[index+1:]...)
	}
}

// Add 添加节点
func (wb *WeightedBalancer) Add(weight float64) int {
	wb.mutex.Lock()
	defer wb.mutex.Unlock()

	wb.weights = append(wb.weights, weight)
	wb.total += weight
	return len(wb.weights) - 1
}

// RateLimiter 速率限制器接口
type RateLimiter interface {
	Allow() bool
	Wait(ctx context.Context) error
	Limit() float64
	Burst() int
}

// TokenBucketLimiter 令牌桶限流器
type TokenBucketLimiter struct {
	tokens   float64
	capacity float64
	rate     float64
	lastTime time.Time
	mutex    sync.Mutex
}

// NewTokenBucketLimiter 创建令牌桶限流器
func NewTokenBucketLimiter(rate float64, capacity int) *TokenBucketLimiter {
	return &TokenBucketLimiter{
		tokens:   float64(capacity),
		capacity: float64(capacity),
		rate:     rate,
		lastTime: time.Now(),
	}
}

// Allow 检查是否允许请求
func (tbl *TokenBucketLimiter) Allow() bool {
	tbl.mutex.Lock()
	defer tbl.mutex.Unlock()

	now := time.Now()
	elapsed := now.Sub(tbl.lastTime).Seconds()
	tbl.lastTime = now

	// 添加令牌
	tbl.tokens += elapsed * tbl.rate
	if tbl.tokens > tbl.capacity {
		tbl.tokens = tbl.capacity
	}

	// 检查是否有足够的令牌
	if tbl.tokens >= 1.0 {
		tbl.tokens -= 1.0
		return true
	}

	return false
}

// Wait 等待直到可以处理请求
func (tbl *TokenBucketLimiter) Wait(ctx context.Context) error {
	for {
		if tbl.Allow() {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Millisecond * 10):
			// 短暂等待后重试
		}
	}
}

// Limit 获取速率限制
func (tbl *TokenBucketLimiter) Limit() float64 {
	return tbl.rate
}

// Burst 获取突发容量
func (tbl *TokenBucketLimiter) Burst() int {
	return int(tbl.capacity)
}
