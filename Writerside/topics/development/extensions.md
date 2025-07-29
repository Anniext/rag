# 扩展和定制开发指南

本文档介绍如何扩展和定制 RAG MySQL 查询系统，包括插件开发、自定义组件实现和系统集成。

## 扩展架构概述

系统采用插件化架构设计，支持多种扩展方式：

1. **插件系统** - 通过插件接口扩展功能
2. **自定义组件** - 实现标准接口替换默认组件
3. **中间件扩展** - 在请求处理流程中插入自定义逻辑
4. **工具链扩展** - 为 LangChain 添加自定义工具
5. **数据源扩展** - 支持新的数据库类型

## 1. 插件系统

### 插件接口定义

```go
// 插件基础接口
type Plugin interface {
    Name() string
    Version() string
    Description() string
    Initialize(ctx context.Context, config map[string]any) error
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    Health() PluginHealth
}

// 插件管理器
type PluginManager interface {
    LoadPlugin(path string) (Plugin, error)
    RegisterPlugin(plugin Plugin) error
    UnregisterPlugin(name string) error
    GetPlugin(name string) (Plugin, error)
    ListPlugins() []PluginInfo
    EnablePlugin(name string) error
    DisablePlugin(name string) error
}

// 插件信息
type PluginInfo struct {
    Name        string            `json:"name"`
    Version     string            `json:"version"`
    Description string            `json:"description"`
    Author      string            `json:"author"`
    License     string            `json:"license"`
    Enabled     bool              `json:"enabled"`
    Config      map[string]any    `json:"config"`
    Dependencies []string         `json:"dependencies"`
}
```

### 插件开发示例

#### 1. 查询预处理插件

```go
// 查询预处理插件接口
type QueryPreprocessorPlugin interface {
    Plugin
    PreprocessQuery(ctx context.Context, query string, user *User) (string, error)
}

// 示例：查询清理插件
type QueryCleanerPlugin struct {
    name        string
    version     string
    description string
    config      *CleanerConfig
    patterns    []*regexp.Regexp
}

func (p *QueryCleanerPlugin) Name() string {
    return p.name
}

func (p *QueryCleanerPlugin) Version() string {
    return p.version
}

func (p *QueryCleanerPlugin) Description() string {
    return p.description
}

func (p *QueryCleanerPlugin) Initialize(ctx context.Context, config map[string]any) error {
    // 解析配置
    p.config = &CleanerConfig{}
    if err := mapstructure.Decode(config, p.config); err != nil {
        return fmt.Errorf("failed to decode config: %w", err)
    }

    // 编译正则表达式
    for _, pattern := range p.config.CleanPatterns {
        regex, err := regexp.Compile(pattern)
        if err != nil {
            return fmt.Errorf("invalid regex pattern %s: %w", pattern, err)
        }
        p.patterns = append(p.patterns, regex)
    }

    return nil
}

func (p *QueryCleanerPlugin) PreprocessQuery(ctx context.Context, query string, user *User) (string, error) {
    cleaned := query

    // 应用清理规则
    for _, pattern := range p.patterns {
        cleaned = pattern.ReplaceAllString(cleaned, "")
    }

    // 去除多余空格
    cleaned = strings.TrimSpace(cleaned)
    cleaned = regexp.MustCompile(`\s+`).ReplaceAllString(cleaned, " ")

    return cleaned, nil
}

// 插件配置
type CleanerConfig struct {
    CleanPatterns []string `mapstructure:"clean_patterns"`
    MaxLength     int      `mapstructure:"max_length"`
}
```

#### 2. 结果后处理插件

```go
// 结果后处理插件接口
type ResultPostprocessorPlugin interface {
    Plugin
    PostprocessResult(ctx context.Context, result *QueryResponse, user *User) (*QueryResponse, error)
}

// 示例：数据脱敏插件
type DataMaskingPlugin struct {
    name        string
    version     string
    description string
    config      *MaskingConfig
    rules       []*MaskingRule
}

func (p *DataMaskingPlugin) PostprocessResult(ctx context.Context, result *QueryResponse, user *User) (*QueryResponse, error) {
    if !result.Success || len(result.Data) == 0 {
        return result, nil
    }

    // 检查用户权限
    if p.shouldMaskForUser(user) {
        maskedData := make([]map[string]any, len(result.Data))
        for i, row := range result.Data {
            maskedRow := make(map[string]any)
            for key, value := range row {
                maskedRow[key] = p.maskValue(key, value)
            }
            maskedData[i] = maskedRow
        }
        result.Data = maskedData
    }

    return result, nil
}

func (p *DataMaskingPlugin) maskValue(field string, value any) any {
    for _, rule := range p.rules {
        if rule.MatchField(field) {
            return rule.MaskValue(value)
        }
    }
    return value
}

// 脱敏规则
type MaskingRule struct {
    FieldPattern *regexp.Regexp
    MaskType     MaskType
    MaskChar     string
    KeepPrefix   int
    KeepSuffix   int
}

type MaskType string

const (
    MaskTypeReplace MaskType = "replace"
    MaskTypePartial MaskType = "partial"
    MaskTypeHash    MaskType = "hash"
)
```

### 插件配置

```yaml
# config/plugins.yaml
plugins:
  query_cleaner:
    enabled: true
    path: "./plugins/query_cleaner.so"
    config:
      clean_patterns:
        - "(?i)\\b(drop|delete|truncate)\\b"
        - "--.*$"
      max_length: 1000

  data_masking:
    enabled: true
    path: "./plugins/data_masking.so"
    config:
      rules:
        - field_pattern: "(?i).*(phone|mobile).*"
          mask_type: "partial"
          keep_prefix: 3
          keep_suffix: 4
        - field_pattern: "(?i).*(email).*"
          mask_type: "partial"
          keep_prefix: 2
          keep_suffix: 0
```

## 2. 自定义组件开发

### LLM 提供者扩展

```go
// 自定义 LLM 提供者
type CustomLLMProvider struct {
    name     string
    config   *CustomLLMConfig
    client   *http.Client
    metrics  *LLMMetrics
}

func (p *CustomLLMProvider) GenerateText(ctx context.Context, prompt string, options *GenerateOptions) (*GenerateResult, error) {
    // 构建请求
    request := &CustomLLMRequest{
        Prompt:      prompt,
        MaxTokens:   options.MaxTokens,
        Temperature: options.Temperature,
        Model:       options.Model,
    }

    // 发送请求
    response, err := p.sendRequest(ctx, request)
    if err != nil {
        return nil, err
    }

    // 解析响应
    result := &GenerateResult{
        Text:   response.Text,
        Usage:  response.Usage,
        Model:  response.Model,
        Finish: response.FinishReason,
    }

    // 记录指标
    p.metrics.RecordGeneration(result.Usage.TotalTokens, time.Since(start))

    return result, nil
}

// 注册自定义提供者
func RegisterCustomLLMProvider() {
    provider := &CustomLLMProvider{
        name: "custom-llm",
        config: loadCustomLLMConfig(),
        client: &http.Client{Timeout: 30 * time.Second},
        metrics: NewLLMMetrics("custom-llm"),
    }

    llm.RegisterProvider("custom-llm", provider)
}
```

### 自定义 Schema 加载器

```go
// 自定义 Schema 加载器
type CustomSchemaLoader struct {
    db     *sql.DB
    config *CustomSchemaConfig
    cache  SchemaCache
}

func (l *CustomSchemaLoader) LoadTables(ctx context.Context) ([]*TableInfo, error) {
    query := `
        SELECT
            table_name,
            table_comment,
            engine,
            table_collation,
            table_rows,
            data_length,
            index_length,
            create_time,
            update_time
        FROM information_schema.tables
        WHERE table_schema = ?
        AND table_type = 'BASE TABLE'
        ORDER BY table_name
    `

    rows, err := l.db.QueryContext(ctx, query, l.config.Database)
    if err != nil {
        return nil, fmt.Errorf("failed to query tables: %w", err)
    }
    defer rows.Close()

    var tables []*TableInfo
    for rows.Next() {
        table := &TableInfo{}
        err := rows.Scan(
            &table.Name,
            &table.Comment,
            &table.Engine,
            &table.Collation,
            &table.RowCount,
            &table.DataLength,
            &table.IndexLength,
            &table.CreatedAt,
            &table.UpdatedAt,
        )
        if err != nil {
            return nil, fmt.Errorf("failed to scan table: %w", err)
        }

        // 加载列信息
        columns, err := l.loadColumns(ctx, table.Name)
        if err != nil {
            return nil, fmt.Errorf("failed to load columns for table %s: %w", table.Name, err)
        }
        table.Columns = columns

        tables = append(tables, table)
    }

    return tables, nil
}

// 注册自定义加载器
func RegisterCustomSchemaLoader(db *sql.DB, config *CustomSchemaConfig) {
    loader := &CustomSchemaLoader{
        db:     db,
        config: config,
        cache:  NewSchemaCache(),
    }

    schema.RegisterLoader("custom", loader)
}
```

## 3. 中间件扩展

### 中间件接口

```go
// 中间件接口
type Middleware interface {
    Name() string
    Process(ctx context.Context, request *MCPRequest, next MiddlewareFunc) (*MCPResponse, error)
}

type MiddlewareFunc func(ctx context.Context, request *MCPRequest) (*MCPResponse, error)

// 中间件链
type MiddlewareChain struct {
    middlewares []Middleware
    handler     MCPHandler
}

func (c *MiddlewareChain) Execute(ctx context.Context, request *MCPRequest) (*MCPResponse, error) {
    return c.executeMiddleware(ctx, request, 0)
}

func (c *MiddlewareChain) executeMiddleware(ctx context.Context, request *MCPRequest, index int) (*MCPResponse, error) {
    if index >= len(c.middlewares) {
        return c.handler.Handle(ctx, request)
    }

    middleware := c.middlewares[index]
    next := func(ctx context.Context, req *MCPRequest) (*MCPResponse, error) {
        return c.executeMiddleware(ctx, req, index+1)
    }

    return middleware.Process(ctx, request, next)
}
```

### 中间件示例

#### 1. 请求限流中间件

```go
type RateLimitMiddleware struct {
    name     string
    limiter  *RateLimiter
    config   *RateLimitConfig
}

func (m *RateLimitMiddleware) Name() string {
    return m.name
}

func (m *RateLimitMiddleware) Process(ctx context.Context, request *MCPRequest, next MiddlewareFunc) (*MCPResponse, error) {
    // 获取用户标识
    userID := getUserID(ctx)
    if userID == "" {
        return nil, errors.New("user not authenticated")
    }

    // 检查限流
    allowed, err := m.limiter.Allow(userID, 1)
    if err != nil {
        return nil, fmt.Errorf("rate limit check failed: %w", err)
    }

    if !allowed {
        return &MCPResponse{
            Success: false,
            Error: &MCPError{
                Code:    429,
                Message: "Rate limit exceeded",
                Data: map[string]any{
                    "retry_after": m.limiter.RetryAfter(userID),
                },
            },
        }, nil
    }

    // 继续处理
    return next(ctx, request)
}

// 限流器实现
type RateLimiter struct {
    redis  redis.Client
    config *RateLimitConfig
}

func (r *RateLimiter) Allow(key string, tokens int) (bool, error) {
    script := `
        local key = KEYS[1]
        local capacity = tonumber(ARGV[1])
        local tokens = tonumber(ARGV[2])
        local interval = tonumber(ARGV[3])
        local now = tonumber(ARGV[4])

        local bucket = redis.call('HMGET', key, 'tokens', 'last_refill')
        local current_tokens = tonumber(bucket[1]) or capacity
        local last_refill = tonumber(bucket[2]) or now

        -- 计算需要补充的令牌
        local elapsed = now - last_refill
        local tokens_to_add = math.floor(elapsed / interval * capacity)
        current_tokens = math.min(capacity, current_tokens + tokens_to_add)

        if current_tokens >= tokens then
            current_tokens = current_tokens - tokens
            redis.call('HMSET', key, 'tokens', current_tokens, 'last_refill', now)
            redis.call('EXPIRE', key, interval * 2)
            return 1
        else
            redis.call('HMSET', key, 'tokens', current_tokens, 'last_refill', now)
            redis.call('EXPIRE', key, interval * 2)
            return 0
        end
    `

    result, err := r.redis.Eval(ctx, script, []string{key},
        r.config.Capacity, tokens, r.config.Interval.Seconds(), time.Now().Unix()).Result()
    if err != nil {
        return false, err
    }

    return result.(int64) == 1, nil
}
```

#### 2. 请求日志中间件

```go
type RequestLogMiddleware struct {
    name   string
    logger log.Logger
    config *LogConfig
}

func (m *RequestLogMiddleware) Process(ctx context.Context, request *MCPRequest, next MiddlewareFunc) (*MCPResponse, error) {
    start := time.Now()

    // 记录请求开始
    requestID := getRequestID(ctx)
    userID := getUserID(ctx)

    m.logger.Info("Request started",
        "request_id", requestID,
        "user_id", userID,
        "method", request.Method,
        "params", m.sanitizeParams(request.Params),
    )

    // 处理请求
    response, err := next(ctx, request)

    duration := time.Since(start)

    // 记录请求结束
    if err != nil {
        m.logger.Error("Request failed",
            "request_id", requestID,
            "user_id", userID,
            "method", request.Method,
            "duration", duration,
            "error", err.Error(),
        )
    } else {
        m.logger.Info("Request completed",
            "request_id", requestID,
            "user_id", userID,
            "method", request.Method,
            "duration", duration,
            "success", response.Success,
        )
    }

    return response, err
}

func (m *RequestLogMiddleware) sanitizeParams(params any) any {
    // 移除敏感信息
    if m.config.SanitizeParams {
        return sanitizeMap(params)
    }
    return params
}
```

## 4. LangChain 工具扩展

### 自定义工具开发

```go
// 数据可视化工具
type DataVisualizationTool struct {
    name        string
    description string
    generator   ChartGenerator
    config      *VisualizationConfig
}

func (t *DataVisualizationTool) Name() string {
    return t.name
}

func (t *DataVisualizationTool) Description() string {
    return t.description
}

func (t *DataVisualizationTool) Execute(ctx context.Context, input *ToolInput) (*ToolOutput, error) {
    // 解析输入参数
    data := input.GetArray("data")
    chartType := input.GetString("chart_type")
    options := input.GetObject("options")

    // 生成图表
    chart, err := t.generator.GenerateChart(ctx, &ChartRequest{
        Data:    data,
        Type:    chartType,
        Options: options,
    })
    if err != nil {
        return nil, fmt.Errorf("failed to generate chart: %w", err)
    }

    return &ToolOutput{
        Result: map[string]any{
            "chart_url":    chart.URL,
            "chart_config": chart.Config,
            "suggestions":  chart.Suggestions,
        },
    }, nil
}

func (t *DataVisualizationTool) GetSchema() *ToolSchema {
    return &ToolSchema{
        Name:        t.name,
        Description: t.description,
        Parameters: map[string]*ParameterSchema{
            "data": {
                Type:        "array",
                Description: "查询结果数据",
                Required:    true,
            },
            "chart_type": {
                Type:        "string",
                Description: "图表类型 (bar, line, pie, scatter)",
                Required:    true,
                Enum:        []string{"bar", "line", "pie", "scatter"},
            },
            "options": {
                Type:        "object",
                Description: "图表选项配置",
                Required:    false,
            },
        },
    }
}

// 注册工具
func RegisterDataVisualizationTool(generator ChartGenerator, config *VisualizationConfig) {
    tool := &DataVisualizationTool{
        name:        "data_visualization",
        description: "Generate charts and visualizations from query results",
        generator:   generator,
        config:      config,
    }

    langchain.RegisterTool(tool)
}
```

### 数据导出工具

```go
type DataExportTool struct {
    name        string
    description string
    exporters   map[string]DataExporter
    storage     ObjectStorage
}

func (t *DataExportTool) Execute(ctx context.Context, input *ToolInput) (*ToolOutput, error) {
    data := input.GetArray("data")
    format := input.GetString("format")
    filename := input.GetString("filename")

    exporter, exists := t.exporters[format]
    if !exists {
        return nil, fmt.Errorf("unsupported export format: %s", format)
    }

    // 导出数据
    exported, err := exporter.Export(ctx, data)
    if err != nil {
        return nil, fmt.Errorf("failed to export data: %w", err)
    }

    // 上传到存储
    url, err := t.storage.Upload(ctx, filename, exported)
    if err != nil {
        return nil, fmt.Errorf("failed to upload file: %w", err)
    }

    return &ToolOutput{
        Result: map[string]any{
            "download_url": url,
            "filename":     filename,
            "format":       format,
            "size":         len(exported),
        },
    }, nil
}

// 数据导出器接口
type DataExporter interface {
    Export(ctx context.Context, data []map[string]any) ([]byte, error)
    GetFormat() string
    GetMimeType() string
}

// CSV 导出器
type CSVExporter struct{}

func (e *CSVExporter) Export(ctx context.Context, data []map[string]any) ([]byte, error) {
    if len(data) == 0 {
        return []byte{}, nil
    }

    var buf bytes.Buffer
    writer := csv.NewWriter(&buf)

    // 写入表头
    var headers []string
    for key := range data[0] {
        headers = append(headers, key)
    }
    sort.Strings(headers) // 保证顺序一致
    writer.Write(headers)

    // 写入数据
    for _, row := range data {
        var values []string
        for _, header := range headers {
            value := fmt.Sprintf("%v", row[header])
            values = append(values, value)
        }
        writer.Write(values)
    }

    writer.Flush()
    return buf.Bytes(), writer.Error()
}
```

## 5. 数据源扩展

### 数据源接口

```go
// 数据源接口
type DataSource interface {
    Name() string
    Type() string
    Connect(ctx context.Context, config map[string]any) error
    Disconnect(ctx context.Context) error
    Query(ctx context.Context, query string) (*QueryResult, error)
    GetSchema(ctx context.Context) (*SchemaInfo, error)
    Ping(ctx context.Context) error
}

// 数据源管理器
type DataSourceManager interface {
    RegisterDataSource(name string, ds DataSource) error
    GetDataSource(name string) (DataSource, error)
    ListDataSources() []string
    RemoveDataSource(name string) error
}
```

### PostgreSQL 数据源示例

```go
type PostgreSQLDataSource struct {
    name   string
    db     *sql.DB
    config *PostgreSQLConfig
    schema *SchemaInfo
}

func (ds *PostgreSQLDataSource) Connect(ctx context.Context, config map[string]any) error {
    // 解析配置
    ds.config = &PostgreSQLConfig{}
    if err := mapstructure.Decode(config, ds.config); err != nil {
        return fmt.Errorf("failed to decode config: %w", err)
    }

    // 构建连接字符串
    dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
        ds.config.Host, ds.config.Port, ds.config.Username,
        ds.config.Password, ds.config.Database, ds.config.SSLMode)

    // 连接数据库
    db, err := sql.Open("postgres", dsn)
    if err != nil {
        return fmt.Errorf("failed to open database: %w", err)
    }

    // 配置连接池
    db.SetMaxOpenConns(ds.config.MaxOpenConns)
    db.SetMaxIdleConns(ds.config.MaxIdleConns)
    db.SetConnMaxLifetime(ds.config.ConnMaxLifetime)

    // 测试连接
    if err := db.PingContext(ctx); err != nil {
        return fmt.Errorf("failed to ping database: %w", err)
    }

    ds.db = db
    return nil
}

func (ds *PostgreSQLDataSource) Query(ctx context.Context, query string) (*QueryResult, error) {
    start := time.Now()

    rows, err := ds.db.QueryContext(ctx, query)
    if err != nil {
        return nil, fmt.Errorf("query failed: %w", err)
    }
    defer rows.Close()

    // 获取列信息
    columns, err := rows.Columns()
    if err != nil {
        return nil, fmt.Errorf("failed to get columns: %w", err)
    }

    columnTypes, err := rows.ColumnTypes()
    if err != nil {
        return nil, fmt.Errorf("failed to get column types: %w", err)
    }

    // 读取数据
    var data []map[string]any
    for rows.Next() {
        values := make([]any, len(columns))
        valuePtrs := make([]any, len(columns))
        for i := range values {
            valuePtrs[i] = &values[i]
        }

        if err := rows.Scan(valuePtrs...); err != nil {
            return nil, fmt.Errorf("failed to scan row: %w", err)
        }

        row := make(map[string]any)
        for i, col := range columns {
            row[col] = values[i]
        }
        data = append(data, row)
    }

    if err := rows.Err(); err != nil {
        return nil, fmt.Errorf("rows iteration error: %w", err)
    }

    return &QueryResult{
        Data:          data,
        Columns:       columns,
        ColumnTypes:   columnTypes,
        RowCount:      len(data),
        ExecutionTime: time.Since(start),
    }, nil
}

func (ds *PostgreSQLDataSource) GetSchema(ctx context.Context) (*SchemaInfo, error) {
    if ds.schema != nil {
        return ds.schema, nil
    }

    // 加载表信息
    tables, err := ds.loadTables(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to load tables: %w", err)
    }

    ds.schema = &SchemaInfo{
        Database:  ds.config.Database,
        Tables:    tables,
        Version:   "postgresql",
        UpdatedAt: time.Now(),
    }

    return ds.schema, nil
}

// 注册 PostgreSQL 数据源
func RegisterPostgreSQLDataSource() {
    ds := &PostgreSQLDataSource{
        name: "postgresql",
    }

    datasource.RegisterDataSource("postgresql", ds)
}
```

## 6. 配置和部署

### 扩展配置

```yaml
# config/extensions.yaml
extensions:
  plugins:
    enabled: true
    directory: "./plugins"
    auto_load: true

  middleware:
    enabled: true
    chain:
      - name: "request_id"
        enabled: true
      - name: "rate_limit"
        enabled: true
        config:
          capacity: 100
          interval: "1m"
      - name: "auth"
        enabled: true
      - name: "logging"
        enabled: true
        config:
          sanitize_params: true

  tools:
    enabled: true
    auto_register: true
    tools:
      - name: "data_visualization"
        enabled: true
        config:
          chart_service_url: "http://chart-service:8080"
      - name: "data_export"
        enabled: true
        config:
          storage_type: "s3"
          bucket: "exports"

  data_sources:
    enabled: true
    sources:
      - name: "postgres_main"
        type: "postgresql"
        config:
          host: "postgres.example.com"
          port: 5432
          database: "main"
          username: "${POSTGRES_USER}"
          password: "${POSTGRES_PASSWORD}"
```

### 扩展加载器

```go
type ExtensionLoader struct {
    config      *ExtensionConfig
    pluginMgr   PluginManager
    middlewares []Middleware
    tools       []Tool
    dataSources map[string]DataSource
    logger      log.Logger
}

func (l *ExtensionLoader) LoadExtensions(ctx context.Context) error {
    // 加载插件
    if l.config.Plugins.Enabled {
        if err := l.loadPlugins(ctx); err != nil {
            return fmt.Errorf("failed to load plugins: %w", err)
        }
    }

    // 加载中间件
    if l.config.Middleware.Enabled {
        if err := l.loadMiddleware(ctx); err != nil {
            return fmt.Errorf("failed to load middleware: %w", err)
        }
    }

    // 加载工具
    if l.config.Tools.Enabled {
        if err := l.loadTools(ctx); err != nil {
            return fmt.Errorf("failed to load tools: %w", err)
        }
    }

    // 加载数据源
    if l.config.DataSources.Enabled {
        if err := l.loadDataSources(ctx); err != nil {
            return fmt.Errorf("failed to load data sources: %w", err)
        }
    }

    return nil
}
```

## 7. 开发最佳实践

### 1. 接口设计原则

- **单一职责**: 每个接口只负责一个功能
- **依赖倒置**: 依赖抽象而不是具体实现
- **开闭原则**: 对扩展开放，对修改关闭
- **接口隔离**: 客户端不应该依赖它不需要的接口

### 2. 错误处理

```go
// 定义扩展特定的错误类型
type ExtensionError struct {
    Type    string `json:"type"`
    Code    string `json:"code"`
    Message string `json:"message"`
    Cause   error  `json:"-"`
}

func (e *ExtensionError) Error() string {
    if e.Cause != nil {
        return fmt.Sprintf("%s: %s: %v", e.Type, e.Message, e.Cause)
    }
    return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// 错误包装函数
func WrapExtensionError(err error, extensionType, code, message string) error {
    return &ExtensionError{
        Type:    extensionType,
        Code:    code,
        Message: message,
        Cause:   err,
    }
}
```

### 3. 配置验证

```go
// 配置验证接口
type ConfigValidator interface {
    Validate(config map[string]any) error
}

// 配置验证器实现
type extensionConfigValidator struct {
    schema *jsonschema.Schema
}

func (v *extensionConfigValidator) Validate(config map[string]any) error {
    result := v.schema.Validate(config)
    if !result.Valid() {
        var errors []string
        for _, err := range result.Errors() {
            errors = append(errors, err.String())
        }
        return fmt.Errorf("config validation failed: %s", strings.Join(errors, "; "))
    }
    return nil
}
```

### 4. 测试支持

```go
// 扩展测试工具
type ExtensionTestSuite struct {
    extension Extension
    config    map[string]any
    ctx       context.Context
}

func NewExtensionTestSuite(extension Extension, config map[string]any) *ExtensionTestSuite {
    return &ExtensionTestSuite{
        extension: extension,
        config:    config,
        ctx:       context.Background(),
    }
}

func (s *ExtensionTestSuite) TestInitialization() error {
    return s.extension.Initialize(s.ctx, s.config)
}

func (s *ExtensionTestSuite) TestHealthCheck() error {
    health := s.extension.Health()
    if health.Status != HealthStatusHealthy {
        return fmt.Errorf("extension unhealthy: %s", health.Message)
    }
    return nil
}
```

### 5. 性能监控

```go
// 扩展性能监控
type ExtensionMetrics struct {
    initDuration    prometheus.Histogram
    executionCount  prometheus.Counter
    executionDuration prometheus.Histogram
    errorCount      prometheus.Counter
}

func NewExtensionMetrics(extensionName string) *ExtensionMetrics {
    return &ExtensionMetrics{
        initDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
            Name: "extension_init_duration_seconds",
            Help: "Extension initialization duration",
            ConstLabels: prometheus.Labels{"extension": extensionName},
        }),
        executionCount: prometheus.NewCounter(prometheus.CounterOpts{
            Name: "extension_execution_total",
            Help: "Total extension executions",
            ConstLabels: prometheus.Labels{"extension": extensionName},
        }),
        executionDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
            Name: "extension_execution_duration_seconds",
            Help: "Extension execution duration",
            ConstLabels: prometheus.Labels{"extension": extensionName},
        }),
        errorCount: prometheus.NewCounter(prometheus.CounterOpts{
            Name: "extension_errors_total",
            Help: "Total extension errors",
            ConstLabels: prometheus.Labels{"extension": extensionName},
        }),
    }
}
```

通过这些扩展机制，开发者可以灵活地定制和扩展 RAG MySQL 查询系统，满足各种特定的业务需求和技术要求。
