package mcp

import (
	"context"
	"fmt"
	"sync"
	"time"

	"pumppill/rag/core"
)

// HandlerRegistry 处理器注册表
type HandlerRegistry struct {
	handlers map[string]core.MCPHandler
	mu       sync.RWMutex
	logger   core.Logger
}

// NewHandlerRegistry 创建新的处理器注册表
func NewHandlerRegistry(logger core.Logger) *HandlerRegistry {
	return &HandlerRegistry{
		handlers: make(map[string]core.MCPHandler),
		logger:   logger,
	}
}

// Register 注册处理器
func (r *HandlerRegistry) Register(method string, handler core.MCPHandler) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.handlers[method]; exists {
		return fmt.Errorf("handler for method %s already exists", method)
	}

	r.handlers[method] = handler
	r.logger.Info("handler registered", "method", method)
	return nil
}

// Unregister 注销处理器
func (r *HandlerRegistry) Unregister(method string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.handlers, method)
	r.logger.Info("handler unregistered", "method", method)
}

// Get 获取处理器
func (r *HandlerRegistry) Get(method string) (core.MCPHandler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	handler, exists := r.handlers[method]
	return handler, exists
}

// GetAll 获取所有已注册的方法
func (r *HandlerRegistry) GetAll() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	methods := make([]string, 0, len(r.handlers))
	for method := range r.handlers {
		methods = append(methods, method)
	}
	return methods
}

// RequestValidator 请求验证器接口
type RequestValidator interface {
	Validate(ctx context.Context, request *core.MCPRequest) error
}

// DefaultRequestValidator 默认请求验证器
type DefaultRequestValidator struct {
	logger core.Logger
}

// NewDefaultRequestValidator 创建默认请求验证器
func NewDefaultRequestValidator(logger core.Logger) *DefaultRequestValidator {
	return &DefaultRequestValidator{
		logger: logger,
	}
}

// Validate 验证请求
func (v *DefaultRequestValidator) Validate(ctx context.Context, request *core.MCPRequest) error {
	if request == nil {
		return fmt.Errorf("request cannot be nil")
	}

	if request.ID == "" {
		return fmt.Errorf("request ID cannot be empty")
	}

	if request.Method == "" {
		return fmt.Errorf("request method cannot be empty")
	}

	// 验证方法名格式
	if len(request.Method) > 100 {
		return fmt.Errorf("method name too long: %d characters", len(request.Method))
	}

	return nil
}

// RateLimiter 限流器接口
type RateLimiter interface {
	Allow(ctx context.Context, key string) (bool, error)
	Reset(ctx context.Context, key string) error
}

// TokenBucketRateLimiter 令牌桶限流器
type TokenBucketRateLimiter struct {
	buckets    map[string]*TokenBucket
	mu         sync.RWMutex
	rate       int           // 每秒生成的令牌数
	capacity   int           // 桶容量
	windowSize time.Duration // 时间窗口大小
	logger     core.Logger
}

// TokenBucket 令牌桶
type TokenBucket struct {
	tokens     int
	lastRefill time.Time
	mu         sync.Mutex
}

// NewTokenBucketRateLimiter 创建令牌桶限流器
func NewTokenBucketRateLimiter(rate, capacity int, windowSize time.Duration, logger core.Logger) *TokenBucketRateLimiter {
	return &TokenBucketRateLimiter{
		buckets:    make(map[string]*TokenBucket),
		rate:       rate,
		capacity:   capacity,
		windowSize: windowSize,
		logger:     logger,
	}
}

// Allow 检查是否允许请求
func (r *TokenBucketRateLimiter) Allow(ctx context.Context, key string) (bool, error) {
	r.mu.Lock()
	bucket, exists := r.buckets[key]
	if !exists {
		bucket = &TokenBucket{
			tokens:     r.capacity,
			lastRefill: time.Now(),
		}
		r.buckets[key] = bucket
	}
	r.mu.Unlock()

	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	// 补充令牌
	now := time.Now()
	elapsed := now.Sub(bucket.lastRefill)
	tokensToAdd := int(elapsed.Seconds()) * r.rate
	if tokensToAdd > 0 {
		bucket.tokens = min(r.capacity, bucket.tokens+tokensToAdd)
		bucket.lastRefill = now
	}

	// 检查是否有可用令牌
	if bucket.tokens > 0 {
		bucket.tokens--
		return true, nil
	}

	return false, nil
}

// Reset 重置限流器
func (r *TokenBucketRateLimiter) Reset(ctx context.Context, key string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.buckets, key)
	return nil
}

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// RequestProcessor 请求处理器
type RequestProcessor struct {
	registry    *HandlerRegistry
	validator   RequestValidator
	rateLimiter RateLimiter
	security    core.SecurityManager
	metrics     core.MetricsCollector
	logger      core.Logger
	timeout     time.Duration
}

// NewRequestProcessor 创建请求处理器
func NewRequestProcessor(
	registry *HandlerRegistry,
	validator RequestValidator,
	rateLimiter RateLimiter,
	security core.SecurityManager,
	metrics core.MetricsCollector,
	logger core.Logger,
	timeout time.Duration,
) *RequestProcessor {
	return &RequestProcessor{
		registry:    registry,
		validator:   validator,
		rateLimiter: rateLimiter,
		security:    security,
		metrics:     metrics,
		logger:      logger,
		timeout:     timeout,
	}
}

// ProcessRequest 处理请求
func (p *RequestProcessor) ProcessRequest(ctx context.Context, request *core.MCPRequest, connectionID string) *core.MCPResponse {
	startTime := time.Now()

	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	response := &core.MCPResponse{
		ID: request.ID,
	}

	// 记录请求指标
	p.metrics.IncrementCounter("mcp_requests_total", map[string]string{
		"method":        request.Method,
		"connection_id": connectionID,
	})

	// 1. 验证请求
	if err := p.validator.Validate(ctx, request); err != nil {
		p.logger.Error("request validation failed", "error", err, "request_id", request.ID)
		response.Error = &core.MCPError{
			Code:    400,
			Message: "Invalid request",
			Data:    err.Error(),
		}
		p.recordError("validation_error", request.Method, connectionID)
		return response
	}

	// 2. 限流检查
	if p.rateLimiter != nil {
		allowed, err := p.rateLimiter.Allow(ctx, connectionID)
		if err != nil {
			p.logger.Error("rate limiter error", "error", err, "connection_id", connectionID)
			response.Error = &core.MCPError{
				Code:    500,
				Message: "Internal server error",
				Data:    "Rate limiter error",
			}
			p.recordError("rate_limiter_error", request.Method, connectionID)
			return response
		}
		if !allowed {
			p.logger.Warn("request rate limited", "connection_id", connectionID, "method", request.Method)
			response.Error = &core.MCPError{
				Code:    429,
				Message: "Too many requests",
				Data:    "Rate limit exceeded",
			}
			p.recordError("rate_limited", request.Method, connectionID)
			return response
		}
	}

	// 3. 权限检查
	if p.security != nil {
		// 从上下文中获取用户信息（如果有的话）
		if userInfo, ok := ctx.Value("user").(*core.UserInfo); ok {
			if err := p.security.CheckPermission(ctx, userInfo, "mcp", request.Method); err != nil {
				p.logger.Warn("permission denied", "user_id", userInfo.ID, "method", request.Method, "error", err)
				response.Error = &core.MCPError{
					Code:    403,
					Message: "Permission denied",
					Data:    err.Error(),
				}
				p.recordError("permission_denied", request.Method, connectionID)
				return response
			}
		}
	}

	// 4. 获取处理器
	handler, exists := p.registry.Get(request.Method)
	if !exists {
		p.logger.Warn("handler not found", "method", request.Method)
		response.Error = &core.MCPError{
			Code:    404,
			Message: "Method not found",
			Data:    fmt.Sprintf("No handler registered for method: %s", request.Method),
		}
		p.recordError("handler_not_found", request.Method, connectionID)
		return response
	}

	// 5. 执行处理器
	result, err := handler.Handle(ctx, request)
	if err != nil {
		p.logger.Error("handler execution failed", "method", request.Method, "error", err)
		response.Error = &core.MCPError{
			Code:    500,
			Message: "Handler execution failed",
			Data:    err.Error(),
		}
		p.recordError("handler_error", request.Method, connectionID)
		return response
	}

	response.Result = result.Result
	if result.Error != nil {
		response.Error = result.Error
		p.recordError("handler_business_error", request.Method, connectionID)
	}

	// 记录成功指标
	duration := time.Since(startTime)
	p.metrics.RecordHistogram("mcp_request_duration_seconds", duration.Seconds(), map[string]string{
		"method":        request.Method,
		"connection_id": connectionID,
		"status":        "success",
	})

	p.logger.Info("request processed successfully",
		"method", request.Method,
		"request_id", request.ID,
		"duration", duration,
		"connection_id", connectionID)

	return response
}

// recordError 记录错误指标
func (p *RequestProcessor) recordError(errorType, method, connectionID string) {
	p.metrics.IncrementCounter("mcp_request_errors_total", map[string]string{
		"error_type":    errorType,
		"method":        method,
		"connection_id": connectionID,
	})
}

// AsyncRequestProcessor 异步请求处理器
type AsyncRequestProcessor struct {
	processor   *RequestProcessor
	workerCount int
	requestChan chan *AsyncRequest
	logger      core.Logger
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
}

// AsyncRequest 异步请求
type AsyncRequest struct {
	Request      *core.MCPRequest
	ConnectionID string
	ResponseChan chan *core.MCPResponse
}

// NewAsyncRequestProcessor 创建异步请求处理器
func NewAsyncRequestProcessor(processor *RequestProcessor, workerCount int, logger core.Logger) *AsyncRequestProcessor {
	ctx, cancel := context.WithCancel(context.Background())

	return &AsyncRequestProcessor{
		processor:   processor,
		workerCount: workerCount,
		requestChan: make(chan *AsyncRequest, workerCount*2), // 缓冲区大小为工作者数量的两倍
		logger:      logger,
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Start 启动异步处理器
func (a *AsyncRequestProcessor) Start() {
	for i := 0; i < a.workerCount; i++ {
		a.wg.Add(1)
		go a.worker(i)
	}
	a.logger.Info("async request processor started", "worker_count", a.workerCount)
}

// Stop 停止异步处理器
func (a *AsyncRequestProcessor) Stop() {
	a.cancel()
	close(a.requestChan)
	a.wg.Wait()
	a.logger.Info("async request processor stopped")
}

// ProcessAsync 异步处理请求
func (a *AsyncRequestProcessor) ProcessAsync(request *core.MCPRequest, connectionID string) <-chan *core.MCPResponse {
	responseChan := make(chan *core.MCPResponse, 1)

	asyncReq := &AsyncRequest{
		Request:      request,
		ConnectionID: connectionID,
		ResponseChan: responseChan,
	}

	select {
	case a.requestChan <- asyncReq:
		// 请求已提交
	case <-a.ctx.Done():
		// 处理器已停止
		response := &core.MCPResponse{
			ID: request.ID,
			Error: &core.MCPError{
				Code:    503,
				Message: "Service unavailable",
				Data:    "Request processor is shutting down",
			},
		}
		responseChan <- response
		close(responseChan)
	default:
		// 队列已满
		response := &core.MCPResponse{
			ID: request.ID,
			Error: &core.MCPError{
				Code:    503,
				Message: "Service unavailable",
				Data:    "Request queue is full",
			},
		}
		responseChan <- response
		close(responseChan)
	}

	return responseChan
}

// worker 工作者 goroutine
func (a *AsyncRequestProcessor) worker(id int) {
	defer a.wg.Done()

	a.logger.Info("async worker started", "worker_id", id)

	for {
		select {
		case <-a.ctx.Done():
			a.logger.Info("async worker stopped", "worker_id", id)
			return
		case asyncReq, ok := <-a.requestChan:
			if !ok {
				a.logger.Info("async worker stopped - channel closed", "worker_id", id)
				return
			}

			// 处理请求
			response := a.processor.ProcessRequest(a.ctx, asyncReq.Request, asyncReq.ConnectionID)

			// 发送响应
			select {
			case asyncReq.ResponseChan <- response:
				close(asyncReq.ResponseChan)
			case <-a.ctx.Done():
				// 处理器正在关闭
				close(asyncReq.ResponseChan)
				return
			}
		}
	}
}

// MiddlewareFunc 中间件函数类型
type MiddlewareFunc func(next core.MCPHandler) core.MCPHandler

// MiddlewareChain 中间件链
type MiddlewareChain struct {
	middlewares []MiddlewareFunc
}

// NewMiddlewareChain 创建中间件链
func NewMiddlewareChain(middlewares ...MiddlewareFunc) *MiddlewareChain {
	return &MiddlewareChain{
		middlewares: middlewares,
	}
}

// Apply 应用中间件链到处理器
func (c *MiddlewareChain) Apply(handler core.MCPHandler) core.MCPHandler {
	// 从后往前应用中间件
	for i := len(c.middlewares) - 1; i >= 0; i-- {
		handler = c.middlewares[i](handler)
	}
	return handler
}

// LoggingMiddleware 日志中间件
func LoggingMiddleware(logger core.Logger) MiddlewareFunc {
	return func(next core.MCPHandler) core.MCPHandler {
		return &loggingHandler{
			next:   next,
			logger: logger,
		}
	}
}

type loggingHandler struct {
	next   core.MCPHandler
	logger core.Logger
}

func (h *loggingHandler) Handle(ctx context.Context, request *core.MCPRequest) (*core.MCPResponse, error) {
	start := time.Now()
	h.logger.Info("handling request", "method", request.Method, "request_id", request.ID)

	response, err := h.next.Handle(ctx, request)

	duration := time.Since(start)
	if err != nil {
		h.logger.Error("request failed", "method", request.Method, "request_id", request.ID, "duration", duration, "error", err)
	} else {
		h.logger.Info("request completed", "method", request.Method, "request_id", request.ID, "duration", duration)
	}

	return response, err
}

// MetricsMiddleware 指标中间件
func MetricsMiddleware(metrics core.MetricsCollector) MiddlewareFunc {
	return func(next core.MCPHandler) core.MCPHandler {
		return &metricsHandler{
			next:    next,
			metrics: metrics,
		}
	}
}

type metricsHandler struct {
	next    core.MCPHandler
	metrics core.MetricsCollector
}

func (h *metricsHandler) Handle(ctx context.Context, request *core.MCPRequest) (*core.MCPResponse, error) {
	start := time.Now()

	h.metrics.IncrementCounter("mcp_handler_requests_total", map[string]string{
		"method": request.Method,
	})

	response, err := h.next.Handle(ctx, request)

	duration := time.Since(start)
	status := "success"
	if err != nil {
		status = "error"
	}

	h.metrics.RecordHistogram("mcp_handler_duration_seconds", duration.Seconds(), map[string]string{
		"method": request.Method,
		"status": status,
	})

	return response, err
}

// RecoveryMiddleware 恢复中间件，捕获 panic
func RecoveryMiddleware(logger core.Logger) MiddlewareFunc {
	return func(next core.MCPHandler) core.MCPHandler {
		return &recoveryHandler{
			next:   next,
			logger: logger,
		}
	}
}

type recoveryHandler struct {
	next   core.MCPHandler
	logger core.Logger
}

func (h *recoveryHandler) Handle(ctx context.Context, request *core.MCPRequest) (response *core.MCPResponse, err error) {
	defer func() {
		if r := recover(); r != nil {
			h.logger.Error("handler panic recovered", "method", request.Method, "request_id", request.ID, "panic", r)
			response = &core.MCPResponse{
				ID: request.ID,
				Error: &core.MCPError{
					Code:    500,
					Message: "Internal server error",
					Data:    "Handler panic",
				},
			}
			err = fmt.Errorf("handler panic: %v", r)
		}
	}()

	return h.next.Handle(ctx, request)
}
