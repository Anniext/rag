// 本文件实现了配置管理器，负责加载、解析、验证和管理应用的各类配置，包括服务器、数据库、LLM、缓存、日志、安全等模块。
// 支持从配置文件和环境变量读取配置，并提供默认值和热更新机制。
// 主要功能：
// 1. 配置文件和环境变量的加载与优先级处理。
// 2. 默认配置值的设置，保证系统可用性。
// 3. 配置解析到结构体，便于类型安全访问。
// 4. 配置验证，防止错误配置导致系统异常。
// 5. 支持多路径查找和灵活扩展。
// 6. 多环境配置支持（development、staging、production）。
// 7. 配置热更新和变更通知机制。
// 8. 配置备份和恢复功能。

package config

import (
	"context" // 上下文管理
	"encoding/json"
	"github.com/Anniext/rag/core"

	// JSON 处理
	"errors"        // 错误处理
	"fmt"           // 格式化输出
	"os"            // 操作系统相关
	"path/filepath" // 路径处理
	"strings"       // 字符串处理
	"sync"          // 同步原语
	"time"          // 时间处理

	"github.com/fsnotify/fsnotify" // 文件变更通知
	"github.com/spf13/viper"       // 配置库
)

// Environment 环境类型
type Environment string

const (
	Development Environment = "development"
	Staging     Environment = "staging"
	Production  Environment = "production"
)

// ChangeEvent 配置变更事件
type ChangeEvent struct {
	Key      string      `json:"key"`       // 变更的配置键
	OldValue interface{} `json:"old_value"` // 旧值
	NewValue interface{} `json:"new_value"` // 新值
	Time     time.Time   `json:"time"`      // 变更时间
}

// ChangeHandler 配置变更处理函数
type ChangeHandler func(event ChangeEvent) error

// Manager 配置管理器，封装 viper 并持有解析后的配置结构体。
type Manager struct {
	config       *core.Config               // 解析后的配置结构体，供业务使用
	viper        *viper.Viper               // viper 实例，负责底层配置读取
	environment  Environment                // 当前环境
	configPath   string                     // 配置文件路径
	handlers     map[string][]ChangeHandler // 配置变更处理器
	mu           sync.RWMutex               // 读写锁
	watchCancel  context.CancelFunc         // 监听取消函数
	backupConfig *core.Config               // 备份配置
}

// NewManager 创建配置管理器实例，初始化 viper。
func NewManager() *Manager {
	return &Manager{
		viper:    viper.New(),                      // 新建 viper 实例
		handlers: make(map[string][]ChangeHandler), // 初始化处理器映射
	}
}

// NewManagerWithEnvironment 创建指定环境的配置管理器
func NewManagerWithEnvironment(env Environment) *Manager {
	manager := NewManager()
	manager.environment = env
	return manager
}

// Load 加载配置文件和环境变量，并解析到结构体。
// configPath: 指定配置文件路径，若为空则按默认路径查找。
// 返回 error 表示加载或解析失败。
func (m *Manager) Load(configPath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 自动检测环境
	if m.environment == "" {
		m.environment = m.detectEnvironment()
	}

	// 设置配置文件路径
	if configPath != "" {
		m.configPath = configPath
		m.viper.SetConfigFile(configPath) // 使用指定路径
	} else {
		// 根据环境加载不同的配置文件
		configName := m.getConfigName()
		m.viper.SetConfigName(configName)
		m.viper.SetConfigType("yaml")
		m.viper.AddConfigPath("./config")
		m.viper.AddConfigPath("./rag/config")
		m.viper.AddConfigPath(".")

		// 记录实际使用的配置文件路径
		m.configPath = fmt.Sprintf("config/%s.yaml", configName)
	}

	// 设置环境变量前缀，自动映射
	m.viper.SetEnvPrefix("RAG")
	m.viper.AutomaticEnv()
	m.viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// 设置默认值，保证配置项完整
	m.setDefaults()

	// 读取基础配置文件
	if err := m.loadBaseConfig(); err != nil {
		return err
	}

	// 读取环境特定配置文件
	if err := m.loadEnvironmentConfig(); err != nil {
		return err
	}

	// 解析配置到结构体，便于类型安全访问
	config := &core.Config{}
	if err := m.viper.Unmarshal(config); err != nil {
		return fmt.Errorf("解析配置失败: %w", err)
	}

	// 验证配置合法性，防止错误配置
	if err := m.validateConfig(config); err != nil {
		return fmt.Errorf("配置验证失败: %w", err)
	}

	// 备份当前配置
	if m.config != nil {
		m.backupConfig = m.deepCopyConfig(m.config)
	}

	m.config = config // 保存解析后的配置
	return nil
}

// detectEnvironment 自动检测运行环境
func (m *Manager) detectEnvironment() Environment {
	env := os.Getenv("RAG_ENV")
	if env == "" {
		env = os.Getenv("ENV")
	}
	if env == "" {
		env = os.Getenv("GO_ENV")
	}

	switch strings.ToLower(env) {
	case "development", "dev":
		return Development
	case "staging", "stage":
		return Staging
	case "production", "prod":
		return Production
	default:
		return Development // 默认为开发环境
	}
}

// getConfigName 根据环境获取配置文件名
func (m *Manager) getConfigName() string {
	switch m.environment {
	case Development:
		return "rag.development"
	case Staging:
		return "rag.staging"
	case Production:
		return "rag.production"
	default:
		return "rag"
	}
}

// loadBaseConfig 加载基础配置文件
func (m *Manager) loadBaseConfig() error {
	// 先尝试加载基础配置文件
	baseViper := viper.New()
	baseViper.SetConfigName("rag")
	baseViper.SetConfigType("yaml")
	baseViper.AddConfigPath("./config")
	baseViper.AddConfigPath("./rag/config")
	baseViper.AddConfigPath(".")

	if err := baseViper.ReadInConfig(); err != nil {
		var configFileNotFoundError viper.ConfigFileNotFoundError
		if !errors.As(err, &configFileNotFoundError) {
			return fmt.Errorf("读取基础配置文件失败: %w", err)
		}
		// 基础配置文件不存在时继续
	} else {
		// 合并基础配置
		if err := m.viper.MergeConfigMap(baseViper.AllSettings()); err != nil {
			return fmt.Errorf("合并基础配置失败: %w", err)
		}
	}

	return nil
}

// loadEnvironmentConfig 加载环境特定配置文件
func (m *Manager) loadEnvironmentConfig() error {
	// 读取环境特定配置文件
	if err := m.viper.ReadInConfig(); err != nil {
		var configFileNotFoundError viper.ConfigFileNotFoundError
		if !errors.As(err, &configFileNotFoundError) {
			return fmt.Errorf("读取环境配置文件失败: %w", err)
		}
		// 环境配置文件不存在时使用默认配置
	}

	return nil
}

// deepCopyConfig 深拷贝配置对象
func (m *Manager) deepCopyConfig(config *core.Config) *core.Config {
	data, _ := json.Marshal(config)
	var copy core.Config
	json.Unmarshal(data, &copy)
	return &copy
}

// setDefaults 设置各模块的默认配置值，保证系统启动时有合理参数。
func (m *Manager) setDefaults() {
	// 服务器配置
	m.viper.SetDefault("server.host", "0.0.0.0")      // 监听地址
	m.viper.SetDefault("server.port", 8080)           // 端口
	m.viper.SetDefault("server.read_timeout", "30s")  // 读超时
	m.viper.SetDefault("server.write_timeout", "30s") // 写超时

	// 数据库配置
	m.viper.SetDefault("database.driver", "mysql")          // 数据库类型
	m.viper.SetDefault("database.host", "localhost")        // 主机
	m.viper.SetDefault("database.port", 3306)               // 端口
	m.viper.SetDefault("database.username", "root")         // 用户名
	m.viper.SetDefault("database.password", "")             // 密码
	m.viper.SetDefault("database.database", "pumppill_rag") // 数据库名
	m.viper.SetDefault("database.max_open_conns", 100)      // 最大连接数
	m.viper.SetDefault("database.max_idle_conns", 10)       // 最大空闲连接
	m.viper.SetDefault("database.conn_max_lifetime", "1h")  // 连接最大生命周期

	// LLM 配置
	m.viper.SetDefault("llm.provider", "openai") // LLM 服务商
	m.viper.SetDefault("llm.model", "gpt-4")     // 模型名称
	m.viper.SetDefault("llm.temperature", 0.1)   // 采样温度
	m.viper.SetDefault("llm.max_tokens", 2048)   // 最大生成 token 数

	// 缓存配置
	m.viper.SetDefault("cache.type", "redis")           // 缓存类型
	m.viper.SetDefault("cache.host", "localhost")       // 缓存主机
	m.viper.SetDefault("cache.port", 6379)              // 缓存端口
	m.viper.SetDefault("cache.password", "")            // 缓存密码
	m.viper.SetDefault("cache.database", 0)             // 缓存数据库编号
	m.viper.SetDefault("cache.schema_ttl", "1h")        // schema 缓存时间
	m.viper.SetDefault("cache.query_result_ttl", "10m") // 查询结果缓存时间
	m.viper.SetDefault("cache.max_cache_size", "100MB") // 最大缓存空间

	// 日志配置
	m.viper.SetDefault("log.level", "info")             // 日志级别
	m.viper.SetDefault("log.format", "json")            // 日志格式
	m.viper.SetDefault("log.output", "stdout")          // 日志输出位置
	m.viper.SetDefault("log.file_path", "logs/rag.log") // 日志文件路径
	m.viper.SetDefault("log.max_size", 100)             // 单文件最大大小
	m.viper.SetDefault("log.max_backups", 3)            // 最大备份数
	m.viper.SetDefault("log.max_age", 7)                // 最大保存天数

	// 安全配置
	m.viper.SetDefault("security.jwt_secret", "default-secret-key") // JWT 密钥
	m.viper.SetDefault("security.token_expiry", "24h")              // token 有效期
	m.viper.SetDefault("security.enable_rbac", true)                // 是否启用 RBAC
	m.viper.SetDefault("security.allowed_origins", []string{"*"})   // 允许跨域来源

	// MCP 配置
	m.viper.SetDefault("mcp.host", "0.0.0.0")       // MCP 服务地址
	m.viper.SetDefault("mcp.port", 8081)            // MCP 服务端口
	m.viper.SetDefault("mcp.timeout", "30s")        // MCP 超时时间
	m.viper.SetDefault("mcp.max_connections", 1000) // MCP 最大连接数
}

// validateConfig 验证配置，确保必需项存在且合法。
func (m *Manager) validateConfig(config *core.Config) error {
	// 验证服务器配置
	if config.Server.Port <= 0 || config.Server.Port > 65535 {
		return fmt.Errorf("无效的服务器端口: %d", config.Server.Port)
	}

	// 验证数据库配置
	if config.Database.Host == "" {
		return fmt.Errorf("数据库主机不能为空")
	}
	if config.Database.Username == "" {
		return fmt.Errorf("数据库用户名不能为空")
	}
	if config.Database.Database == "" {
		return fmt.Errorf("数据库名不能为空")
	}

	// 验证 LLM 配置
	if config.LLM.Provider == "" {
		return fmt.Errorf("LLM 提供商不能为空")
	}
	if config.LLM.Model == "" {
		return fmt.Errorf("LLM 模型不能为空")
	}
	if config.LLM.Temperature < 0 || config.LLM.Temperature > 2 {
		return fmt.Errorf("无效的 LLM 温度值: %f", config.LLM.Temperature)
	}

	// 验证缓存配置
	if config.Cache.Type != "redis" && config.Cache.Type != "memory" {
		return fmt.Errorf("不支持的缓存类型: %s", config.Cache.Type)
	}

	// 验证日志配置
	validLogLevels := []string{"debug", "info", "warn", "error", "fatal"}
	if !contains(validLogLevels, config.Log.Level) {
		return fmt.Errorf("无效的日志级别: %s", config.Log.Level)
	}

	// 验证安全配置
	if config.Security.JWTSecret == "" || config.Security.JWTSecret == "default-secret-key" {
		return fmt.Errorf("JWT 密钥不能为空或使用默认值")
	}

	return nil
}

// GetConfig 获取解析后的配置结构体。
func (m *Manager) GetConfig() *core.Config {
	return m.config
}

// GetString 获取字符串类型配置项。
func (m *Manager) GetString(key string) string {
	return m.viper.GetString(key)
}

// GetInt 获取整数类型配置项。
func (m *Manager) GetInt(key string) int {
	return m.viper.GetInt(key)
}

// GetBool 获取布尔类型配置项。
func (m *Manager) GetBool(key string) bool {
	return m.viper.GetBool(key)
}

// GetDuration 获取时间间隔类型配置项。
func (m *Manager) GetDuration(key string) time.Duration {
	return m.viper.GetDuration(key)
}

// Watch 监听配置文件变化，支持热更新。
// ctx: 上下文，用于取消监听
func (m *Manager) Watch(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 如果已经在监听，先取消
	if m.watchCancel != nil {
		m.watchCancel()
	}

	// 创建新的上下文
	watchCtx, cancel := context.WithCancel(ctx)
	m.watchCancel = cancel

	// 启动配置文件监听
	m.viper.WatchConfig()
	m.viper.OnConfigChange(func(e fsnotify.Event) {
		select {
		case <-watchCtx.Done():
			return
		default:
			m.handleConfigChange(e)
		}
	})

	return nil
}

// handleConfigChange 处理配置文件变更
func (m *Manager) handleConfigChange(e fsnotify.Event) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 保存旧配置
	oldConfig := m.deepCopyConfig(m.config)

	// 重新加载配置
	config := &core.Config{}
	if err := m.viper.Unmarshal(config); err != nil {
		fmt.Printf("配置热更新失败，解析配置错误: %v\n", err)
		return
	}

	// 验证新配置
	if err := m.validateConfig(config); err != nil {
		fmt.Printf("配置热更新失败，配置验证错误: %v\n", err)
		return
	}

	// 检测配置变更
	changes := m.detectChanges(oldConfig, config)
	if len(changes) == 0 {
		return // 没有实际变更
	}

	// 更新配置
	m.config = config

	// 触发变更处理器
	for _, change := range changes {
		m.notifyHandlers(change)
	}

	fmt.Printf("配置热更新成功，共 %d 项变更\n", len(changes))
}

// detectChanges 检测配置变更
func (m *Manager) detectChanges(oldConfig, newConfig *core.Config) []ChangeEvent {
	var changes []ChangeEvent
	now := time.Now()

	// 比较各个配置项
	if oldConfig.Server != newConfig.Server {
		changes = append(changes, ChangeEvent{
			Key:      "server",
			OldValue: oldConfig.Server,
			NewValue: newConfig.Server,
			Time:     now,
		})
	}

	if oldConfig.Database != newConfig.Database {
		changes = append(changes, ChangeEvent{
			Key:      "database",
			OldValue: oldConfig.Database,
			NewValue: newConfig.Database,
			Time:     now,
		})
	}

	if oldConfig.LLM != newConfig.LLM {
		changes = append(changes, ChangeEvent{
			Key:      "llm",
			OldValue: oldConfig.LLM,
			NewValue: newConfig.LLM,
			Time:     now,
		})
	}

	if oldConfig.Cache != newConfig.Cache {
		changes = append(changes, ChangeEvent{
			Key:      "cache",
			OldValue: oldConfig.Cache,
			NewValue: newConfig.Cache,
			Time:     now,
		})
	}

	if oldConfig.Log != newConfig.Log {
		changes = append(changes, ChangeEvent{
			Key:      "log",
			OldValue: oldConfig.Log,
			NewValue: newConfig.Log,
			Time:     now,
		})
	}

	if oldConfig.Security != newConfig.Security {
		changes = append(changes, ChangeEvent{
			Key:      "security",
			OldValue: oldConfig.Security,
			NewValue: newConfig.Security,
			Time:     now,
		})
	}

	if oldConfig.MCP != newConfig.MCP {
		changes = append(changes, ChangeEvent{
			Key:      "mcp",
			OldValue: oldConfig.MCP,
			NewValue: newConfig.MCP,
			Time:     now,
		})
	}

	return changes
}

// notifyHandlers 通知配置变更处理器
func (m *Manager) notifyHandlers(change ChangeEvent) {
	handlers, exists := m.handlers[change.Key]
	if !exists {
		return
	}

	for _, handler := range handlers {
		if err := handler(change); err != nil {
			fmt.Printf("配置变更处理器执行失败: %v\n", err)
		}
	}
}

// RegisterChangeHandler 注册配置变更处理器
func (m *Manager) RegisterChangeHandler(key string, handler ChangeHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.handlers[key] == nil {
		m.handlers[key] = make([]ChangeHandler, 0)
	}
	m.handlers[key] = append(m.handlers[key], handler)
}

// UnregisterChangeHandler 注销配置变更处理器
func (m *Manager) UnregisterChangeHandler(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.handlers, key)
}

// StopWatch 停止配置监听
func (m *Manager) StopWatch() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.watchCancel != nil {
		m.watchCancel()
		m.watchCancel = nil
	}
}

// SaveConfig 保存当前配置到文件。
// configPath: 指定保存路径，若为空则默认为 "config/rag.yaml"。
func (m *Manager) SaveConfig(configPath string) error {
	if configPath == "" {
		configPath = "config/rag.yaml"
	}

	// 确保目录存在
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}

	// 写入配置文件
	return m.viper.WriteConfigAs(configPath)
}

// contains 检查切片是否包含指定元素
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// GetDatabaseDSN 获取数据库连接字符串
func (m *Manager) GetDatabaseDSN() string {
	db := m.config.Database
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		db.Username, db.Password, db.Host, db.Port, db.Database)
}

// GetRedisAddr 获取 Redis 地址
func (m *Manager) GetRedisAddr() string {
	cache := m.config.Cache
	return fmt.Sprintf("%s:%d", cache.Host, cache.Port)
}

// GetServerAddr 获取服务器地址
func (m *Manager) GetServerAddr() string {
	server := m.config.Server
	return fmt.Sprintf("%s:%d", server.Host, server.Port)
}

// GetMCPAddr 获取 MCP 服务器地址
func (m *Manager) GetMCPAddr() string {
	mcp := m.config.MCP
	return fmt.Sprintf("%s:%d", mcp.Host, mcp.Port)
}

// GetEnvironment 获取当前环境
func (m *Manager) GetEnvironment() Environment {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.environment
}

// SetEnvironment 设置环境
func (m *Manager) SetEnvironment(env Environment) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.environment = env
}

// Reload 重新加载配置
func (m *Manager) Reload() error {
	return m.Load(m.configPath)
}

// BackupConfig 备份当前配置
func (m *Manager) BackupConfig() *core.Config {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.config == nil {
		return nil
	}
	return m.deepCopyConfig(m.config)
}

// RestoreConfig 恢复配置
func (m *Manager) RestoreConfig() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.backupConfig == nil {
		return fmt.Errorf("没有可恢复的备份配置")
	}

	// 验证备份配置
	if err := m.validateConfig(m.backupConfig); err != nil {
		return fmt.Errorf("备份配置验证失败: %w", err)
	}

	// 恢复配置
	oldConfig := m.config
	m.config = m.deepCopyConfig(m.backupConfig)

	// 检测变更并通知
	changes := m.detectChanges(oldConfig, m.config)
	for _, change := range changes {
		m.notifyHandlers(change)
	}

	return nil
}

// ValidateConfigFile 验证配置文件
func (m *Manager) ValidateConfigFile(configPath string) error {
	tempViper := viper.New()
	tempViper.SetConfigFile(configPath)

	// 设置环境变量前缀
	tempViper.SetEnvPrefix("RAG")
	tempViper.AutomaticEnv()
	tempViper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// 设置默认值
	tempManager := &Manager{viper: tempViper}
	tempManager.setDefaults()

	// 读取配置文件
	if err := tempViper.ReadInConfig(); err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	// 解析配置
	config := &core.Config{}
	if err := tempViper.Unmarshal(config); err != nil {
		return fmt.Errorf("解析配置失败: %w", err)
	}

	// 验证配置
	return tempManager.validateConfig(config)
}

// GetConfigSummary 获取配置摘要信息
func (m *Manager) GetConfigSummary() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.config == nil {
		return nil
	}

	return map[string]interface{}{
		"environment":   m.environment,
		"config_path":   m.configPath,
		"server_addr":   m.GetServerAddr(),
		"database_host": m.config.Database.Host,
		"cache_type":    m.config.Cache.Type,
		"log_level":     m.config.Log.Level,
		"llm_provider":  m.config.LLM.Provider,
		"llm_model":     m.config.LLM.Model,
		"mcp_addr":      m.GetMCPAddr(),
		"loaded_at":     time.Now().Format(time.RFC3339),
	}
}

// ExportConfig 导出配置到指定格式
func (m *Manager) ExportConfig(format string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.config == nil {
		return nil, fmt.Errorf("配置未加载")
	}

	switch strings.ToLower(format) {
	case "json":
		return json.MarshalIndent(m.config, "", "  ")
	case "yaml":
		// 这里需要使用 yaml 库，暂时返回 JSON
		return json.MarshalIndent(m.config, "", "  ")
	default:
		return nil, fmt.Errorf("不支持的导出格式: %s", format)
	}
}

// UpdateConfig 动态更新配置项
func (m *Manager) UpdateConfig(key string, value interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 保存旧值
	oldValue := m.viper.Get(key)

	// 设置新值
	m.viper.Set(key, value)

	// 重新解析配置
	config := &core.Config{}
	if err := m.viper.Unmarshal(config); err != nil {
		// 恢复旧值
		m.viper.Set(key, oldValue)
		return fmt.Errorf("更新配置失败，解析错误: %w", err)
	}

	// 验证新配置
	if err := m.validateConfig(config); err != nil {
		// 恢复旧值
		m.viper.Set(key, oldValue)
		return fmt.Errorf("更新配置失败，验证错误: %w", err)
	}

	// 更新配置
	m.config = config

	// 通知变更
	change := ChangeEvent{
		Key:      key,
		OldValue: oldValue,
		NewValue: value,
		Time:     time.Now(),
	}
	m.notifyHandlers(change)

	return nil
}

// GetConfigValue 获取指定配置项的值
func (m *Manager) GetConfigValue(key string) interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.viper.Get(key)
}

// IsConfigLoaded 检查配置是否已加载
func (m *Manager) IsConfigLoaded() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config != nil
}
