package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"pumppill/rag/core"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

// MCPEndToEndTestSuite MCP 端到端测试套件
type MCPEndToEndTestSuite struct {
	suite.Suite
	ctx            context.Context
	cancel         context.CancelFunc
	logger         *zap.Logger
	server         *Server
	serverAddr     string
	queryProcessor *MockQueryProcessor
}

// SetupSuite 设置测试套件
func (suite *MCPEndToEndTestSuite) SetupSuite() {
	// 检查是否启用 MCP 端到端测试
	if os.Getenv("MCP_E2E_TEST") != "true" {
		suite.T().Skip("跳过 MCP 端到端测试，设置 MCP_E2E_TEST=true 启用")
	}

	suite.ctx, suite.cancel = context.WithTimeout(context.Background(), 2*time.Minute)

	// 创建日志记录器
	var err error
	suite.logger, err = zap.NewDevelopment()
	require.NoError(suite.T(), err)

	// 设置组件
	suite.setupComponents()

	// 启动服务器
	suite.startServer()
}

// TearDownSuite 清理测试套件
func (suite *MCPEndToEndTestSuite) TearDownSuite() {
	if suite.server != nil {
		suite.server.Stop(suite.ctx)
	}

	if suite.cancel != nil {
		suite.cancel()
	}

	if suite.logger != nil {
		suite.logger.Sync()
	}
}

// setupComponents 设置组件
func (suite *MCPEndToEndTestSuite) setupComponents() {
	// 创建 Mock 查询处理器
	suite.queryProcessor = &MockQueryProcessor{}

	// 创建 MCP 服务器
	config := &Config{
		Host:           "localhost",
		Port:           0, // 使用随机端口
		MaxConnections: 10,
		Timeout:        30 * time.Second,
	}

	suite.server = NewServer(config, suite.queryProcessor, suite.logger)
}

// startServer 启动服务器
func (suite *MCPEndToEndTestSuite) startServer() {
	// 获取可用端口
	listener, err := net.Listen("tcp", ":0")
	require.NoError(suite.T(), err)

	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	suite.serverAddr = fmt.Sprintf("localhost:%d", port)

	// 启动服务器
	go func() {
		err := suite.server.Start(suite.ctx, suite.serverAddr)
		if err != nil && err != context.Canceled {
			suite.logger.Error("MCP 服务器启动失败", zap.Error(err))
		}
	}()

	// 等待服务器启动
	time.Sleep(100 * time.Millisecond)
}

// TestServerStartup 测试服务器启动
func (suite *MCPEndToEndTestSuite) TestServerStartup() {
	// 验证服务器已启动
	assert.NotNil(suite.T(), suite.server)
	assert.NotEmpty(suite.T(), suite.serverAddr)

	// 尝试连接服务器
	conn, err := net.DialTimeout("tcp", suite.serverAddr, 5*time.Second)
	if err == nil {
		conn.Close()
	}
	assert.NoError(suite.T(), err, "应该能够连接到服务器")
}

// TestWebSocketConnection 测试 WebSocket 连接
func (suite *MCPEndToEndTestSuite) TestWebSocketConnection() {
	// 连接到 WebSocket 服务器
	url := fmt.Sprintf("ws://%s/mcp", suite.serverAddr)
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	require.NoError(suite.T(), err)
	defer conn.Close()

	// 设置读取超时
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	// 发送 ping 消息
	err = conn.WriteMessage(websocket.PingMessage, []byte("ping"))
	assert.NoError(suite.T(), err)

	// 等待 pong 响应
	messageType, message, err := conn.ReadMessage()
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), websocket.PongMessage, messageType)
	assert.Equal(suite.T(), []byte("ping"), message)
}

// TestMCPHandshake 测试 MCP 握手
func (suite *MCPEndToEndTestSuite) TestMCPHandshake() {
	// 连接到服务器
	conn := suite.connectToServer()
	defer conn.Close()

	// 发送初始化请求
	initRequest := &core.MCPMessage{
		Type:   "request",
		ID:     "init_1",
		Method: "initialize",
		Params: map[string]interface{}{
			"protocolVersion": "1.0",
			"clientInfo": map[string]interface{}{
				"name":    "test-client",
				"version": "1.0.0",
			},
		},
	}

	err := suite.sendMessage(conn, initRequest)
	require.NoError(suite.T(), err)

	// 接收初始化响应
	response, err := suite.receiveMessage(conn)
	require.NoError(suite.T(), err)

	assert.Equal(suite.T(), "response", response.Type)
	assert.Equal(suite.T(), "init_1", response.ID)
	assert.NotNil(suite.T(), response.Result)
}

// TestQueryExecution 测试查询执行
func (suite *MCPEndToEndTestSuite) TestQueryExecution() {
	// 设置 Mock 查询处理器响应
	suite.queryProcessor.SetResponse(&core.QueryResponse{
		Success: true,
		Data: []map[string]any{
			{"id": 1, "name": "张三", "age": 25},
			{"id": 2, "name": "李四", "age": 30},
		},
		SQL:         "SELECT * FROM users",
		Explanation: "查询所有用户信息",
		Metadata: &core.QueryMetadata{
			ExecutionTime: 50 * time.Millisecond,
			RowCount:      2,
		},
	})

	// 连接到服务器
	conn := suite.connectToServer()
	defer conn.Close()

	// 发送查询请求
	queryRequest := &core.MCPMessage{
		Type:   "request",
		ID:     "query_1",
		Method: "query",
		Params: map[string]interface{}{
			"query":      "查找所有用户",
			"session_id": "test_session",
			"options": map[string]interface{}{
				"explain": true,
				"limit":   100,
			},
		},
	}

	err := suite.sendMessage(conn, queryRequest)
	require.NoError(suite.T(), err)

	// 接收查询响应
	response, err := suite.receiveMessage(conn)
	require.NoError(suite.T(), err)

	assert.Equal(suite.T(), "response", response.Type)
	assert.Equal(suite.T(), "query_1", response.ID)
	assert.NotNil(suite.T(), response.Result)

	// 验证响应内容
	result, ok := response.Result.(map[string]interface{})
	require.True(suite.T(), ok)

	assert.True(suite.T(), result["success"].(bool))
	assert.NotNil(suite.T(), result["data"])
	assert.Equal(suite.T(), "SELECT * FROM users", result["sql"])
	assert.Equal(suite.T(), "查询所有用户信息", result["explanation"])
}

// TestQuerySuggestions 测试查询建议
func (suite *MCPEndToEndTestSuite) TestQuerySuggestions() {
	// 设置 Mock 查询处理器响应
	suite.queryProcessor.SetSuggestions([]string{
		"SELECT * FROM users",
		"SELECT name FROM users",
		"SELECT COUNT(*) FROM users",
	})

	// 连接到服务器
	conn := suite.connectToServer()
	defer conn.Close()

	// 发送建议请求
	suggestionRequest := &core.MCPMessage{
		Type:   "request",
		ID:     "suggest_1",
		Method: "suggestions",
		Params: map[string]interface{}{
			"partial": "用户",
		},
	}

	err := suite.sendMessage(conn, suggestionRequest)
	require.NoError(suite.T(), err)

	// 接收建议响应
	response, err := suite.receiveMessage(conn)
	require.NoError(suite.T(), err)

	assert.Equal(suite.T(), "response", response.Type)
	assert.Equal(suite.T(), "suggest_1", response.ID)
	assert.NotNil(suite.T(), response.Result)

	// 验证建议内容
	suggestions, ok := response.Result.([]interface{})
	require.True(suite.T(), ok)
	assert.Len(suite.T(), suggestions, 3)
	assert.Contains(suite.T(), suggestions, "SELECT * FROM users")
}

// TestQueryExplanation 测试查询解释
func (suite *MCPEndToEndTestSuite) TestQueryExplanation() {
	// 设置 Mock 查询处理器响应
	suite.queryProcessor.SetExplanation(&core.QueryExplanation{
		Intent:     "查询用户信息",
		Tables:     []string{"users"},
		Columns:    []string{"id", "name", "age"},
		Conditions: []string{"无条件"},
		Operations: []string{"SELECT"},
		Complexity: "简单",
		Suggestions: []string{
			"可以添加 WHERE 条件来过滤结果",
			"考虑使用 LIMIT 限制返回行数",
		},
	})

	// 连接到服务器
	conn := suite.connectToServer()
	defer conn.Close()

	// 发送解释请求
	explainRequest := &core.MCPMessage{
		Type:   "request",
		ID:     "explain_1",
		Method: "explain",
		Params: map[string]interface{}{
			"query": "SELECT * FROM users",
		},
	}

	err := suite.sendMessage(conn, explainRequest)
	require.NoError(suite.T(), err)

	// 接收解释响应
	response, err := suite.receiveMessage(conn)
	require.NoError(suite.T(), err)

	assert.Equal(suite.T(), "response", response.Type)
	assert.Equal(suite.T(), "explain_1", response.ID)
	assert.NotNil(suite.T(), response.Result)

	// 验证解释内容
	explanation, ok := response.Result.(map[string]interface{})
	require.True(suite.T(), ok)

	assert.Equal(suite.T(), "查询用户信息", explanation["intent"])
	assert.Contains(suite.T(), explanation["tables"], "users")
	assert.Equal(suite.T(), "简单", explanation["complexity"])
}

// TestErrorHandling 测试错误处理
func (suite *MCPEndToEndTestSuite) TestErrorHandling() {
	// 设置 Mock 查询处理器返回错误
	suite.queryProcessor.SetError(core.NewRAGError(
		core.ErrorTypeValidation,
		"INVALID_QUERY",
		"查询语句无效",
	))

	// 连接到服务器
	conn := suite.connectToServer()
	defer conn.Close()

	// 发送无效查询请求
	queryRequest := &core.MCPMessage{
		Type:   "request",
		ID:     "error_1",
		Method: "query",
		Params: map[string]interface{}{
			"query": "", // 空查询
		},
	}

	err := suite.sendMessage(conn, queryRequest)
	require.NoError(suite.T(), err)

	// 接收错误响应
	response, err := suite.receiveMessage(conn)
	require.NoError(suite.T(), err)

	assert.Equal(suite.T(), "response", response.Type)
	assert.Equal(suite.T(), "error_1", response.ID)
	assert.NotNil(suite.T(), response.Error)

	// 验证错误内容
	mcpError := response.Error
	assert.Equal(suite.T(), 400, mcpError.Code)
	assert.Equal(suite.T(), "查询语句无效", mcpError.Message)
}

// TestConcurrentConnections 测试并发连接
func (suite *MCPEndToEndTestSuite) TestConcurrentConnections() {
	const numConnections = 5
	const numRequests = 3

	// 设置 Mock 查询处理器响应
	suite.queryProcessor.SetResponse(&core.QueryResponse{
		Success: true,
		Data:    []map[string]any{{"result": "ok"}},
		SQL:     "SELECT 1",
	})

	done := make(chan bool, numConnections)

	// 创建多个并发连接
	for i := 0; i < numConnections; i++ {
		go func(connID int) {
			defer func() { done <- true }()

			// 连接到服务器
			conn := suite.connectToServer()
			defer conn.Close()

			// 发送多个请求
			for j := 0; j < numRequests; j++ {
				requestID := fmt.Sprintf("concurrent_%d_%d", connID, j)

				queryRequest := &core.MCPMessage{
					Type:   "request",
					ID:     requestID,
					Method: "query",
					Params: map[string]interface{}{
						"query": fmt.Sprintf("测试查询 %d-%d", connID, j),
					},
				}

				err := suite.sendMessage(conn, queryRequest)
				assert.NoError(suite.T(), err)

				// 接收响应
				response, err := suite.receiveMessage(conn)
				assert.NoError(suite.T(), err)
				assert.Equal(suite.T(), requestID, response.ID)
			}
		}(i)
	}

	// 等待所有连接完成
	for i := 0; i < numConnections; i++ {
		<-done
	}
}

// TestConnectionTimeout 测试连接超时
func (suite *MCPEndToEndTestSuite) TestConnectionTimeout() {
	// 连接到服务器
	conn := suite.connectToServer()
	defer conn.Close()

	// 设置较短的读取超时
	conn.SetReadDeadline(time.Now().Add(1 * time.Second))

	// 不发送任何消息，等待超时
	_, _, err := conn.ReadMessage()
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "timeout")
}

// TestInvalidMessage 测试无效消息
func (suite *MCPEndToEndTestSuite) TestInvalidMessage() {
	// 连接到服务器
	conn := suite.connectToServer()
	defer conn.Close()

	// 发送无效的 JSON 消息
	err := conn.WriteMessage(websocket.TextMessage, []byte("invalid json"))
	require.NoError(suite.T(), err)

	// 接收错误响应
	response, err := suite.receiveMessage(conn)
	require.NoError(suite.T(), err)

	assert.Equal(suite.T(), "response", response.Type)
	assert.NotNil(suite.T(), response.Error)
	assert.Contains(suite.T(), response.Error.Message, "JSON")
}

// TestHealthCheck 测试健康检查
func (suite *MCPEndToEndTestSuite) TestHealthCheck() {
	// 连接到服务器
	conn := suite.connectToServer()
	defer conn.Close()

	// 发送健康检查请求
	healthRequest := &core.MCPMessage{
		Type:   "request",
		ID:     "health_1",
		Method: "health",
		Params: map[string]interface{}{},
	}

	err := suite.sendMessage(conn, healthRequest)
	require.NoError(suite.T(), err)

	// 接收健康检查响应
	response, err := suite.receiveMessage(conn)
	require.NoError(suite.T(), err)

	assert.Equal(suite.T(), "response", response.Type)
	assert.Equal(suite.T(), "health_1", response.ID)
	assert.NotNil(suite.T(), response.Result)

	// 验证健康状态
	health, ok := response.Result.(map[string]interface{})
	require.True(suite.T(), ok)
	assert.Equal(suite.T(), "healthy", health["status"])
}

// TestVersionInfo 测试版本信息
func (suite *MCPEndToEndTestSuite) TestVersionInfo() {
	// 连接到服务器
	conn := suite.connectToServer()
	defer conn.Close()

	// 发送版本信息请求
	versionRequest := &core.MCPMessage{
		Type:   "request",
		ID:     "version_1",
		Method: "version",
		Params: map[string]interface{}{},
	}

	err := suite.sendMessage(conn, versionRequest)
	require.NoError(suite.T(), err)

	// 接收版本信息响应
	response, err := suite.receiveMessage(conn)
	require.NoError(suite.T(), err)

	assert.Equal(suite.T(), "response", response.Type)
	assert.Equal(suite.T(), "version_1", response.ID)
	assert.NotNil(suite.T(), response.Result)

	// 验证版本信息
	version, ok := response.Result.(map[string]interface{})
	require.True(suite.T(), ok)
	assert.NotEmpty(suite.T(), version["version"])
	assert.NotEmpty(suite.T(), version["protocol_version"])
}

// connectToServer 连接到服务器
func (suite *MCPEndToEndTestSuite) connectToServer() *websocket.Conn {
	url := fmt.Sprintf("ws://%s/mcp", suite.serverAddr)
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	require.NoError(suite.T(), err)

	// 设置读写超时
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))

	return conn
}

// sendMessage 发送消息
func (suite *MCPEndToEndTestSuite) sendMessage(conn *websocket.Conn, message *core.MCPMessage) error {
	data, err := json.Marshal(message)
	if err != nil {
		return err
	}

	return conn.WriteMessage(websocket.TextMessage, data)
}

// receiveMessage 接收消息
func (suite *MCPEndToEndTestSuite) receiveMessage(conn *websocket.Conn) (*core.MCPMessage, error) {
	_, data, err := conn.ReadMessage()
	if err != nil {
		return nil, err
	}

	var message core.MCPMessage
	err = json.Unmarshal(data, &message)
	if err != nil {
		return nil, err
	}

	return &message, nil
}

// MockQueryProcessor Mock 查询处理器
type MockQueryProcessor struct {
	response    *core.QueryResponse
	suggestions []string
	explanation *core.QueryExplanation
	error       error
}

func (m *MockQueryProcessor) ProcessQuery(ctx context.Context, request *core.QueryRequest) (*core.QueryResponse, error) {
	if m.error != nil {
		return nil, m.error
	}
	return m.response, nil
}

func (m *MockQueryProcessor) GetSuggestions(ctx context.Context, partial string) ([]string, error) {
	if m.error != nil {
		return nil, m.error
	}
	return m.suggestions, nil
}

func (m *MockQueryProcessor) ExplainQuery(ctx context.Context, query string) (*core.QueryExplanation, error) {
	if m.error != nil {
		return nil, m.error
	}
	return m.explanation, nil
}

func (m *MockQueryProcessor) SetResponse(response *core.QueryResponse) {
	m.response = response
	m.error = nil
}

func (m *MockQueryProcessor) SetSuggestions(suggestions []string) {
	m.suggestions = suggestions
	m.error = nil
}

func (m *MockQueryProcessor) SetExplanation(explanation *core.QueryExplanation) {
	m.explanation = explanation
	m.error = nil
}

func (m *MockQueryProcessor) SetError(err error) {
	m.error = err
}

// TestMCPEndToEnd 运行 MCP 端到端测试套件
func TestMCPEndToEnd(t *testing.T) {
	suite.Run(t, new(MCPEndToEndTestSuite))
}

// BenchmarkMCPCommunication MCP 通信基准测试
func BenchmarkMCPCommunication(b *testing.B) {
	if os.Getenv("MCP_E2E_TEST") != "true" {
		b.Skip("跳过 MCP 端到端基准测试，设置 MCP_E2E_TEST=true 启用")
	}

	// 设置测试环境
	logger, _ := zap.NewDevelopment()
	queryProcessor := &MockQueryProcessor{}
	queryProcessor.SetResponse(&core.QueryResponse{
		Success: true,
		Data:    []map[string]any{{"result": "ok"}},
		SQL:     "SELECT 1",
	})

	// 创建并启动服务器
	config := &Config{
		Host:           "localhost",
		Port:           0,
		MaxConnections: 100,
		Timeout:        30 * time.Second,
	}

	server := NewServer(config, queryProcessor, logger)

	// 获取可用端口
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		b.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	serverAddr := fmt.Sprintf("localhost:%d", port)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		server.Start(ctx, serverAddr)
	}()

	time.Sleep(100 * time.Millisecond)

	// 连接到服务器
	url := fmt.Sprintf("ws://%s/mcp", serverAddr)
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		b.Fatal(err)
	}
	defer conn.Close()

	// 基准测试
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		queryRequest := &core.MCPMessage{
			Type:   "request",
			ID:     fmt.Sprintf("bench_%d", i),
			Method: "query",
			Params: map[string]interface{}{
				"query": "测试查询",
			},
		}

		// 发送请求
		data, _ := json.Marshal(queryRequest)
		err := conn.WriteMessage(websocket.TextMessage, data)
		if err != nil {
			b.Fatal(err)
		}

		// 接收响应
		_, _, err = conn.ReadMessage()
		if err != nil {
			b.Fatal(err)
		}
	}
}
