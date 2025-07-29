package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"pumppill/rag/core"
)

// ConnectionState 连接状态枚举
type ConnectionState int

const (
	StateDisconnected ConnectionState = iota
	StateConnecting
	StateConnected
	StateReconnecting
	StateClosed
)

// String 返回连接状态的字符串表示
func (s ConnectionState) String() string {
	switch s {
	case StateDisconnected:
		return "disconnected"
	case StateConnecting:
		return "connecting"
	case StateConnected:
		return "connected"
	case StateReconnecting:
		return "reconnecting"
	case StateClosed:
		return "closed"
	default:
		return "unknown"
	}
}

// Connection 表示一个 MCP 连接
type Connection struct {
	id          string
	conn        net.Conn
	state       int32 // 使用 atomic 操作的连接状态
	lastPing    time.Time
	lastPong    time.Time
	createdAt   time.Time
	metadata    map[string]interface{}
	sendChan    chan []byte
	receiveChan chan []byte
	closeChan   chan struct{}
	mu          sync.RWMutex
	logger      core.Logger
}

// NewConnection 创建新的连接实例
func NewConnection(id string, conn net.Conn, logger core.Logger) *Connection {
	return &Connection{
		id:          id,
		conn:        conn,
		state:       int32(StateConnected),
		lastPing:    time.Now(),
		lastPong:    time.Now(),
		createdAt:   time.Now(),
		metadata:    make(map[string]interface{}),
		sendChan:    make(chan []byte, 100),
		receiveChan: make(chan []byte, 100),
		closeChan:   make(chan struct{}),
		logger:      logger,
	}
}

// ID 返回连接 ID
func (c *Connection) ID() string {
	return c.id
}

// State 返回当前连接状态
func (c *Connection) State() ConnectionState {
	return ConnectionState(atomic.LoadInt32(&c.state))
}

// SetState 设置连接状态
func (c *Connection) SetState(state ConnectionState) {
	atomic.StoreInt32(&c.state, int32(state))
}

// IsConnected 检查连接是否处于连接状态
func (c *Connection) IsConnected() bool {
	return c.State() == StateConnected
}

// Send 发送数据到连接
func (c *Connection) Send(data []byte) error {
	if !c.IsConnected() {
		return fmt.Errorf("connection %s is not connected", c.id)
	}

	select {
	case c.sendChan <- data:
		return nil
	case <-c.closeChan:
		return fmt.Errorf("connection %s is closed", c.id)
	default:
		return fmt.Errorf("send buffer full for connection %s", c.id)
	}
}

// Receive 从连接接收数据
func (c *Connection) Receive() ([]byte, error) {
	select {
	case data := <-c.receiveChan:
		return data, nil
	case <-c.closeChan:
		return nil, fmt.Errorf("connection %s is closed", c.id)
	}
}

// Close 关闭连接
func (c *Connection) Close() error {
	c.SetState(StateClosed)

	// 安全关闭通道，避免重复关闭
	select {
	case <-c.closeChan:
		// 通道已经关闭
	default:
		close(c.closeChan)
	}

	return c.conn.Close()
}

// GetMetadata 获取连接元数据
func (c *Connection) GetMetadata(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	value, exists := c.metadata[key]
	return value, exists
}

// SetMetadata 设置连接元数据
func (c *Connection) SetMetadata(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metadata[key] = value
}

// UpdatePing 更新 ping 时间
func (c *Connection) UpdatePing() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastPing = time.Now()
}

// UpdatePong 更新 pong 时间
func (c *Connection) UpdatePong() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastPong = time.Now()
}

// IsAlive 检查连接是否存活（基于心跳）
func (c *Connection) IsAlive(timeout time.Duration) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return time.Since(c.lastPong) < timeout
}

// startIOLoop 启动 IO 循环
func (c *Connection) startIOLoop(ctx context.Context) {
	go c.readLoop(ctx)
	go c.writeLoop(ctx)
}

// readLoop 读取循环
func (c *Connection) readLoop(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			c.logger.Error("read loop panic", "connection_id", c.id, "error", r)
		}
		c.Close()
	}()

	buffer := make([]byte, 4096)
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.closeChan:
			return
		default:
			c.conn.SetReadDeadline(time.Now().Add(30 * time.Second))
			n, err := c.conn.Read(buffer)
			if err != nil {
				if err != io.EOF {
					c.logger.Error("read error", "connection_id", c.id, "error", err)
				}
				return
			}

			if n > 0 {
				data := make([]byte, n)
				copy(data, buffer[:n])
				select {
				case c.receiveChan <- data:
				case <-c.closeChan:
					return
				}
			}
		}
	}
}

// writeLoop 写入循环
func (c *Connection) writeLoop(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			c.logger.Error("write loop panic", "connection_id", c.id, "error", r)
		}
		c.Close()
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.closeChan:
			return
		case data := <-c.sendChan:
			c.conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
			_, err := c.conn.Write(data)
			if err != nil {
				c.logger.Error("write error", "connection_id", c.id, "error", err)
				return
			}
		}
	}
}

// ConnectionPool 连接池结构
type ConnectionPool struct {
	connections map[string]*Connection
	mu          sync.RWMutex
	maxSize     int
	logger      core.Logger
	metrics     core.MetricsCollector
}

// NewConnectionPool 创建新的连接池
func NewConnectionPool(maxSize int, logger core.Logger, metrics core.MetricsCollector) *ConnectionPool {
	return &ConnectionPool{
		connections: make(map[string]*Connection),
		maxSize:     maxSize,
		logger:      logger,
		metrics:     metrics,
	}
}

// Add 添加连接到池中
func (p *ConnectionPool) Add(conn *Connection) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.connections) >= p.maxSize {
		return fmt.Errorf("connection pool is full (max: %d)", p.maxSize)
	}

	p.connections[conn.ID()] = conn
	p.metrics.SetGauge("mcp_connections_active", float64(len(p.connections)), nil)
	p.logger.Info("connection added to pool", "connection_id", conn.ID(), "total_connections", len(p.connections))

	return nil
}

// Remove 从池中移除连接
func (p *ConnectionPool) Remove(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if conn, exists := p.connections[id]; exists {
		conn.Close()
		delete(p.connections, id)
		p.metrics.SetGauge("mcp_connections_active", float64(len(p.connections)), nil)
		p.logger.Info("connection removed from pool", "connection_id", id, "total_connections", len(p.connections))
	}
}

// Get 获取指定 ID 的连接
func (p *ConnectionPool) Get(id string) (*Connection, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	conn, exists := p.connections[id]
	return conn, exists
}

// GetAll 获取所有连接
func (p *ConnectionPool) GetAll() []*Connection {
	p.mu.RLock()
	defer p.mu.RUnlock()

	connections := make([]*Connection, 0, len(p.connections))
	for _, conn := range p.connections {
		connections = append(connections, conn)
	}
	return connections
}

// Size 返回连接池大小
func (p *ConnectionPool) Size() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.connections)
}

// Broadcast 向所有连接广播消息
func (p *ConnectionPool) Broadcast(data []byte) {
	p.mu.RLock()
	connections := make([]*Connection, 0, len(p.connections))
	for _, conn := range p.connections {
		if conn.IsConnected() {
			connections = append(connections, conn)
		}
	}
	p.mu.RUnlock()

	for _, conn := range connections {
		if err := conn.Send(data); err != nil {
			p.logger.Error("broadcast failed", "connection_id", conn.ID(), "error", err)
		}
	}
}

// CleanupStaleConnections 清理过期连接
func (p *ConnectionPool) CleanupStaleConnections(timeout time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()

	staleConnections := make([]string, 0)
	for id, conn := range p.connections {
		if !conn.IsAlive(timeout) {
			staleConnections = append(staleConnections, id)
		}
	}

	for _, id := range staleConnections {
		if conn, exists := p.connections[id]; exists {
			conn.Close()
			delete(p.connections, id)
			p.logger.Info("stale connection removed", "connection_id", id)
		}
	}

	if len(staleConnections) > 0 {
		p.metrics.SetGauge("mcp_connections_active", float64(len(p.connections)), nil)
		p.metrics.IncrementCounter("mcp_connections_cleaned", map[string]string{
			"count": fmt.Sprintf("%d", len(staleConnections)),
		})
	}
}

// LoadBalancer 负载均衡器接口
type LoadBalancer interface {
	SelectConnection(connections []*Connection) *Connection
}

// RoundRobinBalancer 轮询负载均衡器
type RoundRobinBalancer struct {
	counter uint64
}

// NewRoundRobinBalancer 创建轮询负载均衡器
func NewRoundRobinBalancer() *RoundRobinBalancer {
	return &RoundRobinBalancer{}
}

// SelectConnection 选择连接（轮询方式）
func (b *RoundRobinBalancer) SelectConnection(connections []*Connection) *Connection {
	if len(connections) == 0 {
		return nil
	}

	index := atomic.AddUint64(&b.counter, 1) % uint64(len(connections))
	return connections[index]
}

// LeastConnectionsBalancer 最少连接负载均衡器
type LeastConnectionsBalancer struct{}

// NewLeastConnectionsBalancer 创建最少连接负载均衡器
func NewLeastConnectionsBalancer() *LeastConnectionsBalancer {
	return &LeastConnectionsBalancer{}
}

// SelectConnection 选择连接（最少连接方式）
func (b *LeastConnectionsBalancer) SelectConnection(connections []*Connection) *Connection {
	if len(connections) == 0 {
		return nil
	}

	// 简化实现：返回第一个连接
	// 在实际实现中，应该根据连接的负载情况选择
	return connections[0]
}

// ConnectionManager 连接管理器
type ConnectionManager struct {
	pool            *ConnectionPool
	balancer        LoadBalancer
	heartbeatTicker *time.Ticker
	cleanupTicker   *time.Ticker
	ctx             context.Context
	cancel          context.CancelFunc
	logger          core.Logger
	metrics         core.MetricsCollector
	config          *core.MCPConfig
}

// NewConnectionManager 创建连接管理器
func NewConnectionManager(config *core.MCPConfig, logger core.Logger, metrics core.MetricsCollector) *ConnectionManager {
	ctx, cancel := context.WithCancel(context.Background())

	return &ConnectionManager{
		pool:            NewConnectionPool(config.MaxConnections, logger, metrics),
		balancer:        NewRoundRobinBalancer(),
		heartbeatTicker: time.NewTicker(30 * time.Second),
		cleanupTicker:   time.NewTicker(60 * time.Second),
		ctx:             ctx,
		cancel:          cancel,
		logger:          logger,
		metrics:         metrics,
		config:          config,
	}
}

// Start 启动连接管理器
func (m *ConnectionManager) Start() {
	go m.heartbeatLoop()
	go m.cleanupLoop()
	m.logger.Info("connection manager started")
}

// Stop 停止连接管理器
func (m *ConnectionManager) Stop() {
	m.cancel()
	m.heartbeatTicker.Stop()
	m.cleanupTicker.Stop()

	// 关闭所有连接
	connections := m.pool.GetAll()
	for _, conn := range connections {
		conn.Close()
	}

	m.logger.Info("connection manager stopped")
}

// AddConnection 添加新连接
func (m *ConnectionManager) AddConnection(conn net.Conn) (*Connection, error) {
	id := fmt.Sprintf("conn_%d", time.Now().UnixNano())
	connection := NewConnection(id, conn, m.logger)

	if err := m.pool.Add(connection); err != nil {
		connection.Close()
		return nil, err
	}

	// 启动连接的 IO 循环
	connection.startIOLoop(m.ctx)

	m.metrics.IncrementCounter("mcp_connections_created", nil)
	return connection, nil
}

// RemoveConnection 移除连接
func (m *ConnectionManager) RemoveConnection(id string) {
	m.pool.Remove(id)
	m.metrics.IncrementCounter("mcp_connections_removed", nil)
}

// GetConnection 获取连接
func (m *ConnectionManager) GetConnection(id string) (*Connection, bool) {
	return m.pool.Get(id)
}

// SelectConnection 选择一个可用连接（负载均衡）
func (m *ConnectionManager) SelectConnection() *Connection {
	connections := m.pool.GetAll()
	activeConnections := make([]*Connection, 0)

	for _, conn := range connections {
		if conn.IsConnected() {
			activeConnections = append(activeConnections, conn)
		}
	}

	return m.balancer.SelectConnection(activeConnections)
}

// BroadcastMessage 广播消息到所有连接
func (m *ConnectionManager) BroadcastMessage(message *core.MCPMessage) error {
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	m.pool.Broadcast(data)
	m.metrics.IncrementCounter("mcp_messages_broadcast", nil)
	return nil
}

// SendMessage 发送消息到指定连接
func (m *ConnectionManager) SendMessage(connectionID string, message *core.MCPMessage) error {
	conn, exists := m.pool.Get(connectionID)
	if !exists {
		return fmt.Errorf("connection %s not found", connectionID)
	}

	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	if err := conn.Send(data); err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	m.metrics.IncrementCounter("mcp_messages_sent", map[string]string{
		"connection_id": connectionID,
	})
	return nil
}

// GetConnectionStats 获取连接统计信息
func (m *ConnectionManager) GetConnectionStats() map[string]interface{} {
	connections := m.pool.GetAll()
	stats := map[string]interface{}{
		"total_connections":  len(connections),
		"active_connections": 0,
		"connection_details": make([]map[string]interface{}, 0),
	}

	activeCount := 0
	for _, conn := range connections {
		if conn.IsConnected() {
			activeCount++
		}

		connStats := map[string]interface{}{
			"id":         conn.ID(),
			"state":      conn.State().String(),
			"created_at": conn.createdAt,
			"is_alive":   conn.IsAlive(60 * time.Second),
		}
		stats["connection_details"] = append(stats["connection_details"].([]map[string]interface{}), connStats)
	}

	stats["active_connections"] = activeCount
	return stats
}

// heartbeatLoop 心跳循环
func (m *ConnectionManager) heartbeatLoop() {
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-m.heartbeatTicker.C:
			m.sendHeartbeat()
		}
	}
}

// cleanupLoop 清理循环
func (m *ConnectionManager) cleanupLoop() {
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-m.cleanupTicker.C:
			m.pool.CleanupStaleConnections(90 * time.Second)
		}
	}
}

// sendHeartbeat 发送心跳
func (m *ConnectionManager) sendHeartbeat() {
	heartbeatMsg := &core.MCPMessage{
		Type:   "notification",
		Method: "heartbeat",
		Params: map[string]interface{}{
			"timestamp": time.Now().Unix(),
		},
	}

	if err := m.BroadcastMessage(heartbeatMsg); err != nil {
		m.logger.Error("failed to send heartbeat", "error", err)
	}
}

// SetLoadBalancer 设置负载均衡器
func (m *ConnectionManager) SetLoadBalancer(balancer LoadBalancer) {
	m.balancer = balancer
}
