package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Anniext/rag/core"
	"net"
	"sync"
	"time"
)

// EventType 事件类型
type EventType string

const (
	EventConnectionOpened EventType = "connection.opened"
	EventConnectionClosed EventType = "connection.closed"
	EventRequestReceived  EventType = "request.received"
	EventRequestProcessed EventType = "request.processed"
	EventNotificationSent EventType = "notification.sent"
	EventError            EventType = "error"
)

// Event 事件结构
type Event struct {
	Type         EventType              `json:"type"`
	Timestamp    time.Time              `json:"timestamp"`
	ConnectionID string                 `json:"connection_id,omitempty"`
	Data         map[string]interface{} `json:"data,omitempty"`
}

// EventSubscriber 事件订阅者接口
type EventSubscriber interface {
	OnEvent(event *Event)
	GetSubscribedEvents() []EventType
}

// EventBus 事件总线
type EventBus struct {
	subscribers map[EventType][]EventSubscriber
	mu          sync.RWMutex
	logger      core.Logger
}

// NewEventBus 创建事件总线
func NewEventBus(logger core.Logger) *EventBus {
	return &EventBus{
		subscribers: make(map[EventType][]EventSubscriber),
		logger:      logger,
	}
}

// Subscribe 订阅事件
func (b *EventBus) Subscribe(eventType EventType, subscriber EventSubscriber) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.subscribers[eventType] = append(b.subscribers[eventType], subscriber)
	b.logger.Info("event subscriber added", "event_type", eventType)
}

// Unsubscribe 取消订阅
func (b *EventBus) Unsubscribe(eventType EventType, subscriber EventSubscriber) {
	b.mu.Lock()
	defer b.mu.Unlock()

	subscribers := b.subscribers[eventType]
	for i, sub := range subscribers {
		if sub == subscriber {
			b.subscribers[eventType] = append(subscribers[:i], subscribers[i+1:]...)
			break
		}
	}
	b.logger.Info("event subscriber removed", "event_type", eventType)
}

// Publish 发布事件
func (b *EventBus) Publish(event *Event) {
	b.mu.RLock()
	subscribers := make([]EventSubscriber, len(b.subscribers[event.Type]))
	copy(subscribers, b.subscribers[event.Type])
	b.mu.RUnlock()

	// 异步通知订阅者
	for _, subscriber := range subscribers {
		go func(sub EventSubscriber) {
			defer func() {
				if r := recover(); r != nil {
					b.logger.Error("event subscriber panic", "event_type", event.Type, "panic", r)
				}
			}()
			sub.OnEvent(event)
		}(subscriber)
	}
}

// StatusReporter 状态报告器
type StatusReporter struct {
	server  *MCPServerImpl
	logger  core.Logger
	metrics core.MetricsCollector
}

// NewStatusReporter 创建状态报告器
func NewStatusReporter(server *MCPServerImpl, logger core.Logger, metrics core.MetricsCollector) *StatusReporter {
	return &StatusReporter{
		server:  server,
		logger:  logger,
		metrics: metrics,
	}
}

// OnEvent 处理事件
func (r *StatusReporter) OnEvent(event *Event) {
	switch event.Type {
	case EventConnectionOpened:
		r.metrics.IncrementCounter("mcp_connections_opened_total", nil)
		r.logger.Info("connection opened", "connection_id", event.ConnectionID)
	case EventConnectionClosed:
		r.metrics.IncrementCounter("mcp_connections_closed_total", nil)
		r.logger.Info("connection closed", "connection_id", event.ConnectionID)
	case EventRequestReceived:
		r.metrics.IncrementCounter("mcp_requests_received_total", nil)
	case EventRequestProcessed:
		r.metrics.IncrementCounter("mcp_requests_processed_total", nil)
	case EventNotificationSent:
		r.metrics.IncrementCounter("mcp_notifications_sent_total", nil)
	case EventError:
		r.metrics.IncrementCounter("mcp_errors_total", nil)
		r.logger.Error("mcp error event", "data", event.Data)
	}
}

// GetSubscribedEvents 获取订阅的事件类型
func (r *StatusReporter) GetSubscribedEvents() []EventType {
	return []EventType{
		EventConnectionOpened,
		EventConnectionClosed,
		EventRequestReceived,
		EventRequestProcessed,
		EventNotificationSent,
		EventError,
	}
}

// ProgressNotifier 进度通知器
type ProgressNotifier struct {
	connectionManager *ConnectionManager
	logger            core.Logger
}

// NewProgressNotifier 创建进度通知器
func NewProgressNotifier(connectionManager *ConnectionManager, logger core.Logger) *ProgressNotifier {
	return &ProgressNotifier{
		connectionManager: connectionManager,
		logger:            logger,
	}
}

// NotifyProgress 发送进度通知
func (n *ProgressNotifier) NotifyProgress(connectionID string, progress map[string]interface{}) error {
	notification := &core.MCPMessage{
		Type:   "notification",
		Method: "progress.update",
		Params: progress,
	}

	return n.connectionManager.SendMessage(connectionID, notification)
}

// BroadcastProgress 广播进度通知
func (n *ProgressNotifier) BroadcastProgress(progress map[string]interface{}) error {
	notification := &core.MCPMessage{
		Type:   "notification",
		Method: "progress.update",
		Params: progress,
	}

	return n.connectionManager.BroadcastMessage(notification)
}

// MCPServerImpl MCP 服务器实现
type MCPServerImpl struct {
	config            *core.MCPConfig
	connectionManager *ConnectionManager
	requestProcessor  *RequestProcessor
	asyncProcessor    *AsyncRequestProcessor
	handlerRegistry   *HandlerRegistry
	eventBus          *EventBus
	statusReporter    *StatusReporter
	progressNotifier  *ProgressNotifier
	listener          net.Listener
	logger            core.Logger
	metrics           core.MetricsCollector
	ctx               context.Context
	cancel            context.CancelFunc
	wg                sync.WaitGroup
	running           bool
	mu                sync.RWMutex
}

// NewMCPServer 创建 MCP 服务器
func NewMCPServer(
	config *core.MCPConfig,
	logger core.Logger,
	metrics core.MetricsCollector,
	security core.SecurityManager,
) *MCPServerImpl {
	ctx, cancel := context.WithCancel(context.Background())

	// 创建组件
	connectionManager := NewConnectionManager(config, logger, metrics)
	handlerRegistry := NewHandlerRegistry(logger)
	validator := NewDefaultRequestValidator(logger)
	rateLimiter := NewTokenBucketRateLimiter(100, 1000, time.Minute, logger) // 每分钟100个请求，桶容量1000

	requestProcessor := NewRequestProcessor(
		handlerRegistry,
		validator,
		rateLimiter,
		security,
		metrics,
		logger,
		config.Timeout,
	)

	asyncProcessor := NewAsyncRequestProcessor(requestProcessor, 10, logger) // 10个工作者
	eventBus := NewEventBus(logger)

	server := &MCPServerImpl{
		config:            config,
		connectionManager: connectionManager,
		requestProcessor:  requestProcessor,
		asyncProcessor:    asyncProcessor,
		handlerRegistry:   handlerRegistry,
		eventBus:          eventBus,
		logger:            logger,
		metrics:           metrics,
		ctx:               ctx,
		cancel:            cancel,
	}

	// 创建状态报告器和进度通知器
	server.statusReporter = NewStatusReporter(server, logger, metrics)
	server.progressNotifier = NewProgressNotifier(connectionManager, logger)

	// 订阅事件
	for _, eventType := range server.statusReporter.GetSubscribedEvents() {
		eventBus.Subscribe(eventType, server.statusReporter)
	}

	return server
}

// Start 启动服务器
func (s *MCPServerImpl) Start(ctx context.Context, addr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("server is already running")
	}

	// 创建监听器
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	s.listener = listener
	s.running = true

	// 启动组件
	s.connectionManager.Start()
	s.asyncProcessor.Start()

	// 启动连接接受循环
	s.wg.Add(1)
	go s.acceptLoop()

	s.logger.Info("MCP server started", "address", addr)
	return nil
}

// Stop 停止服务器
func (s *MCPServerImpl) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	s.running = false

	// 关闭监听器
	if s.listener != nil {
		s.listener.Close()
	}

	// 取消上下文
	s.cancel()

	// 停止组件
	s.asyncProcessor.Stop()
	s.connectionManager.Stop()

	// 等待所有 goroutines 完成
	s.wg.Wait()

	s.logger.Info("MCP server stopped")
	return nil
}

// RegisterHandler 注册处理器
func (s *MCPServerImpl) RegisterHandler(method string, handler core.MCPHandler) error {
	return s.handlerRegistry.Register(method, handler)
}

// BroadcastNotification 广播通知
func (s *MCPServerImpl) BroadcastNotification(notification *core.MCPNotification) error {
	message := &core.MCPMessage{
		Type:   "notification",
		Method: notification.Method,
		Params: notification.Params,
	}

	err := s.connectionManager.BroadcastMessage(message)
	if err == nil {
		s.eventBus.Publish(&Event{
			Type:      EventNotificationSent,
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"method": notification.Method,
			},
		})
	}

	return err
}

// GetConnectionStats 获取连接统计信息
func (s *MCPServerImpl) GetConnectionStats() map[string]interface{} {
	return s.connectionManager.GetConnectionStats()
}

// GetHandlerMethods 获取已注册的处理器方法
func (s *MCPServerImpl) GetHandlerMethods() []string {
	return s.handlerRegistry.GetAll()
}

// acceptLoop 连接接受循环
func (s *MCPServerImpl) acceptLoop() {
	defer s.wg.Done()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.ctx.Done():
				// 服务器正在关闭
				return
			default:
				s.logger.Error("failed to accept connection", "error", err)
				s.eventBus.Publish(&Event{
					Type:      EventError,
					Timestamp: time.Now(),
					Data: map[string]interface{}{
						"error": err.Error(),
						"type":  "accept_error",
					},
				})
				continue
			}
		}

		// 处理新连接
		s.wg.Add(1)
		go s.handleConnection(conn)
	}
}

// handleConnection 处理连接
func (s *MCPServerImpl) handleConnection(conn net.Conn) {
	defer s.wg.Done()

	// 添加连接到管理器
	connection, err := s.connectionManager.AddConnection(conn)
	if err != nil {
		s.logger.Error("failed to add connection", "error", err)
		conn.Close()
		return
	}

	connectionID := connection.ID()
	s.logger.Info("new connection established", "connection_id", connectionID)

	// 发布连接打开事件
	s.eventBus.Publish(&Event{
		Type:         EventConnectionOpened,
		Timestamp:    time.Now(),
		ConnectionID: connectionID,
	})

	// 处理连接消息
	defer func() {
		s.connectionManager.RemoveConnection(connectionID)
		s.eventBus.Publish(&Event{
			Type:         EventConnectionClosed,
			Timestamp:    time.Now(),
			ConnectionID: connectionID,
		})
	}()

	// 消息处理循环
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			// 接收消息
			data, err := connection.Receive()
			if err != nil {
				s.logger.Error("failed to receive message", "connection_id", connectionID, "error", err)
				return
			}

			// 解析消息
			var message core.MCPMessage
			if err := json.Unmarshal(data, &message); err != nil {
				s.logger.Error("failed to parse message", "connection_id", connectionID, "error", err)
				continue
			}

			// 处理消息
			s.handleMessage(connection, &message)
		}
	}
}

// handleMessage 处理消息
func (s *MCPServerImpl) handleMessage(connection *Connection, message *core.MCPMessage) {
	connectionID := connection.ID()

	switch message.Type {
	case "request":
		s.handleRequest(connection, message)
	case "notification":
		s.handleNotification(connection, message)
	default:
		s.logger.Warn("unknown message type", "type", message.Type, "connection_id", connectionID)
	}
}

// handleRequest 处理请求
func (s *MCPServerImpl) handleRequest(connection *Connection, message *core.MCPMessage) {
	connectionID := connection.ID()

	// 发布请求接收事件
	s.eventBus.Publish(&Event{
		Type:         EventRequestReceived,
		Timestamp:    time.Now(),
		ConnectionID: connectionID,
		Data: map[string]interface{}{
			"method": message.Method,
			"id":     message.ID,
		},
	})

	// 构建请求
	request := &core.MCPRequest{
		ID:     message.ID,
		Method: message.Method,
		Params: message.Params,
	}

	// 异步处理请求
	responseChan := s.asyncProcessor.ProcessAsync(request, connectionID)

	// 等待响应并发送
	go func() {
		select {
		case response := <-responseChan:
			s.sendResponse(connection, response)

			// 发布请求处理完成事件
			s.eventBus.Publish(&Event{
				Type:         EventRequestProcessed,
				Timestamp:    time.Now(),
				ConnectionID: connectionID,
				Data: map[string]interface{}{
					"method":  message.Method,
					"id":      message.ID,
					"success": response.Error == nil,
				},
			})

		case <-s.ctx.Done():
			// 服务器正在关闭
			return
		}
	}()
}

// handleNotification 处理通知
func (s *MCPServerImpl) handleNotification(connection *Connection, message *core.MCPMessage) {
	connectionID := connection.ID()

	s.logger.Info("notification received",
		"method", message.Method,
		"connection_id", connectionID)

	// 处理特殊通知
	switch message.Method {
	case "heartbeat":
		connection.UpdatePong()
	case "ping":
		// 响应 pong
		pongMessage := &core.MCPMessage{
			Type:   "notification",
			Method: "pong",
			Params: map[string]interface{}{
				"timestamp": time.Now().Unix(),
			},
		}
		s.sendMessage(connection, pongMessage)
	default:
		// 其他通知可以在这里处理
		s.logger.Debug("unhandled notification", "method", message.Method)
	}
}

// sendResponse 发送响应
func (s *MCPServerImpl) sendResponse(connection *Connection, response *core.MCPResponse) {
	message := &core.MCPMessage{
		Type:   "response",
		ID:     response.ID,
		Result: response.Result,
		Error:  response.Error,
	}

	s.sendMessage(connection, message)
}

// sendMessage 发送消息
func (s *MCPServerImpl) sendMessage(connection *Connection, message *core.MCPMessage) {
	data, err := json.Marshal(message)
	if err != nil {
		s.logger.Error("failed to marshal message", "error", err)
		return
	}

	if err := connection.Send(data); err != nil {
		s.logger.Error("failed to send message", "connection_id", connection.ID(), "error", err)
	}
}

// IsRunning 检查服务器是否正在运行
func (s *MCPServerImpl) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// GetEventBus 获取事件总线
func (s *MCPServerImpl) GetEventBus() *EventBus {
	return s.eventBus
}

// GetProgressNotifier 获取进度通知器
func (s *MCPServerImpl) GetProgressNotifier() *ProgressNotifier {
	return s.progressNotifier
}

// SubscribeToEvents 订阅事件
func (s *MCPServerImpl) SubscribeToEvents(eventType EventType, subscriber EventSubscriber) {
	s.eventBus.Subscribe(eventType, subscriber)
}

// UnsubscribeFromEvents 取消事件订阅
func (s *MCPServerImpl) UnsubscribeFromEvents(eventType EventType, subscriber EventSubscriber) {
	s.eventBus.Unsubscribe(eventType, subscriber)
}
