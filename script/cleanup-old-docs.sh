#!/bin/bash

# 清理旧文档目录的脚本
# 在确认文档已成功转移到 Writerside 后运行

echo "正在清理旧的文档目录..."

# 备份原始 docs 目录（可选）
if [ -d "../docs" ]; then
    echo "创建 docs 目录的备份..."
    tar -czf "docs-backup-$(date +%Y%m%d-%H%M%S).tar.gz" docs/
    echo "备份已创建"
fi

# 删除原始 docs 目录
if [ -d "../docs" ]; then
    echo "删除原始 docs 目录..."
    rm -rf ../docs/
    echo "docs 目录已删除"
fi

echo "清理完成！"
echo "所有文档现在都在 Writerside/topics/ 目录中"
echo ""
echo "目录结构："
echo "Writerside/topics/"
echo "├── api/                    # API 文档"
echo "├── development/            # 开发文档"
echo "├── examples/               # 使用示例"
echo "│   ├── clients/           # 客户端集成示例"
echo "│   ├── deployment/        # 部署配置示例"
echo "│   └── queries/           # 查询示例"
echo "├── modules/               # 模块文档"
echo "├── overview.md            # 系统概览"
echo "├── implementation.md      # 实现详情"
echo "├── performance-testing.md # 性能测试"
echo "└── documentation-index.md # 文档索引"