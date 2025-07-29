# 使用示例

本目录包含 RAG MySQL 查询系统的各种使用示例，涵盖不同场景和用例。

## 示例分类

### 基础使用

- [基本查询示例](basic-usage.md) - 简单的查询操作

### 查询场景

- [简单查询](queries/simple-queries.md) - 基础的数据查询
- [复杂查询](queries/complex-queries.md) - 多表关联和聚合查询
- [统计分析查询](queries/analytics.md) - 数据统计和分析

### 客户端集成

- [JavaScript 客户端](clients/javascript.md) - Web 前端集成
- [Python 客户端](clients/python.md) - Python 应用集成
- [Go 客户端](clients/go.md) - Go 应用集成

### 部署配置

- [配置管理](deployment/configuration.md) - 配置文件管理和部署最佳实践

## 快速开始

如果你是第一次使用，建议按以下顺序阅读：

1. [基本查询示例](basic-usage.md) - 了解基本用法
2. [简单查询](queries/simple-queries.md) - 掌握查询技巧
3. [客户端集成](clients/) - 选择适合的客户端语言

## 系统概述

RAG MySQL 查询系统是一个基于 LangChain 和 MCP (Model Context Protocol) 协议的智能数据库查询系统。它能够：

- 将自然语言查询转换为 SQL 语句
- 智能分析数据库 Schema 结构
- 提供查询结果的自然语言解释
- 支持多用户会话管理
- 提供完整的安全和权限控制

## 主要特性

- **智能查询转换**: 使用大语言模型将自然语言转换为准确的 SQL 查询
- **Schema 感知**: 自动分析数据库结构，提供智能的表关系推断
- **多层缓存**: 支持 Schema 缓存和查询结果缓存，提升性能
- **会话管理**: 支持多用户会话，维护查询历史和上下文
- **安全控制**: 完整的认证授权机制和 SQL 注入防护
- **监控告警**: 全面的性能监控和健康检查
- **MCP 协议**: 标准化的通信协议，易于集成

## 技术架构

系统采用模块化设计，主要组件包括：

- **MCP 服务层**: 处理 MCP 协议通信
- **RAG 核心层**: 查询解析、SQL 生成、结果处理
- **LangChain 组件层**: LLM 集成、提示模板、工具管理
- **数据访问层**: MySQL 连接、Schema 管理、查询执行
- **缓存层**: Redis 缓存、内存缓存
- **监控层**: 日志记录、指标收集、健康检查

## 支持的功能

### 查询类型

- 简单查询 (SELECT)
- 复杂关联查询 (JOIN)
- 聚合查询 (GROUP BY, HAVING)
- 子查询和嵌套查询
- 时间序列查询

### 数据库支持

- MySQL 5.7+
- MySQL 8.0+
- 支持多种字符集和排序规则

### 客户端支持

- 标准 MCP 客户端
- WebSocket 连接
- HTTP REST API (通过 MCP 桥接)

## 许可证

本项目采用 MIT 许可证，详见 [LICENSE](../LICENSE) 文件。
