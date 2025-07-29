# RAG (Retrieval-Augmented Generation) 模块

本模块实现了基于 LangChain 的 MySQL 数据库查询 RAG 系统，支持通过 MCP (Model Context Protocol) 协议提供智能数据库查询服务。

## 目录结构

```
rag/
├── README.md                    # 模块说明文档
├── config/                      # 配置管理
├── core/                        # 核心接口定义
├── schema/                      # 数据库 Schema 管理
├── langchain/                   # LangChain 集成组件
├── query/                       # 查询处理核心
├── mcp/                         # MCP 协议服务器
├── session/                     # 会话和内存管理
├── cache/                       # 缓存和性能优化
├── monitor/                     # 监控和日志系统
├── security/                    # 安全和权限控制
└── deploy/                      # 部署配置
```

## 功能特性

- 自然语言到 SQL 的智能转换
- 基于数据库 Schema 的智能建议
- MCP 协议支持
- 会话管理和上下文维护
- 多层缓存优化
- 完整的监控和日志系统
- 安全的权限控制

## 使用方式

详细的使用说明请参考各子模块的文档。
