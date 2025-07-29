# MCP 协议 API 文档

## 概述

RAG MySQL 查询系统基于 MCP (Model Context Protocol) 协议提供服务。MCP 是一个标准化的通信协议，支持请求-响应模式和通知机制。

## 协议版本

- **当前版本**: 1.0.0
- **支持版本**: 1.0.0
- **协议格式**: JSON over WebSocket/TCP

## 连接建立

### WebSocket 连接

```
ws://localhost:8080/mcp
```

### TCP 连接

```
tcp://localhost:8080
```

## 消息格式

### 基本消息结构

```json
{
  "type": "request|response|notification|error",
  "id": "unique-message-id",
  "method": "method-name",
  "params": {},
  "result": {},
  "error": {
    "code": -32000,
    "message": "Error description",
    "data": {}
  }
}
```

### 消息类型

- **request**: 客户端请求，需要服务器响应
- **response**: 服务器响应客户端请求
- **notification**: 单向通知，不需要响应
- **error**: 错误消息

## 协议握手

### 版本协商

客户端连接后首先进行版本协商：

**请求**:

```json
{
  "type": "request",
  "id": "negotiate-1",
  "method": "protocol.negotiate",
  "params": {
    "versions": ["1.0.0"]
  }
}
```

**响应**:

```json
{
  "type": "response",
  "id": "negotiate-1",
  "result": {
    "version": "1.0.0",
    "supported_versions": ["1.0.0"],
    "server_info": {
      "name": "RAG MySQL Query System",
      "version": "1.0.0"
    }
  }
}
```

## 核心 API 方法

### 1. 查询执行 (query.execute)

执行自然语言查询并返回结果。

**请求**:

```json
{
  "type": "request",
  "id": "query-1",
  "method": "query.execute",
  "params": {
    "query": "查询所有用户的信息",
    "options": {
      "limit": 20,
      "offset": 0,
      "format": "json",
      "explain": true,
      "optimize": true,
      "timeout": 30
    },
    "session_id": "session-123",
    "user_id": "user-456"
  }
}
```

**响应**:

```json
{
  "type": "response",
  "id": "query-1",
  "result": {
    "success": true,
    "data": [
      {
        "id": 1,
        "name": "张三",
        "email": "zhangsan@example.com",
        "created_at": "2025-01-20T10:30:00Z"
      }
    ],
    "sql": "SELECT id, name, email, created_at FROM users LIMIT 20",
    "explanation": "查询用户表中的所有用户基本信息，包括ID、姓名、邮箱和创建时间",
    "metadata": {
      "execution_time": "0.025s",
      "row_count": 1,
      "cache_hit": false
    }
  }
}
```

**参数说明**:

- `query` (string, 必填): 自然语言查询内容
- `options` (object, 可选): 查询选项
  - `limit` (int): 返回结果最大条数，默认 20
  - `offset` (int): 结果偏移量，默认 0
  - `format` (string): 结果格式，支持 "json", "csv", "table"
  - `explain` (bool): 是否返回查询解释，默认 false
  - `optimize` (bool): 是否优化查询，默认 true
  - `timeout` (int): 查询超时时间（秒），默认 30
- `session_id` (string, 可选): 会话 ID，用于维护查询上下文
- `user_id` (string, 可选): 用户 ID，用于权限控制

### 2. 查询解释 (query.explain)

解释查询意图和生成的 SQL 语句。

**请求**:

```json
{
  "type": "request",
  "id": "explain-1",
  "method": "query.explain",
  "params": {
    "query": "统计每个部门的员工数量"
  }
}
```

**响应**:

```json
{
  "type": "response",
  "id": "explain-1",
  "result": {
    "intent": "统计聚合查询",
    "tables": ["employees", "departments"],
    "columns": ["department_id", "COUNT(*)"],
    "conditions": [],
    "operations": ["JOIN", "GROUP BY", "COUNT"],
    "complexity": "medium",
    "sql": "SELECT d.name, COUNT(e.id) as employee_count FROM departments d LEFT JOIN employees e ON d.id = e.department_id GROUP BY d.id, d.name",
    "explanation": "这个查询统计每个部门的员工数量。通过左连接部门表和员工表，按部门分组统计员工数量。",
    "suggestions": [
      "可以添加 ORDER BY employee_count DESC 按员工数量降序排列",
      "考虑添加 HAVING 子句过滤员工数量为0的部门"
    ]
  }
}
```

### 3. 查询建议 (query.suggest)

根据部分输入提供查询建议。

**请求**:

```json
{
  "type": "request",
  "id": "suggest-1",
  "method": "query.suggest",
  "params": {
    "partial": "查询用户",
    "limit": 5
  }
}
```

**响应**:

```json
{
  "type": "response",
  "id": "suggest-1",
  "result": {
    "suggestions": [
      "查询用户的基本信息",
      "查询用户的注册时间",
      "查询用户的登录记录",
      "查询用户的订单信息",
      "查询用户的权限设置"
    ]
  }
}
```

### 4. Schema 信息 (schema.info)

获取数据库 Schema 信息。

**请求**:

```json
{
  "type": "request",
  "id": "schema-1",
  "method": "schema.info",
  "params": {
    "table": "users",
    "include_relationships": true
  }
}
```

**响应**:

```json
{
  "type": "response",
  "id": "schema-1",
  "result": {
    "table": {
      "name": "users",
      "comment": "用户信息表",
      "engine": "InnoDB",
      "charset": "utf8mb4",
      "columns": [
        {
          "name": "id",
          "type": "bigint",
          "nullable": false,
          "default_value": "",
          "comment": "用户ID",
          "is_primary_key": true,
          "is_auto_increment": true
        },
        {
          "name": "name",
          "type": "varchar(100)",
          "nullable": false,
          "default_value": "",
          "comment": "用户姓名"
        },
        {
          "name": "email",
          "type": "varchar(255)",
          "nullable": false,
          "default_value": "",
          "comment": "邮箱地址"
        }
      ],
      "indexes": [
        {
          "name": "PRIMARY",
          "type": "PRIMARY",
          "columns": ["id"],
          "is_unique": true
        },
        {
          "name": "idx_email",
          "type": "UNIQUE",
          "columns": ["email"],
          "is_unique": true
        }
      ],
      "foreign_keys": [],
      "relationships": [
        {
          "type": "one_to_many",
          "from_table": "users",
          "from_column": "id",
          "to_table": "orders",
          "to_column": "user_id"
        }
      ]
    }
  }
}
```

### 5. Schema 搜索 (schema.search)

搜索相关的表和字段。

**请求**:

```json
{
  "type": "request",
  "id": "search-1",
  "method": "schema.search",
  "params": {
    "query": "用户订单",
    "limit": 10
  }
}
```

**响应**:

```json
{
  "type": "response",
  "id": "search-1",
  "result": {
    "tables": [
      {
        "name": "users",
        "comment": "用户信息表",
        "relevance": 0.9
      },
      {
        "name": "orders",
        "comment": "订单信息表",
        "relevance": 0.8
      },
      {
        "name": "user_profiles",
        "comment": "用户详细信息表",
        "relevance": 0.7
      }
    ],
    "columns": [
      {
        "table": "users",
        "column": "id",
        "comment": "用户ID",
        "relevance": 0.8
      },
      {
        "table": "orders",
        "column": "user_id",
        "comment": "用户ID",
        "relevance": 0.8
      }
    ]
  }
}
```

### 6. 会话管理 (session.\*)

#### 创建会话 (session.create)

**请求**:

```json
{
  "type": "request",
  "id": "session-create-1",
  "method": "session.create",
  "params": {
    "user_id": "user-123"
  }
}
```

**响应**:

```json
{
  "type": "response",
  "id": "session-create-1",
  "result": {
    "session_id": "session-456",
    "created_at": "2025-01-20T10:30:00Z",
    "expires_at": "2025-01-20T11:30:00Z"
  }
}
```

#### 获取会话 (session.get)

**请求**:

```json
{
  "type": "request",
  "id": "session-get-1",
  "method": "session.get",
  "params": {
    "session_id": "session-456"
  }
}
```

**响应**:

```json
{
  "type": "response",
  "id": "session-get-1",
  "result": {
    "session_id": "session-456",
    "user_id": "user-123",
    "history": [
      {
        "query": "查询所有用户",
        "sql": "SELECT * FROM users",
        "timestamp": "2025-01-20T10:25:00Z",
        "success": true
      }
    ],
    "context": {
      "last_table": "users",
      "preferred_limit": 20
    },
    "created_at": "2025-01-20T10:30:00Z",
    "last_accessed": "2025-01-20T10:35:00Z"
  }
}
```

#### 销毁会话 (session.destroy)

**请求**:

```json
{
  "type": "request",
  "id": "session-destroy-1",
  "method": "session.destroy",
  "params": {
    "session_id": "session-456"
  }
}
```

**响应**:

```json
{
  "type": "response",
  "id": "session-destroy-1",
  "result": {
    "success": true,
    "message": "会话已成功销毁"
  }
}
```

## 错误处理

### 错误码定义

| 错误码 | 名称                 | 描述           |
| ------ | -------------------- | -------------- |
| -32700 | Parse Error          | JSON 解析错误  |
| -32600 | Invalid Request      | 无效请求       |
| -32601 | Method Not Found     | 方法未找到     |
| -32602 | Invalid Params       | 无效参数       |
| -32603 | Internal Error       | 内部错误       |
| -32000 | Server Error         | 服务器错误     |
| -32001 | Version Error        | 版本不兼容     |
| -32002 | Timeout Error        | 超时错误       |
| -32003 | Rate Limit Error     | 限流错误       |
| -31000 | Authentication Error | 认证错误       |
| -31001 | Authorization Error  | 授权错误       |
| -31002 | Session Error        | 会话错误       |
| -31003 | Database Error       | 数据库错误     |
| -31004 | SQL Error            | SQL 错误       |
| -31005 | Schema Error         | Schema 错误    |
| -31006 | Query Error          | 查询错误       |
| -31007 | LLM Error            | 大语言模型错误 |

### 错误响应格式

```json
{
  "type": "response",
  "id": "request-id",
  "error": {
    "code": -31004,
    "message": "SQL 语法错误",
    "data": {
      "sql": "SELECT * FORM users",
      "position": 9,
      "suggestion": "将 'FORM' 修正为 'FROM'"
    }
  }
}
```

### 常见错误示例

#### 1. 认证错误

```json
{
  "type": "response",
  "id": "query-1",
  "error": {
    "code": -31000,
    "message": "认证失败",
    "data": {
      "reason": "访问令牌已过期",
      "expires_at": "2025-01-20T10:00:00Z"
    }
  }
}
```

#### 2. 权限错误

```json
{
  "type": "response",
  "id": "query-1",
  "error": {
    "code": -31001,
    "message": "权限不足",
    "data": {
      "required_permission": "table:users:read",
      "user_permissions": ["table:orders:read"]
    }
  }
}
```

#### 3. SQL 错误

```json
{
  "type": "response",
  "id": "query-1",
  "error": {
    "code": -31004,
    "message": "SQL 执行错误",
    "data": {
      "sql": "SELECT * FROM non_existent_table",
      "mysql_error": "Table 'database.non_existent_table' doesn't exist",
      "mysql_errno": 1146
    }
  }
}
```

#### 4. 查询超时

```json
{
  "type": "response",
  "id": "query-1",
  "error": {
    "code": -32002,
    "message": "查询超时",
    "data": {
      "timeout": 30,
      "elapsed": 35.2
    }
  }
}
```

## 通知消息

### 查询进度通知 (query.progress)

对于长时间运行的查询，服务器会发送进度通知：

```json
{
  "type": "notification",
  "method": "query.progress",
  "params": {
    "query_id": "query-1",
    "progress": 0.6,
    "message": "正在执行查询...",
    "elapsed": 15.5
  }
}
```

### 会话过期通知 (session.expired)

```json
{
  "type": "notification",
  "method": "session.expired",
  "params": {
    "session_id": "session-456",
    "expired_at": "2025-01-20T11:30:00Z"
  }
}
```

### 系统状态通知 (system.status)

```json
{
  "type": "notification",
  "method": "system.status",
  "params": {
    "status": "maintenance",
    "message": "系统将在 5 分钟后进入维护模式",
    "scheduled_at": "2025-01-20T12:00:00Z"
  }
}
```

## 认证和授权

### JWT Token 认证

客户端需要在连接建立后发送认证信息：

**请求**:

```json
{
  "type": "request",
  "id": "auth-1",
  "method": "auth.authenticate",
  "params": {
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
  }
}
```

**响应**:

```json
{
  "type": "response",
  "id": "auth-1",
  "result": {
    "success": true,
    "user": {
      "id": "user-123",
      "username": "zhangsan",
      "roles": ["user"],
      "permissions": ["table:users:read", "table:orders:read"]
    },
    "expires_at": "2025-01-20T12:00:00Z"
  }
}
```

### 权限检查

每个需要权限的操作都会进行权限检查：

- `table:{table_name}:read` - 读取表数据
- `table:{table_name}:write` - 写入表数据
- `schema:read` - 读取 Schema 信息
- `session:manage` - 管理会话
- `admin:*` - 管理员权限

## 性能和限制

### 请求限制

- 最大并发连接数: 1000
- 每个连接的最大并发请求数: 10
- 请求频率限制: 100 请求/分钟
- 最大消息大小: 10MB
- 查询超时时间: 300 秒

### 响应限制

- 最大返回行数: 10000
- 最大响应大小: 100MB
- 缓存有效期: Schema 1 小时，查询结果 10 分钟

### 性能优化建议

1. **使用会话**: 维护会话可以利用查询历史和上下文优化
2. **合理分页**: 使用 limit 和 offset 参数进行分页
3. **启用缓存**: 重复查询会从缓存中返回结果
4. **优化查询**: 启用 optimize 选项让系统优化 SQL
5. **批量操作**: 对于多个相关查询，考虑合并为单个复杂查询

## 最佳实践

### 1. 连接管理

```javascript
// 建立连接
const ws = new WebSocket("ws://localhost:8080/mcp");

// 版本协商
ws.send(
  JSON.stringify({
    type: "request",
    id: "negotiate-1",
    method: "protocol.negotiate",
    params: { versions: ["1.0.0"] },
  })
);

// 认证
ws.send(
  JSON.stringify({
    type: "request",
    id: "auth-1",
    method: "auth.authenticate",
    params: { token: "your-jwt-token" },
  })
);
```

### 2. 错误处理

```javascript
function handleResponse(response) {
  if (response.error) {
    switch (response.error.code) {
      case -31000: // 认证错误
        // 重新认证
        break;
      case -31001: // 权限错误
        // 显示权限不足提示
        break;
      case -32002: // 超时错误
        // 重试或调整超时时间
        break;
      default:
        // 通用错误处理
        console.error("API Error:", response.error);
    }
  } else {
    // 处理成功响应
    console.log("Result:", response.result);
  }
}
```

### 3. 会话管理

```javascript
class SessionManager {
  constructor() {
    this.sessionId = null;
  }

  async createSession(userId) {
    const response = await this.sendRequest("session.create", {
      user_id: userId,
    });
    this.sessionId = response.result.session_id;
    return this.sessionId;
  }

  async query(queryText, options = {}) {
    return this.sendRequest("query.execute", {
      query: queryText,
      session_id: this.sessionId,
      options,
    });
  }

  async destroySession() {
    if (this.sessionId) {
      await this.sendRequest("session.destroy", { session_id: this.sessionId });
      this.sessionId = null;
    }
  }
}
```

## 故障排查

### 常见问题

#### 1. 连接失败

**问题**: WebSocket 连接无法建立
**解决方案**:

- 检查服务器是否运行在指定端口
- 确认防火墙设置允许连接
- 验证 URL 格式是否正确

#### 2. 认证失败

**问题**: JWT Token 认证失败
**解决方案**:

- 检查 Token 是否过期
- 验证 Token 签名是否正确
- 确认用户权限是否足够

#### 3. 查询错误

**问题**: 自然语言查询无法转换为 SQL
**解决方案**:

- 使用更具体的查询描述
- 检查表名和字段名是否正确
- 查看 Schema 信息确认数据结构

#### 4. 性能问题

**问题**: 查询响应缓慢
**解决方案**:

- 启用查询优化选项
- 使用适当的分页参数
- 检查数据库索引是否合理
- 监控系统资源使用情况

### 调试工具

#### 1. 启用详细日志

在配置文件中设置日志级别为 debug：

```yaml
log:
  level: debug
  format: json
  output: stdout
```

#### 2. 查询解释

使用 `explain: true` 选项查看查询解释：

```json
{
  "method": "query.execute",
  "params": {
    "query": "查询用户信息",
    "options": { "explain": true }
  }
}
```

#### 3. 性能监控

访问监控端点查看系统状态：

```
GET http://localhost:9090/metrics
```

## 版本历史

### v1.0.0 (2025-01-20)

- 初始版本发布
- 支持基本的查询执行功能
- 实现 Schema 管理和搜索
- 添加会话管理功能
- 完整的错误处理机制
- JWT 认证和 RBAC 授权
- 性能监控和日志记录

## 相关链接

- [使用示例](../examples/)
- [开发指南](../development/)
- [配置参考](../development/configuration.md)
- [部署指南](../development/deployment.md)
