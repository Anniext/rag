// 本文件实现了一个用于管理大语言模型（LLM）连接的连接池，支持连接的创建、获取、归还、健康检查和统计信息收集。
// 主要功能包括：
// 1. 连接池初始化与预创建连接。
// 2. 动态获取和归还连接，支持超时和健康检查。
// 3. 连接池的健康检查机制，定期检测连接状态。
// 4. 连接池统计信息收集与查询。
// 5. 连接池关闭与资源清理。

package langchain

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// connectionPoolImpl 是连接池的具体实现，负责管理所有连接的生命周期。
type connectionPoolImpl struct {
	config      *ConfigManager            // 配置管理器，包含连接池和客户端配置
	logger      *zap.Logger               // 日志记录器
	connections chan *LLMConnection       // 可用连接的通道，实现池化
	allConns    map[string]*LLMConnection // 所有连接的映射，便于管理和健康检查
	mutex       sync.RWMutex              // 读写锁，保护并发访问
	closed      bool                      // 标记连接池是否已关闭

	// 健康检查相关
	healthCheckTicker *time.Ticker // 健康检查定时器
	healthCheckDone   chan bool    // 健康检查停止信号

	// 统计信息
	createdCount  int64 // 创建的连接数
	borrowedCount int64 // 借出的连接数
	returnedCount int64 // 归还的连接数
	errorCount    int64 // 错误次数
}

// NewConnectionPool 创建并初始化连接池。
// 参数：config 配置管理器，logger 日志记录器。
// 返回：连接池实例或错误。
func NewConnectionPool(config *ConfigManager, logger *zap.Logger) (ConnectionPool, error) {
	poolConfig := config.GetPoolConfig()
	if poolConfig == nil {
		return nil, fmt.Errorf("pool config is required")
	}

	pool := &connectionPoolImpl{
		config:          config,
		logger:          logger,
		connections:     make(chan *LLMConnection, poolConfig.MaxConnections), // 初始化连接通道
		allConns:        make(map[string]*LLMConnection),                      // 初始化连接映射
		healthCheckDone: make(chan bool),                                      // 初始化健康检查信号通道
	}

	// 预创建部分连接
	if err := pool.initializeConnections(); err != nil {
		return nil, fmt.Errorf("failed to initialize connections: %w", err)
	}

	// 启动健康检查协程
	pool.startHealthCheck()

	logger.Info("Connection pool created",
		zap.Int("max_connections", poolConfig.MaxConnections),
		zap.Duration("max_idle_time", poolConfig.MaxIdleTime),
	)

	return pool, nil
}

// initializeConnections 初始化连接池中的连接。
// 按配置预创建一部分连接，至少创建一个。
func (p *connectionPoolImpl) initializeConnections() error {
	poolConfig := p.config.GetPoolConfig()
	clientConfig := p.config.GetClientConfig()

	// 计算初始连接数，默认为最大连接数的一半，至少为1
	initialCount := poolConfig.MaxConnections / 2
	if initialCount == 0 {
		initialCount = 1
	}

	for i := 0; i < initialCount; i++ {
		conn, err := p.createConnection()
		if err != nil {
			p.logger.Error("Failed to create initial connection",
				zap.Int("index", i),
				zap.Error(err),
			)
			continue
		}

		p.connections <- conn      // 放入连接池通道
		p.allConns[conn.ID] = conn // 记录到连接映射
		p.createdCount++           // 统计创建数
	}

	if len(p.allConns) == 0 {
		return fmt.Errorf("failed to create any connections")
	}

	p.logger.Info("Initialized connection pool",
		zap.Int("created_connections", len(p.allConns)),
		zap.String("provider", clientConfig.Provider),
		zap.String("model", clientConfig.Model),
	)

	return nil
}

// createConnection 创建新连接
func (p *connectionPoolImpl) createConnection() (*LLMConnection, error) {
	clientConfig := p.config.GetClientConfig()

	llm, err := createLLMClient(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM client: %w", err)
	}

	conn := &LLMConnection{
		ID:         uuid.New().String(),
		LLM:        llm,
		CreatedAt:  time.Now(),
		LastUsedAt: time.Now(),
		IsHealthy:  true,
	}

	return conn, nil
}

// GetConnection 获取连接
func (p *connectionPoolImpl) GetConnection(ctx context.Context) (*LLMConnection, error) {
	p.mutex.RLock()
	if p.closed {
		p.mutex.RUnlock()
		return nil, fmt.Errorf("connection pool is closed")
	}
	p.mutex.RUnlock()

	poolConfig := p.config.GetPoolConfig()

	// 尝试从池中获取连接
	select {
	case conn := <-p.connections:
		p.borrowedCount++

		// 检查连接是否健康
		if !conn.IsHealthy {
			p.logger.Warn("Got unhealthy connection, creating new one", zap.String("conn_id", conn.ID))

			// 尝试创建新连接
			newConn, err := p.createConnection()
			if err != nil {
				// 如果创建失败，仍然返回原连接，让调用者处理
				p.logger.Error("Failed to create replacement connection", zap.Error(err))
				return conn, nil
			}

			// 替换连接
			p.mutex.Lock()
			delete(p.allConns, conn.ID)
			p.allConns[newConn.ID] = newConn
			p.mutex.Unlock()

			return newConn, nil
		}

		return conn, nil

	case <-ctx.Done():
		p.errorCount++
		return nil, fmt.Errorf("context cancelled while waiting for connection")

	case <-time.After(poolConfig.ConnectionTimeout):
		p.errorCount++

		// 超时后尝试创建新连接（如果池未满）
		p.mutex.Lock()
		currentSize := len(p.allConns)
		maxSize := poolConfig.MaxConnections
		p.mutex.Unlock()

		if currentSize < maxSize {
			p.logger.Warn("Connection timeout, creating new connection",
				zap.Int("current_size", currentSize),
				zap.Int("max_size", maxSize),
			)

			conn, err := p.createConnection()
			if err != nil {
				return nil, fmt.Errorf("connection timeout and failed to create new connection: %w", err)
			}

			p.mutex.Lock()
			p.allConns[conn.ID] = conn
			p.createdCount++
			p.borrowedCount++
			p.mutex.Unlock()

			return conn, nil
		}

		return nil, fmt.Errorf("connection timeout after %v", poolConfig.ConnectionTimeout)
	}
}

// ReturnConnection 归还连接
func (p *connectionPoolImpl) ReturnConnection(conn *LLMConnection) {
	if conn == nil {
		return
	}

	p.mutex.RLock()
	if p.closed {
		p.mutex.RUnlock()
		return
	}
	p.mutex.RUnlock()

	// 检查连接是否仍在池中
	p.mutex.RLock()
	_, exists := p.allConns[conn.ID]
	p.mutex.RUnlock()

	if !exists {
		p.logger.Warn("Attempting to return unknown connection", zap.String("conn_id", conn.ID))
		return
	}

	// 检查连接是否空闲太久
	poolConfig := p.config.GetPoolConfig()
	if conn.IsIdle(poolConfig.MaxIdleTime) {
		p.logger.Debug("Connection idle too long, removing",
			zap.String("conn_id", conn.ID),
			zap.Duration("idle_time", time.Since(conn.LastUsedAt)),
		)

		p.mutex.Lock()
		delete(p.allConns, conn.ID)
		p.mutex.Unlock()

		return
	}

	// 归还连接到池中
	select {
	case p.connections <- conn:
		p.returnedCount++
	default:
		// 池已满，丢弃连接
		p.logger.Debug("Connection pool full, discarding connection", zap.String("conn_id", conn.ID))

		p.mutex.Lock()
		delete(p.allConns, conn.ID)
		p.mutex.Unlock()
	}
}

// startHealthCheck 启动健康检查
func (p *connectionPoolImpl) startHealthCheck() {
	poolConfig := p.config.GetPoolConfig()
	if poolConfig.HealthCheckPeriod <= 0 {
		return
	}

	p.healthCheckTicker = time.NewTicker(poolConfig.HealthCheckPeriod)

	go func() {
		defer p.healthCheckTicker.Stop()

		for {
			select {
			case <-p.healthCheckTicker.C:
				p.performHealthCheck()

			case <-p.healthCheckDone:
				return
			}
		}
	}()
}

// performHealthCheck 执行健康检查
func (p *connectionPoolImpl) performHealthCheck() {
	p.mutex.RLock()
	connections := make([]*LLMConnection, 0, len(p.allConns))
	for _, conn := range p.allConns {
		connections = append(connections, conn)
	}
	p.mutex.RUnlock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for _, conn := range connections {
		go func(c *LLMConnection) {
			if err := c.HealthCheck(ctx); err != nil {
				p.logger.Warn("Connection health check failed",
					zap.String("conn_id", c.ID),
					zap.Error(err),
				)
			}
		}(conn)
	}
}

// GetStats 获取连接池统计信息
func (p *connectionPoolImpl) GetStats() map[string]interface{} {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	activeConnections := len(p.connections)
	totalConnections := len(p.allConns)

	healthyCount := 0
	for _, conn := range p.allConns {
		if conn.IsHealthy {
			healthyCount++
		}
	}

	return map[string]interface{}{
		"active_connections":  activeConnections,
		"total_connections":   totalConnections,
		"healthy_connections": healthyCount,
		"created_count":       p.createdCount,
		"borrowed_count":      p.borrowedCount,
		"returned_count":      p.returnedCount,
		"error_count":         p.errorCount,
		"pool_utilization":    float64(totalConnections-activeConnections) / float64(totalConnections),
	}
}

// Close 关闭连接池
func (p *connectionPoolImpl) Close() error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.closed {
		return nil
	}

	p.closed = true

	// 停止健康检查
	if p.healthCheckTicker != nil {
		close(p.healthCheckDone)
	}

	// 关闭所有连接
	close(p.connections)

	// 清理连接映射
	connectionCount := len(p.allConns)
	p.allConns = make(map[string]*LLMConnection)

	p.logger.Info("Connection pool closed",
		zap.Int("closed_connections", connectionCount),
	)

	return nil
}

// ConnectionPoolStats 连接池统计信息
type ConnectionPoolStats struct {
	ActiveConnections  int     `json:"active_connections"`
	TotalConnections   int     `json:"total_connections"`
	HealthyConnections int     `json:"healthy_connections"`
	CreatedCount       int64   `json:"created_count"`
	BorrowedCount      int64   `json:"borrowed_count"`
	ReturnedCount      int64   `json:"returned_count"`
	ErrorCount         int64   `json:"error_count"`
	PoolUtilization    float64 `json:"pool_utilization"`
}

// GetDetailedStats 获取详细统计信息
func (p *connectionPoolImpl) GetDetailedStats() *ConnectionPoolStats {
	stats := p.GetStats()

	return &ConnectionPoolStats{
		ActiveConnections:  stats["active_connections"].(int),
		TotalConnections:   stats["total_connections"].(int),
		HealthyConnections: stats["healthy_connections"].(int),
		CreatedCount:       p.createdCount,
		BorrowedCount:      p.borrowedCount,
		ReturnedCount:      p.returnedCount,
		ErrorCount:         p.errorCount,
		PoolUtilization:    stats["pool_utilization"].(float64),
	}
}
