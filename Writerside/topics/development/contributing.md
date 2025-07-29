# 贡献指南

欢迎为 RAG MySQL 查询系统贡献代码！本指南将帮助您了解如何参与项目开发，包括代码贡献、问题报告、功能建议等。

## 贡献方式

### 1. 代码贡献

- 修复 Bug
- 添加新功能
- 改进性能
- 完善文档
- 编写测试

### 2. 问题报告

- 报告 Bug
- 提出功能请求
- 改进建议
- 文档问题

### 3. 社区参与

- 回答问题
- 代码审查
- 技术讨论
- 经验分享

## 开发流程

### 1. 准备工作

#### Fork 项目

1. 访问项目 GitHub 页面
2. 点击 "Fork" 按钮
3. 克隆您的 Fork 到本地

```bash
git clone https://github.com/YOUR_USERNAME/rag-mysql-query.git
cd rag-mysql-query
```

#### 设置开发环境

```bash
# 安装 Go 1.24.2+
go version

# 安装依赖
go mod download

# 安装开发工具
make install-tools

# 运行测试确保环境正常
make test
```

#### 配置 Git

```bash
# 设置上游仓库
git remote add upstream https://github.com/original/rag-mysql-query.git

# 配置用户信息
git config user.name "Your Name"
git config user.email "your.email@example.com"
```

### 2. 开发流程

#### 创建功能分支

```bash
# 同步最新代码
git checkout main
git pull upstream main

# 创建功能分支
git checkout -b feature/your-feature-name
```

#### 分支命名规范

- `feature/功能名称` - 新功能开发
- `bugfix/问题描述` - Bug 修复
- `hotfix/紧急修复` - 紧急修复
- `docs/文档更新` - 文档更新
- `refactor/重构描述` - 代码重构
- `test/测试相关` - 测试相关

#### 开发和测试

```bash
# 开发代码
# ... 编写代码 ...

# 运行测试
make test

# 运行代码检查
make lint

# 运行格式化
make fmt

# 运行完整检查
make check
```

#### 提交代码

```bash
# 添加文件
git add .

# 提交代码（遵循提交规范）
git commit -m "feat: add new query optimization feature"

# 推送到您的 Fork
git push origin feature/your-feature-name
```

### 3. 提交 Pull Request

#### PR 准备清单

- [ ] 代码已经过测试
- [ ] 所有测试通过
- [ ] 代码符合规范
- [ ] 文档已更新
- [ ] 提交信息清晰
- [ ] 没有合并冲突

#### 创建 PR

1. 访问您的 Fork 页面
2. 点击 "New Pull Request"
3. 选择正确的分支
4. 填写 PR 模板
5. 提交 PR

#### PR 模板

```markdown
## 变更描述

简要描述本次变更的内容和目的。

## 变更类型

- [ ] Bug 修复
- [ ] 新功能
- [ ] 性能改进
- [ ] 代码重构
- [ ] 文档更新
- [ ] 测试相关

## 测试

描述如何测试这些变更：

- [ ] 单元测试
- [ ] 集成测试
- [ ] 手动测试

## 检查清单

- [ ] 代码遵循项目规范
- [ ] 自测通过
- [ ] 添加了必要的测试
- [ ] 更新了相关文档
- [ ] 没有引入破坏性变更

## 相关 Issue

关联的 Issue 编号：#123

## 截图（如适用）

如果有 UI 变更，请提供截图。

## 其他说明

其他需要说明的内容。
```

### 4. 代码审查

#### 审查流程

1. 自动化检查（CI/CD）
2. 代码审查（Maintainer）
3. 测试验证
4. 合并到主分支

#### 审查标准

- **功能正确性**: 代码是否实现了预期功能
- **代码质量**: 代码是否清晰、可维护
- **性能影响**: 是否有性能问题
- **安全性**: 是否存在安全隐患
- **测试覆盖**: 测试是否充分
- **文档完整**: 文档是否完整准确

#### 处理反馈

```bash
# 根据反馈修改代码
# ... 修改代码 ...

# 提交修改
git add .
git commit -m "fix: address review comments"
git push origin feature/your-feature-name
```

## 代码规范

### 1. Go 代码规范

#### 命名规范

```go
// 包名：小写，简短，有意义
package query

// 常量：大写字母开头的驼峰命名
const DefaultTimeout = 30 * time.Second

// 变量：驼峰命名
var maxRetryCount = 3

// 函数：大写字母开头（公开），小写字母开头（私有）
func ProcessQuery(ctx context.Context, query string) (*Result, error) {
    return processQueryInternal(ctx, query)
}

func processQueryInternal(ctx context.Context, query string) (*Result, error) {
    // 实现
}

// 结构体：大写字母开头的驼峰命名
type QueryProcessor struct {
    timeout time.Duration
    logger  log.Logger
}

// 接口：通常以 -er 结尾
type QueryExecutor interface {
    Execute(ctx context.Context, sql string) (*Result, error)
}
```

#### 代码组织

```go
package query

import (
    // 标准库
    "context"
    "fmt"
    "time"

    // 第三方库
    "github.com/gin-gonic/gin"
    "go.uber.org/zap"

    // 项目内部包
    "github.com/project/rag/core"
    "github.com/project/rag/schema"
)

// 常量定义
const (
    DefaultTimeout = 30 * time.Second
    MaxRetryCount  = 3
)

// 变量定义
var (
    ErrInvalidQuery = errors.New("invalid query")
    ErrTimeout      = errors.New("query timeout")
)

// 类型定义
type QueryType string

const (
    QueryTypeSelect QueryType = "select"
    QueryTypeInsert QueryType = "insert"
)

// 结构体定义
type QueryProcessor struct {
    // 导出字段
    Timeout time.Duration
    Logger  log.Logger

    // 私有字段
    db     *sql.DB
    cache  Cache
}

// 构造函数
func NewQueryProcessor(db *sql.DB, logger log.Logger) *QueryProcessor {
    return &QueryProcessor{
        Timeout: DefaultTimeout,
        Logger:  logger,
        db:      db,
        cache:   NewCache(),
    }
}

// 方法实现
func (p *QueryProcessor) Process(ctx context.Context, query string) (*Result, error) {
    // 实现
}
```

#### 错误处理

```go
// 定义错误类型
type QueryError struct {
    Type    string `json:"type"`
    Message string `json:"message"`
    Cause   error  `json:"-"`
}

func (e *QueryError) Error() string {
    if e.Cause != nil {
        return fmt.Sprintf("%s: %s: %v", e.Type, e.Message, e.Cause)
    }
    return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// 错误包装
func WrapQueryError(err error, message string) error {
    return &QueryError{
        Type:    "query_error",
        Message: message,
        Cause:   err,
    }
}

// 错误处理示例
func (p *QueryProcessor) executeQuery(ctx context.Context, sql string) (*Result, error) {
    result, err := p.db.QueryContext(ctx, sql)
    if err != nil {
        return nil, WrapQueryError(err, "failed to execute query")
    }
    defer result.Close()

    // 处理结果
    data, err := p.processResult(result)
    if err != nil {
        return nil, WrapQueryError(err, "failed to process result")
    }

    return data, nil
}
```

### 2. 注释规范

#### 包注释

```go
// Package query 提供自然语言查询处理功能。
//
// 该包实现了将自然语言查询转换为 SQL 语句的核心功能，
// 包括查询解析、SQL 生成、结果处理等。
//
// 基本用法：
//
//	processor := query.NewProcessor(db, logger)
//	result, err := processor.Process(ctx, "查找所有用户")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// 支持的查询类型包括：
//   - 简单查询：SELECT 语句
//   - 聚合查询：GROUP BY、COUNT 等
//   - 关联查询：JOIN 操作
//
package query
```

#### 函数注释

```go
// ProcessQuery 处理自然语言查询并返回结果。
//
// 该函数接受自然语言查询字符串，通过 LLM 转换为 SQL 语句，
// 执行查询并返回格式化的结果。
//
// 参数：
//   - ctx: 上下文，用于取消和超时控制
//   - query: 自然语言查询字符串
//   - options: 查询选项，可以为 nil
//
// 返回值：
//   - *QueryResult: 查询结果，包含数据和元信息
//   - error: 错误信息，如果处理成功则为 nil
//
// 示例：
//
//	result, err := processor.ProcessQuery(ctx, "查找年龄大于25的用户", nil)
//	if err != nil {
//	    return err
//	}
//	fmt.Printf("找到 %d 条记录\n", len(result.Data))
//
// 注意：
//   - 查询字符串不能为空
//   - 上下文取消会中断查询执行
//   - 复杂查询可能需要更长的处理时间
func (p *QueryProcessor) ProcessQuery(ctx context.Context, query string, options *QueryOptions) (*QueryResult, error) {
    // 实现
}
```

#### 结构体注释

```go
// QueryProcessor 负责处理自然语言查询。
//
// 它集成了查询解析、SQL 生成、查询执行等功能，
// 提供了完整的查询处理流程。
//
// 字段说明：
//   - Timeout: 查询超时时间，默认 30 秒
//   - Logger: 日志记录器，用于记录处理过程
//   - MaxRetries: 最大重试次数，默认 3 次
//
// 使用前需要调用 Initialize 方法进行初始化。
type QueryProcessor struct {
    // Timeout 设置查询超时时间
    Timeout time.Duration `json:"timeout"`

    // Logger 用于记录处理日志
    Logger log.Logger `json:"-"`

    // MaxRetries 设置最大重试次数
    MaxRetries int `json:"max_retries"`

    // 私有字段
    db        *sql.DB
    llm       LLMProvider
    schema    SchemaManager
    cache     CacheManager
    metrics   *Metrics
}
```

### 3. 测试规范

#### 单元测试

```go
package query

import (
    "context"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
    "github.com/stretchr/testify/require"
)

// TestQueryProcessor_ProcessQuery 测试查询处理功能
func TestQueryProcessor_ProcessQuery(t *testing.T) {
    tests := []struct {
        name        string
        query       string
        options     *QueryOptions
        mockSetup   func(*MockLLMProvider, *MockSchemaManager)
        expected    *QueryResult
        expectedErr string
    }{
        {
            name:  "简单查询成功",
            query: "查找所有用户",
            options: &QueryOptions{
                Limit: 10,
            },
            mockSetup: func(llm *MockLLMProvider, schema *MockSchemaManager) {
                schema.On("GetTableInfo", "users").Return(&TableInfo{
                    Name: "users",
                    Columns: []*Column{
                        {Name: "id", Type: "int"},
                        {Name: "name", Type: "varchar"},
                    },
                }, nil)

                llm.On("GenerateSQL", mock.Anything, mock.Anything).Return(&SQLResult{
                    SQL: "SELECT id, name FROM users LIMIT 10",
                }, nil)
            },
            expected: &QueryResult{
                Success: true,
                Data: []map[string]any{
                    {"id": 1, "name": "张三"},
                    {"id": 2, "name": "李四"},
                },
                SQL: "SELECT id, name FROM users LIMIT 10",
            },
        },
        {
            name:        "空查询字符串",
            query:       "",
            expectedErr: "query cannot be empty",
        },
        {
            name:  "LLM 服务错误",
            query: "查找用户",
            mockSetup: func(llm *MockLLMProvider, schema *MockSchemaManager) {
                llm.On("GenerateSQL", mock.Anything, mock.Anything).Return(nil, errors.New("LLM service unavailable"))
            },
            expectedErr: "failed to generate SQL: LLM service unavailable",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // 设置 Mock
            mockLLM := &MockLLMProvider{}
            mockSchema := &MockSchemaManager{}
            mockDB := &MockDB{}

            if tt.mockSetup != nil {
                tt.mockSetup(mockLLM, mockSchema)
            }

            // 创建处理器
            processor := &QueryProcessor{
                Timeout: 30 * time.Second,
                Logger:  &MockLogger{},
                llm:     mockLLM,
                schema:  mockSchema,
                db:      mockDB,
            }

            // 执行测试
            ctx := context.Background()
            result, err := processor.ProcessQuery(ctx, tt.query, tt.options)

            // 验证结果
            if tt.expectedErr != "" {
                require.Error(t, err)
                assert.Contains(t, err.Error(), tt.expectedErr)
                assert.Nil(t, result)
            } else {
                require.NoError(t, err)
                require.NotNil(t, result)
                assert.Equal(t, tt.expected.Success, result.Success)
                assert.Equal(t, tt.expected.SQL, result.SQL)
                assert.Len(t, result.Data, len(tt.expected.Data))
            }

            // 验证 Mock 调用
            mockLLM.AssertExpectations(t)
            mockSchema.AssertExpectations(t)
        })
    }
}

// 基准测试
func BenchmarkQueryProcessor_ProcessQuery(b *testing.B) {
    processor := setupTestProcessor()
    ctx := context.Background()
    query := "查找所有用户"

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, err := processor.ProcessQuery(ctx, query, nil)
        if err != nil {
            b.Fatal(err)
        }
    }
}

// 表格驱动测试辅助函数
func setupTestProcessor() *QueryProcessor {
    return &QueryProcessor{
        Timeout: 30 * time.Second,
        Logger:  &MockLogger{},
        // ... 其他字段
    }
}
```

#### 集成测试

```go
// integration_test.go
//go:build integration

package query_test

import (
    "context"
    "database/sql"
    "testing"

    _ "github.com/go-sql-driver/mysql"
    "github.com/stretchr/testify/require"
    "github.com/testcontainers/testcontainers-go"
    "github.com/testcontainers/testcontainers-go/modules/mysql"
)

func TestQueryProcessor_Integration(t *testing.T) {
    // 启动测试数据库
    ctx := context.Background()
    mysqlContainer, err := mysql.RunContainer(ctx,
        testcontainers.WithImage("mysql:8.0"),
        mysql.WithDatabase("testdb"),
        mysql.WithUsername("testuser"),
        mysql.WithPassword("testpass"),
    )
    require.NoError(t, err)
    defer mysqlContainer.Terminate(ctx)

    // 获取连接信息
    connectionString, err := mysqlContainer.ConnectionString(ctx)
    require.NoError(t, err)

    // 连接数据库
    db, err := sql.Open("mysql", connectionString)
    require.NoError(t, err)
    defer db.Close()

    // 初始化测试数据
    err = setupTestData(db)
    require.NoError(t, err)

    // 创建处理器
    processor := query.NewProcessor(db, logger)
    err = processor.Initialize(ctx)
    require.NoError(t, err)

    // 执行集成测试
    t.Run("查询用户信息", func(t *testing.T) {
        result, err := processor.ProcessQuery(ctx, "查找所有用户", nil)
        require.NoError(t, err)
        require.True(t, result.Success)
        require.NotEmpty(t, result.Data)
    })

    t.Run("聚合查询", func(t *testing.T) {
        result, err := processor.ProcessQuery(ctx, "统计用户数量", nil)
        require.NoError(t, err)
        require.True(t, result.Success)
        require.Len(t, result.Data, 1)
    })
}

func setupTestData(db *sql.DB) error {
    queries := []string{
        `CREATE TABLE users (
            id INT PRIMARY KEY AUTO_INCREMENT,
            name VARCHAR(100) NOT NULL,
            email VARCHAR(100) UNIQUE,
            age INT,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        )`,
        `INSERT INTO users (name, email, age) VALUES
            ('张三', 'zhangsan@example.com', 25),
            ('李四', 'lisi@example.com', 30),
            ('王五', 'wangwu@example.com', 28)`,
    }

    for _, query := range queries {
        _, err := db.Exec(query)
        if err != nil {
            return err
        }
    }

    return nil
}
```

### 4. 文档规范

#### README 文档

````markdown
# 组件名称

简要描述组件的功能和用途。

## 功能特性

- 特性 1
- 特性 2
- 特性 3

## 安装使用

### 安装

```bash
go get github.com/project/component
```
````

### 基本用法

```go
package main

import "github.com/project/component"

func main() {
    // 示例代码
}
```

## API 文档

### 类型定义

#### ComponentConfig

配置结构体说明。

#### Component

主要组件说明。

### 方法

#### NewComponent

创建新组件实例。

#### Component.Method

方法说明。

## 配置说明

配置文件格式和选项说明。

## 示例

更多使用示例。

## 贡献

如何贡献代码。

## 许可证

许可证信息。

```

## 提交规范

### 提交信息格式
```

<type>(<scope>): <subject>

<body>

<footer>
```

### 类型说明

- `feat`: 新功能
- `fix`: Bug 修复
- `docs`: 文档更新
- `style`: 代码格式调整
- `refactor`: 代码重构
- `test`: 测试相关
- `chore`: 构建过程或辅助工具的变动
- `perf`: 性能优化
- `ci`: CI/CD 相关

### 示例

```
feat(query): add natural language query processing

- Implement query parser for natural language input
- Add LLM integration for SQL generation
- Support basic SELECT queries with filtering

Closes #123
```

## 发布流程

### 版本号规范

采用语义化版本号 (Semantic Versioning)：

- `MAJOR.MINOR.PATCH`
- `1.0.0` - 主版本号.次版本号.修订号

### 发布步骤

1. 更新版本号
2. 更新 CHANGELOG
3. 创建 Release Tag
4. 发布 Release Notes
5. 更新文档

### 发布检查清单

- [ ] 所有测试通过
- [ ] 文档已更新
- [ ] CHANGELOG 已更新
- [ ] 版本号已更新
- [ ] 无破坏性变更（或已标注）
- [ ] Release Notes 已准备

## 社区规范

### 行为准则

- 尊重他人，友善交流
- 建设性的讨论和反馈
- 包容不同的观点和经验
- 专注于技术问题的解决

### 沟通渠道

- **GitHub Issues**: 问题报告和功能请求
- **GitHub Discussions**: 技术讨论和问答
- **Pull Requests**: 代码审查和讨论

### 响应时间

- Issue 响应：1-3 个工作日
- PR 审查：2-5 个工作日
- 安全问题：24 小时内

## 常见问题

### Q: 如何报告安全漏洞？

A: 请通过私有渠道联系维护者，不要在公开 Issue 中报告安全问题。

### Q: 如何提出新功能建议？

A: 在 GitHub Issues 中创建功能请求，详细描述需求和用例。

### Q: 代码审查需要多长时间？

A: 通常在 2-5 个工作日内完成，复杂的 PR 可能需要更长时间。

### Q: 如何成为项目维护者？

A: 通过持续的高质量贡献和社区参与，可以被邀请成为维护者。

感谢您对项目的贡献！
