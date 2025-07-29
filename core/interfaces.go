package core

import (
	"context"
	"time"
)

// RAGQueryProcessor RAG 查询处理器接口
type RAGQueryProcessor interface {
	ProcessQuery(ctx context.Context, request *QueryRequest) (*QueryResponse, error)
	GetSuggestions(ctx context.Context, partial string) ([]string, error)
	ExplainQuery(ctx context.Context, query string) (*QueryExplanation, error)
}

// SchemaManager 数据库 Schema 管理器接口
type SchemaManager interface {
	LoadSchema(ctx context.Context) error
	GetTableInfo(tableName string) (*TableInfo, error)
	GetRelationships(tableName string) ([]*Relationship, error)
	FindSimilarTables(query string) ([]*TableInfo, error)
	RefreshSchema(ctx context.Context) error
}

// LangChainManager LangChain 集成管理器接口
type LangChainManager interface {
	CreateChain(ctx context.Context, config *ChainConfig) (Chain, error)
	ExecuteChain(ctx context.Context, chain Chain, input *ChainInput) (*ChainOutput, error)
	GetMemory(sessionID string) (Memory, error)
	UpdateMemory(sessionID string, memory Memory) error
}

// Chain LangChain Chain 接口
type Chain interface {
	Run(ctx context.Context, input *ChainInput) (*ChainOutput, error)
	GetTools() []Tool
	AddTool(tool Tool) error
}

// Tool LangChain 工具接口
type Tool interface {
	Name() string
	Description() string
	Execute(ctx context.Context, input string) (string, error)
}

// Memory 内存管理接口
type Memory interface {
	GetHistory() []*QueryHistory
	AddHistory(history *QueryHistory)
	GetContext() map[string]interface{}
	SetContext(key string, value interface{})
	Clear()
}

// MCPServer MCP 协议服务器接口
type MCPServer interface {
	Start(ctx context.Context, addr string) error
	Stop(ctx context.Context) error
	RegisterHandler(method string, handler MCPHandler) error
	BroadcastNotification(notification *MCPNotification) error
}

// MCPHandler MCP 请求处理器接口
type MCPHandler interface {
	Handle(ctx context.Context, request *MCPRequest) (*MCPResponse, error)
}

// CacheManager 缓存管理器接口
type CacheManager interface {
	Get(ctx context.Context, key string) (interface{}, error)
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Clear(ctx context.Context) error
}

// Logger 日志记录器接口
type Logger interface {
	Debug(msg string, fields ...any)
	Info(msg string, fields ...any)
	Warn(msg string, fields ...any)
	Error(msg string, fields ...any)
	Fatal(msg string, fields ...any)
}

// MetricsCollector 指标收集器接口，用于收集和记录系统运行时的各类指标数据
type MetricsCollector interface {
	// IncrementCounter 增加指定名称和标签的计数器的值（通常用于记录事件发生次数）
	IncrementCounter(name string, labels map[string]string)
	// RecordHistogram 记录指定名称和标签的直方图数据（通常用于记录分布型数据，如响应时间）
	RecordHistogram(name string, value float64, labels map[string]string)
	// SetGauge 设置指定名称和标签的仪表值（通常用于记录当前状态值，如内存使用量）
	SetGauge(name string, value float64, labels map[string]string)
}

// SecurityManager 安全管理器接口
type SecurityManager interface {
	ValidateToken(ctx context.Context, token string) (*UserInfo, error)
	CheckPermission(ctx context.Context, user *UserInfo, resource string, action string) error
	ValidateSQL(ctx context.Context, sql string) error
}

// SessionManager 会话管理器接口
type SessionManager interface {
	CreateSession(ctx context.Context, userID string) (*SessionMemory, error)
	GetSession(ctx context.Context, sessionID string) (*SessionMemory, error)
	UpdateSession(ctx context.Context, session *SessionMemory) error
	DeleteSession(ctx context.Context, sessionID string) error
	CleanupExpiredSessions(ctx context.Context) error
	ListUserSessions(ctx context.Context, userID string) ([]*SessionMemory, error)
}
