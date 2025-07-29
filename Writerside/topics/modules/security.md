# 安全模块

本模块实现了 RAG 系统的安全和权限控制功能，提供了完整的安全防护体系。

## 功能特性

### 认证授权机制

- **JWT Token 验证和管理**：支持 JWT Token 的解析、验证和缓存
- **基于角色的权限控制 (RBAC)**：集成 Casbin 实现细粒度权限控制
- **API 访问权限验证**：支持 API 路径和方法级别的权限验证
- **用户会话管理**：支持用户信息缓存和会话管理

### SQL 注入防护

- **SQL 语句安全检查和过滤**：检测和阻止常见的 SQL 注入攻击模式
- **危险操作检测和阻止**：识别和阻止危险的 SQL 操作（DROP、DELETE 等）
- **查询权限细粒度控制**：基于用户权限控制表级和操作级访问
- **参数化查询验证**：验证参数化查询的安全性
- **查询复杂度控制**：限制 JOIN 数量和子查询深度
- **表访问控制**：支持表级别的白名单和黑名单

### 数据访问控制

- **基于用户角色的数据访问限制**：根据用户角色过滤可访问的数据
- **敏感数据脱敏和保护**：自动识别和脱敏敏感字段（密码、邮箱、手机号等）
- **审计日志和操作追踪**：记录所有数据访问和安全事件
- **数据行级权限控制**：支持基于条件的行级数据过滤
- **数据治理策略**：支持数据分类、加密、保留期等治理功能
- **字段级权限控制**：支持字段级别的访问控制

### 审计日志系统

- **全面的事件记录**：记录登录、权限检查、数据访问等所有安全事件
- **安全违规检测**：自动检测和记录安全违规行为
- **事件统计和分析**：提供审计事件的统计和分析功能
- **告警机制**：支持频繁违规的自动告警
- **合规性报告**：支持审计事件的导出和报告生成

## 模块结构

```
security/
├── README.md              # 模块说明文档
├── auth.go               # 认证授权实现
├── auth_test.go          # 认证授权测试
├── sql_guard.go          # SQL 注入防护
├── sql_guard_test.go     # SQL 防护测试
├── access_control.go     # 数据访问控制
├── access_control_test.go # 访问控制测试
├── audit.go              # 审计日志
├── audit_test.go         # 审计日志测试
└── manager.go            # 安全管理器
```

## 核心组件

### AuthManager - 认证授权管理器

负责 JWT Token 验证、用户权限检查和角色管理。

**主要功能：**

- JWT Token 解析和验证
- 用户角色和权限管理
- 权限检查和授权
- 用户信息缓存

### SQLGuard - SQL 安全防护器

负责 SQL 语句的安全检查和注入防护。

**主要功能：**

- SQL 注入模式检测
- 危险操作阻止
- 查询复杂度控制
- 表访问权限验证
- 参数化查询验证

### AccessController - 数据访问控制器

负责数据级别的访问控制和敏感数据保护。

**主要功能：**

- 数据过滤和脱敏
- 行级权限控制
- 字段级权限控制
- 数据治理策略
- 敏感数据加密

### AuditLogger - 审计日志记录器

负责安全事件的记录、分析和报告。

**主要功能：**

- 安全事件记录
- 违规行为检测
- 事件统计分析
- 告警机制
- 合规性报告

### Manager - 安全管理器

统一的安全管理接口，集成所有安全组件。

**主要功能：**

- 统一安全接口
- 组件协调管理
- 安全策略配置
- 事件监控告警

## 使用示例

### 认证授权

```go
// 创建安全管理器
securityManager := security.NewManager(config, cache, logger, metrics)

// 验证 JWT Token
userInfo, err := securityManager.ValidateToken(ctx, token)
if err != nil {
    return fmt.Errorf("token 验证失败: %w", err)
}

// 检查权限
err = securityManager.CheckPermission(ctx, userInfo, "users", "read")
if err != nil {
    return fmt.Errorf("权限不足: %w", err)
}

// 检查 API 访问权限
err = securityManager.CheckAPIAccess(ctx, userInfo, "/api/users", "GET")
```

### SQL 安全检查

```go
// 验证 SQL 语句安全性
err := securityManager.ValidateSQL(ctx, sqlQuery)
if err != nil {
    return fmt.Errorf("SQL 安全检查失败: %w", err)
}

// 综合验证（SQL + 表权限 + 复杂度）
tables := []string{"users", "profiles"}
err = securityManager.ValidateQueryAccess(ctx, userInfo, sqlQuery, tables)
```

### 数据访问控制

```go
// 过滤查询结果
filteredData, err := securityManager.FilterQueryResult(ctx, userInfo, "users", data)
if err != nil {
    return fmt.Errorf("数据过滤失败: %w", err)
}

// 应用数据治理策略
governedData, err := accessController.ApplyDataGovernance(ctx, userInfo, "sensitive_table", data)
```

### 审计日志

```go
// 记录查询执行
securityManager.LogQueryExecution(ctx, userInfo, sql, success, duration, rowCount, err)

// 记录安全违规
securityManager.LogSecurityViolation(ctx, userInfo, "sql_injection", "检测到 SQL 注入尝试", details)

// 获取审计事件
events, err := securityManager.GetAuditEvents(ctx, userID, 100)

// 导出审计报告
report, err := securityManager.ExportAuditEvents(ctx, startTime, endTime, eventTypes)
```

## 配置示例

```yaml
security:
  jwt_secret: "your-secret-key"
  token_expiry: "24h"
  enable_rbac: true

  sql_guard:
    max_query_length: 10000
    strict_mode: true
    allowed_operations: ["SELECT", "SHOW", "DESCRIBE", "EXPLAIN"]
    max_join_count: 5
    max_subquery_depth: 3

  access_control:
    enable_data_masking: true
    enable_audit_logging: true
    default_retention_days: 365

  audit:
    enable_security_alerts: true
    violation_threshold: 5
    alert_window: "15m"
```

## 安全特性

### 防护能力

- **SQL 注入防护**：检测和阻止各种 SQL 注入攻击
- **权限控制**：细粒度的 RBAC 权限控制
- **数据脱敏**：自动识别和脱敏敏感数据
- **访问控制**：多层次的数据访问控制
- **审计追踪**：完整的操作审计和追踪

### 合规性支持

- **数据分类**：支持数据分类和标记
- **保留策略**：支持数据保留和清理策略
- **加密保护**：支持敏感数据加密
- **审计报告**：支持合规性审计报告
- **访问记录**：完整的数据访问记录

### 性能优化

- **缓存机制**：用户信息和权限缓存
- **批量处理**：支持批量数据处理
- **异步日志**：异步审计日志记录
- **索引优化**：审计数据索引优化
- **资源控制**：查询复杂度控制

## 测试覆盖

模块包含完整的单元测试，覆盖所有核心功能：

- **认证授权测试**：Token 验证、权限检查、角色管理
- **SQL 防护测试**：注入检测、操作控制、复杂度限制
- **访问控制测试**：数据过滤、脱敏、治理策略
- **审计日志测试**：事件记录、统计分析、报告生成

运行测试：

```bash
go test ./rag/security -v
```

## 扩展性

模块设计支持灵活的扩展：

- **自定义脱敏规则**：支持添加自定义数据脱敏规则
- **插件化权限**：支持自定义权限检查逻辑
- **可配置策略**：支持动态配置安全策略
- **事件钩子**：支持自定义安全事件处理
- **多数据源**：支持多种数据源的安全控制
