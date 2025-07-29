# RAG MySQL 查询系统文档

本目录包含使用 JetBrains Writerside 构建的 RAG MySQL 查询系统完整文档。

## 文档结构

```
Writerside/
├── cfg/                        # Writerside 配置文件
├── images/                     # 文档图片资源
├── topics/                     # 文档内容
│   ├── api/                   # API 文档
│   │   ├── mcp-protocol.md    # MCP 协议详细说明
│   │   └── error-codes.md     # 错误码参考
│   ├── development/           # 开发文档
│   │   ├── README.md          # 开发指南
│   │   ├── architecture.md    # 系统架构
│   │   ├── components.md      # 组件说明
│   │   ├── coding-standards.md # 编码规范
│   │   ├── contributing.md    # 贡献指南
│   │   └── extensions.md      # 扩展开发
│   ├── examples/              # 使用示例
│   │   ├── clients/          # 客户端集成示例
│   │   │   ├── javascript.md # JavaScript 客户端
│   │   │   ├── python.md     # Python 客户端
│   │   │   └── go.md         # Go 客户端
│   │   ├── deployment/       # 部署配置示例
│   │   │   └── configuration.md # 配置管理
│   │   ├── queries/          # 查询示例
│   │   │   ├── simple-queries.md # 简单查询
│   │   │   ├── complex-queries.md # 复杂查询
│   │   │   └── analytics.md  # 统计分析
│   │   ├── basic-usage.md    # 基础使用示例
│   │   └── examples-overview.md # 示例概览
│   ├── modules/              # 模块文档
│   │   ├── modules-overview.md # 模块概览
│   │   ├── cache.md          # 缓存模块
│   │   ├── query.md          # 查询模块
│   │   ├── schema.md         # Schema 模块
│   │   ├── session.md        # 会话模块
│   │   ├── langchain.md      # LangChain 模块
│   │   ├── mcp.md            # MCP 模块
│   │   ├── monitor.md        # 监控模块
│   │   ├── security.md       # 安全模块
│   │   └── deploy.md         # 部署模块
│   ├── overview.md           # 系统概览（主页）
│   ├── implementation.md     # 实现详情
│   ├── performance-testing.md # 性能测试指南
│   └── documentation-index.md # 文档索引
├── rag.tree                   # 文档目录结构配置
├── writerside.cfg            # Writerside 主配置
└── README.md                 # 本文件
```

## 构建文档

### 使用 Writerside IDE

1. 使用 JetBrains Writerside 打开此目录
2. 在 IDE 中预览和编辑文档
3. 使用内置构建功能生成最终文档

### 使用命令行

```bash
# 安装 Writerside CLI（如果尚未安装）
# 参考：https://www.jetbrains.com/help/writerside/deploy-docs-to-github-pages.html

# 构建文档
writerside-docker-builder --input . --output ./build
```

## 文档编辑

### 添加新文档

1. 在 `topics/` 目录下创建新的 Markdown 文件
2. 在 `rag.tree` 文件中添加对应的 `<toc-element>` 条目
3. 更新相关的索引和概览文档

### 修改现有文档

1. 直接编辑 `topics/` 目录下的 Markdown 文件
2. 如果修改了文档标题，需要更新 `rag.tree` 中的引用
3. 更新相关的交叉引用链接

### 文档规范

- 使用标准的 Markdown 语法
- 代码块使用适当的语言标识符
- 图片放在 `images/` 目录下
- 内部链接使用相对路径
- 遵循统一的文档结构和风格

## 部署

文档可以部署到以下平台：

- **GitHub Pages**: 使用 GitHub Actions 自动构建和部署
- **内部服务器**: 构建静态文件后部署到 Web 服务器
- **文档托管平台**: 如 GitBook、Confluence 等

## 维护

- 定期检查和更新文档内容
- 确保代码示例与实际代码保持同步
- 及时修复文档中的错误和过时信息
- 收集用户反馈并改进文档质量

## 联系方式

如有文档相关问题，请：

1. 在项目仓库中提交 Issue
2. 联系文档维护团队
3. 参与文档改进讨论

---

最后更新：2025-01-29
