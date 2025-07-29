# 部署和运维工具

本目录包含 RAG 系统的部署和运维相关工具，提供服务的优雅启动和关闭、数据库迁移、性能调优和故障排查等功能。

## 功能模块

### 1. 服务生命周期管理 (lifecycle.go)

- 优雅启动机制
- 优雅关闭机制
- 信号处理
- 资源清理

### 2. 数据库迁移工具 (migration.go)

- 数据库初始化
- 版本迁移
- 回滚机制
- 迁移状态管理

### 3. 性能调优工具 (performance.go)

- 性能分析
- 资源监控
- 调优建议
- 基准测试

### 4. 故障排查工具 (troubleshoot.go)

- 日志分析
- 错误诊断
- 健康检查
- 系统状态检查

## 使用方法

### 启动服务

```bash
# 使用默认配置启动
./rag-server

# 指定配置文件启动
./rag-server -config config/rag.production.yaml

# 启用性能分析
./rag-server -profile -profile-port 6060
```

### 数据库迁移

```bash
# 初始化数据库
./rag-migrate init

# 执行迁移
./rag-migrate up

# 回滚迁移
./rag-migrate down

# 查看迁移状态
./rag-migrate status
```

### 性能调优

```bash
# 运行性能分析
./rag-perf analyze

# 生成调优报告
./rag-perf report

# 运行基准测试
./rag-perf benchmark
```

### 故障排查

```bash
# 检查系统状态
./rag-troubleshoot status

# 分析日志
./rag-troubleshoot logs

# 诊断问题
./rag-troubleshoot diagnose
```

## 配置说明

### 部署配置

```yaml
deploy:
  graceful_timeout: "30s"
  shutdown_timeout: "60s"
  health_check_interval: "10s"
  max_startup_time: "120s"

migration:
  auto_migrate: true
  backup_before_migrate: true
  migration_timeout: "300s"

performance:
  enable_profiling: false
  profile_port: 6060
  metrics_interval: "30s"

troubleshoot:
  log_level: "info"
  log_retention: "7d"
  diagnostic_timeout: "60s"
```

## 最佳实践

1. **优雅启动**

   - 按依赖顺序启动组件
   - 等待依赖服务就绪
   - 执行健康检查
   - 注册服务发现

2. **优雅关闭**

   - 停止接收新请求
   - 等待现有请求完成
   - 清理资源和连接
   - 注销服务发现

3. **数据库迁移**

   - 迁移前备份数据
   - 使用事务确保一致性
   - 验证迁移结果
   - 记录迁移日志

4. **性能监控**

   - 定期收集性能指标
   - 设置告警阈值
   - 分析性能趋势
   - 优化瓶颈组件

5. **故障处理**
   - 快速定位问题
   - 收集诊断信息
   - 执行恢复操作
   - 记录处理过程
