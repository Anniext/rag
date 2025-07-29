# 配置和部署最佳实践指南

本文档提供 RAG MySQL 查询系统的配置管理和部署最佳实践，涵盖不同环境的配置策略和部署方案。

## 配置管理

### 1. 配置文件结构

#### 开发环境配置 (rag.development.yaml)

```yaml
# 开发环境配置
rag:
  # MCP 服务器配置
  mcp:
    server:
      host: "0.0.0.0"
      port: 8080
      timeout: "30s"
      max_connections: 100
      cors:
        enabled: true
        origins: ["http://localhost:3000", "http://localhost:8080"]
        methods: ["GET", "POST", "PUT", "DELETE", "OPTIONS"]
        headers: ["Content-Type", "Authorization"]

  # 数据库配置
  database:
    host: "localhost"
    port: 3306
    username: "rag_dev"
    password: "dev_password"
    database: "rag_development"
    charset: "utf8mb4"
    max_open_conns: 10
    max_idle_conns: 5
    conn_max_lifetime: "1h"
    conn_max_idle_time: "30m"

    # 开发环境启用详细日志
    log_level: "debug"
    slow_query_threshold: "100ms"

  # LLM 配置
  llm:
    provider: "openai"
    model: "gpt-4"
    api_key: "${OPENAI_API_KEY}"
    base_url: "${OPENAI_BASE_URL:-https://api.openai.com/v1}"
    temperature: 0.1
    max_tokens: 2048
    timeout: "30s"

    # 开发环境使用较低的限流
    rate_limit:
      requests_per_minute: 60
      tokens_per_minute: 10000

  # LangChain 配置
  langchain:
    memory:
      type: "memory" # 开发环境使用内存存储
      ttl: "30m"
      max_size: 1000

    tools:
      - name: "sql_generator"
        enabled: true
        config:
          max_complexity: 5
      - name: "query_optimizer"
        enabled: true
      - name: "schema_analyzer"
        enabled: true

  # 缓存配置
  cache:
    type: "memory" # 开发环境使用内存缓存
    schema_ttl: "10m" # 开发环境较短的缓存时间
    query_result_ttl: "5m"
    max_cache_size: "50MB"

  # 日志配置
  logging:
    level: "debug"
    format: "text" # 开发环境使用文本格式便于阅读
    output: "stdout"
    file:
      enabled: false

  # 监控配置
  monitoring:
    metrics:
      enabled: true
      port: 9090
      path: "/metrics"

    health:
      enabled: true
      port: 8081
      path: "/health"

    # 开发环境启用性能分析
    pprof:
      enabled: true
      port: 6060

  # 安全配置
  security:
    jwt:
      secret: "dev-secret-key-change-in-production"
      expiry: "24h"

    # 开发环境较宽松的安全设置
    sql_guard:
      enabled: true
      max_query_length: 10000
      allowed_operations: ["SELECT", "INSERT", "UPDATE", "DELETE"]
      blocked_keywords: ["DROP", "TRUNCATE", "ALTER"]

    rate_limiting:
      enabled: false # 开发环境禁用限流
```

#### 生产环境配置 (rag.production.yaml)

```yaml
# 生产环境配置
rag:
  # MCP 服务器配置
  mcp:
    server:
      host: "0.0.0.0"
      port: 8080
      timeout: "60s"
      max_connections: 1000
      read_buffer_size: 4096
      write_buffer_size: 4096
      cors:
        enabled: true
        origins: ["https://yourdomain.com"]
        methods: ["GET", "POST"]
        headers: ["Content-Type", "Authorization"]

  # 数据库配置
  database:
    host: "${DB_HOST}"
    port: 3306
    username: "${DB_USERNAME}"
    password: "${DB_PASSWORD}"
    database: "${DB_DATABASE}"
    charset: "utf8mb4"
    max_open_conns: 100
    max_idle_conns: 20
    conn_max_lifetime: "1h"
    conn_max_idle_time: "10m"

    # 生产环境日志配置
    log_level: "warn"
    slow_query_threshold: "1s"

    # 连接池监控
    pool_monitoring:
      enabled: true
      interval: "30s"

  # LLM 配置
  llm:
    provider: "openai"
    model: "gpt-4"
    api_key: "${OPENAI_API_KEY}"
    base_url: "${OPENAI_BASE_URL}"
    temperature: 0.1
    max_tokens: 2048
    timeout: "60s"

    # 生产环境严格限流
    rate_limit:
      requests_per_minute: 1000
      tokens_per_minute: 100000
      burst_size: 10

    # 重试配置
    retry:
      max_attempts: 3
      base_delay: "1s"
      max_delay: "30s"

  # LangChain 配置
  langchain:
    memory:
      type: "redis"
      redis_url: "${REDIS_URL}"
      ttl: "1h"
      max_size: 10000

      # Redis 连接池
      pool:
        max_active: 100
        max_idle: 20
        idle_timeout: "5m"

    tools:
      - name: "sql_generator"
        enabled: true
        config:
          max_complexity: 10
          cache_enabled: true
      - name: "query_optimizer"
        enabled: true
        config:
          cache_enabled: true
      - name: "schema_analyzer"
        enabled: true

  # 缓存配置
  cache:
    type: "redis"
    redis_url: "${REDIS_URL}"
    schema_ttl: "1h"
    query_result_ttl: "10m"
    max_cache_size: "1GB"

    # 缓存预热
    preload:
      enabled: true
      schema_tables: ["users", "orders", "products"]

  # 日志配置
  logging:
    level: "info"
    format: "json"
    output: "file"
    file:
      enabled: true
      path: "/var/log/rag/rag.log"
      max_size: "100MB"
      max_backups: 10
      max_age: 30
      compress: true

    # 结构化日志字段
    fields:
      service: "rag-mysql-query"
      version: "${APP_VERSION}"
      environment: "production"

  # 监控配置
  monitoring:
    metrics:
      enabled: true
      port: 9090
      path: "/metrics"

      # Prometheus 配置
      prometheus:
        namespace: "rag"
        subsystem: "mysql_query"

    health:
      enabled: true
      port: 8081
      path: "/health"

      # 健康检查配置
      checks:
        database: true
        redis: true
        llm: true

    # 分布式追踪
    tracing:
      enabled: true
      jaeger_endpoint: "${JAEGER_ENDPOINT}"
      sample_rate: 0.1

  # 安全配置
  security:
    jwt:
      secret: "${JWT_SECRET}"
      expiry: "1h"
      refresh_expiry: "24h"

    # 生产环境严格的安全设置
    sql_guard:
      enabled: true
      max_query_length: 5000
      allowed_operations: ["SELECT"] # 生产环境只允许查询
      blocked_keywords:
        ["DROP", "TRUNCATE", "ALTER", "DELETE", "UPDATE", "INSERT"]

      # SQL 注入检测
      injection_detection:
        enabled: true
        patterns: ["union", "or 1=1", "'; drop", "script>"]

    rate_limiting:
      enabled: true
      global_limit: 1000 # 每秒全局请求限制
      per_user_limit: 10 # 每用户每秒请求限制
      per_ip_limit: 100 # 每IP每秒请求限制

    # TLS 配置
    tls:
      enabled: true
      cert_file: "/etc/ssl/certs/rag.crt"
      key_file: "/etc/ssl/private/rag.key"
      min_version: "1.2"

  # 性能配置
  performance:
    # 连接池配置
    connection_pool:
      size: 50
      timeout: "30s"
      idle_timeout: "5m"

    # 查询优化
    query_optimization:
      enabled: true
      max_execution_time: "30s"
      result_size_limit: 10000

      # 查询缓存
      query_cache:
        enabled: true
        size: "500MB"
        ttl: "15m"

    # 内存管理
    memory:
      max_heap_size: "2GB"
      gc_target_percentage: 80
```

#### 测试环境配置 (rag.testing.yaml)

```yaml
# 测试环境配置
rag:
  mcp:
    server:
      host: "0.0.0.0"
      port: 8080
      timeout: "10s"
      max_connections: 50

  database:
    host: "localhost"
    port: 3306
    username: "rag_test"
    password: "test_password"
    database: "rag_test"
    max_open_conns: 5
    max_idle_conns: 2
    conn_max_lifetime: "30m"

  llm:
    provider: "mock" # 测试环境使用模拟 LLM
    model: "mock-gpt-4"
    temperature: 0.0
    max_tokens: 1000
    timeout: "5s"

  langchain:
    memory:
      type: "memory"
      ttl: "5m"
      max_size: 100

  cache:
    type: "memory"
    schema_ttl: "1m"
    query_result_ttl: "30s"
    max_cache_size: "10MB"

  logging:
    level: "debug"
    format: "text"
    output: "stdout"

  security:
    jwt:
      secret: "test-secret-key"
      expiry: "1h"

    sql_guard:
      enabled: true
      max_query_length: 1000
      allowed_operations: ["SELECT"]

    rate_limiting:
      enabled: false
```

### 2. 环境变量管理

#### 开发环境 (.env.development)

```bash
# 应用配置
APP_ENV=development
APP_VERSION=1.0.0-dev
APP_DEBUG=true

# 数据库配置
DB_HOST=localhost
DB_PORT=3306
DB_USERNAME=rag_dev
DB_PASSWORD=dev_password
DB_DATABASE=rag_development

# LLM 配置
OPENAI_API_KEY=sk-your-development-key
OPENAI_BASE_URL=https://api.openai.com/v1

# Redis 配置
REDIS_URL=redis://localhost:6379/0

# JWT 配置
JWT_SECRET=dev-jwt-secret-change-in-production

# 日志配置
LOG_LEVEL=debug
LOG_FORMAT=text

# 监控配置
METRICS_ENABLED=true
HEALTH_CHECK_ENABLED=true
PPROF_ENABLED=true
```

#### 生产环境 (.env.production)

```bash
# 应用配置
APP_ENV=production
APP_VERSION=1.0.0
APP_DEBUG=false

# 数据库配置
DB_HOST=prod-mysql-host
DB_PORT=3306
DB_USERNAME=rag_prod_user
DB_PASSWORD=secure-production-password
DB_DATABASE=rag_production

# LLM 配置
OPENAI_API_KEY=sk-your-production-key
OPENAI_BASE_URL=https://api.openai.com/v1

# Redis 配置
REDIS_URL=redis://prod-redis-host:6379/0

# JWT 配置
JWT_SECRET=super-secure-jwt-secret-for-production

# 日志配置
LOG_LEVEL=info
LOG_FORMAT=json

# 监控配置
METRICS_ENABLED=true
HEALTH_CHECK_ENABLED=true
PPROF_ENABLED=false

# 追踪配置
JAEGER_ENDPOINT=http://jaeger-collector:14268/api/traces

# TLS 配置
TLS_ENABLED=true
TLS_CERT_FILE=/etc/ssl/certs/rag.crt
TLS_KEY_FILE=/etc/ssl/private/rag.key
```

### 3. 配置验证

#### 配置验证器实现

```go
package config

import (
    "fmt"
    "net/url"
    "time"
)

// Validator 配置验证器
type Validator struct {
    errors []string
}

// NewValidator 创建配置验证器
func NewValidator() *Validator {
    return &Validator{
        errors: make([]string, 0),
    }
}

// ValidateConfig 验证配置
func (v *Validator) ValidateConfig(cfg *Config) error {
    v.validateMCPConfig(&cfg.MCP)
    v.validateDatabaseConfig(&cfg.Database)
    v.validateLLMConfig(&cfg.LLM)
    v.validateCacheConfig(&cfg.Cache)
    v.validateSecurityConfig(&cfg.Security)

    if len(v.errors) > 0 {
        return fmt.Errorf("配置验证失败: %v", v.errors)
    }

    return nil
}

// validateMCPConfig 验证 MCP 配置
func (v *Validator) validateMCPConfig(cfg *MCPConfig) {
    if cfg.Server.Port <= 0 || cfg.Server.Port > 65535 {
        v.addError("MCP 服务器端口必须在 1-65535 范围内")
    }

    if cfg.Server.MaxConnections <= 0 {
        v.addError("MCP 最大连接数必须大于 0")
    }

    if cfg.Server.Timeout <= 0 {
        v.addError("MCP 超时时间必须大于 0")
    }
}

// validateDatabaseConfig 验证数据库配置
func (v *Validator) validateDatabaseConfig(cfg *DatabaseConfig) {
    if cfg.Host == "" {
        v.addError("数据库主机不能为空")
    }

    if cfg.Port <= 0 || cfg.Port > 65535 {
        v.addError("数据库端口必须在 1-65535 范围内")
    }

    if cfg.Username == "" {
        v.addError("数据库用户名不能为空")
    }

    if cfg.Database == "" {
        v.addError("数据库名不能为空")
    }

    if cfg.MaxOpenConns <= 0 {
        v.addError("数据库最大连接数必须大于 0")
    }

    if cfg.MaxIdleConns < 0 {
        v.addError("数据库最大空闲连接数不能小于 0")
    }

    if cfg.MaxIdleConns > cfg.MaxOpenConns {
        v.addError("数据库最大空闲连接数不能大于最大连接数")
    }
}

// validateLLMConfig 验证 LLM 配置
func (v *Validator) validateLLMConfig(cfg *LLMConfig) {
    if cfg.Provider == "" {
        v.addError("LLM 提供商不能为空")
    }

    if cfg.Model == "" {
        v.addError("LLM 模型不能为空")
    }

    if cfg.Provider == "openai" && cfg.APIKey == "" {
        v.addError("OpenAI API Key 不能为空")
    }

    if cfg.BaseURL != "" {
        if _, err := url.Parse(cfg.BaseURL); err != nil {
            v.addError("LLM Base URL 格式无效")
        }
    }

    if cfg.Temperature < 0 || cfg.Temperature > 2 {
        v.addError("LLM 温度参数必须在 0-2 范围内")
    }

    if cfg.MaxTokens <= 0 {
        v.addError("LLM 最大 Token 数必须大于 0")
    }
}

// validateCacheConfig 验证缓存配置
func (v *Validator) validateCacheConfig(cfg *CacheConfig) {
    if cfg.Type != "memory" && cfg.Type != "redis" {
        v.addError("缓存类型必须是 memory 或 redis")
    }

    if cfg.Type == "redis" && cfg.RedisURL == "" {
        v.addError("Redis 缓存需要提供 Redis URL")
    }

    if cfg.SchemaTTL <= 0 {
        v.addError("Schema 缓存 TTL 必须大于 0")
    }

    if cfg.QueryResultTTL <= 0 {
        v.addError("查询结果缓存 TTL 必须大于 0")
    }
}

// validateSecurityConfig 验证安全配置
func (v *Validator) validateSecurityConfig(cfg *SecurityConfig) {
    if cfg.JWT.Secret == "" {
        v.addError("JWT 密钥不能为空")
    }

    if len(cfg.JWT.Secret) < 32 {
        v.addError("JWT 密钥长度至少 32 字符")
    }

    if cfg.JWT.Expiry <= 0 {
        v.addError("JWT 过期时间必须大于 0")
    }

    if cfg.SQLGuard.MaxQueryLength <= 0 {
        v.addError("SQL 查询最大长度必须大于 0")
    }
}

// addError 添加错误
func (v *Validator) addError(msg string) {
    v.errors = append(v.errors, msg)
}
```

## 部署方案

### 1. Docker 部署

#### Dockerfile

```dockerfile
# 多阶段构建
FROM golang:1.21-alpine AS builder

# 设置工作目录
WORKDIR /app

# 安装依赖
RUN apk add --no-cache git ca-certificates tzdata

# 复制 go mod 文件
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY . .

# 构建应用
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o rag-server ./cmd/server

# 运行阶段
FROM alpine:latest

# 安装运行时依赖
RUN apk --no-cache add ca-certificates tzdata

# 创建非 root 用户
RUN addgroup -g 1001 -S rag && \
    adduser -u 1001 -S rag -G rag

# 设置工作目录
WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /app/rag-server .

# 复制配置文件
COPY --from=builder /app/config ./config

# 创建日志目录
RUN mkdir -p /var/log/rag && \
    chown -R rag:rag /var/log/rag

# 切换到非 root 用户
USER rag

# 暴露端口
EXPOSE 8080 9090 8081

# 健康检查
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8081/health || exit 1

# 启动命令
CMD ["./rag-server", "--config", "config/rag.yaml"]
```

#### docker-compose.yml

```yaml
version: "3.8"

services:
  # RAG 服务
  rag-server:
    build:
      context: .
      dockerfile: Dockerfile
    ports:
      - "8080:8080" # MCP 服务端口
      - "9090:9090" # Metrics 端口
      - "8081:8081" # Health 端口
    environment:
      - APP_ENV=production
      - DB_HOST=mysql
      - DB_USERNAME=rag_user
      - DB_PASSWORD=rag_password
      - DB_DATABASE=rag_production
      - REDIS_URL=redis://redis:6379/0
      - OPENAI_API_KEY=${OPENAI_API_KEY}
      - JWT_SECRET=${JWT_SECRET}
    volumes:
      - ./config:/app/config:ro
      - ./logs:/var/log/rag
    depends_on:
      mysql:
        condition: service_healthy
      redis:
        condition: service_healthy
    restart: unless-stopped
    networks:
      - rag-network
    deploy:
      resources:
        limits:
          memory: 2G
          cpus: "1.0"
        reservations:
          memory: 1G
          cpus: "0.5"

  # MySQL 数据库
  mysql:
    image: mysql:8.0
    environment:
      - MYSQL_ROOT_PASSWORD=root_password
      - MYSQL_DATABASE=rag_production
      - MYSQL_USER=rag_user
      - MYSQL_PASSWORD=rag_password
    ports:
      - "3306:3306"
    volumes:
      - mysql_data:/var/lib/mysql
      - ./init.sql:/docker-entrypoint-initdb.d/init.sql:ro
    command: >
      --character-set-server=utf8mb4
      --collation-server=utf8mb4_unicode_ci
      --max_connections=1000
      --innodb_buffer_pool_size=1G
      --slow_query_log=1
      --slow_query_log_file=/var/log/mysql/slow.log
      --long_query_time=1
    healthcheck:
      test: ["CMD", "mysqladmin", "ping", "-h", "localhost"]
      timeout: 20s
      retries: 10
    restart: unless-stopped
    networks:
      - rag-network

  # Redis 缓存
  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data
      - ./redis.conf:/usr/local/etc/redis/redis.conf:ro
    command: redis-server /usr/local/etc/redis/redis.conf
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      timeout: 10s
      retries: 5
    restart: unless-stopped
    networks:
      - rag-network

  # Nginx 反向代理
  nginx:
    image: nginx:alpine
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./nginx.conf:/etc/nginx/nginx.conf:ro
      - ./ssl:/etc/ssl:ro
    depends_on:
      - rag-server
    restart: unless-stopped
    networks:
      - rag-network

  # Prometheus 监控
  prometheus:
    image: prom/prometheus:latest
    ports:
      - "9091:9090"
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml:ro
      - prometheus_data:/prometheus
    command:
      - "--config.file=/etc/prometheus/prometheus.yml"
      - "--storage.tsdb.path=/prometheus"
      - "--web.console.libraries=/etc/prometheus/console_libraries"
      - "--web.console.templates=/etc/prometheus/consoles"
      - "--storage.tsdb.retention.time=200h"
      - "--web.enable-lifecycle"
    restart: unless-stopped
    networks:
      - rag-network

  # Grafana 可视化
  grafana:
    image: grafana/grafana:latest
    ports:
      - "3000:3000"
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=admin
    volumes:
      - grafana_data:/var/lib/grafana
      - ./grafana/dashboards:/etc/grafana/provisioning/dashboards:ro
      - ./grafana/datasources:/etc/grafana/provisioning/datasources:ro
    depends_on:
      - prometheus
    restart: unless-stopped
    networks:
      - rag-network

volumes:
  mysql_data:
  redis_data:
  prometheus_data:
  grafana_data:

networks:
  rag-network:
    driver: bridge
```

### 2. Kubernetes 部署

#### Namespace

```yaml
# namespace.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: rag-system
  labels:
    name: rag-system
```

#### ConfigMap

```yaml
# configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: rag-config
  namespace: rag-system
data:
  rag.yaml: |
    rag:
      mcp:
        server:
          host: "0.0.0.0"
          port: 8080
          timeout: "60s"
          max_connections: 1000
      
      database:
        host: "mysql-service"
        port: 3306
        username: "rag_user"
        database: "rag_production"
        max_open_conns: 100
        max_idle_conns: 20
        conn_max_lifetime: "1h"
      
      cache:
        type: "redis"
        redis_url: "redis://redis-service:6379/0"
        schema_ttl: "1h"
        query_result_ttl: "10m"
      
      logging:
        level: "info"
        format: "json"
        output: "stdout"
      
      monitoring:
        metrics:
          enabled: true
          port: 9090
        health:
          enabled: true
          port: 8081
```

#### Secret

```yaml
# secret.yaml
apiVersion: v1
kind: Secret
metadata:
  name: rag-secrets
  namespace: rag-system
type: Opaque
data:
  db-password: <base64-encoded-password>
  jwt-secret: <base64-encoded-jwt-secret>
  openai-api-key: <base64-encoded-openai-key>
```

#### Deployment

```yaml
# deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: rag-server
  namespace: rag-system
  labels:
    app: rag-server
spec:
  replicas: 3
  selector:
    matchLabels:
      app: rag-server
  template:
    metadata:
      labels:
        app: rag-server
    spec:
      containers:
        - name: rag-server
          image: rag-server:latest
          ports:
            - containerPort: 8080
              name: mcp
            - containerPort: 9090
              name: metrics
            - containerPort: 8081
              name: health
          env:
            - name: APP_ENV
              value: "production"
            - name: DB_HOST
              value: "mysql-service"
            - name: DB_USERNAME
              value: "rag_user"
            - name: DB_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: rag-secrets
                  key: db-password
            - name: DB_DATABASE
              value: "rag_production"
            - name: REDIS_URL
              value: "redis://redis-service:6379/0"
            - name: OPENAI_API_KEY
              valueFrom:
                secretKeyRef:
                  name: rag-secrets
                  key: openai-api-key
            - name: JWT_SECRET
              valueFrom:
                secretKeyRef:
                  name: rag-secrets
                  key: jwt-secret
          volumeMounts:
            - name: config-volume
              mountPath: /app/config
              readOnly: true
          resources:
            requests:
              memory: "1Gi"
              cpu: "500m"
            limits:
              memory: "2Gi"
              cpu: "1000m"
          livenessProbe:
            httpGet:
              path: /health
              port: 8081
            initialDelaySeconds: 30
            periodSeconds: 10
          readinessProbe:
            httpGet:
              path: /health
              port: 8081
            initialDelaySeconds: 5
            periodSeconds: 5
      volumes:
        - name: config-volume
          configMap:
            name: rag-config
```

#### Service

```yaml
# service.yaml
apiVersion: v1
kind: Service
metadata:
  name: rag-service
  namespace: rag-system
  labels:
    app: rag-server
spec:
  selector:
    app: rag-server
  ports:
    - name: mcp
      port: 8080
      targetPort: 8080
    - name: metrics
      port: 9090
      targetPort: 9090
    - name: health
      port: 8081
      targetPort: 8081
  type: ClusterIP
```

#### Ingress

```yaml
# ingress.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: rag-ingress
  namespace: rag-system
  annotations:
    kubernetes.io/ingress.class: nginx
    cert-manager.io/cluster-issuer: letsencrypt-prod
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
    nginx.ingress.kubernetes.io/proxy-read-timeout: "300"
    nginx.ingress.kubernetes.io/proxy-send-timeout: "300"
spec:
  tls:
    - hosts:
        - rag.yourdomain.com
      secretName: rag-tls
  rules:
    - host: rag.yourdomain.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: rag-service
                port:
                  number: 8080
```

### 3. 监控和日志

#### Prometheus 配置

```yaml
# prometheus.yml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

rule_files:
  - "rag_rules.yml"

scrape_configs:
  - job_name: "rag-server"
    static_configs:
      - targets: ["rag-service:9090"]
    metrics_path: /metrics
    scrape_interval: 10s

  - job_name: "mysql"
    static_configs:
      - targets: ["mysql-exporter:9104"]

  - job_name: "redis"
    static_configs:
      - targets: ["redis-exporter:9121"]

alerting:
  alertmanagers:
    - static_configs:
        - targets:
            - alertmanager:9093
```

#### 告警规则

```yaml
# rag_rules.yml
groups:
  - name: rag-alerts
    rules:
      - alert: RAGServerDown
        expr: up{job="rag-server"} == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "RAG 服务器宕机"
          description: "RAG 服务器已宕机超过 1 分钟"

      - alert: HighQueryLatency
        expr: histogram_quantile(0.95, rate(rag_query_duration_seconds_bucket[5m])) > 5
        for: 2m
        labels:
          severity: warning
        annotations:
          summary: "查询延迟过高"
          description: "95% 的查询延迟超过 5 秒"

      - alert: HighErrorRate
        expr: rate(rag_query_errors_total[5m]) > 0.1
        for: 1m
        labels:
          severity: warning
        annotations:
          summary: "错误率过高"
          description: "查询错误率超过 10%"

      - alert: DatabaseConnectionPoolExhausted
        expr: rag_db_connections_active / rag_db_connections_max > 0.9
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "数据库连接池即将耗尽"
          description: "数据库连接池使用率超过 90%"
```

### 4. 部署脚本

#### 部署脚本 (deploy.sh)

```bash
#!/bin/bash

set -e

# 配置变量
ENVIRONMENT=${1:-production}
NAMESPACE="rag-system"
IMAGE_TAG=${2:-latest}

echo "开始部署 RAG 系统到 ${ENVIRONMENT} 环境..."

# 检查必要的工具
command -v kubectl >/dev/null 2>&1 || { echo "kubectl 未安装" >&2; exit 1; }
command -v docker >/dev/null 2>&1 || { echo "docker 未安装" >&2; exit 1; }

# 构建 Docker 镜像
echo "构建 Docker 镜像..."
docker build -t rag-server:${IMAGE_TAG} .

# 如果是生产环境，推送到镜像仓库
if [ "$ENVIRONMENT" = "production" ]; then
    echo "推送镜像到仓库..."
    docker tag rag-server:${IMAGE_TAG} your-registry/rag-server:${IMAGE_TAG}
    docker push your-registry/rag-server:${IMAGE_TAG}
fi

# 创建命名空间
echo "创建命名空间..."
kubectl apply -f k8s/namespace.yaml

# 应用配置
echo "应用配置..."
kubectl apply -f k8s/configmap.yaml
kubectl apply -f k8s/secret.yaml

# 部署数据库和缓存
echo "部署依赖服务..."
kubectl apply -f k8s/mysql.yaml
kubectl apply -f k8s/redis.yaml

# 等待依赖服务就绪
echo "等待依赖服务就绪..."
kubectl wait --for=condition=ready pod -l app=mysql -n ${NAMESPACE} --timeout=300s
kubectl wait --for=condition=ready pod -l app=redis -n ${NAMESPACE} --timeout=300s

# 部署应用
echo "部署 RAG 服务..."
kubectl apply -f k8s/deployment.yaml
kubectl apply -f k8s/service.yaml

# 等待应用就绪
echo "等待应用就绪..."
kubectl wait --for=condition=ready pod -l app=rag-server -n ${NAMESPACE} --timeout=300s

# 部署 Ingress
if [ "$ENVIRONMENT" = "production" ]; then
    echo "部署 Ingress..."
    kubectl apply -f k8s/ingress.yaml
fi

# 部署监控
echo "部署监控..."
kubectl apply -f k8s/monitoring/

echo "部署完成！"

# 显示服务状态
echo "服务状态："
kubectl get pods -n ${NAMESPACE}
kubectl get services -n ${NAMESPACE}

# 显示访问信息
if [ "$ENVIRONMENT" = "production" ]; then
    echo "生产环境访问地址："
    kubectl get ingress -n ${NAMESPACE}
else
    echo "开发环境访问地址："
    echo "MCP 服务: http://localhost:8080"
    echo "监控指标: http://localhost:9090"
    echo "健康检查: http://localhost:8081/health"
fi
```

#### 回滚脚本 (rollback.sh)

```bash
#!/bin/bash

set -e

NAMESPACE="rag-system"
REVISION=${1:-1}

echo "回滚 RAG 系统到版本 ${REVISION}..."

# 回滚部署
kubectl rollout undo deployment/rag-server -n ${NAMESPACE} --to-revision=${REVISION}

# 等待回滚完成
kubectl rollout status deployment/rag-server -n ${NAMESPACE}

echo "回滚完成！"

# 显示当前状态
kubectl get pods -n ${NAMESPACE}
```

### 5. 性能调优

#### 数据库优化配置

```sql
-- MySQL 性能优化配置
-- my.cnf 配置建议

[mysqld]
# 基础配置
port = 3306
socket = /var/lib/mysql/mysql.sock
datadir = /var/lib/mysql

# 字符集配置
character-set-server = utf8mb4
collation-server = utf8mb4_unicode_ci

# 连接配置
max_connections = 1000
max_connect_errors = 100000
max_allowed_packet = 64M
interactive_timeout = 600
wait_timeout = 600

# InnoDB 配置
innodb_buffer_pool_size = 2G
innodb_log_file_size = 256M
innodb_log_buffer_size = 64M
innodb_flush_log_at_trx_commit = 2
innodb_flush_method = O_DIRECT
innodb_file_per_table = 1
innodb_io_capacity = 2000
innodb_read_io_threads = 8
innodb_write_io_threads = 8

# 查询缓存
query_cache_type = 1
query_cache_size = 256M
query_cache_limit = 2M

# 慢查询日志
slow_query_log = 1
slow_query_log_file = /var/log/mysql/slow.log
long_query_time = 1
log_queries_not_using_indexes = 1

# 二进制日志
log_bin = mysql-bin
binlog_format = ROW
expire_logs_days = 7
max_binlog_size = 100M

# 临时表配置
tmp_table_size = 256M
max_heap_table_size = 256M

# 排序和分组配置
sort_buffer_size = 2M
read_buffer_size = 2M
read_rnd_buffer_size = 8M
join_buffer_size = 2M
```

#### Redis 优化配置

```conf
# redis.conf 性能优化配置

# 内存配置
maxmemory 1gb
maxmemory-policy allkeys-lru

# 持久化配置
save 900 1
save 300 10
save 60 10000

# AOF 配置
appendonly yes
appendfsync everysec
no-appendfsync-on-rewrite no
auto-aof-rewrite-percentage 100
auto-aof-rewrite-min-size 64mb

# 网络配置
tcp-keepalive 300
timeout 0
tcp-backlog 511

# 客户端配置
maxclients 10000

# 慢日志配置
slowlog-log-slower-than 10000
slowlog-max-len 128

# 内存优化
hash-max-ziplist-entries 512
hash-max-ziplist-value 64
list-max-ziplist-size -2
list-compress-depth 0
set-max-intset-entries 512
zset-max-ziplist-entries 128
zset-max-ziplist-value 64
```

这个配置和部署指南提供了完整的生产环境部署方案，包括配置管理、Docker 容器化、Kubernetes 编排、监控告警和性能优化等最佳实践。
