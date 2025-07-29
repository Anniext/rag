package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"pumppill/rag/core"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewManager 测试创建配置管理器
func TestNewManager(t *testing.T) {
	manager := NewManager()
	assert.NotNil(t, manager)
	assert.NotNil(t, manager.viper)
	assert.NotNil(t, manager.handlers)
	assert.Equal(t, Environment(""), manager.environment)
}

// TestNewManagerWithEnvironment 测试创建指定环境的配置管理器
func TestNewManagerWithEnvironment(t *testing.T) {
	manager := NewManagerWithEnvironment(Production)
	assert.NotNil(t, manager)
	assert.Equal(t, Production, manager.environment)
}

// TestDetectEnvironment 测试环境检测
func TestDetectEnvironment(t *testing.T) {
	manager := NewManager()

	// 测试默认环境
	env := manager.detectEnvironment()
	assert.Equal(t, Development, env)

	// 测试 RAG_ENV 环境变量
	os.Setenv("RAG_ENV", "production")
	defer os.Unsetenv("RAG_ENV")
	env = manager.detectEnvironment()
	assert.Equal(t, Production, env)

	// 测试 ENV 环境变量
	os.Unsetenv("RAG_ENV")
	os.Setenv("ENV", "staging")
	defer os.Unsetenv("ENV")
	env = manager.detectEnvironment()
	assert.Equal(t, Staging, env)

	// 测试 GO_ENV 环境变量
	os.Unsetenv("ENV")
	os.Setenv("GO_ENV", "dev")
	defer os.Unsetenv("GO_ENV")
	env = manager.detectEnvironment()
	assert.Equal(t, Development, env)
}

// TestGetConfigName 测试配置文件名获取
func TestGetConfigName(t *testing.T) {
	tests := []struct {
		env      Environment
		expected string
	}{
		{Development, "rag.development"},
		{Staging, "rag.staging"},
		{Production, "rag.production"},
		{Environment("unknown"), "rag"},
	}

	for _, test := range tests {
		manager := NewManagerWithEnvironment(test.env)
		result := manager.getConfigName()
		assert.Equal(t, test.expected, result)
	}
}

// TestSetDefaults 测试默认值设置
func TestSetDefaults(t *testing.T) {
	manager := NewManager()
	manager.setDefaults()

	// 验证服务器默认配置
	assert.Equal(t, "0.0.0.0", manager.viper.GetString("server.host"))
	assert.Equal(t, 8080, manager.viper.GetInt("server.port"))
	assert.Equal(t, "30s", manager.viper.GetString("server.read_timeout"))

	// 验证数据库默认配置
	assert.Equal(t, "mysql", manager.viper.GetString("database.driver"))
	assert.Equal(t, "localhost", manager.viper.GetString("database.host"))
	assert.Equal(t, 3306, manager.viper.GetInt("database.port"))

	// 验证 LLM 默认配置
	assert.Equal(t, "openai", manager.viper.GetString("llm.provider"))
	assert.Equal(t, "gpt-4", manager.viper.GetString("llm.model"))
	assert.Equal(t, 0.1, manager.viper.GetFloat64("llm.temperature"))

	// 验证缓存默认配置
	assert.Equal(t, "redis", manager.viper.GetString("cache.type"))
	assert.Equal(t, "localhost", manager.viper.GetString("cache.host"))
	assert.Equal(t, 6379, manager.viper.GetInt("cache.port"))

	// 验证日志默认配置
	assert.Equal(t, "info", manager.viper.GetString("log.level"))
	assert.Equal(t, "json", manager.viper.GetString("log.format"))
	assert.Equal(t, "stdout", manager.viper.GetString("log.output"))
}

// TestValidateConfig 测试配置验证
func TestValidateConfig(t *testing.T) {
	manager := NewManager()

	// 测试有效配置
	validConfig := &core.Config{
		Server: &core.ServerConfig{
			Host: "0.0.0.0",
			Port: 8080,
		},
		Database: &core.DatabaseConfig{
			Host:     "localhost",
			Username: "root",
			Database: "test",
		},
		LLM: &core.LLMConfig{
			Provider:    "openai",
			Model:       "gpt-4",
			Temperature: 0.1,
		},
		Cache: &core.CacheConfig{
			Type: "redis",
		},
		Log: &core.LogConfig{
			Level: "info",
		},
		Security: &core.SecurityConfig{
			JWTSecret: "test-secret-key-very-secure",
		},
	}

	err := manager.validateConfig(validConfig)
	assert.NoError(t, err)

	// 测试无效端口
	invalidConfig := &core.Config{
		Server: &core.ServerConfig{
			Host: "0.0.0.0",
			Port: 0,
		},
		Database: &core.DatabaseConfig{
			Host:     "localhost",
			Username: "root",
			Database: "test",
		},
		LLM: &core.LLMConfig{
			Provider:    "openai",
			Model:       "gpt-4",
			Temperature: 0.1,
		},
		Cache: &core.CacheConfig{
			Type: "redis",
		},
		Log: &core.LogConfig{
			Level: "info",
		},
		Security: &core.SecurityConfig{
			JWTSecret: "test-secret-key-very-secure",
		},
	}
	err = manager.validateConfig(invalidConfig)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "无效的服务器端口")

	// 测试空数据库主机
	invalidConfig = &core.Config{
		Server: &core.ServerConfig{
			Host: "0.0.0.0",
			Port: 8080,
		},
		Database: &core.DatabaseConfig{
			Host:     "",
			Username: "root",
			Database: "test",
		},
		LLM: &core.LLMConfig{
			Provider:    "openai",
			Model:       "gpt-4",
			Temperature: 0.1,
		},
		Cache: &core.CacheConfig{
			Type: "redis",
		},
		Log: &core.LogConfig{
			Level: "info",
		},
		Security: &core.SecurityConfig{
			JWTSecret: "test-secret-key-very-secure",
		},
	}
	err = manager.validateConfig(invalidConfig)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "数据库主机不能为空")

	// 测试空 LLM 提供商
	invalidConfig = &core.Config{
		Server: &core.ServerConfig{
			Host: "0.0.0.0",
			Port: 8080,
		},
		Database: &core.DatabaseConfig{
			Host:     "localhost",
			Username: "root",
			Database: "test",
		},
		LLM: &core.LLMConfig{
			Provider:    "",
			Model:       "gpt-4",
			Temperature: 0.1,
		},
		Cache: &core.CacheConfig{
			Type: "redis",
		},
		Log: &core.LogConfig{
			Level: "info",
		},
		Security: &core.SecurityConfig{
			JWTSecret: "test-secret-key-very-secure",
		},
	}
	err = manager.validateConfig(invalidConfig)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "LLM 提供商不能为空")

	// 测试无效温度值
	invalidConfig = &core.Config{
		Server: &core.ServerConfig{
			Host: "0.0.0.0",
			Port: 8080,
		},
		Database: &core.DatabaseConfig{
			Host:     "localhost",
			Username: "root",
			Database: "test",
		},
		LLM: &core.LLMConfig{
			Provider:    "openai",
			Model:       "gpt-4",
			Temperature: 3.0,
		},
		Cache: &core.CacheConfig{
			Type: "redis",
		},
		Log: &core.LogConfig{
			Level: "info",
		},
		Security: &core.SecurityConfig{
			JWTSecret: "test-secret-key-very-secure",
		},
	}
	err = manager.validateConfig(invalidConfig)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "无效的 LLM 温度值")

	// 测试不支持的缓存类型
	invalidConfig = &core.Config{
		Server: &core.ServerConfig{
			Host: "0.0.0.0",
			Port: 8080,
		},
		Database: &core.DatabaseConfig{
			Host:     "localhost",
			Username: "root",
			Database: "test",
		},
		LLM: &core.LLMConfig{
			Provider:    "openai",
			Model:       "gpt-4",
			Temperature: 0.1,
		},
		Cache: &core.CacheConfig{
			Type: "invalid",
		},
		Log: &core.LogConfig{
			Level: "info",
		},
		Security: &core.SecurityConfig{
			JWTSecret: "test-secret-key-very-secure",
		},
	}
	err = manager.validateConfig(invalidConfig)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不支持的缓存类型")

	// 测试无效日志级别
	invalidConfig = &core.Config{
		Server: &core.ServerConfig{
			Host: "0.0.0.0",
			Port: 8080,
		},
		Database: &core.DatabaseConfig{
			Host:     "localhost",
			Username: "root",
			Database: "test",
		},
		LLM: &core.LLMConfig{
			Provider:    "openai",
			Model:       "gpt-4",
			Temperature: 0.1,
		},
		Cache: &core.CacheConfig{
			Type: "redis",
		},
		Log: &core.LogConfig{
			Level: "invalid",
		},
		Security: &core.SecurityConfig{
			JWTSecret: "test-secret-key-very-secure",
		},
	}
	err = manager.validateConfig(invalidConfig)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "无效的日志级别")

	// 测试默认 JWT 密钥
	invalidConfig = &core.Config{
		Server: &core.ServerConfig{
			Host: "0.0.0.0",
			Port: 8080,
		},
		Database: &core.DatabaseConfig{
			Host:     "localhost",
			Username: "root",
			Database: "test",
		},
		LLM: &core.LLMConfig{
			Provider:    "openai",
			Model:       "gpt-4",
			Temperature: 0.1,
		},
		Cache: &core.CacheConfig{
			Type: "redis",
		},
		Log: &core.LogConfig{
			Level: "info",
		},
		Security: &core.SecurityConfig{
			JWTSecret: "default-secret-key",
		},
	}
	err = manager.validateConfig(invalidConfig)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "JWT 密钥不能为空或使用默认值")
}

// TestLoadWithTempConfig 测试加载临时配置文件
func TestLoadWithTempConfig(t *testing.T) {
	// 创建临时配置文件
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test_config.yaml")

	configContent := `
server:
  host: "127.0.0.1"
  port: 9000
  read_timeout: "60s"
  write_timeout: "60s"

database:
  driver: "mysql"
  host: "testdb"
  port: 3306
  username: "testuser"
  password: "testpass"
  database: "testdb"
  max_open_conns: 50
  max_idle_conns: 5
  conn_max_lifetime: "2h"

llm:
  provider: "openai"
  model: "gpt-3.5-turbo"
  api_key: "test-api-key"
  base_url: "https://api.openai.com/v1"
  temperature: 0.2
  max_tokens: 1024
  timeout: "60s"

cache:
  type: "redis"
  host: "testredis"
  port: 6380
  password: "testpass"
  database: 1
  schema_ttl: "2h"
  query_result_ttl: "20m"
  max_cache_size: "200MB"

log:
  level: "debug"
  format: "text"
  output: "file"
  file_path: "logs/test.log"
  max_size: 50
  max_backups: 5
  max_age: 14

security:
  jwt_secret: "test-jwt-secret-key-very-secure"
  token_expiry: "12h"
  enable_rbac: false
  allowed_origins: ["http://localhost:3000"]

mcp:
  host: "127.0.0.1"
  port: 9001
  timeout: "60s"
  max_connections: 500
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// 加载配置
	manager := NewManager()
	err = manager.Load(configPath)
	require.NoError(t, err)

	// 验证配置加载
	config := manager.GetConfig()
	require.NotNil(t, config)

	// 验证服务器配置
	assert.Equal(t, "127.0.0.1", config.Server.Host)
	assert.Equal(t, 9000, config.Server.Port)
	assert.Equal(t, "60s", config.Server.ReadTimeout)

	// 验证数据库配置
	assert.Equal(t, "testdb", config.Database.Host)
	assert.Equal(t, "testuser", config.Database.Username)
	assert.Equal(t, "testdb", config.Database.Database)

	// 验证 LLM 配置
	assert.Equal(t, "gpt-3.5-turbo", config.LLM.Model)
	assert.Equal(t, float32(0.2), config.LLM.Temperature)

	// 验证缓存配置
	assert.Equal(t, "testredis", config.Cache.Host)
	assert.Equal(t, 6380, config.Cache.Port)

	// 验证日志配置
	assert.Equal(t, "debug", config.Log.Level)
	assert.Equal(t, "text", config.Log.Format)

	// 验证安全配置
	assert.Equal(t, "test-jwt-secret-key-very-secure", config.Security.JWTSecret)
	assert.Equal(t, "12h", config.Security.TokenExpiry)

	// 验证 MCP 配置
	assert.Equal(t, "127.0.0.1", config.MCP.Host)
	assert.Equal(t, 9001, config.MCP.Port)
}

// TestGetters 测试配置获取方法
func TestGetters(t *testing.T) {
	manager := NewManager()
	manager.setDefaults()

	// 设置配置结构体
	manager.config = &core.Config{
		Server: &core.ServerConfig{
			Host: "0.0.0.0",
			Port: 8080,
		},
		Database: &core.DatabaseConfig{
			Host: "localhost",
			Port: 3306,
		},
		Cache: &core.CacheConfig{
			Host: "localhost",
			Port: 6379,
		},
		MCP: &core.MCPConfig{
			Host: "0.0.0.0",
			Port: 8081,
		},
	}

	// 测试基本获取方法
	assert.Equal(t, "0.0.0.0", manager.GetString("server.host"))
	assert.Equal(t, 8080, manager.GetInt("server.port"))
	assert.Equal(t, true, manager.GetBool("security.enable_rbac"))
	assert.Equal(t, 30*time.Second, manager.GetDuration("server.read_timeout"))

	// 测试地址获取方法
	assert.Equal(t, "0.0.0.0:8080", manager.GetServerAddr())
	assert.Equal(t, "localhost:6379", manager.GetRedisAddr())
	assert.Equal(t, "0.0.0.0:8081", manager.GetMCPAddr())
}

// TestChangeHandler 测试配置变更处理器
func TestChangeHandler(t *testing.T) {
	manager := NewManager()
	manager.setDefaults()

	// 注册变更处理器
	var receivedEvent ChangeEvent
	handler := func(event ChangeEvent) error {
		receivedEvent = event
		return nil
	}

	manager.RegisterChangeHandler("test_key", handler)

	// 模拟配置变更
	event := ChangeEvent{
		Key:      "test_key",
		OldValue: "old_value",
		NewValue: "new_value",
		Time:     time.Now(),
	}

	manager.notifyHandlers(event)

	// 验证处理器被调用
	assert.Equal(t, event.Key, receivedEvent.Key)
	assert.Equal(t, event.OldValue, receivedEvent.OldValue)
	assert.Equal(t, event.NewValue, receivedEvent.NewValue)

	// 测试注销处理器
	manager.UnregisterChangeHandler("test_key")
	receivedEvent = ChangeEvent{} // 重置

	manager.notifyHandlers(event)
	assert.Equal(t, ChangeEvent{}, receivedEvent) // 应该没有变化
}

// TestUpdateConfig 测试动态配置更新
func TestUpdateConfig(t *testing.T) {
	// 创建临时配置文件
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test_config.yaml")

	configContent := `
server:
  host: "0.0.0.0"
  port: 8080

database:
  host: "localhost"
  username: "root"
  database: "test"

llm:
  provider: "openai"
  model: "gpt-4"
  temperature: 0.1

cache:
  type: "redis"

log:
  level: "info"

security:
  jwt_secret: "test-secret-key-very-secure"
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	manager := NewManager()
	err = manager.Load(configPath)
	require.NoError(t, err)

	// 测试更新有效配置
	err = manager.UpdateConfig("server.port", 9000)
	assert.NoError(t, err)
	assert.Equal(t, 9000, manager.GetConfig().Server.Port)

	// 测试更新无效配置
	err = manager.UpdateConfig("server.port", -1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "验证错误")
	// 配置应该保持原值
	assert.Equal(t, 9000, manager.GetConfig().Server.Port)
}

// TestBackupAndRestore 测试配置备份和恢复
func TestBackupAndRestore(t *testing.T) {
	// 创建临时配置文件
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test_config.yaml")

	configContent := `
server:
  host: "0.0.0.0"
  port: 8080

database:
  host: "localhost"
  username: "root"
  database: "test"

llm:
  provider: "openai"
  model: "gpt-4"
  temperature: 0.1

cache:
  type: "redis"

log:
  level: "info"

security:
  jwt_secret: "test-secret-key-very-secure"
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	manager := NewManager()
	err = manager.Load(configPath)
	require.NoError(t, err)

	// 获取备份
	backup := manager.BackupConfig()
	require.NotNil(t, backup)
	assert.Equal(t, 8080, backup.Server.Port)

	// 更新配置
	err = manager.UpdateConfig("server.port", 9000)
	require.NoError(t, err)
	assert.Equal(t, 9000, manager.GetConfig().Server.Port)

	// 恢复配置
	err = manager.RestoreConfig()
	require.NoError(t, err)
	assert.Equal(t, 8080, manager.GetConfig().Server.Port)
}

// TestValidateConfigFile 测试配置文件验证
func TestValidateConfigFile(t *testing.T) {
	tempDir := t.TempDir()

	// 创建有效配置文件
	validConfigPath := filepath.Join(tempDir, "valid_config.yaml")
	validContent := `
server:
  host: "0.0.0.0"
  port: 8080

database:
  host: "localhost"
  username: "root"
  database: "test"

llm:
  provider: "openai"
  model: "gpt-4"
  temperature: 0.1

cache:
  type: "redis"

log:
  level: "info"

security:
  jwt_secret: "test-secret-key-very-secure"
`

	err := os.WriteFile(validConfigPath, []byte(validContent), 0644)
	require.NoError(t, err)

	manager := NewManager()
	err = manager.ValidateConfigFile(validConfigPath)
	assert.NoError(t, err)

	// 创建无效配置文件
	invalidConfigPath := filepath.Join(tempDir, "invalid_config.yaml")
	invalidContent := `
server:
  host: "0.0.0.0"
  port: -1  # 无效端口

database:
  host: ""  # 空主机
  username: "root"
  database: "test"

llm:
  provider: ""  # 空提供商
  model: "gpt-4"
  temperature: 0.1

cache:
  type: "redis"

log:
  level: "info"

security:
  jwt_secret: "test-secret-key-very-secure"
`

	err = os.WriteFile(invalidConfigPath, []byte(invalidContent), 0644)
	require.NoError(t, err)

	err = manager.ValidateConfigFile(invalidConfigPath)
	assert.Error(t, err)
}

// TestGetConfigSummary 测试配置摘要
func TestGetConfigSummary(t *testing.T) {
	manager := NewManager()

	// 未加载配置时
	summary := manager.GetConfigSummary()
	assert.Nil(t, summary)

	// 加载配置后
	manager.setDefaults()
	manager.config = &core.Config{
		Server: &core.ServerConfig{
			Host: "0.0.0.0",
			Port: 8080,
		},
		Database: &core.DatabaseConfig{
			Host: "localhost",
		},
		Cache: &core.CacheConfig{
			Type: "redis",
		},
		Log: &core.LogConfig{
			Level: "info",
		},
		LLM: &core.LLMConfig{
			Provider: "openai",
			Model:    "gpt-4",
		},
		MCP: &core.MCPConfig{
			Host: "0.0.0.0",
			Port: 8081,
		},
	}
	manager.environment = Development

	summary = manager.GetConfigSummary()
	require.NotNil(t, summary)

	assert.Equal(t, Development, summary["environment"])
	assert.Equal(t, "0.0.0.0:8080", summary["server_addr"])
	assert.Equal(t, "localhost", summary["database_host"])
	assert.Equal(t, "redis", summary["cache_type"])
	assert.Equal(t, "info", summary["log_level"])
	assert.Equal(t, "openai", summary["llm_provider"])
	assert.Equal(t, "gpt-4", summary["llm_model"])
	assert.Equal(t, "0.0.0.0:8081", summary["mcp_addr"])
}

// TestExportConfig 测试配置导出
func TestExportConfig(t *testing.T) {
	manager := NewManager()

	// 未加载配置时
	_, err := manager.ExportConfig("json")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "配置未加载")

	// 加载配置后
	manager.config = &core.Config{
		Server: &core.ServerConfig{
			Host: "0.0.0.0",
			Port: 8080,
		},
	}

	// 导出 JSON
	data, err := manager.ExportConfig("json")
	assert.NoError(t, err)
	assert.Contains(t, string(data), "0.0.0.0")
	assert.Contains(t, string(data), "8080")

	// 导出 YAML
	data, err = manager.ExportConfig("yaml")
	assert.NoError(t, err)
	assert.NotEmpty(t, data)

	// 不支持的格式
	_, err = manager.ExportConfig("xml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不支持的导出格式")
}

// TestIsConfigLoaded 测试配置加载状态检查
func TestIsConfigLoaded(t *testing.T) {
	manager := NewManager()

	// 未加载时
	assert.False(t, manager.IsConfigLoaded())

	// 加载后
	manager.config = &core.Config{}
	assert.True(t, manager.IsConfigLoaded())
}

// TestGetConfigValue 测试获取配置值
func TestGetConfigValue(t *testing.T) {
	manager := NewManager()
	manager.setDefaults()

	value := manager.GetConfigValue("server.host")
	assert.Equal(t, "0.0.0.0", value)

	value = manager.GetConfigValue("server.port")
	assert.Equal(t, 8080, value)

	value = manager.GetConfigValue("non.existent.key")
	assert.Nil(t, value)
}

// TestEnvironmentMethods 测试环境相关方法
func TestEnvironmentMethods(t *testing.T) {
	manager := NewManager()

	// 测试设置和获取环境
	manager.SetEnvironment(Production)
	assert.Equal(t, Production, manager.GetEnvironment())

	manager.SetEnvironment(Staging)
	assert.Equal(t, Staging, manager.GetEnvironment())
}

// TestWatch 测试配置监听
func TestWatch(t *testing.T) {
	// 创建临时配置文件
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "watch_config.yaml")

	configContent := `
server:
  host: "0.0.0.0"
  port: 8080

database:
  host: "localhost"
  username: "root"
  database: "test"

llm:
  provider: "openai"
  model: "gpt-4"
  temperature: 0.1

cache:
  type: "redis"

log:
  level: "info"

security:
  jwt_secret: "test-secret-key-very-secure"
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	manager := NewManager()
	err = manager.Load(configPath)
	require.NoError(t, err)

	// 启动监听
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = manager.Watch(ctx)
	assert.NoError(t, err)

	// 停止监听
	manager.StopWatch()

	// 再次启动监听应该成功
	err = manager.Watch(ctx)
	assert.NoError(t, err)
}

// TestDeepCopyConfig 测试配置深拷贝
func TestDeepCopyConfig(t *testing.T) {
	manager := NewManager()

	original := &core.Config{
		Server: &core.ServerConfig{
			Host: "0.0.0.0",
			Port: 8080,
		},
		Database: &core.DatabaseConfig{
			Host:     "localhost",
			Username: "root",
		},
	}

	copy := manager.deepCopyConfig(original)

	// 验证拷贝成功
	assert.Equal(t, original.Server.Host, copy.Server.Host)
	assert.Equal(t, original.Server.Port, copy.Server.Port)
	assert.Equal(t, original.Database.Host, copy.Database.Host)

	// 验证是深拷贝（修改拷贝不影响原对象）
	copy.Server.Port = 9000
	assert.Equal(t, 8080, original.Server.Port)
	assert.Equal(t, 9000, copy.Server.Port)
}

// TestContains 测试辅助函数
func TestContains(t *testing.T) {
	slice := []string{"apple", "banana", "cherry"}

	assert.True(t, contains(slice, "banana"))
	assert.False(t, contains(slice, "orange"))
	assert.False(t, contains([]string{}, "test"))
}

// TestGetDatabaseDSN 测试数据库连接字符串生成
func TestGetDatabaseDSN(t *testing.T) {
	manager := NewManager()
	manager.config = &core.Config{
		Database: &core.DatabaseConfig{
			Username: "testuser",
			Password: "testpass",
			Host:     "testhost",
			Port:     3306,
			Database: "testdb",
		},
	}

	dsn := manager.GetDatabaseDSN()
	expected := "testuser:testpass@tcp(testhost:3306)/testdb?charset=utf8mb4&parseTime=True&loc=Local"
	assert.Equal(t, expected, dsn)
}

// BenchmarkLoadConfig 基准测试配置加载
func BenchmarkLoadConfig(b *testing.B) {
	// 创建临时配置文件
	tempDir := b.TempDir()
	configPath := filepath.Join(tempDir, "bench_config.yaml")

	configContent := `
server:
  host: "0.0.0.0"
  port: 8080

database:
  host: "localhost"
  username: "root"
  database: "test"

llm:
  provider: "openai"
  model: "gpt-4"
  temperature: 0.1

cache:
  type: "redis"

log:
  level: "info"

security:
  jwt_secret: "test-secret-key-very-secure"
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager := NewManager()
		err := manager.Load(configPath)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkGetConfig 基准测试配置获取
func BenchmarkGetConfig(b *testing.B) {
	manager := NewManager()
	manager.config = &core.Config{
		Server: &core.ServerConfig{
			Host: "0.0.0.0",
			Port: 8080,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = manager.GetConfig()
	}
}

// BenchmarkValidateConfig 基准测试配置验证
func BenchmarkValidateConfig(b *testing.B) {
	manager := NewManager()
	config := &core.Config{
		Server: &core.ServerConfig{
			Host: "0.0.0.0",
			Port: 8080,
		},
		Database: &core.DatabaseConfig{
			Host:     "localhost",
			Username: "root",
			Database: "test",
		},
		LLM: &core.LLMConfig{
			Provider:    "openai",
			Model:       "gpt-4",
			Temperature: 0.1,
		},
		Cache: &core.CacheConfig{
			Type: "redis",
		},
		Log: &core.LogConfig{
			Level: "info",
		},
		Security: &core.SecurityConfig{
			JWTSecret: "test-secret-key-very-secure",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := manager.validateConfig(config)
		if err != nil {
			b.Fatal(err)
		}
	}
}
