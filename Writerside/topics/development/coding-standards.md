# 代码规范

本文档定义了 RAG MySQL 查询系统的代码规范和质量标准，确保代码的一致性、可读性和可维护性。

## 1. Go 语言规范

### 1.1 基础规范

遵循 Go 官方代码规范：

- [Effective Go](https://golang.org/doc/effective_go.html)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- [Go Style Guide](https://google.github.io/styleguide/go/)

### 1.2 命名规范

#### 包名 (Package Names)

```go
// ✅ 好的包名
package query
package schema
package cache
package mcp

// ❌ 避免的包名
package queryProcessor  // 使用驼峰
package query_parser    // 使用下划线
package utils           // 过于通用
package common          // 过于通用
```

#### 变量和函数名

```go
// ✅ 变量命名
var userID int64
var maxRetryCount = 3
var isConnected bool
var hasPermission bool

// ✅ 函数命名
func ProcessQuery(ctx context.Context, query string) (*Result, error)
func validateInput(input string) error
func getUserByID(id int64) (*User, error)

// ❌ 避免的命名
var user_id int64        // 使用下划线
var MaxRetryCount = 3    // 私有变量使用大写
var connected bool       // 布尔变量缺少前缀
func processquery()      // 缺少驼峰
func Process_Query()     // 使用下划线
```

#### 常量命名

```go
// ✅ 常量命名
const (
    DefaultTimeout = 30 * time.Second
    MaxConnections = 1000

    // 枚举类型
    StatusActive   = "active"
    StatusInactive = "inactive"
    StatusPending  = "pending"
)

// ✅ 错误变量
var (
    ErrInvalidQuery = errors.New("invalid query")
    ErrTimeout      = errors.New("operation timeout")
    ErrNotFound     = errors.New("resource not found")
)

// ❌ 避免的命名
const defaultTimeout = 30  // 私有常量使用小写
const MAX_CONNECTIONS = 1000  // 使用下划线
var errInvalidQuery = errors.New("invalid query")  // 错误变量使用小写
```

#### 结构体和接口命名

```go
// ✅ 结构体命名
type QueryProcessor struct {
    timeout time.Duration
    logger  log.Logger
}

type UserProfile struct {
    UserID    int64     `json:"user_id"`
    Name      string    `json:"name"`
    CreatedAt time.Time `json:"created_at"`
}

// ✅ 接口命名
type QueryExecutor interface {
    Execute(ctx context.Context, sql string) (*Result, error)
}

type SchemaLoader interface {
    LoadTables(ctx context.Context) ([]*TableInfo, error)
}

// ❌ 避免的命名
type queryProcessor struct{}  // 公开结构体使用小写
type IQueryExecutor interface{}  // 接口名使用 I 前缀
type QueryExecutorInterface interface{}  // 接口名使用 Interface 后缀
```

### 1.3 代码组织

#### 文件结构

```go
package query

import (
    // 1. 标准库导入
    "context"
    "fmt"
    "time"

    // 2. 第三方库导入
    "github.com/gin-gonic/gin"
    "go.uber.org/zap"

    // 3. 项目内部包导入
    "github.com/project/rag/core"
    "github.com/project/rag/schema"
)

// 4. 常量定义
const (
    DefaultTimeout = 30 * time.Second
    MaxRetryCount  = 3
)

// 5. 变量定义
var (
    ErrInvalidQuery = errors.New("invalid query")
    logger          = zap.NewNop()
)

// 6. 类型定义
type QueryType string

const (
    QueryTypeSelect QueryType = "select"
    QueryTypeInsert QueryType = "insert"
)

// 7. 结构体定义
type QueryProcessor struct {
    timeout time.Duration
    logger  log.Logger
}

// 8. 构造函数
func NewQueryProcessor(logger log.Logger) *QueryProcessor {
    return &QueryProcessor{
        timeout: DefaultTimeout,
        logger:  logger,
    }
}

// 9. 方法实现
func (p *QueryProcessor) Process(ctx context.Context, query string) (*Result, error) {
    // 实现
}

// 10. 私有函数
func validateQuery(query string) error {
    // 实现
}
```

#### 包组织原则

```
rag/
├── cmd/                    # 应用程序入口
│   └── server/
│       └── main.go
├── internal/               # 私有应用程序代码
│   ├── config/            # 配置管理
│   ├── handler/           # HTTP 处理器
│   └── service/           # 业务逻辑
├── pkg/                   # 可被外部使用的库代码
│   ├── query/             # 查询处理
│   ├── schema/            # Schema 管理
│   └── cache/             # 缓存管理
├── api/                   # API 定义
├── docs/                  # 文档
├── scripts/               # 脚本
├── configs/               # 配置文件
└── deployments/           # 部署配置
```

### 1.4 错误处理

#### 错误定义

```go
// ✅ 自定义错误类型
type QueryError struct {
    Type    string `json:"type"`
    Code    string `json:"code"`
    Message string `json:"message"`
    Cause   error  `json:"-"`
}

func (e *QueryError) Error() string {
    if e.Cause != nil {
        return fmt.Sprintf("%s [%s]: %s: %v", e.Type, e.Code, e.Message, e.Cause)
    }
    return fmt.Sprintf("%s [%s]: %s", e.Type, e.Code, e.Message)
}

func (e *QueryError) Unwrap() error {
    return e.Cause
}

// ✅ 错误构造函数
func NewQueryError(code, message string, cause error) *QueryError {
    return &QueryError{
        Type:    "query_error",
        Code:    code,
        Message: message,
        Cause:   cause,
    }
}

// ✅ 错误包装
func WrapQueryError(err error, code, message string) error {
    if err == nil {
        return nil
    }
    return NewQueryError(code, message, err)
}
```

#### 错误处理模式

```go
// ✅ 错误处理最佳实践
func (p *QueryProcessor) ProcessQuery(ctx context.Context, query string) (*Result, error) {
    // 1. 参数验证
    if query == "" {
        return nil, NewQueryError("INVALID_INPUT", "query cannot be empty", nil)
    }

    // 2. 业务逻辑处理
    parsed, err := p.parseQuery(query)
    if err != nil {
        return nil, WrapQueryError(err, "PARSE_FAILED", "failed to parse query")
    }

    sql, err := p.generateSQL(ctx, parsed)
    if err != nil {
        return nil, WrapQueryError(err, "SQL_GENERATION_FAILED", "failed to generate SQL")
    }

    result, err := p.executeSQL(ctx, sql)
    if err != nil {
        return nil, WrapQueryError(err, "EXECUTION_FAILED", "failed to execute SQL")
    }

    return result, nil
}

// ✅ 错误检查
func (p *QueryProcessor) handleError(err error) {
    var queryErr *QueryError
    if errors.As(err, &queryErr) {
        p.logger.Error("Query processing failed",
            "type", queryErr.Type,
            "code", queryErr.Code,
            "message", queryErr.Message,
            "cause", queryErr.Cause,
        )
    } else {
        p.logger.Error("Unexpected error", "error", err)
    }
}

// ❌ 避免的错误处理
func badErrorHandling() error {
    _, err := someOperation()
    if err != nil {
        return errors.New("something went wrong")  // 丢失原始错误信息
    }

    // 忽略错误
    anotherOperation()  // 没有检查错误

    return nil
}
```

### 1.5 并发编程

#### Goroutine 使用

```go
// ✅ 正确的 goroutine 使用
func (p *QueryProcessor) ProcessConcurrent(ctx context.Context, queries []string) ([]*Result, error) {
    results := make([]*Result, len(queries))
    errors := make([]error, len(queries))

    var wg sync.WaitGroup
    sem := make(chan struct{}, 10) // 限制并发数

    for i, query := range queries {
        wg.Add(1)
        go func(index int, q string) {
            defer wg.Done()

            sem <- struct{}{} // 获取信号量
            defer func() { <-sem }() // 释放信号量

            result, err := p.ProcessQuery(ctx, q)
            results[index] = result
            errors[index] = err
        }(i, query)
    }

    wg.Wait()

    // 检查错误
    for _, err := range errors {
        if err != nil {
            return nil, err
        }
    }

    return results, nil
}

// ✅ 使用 context 控制取消
func (p *QueryProcessor) ProcessWithTimeout(ctx context.Context, query string, timeout time.Duration) (*Result, error) {
    ctx, cancel := context.WithTimeout(ctx, timeout)
    defer cancel()

    resultCh := make(chan *Result, 1)
    errorCh := make(chan error, 1)

    go func() {
        result, err := p.ProcessQuery(ctx, query)
        if err != nil {
            errorCh <- err
            return
        }
        resultCh <- result
    }()

    select {
    case result := <-resultCh:
        return result, nil
    case err := <-errorCh:
        return nil, err
    case <-ctx.Done():
        return nil, ctx.Err()
    }
}

// ❌ 避免的并发模式
func badConcurrency() {
    // 没有限制 goroutine 数量
    for i := 0; i < 10000; i++ {
        go func() {
            // 可能导致资源耗尽
        }()
    }

    // 没有等待 goroutine 完成
    go func() {
        // 可能导致程序提前退出
    }()

    // 没有处理 context 取消
    go func() {
        for {
            // 无法取消的循环
        }
    }()
}
```

#### 同步原语使用

```go
// ✅ 正确使用 mutex
type SafeCounter struct {
    mu    sync.RWMutex
    count map[string]int
}

func (c *SafeCounter) Increment(key string) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.count[key]++
}

func (c *SafeCounter) Get(key string) int {
    c.mu.RLock()
    defer c.mu.RUnlock()
    return c.count[key]
}

// ✅ 使用 sync.Once
type DatabaseConnection struct {
    db   *sql.DB
    once sync.Once
    err  error
}

func (d *DatabaseConnection) GetDB() (*sql.DB, error) {
    d.once.Do(func() {
        d.db, d.err = sql.Open("mysql", "connection_string")
    })
    return d.db, d.err
}

// ✅ 使用 channel 进行通信
type WorkerPool struct {
    workers int
    jobs    chan Job
    results chan Result
}

func (p *WorkerPool) Start(ctx context.Context) {
    for i := 0; i < p.workers; i++ {
        go p.worker(ctx)
    }
}

func (p *WorkerPool) worker(ctx context.Context) {
    for {
        select {
        case job := <-p.jobs:
            result := job.Process()
            p.results <- result
        case <-ctx.Done():
            return
        }
    }
}
```

## 2. 代码质量标准

### 2.1 函数设计

#### 函数长度和复杂度

```go
// ✅ 简洁的函数
func (p *QueryProcessor) validateQuery(query string) error {
    if query == "" {
        return ErrEmptyQuery
    }

    if len(query) > MaxQueryLength {
        return ErrQueryTooLong
    }

    if containsSQLInjection(query) {
        return ErrSQLInjection
    }

    return nil
}

// ✅ 单一职责
func (p *QueryProcessor) parseQuery(query string) (*ParsedQuery, error) {
    // 只负责查询解析
    return p.parser.Parse(query)
}

func (p *QueryProcessor) generateSQL(parsed *ParsedQuery) (string, error) {
    // 只负责 SQL 生成
    return p.generator.Generate(parsed)
}

// ❌ 避免过长的函数
func badLongFunction() error {
    // 100+ 行的函数
    // 多个职责混合
    // 复杂的嵌套逻辑
    return nil
}
```

#### 参数设计

```go
// ✅ 使用结构体传递多个参数
type QueryOptions struct {
    Limit     int           `json:"limit"`
    Offset    int           `json:"offset"`
    Timeout   time.Duration `json:"timeout"`
    Format    string        `json:"format"`
    Explain   bool          `json:"explain"`
}

func (p *QueryProcessor) ProcessQuery(ctx context.Context, query string, options *QueryOptions) (*Result, error) {
    if options == nil {
        options = &QueryOptions{
            Limit:   DefaultLimit,
            Timeout: DefaultTimeout,
            Format:  "json",
        }
    }
    // 处理逻辑
}

// ✅ 使用函数选项模式
type ProcessorOption func(*QueryProcessor)

func WithTimeout(timeout time.Duration) ProcessorOption {
    return func(p *QueryProcessor) {
        p.timeout = timeout
    }
}

func WithLogger(logger log.Logger) ProcessorOption {
    return func(p *QueryProcessor) {
        p.logger = logger
    }
}

func NewQueryProcessor(options ...ProcessorOption) *QueryProcessor {
    p := &QueryProcessor{
        timeout: DefaultTimeout,
        logger:  log.NewNopLogger(),
    }

    for _, option := range options {
        option(p)
    }

    return p
}

// ❌ 避免过多参数
func badManyParameters(a, b, c, d, e, f, g, h string) error {
    // 参数过多，难以使用和维护
    return nil
}
```

### 2.2 接口设计

#### 接口定义原则

```go
// ✅ 小而专注的接口
type QueryParser interface {
    Parse(ctx context.Context, query string) (*ParsedQuery, error)
}

type SQLGenerator interface {
    Generate(ctx context.Context, parsed *ParsedQuery) (string, error)
}

type QueryExecutor interface {
    Execute(ctx context.Context, sql string) (*Result, error)
}

// ✅ 组合接口
type QueryProcessor interface {
    QueryParser
    SQLGenerator
    QueryExecutor
}

// ✅ 上下文感知的接口
type SchemaManager interface {
    LoadSchema(ctx context.Context) error
    GetTableInfo(ctx context.Context, tableName string) (*TableInfo, error)
    RefreshSchema(ctx context.Context) error
}

// ❌ 避免过大的接口
type BadLargeInterface interface {
    Parse(string) error
    Generate(string) string
    Execute(string) error
    Validate(string) bool
    Cache(string, interface{})
    Log(string)
    Monitor(string)
    // ... 更多方法
}
```

#### 接口实现

```go
// ✅ 明确的接口实现
type mysqlQueryExecutor struct {
    db     *sql.DB
    logger log.Logger
}

func (e *mysqlQueryExecutor) Execute(ctx context.Context, sql string) (*Result, error) {
    start := time.Now()

    rows, err := e.db.QueryContext(ctx, sql)
    if err != nil {
        e.logger.Error("Query execution failed", "sql", sql, "error", err)
        return nil, WrapQueryError(err, "EXECUTION_FAILED", "failed to execute SQL")
    }
    defer rows.Close()

    result, err := e.processRows(rows)
    if err != nil {
        return nil, WrapQueryError(err, "RESULT_PROCESSING_FAILED", "failed to process query result")
    }

    e.logger.Info("Query executed successfully",
        "sql", sql,
        "duration", time.Since(start),
        "rows", len(result.Data),
    )

    return result, nil
}

// ✅ 接口验证
var _ QueryExecutor = (*mysqlQueryExecutor)(nil)
```

### 2.3 测试标准

#### 测试覆盖率

- 单元测试覆盖率 ≥ 80%
- 核心业务逻辑覆盖率 ≥ 90%
- 关键路径覆盖率 = 100%

#### 测试结构

```go
// ✅ 测试文件组织
package query

import (
    "context"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/stretchr/testify/mock"
)

// 测试数据
var (
    testQuery = "查找所有用户"
    testSQL   = "SELECT * FROM users"
    testResult = &Result{
        Data: []map[string]any{
            {"id": 1, "name": "张三"},
        },
    }
)

// 测试辅助函数
func setupTestProcessor() *QueryProcessor {
    return &QueryProcessor{
        timeout: 30 * time.Second,
        logger:  &MockLogger{},
    }
}

// 表格驱动测试
func TestQueryProcessor_ProcessQuery(t *testing.T) {
    tests := []struct {
        name        string
        query       string
        options     *QueryOptions
        mockSetup   func(*MockLLM, *MockDB)
        expected    *Result
        expectedErr string
    }{
        {
            name:  "成功处理简单查询",
            query: testQuery,
            options: &QueryOptions{Limit: 10},
            mockSetup: func(llm *MockLLM, db *MockDB) {
                llm.On("GenerateSQL", mock.Anything, testQuery).Return(testSQL, nil)
                db.On("Query", mock.Anything, testSQL).Return(testResult, nil)
            },
            expected: testResult,
        },
        {
            name:        "空查询字符串",
            query:       "",
            expectedErr: "query cannot be empty",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // 设置
            processor := setupTestProcessor()
            mockLLM := &MockLLM{}
            mockDB := &MockDB{}

            if tt.mockSetup != nil {
                tt.mockSetup(mockLLM, mockDB)
            }

            processor.llm = mockLLM
            processor.db = mockDB

            // 执行
            ctx := context.Background()
            result, err := processor.ProcessQuery(ctx, tt.query, tt.options)

            // 验证
            if tt.expectedErr != "" {
                require.Error(t, err)
                assert.Contains(t, err.Error(), tt.expectedErr)
            } else {
                require.NoError(t, err)
                assert.Equal(t, tt.expected, result)
            }

            // 验证 Mock
            mockLLM.AssertExpectations(t)
            mockDB.AssertExpectations(t)
        })
    }
}

// 基准测试
func BenchmarkQueryProcessor_ProcessQuery(b *testing.B) {
    processor := setupTestProcessor()
    ctx := context.Background()

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, err := processor.ProcessQuery(ctx, testQuery, nil)
        if err != nil {
            b.Fatal(err)
        }
    }
}

// 示例测试
func ExampleQueryProcessor_ProcessQuery() {
    processor := NewQueryProcessor()
    ctx := context.Background()

    result, err := processor.ProcessQuery(ctx, "查找所有用户", nil)
    if err != nil {
        panic(err)
    }

    fmt.Printf("找到 %d 条记录\n", len(result.Data))
    // Output: 找到 2 条记录
}
```

#### Mock 使用

```go
// ✅ Mock 接口定义
type MockLLMProvider struct {
    mock.Mock
}

func (m *MockLLMProvider) GenerateSQL(ctx context.Context, query string) (string, error) {
    args := m.Called(ctx, query)
    return args.String(0), args.Error(1)
}

func (m *MockLLMProvider) GenerateExplanation(ctx context.Context, sql string, result *Result) (string, error) {
    args := m.Called(ctx, sql, result)
    return args.String(0), args.Error(1)
}

// ✅ 测试中使用 Mock
func TestWithMock(t *testing.T) {
    mockLLM := &MockLLMProvider{}

    // 设置期望
    mockLLM.On("GenerateSQL", mock.Anything, "查找用户").Return("SELECT * FROM users", nil)

    // 使用 Mock
    processor := &QueryProcessor{llm: mockLLM}
    result, err := processor.ProcessQuery(context.Background(), "查找用户", nil)

    // 验证
    require.NoError(t, err)
    assert.NotNil(t, result)

    // 验证 Mock 调用
    mockLLM.AssertExpectations(t)
}
```

## 3. 性能标准

### 3.1 性能要求

#### 响应时间

- 简单查询：< 100ms
- 复杂查询：< 500ms
- Schema 加载：< 1s
- 缓存命中：< 10ms

#### 吞吐量

- 并发查询：≥ 1000 QPS
- 连接数：≥ 10000
- 内存使用：< 1GB (正常负载)

#### 资源使用

```go
// ✅ 资源管理
type ResourceManager struct {
    maxMemory     int64
    maxGoroutines int
    currentMemory int64
    goroutinePool *sync.Pool
}

func (r *ResourceManager) AcquireGoroutine() bool {
    if r.getCurrentGoroutines() >= r.maxGoroutines {
        return false
    }
    return true
}

func (r *ResourceManager) CheckMemory() error {
    var m runtime.MemStats
    runtime.ReadMemStats(&m)

    if int64(m.Alloc) > r.maxMemory {
        return errors.New("memory limit exceeded")
    }

    return nil
}

// ✅ 对象池使用
var queryPool = sync.Pool{
    New: func() interface{} {
        return &Query{}
    },
}

func (p *QueryProcessor) processQuery(query string) error {
    q := queryPool.Get().(*Query)
    defer queryPool.Put(q)

    q.Reset()
    q.Text = query

    // 处理查询
    return nil
}
```

### 3.2 性能优化

#### 缓存策略

```go
// ✅ 多层缓存
type CacheManager struct {
    l1Cache *sync.Map        // 内存缓存
    l2Cache redis.Client     // Redis 缓存
    metrics *CacheMetrics
}

func (c *CacheManager) Get(ctx context.Context, key string) (interface{}, error) {
    // L1 缓存
    if value, ok := c.l1Cache.Load(key); ok {
        c.metrics.L1Hits.Inc()
        return value, nil
    }

    // L2 缓存
    value, err := c.l2Cache.Get(ctx, key).Result()
    if err == nil {
        c.metrics.L2Hits.Inc()
        c.l1Cache.Store(key, value)
        return value, nil
    }

    c.metrics.Misses.Inc()
    return nil, ErrCacheMiss
}

// ✅ 批量操作
func (p *QueryProcessor) ProcessBatch(ctx context.Context, queries []string) ([]*Result, error) {
    // 批量预处理
    parsed := make([]*ParsedQuery, len(queries))
    for i, query := range queries {
        p, err := p.parseQuery(query)
        if err != nil {
            return nil, err
        }
        parsed[i] = p
    }

    // 批量生成 SQL
    sqls, err := p.generateSQLBatch(ctx, parsed)
    if err != nil {
        return nil, err
    }

    // 批量执行
    return p.executeBatch(ctx, sqls)
}
```

#### 内存优化

```go
// ✅ 流式处理
func (p *QueryProcessor) ProcessLargeResult(ctx context.Context, sql string) (<-chan *Row, error) {
    rows, err := p.db.QueryContext(ctx, sql)
    if err != nil {
        return nil, err
    }

    rowCh := make(chan *Row, 100) // 缓冲通道

    go func() {
        defer close(rowCh)
        defer rows.Close()

        for rows.Next() {
            row := &Row{}
            if err := rows.Scan(&row.Data); err != nil {
                p.logger.Error("Failed to scan row", "error", err)
                return
            }

            select {
            case rowCh <- row:
            case <-ctx.Done():
                return
            }
        }
    }()

    return rowCh, nil
}

// ✅ 内存复用
type ResultBuilder struct {
    buffer []map[string]interface{}
    pool   *sync.Pool
}

func (b *ResultBuilder) Build(rows *sql.Rows) (*Result, error) {
    // 复用 buffer
    b.buffer = b.buffer[:0]

    for rows.Next() {
        row := b.pool.Get().(map[string]interface{})
        defer b.pool.Put(row)

        // 清空 map
        for k := range row {
            delete(row, k)
        }

        // 扫描数据
        if err := rows.Scan(row); err != nil {
            return nil, err
        }

        // 复制数据
        rowCopy := make(map[string]interface{}, len(row))
        for k, v := range row {
            rowCopy[k] = v
        }

        b.buffer = append(b.buffer, rowCopy)
    }

    return &Result{Data: b.buffer}, nil
}
```

## 4. 安全标准

### 4.1 输入验证

```go
// ✅ 输入验证
func (p *QueryProcessor) validateInput(query string) error {
    // 长度检查
    if len(query) > MaxQueryLength {
        return NewValidationError("QUERY_TOO_LONG", "query exceeds maximum length")
    }

    // 字符检查
    if !isValidUTF8(query) {
        return NewValidationError("INVALID_ENCODING", "query contains invalid characters")
    }

    // SQL 注入检查
    if containsSQLInjection(query) {
        return NewValidationError("SQL_INJECTION", "query contains potential SQL injection")
    }

    // 敏感操作检查
    if containsDangerousOperations(query) {
        return NewValidationError("DANGEROUS_OPERATION", "query contains dangerous operations")
    }

    return nil
}

func containsSQLInjection(query string) bool {
    dangerousPatterns := []string{
        `(?i)\b(union|select|insert|update|delete|drop|create|alter|exec|execute)\b`,
        `(?i)(\-\-|\#|\/\*|\*\/)`,
        `(?i)(\bor\b|\band\b).*(\=|\<|\>)`,
        `(?i)(\bor\b|\band\b).*(\d+\s*\=\s*\d+)`,
    }

    for _, pattern := range dangerousPatterns {
        if matched, _ := regexp.MatchString(pattern, query); matched {
            return true
        }
    }

    return false
}
```

### 4.2 权限控制

```go
// ✅ 权限检查
func (p *QueryProcessor) checkPermissions(ctx context.Context, user *User, sql string) error {
    // 解析 SQL 获取涉及的表
    tables, err := p.extractTables(sql)
    if err != nil {
        return err
    }

    // 检查表访问权限
    for _, table := range tables {
        if !user.HasTablePermission(table, "read") {
            return NewPermissionError("TABLE_ACCESS_DENIED",
                fmt.Sprintf("user %s has no read permission for table %s", user.ID, table))
        }
    }

    // 检查操作权限
    operation := p.extractOperation(sql)
    if !user.HasOperationPermission(operation) {
        return NewPermissionError("OPERATION_NOT_ALLOWED",
            fmt.Sprintf("user %s is not allowed to perform %s operation", user.ID, operation))
    }

    return nil
}

// ✅ 数据脱敏
func (p *QueryProcessor) maskSensitiveData(result *Result, user *User) *Result {
    if user.HasRole("admin") {
        return result // 管理员不需要脱敏
    }

    maskedResult := &Result{
        Success: result.Success,
        Data:    make([]map[string]interface{}, len(result.Data)),
    }

    for i, row := range result.Data {
        maskedRow := make(map[string]interface{})
        for key, value := range row {
            if p.isSensitiveField(key) {
                maskedRow[key] = p.maskValue(value)
            } else {
                maskedRow[key] = value
            }
        }
        maskedResult.Data[i] = maskedRow
    }

    return maskedResult
}
```

### 4.3 日志和审计

```go
// ✅ 安全日志
func (p *QueryProcessor) auditLog(ctx context.Context, event *AuditEvent) {
    logEntry := map[string]interface{}{
        "timestamp":  time.Now().UTC(),
        "user_id":    event.UserID,
        "session_id": event.SessionID,
        "action":     event.Action,
        "resource":   event.Resource,
        "result":     event.Result,
        "ip_address": event.IPAddress,
        "user_agent": event.UserAgent,
    }

    // 记录到安全日志
    p.securityLogger.Info("Security audit", logEntry)

    // 发送到 SIEM 系统
    if p.siemClient != nil {
        p.siemClient.SendEvent(ctx, logEntry)
    }
}

// ✅ 敏感信息过滤
func (p *QueryProcessor) sanitizeLogData(data interface{}) interface{} {
    switch v := data.(type) {
    case string:
        return p.maskSensitiveString(v)
    case map[string]interface{}:
        sanitized := make(map[string]interface{})
        for key, value := range v {
            if p.isSensitiveField(key) {
                sanitized[key] = "[MASKED]"
            } else {
                sanitized[key] = p.sanitizeLogData(value)
            }
        }
        return sanitized
    case []interface{}:
        sanitized := make([]interface{}, len(v))
        for i, item := range v {
            sanitized[i] = p.sanitizeLogData(item)
        }
        return sanitized
    default:
        return v
    }
}
```

## 5. 工具和检查

### 5.1 代码检查工具

#### Makefile 配置

```makefile
# 代码质量检查
.PHONY: lint
lint:
	golangci-lint run ./...

.PHONY: fmt
fmt:
	gofmt -s -w .
	goimports -w .

.PHONY: vet
vet:
	go vet ./...

.PHONY: test
test:
	go test -v -race -coverprofile=coverage.out ./...

.PHONY: test-coverage
test-coverage: test
	go tool cover -html=coverage.out -o coverage.html
	go tool cover -func=coverage.out

.PHONY: security
security:
	gosec ./...

.PHONY: check
check: fmt vet lint security test

.PHONY: install-tools
install-tools:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install golang.org/x/tools/cmd/goimports@latest
	go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest
```

#### golangci-lint 配置

```yaml
# .golangci.yml
run:
  timeout: 5m
  modules-download-mode: readonly

linters-settings:
  gocyclo:
    min-complexity: 10

  goconst:
    min-len: 3
    min-occurrences: 3

  goimports:
    local-prefixes: github.com/project/rag

  govet:
    check-shadowing: true

  misspell:
    locale: US

  unused:
    check-exported: false

linters:
  enable:
    - gocyclo
    - goconst
    - goimports
    - govet
    - ineffassign
    - misspell
    - unused
    - deadcode
    - varcheck
    - structcheck
    - errcheck
    - gosimple
    - staticcheck
    - typecheck

issues:
  exclude-rules:
    - path: _test\.go
      linters:
        - gocyclo
        - errcheck
        - dupl
        - gosec
```

### 5.2 CI/CD 集成

#### GitHub Actions 配置

```yaml
# .github/workflows/ci.yml
name: CI

on:
  push:
    branches: [main, develop]
  pull_request:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest

    steps:
      - uses: actions/checkout@v3

      - name: Set up Go
        uses: actions/setup-go@v3
        with:
          go-version: 1.24.2

      - name: Cache Go modules
        uses: actions/cache@v3
        with:
          path: ~/go/pkg/mod
          key: ${{ runner.os }}-go-${{ hashFiles('**/go.sum') }}
          restore-keys: |
            ${{ runner.os }}-go-

      - name: Install dependencies
        run: go mod download

      - name: Run tests
        run: make test

      - name: Run linter
        uses: golangci/golangci-lint-action@v3
        with:
          version: latest

      - name: Run security scan
        run: make security

      - name: Upload coverage
        uses: codecov/codecov-action@v3
        with:
          file: ./coverage.out
```

### 5.3 代码质量指标

#### 质量门禁

- 测试覆盖率 ≥ 80%
- 代码重复率 ≤ 3%
- 圈复杂度 ≤ 10
- 技术债务比率 ≤ 5%
- 安全漏洞数量 = 0

#### 监控指标

```go
// 代码质量指标收集
type QualityMetrics struct {
    TestCoverage     float64 `json:"test_coverage"`
    CodeDuplication  float64 `json:"code_duplication"`
    CyclomaticComplexity int `json:"cyclomatic_complexity"`
    TechnicalDebt    float64 `json:"technical_debt"`
    SecurityIssues   int     `json:"security_issues"`
    LinesOfCode      int     `json:"lines_of_code"`
    Maintainability  string  `json:"maintainability"`
}

func CollectQualityMetrics() *QualityMetrics {
    return &QualityMetrics{
        TestCoverage:         getCoverageFromReport(),
        CodeDuplication:      getDuplicationFromReport(),
        CyclomaticComplexity: getComplexityFromReport(),
        TechnicalDebt:        getTechnicalDebtFromReport(),
        SecurityIssues:       getSecurityIssuesFromReport(),
        LinesOfCode:          countLinesOfCode(),
        Maintainability:      calculateMaintainability(),
    }
}
```

通过遵循这些代码规范和质量标准，我们可以确保 RAG MySQL 查询系统的代码质量、可维护性和安全性。
