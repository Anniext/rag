# Go 客户端集成

本文档展示如何在 Go 应用中集成 RAG MySQL 查询系统，包括同步和异步客户端的实现。

## 依赖安装

```bash
go mod init rag-client-example
go get github.com/gorilla/websocket
go get github.com/golang-jwt/jwt/v5
```

## 基础客户端实现

### 1. 客户端结构定义

```go
package ragclient

import (
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "log"
    "net/http"
    "sync"
    "time"

    "github.com/gorilla/websocket"
)

// RAGClient RAG MySQL 查询系统客户端
type RAGClient struct {
    url             string
    conn            *websocket.Conn
    sessionID       string
    messageID       int64
    pendingRequests map[string]*PendingRequest
    mutex           sync.RWMutex
    isConnected     bool
    isAuthenticated bool

    // 配置选项
    options ClientOptions

    // 事件处理器
    eventHandlers map[string][]EventHandler

    // 日志
    logger *log.Logger
}

// ClientOptions 客户端配置选项
type ClientOptions struct {
    ReconnectInterval    time.Duration
    MaxReconnectAttempts int
    Timeout             time.Duration
    PingInterval        time.Duration
}

// PendingRequest 待处理的请求
type PendingRequest struct {
    ID        string
    Response  chan *MCPMessage
    Timestamp time.Time
}

// EventHandler 事件处理器
type EventHandler func(data interface{})
```

### 2. 消息结构定义

```go
// MCPMessage MCP 协议消息
type MCPMessage struct {
    Type   string      `json:"type"`
    ID     string      `json:"id,omitempty"`
    Method string      `json:"method,omitempty"`
    Params interface{} `json:"params,omitempty"`
    Result interface{} `json:"result,omitempty"`
    Error  *MCPError   `json:"error,omitempty"`
}

// MCPError MCP 错误
type MCPError struct {
    Code    int         `json:"code"`
    Message string      `json:"message"`
    Data    interface{} `json:"data,omitempty"`
}

// QueryRequest 查询请求
type QueryRequest struct {
    Query     string                 `json:"query"`
    SessionID string                 `json:"session_id,omitempty"`
    Options   map[string]interface{} `json:"options,omitempty"`
}

// QueryResponse 查询响应
type QueryResponse struct {
    Success     bool                     `json:"success"`
    Data        []map[string]interface{} `json:"data,omitempty"`
    SQL         string                   `json:"sql,omitempty"`
    Explanation string                   `json:"explanation,omitempty"`
    Suggestions []string                 `json:"suggestions,omitempty"`
    Error       string                   `json:"error,omitempty"`
    Metadata    *QueryMetadata           `json:"metadata,omitempty"`
}

// QueryMetadata 查询元数据
type QueryMetadata struct {
    ExecutionTime time.Duration `json:"execution_time"`
    RowCount      int           `json:"row_count"`
    AffectedRows  int           `json:"affected_rows,omitempty"`
    QueryPlan     string        `json:"query_plan,omitempty"`
    Warnings      []string      `json:"warnings,omitempty"`
}
```

### 3. 客户端构造函数

```go
// NewRAGClient 创建新的 RAG 客户端
func NewRAGClient(url string, options ...ClientOption) *RAGClient {
    client := &RAGClient{
        url:             url,
        messageID:       1,
        pendingRequests: make(map[string]*PendingRequest),
        eventHandlers:   make(map[string][]EventHandler),
        logger:          log.New(os.Stdout, "[RAGClient] ", log.LstdFlags),
        options: ClientOptions{
            ReconnectInterval:    5 * time.Second,
            MaxReconnectAttempts: 5,
            Timeout:             30 * time.Second,
            PingInterval:        30 * time.Second,
        },
    }

    // 应用配置选项
    for _, option := range options {
        option(client)
    }

    return client
}

// ClientOption 客户端配置选项函数
type ClientOption func(*RAGClient)

// WithTimeout 设置超时时间
func WithTimeout(timeout time.Duration) ClientOption {
    return func(c *RAGClient) {
        c.options.Timeout = timeout
    }
}

// WithReconnectInterval 设置重连间隔
func WithReconnectInterval(interval time.Duration) ClientOption {
    return func(c *RAGClient) {
        c.options.ReconnectInterval = interval
    }
}

// WithMaxReconnectAttempts 设置最大重连次数
func WithMaxReconnectAttempts(attempts int) ClientOption {
    return func(c *RAGClient) {
        c.options.MaxReconnectAttempts = attempts
    }
}
```

### 4. 连接管理

```go
// Connect 连接到 RAG 服务器
func (c *RAGClient) Connect(ctx context.Context) error {
    dialer := websocket.Dialer{
        HandshakeTimeout: c.options.Timeout,
    }

    conn, _, err := dialer.DialContext(ctx, c.url, nil)
    if err != nil {
        return fmt.Errorf("连接失败: %w", err)
    }

    c.conn = conn
    c.isConnected = true
    c.logger.Println("WebSocket 连接已建立")

    // 启动消息监听
    go c.messageListener(ctx)

    // 触发连接事件
    c.emitEvent("connected", nil)

    // 协议协商
    if err := c.negotiateProtocol(ctx); err != nil {
        c.conn.Close()
        c.isConnected = false
        return fmt.Errorf("协议协商失败: %w", err)
    }

    return nil
}

// negotiateProtocol 协议协商
func (c *RAGClient) negotiateProtocol(ctx context.Context) error {
    response, err := c.sendRequest(ctx, "protocol.negotiate", map[string]interface{}{
        "versions": []string{"1.0.0"},
    })

    if err != nil {
        return err
    }

    if response.Error != nil {
        return fmt.Errorf("协议协商失败: %s", response.Error.Message)
    }

    c.logger.Printf("协议协商成功: %v", response.Result)
    return nil
}

// Authenticate 用户认证
func (c *RAGClient) Authenticate(ctx context.Context, token string) error {
    response, err := c.sendRequest(ctx, "auth.authenticate", map[string]interface{}{
        "token": token,
    })

    if err != nil {
        return err
    }

    if response.Error != nil {
        return fmt.Errorf("认证失败: %s", response.Error.Message)
    }

    result, ok := response.Result.(map[string]interface{})
    if !ok || !result["success"].(bool) {
        return errors.New("认证失败")
    }

    c.isAuthenticated = true
    c.logger.Println("用户认证成功")
    c.emitEvent("authenticated", result["user"])

    return nil
}

// CreateSession 创建会话
func (c *RAGClient) CreateSession(ctx context.Context, userID string) error {
    response, err := c.sendRequest(ctx, "session.create", map[string]interface{}{
        "user_id": userID,
    })

    if err != nil {
        return err
    }

    if response.Error != nil {
        return fmt.Errorf("会话创建失败: %s", response.Error.Message)
    }

    result, ok := response.Result.(map[string]interface{})
    if !ok {
        return errors.New("会话创建失败: 无效响应")
    }

    c.sessionID = result["session_id"].(string)
    c.logger.Printf("会话创建成功: %s", c.sessionID)

    return nil
}
```

### 5. 查询功能

```go
// Query 执行查询
func (c *RAGClient) Query(ctx context.Context, queryText string, options map[string]interface{}) (*QueryResponse, error) {
    if c.sessionID == "" {
        return nil, errors.New("会话未建立，请先创建会话")
    }

    if options == nil {
        options = make(map[string]interface{})
    }

    // 设置默认选项
    if _, exists := options["limit"]; !exists {
        options["limit"] = 20
    }
    if _, exists := options["explain"]; !exists {
        options["explain"] = false
    }
    if _, exists := options["optimize"]; !exists {
        options["optimize"] = true
    }

    params := map[string]interface{}{
        "query":      queryText,
        "session_id": c.sessionID,
        "options":    options,
    }

    response, err := c.sendRequest(ctx, "query.execute", params)
    if err != nil {
        return nil, err
    }

    if response.Error != nil {
        return nil, fmt.Errorf("查询失败: %s", response.Error.Message)
    }

    // 解析响应
    resultBytes, err := json.Marshal(response.Result)
    if err != nil {
        return nil, fmt.Errorf("响应解析失败: %w", err)
    }

    var queryResponse QueryResponse
    if err := json.Unmarshal(resultBytes, &queryResponse); err != nil {
        return nil, fmt.Errorf("响应解析失败: %w", err)
    }

    return &queryResponse, nil
}

// ExplainQuery 查询解释
func (c *RAGClient) ExplainQuery(ctx context.Context, queryText string) (map[string]interface{}, error) {
    response, err := c.sendRequest(ctx, "query.explain", map[string]interface{}{
        "query": queryText,
    })

    if err != nil {
        return nil, err
    }

    if response.Error != nil {
        return nil, fmt.Errorf("查询解释失败: %s", response.Error.Message)
    }

    result, ok := response.Result.(map[string]interface{})
    if !ok {
        return nil, errors.New("查询解释失败: 无效响应")
    }

    return result, nil
}

// GetSuggestions 获取查询建议
func (c *RAGClient) GetSuggestions(ctx context.Context, partial string, limit int) ([]string, error) {
    response, err := c.sendRequest(ctx, "query.suggest", map[string]interface{}{
        "partial": partial,
        "limit":   limit,
    })

    if err != nil {
        return nil, err
    }

    if response.Error != nil {
        return nil, fmt.Errorf("获取建议失败: %s", response.Error.Message)
    }

    result, ok := response.Result.(map[string]interface{})
    if !ok {
        return nil, errors.New("获取建议失败: 无效响应")
    }

    suggestions, ok := result["suggestions"].([]interface{})
    if !ok {
        return nil, errors.New("获取建议失败: 无效建议格式")
    }

    var stringSlice []string
    for _, s := range suggestions {
        if str, ok := s.(string); ok {
            stringSlice = append(stringSlice, str)
        }
    }

    return stringSlice, nil
}

// GetSchemaInfo 获取 Schema 信息
func (c *RAGClient) GetSchemaInfo(ctx context.Context, tableName string, includeRelationships bool) (map[string]interface{}, error) {
    params := map[string]interface{}{
        "include_relationships": includeRelationships,
    }

    if tableName != "" {
        params["table"] = tableName
    }

    response, err := c.sendRequest(ctx, "schema.info", params)
    if err != nil {
        return nil, err
    }

    if response.Error != nil {
        return nil, fmt.Errorf("获取 Schema 失败: %s", response.Error.Message)
    }

    result, ok := response.Result.(map[string]interface{})
    if !ok {
        return nil, errors.New("获取 Schema 失败: 无效响应")
    }

    return result, nil
}
```

### 6. 消息处理

```go
// sendRequest 发送请求
func (c *RAGClient) sendRequest(ctx context.Context, method string, params interface{}) (*MCPMessage, error) {
    if !c.isConnected {
        return nil, errors.New("WebSocket 连接未建立")
    }

    c.mutex.Lock()
    requestID := fmt.Sprintf("req-%d", c.messageID)
    c.messageID++
    c.mutex.Unlock()

    message := &MCPMessage{
        Type:   "request",
        ID:     requestID,
        Method: method,
        Params: params,
    }

    // 创建响应通道
    responseChan := make(chan *MCPMessage, 1)
    pendingRequest := &PendingRequest{
        ID:        requestID,
        Response:  responseChan,
        Timestamp: time.Now(),
    }

    c.mutex.Lock()
    c.pendingRequests[requestID] = pendingRequest
    c.mutex.Unlock()

    // 发送消息
    if err := c.conn.WriteJSON(message); err != nil {
        c.mutex.Lock()
        delete(c.pendingRequests, requestID)
        c.mutex.Unlock()
        return nil, fmt.Errorf("发送消息失败: %w", err)
    }

    // 等待响应
    select {
    case response := <-responseChan:
        return response, nil
    case <-time.After(c.options.Timeout):
        c.mutex.Lock()
        delete(c.pendingRequests, requestID)
        c.mutex.Unlock()
        return nil, errors.New("请求超时")
    case <-ctx.Done():
        c.mutex.Lock()
        delete(c.pendingRequests, requestID)
        c.mutex.Unlock()
        return nil, ctx.Err()
    }
}

// messageListener 消息监听器
func (c *RAGClient) messageListener(ctx context.Context) {
    defer func() {
        c.isConnected = false
        c.emitEvent("disconnected", nil)
    }()

    for {
        select {
        case <-ctx.Done():
            return
        default:
            var message MCPMessage
            if err := c.conn.ReadJSON(&message); err != nil {
                c.logger.Printf("读取消息失败: %v", err)
                return
            }

            c.handleMessage(&message)
        }
    }
}

// handleMessage 处理接收到的消息
func (c *RAGClient) handleMessage(message *MCPMessage) {
    c.emitEvent("message", message)

    if message.Type == "response" && message.ID != "" {
        // 处理响应消息
        c.mutex.RLock()
        pendingRequest, exists := c.pendingRequests[message.ID]
        c.mutex.RUnlock()

        if exists {
            select {
            case pendingRequest.Response <- message:
            default:
            }

            c.mutex.Lock()
            delete(c.pendingRequests, message.ID)
            c.mutex.Unlock()
        }
    } else if message.Type == "notification" {
        // 处理通知消息
        c.handleNotification(message)
    }
}

// handleNotification 处理通知消息
func (c *RAGClient) handleNotification(message *MCPMessage) {
    switch message.Method {
    case "query.progress":
        c.emitEvent("query_progress", message.Params)
    case "session.expired":
        c.sessionID = ""
        c.emitEvent("session_expired", message.Params)
    case "system.status":
        c.emitEvent("system_status", message.Params)
    }
}
```

### 7. 事件处理

```go
// On 注册事件处理器
func (c *RAGClient) On(event string, handler EventHandler) {
    c.mutex.Lock()
    defer c.mutex.Unlock()

    c.eventHandlers[event] = append(c.eventHandlers[event], handler)
}

// Off 移除事件处理器
func (c *RAGClient) Off(event string, handler EventHandler) {
    c.mutex.Lock()
    defer c.mutex.Unlock()

    handlers := c.eventHandlers[event]
    for i, h := range handlers {
        if &h == &handler {
            c.eventHandlers[event] = append(handlers[:i], handlers[i+1:]...)
            break
        }
    }
}

// emitEvent 触发事件
func (c *RAGClient) emitEvent(event string, data interface{}) {
    c.mutex.RLock()
    handlers := c.eventHandlers[event]
    c.mutex.RUnlock()

    for _, handler := range handlers {
        go func(h EventHandler) {
            defer func() {
                if r := recover(); r != nil {
                    c.logger.Printf("事件处理器错误: %v", r)
                }
            }()
            h(data)
        }(handler)
    }
}

// Disconnect 断开连接
func (c *RAGClient) Disconnect() error {
    if c.conn != nil {
        err := c.conn.Close()
        c.conn = nil
        c.isConnected = false
        c.isAuthenticated = false
        c.sessionID = ""

        // 清理待处理的请求
        c.mutex.Lock()
        for _, req := range c.pendingRequests {
            close(req.Response)
        }
        c.pendingRequests = make(map[string]*PendingRequest)
        c.mutex.Unlock()

        return err
    }
    return nil
}

// IsConnected 检查连接状态
func (c *RAGClient) IsConnected() bool {
    return c.isConnected
}

// IsAuthenticated 检查认证状态
func (c *RAGClient) IsAuthenticated() bool {
    return c.isAuthenticated
}

// GetSessionID 获取会话ID
func (c *RAGClient) GetSessionID() string {
    return c.sessionID
}
```

## 使用示例

### 1. 基本使用

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"
)

func main() {
    // 创建客户端
    client := NewRAGClient("ws://localhost:8080/mcp",
        WithTimeout(30*time.Second),
        WithReconnectInterval(3*time.Second),
        WithMaxReconnectAttempts(3),
    )

    // 设置事件处理器
    client.On("connected", func(data interface{}) {
        fmt.Println("客户端已连接")
    })

    client.On("authenticated", func(data interface{}) {
        fmt.Printf("用户已认证: %v\n", data)
    })

    client.On("error", func(data interface{}) {
        fmt.Printf("客户端错误: %v\n", data)
    })

    ctx := context.Background()

    // 连接到服务器
    if err := client.Connect(ctx); err != nil {
        log.Fatalf("连接失败: %v", err)
    }
    defer client.Disconnect()

    // 用户认证
    if err := client.Authenticate(ctx, "your-jwt-token"); err != nil {
        log.Fatalf("认证失败: %v", err)
    }

    // 创建会话
    if err := client.CreateSession(ctx, "go-user"); err != nil {
        log.Fatalf("会话创建失败: %v", err)
    }

    // 执行查询
    result, err := client.Query(ctx, "查询所有用户信息", map[string]interface{}{
        "limit":   10,
        "explain": true,
    })
    if err != nil {
        log.Fatalf("查询失败: %v", err)
    }

    fmt.Printf("查询结果:\n")
    fmt.Printf("SQL: %s\n", result.SQL)
    fmt.Printf("数据条数: %d\n", len(result.Data))
    fmt.Printf("解释: %s\n", result.Explanation)

    // 获取查询建议
    suggestions, err := client.GetSuggestions(ctx, "查询用户", 5)
    if err != nil {
        log.Printf("获取建议失败: %v", err)
    } else {
        fmt.Printf("查询建议: %v\n", suggestions)
    }

    // 获取 Schema 信息
    schema, err := client.GetSchemaInfo(ctx, "users", true)
    if err != nil {
        log.Printf("获取 Schema 失败: %v", err)
    } else {
        fmt.Printf("Schema 信息: %v\n", schema)
    }
}
```

### 2. Gin 框架集成

```go
package main

import (
    "context"
    "net/http"
    "strconv"
    "sync"
    "time"

    "github.com/gin-gonic/gin"
)

var (
    ragClient *RAGClient
    clientMux sync.RWMutex
)

// 获取 RAG 客户端实例（线程安全）
func getRAGClient() (*RAGClient, error) {
    clientMux.RLock()
    if ragClient != nil && ragClient.IsConnected() {
        defer clientMux.RUnlock()
        return ragClient, nil
    }
    clientMux.RUnlock()

    clientMux.Lock()
    defer clientMux.Unlock()

    // 双重检查
    if ragClient != nil && ragClient.IsConnected() {
        return ragClient, nil
    }

    // 创建新客户端
    client := NewRAGClient("ws://localhost:8080/mcp")

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    if err := client.Connect(ctx); err != nil {
        return nil, err
    }

    if err := client.Authenticate(ctx, "server-jwt-token"); err != nil {
        client.Disconnect()
        return nil, err
    }

    if err := client.CreateSession(ctx, "gin-server"); err != nil {
        client.Disconnect()
        return nil, err
    }

    ragClient = client
    return ragClient, nil
}

// 查询接口
func queryHandler(c *gin.Context) {
    var request struct {
        Query   string                 `json:"query" binding:"required"`
        Options map[string]interface{} `json:"options"`
    }

    if err := c.ShouldBindJSON(&request); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    client, err := getRAGClient()
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "无法连接到 RAG 服务器"})
        return
    }

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    result, err := client.Query(ctx, request.Query, request.Options)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, result)
}

// Schema 信息接口
func schemaHandler(c *gin.Context) {
    tableName := c.Param("table")

    client, err := getRAGClient()
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "无法连接到 RAG 服务器"})
        return
    }

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    schema, err := client.GetSchemaInfo(ctx, tableName, true)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, schema)
}

// 查询建议接口
func suggestionsHandler(c *gin.Context) {
    partial := c.Query("q")
    if partial == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "查询参数不能为空"})
        return
    }

    limit := 5
    if l := c.Query("limit"); l != "" {
        if parsed, err := strconv.Atoi(l); err == nil {
            limit = parsed
        }
    }

    client, err := getRAGClient()
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "无法连接到 RAG 服务器"})
        return
    }

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    suggestions, err := client.GetSuggestions(ctx, partial, limit)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{"suggestions": suggestions})
}

// 健康检查接口
func healthHandler(c *gin.Context) {
    clientMux.RLock()
    defer clientMux.RUnlock()

    if ragClient == nil {
        c.JSON(http.StatusServiceUnavailable, gin.H{
            "status":        "unhealthy",
            "connected":     false,
            "authenticated": false,
        })
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "status":        "healthy",
        "connected":     ragClient.IsConnected(),
        "authenticated": ragClient.IsAuthenticated(),
        "session_id":    ragClient.GetSessionID(),
    })
}

func main() {
    r := gin.Default()

    // 设置路由
    api := r.Group("/api")
    {
        api.POST("/query", queryHandler)
        api.GET("/schema", schemaHandler)
        api.GET("/schema/:table", schemaHandler)
        api.GET("/suggestions", suggestionsHandler)
    }

    r.GET("/health", healthHandler)

    // 启动服务器
    r.Run(":8080")
}
```

### 3. 连接池实现

```go
package ragclient

import (
    "context"
    "errors"
    "sync"
    "time"
)

// ConnectionPool RAG 客户端连接池
type ConnectionPool struct {
    url         string
    poolSize    int
    options     []ClientOption
    connections chan *RAGClient
    mutex       sync.RWMutex
    closed      bool
}

// NewConnectionPool 创建连接池
func NewConnectionPool(url string, poolSize int, options ...ClientOption) *ConnectionPool {
    return &ConnectionPool{
        url:         url,
        poolSize:    poolSize,
        options:     options,
        connections: make(chan *RAGClient, poolSize),
    }
}

// Initialize 初始化连接池
func (p *ConnectionPool) Initialize(ctx context.Context) error {
    for i := 0; i < p.poolSize; i++ {
        client := NewRAGClient(p.url, p.options...)

        if err := client.Connect(ctx); err != nil {
            return err
        }

        if err := client.Authenticate(ctx, "pool-jwt-token"); err != nil {
            client.Disconnect()
            return err
        }

        if err := client.CreateSession(ctx, fmt.Sprintf("pool-session-%d", i)); err != nil {
            client.Disconnect()
            return err
        }

        p.connections <- client
    }

    return nil
}

// Get 获取连接
func (p *ConnectionPool) Get(ctx context.Context) (*RAGClient, error) {
    p.mutex.RLock()
    if p.closed {
        p.mutex.RUnlock()
        return nil, errors.New("连接池已关闭")
    }
    p.mutex.RUnlock()

    select {
    case client := <-p.connections:
        // 检查连接状态
        if !client.IsConnected() {
            if err := client.Connect(ctx); err != nil {
                return nil, err
            }
            if err := client.Authenticate(ctx, "pool-jwt-token"); err != nil {
                return nil, err
            }
            if err := client.CreateSession(ctx, "pool-session-reconnect"); err != nil {
                return nil, err
            }
        }
        return client, nil
    case <-ctx.Done():
        return nil, ctx.Err()
    }
}

// Put 归还连接
func (p *ConnectionPool) Put(client *RAGClient) {
    p.mutex.RLock()
    if p.closed {
        p.mutex.RUnlock()
        client.Disconnect()
        return
    }
    p.mutex.RUnlock()

    select {
    case p.connections <- client:
    default:
        // 连接池已满，关闭连接
        client.Disconnect()
    }
}

// ExecuteQuery 使用连接池执行查询
func (p *ConnectionPool) ExecuteQuery(ctx context.Context, queryText string, options map[string]interface{}) (*QueryResponse, error) {
    client, err := p.Get(ctx)
    if err != nil {
        return nil, err
    }
    defer p.Put(client)

    return client.Query(ctx, queryText, options)
}

// Close 关闭连接池
func (p *ConnectionPool) Close() {
    p.mutex.Lock()
    defer p.mutex.Unlock()

    if p.closed {
        return
    }

    p.closed = true
    close(p.connections)

    // 关闭所有连接
    for client := range p.connections {
        client.Disconnect()
    }
}

// 使用示例
func ExampleConnectionPool() {
    pool := NewConnectionPool("ws://localhost:8080/mcp", 5,
        WithTimeout(30*time.Second),
    )

    ctx := context.Background()

    if err := pool.Initialize(ctx); err != nil {
        log.Fatalf("连接池初始化失败: %v", err)
    }
    defer pool.Close()

    // 执行查询
    result, err := pool.ExecuteQuery(ctx, "查询用户信息", nil)
    if err != nil {
        log.Printf("查询失败: %v", err)
        return
    }

    fmt.Printf("查询结果: %v\n", result)
}
```

### 4. 重试机制

```go
package ragclient

import (
    "context"
    "math"
    "math/rand"
    "time"
)

// RetryConfig 重试配置
type RetryConfig struct {
    MaxAttempts int
    BaseDelay   time.Duration
    MaxDelay    time.Duration
    Multiplier  float64
    Jitter      bool
}

// DefaultRetryConfig 默认重试配置
func DefaultRetryConfig() *RetryConfig {
    return &RetryConfig{
        MaxAttempts: 3,
        BaseDelay:   1 * time.Second,
        MaxDelay:    30 * time.Second,
        Multiplier:  2.0,
        Jitter:      true,
    }
}

// RetryableClient 支持重试的客户端
type RetryableClient struct {
    *RAGClient
    retryConfig *RetryConfig
}

// NewRetryableClient 创建支持重试的客户端
func NewRetryableClient(url string, retryConfig *RetryConfig, options ...ClientOption) *RetryableClient {
    if retryConfig == nil {
        retryConfig = DefaultRetryConfig()
    }

    return &RetryableClient{
        RAGClient:   NewRAGClient(url, options...),
        retryConfig: retryConfig,
    }
}

// QueryWithRetry 带重试的查询
func (rc *RetryableClient) QueryWithRetry(ctx context.Context, queryText string, options map[string]interface{}) (*QueryResponse, error) {
    var lastErr error

    for attempt := 0; attempt < rc.retryConfig.MaxAttempts; attempt++ {
        result, err := rc.Query(ctx, queryText, options)
        if err == nil {
            return result, nil
        }

        lastErr = err

        // 最后一次尝试，不需要等待
        if attempt == rc.retryConfig.MaxAttempts-1 {
            break
        }

        // 计算延迟时间
        delay := rc.calculateDelay(attempt)

        select {
        case <-time.After(delay):
            // 继续重试
        case <-ctx.Done():
            return nil, ctx.Err()
        }
    }

    return nil, lastErr
}

// calculateDelay 计算延迟时间
func (rc *RetryableClient) calculateDelay(attempt int) time.Duration {
    delay := float64(rc.retryConfig.BaseDelay) * math.Pow(rc.retryConfig.Multiplier, float64(attempt))

    if delay > float64(rc.retryConfig.MaxDelay) {
        delay = float64(rc.retryConfig.MaxDelay)
    }

    if rc.retryConfig.Jitter {
        // 添加随机抖动
        jitter := delay * 0.1 * rand.Float64()
        delay += jitter
    }

    return time.Duration(delay)
}

// ConnectWithRetry 带重试的连接
func (rc *RetryableClient) ConnectWithRetry(ctx context.Context) error {
    var lastErr error

    for attempt := 0; attempt < rc.retryConfig.MaxAttempts; attempt++ {
        err := rc.Connect(ctx)
        if err == nil {
            return nil
        }

        lastErr = err

        if attempt == rc.retryConfig.MaxAttempts-1 {
            break
        }

        delay := rc.calculateDelay(attempt)

        select {
        case <-time.After(delay):
        case <-ctx.Done():
            return ctx.Err()
        }
    }

    return lastErr
}
```

### 5. 测试示例

```go
package ragclient

import (
    "context"
    "testing"
    "time"
)

func TestRAGClient_Connect(t *testing.T) {
    client := NewRAGClient("ws://localhost:8080/mcp")
    defer client.Disconnect()

    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    err := client.Connect(ctx)
    if err != nil {
        t.Fatalf("连接失败: %v", err)
    }

    if !client.IsConnected() {
        t.Error("客户端应该处于连接状态")
    }
}

func TestRAGClient_Query(t *testing.T) {
    client := NewRAGClient("ws://localhost:8080/mcp")
    defer client.Disconnect()

    ctx := context.Background()

    // 连接和认证
    if err := client.Connect(ctx); err != nil {
        t.Fatalf("连接失败: %v", err)
    }

    if err := client.Authenticate(ctx, "test-token"); err != nil {
        t.Fatalf("认证失败: %v", err)
    }

    if err := client.CreateSession(ctx, "test-user"); err != nil {
        t.Fatalf("会话创建失败: %v", err)
    }

    // 执行查询
    result, err := client.Query(ctx, "查询用户信息", map[string]interface{}{
        "limit": 5,
    })

    if err != nil {
        t.Fatalf("查询失败: %v", err)
    }

    if !result.Success {
        t.Error("查询应该成功")
    }

    if len(result.Data) == 0 {
        t.Error("查询结果不应该为空")
    }
}

func TestConnectionPool(t *testing.T) {
    pool := NewConnectionPool("ws://localhost:8080/mcp", 3)
    defer pool.Close()

    ctx := context.Background()

    if err := pool.Initialize(ctx); err != nil {
        t.Fatalf("连接池初始化失败: %v", err)
    }

    // 测试并发查询
    results := make(chan *QueryResponse, 5)
    errors := make(chan error, 5)

    for i := 0; i < 5; i++ {
        go func() {
            result, err := pool.ExecuteQuery(ctx, "查询用户信息", nil)
            if err != nil {
                errors <- err
            } else {
                results <- result
            }
        }()
    }

    // 收集结果
    successCount := 0
    errorCount := 0

    for i := 0; i < 5; i++ {
        select {
        case <-results:
            successCount++
        case <-errors:
            errorCount++
        case <-time.After(30 * time.Second):
            t.Fatal("查询超时")
        }
    }

    if successCount != 5 {
        t.Errorf("期望5个成功查询，实际得到%d个", successCount)
    }
}

// 基准测试
func BenchmarkRAGClient_Query(b *testing.B) {
    client := NewRAGClient("ws://localhost:8080/mcp")
    defer client.Disconnect()

    ctx := context.Background()
    client.Connect(ctx)
    client.Authenticate(ctx, "test-token")
    client.CreateSession(ctx, "bench-user")

    b.ResetTimer()

    for i := 0; i < b.N; i++ {
        _, err := client.Query(ctx, "查询用户信息", map[string]interface{}{
            "limit": 10,
        })
        if err != nil {
            b.Fatalf("查询失败: %v", err)
        }
    }
}
```

这个 Go 客户端集成指南提供了完整的客户端实现，包括连接管理、查询功能、事件处理、连接池、重试机制和测试示例。开发者可以根据自己的需求选择合适的功能模块。
