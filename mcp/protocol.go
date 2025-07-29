package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"pumppill/rag/core"
)

// ProtocolVersion MCP 协议版本
type ProtocolVersion struct {
	Major int `json:"major"`
	Minor int `json:"minor"`
	Patch int `json:"patch"`
}

// String 返回版本的字符串表示
func (v ProtocolVersion) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// IsCompatible 检查版本兼容性
func (v ProtocolVersion) IsCompatible(other ProtocolVersion) bool {
	// 主版本号必须相同，次版本号向后兼容
	return v.Major == other.Major && v.Minor >= other.Minor
}

// ParseVersion 解析版本字符串
func ParseVersion(versionStr string) (ProtocolVersion, error) {
	parts := strings.Split(versionStr, ".")
	if len(parts) != 3 {
		return ProtocolVersion{}, fmt.Errorf("invalid version format: %s", versionStr)
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return ProtocolVersion{}, fmt.Errorf("invalid major version: %s", parts[0])
	}

	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return ProtocolVersion{}, fmt.Errorf("invalid minor version: %s", parts[1])
	}

	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return ProtocolVersion{}, fmt.Errorf("invalid patch version: %s", parts[2])
	}

	return ProtocolVersion{
		Major: major,
		Minor: minor,
		Patch: patch,
	}, nil
}

// 支持的协议版本
var (
	CurrentProtocolVersion = ProtocolVersion{Major: 1, Minor: 0, Patch: 0}
	SupportedVersions      = []ProtocolVersion{
		{Major: 1, Minor: 0, Patch: 0},
	}
)

// MessageType MCP 消息类型枚举
type MessageType string

const (
	MessageTypeRequest      MessageType = "request"
	MessageTypeResponse     MessageType = "response"
	MessageTypeNotification MessageType = "notification"
	MessageTypeError        MessageType = "error"
)

// IsValid 检查消息类型是否有效
func (mt MessageType) IsValid() bool {
	switch mt {
	case MessageTypeRequest, MessageTypeResponse, MessageTypeNotification, MessageTypeError:
		return true
	default:
		return false
	}
}

// ProtocolError 协议错误类型
type ProtocolError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Error 实现 error 接口
func (e *ProtocolError) Error() string {
	return fmt.Sprintf("protocol error %d: %s", e.Code, e.Message)
}

// 预定义的协议错误码
const (
	ErrorCodeParseError     = -32700 // JSON 解析错误
	ErrorCodeInvalidRequest = -32600 // 无效请求
	ErrorCodeMethodNotFound = -32601 // 方法未找到
	ErrorCodeInvalidParams  = -32602 // 无效参数
	ErrorCodeInternalError  = -32603 // 内部错误
	ErrorCodeServerError    = -32000 // 服务器错误
	ErrorCodeVersionError   = -32001 // 版本不兼容错误
	ErrorCodeTimeout        = -32002 // 超时错误
	ErrorCodeRateLimit      = -32003 // 限流错误
)

// NewProtocolError 创建协议错误
func NewProtocolError(code int, message string, data any) *ProtocolError {
	return &ProtocolError{
		Code:    code,
		Message: message,
		Data:    data,
	}
}

// MessageParser 消息解析器接口
type MessageParser interface {
	ParseMessage(data []byte) (*core.MCPMessage, error)
	SerializeMessage(message *core.MCPMessage) ([]byte, error)
	ValidateMessage(message *core.MCPMessage) error
}

// JSONMessageParser JSON 消息解析器
type JSONMessageParser struct {
	logger core.Logger
}

// NewJSONMessageParser 创建 JSON 消息解析器
func NewJSONMessageParser(logger core.Logger) *JSONMessageParser {
	return &JSONMessageParser{
		logger: logger,
	}
}

// ParseMessage 解析消息
func (p *JSONMessageParser) ParseMessage(data []byte) (*core.MCPMessage, error) {
	if len(data) == 0 {
		return nil, NewProtocolError(ErrorCodeParseError, "Empty message", nil)
	}

	var message core.MCPMessage
	if err := json.Unmarshal(data, &message); err != nil {
		p.logger.Error("failed to parse JSON message", "error", err, "data", string(data))
		return nil, NewProtocolError(ErrorCodeParseError, "Invalid JSON", err.Error())
	}

	// 验证消息
	if err := p.ValidateMessage(&message); err != nil {
		return nil, err
	}

	return &message, nil
}

// SerializeMessage 序列化消息
func (p *JSONMessageParser) SerializeMessage(message *core.MCPMessage) ([]byte, error) {
	if message == nil {
		return nil, NewProtocolError(ErrorCodeInvalidRequest, "Message cannot be nil", nil)
	}

	// 验证消息
	if err := p.ValidateMessage(message); err != nil {
		return nil, err
	}

	data, err := json.Marshal(message)
	if err != nil {
		p.logger.Error("failed to serialize message", "error", err, "message", message)
		return nil, NewProtocolError(ErrorCodeInternalError, "Serialization failed", err.Error())
	}

	return data, nil
}

// ValidateMessage 验证消息
func (p *JSONMessageParser) ValidateMessage(message *core.MCPMessage) error {
	if message == nil {
		return NewProtocolError(ErrorCodeInvalidRequest, "Message cannot be nil", nil)
	}

	// 验证消息类型
	if !MessageType(message.Type).IsValid() {
		return NewProtocolError(ErrorCodeInvalidRequest, "Invalid message type", message.Type)
	}

	// 根据消息类型进行特定验证
	switch MessageType(message.Type) {
	case MessageTypeRequest:
		if message.ID == "" {
			return NewProtocolError(ErrorCodeInvalidRequest, "Request ID cannot be empty", nil)
		}
		if message.Method == "" {
			return NewProtocolError(ErrorCodeInvalidRequest, "Request method cannot be empty", nil)
		}
	case MessageTypeResponse:
		if message.ID == "" {
			return NewProtocolError(ErrorCodeInvalidRequest, "Response ID cannot be empty", nil)
		}
		// 响应必须有结果或错误，但不能同时有
		if message.Result == nil && message.Error == nil {
			return NewProtocolError(ErrorCodeInvalidRequest, "Response must have either result or error", nil)
		}
		if message.Result != nil && message.Error != nil {
			return NewProtocolError(ErrorCodeInvalidRequest, "Response cannot have both result and error", nil)
		}
	case MessageTypeNotification:
		if message.Method == "" {
			return NewProtocolError(ErrorCodeInvalidRequest, "Notification method cannot be empty", nil)
		}
		// 通知不应该有 ID
		if message.ID != "" {
			return NewProtocolError(ErrorCodeInvalidRequest, "Notification should not have ID", nil)
		}
	}

	return nil
}

// VersionNegotiator 版本协商器
type VersionNegotiator struct {
	supportedVersions []ProtocolVersion
	logger            core.Logger
}

// NewVersionNegotiator 创建版本协商器
func NewVersionNegotiator(supportedVersions []ProtocolVersion, logger core.Logger) *VersionNegotiator {
	return &VersionNegotiator{
		supportedVersions: supportedVersions,
		logger:            logger,
	}
}

// NegotiateVersion 协商版本
func (n *VersionNegotiator) NegotiateVersion(clientVersions []ProtocolVersion) (ProtocolVersion, error) {
	if len(clientVersions) == 0 {
		return ProtocolVersion{}, NewProtocolError(ErrorCodeVersionError, "No client versions provided", nil)
	}

	// 寻找最高的兼容版本
	var bestVersion ProtocolVersion
	var found bool

	for _, clientVersion := range clientVersions {
		for _, supportedVersion := range n.supportedVersions {
			if supportedVersion.IsCompatible(clientVersion) {
				if !found || supportedVersion.Major > bestVersion.Major ||
					(supportedVersion.Major == bestVersion.Major && supportedVersion.Minor > bestVersion.Minor) {
					bestVersion = supportedVersion
					found = true
				}
			}
		}
	}

	if !found {
		n.logger.Warn("no compatible version found", "client_versions", clientVersions, "supported_versions", n.supportedVersions)
		return ProtocolVersion{}, NewProtocolError(ErrorCodeVersionError, "No compatible version found", map[string]any{
			"client_versions":    clientVersions,
			"supported_versions": n.supportedVersions,
		})
	}

	n.logger.Info("version negotiated", "selected_version", bestVersion, "client_versions", clientVersions)
	return bestVersion, nil
}

// GetSupportedVersions 获取支持的版本列表
func (n *VersionNegotiator) GetSupportedVersions() []ProtocolVersion {
	return n.supportedVersions
}

// MessageRouter 消息路由器
type MessageRouter struct {
	routes  map[string]RouteHandler
	mu      sync.RWMutex
	logger  core.Logger
	metrics core.MetricsCollector
}

// RouteHandler 路由处理器
type RouteHandler struct {
	Handler     core.MCPHandler
	Middleware  []MiddlewareFunc
	Description string
	Version     ProtocolVersion
}

// NewMessageRouter 创建消息路由器
func NewMessageRouter(logger core.Logger, metrics core.MetricsCollector) *MessageRouter {
	return &MessageRouter{
		routes:  make(map[string]RouteHandler),
		logger:  logger,
		metrics: metrics,
	}
}

// RegisterRoute 注册路由
func (r *MessageRouter) RegisterRoute(method string, handler core.MCPHandler, middleware []MiddlewareFunc, description string, version ProtocolVersion) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.routes[method]; exists {
		return fmt.Errorf("route for method %s already exists", method)
	}

	r.routes[method] = RouteHandler{
		Handler:     handler,
		Middleware:  middleware,
		Description: description,
		Version:     version,
	}

	r.logger.Info("route registered", "method", method, "description", description, "version", version)
	return nil
}

// UnregisterRoute 注销路由
func (r *MessageRouter) UnregisterRoute(method string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.routes, method)
	r.logger.Info("route unregistered", "method", method)
}

// RouteMessage 路由消息
func (r *MessageRouter) RouteMessage(ctx context.Context, message *core.MCPMessage, connectionID string, version ProtocolVersion) (*core.MCPResponse, error) {
	if MessageType(message.Type) != MessageTypeRequest {
		return nil, NewProtocolError(ErrorCodeInvalidRequest, "Only request messages can be routed", message.Type)
	}

	r.mu.RLock()
	route, exists := r.routes[message.Method]
	r.mu.RUnlock()

	if !exists {
		r.logger.Warn("route not found", "method", message.Method)
		r.metrics.IncrementCounter("mcp_route_not_found", map[string]string{
			"method": message.Method,
		})
		return &core.MCPResponse{
			ID: message.ID,
			Error: &core.MCPError{
				Code:    ErrorCodeMethodNotFound,
				Message: fmt.Sprintf("Method not found: %s", message.Method),
			},
		}, nil
	}

	// 检查版本兼容性
	if !route.Version.IsCompatible(version) {
		r.logger.Warn("version incompatible", "method", message.Method, "route_version", route.Version, "client_version", version)
		return &core.MCPResponse{
			ID: message.ID,
			Error: &core.MCPError{
				Code:    ErrorCodeVersionError,
				Message: fmt.Sprintf("Method %s requires version %s, but client uses %s", message.Method, route.Version, version),
			},
		}, nil
	}

	// 构建请求
	request := &core.MCPRequest{
		ID:     message.ID,
		Method: message.Method,
		Params: message.Params,
	}

	// 应用中间件
	handler := route.Handler
	if len(route.Middleware) > 0 {
		chain := NewMiddlewareChain(route.Middleware...)
		handler = chain.Apply(handler)
	}

	// 记录路由指标
	r.metrics.IncrementCounter("mcp_route_requests", map[string]string{
		"method":        message.Method,
		"connection_id": connectionID,
	})

	// 执行处理器
	start := time.Now()
	response, err := handler.Handle(ctx, request)
	duration := time.Since(start)

	// 记录性能指标
	r.metrics.RecordHistogram("mcp_route_duration_seconds", duration.Seconds(), map[string]string{
		"method":        message.Method,
		"connection_id": connectionID,
	})

	if err != nil {
		r.logger.Error("route handler failed", "method", message.Method, "error", err)
		r.metrics.IncrementCounter("mcp_route_errors", map[string]string{
			"method":        message.Method,
			"connection_id": connectionID,
		})
		return &core.MCPResponse{
			ID: message.ID,
			Error: &core.MCPError{
				Code:    ErrorCodeInternalError,
				Message: "Handler execution failed",
				Data:    err.Error(),
			},
		}, nil
	}

	return response, nil
}

// GetRoutes 获取所有路由信息
func (r *MessageRouter) GetRoutes() map[string]RouteHandler {
	r.mu.RLock()
	defer r.mu.RUnlock()

	routes := make(map[string]RouteHandler)
	for method, route := range r.routes {
		routes[method] = route
	}
	return routes
}

// GetRouteMethods 获取所有路由方法
func (r *MessageRouter) GetRouteMethods() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	methods := make([]string, 0, len(r.routes))
	for method := range r.routes {
		methods = append(methods, method)
	}
	return methods
}

// MessageDispatcher 消息分发器
type MessageDispatcher struct {
	parser      MessageParser
	router      *MessageRouter
	negotiator  *VersionNegotiator
	connections map[string]ProtocolVersion // 连接ID到协议版本的映射
	mu          sync.RWMutex
	logger      core.Logger
	metrics     core.MetricsCollector
}

// NewMessageDispatcher 创建消息分发器
func NewMessageDispatcher(
	parser MessageParser,
	router *MessageRouter,
	negotiator *VersionNegotiator,
	logger core.Logger,
	metrics core.MetricsCollector,
) *MessageDispatcher {
	return &MessageDispatcher{
		parser:      parser,
		router:      router,
		negotiator:  negotiator,
		connections: make(map[string]ProtocolVersion),
		logger:      logger,
		metrics:     metrics,
	}
}

// DispatchMessage 分发消息
func (d *MessageDispatcher) DispatchMessage(ctx context.Context, data []byte, connectionID string) ([]byte, error) {
	// 解析消息
	message, err := d.parser.ParseMessage(data)
	if err != nil {
		d.logger.Error("failed to parse message", "error", err, "connection_id", connectionID)
		d.metrics.IncrementCounter("mcp_parse_errors", map[string]string{
			"connection_id": connectionID,
		})
		return d.createErrorResponse("", ErrorCodeParseError, "Parse error", err.Error())
	}

	// 记录消息指标
	d.metrics.IncrementCounter("mcp_messages_received", map[string]string{
		"type":          message.Type,
		"method":        message.Method,
		"connection_id": connectionID,
	})

	// 处理版本协商
	if message.Method == "protocol.negotiate" {
		return d.handleVersionNegotiation(message, connectionID)
	}

	// 获取连接的协议版本
	d.mu.RLock()
	version, exists := d.connections[connectionID]
	d.mu.RUnlock()

	if !exists {
		// 如果没有协商版本，使用默认版本
		version = CurrentProtocolVersion
		d.mu.Lock()
		d.connections[connectionID] = version
		d.mu.Unlock()
	}

	// 根据消息类型进行处理
	switch MessageType(message.Type) {
	case MessageTypeRequest:
		return d.handleRequest(ctx, message, connectionID, version)
	case MessageTypeNotification:
		return d.handleNotification(ctx, message, connectionID, version)
	default:
		d.logger.Warn("unsupported message type", "type", message.Type, "connection_id", connectionID)
		return d.createErrorResponse(message.ID, ErrorCodeInvalidRequest, "Unsupported message type", message.Type)
	}
}

// handleVersionNegotiation 处理版本协商
func (d *MessageDispatcher) handleVersionNegotiation(message *core.MCPMessage, connectionID string) ([]byte, error) {
	// 解析客户端版本
	params, ok := message.Params.(map[string]any)
	if !ok {
		return d.createErrorResponse(message.ID, ErrorCodeInvalidParams, "Invalid negotiation parameters", nil)
	}

	versionsData, ok := params["versions"].([]any)
	if !ok {
		return d.createErrorResponse(message.ID, ErrorCodeInvalidParams, "Missing versions parameter", nil)
	}

	var clientVersions []ProtocolVersion
	for _, versionData := range versionsData {
		versionStr, ok := versionData.(string)
		if !ok {
			continue
		}
		version, err := ParseVersion(versionStr)
		if err != nil {
			continue
		}
		clientVersions = append(clientVersions, version)
	}

	// 协商版本
	negotiatedVersion, err := d.negotiator.NegotiateVersion(clientVersions)
	if err != nil {
		return d.createErrorResponse(message.ID, ErrorCodeVersionError, "Version negotiation failed", err.Error())
	}

	// 保存连接的协议版本
	d.mu.Lock()
	d.connections[connectionID] = negotiatedVersion
	d.mu.Unlock()

	// 创建响应
	response := &core.MCPResponse{
		ID: message.ID,
		Result: map[string]any{
			"version":            negotiatedVersion.String(),
			"supported_versions": d.negotiator.GetSupportedVersions(),
		},
	}

	return d.parser.SerializeMessage(&core.MCPMessage{
		Type:   string(MessageTypeResponse),
		ID:     response.ID,
		Result: response.Result,
	})
}

// handleRequest 处理请求
func (d *MessageDispatcher) handleRequest(ctx context.Context, message *core.MCPMessage, connectionID string, version ProtocolVersion) ([]byte, error) {
	// 路由请求
	response, err := d.router.RouteMessage(ctx, message, connectionID, version)
	if err != nil {
		d.logger.Error("failed to route message", "error", err, "method", message.Method, "connection_id", connectionID)
		return d.createErrorResponse(message.ID, ErrorCodeInternalError, "Routing failed", err.Error())
	}

	// 序列化响应
	responseMessage := &core.MCPMessage{
		Type:   string(MessageTypeResponse),
		ID:     response.ID,
		Result: response.Result,
		Error:  response.Error,
	}

	return d.parser.SerializeMessage(responseMessage)
}

// handleNotification 处理通知
func (d *MessageDispatcher) handleNotification(ctx context.Context, message *core.MCPMessage, connectionID string, version ProtocolVersion) ([]byte, error) {
	// 通知不需要响应，但可以记录日志
	d.logger.Info("notification received", "method", message.Method, "connection_id", connectionID)

	// 处理特殊通知
	switch message.Method {
	case "connection.close":
		d.removeConnection(connectionID)
	}

	return nil, nil // 通知不需要响应
}

// createErrorResponse 创建错误响应
func (d *MessageDispatcher) createErrorResponse(id string, code int, message string, data any) ([]byte, error) {
	// 如果 ID 为空，使用默认 ID
	if id == "" {
		id = "error"
	}

	errorResponse := &core.MCPMessage{
		Type: string(MessageTypeResponse),
		ID:   id,
		Error: &core.MCPError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}

	return d.parser.SerializeMessage(errorResponse)
}

// removeConnection 移除连接
func (d *MessageDispatcher) removeConnection(connectionID string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.connections, connectionID)
	d.logger.Info("connection removed from dispatcher", "connection_id", connectionID)
}

// GetConnectionVersion 获取连接的协议版本
func (d *MessageDispatcher) GetConnectionVersion(connectionID string) (ProtocolVersion, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	version, exists := d.connections[connectionID]
	return version, exists
}

// GetConnectionCount 获取连接数量
func (d *MessageDispatcher) GetConnectionCount() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.connections)
}

// ProtocolHandler 协议处理器，整合所有协议相关功能
type ProtocolHandler struct {
	dispatcher *MessageDispatcher
	parser     MessageParser
	router     *MessageRouter
	negotiator *VersionNegotiator
	logger     core.Logger
}

// NewProtocolHandler 创建协议处理器
func NewProtocolHandler(logger core.Logger, metrics core.MetricsCollector) *ProtocolHandler {
	parser := NewJSONMessageParser(logger)
	router := NewMessageRouter(logger, metrics)
	negotiator := NewVersionNegotiator(SupportedVersions, logger)
	dispatcher := NewMessageDispatcher(parser, router, negotiator, logger, metrics)

	return &ProtocolHandler{
		dispatcher: dispatcher,
		parser:     parser,
		router:     router,
		negotiator: negotiator,
		logger:     logger,
	}
}

// ProcessMessage 处理消息
func (h *ProtocolHandler) ProcessMessage(ctx context.Context, data []byte, connectionID string) ([]byte, error) {
	return h.dispatcher.DispatchMessage(ctx, data, connectionID)
}

// RegisterHandler 注册处理器
func (h *ProtocolHandler) RegisterHandler(method string, handler core.MCPHandler, middleware []MiddlewareFunc, description string) error {
	return h.router.RegisterRoute(method, handler, middleware, description, CurrentProtocolVersion)
}

// UnregisterHandler 注销处理器
func (h *ProtocolHandler) UnregisterHandler(method string) {
	h.router.UnregisterRoute(method)
}

// GetSupportedMethods 获取支持的方法列表
func (h *ProtocolHandler) GetSupportedMethods() []string {
	methods := h.router.GetRouteMethods()
	// 添加内置方法
	methods = append(methods, "protocol.negotiate")
	return methods
}

// GetSupportedVersions 获取支持的版本列表
func (h *ProtocolHandler) GetSupportedVersions() []ProtocolVersion {
	return h.negotiator.GetSupportedVersions()
}

// RemoveConnection 移除连接
func (h *ProtocolHandler) RemoveConnection(connectionID string) {
	h.dispatcher.removeConnection(connectionID)
}
