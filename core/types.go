package core

import (
	"time"
)

// QueryRequest 查询请求结构体，表示一次查询的所有输入参数。
// Query：查询语句或问题内容。
// Context：查询上下文信息，可包含用户、环境等额外参数。
// Options：查询选项，控制查询行为（如限制、格式等）。
// SessionID：会话标识，用于追踪同一会话的多次查询。
// UserID：用户标识，区分不同用户。
// RequestID：请求唯一标识，便于日志和追踪。
type QueryRequest struct {
	Query     string         `json:"query" validate:"required"` // 查询内容，必填
	Context   map[string]any `json:"context,omitempty"`         // 查询上下文，可选
	Options   *QueryOptions  `json:"options,omitempty"`         // 查询选项，可选
	SessionID string         `json:"session_id,omitempty"`      // 会话ID，可选
	UserID    string         `json:"user_id,omitempty"`         // 用户ID，可选
	RequestID string         `json:"request_id,omitempty"`      // 请求ID，可选
}

// QueryOptions 查询选项结构体，控制查询的细节参数。
// Limit：返回结果的最大条数。
// Offset：结果偏移量，用于分页。
// Format：结果格式，如json、csv、table。
// Explain：是否返回查询解释。
// Optimize：是否优化查询。
// Timeout：查询超时时间（秒）。
type QueryOptions struct {
	Limit    int    `json:"limit,omitempty"`    // 最大返回条数
	Offset   int    `json:"offset,omitempty"`   // 结果偏移量
	Format   string `json:"format,omitempty"`   // 结果格式
	Explain  bool   `json:"explain,omitempty"`  // 是否解释查询
	Optimize bool   `json:"optimize,omitempty"` // 是否优化查询
	Timeout  int    `json:"timeout,omitempty"`  // 超时时间（秒）
}

// QueryResponse 查询响应结构体，表示一次查询的返回结果。
// Success：查询是否成功。
// Data：查询结果数据。
// SQL：实际执行的SQL语句。
// Explanation：查询解释信息。
// Suggestions：查询建议。
// Error：错误信息。
// Metadata：查询元数据。
// RequestID：请求唯一标识。
type QueryResponse struct {
	Success     bool             `json:"success"`               // 查询是否成功
	Data        []map[string]any `json:"data,omitempty"`        // 查询结果数据
	SQL         string           `json:"sql,omitempty"`         // 执行的SQL语句
	Explanation string           `json:"explanation,omitempty"` // 查询解释
	Suggestions []string         `json:"suggestions,omitempty"` // 查询建议
	Error       string           `json:"error,omitempty"`       // 错误信息
	Metadata    *QueryMetadata   `json:"metadata,omitempty"`    // 查询元数据
	RequestID   string           `json:"request_id,omitempty"`  // 请求ID
}

// QueryMetadata 查询元数据结构体，描述查询的统计和执行信息。
// ExecutionTime：查询执行耗时。
// RowCount：返回行数。
// AffectedRows：受影响行数。
// QueryPlan：查询执行计划。
// Warnings：警告信息。
// CacheHit：是否命中缓存。
type QueryMetadata struct {
	ExecutionTime time.Duration `json:"execution_time"`          // 执行耗时
	RowCount      int           `json:"row_count"`               // 返回行数
	AffectedRows  int           `json:"affected_rows,omitempty"` // 受影响行数
	QueryPlan     string        `json:"query_plan,omitempty"`    // 查询计划
	Warnings      []string      `json:"warnings,omitempty"`      // 警告信息
	CacheHit      bool          `json:"cache_hit,omitempty"`     // 是否命中缓存
}

// QueryExplanation 查询解释结构体，分析查询意图和结构。
// Intent：查询意图。
// Tables：涉及的表。
// Columns：涉及的列。
// Conditions：查询条件。
// Operations：执行的操作。
// Complexity：查询复杂度。
// Suggestions：优化建议。
type QueryExplanation struct {
	Intent      string   `json:"intent"`                // 查询意图
	Tables      []string `json:"tables"`                // 涉及表
	Columns     []string `json:"columns"`               // 涉及列
	Conditions  []string `json:"conditions"`            // 查询条件
	Operations  []string `json:"operations"`            // 操作类型
	Complexity  string   `json:"complexity"`            // 查询复杂度
	Suggestions []string `json:"suggestions,omitempty"` // 优化建议
}

// TableInfo 表信息结构体，描述数据库表的详细信息。
// Name：表名。
// Comment：表注释。
// Engine：存储引擎。
// Charset：字符集。
// Columns：列信息。
// Indexes：索引信息。
// ForeignKeys：外键信息。
// RowCount：行数。
// DataLength：数据长度。
// CreatedAt：创建时间。
// UpdatedAt：更新时间。
type TableInfo struct {
	Name        string        `json:"name"`         // 表名
	Comment     string        `json:"comment"`      // 表注释
	Engine      string        `json:"engine"`       // 存储引擎
	Charset     string        `json:"charset"`      // 字符集
	Columns     []*Column     `json:"columns"`      // 列信息
	Indexes     []*Index      `json:"indexes"`      // 索引信息
	ForeignKeys []*ForeignKey `json:"foreign_keys"` // 外键信息
	RowCount    int64         `json:"row_count"`    // 行数
	DataLength  int64         `json:"data_length"`  // 数据长度
	CreatedAt   time.Time     `json:"created_at"`   // 创建时间
	UpdatedAt   time.Time     `json:"updated_at"`   // 更新时间
}

// Column 列信息结构体，描述表的字段属性。
// Name：列名。
// Type：数据类型。
// Nullable：是否可为空。
// DefaultValue：默认值。
// Comment：列注释。
// IsPrimaryKey：是否主键。
// IsAutoIncrement：是否自增。
// CharacterSet：字符集。
// Collation：排序规则。
// Position：列位置。
type Column struct {
	Name            string `json:"name"`                    // 列名
	Type            string `json:"type"`                    // 数据类型
	Nullable        bool   `json:"nullable"`                // 是否可为空
	DefaultValue    string `json:"default_value"`           // 默认值
	Comment         string `json:"comment"`                 // 列注释
	IsPrimaryKey    bool   `json:"is_primary_key"`          // 是否主键
	IsAutoIncrement bool   `json:"is_auto_increment"`       // 是否自增
	CharacterSet    string `json:"character_set,omitempty"` // 字符集
	Collation       string `json:"collation,omitempty"`     // 排序规则
	Position        int    `json:"position"`                // 列位置
}

// Index 索引信息结构体，描述表的索引属性。
// Name：索引名。
// Type：索引类型（主键、唯一、普通、全文）。
// Columns：索引包含的列。
// IsUnique：是否唯一索引。
type Index struct {
	Name     string   `json:"name"`      // 索引名
	Type     string   `json:"type"`      // 索引类型
	Columns  []string `json:"columns"`   // 索引列
	IsUnique bool     `json:"is_unique"` // 是否唯一
}

// ForeignKey 外键信息结构体，描述表的外键约束。
// Name：外键名。
// Column：本表列。
// ReferencedTable：引用表。
// ReferencedColumn：引用表列。
// OnUpdate：更新时动作。
// OnDelete：删除时动作。
type ForeignKey struct {
	Name             string `json:"name"`              // 外键名
	Column           string `json:"column"`            // 本表列
	ReferencedTable  string `json:"referenced_table"`  // 引用表
	ReferencedColumn string `json:"referenced_column"` // 引用表列
	OnUpdate         string `json:"on_update"`         // 更新时动作
	OnDelete         string `json:"on_delete"`         // 删除时动作
}

// Relationship 表关系结构体，描述表之间的关联关系。
// Type：关系类型（如一对一、一对多、多对多）。
// FromTable：源表。
// FromColumn：源表列。
// ToTable：目标表。
// ToColumn：目标表列。
// RelationshipKey：关系键。
type Relationship struct {
	Type            string `json:"type"`             // 关系类型
	FromTable       string `json:"from_table"`       // 源表
	FromColumn      string `json:"from_column"`      // 源表列
	ToTable         string `json:"to_table"`         // 目标表
	ToColumn        string `json:"to_column"`        // 目标表列
	RelationshipKey string `json:"relationship_key"` // 关系键
}

// SchemaInfo Schema信息结构体，描述数据库结构。
// Database：数据库名。
// Tables：表信息列表。
// Views：视图信息列表。
// Version：Schema版本。
// UpdatedAt：更新时间。
type SchemaInfo struct {
	Database  string       `json:"database"`   // 数据库名
	Tables    []*TableInfo `json:"tables"`     // 表信息
	Views     []*ViewInfo  `json:"views"`      // 视图信息
	Version   string       `json:"version"`    // Schema版本
	UpdatedAt time.Time    `json:"updated_at"` // 更新时间
}

// ViewInfo 视图信息结构体，描述数据库视图。
// Name：视图名。
// Definition：视图定义SQL。
// Comment：视图注释。
// CreatedAt：创建时间。
type ViewInfo struct {
	Name       string    `json:"name"`       // 视图名
	Definition string    `json:"definition"` // 视图定义
	Comment    string    `json:"comment"`    // 视图注释
	CreatedAt  time.Time `json:"created_at"` // 创建时间
}

// ChainConfig 链式处理配置结构体，定义链的类型、模型、工具、内存和扩展参数。
// Type：链类型（如SQL生成、查询解释）。
// LLMConfig：大语言模型配置。
// Tools：工具列表。
// Memory：内存配置。
// Options：其他可选参数。
type ChainConfig struct {
	Type      string         `json:"type"`              // 链类型
	LLMConfig *LLMConfig     `json:"llm_config"`        // 大语言模型配置
	Tools     []string       `json:"tools"`             // 工具列表
	Memory    *MemoryConfig  `json:"memory,omitempty"`  // 内存配置
	Options   map[string]any `json:"options,omitempty"` // 其他参数
}

// LLMConfig 大语言模型配置结构体，描述模型服务商、模型类型及参数。
// Provider：模型服务商。
// Model：模型名称。
// Temperature：采样温度。
// MaxTokens：最大生成Token数。
// Options：其他扩展参数。
type LLMConfig struct {
	Provider    string         `json:"provider"`          // 服务商
	Model       string         `json:"model"`             // 模型名称
	Temperature float32        `json:"temperature"`       // 采样温度
	MaxTokens   int            `json:"max_tokens"`        // 最大Token数
	Options     map[string]any `json:"options,omitempty"` // 其他参数
}

// MemoryConfig 内存配置结构体，定义链式处理或系统运行时的内存参数。
// Type：内存类型（如buffer、redis）。
// Size：容量限制。
// TTL：数据生存时间。
// Options：其他扩展参数。
type MemoryConfig struct {
	Type    string         `json:"type"`              // 内存类型
	Size    int            `json:"size"`              // 容量限制
	TTL     time.Duration  `json:"ttl"`               // 生存时间
	Options map[string]any `json:"options,omitempty"` // 其他参数
}

// ChainInput 链输入结构体，定义链式处理的输入参数。
// Query：查询内容。
// Context：上下文信息。
// Variables：变量参数。
// Schema：数据库结构信息。
type ChainInput struct {
	Query     string         `json:"query"`               // 查询内容
	Context   map[string]any `json:"context,omitempty"`   // 上下文
	Variables map[string]any `json:"variables,omitempty"` // 变量
	Schema    *SchemaInfo    `json:"schema,omitempty"`    // 数据库结构
}

// ChainOutput 链输出结构体，定义链式处理的输出结果。
// Result：处理结果。
// Metadata：元数据信息。
// Usage：Token使用统计。
type ChainOutput struct {
	Result   any            `json:"result"`             // 处理结果
	Metadata map[string]any `json:"metadata,omitempty"` // 元数据
	Usage    *TokenUsage    `json:"usage,omitempty"`    // Token统计
}

// TokenUsage Token使用统计结构体，记录Token消耗情况。
// PromptTokens：提示词Token数。
// CompletionTokens：生成内容Token数。
// TotalTokens：总Token数。
type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`     // 提示词Token数
	CompletionTokens int `json:"completion_tokens"` // 生成内容Token数
	TotalTokens      int `json:"total_tokens"`      // 总Token数
}

// QueryHistory 查询历史结构体，记录每次查询的详细信息。
// Query：查询内容。
// SQL：执行的SQL语句。
// Results：查询结果。
// Timestamp：时间戳。
// Success：是否成功。
// Error：错误信息。
// ExecutionTime：执行耗时。
type QueryHistory struct {
	Query         string        `json:"query"`           // 查询内容
	SQL           string        `json:"sql"`             // 执行SQL
	Results       any           `json:"results"`         // 查询结果
	Timestamp     time.Time     `json:"timestamp"`       // 时间戳
	Success       bool          `json:"success"`         // 是否成功
	Error         string        `json:"error,omitempty"` // 错误信息
	ExecutionTime time.Duration `json:"execution_time"`  // 执行耗时
}

// SessionMemory 会话内存结构体，记录用户会话相关信息。
// SessionID：会话ID。
// UserID：用户ID。
// History：查询历史。
// Context：上下文信息。
// Preferences：用户偏好。
// CreatedAt：创建时间。
// LastAccessed：最后访问时间。
type SessionMemory struct {
	SessionID    string           `json:"session_id"`    // 会话ID
	UserID       string           `json:"user_id"`       // 用户ID
	History      []*QueryHistory  `json:"history"`       // 查询历史
	Context      map[string]any   `json:"context"`       // 上下文
	Preferences  *UserPreferences `json:"preferences"`   // 用户偏好
	CreatedAt    time.Time        `json:"created_at"`    // 创建时间
	LastAccessed time.Time        `json:"last_accessed"` // 最后访问时间
}

// UserPreferences 用户偏好设置结构体，记录用户的个性化设置。
// Language：语言。
// DateFormat：日期格式。
// TimeZone：时区。
// DefaultLimit：默认查询条数。
// ExplainQueries：是否自动解释查询。
// OptimizeQueries：是否自动优化查询。
type UserPreferences struct {
	Language        string `json:"language"`         // 语言
	DateFormat      string `json:"date_format"`      // 日期格式
	TimeZone        string `json:"time_zone"`        // 时区
	DefaultLimit    int    `json:"default_limit"`    // 默认条数
	ExplainQueries  bool   `json:"explain_queries"`  // 自动解释
	OptimizeQueries bool   `json:"optimize_queries"` // 自动优化
}

// UserInfo 用户信息结构体，描述系统用户的基本信息。
// ID：用户ID。
// Username：用户名。
// Email：邮箱。
// Roles：角色列表。
// Permissions：权限列表。
type UserInfo struct {
	ID          string   `json:"id"`          // 用户ID
	Username    string   `json:"username"`    // 用户名
	Email       string   `json:"email"`       // 邮箱
	Roles       []string `json:"roles"`       // 角色
	Permissions []string `json:"permissions"` // 权限
}

// MCPMessage MCP消息结构体，表示MCP协议的消息内容。
// Type：消息类型（请求、响应、通知）。
// ID：消息ID。
// Method：方法名。
// Params：参数。
// Result：结果。
// Error：错误信息。
type MCPMessage struct {
	Type   string    `json:"type"`             // 消息类型
	ID     string    `json:"id,omitempty"`     // 消息ID
	Method string    `json:"method,omitempty"` // 方法名
	Params any       `json:"params,omitempty"` // 参数
	Result any       `json:"result,omitempty"` // 结果
	Error  *MCPError `json:"error,omitempty"`  // 错误信息
}

// MCPRequest MCP请求结构体，表示MCP协议的请求消息。
// ID：请求ID。
// Method：方法名。
// Params：参数。
type MCPRequest struct {
	ID     string `json:"id"`     // 请求ID
	Method string `json:"method"` // 方法名
	Params any    `json:"params"` // 参数
}

// MCPResponse MCP响应结构体，表示MCP协议的响应消息。
// ID：响应对应的请求ID。
// Result：响应结果。
// Error：错误信息。
type MCPResponse struct {
	ID     string    `json:"id"`               // 响应ID
	Result any       `json:"result,omitempty"` // 响应结果
	Error  *MCPError `json:"error,omitempty"`  // 错误信息
}

// MCPNotification MCP通知结构体，表示MCP协议的通知消息。
// Method：方法名。
// Params：参数。
type MCPNotification struct {
	Method string `json:"method"` // 方法名
	Params any    `json:"params"` // 参数
}

// MCPError MCP错误结构体，描述MCP协议的错误信息。
// Code：错误码。
// Message：错误描述。
// Data：附加数据。
type MCPError struct {
	Code    int    `json:"code"`           // 错误码
	Message string `json:"message"`        // 错误描述
	Data    any    `json:"data,omitempty"` // 附加数据
}

// Config 系统配置结构体，集中管理各模块配置。
// Server：服务器配置。
// Database：数据库配置。
// LLM：大语言模型配置。
// Cache：缓存配置。
// Log：日志配置。
// Security：安全配置。
// MCP：MCP协议配置。
type Config struct {
	Server   *ServerConfig   `yaml:"server"`   // 服务器配置
	Database *DatabaseConfig `yaml:"database"` // 数据库配置
	LLM      *LLMConfig      `yaml:"llm"`      // 大语言模型配置
	Cache    *CacheConfig    `yaml:"cache"`    // 缓存配置
	Log      *LogConfig      `yaml:"log"`      // 日志配置
	Security *SecurityConfig `yaml:"security"` // 安全配置
	MCP      *MCPConfig      `yaml:"mcp"`      // MCP配置
}

// ServerConfig 服务器配置结构体，定义主机、端口及超时时间。
// Host：主机地址。
// Port：端口号。
// ReadTimeout：读取超时时间。
// WriteTimeout：写入超时时间。
type ServerConfig struct {
	Host         string        `yaml:"host"`          // 主机地址
	Port         int           `yaml:"port"`          // 端口号
	ReadTimeout  time.Duration `yaml:"read_timeout"`  // 读取超时
	WriteTimeout time.Duration `yaml:"write_timeout"` // 写入超时
}

// DatabaseConfig 数据库配置结构体，定义数据库连接参数。
// Driver：数据库驱动。
// Host：主机地址。
// Port：端口号。
// Username：用户名。
// Password：密码。
// Database：数据库名。
// MaxOpenConns：最大连接数。
// MaxIdleConns：最大空闲连接数。
// ConnMaxLifetime：连接最大生命周期。
type DatabaseConfig struct {
	Driver          string        `yaml:"driver"`            // 驱动
	Host            string        `yaml:"host"`              // 主机
	Port            int           `yaml:"port"`              // 端口
	Username        string        `yaml:"username"`          // 用户名
	Password        string        `yaml:"password"`          // 密码
	Database        string        `yaml:"database"`          // 数据库名
	MaxOpenConns    int           `yaml:"max_open_conns"`    // 最大连接数
	MaxIdleConns    int           `yaml:"max_idle_conns"`    // 最大空闲连接数
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"` // 最大生命周期
}

// CacheConfig 缓存配置结构体，定义缓存类型及参数。
// Type：缓存类型。
// Host：主机地址。
// Port：端口号。
// Password：访问密码。
// Database：数据库编号。
// SchemaTTL：结构缓存有效期。
// QueryResultTTL：查询结果缓存有效期。
// MaxCacheSize：最大缓存空间。
type CacheConfig struct {
	Type           string        `yaml:"type"`             // 缓存类型
	Host           string        `yaml:"host"`             // 主机
	Port           int           `yaml:"port"`             // 端口
	Password       string        `yaml:"password"`         // 密码
	Database       int           `yaml:"database"`         // 数据库编号
	SchemaTTL      time.Duration `yaml:"schema_ttl"`       // 结构缓存有效期
	QueryResultTTL time.Duration `yaml:"query_result_ttl"` // 查询结果缓存有效期
	MaxCacheSize   string        `yaml:"max_cache_size"`   // 最大缓存空间
}

// LogConfig 日志配置结构体，定义日志级别、格式及输出方式。
// Level：日志级别。
// Format：日志格式。
// Output：输出方式。
// FilePath：日志文件路径。
// MaxSize：单文件最大大小。
// MaxBackups：最大备份数。
// MaxAge：最大保存天数。
type LogConfig struct {
	Level      string `yaml:"level"`       // 日志级别
	Format     string `yaml:"format"`      // 日志格式
	Output     string `yaml:"output"`      // 输出方式
	FilePath   string `yaml:"file_path"`   // 文件路径
	MaxSize    int    `yaml:"max_size"`    // 单文件最大大小
	MaxBackups int    `yaml:"max_backups"` // 最大备份数
	MaxAge     int    `yaml:"max_age"`     // 最大保存天数
}

// SecurityConfig 安全配置结构体，定义JWT密钥、令牌有效期、RBAC及跨域。
// JWTSecret：JWT密钥。
// TokenExpiry：令牌有效期。
// EnableRBAC：是否启用RBAC。
// AllowedOrigins：允许跨域来源。
type SecurityConfig struct {
	JWTSecret      string        `yaml:"jwt_secret"`      // JWT密钥
	TokenExpiry    time.Duration `yaml:"token_expiry"`    // 令牌有效期
	EnableRBAC     bool          `yaml:"enable_rbac"`     // 启用RBAC
	AllowedOrigins []string      `yaml:"allowed_origins"` // 允许跨域
}

// MCPConfig MCP配置结构体，定义MCP服务参数。
// Host：主机地址。
// Port：端口号。
// Timeout：连接超时时间。
// MaxConnections：最大连接数。
type MCPConfig struct {
	Host           string        `yaml:"host"`            // 主机地址
	Port           int           `yaml:"port"`            // 端口号
	Timeout        time.Duration `yaml:"timeout"`         // 连接超时
	MaxConnections int           `yaml:"max_connections"` // 最大连接数
}
