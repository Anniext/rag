# 错误码参考手册

## 概述

本文档详细描述了 RAG MySQL 查询系统中所有可能的错误码、错误原因和解决方案。

## 错误码分类

错误码按照功能模块进行分类：

- **-327xx**: 协议层错误
- **-320xx**: 服务器通用错误
- **-310xx**: 认证授权错误
- **-311xx**: 会话管理错误
- **-312xx**: 数据库相关错误
- **-313xx**: 查询处理错误
- **-314xx**: Schema 相关错误
- **-315xx**: LLM 相关错误
- **-316xx**: 缓存相关错误
- **-317xx**: 监控相关错误

## 协议层错误 (-327xx)

### -32700 Parse Error

**描述**: JSON 解析错误
**原因**:

- 消息格式不是有效的 JSON
- JSON 语法错误
- 消息编码问题

**示例**:

```json
{
  "type": "response",
  "id": "error",
  "error": {
    "code": -32700,
    "message": "JSON 解析错误",
    "data": {
      "raw_message": "{ invalid json",
      "parse_error": "unexpected token at position 2"
    }
  }
}
```

**解决方案**:

- 检查发送的消息是否为有效 JSON 格式
- 确认消息编码为 UTF-8
- 使用 JSON 验证工具检查语法

### -32600 Invalid Request

**描述**: 无效请求
**原因**:

- 缺少必需的字段（type, id, method）
- 字段类型不正确
- 消息结构不符合 MCP 协议规范

**示例**:

```json
{
  "type": "response",
  "id": "request-1",
  "error": {
    "code": -32600,
    "message": "无效请求",
    "data": {
      "missing_fields": ["method"],
      "invalid_fields": {
        "type": "expected string, got number"
      }
    }
  }
}
```

**解决方案**:

- 确保请求包含所有必需字段
- 检查字段类型是否正确
- 参考 MCP 协议规范调整消息格式

### -32601 Method Not Found

**描述**: 方法未找到
**原因**:

- 请求的方法名不存在
- 方法名拼写错误
- 方法未注册或已被移除

**示例**:

```json
{
  "type": "response",
  "id": "request-1",
  "error": {
    "code": -32601,
    "message": "方法未找到: query.invalid",
    "data": {
      "method": "query.invalid",
      "available_methods": [
        "query.execute",
        "query.explain",
        "query.suggest",
        "schema.info",
        "session.create"
      ]
    }
  }
}
```

**解决方案**:

- 检查方法名拼写是否正确
- 查看 API 文档确认可用方法
- 确认使用的协议版本支持该方法

### -32602 Invalid Params

**描述**: 无效参数
**原因**:

- 缺少必需参数
- 参数类型不正确
- 参数值超出允许范围

**示例**:

```json
{
  "type": "response",
  "id": "request-1",
  "error": {
    "code": -32602,
    "message": "无效参数",
    "data": {
      "method": "query.execute",
      "errors": [
        {
          "field": "query",
          "error": "required field missing"
        },
        {
          "field": "options.limit",
          "error": "value 10000 exceeds maximum 1000"
        }
      ]
    }
  }
}
```

**解决方案**:

- 检查所有必需参数是否提供
- 验证参数类型和值范围
- 参考 API 文档确认参数规范

### -32603 Internal Error

**描述**: 内部错误
**原因**:

- 服务器内部异常
- 未处理的运行时错误
- 系统资源不足

**示例**:

```json
{
  "type": "response",
  "id": "request-1",
  "error": {
    "code": -32603,
    "message": "内部服务器错误",
    "data": {
      "error_id": "err-20250120-001",
      "timestamp": "2025-01-20T10:30:00Z",
      "component": "query_processor"
    }
  }
}
```

**解决方案**:

- 记录错误 ID 并联系技术支持
- 检查服务器日志获取详细信息
- 稍后重试请求

## 服务器通用错误 (-320xx)

### -32000 Server Error

**描述**: 服务器错误
**原因**:

- 服务器配置错误
- 依赖服务不可用
- 系统维护中

**示例**:

```json
{
  "type": "response",
  "id": "request-1",
  "error": {
    "code": -32000,
    "message": "服务器错误",
    "data": {
      "reason": "database connection failed",
      "retry_after": 30
    }
  }
}
```

**解决方案**:

- 等待指定时间后重试
- 检查服务器状态
- 联系系统管理员

### -32001 Version Error

**描述**: 版本不兼容错误
**原因**:

- 客户端协议版本不受支持
- 方法在当前版本中不可用
- 版本协商失败

**示例**:

```json
{
  "type": "response",
  "id": "negotiate-1",
  "error": {
    "code": -32001,
    "message": "版本不兼容",
    "data": {
      "client_versions": ["2.0.0"],
      "supported_versions": ["1.0.0"],
      "message": "请升级服务器或降级客户端版本"
    }
  }
}
```

**解决方案**:

- 使用支持的协议版本
- 升级客户端或服务器
- 检查版本兼容性矩阵

### -32002 Timeout Error

**描述**: 超时错误
**原因**:

- 请求处理时间超过限制
- 数据库查询超时
- 网络连接超时

**示例**:

```json
{
  "type": "response",
  "id": "query-1",
  "error": {
    "code": -32002,
    "message": "请求超时",
    "data": {
      "timeout": 30,
      "elapsed": 35.2,
      "stage": "database_query"
    }
  }
}
```

**解决方案**:

- 增加超时时间设置
- 优化查询复杂度
- 检查网络连接状态

### -32003 Rate Limit Error

**描述**: 限流错误
**原因**:

- 请求频率超过限制
- 并发连接数超过限制
- 用户配额已用完

**示例**:

```json
{
  "type": "response",
  "id": "query-1",
  "error": {
    "code": -32003,
    "message": "请求频率超限",
    "data": {
      "limit": 100,
      "window": 60,
      "retry_after": 45,
      "current_usage": 105
    }
  }
}
```

**解决方案**:

- 等待指定时间后重试
- 降低请求频率
- 申请更高的配额限制

## 认证授权错误 (-310xx)

### -31000 Authentication Error

**描述**: 认证错误
**原因**:

- JWT Token 无效或过期
- Token 签名验证失败
- 缺少认证信息

**示例**:

```json
{
  "type": "response",
  "id": "auth-1",
  "error": {
    "code": -31000,
    "message": "认证失败",
    "data": {
      "reason": "token_expired",
      "expired_at": "2025-01-20T10:00:00Z",
      "current_time": "2025-01-20T10:30:00Z"
    }
  }
}
```

**解决方案**:

- 刷新或重新获取 Token
- 检查 Token 格式和签名
- 确认系统时间同步

### -31001 Authorization Error

**描述**: 授权错误
**原因**:

- 用户权限不足
- 角色不匹配
- 资源访问被拒绝

**示例**:

```json
{
  "type": "response",
  "id": "query-1",
  "error": {
    "code": -31001,
    "message": "权限不足",
    "data": {
      "required_permission": "table:users:read",
      "user_permissions": ["table:orders:read"],
      "user_roles": ["customer"]
    }
  }
}
```

**解决方案**:

- 联系管理员申请相应权限
- 检查用户角色配置
- 使用有权限的账户操作

## 会话管理错误 (-311xx)

### -31100 Session Not Found

**描述**: 会话不存在
**原因**:

- 会话 ID 无效
- 会话已过期
- 会话已被删除

**示例**:

```json
{
  "type": "response",
  "id": "session-get-1",
  "error": {
    "code": -31100,
    "message": "会话不存在",
    "data": {
      "session_id": "session-invalid",
      "possible_reasons": ["会话已过期", "会话ID格式错误", "会话已被手动删除"]
    }
  }
}
```

**解决方案**:

- 创建新的会话
- 检查会话 ID 格式
- 确认会话未过期

### -31101 Session Expired

**描述**: 会话已过期
**原因**:

- 会话超过有效期
- 长时间未活动
- 系统重启导致会话丢失

**示例**:

```json
{
  "type": "response",
  "id": "query-1",
  "error": {
    "code": -31101,
    "message": "会话已过期",
    "data": {
      "session_id": "session-123",
      "expired_at": "2025-01-20T10:00:00Z",
      "max_idle_time": 3600
    }
  }
}
```

**解决方案**:

- 创建新的会话
- 增加会话有效期配置
- 实现会话自动续期机制

### -31102 Session Limit Exceeded

**描述**: 会话数量超限
**原因**:

- 用户会话数超过限制
- 系统总会话数达到上限
- 内存资源不足

**示例**:

```json
{
  "type": "response",
  "id": "session-create-1",
  "error": {
    "code": -31102,
    "message": "会话数量超限",
    "data": {
      "user_id": "user-123",
      "current_sessions": 5,
      "max_sessions": 5,
      "oldest_session": "session-456"
    }
  }
}
```

**解决方案**:

- 关闭不需要的会话
- 增加会话限制配置
- 实现会话自动清理

## 数据库相关错误 (-312xx)

### -31200 Database Connection Error

**描述**: 数据库连接错误
**原因**:

- 数据库服务不可用
- 连接池耗尽
- 网络连接问题

**示例**:

```json
{
  "type": "response",
  "id": "query-1",
  "error": {
    "code": -31200,
    "message": "数据库连接失败",
    "data": {
      "host": "mysql.example.com",
      "port": 3306,
      "error": "connection refused",
      "retry_count": 3
    }
  }
}
```

**解决方案**:

- 检查数据库服务状态
- 验证连接配置
- 检查网络连通性

### -31201 Database Query Error

**描述**: 数据库查询错误
**原因**:

- SQL 语法错误
- 表或字段不存在
- 数据类型不匹配

**示例**:

```json
{
  "type": "response",
  "id": "query-1",
  "error": {
    "code": -31201,
    "message": "数据库查询错误",
    "data": {
      "sql": "SELECT * FROM non_existent_table",
      "mysql_error": "Table 'db.non_existent_table' doesn't exist",
      "mysql_errno": 1146,
      "position": 14
    }
  }
}
```

**解决方案**:

- 检查 SQL 语法
- 验证表和字段名
- 查看数据库 Schema

### -31202 Database Transaction Error

**描述**: 数据库事务错误
**原因**:

- 事务冲突
- 死锁检测
- 事务超时

**示例**:

```json
{
  "type": "response",
  "id": "query-1",
  "error": {
    "code": -31202,
    "message": "数据库事务错误",
    "data": {
      "error_type": "deadlock",
      "mysql_error": "Deadlock found when trying to get lock",
      "mysql_errno": 1213,
      "affected_tables": ["users", "orders"]
    }
  }
}
```

**解决方案**:

- 重试事务操作
- 优化事务逻辑
- 检查索引设计

## 查询处理错误 (-313xx)

### -31300 Query Parse Error

**描述**: 查询解析错误
**原因**:

- 自然语言查询无法理解
- 查询意图不明确
- 缺少必要的上下文信息

**示例**:

```json
{
  "type": "response",
  "id": "query-1",
  "error": {
    "code": -31300,
    "message": "查询解析错误",
    "data": {
      "query": "查询那个东西",
      "reason": "查询意图不明确",
      "suggestions": [
        "请指定要查询的表或数据类型",
        "提供更具体的查询条件",
        "使用完整的句子描述查询需求"
      ]
    }
  }
}
```

**解决方案**:

- 使用更具体的查询描述
- 提供必要的上下文信息
- 参考查询示例

### -31301 SQL Generation Error

**描述**: SQL 生成错误
**原因**:

- LLM 无法生成有效 SQL
- Schema 信息不完整
- 查询过于复杂

**示例**:

```json
{
  "type": "response",
  "id": "query-1",
  "error": {
    "code": -31301,
    "message": "SQL 生成失败",
    "data": {
      "query": "查询用户和订单的复杂关联统计",
      "reason": "无法确定表之间的关联关系",
      "missing_info": ["users 和 orders 表的关联字段", "统计的具体维度"]
    }
  }
}
```

**解决方案**:

- 简化查询复杂度
- 提供更多上下文信息
- 检查 Schema 完整性

### -31302 Query Validation Error

**描述**: 查询验证错误
**原因**:

- 生成的 SQL 不安全
- 包含危险操作
- 违反安全策略

**示例**:

```json
{
  "type": "response",
  "id": "query-1",
  "error": {
    "code": -31302,
    "message": "查询验证失败",
    "data": {
      "sql": "DROP TABLE users",
      "violation": "dangerous_operation",
      "policy": "只允许 SELECT 操作"
    }
  }
}
```

**解决方案**:

- 修改查询避免危险操作
- 检查安全策略配置
- 使用只读权限账户

## Schema 相关错误 (-314xx)

### -31400 Schema Load Error

**描述**: Schema 加载错误
**原因**:

- 无法连接到数据库
- 权限不足读取 Schema
- Schema 信息损坏

**示例**:

```json
{
  "type": "response",
  "id": "schema-1",
  "error": {
    "code": -31400,
    "message": "Schema 加载失败",
    "data": {
      "database": "test_db",
      "error": "access denied for user 'readonly'@'%' to database 'information_schema'",
      "required_privileges": ["SELECT on information_schema.*"]
    }
  }
}
```

**解决方案**:

- 检查数据库连接权限
- 授予 Schema 读取权限
- 验证数据库配置

### -31401 Table Not Found

**描述**: 表不存在
**原因**:

- 表名错误
- 表已被删除
- 权限不足访问表

**示例**:

```json
{
  "type": "response",
  "id": "schema-1",
  "error": {
    "code": -31401,
    "message": "表不存在",
    "data": {
      "table": "user_profiles",
      "database": "test_db",
      "similar_tables": ["users", "user_settings", "profiles"]
    }
  }
}
```

**解决方案**:

- 检查表名拼写
- 确认表是否存在
- 检查访问权限

### -31402 Schema Cache Error

**描述**: Schema 缓存错误
**原因**:

- 缓存服务不可用
- 缓存数据损坏
- 缓存过期处理失败

**示例**:

```json
{
  "type": "response",
  "id": "schema-1",
  "error": {
    "code": -31402,
    "message": "Schema 缓存错误",
    "data": {
      "cache_key": "schema:test_db",
      "error": "redis connection failed",
      "fallback": "direct_database_query"
    }
  }
}
```

**解决方案**:

- 检查缓存服务状态
- 清理损坏的缓存数据
- 配置缓存故障转移

## LLM 相关错误 (-315xx)

### -31500 LLM Connection Error

**描述**: LLM 连接错误
**原因**:

- LLM 服务不可用
- API 密钥无效
- 网络连接问题

**示例**:

```json
{
  "type": "response",
  "id": "query-1",
  "error": {
    "code": -31500,
    "message": "LLM 服务连接失败",
    "data": {
      "provider": "openai",
      "model": "gpt-4",
      "error": "invalid api key",
      "endpoint": "https://api.openai.com/v1/chat/completions"
    }
  }
}
```

**解决方案**:

- 检查 API 密钥配置
- 验证 LLM 服务状态
- 检查网络连接

### -31501 LLM Rate Limit Error

**描述**: LLM 限流错误
**原因**:

- API 调用频率超限
- Token 配额用完
- 并发请求过多

**示例**:

```json
{
  "type": "response",
  "id": "query-1",
  "error": {
    "code": -31501,
    "message": "LLM 服务限流",
    "data": {
      "provider": "openai",
      "limit_type": "requests_per_minute",
      "limit": 60,
      "retry_after": 30
    }
  }
}
```

**解决方案**:

- 等待指定时间后重试
- 升级 API 配额
- 实现请求队列机制

### -31502 LLM Response Error

**描述**: LLM 响应错误
**原因**:

- LLM 返回无效响应
- 响应格式不正确
- 内容过滤触发

**示例**:

```json
{
  "type": "response",
  "id": "query-1",
  "error": {
    "code": -31502,
    "message": "LLM 响应错误",
    "data": {
      "provider": "openai",
      "error": "content_filter",
      "reason": "The response was filtered due to content policy"
    }
  }
}
```

**解决方案**:

- 调整查询内容
- 检查内容策略
- 重新生成响应

## 缓存相关错误 (-316xx)

### -31600 Cache Connection Error

**描述**: 缓存连接错误
**原因**:

- Redis 服务不可用
- 连接配置错误
- 网络连接问题

**示例**:

```json
{
  "type": "response",
  "id": "query-1",
  "error": {
    "code": -31600,
    "message": "缓存服务连接失败",
    "data": {
      "host": "redis.example.com",
      "port": 6379,
      "error": "connection timeout",
      "fallback": "direct_query"
    }
  }
}
```

**解决方案**:

- 检查 Redis 服务状态
- 验证连接配置
- 配置缓存故障转移

### -31601 Cache Operation Error

**描述**: 缓存操作错误
**原因**:

- 缓存写入失败
- 缓存读取错误
- 缓存空间不足

**示例**:

```json
{
  "type": "response",
  "id": "query-1",
  "error": {
    "code": -31601,
    "message": "缓存操作失败",
    "data": {
      "operation": "set",
      "key": "query:hash123",
      "error": "out of memory",
      "cache_usage": "95%"
    }
  }
}
```

**解决方案**:

- 清理过期缓存数据
- 增加缓存容量
- 优化缓存策略

## 监控相关错误 (-317xx)

### -31700 Metrics Collection Error

**描述**: 指标收集错误
**原因**:

- 监控服务不可用
- 指标格式错误
- 存储空间不足

**示例**:

```json
{
  "type": "response",
  "id": "metrics-1",
  "error": {
    "code": -31700,
    "message": "指标收集失败",
    "data": {
      "metric": "query_duration",
      "error": "prometheus server unavailable",
      "endpoint": "http://prometheus:9090"
    }
  }
}
```

**解决方案**:

- 检查监控服务状态
- 验证指标格式
- 清理历史数据

## 故障排查流程

### 1. 错误信息收集

当遇到错误时，首先收集以下信息：

```bash
# 查看服务日志
tail -f /var/log/rag/app.log

# 检查系统资源
top
df -h
free -m

# 查看网络连接
netstat -tlnp | grep :8080

# 检查数据库连接
mysql -h localhost -u username -p -e "SELECT 1"
```

### 2. 错误分析步骤

1. **确定错误类型**: 根据错误码确定问题所属模块
2. **查看详细信息**: 检查 error.data 中的详细错误信息
3. **检查系统状态**: 验证相关服务是否正常运行
4. **查看日志记录**: 分析服务器端日志获取更多上下文
5. **尝试解决方案**: 根据错误码说明尝试相应的解决方案

### 3. 常用调试命令

```bash
# 检查服务状态
systemctl status rag-server

# 测试数据库连接
mysql -h $DB_HOST -u $DB_USER -p$DB_PASSWORD -e "SHOW DATABASES;"

# 测试 Redis 连接
redis-cli -h $REDIS_HOST -p $REDIS_PORT ping

# 检查端口占用
lsof -i :8080

# 查看进程信息
ps aux | grep rag

# 检查配置文件
cat /etc/rag/config.yaml
```

### 4. 日志分析

日志级别和含义：

- **DEBUG**: 详细的调试信息
- **INFO**: 一般信息记录
- **WARN**: 警告信息，不影响正常功能
- **ERROR**: 错误信息，影响部分功能
- **FATAL**: 致命错误，导致服务停止

日志格式示例：

```json
{
  "timestamp": "2025-01-20T10:30:00Z",
  "level": "ERROR",
  "component": "query_processor",
  "message": "SQL generation failed",
  "error": "LLM service unavailable",
  "request_id": "req-123456",
  "user_id": "user-789",
  "query": "查询用户信息"
}
```

### 5. 性能问题排查

当遇到性能问题时：

1. **检查响应时间**: 分析各组件的响应时间
2. **查看资源使用**: 监控 CPU、内存、磁盘使用情况
3. **分析数据库性能**: 检查慢查询日志
4. **检查缓存命中率**: 分析缓存效果
5. **监控网络延迟**: 检查网络连接质量

### 6. 联系技术支持

如果无法自行解决问题，请提供以下信息：

- 错误码和完整错误消息
- 相关的日志片段
- 系统环境信息
- 重现问题的步骤
- 配置文件内容（敏感信息需脱敏）

## 预防措施

### 1. 监控告警

设置以下监控告警：

- 错误率超过阈值
- 响应时间过长
- 系统资源使用率过高
- 依赖服务不可用

### 2. 健康检查

定期执行健康检查：

```bash
# API 健康检查
curl -f http://localhost:8080/health

# 数据库连接检查
curl -f http://localhost:8080/health/database

# 缓存服务检查
curl -f http://localhost:8080/health/cache
```

### 3. 备份策略

- 定期备份数据库
- 备份配置文件
- 保留日志文件
- 制定恢复计划

### 4. 容量规划

- 监控系统负载趋势
- 预估资源需求增长
- 制定扩容计划
- 优化系统性能

通过遵循这些故障排查指南，可以快速定位和解决系统中的各种问题，确保 RAG MySQL 查询系统的稳定运行。
