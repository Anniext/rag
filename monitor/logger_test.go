package monitor

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

func TestNewLoggerManager(t *testing.T) {
	tests := []struct {
		name    string
		config  *core.LogConfig
		wantErr bool
	}{
		{
			name: "有效配置",
			config: &core.LogConfig{
				Level:      "info",
				Format:     "json",
				Output:     "stdout",
				FilePath:   "logs/test.log",
				MaxSize:    10,
				MaxBackups: 3,
				MaxAge:     7,
			},
			wantErr: false,
		},
		{
			name:    "空配置",
			config:  nil,
			wantErr: true,
		},
		{
			name: "无效日志级别",
			config: &core.LogConfig{
				Level:      "invalid",
				Format:     "json",
				Output:     "stdout",
				FilePath:   "logs/test.log",
				MaxSize:    10,
				MaxBackups: 3,
				MaxAge:     7,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager, err := NewLoggerManager(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, manager)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, manager)
				if manager != nil {
					manager.Close()
				}
			}
		})
	}
}

func TestLoggerManager_GetLogger(t *testing.T) {
	config := &core.LogConfig{
		Level:      "debug",
		Format:     "json",
		Output:     "stdout",
		FilePath:   "logs/test.log",
		MaxSize:    10,
		MaxBackups: 3,
		MaxAge:     7,
	}

	manager, err := NewLoggerManager(config)
	require.NoError(t, err)
	defer manager.Close()

	logger := manager.GetLogger()
	assert.NotNil(t, logger)

	// 测试日志记录
	logger.Debug("测试调试日志", String("key", "value"))
	logger.Info("测试信息日志", Int("count", 42))
	logger.Warn("测试警告日志", Bool("flag", true))
	logger.Error("测试错误日志", Error(assert.AnError))
}

func TestLoggerManager_GetNamedLogger(t *testing.T) {
	config := &core.LogConfig{
		Level:      "info",
		Format:     "json",
		Output:     "stdout",
		FilePath:   "logs/test.log",
		MaxSize:    10,
		MaxBackups: 3,
		MaxAge:     7,
	}

	manager, err := NewLoggerManager(config)
	require.NoError(t, err)
	defer manager.Close()

	logger := manager.GetNamedLogger("test-component")
	assert.NotNil(t, logger)

	logger.Info("命名日志记录器测试", String("component", "test"))
}

func TestLoggerManager_FileOutput(t *testing.T) {
	// 创建临时目录
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "test.log")

	config := &core.LogConfig{
		Level:      "info",
		Format:     "json",
		Output:     "file",
		FilePath:   logFile,
		MaxSize:    1, // 1MB
		MaxBackups: 2,
		MaxAge:     1,
	}

	manager, err := NewLoggerManager(config)
	require.NoError(t, err)
	defer manager.Close()

	logger := manager.GetLogger()
	logger.Info("测试文件输出", String("test", "file-output"))

	// 同步日志确保写入文件
	err = manager.Sync()
	assert.NoError(t, err)

	// 检查文件是否存在
	_, err = os.Stat(logFile)
	assert.NoError(t, err)
}

func TestLoggerManager_BothOutput(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "test.log")

	config := &core.LogConfig{
		Level:      "info",
		Format:     "json",
		Output:     "both",
		FilePath:   logFile,
		MaxSize:    1,
		MaxBackups: 2,
		MaxAge:     1,
	}

	manager, err := NewLoggerManager(config)
	require.NoError(t, err)
	defer manager.Close()

	logger := manager.GetLogger()
	logger.Info("测试同时输出", String("test", "both-output"))

	// 只同步文件输出，避免stdout同步问题
	time.Sleep(100 * time.Millisecond) // 等待日志写入

	// 检查文件是否存在
	_, err = os.Stat(logFile)
	assert.NoError(t, err)
}

func TestLoggerManager_UpdateConfig(t *testing.T) {
	tempDir := t.TempDir()

	config1 := &core.LogConfig{
		Level:      "info",
		Format:     "json",
		Output:     "file",
		FilePath:   filepath.Join(tempDir, "test1.log"),
		MaxSize:    10,
		MaxBackups: 3,
		MaxAge:     7,
	}

	manager, err := NewLoggerManager(config1)
	require.NoError(t, err)
	defer manager.Close()

	logger1 := manager.GetLogger()
	logger1.Info("第一个配置的日志")

	// 更新配置
	config2 := &core.LogConfig{
		Level:      "debug",
		Format:     "json",
		Output:     "file",
		FilePath:   filepath.Join(tempDir, "test2.log"),
		MaxSize:    20,
		MaxBackups: 5,
		MaxAge:     14,
	}

	err = manager.UpdateConfig(config2)
	assert.NoError(t, err)

	logger2 := manager.GetLogger()
	logger2.Debug("第二个配置的日志")
	logger2.Info("更新后的日志")

	// 等待日志写入
	time.Sleep(100 * time.Millisecond)
}

func TestLoggerManager_Close(t *testing.T) {
	config := &core.LogConfig{
		Level:      "info",
		Format:     "json",
		Output:     "stdout",
		FilePath:   "logs/test.log",
		MaxSize:    10,
		MaxBackups: 3,
		MaxAge:     7,
	}

	manager, err := NewLoggerManager(config)
	require.NoError(t, err)

	logger := manager.GetLogger()
	logger.Info("关闭前的日志")

	err = manager.Close()
	assert.NoError(t, err)

	// 关闭后获取的logger应该是noop logger
	logger2 := manager.GetLogger()
	assert.NotNil(t, logger2)

	// noop logger不应该panic
	logger2.Info("关闭后的日志")
}

func TestZapLoggerWrapper_ContextMethods(t *testing.T) {
	config := &core.LogConfig{
		Level:      "debug",
		Format:     "json",
		Output:     "stdout",
		FilePath:   "logs/test.log",
		MaxSize:    10,
		MaxBackups: 3,
		MaxAge:     7,
	}

	manager, err := NewLoggerManager(config)
	require.NoError(t, err)
	defer manager.Close()

	logger := manager.GetLogger()

	// 创建带有上下文信息的context
	ctx := context.Background()
	ctx = context.WithValue(ctx, "request_id", "req-123")
	ctx = context.WithValue(ctx, "user_id", "user-456")
	ctx = context.WithValue(ctx, "session_id", "sess-789")
	ctx = context.WithValue(ctx, "trace_id", "trace-abc")

	// 测试带上下文的日志记录
	logger.DebugContext(ctx, "调试日志", String("action", "test"))
	logger.InfoContext(ctx, "信息日志", String("action", "test"))
	logger.WarnContext(ctx, "警告日志", String("action", "test"))
	logger.ErrorContext(ctx, "错误日志", String("action", "test"))
}

func TestZapLoggerWrapper_With(t *testing.T) {
	config := &core.LogConfig{
		Level:      "info",
		Format:     "json",
		Output:     "stdout",
		FilePath:   "logs/test.log",
		MaxSize:    10,
		MaxBackups: 3,
		MaxAge:     7,
	}

	manager, err := NewLoggerManager(config)
	require.NoError(t, err)
	defer manager.Close()

	logger := manager.GetLogger()

	// 创建带有预设字段的logger
	componentLogger := logger.With(
		String("component", "test"),
		String("version", "1.0.0"),
	)

	componentLogger.Info("组件日志", String("action", "start"))
	componentLogger.Info("组件日志", String("action", "process"))
	componentLogger.Info("组件日志", String("action", "end"))
}

func TestZapLoggerWrapper_Named(t *testing.T) {
	config := &core.LogConfig{
		Level:      "info",
		Format:     "json",
		Output:     "stdout",
		FilePath:   "logs/test.log",
		MaxSize:    10,
		MaxBackups: 3,
		MaxAge:     7,
	}

	manager, err := NewLoggerManager(config)
	require.NoError(t, err)
	defer manager.Close()

	logger := manager.GetLogger()

	// 创建命名的logger
	namedLogger := logger.Named("database")
	namedLogger.Info("数据库连接成功", String("host", "localhost"))

	subLogger := namedLogger.Named("query")
	subLogger.Info("执行查询", String("sql", "SELECT * FROM users"))
}

func TestGlobalLogger(t *testing.T) {
	config := &core.LogConfig{
		Level:      "info",
		Format:     "json",
		Output:     "stdout",
		FilePath:   "logs/global.log",
		MaxSize:    10,
		MaxBackups: 3,
		MaxAge:     7,
	}

	// 初始化全局日志
	err := InitGlobalLogger(config)
	require.NoError(t, err)
	defer CloseGlobalLogger()

	// 获取全局日志记录器
	logger := GetGlobalLogger()
	assert.NotNil(t, logger)

	logger.Info("全局日志测试", String("test", "global"))

	// 获取命名的全局日志记录器
	namedLogger := GetNamedGlobalLogger("global-test")
	namedLogger.Info("命名全局日志测试", String("component", "global-test"))

	// 同步全局日志
	err = SyncGlobalLogger()
	assert.NoError(t, err)
}

func TestLoggerFields(t *testing.T) {
	// 测试预定义的字段构造函数
	stringField := String("string", "value")
	assert.Equal(t, "string", stringField.Key)

	intField := Int("int", 42)
	assert.Equal(t, "int", intField.Key)

	int64Field := Int64("int64", int64(123))
	assert.Equal(t, "int64", int64Field.Key)

	float64Field := Float64("float64", 3.14)
	assert.Equal(t, "float64", float64Field.Key)

	boolField := Bool("bool", true)
	assert.Equal(t, "bool", boolField.Key)

	durationField := Duration("duration", time.Second)
	assert.Equal(t, "duration", durationField.Key)

	timeField := Time("time", time.Now())
	assert.Equal(t, "time", timeField.Key)

	errorField := Error(assert.AnError)
	assert.Equal(t, "error", errorField.Key)

	anyField := Any("any", map[string]string{"key": "value"})
	assert.Equal(t, "any", anyField.Key)
}

func TestNoopLogger(t *testing.T) {
	logger := &noopLogger{}

	// 所有方法都不应该panic
	logger.Debug("debug")
	logger.Info("info")
	logger.Warn("warn")
	logger.Error("error")
	logger.Fatal("fatal")

	ctx := context.Background()
	logger.DebugContext(ctx, "debug")
	logger.InfoContext(ctx, "info")
	logger.WarnContext(ctx, "warn")
	logger.ErrorContext(ctx, "error")

	withLogger := logger.With(String("key", "value"))
	assert.Equal(t, logger, withLogger)

	namedLogger := logger.Named("test")
	assert.Equal(t, logger, namedLogger)

	err := logger.Sync()
	assert.NoError(t, err)

	err = logger.Close()
	assert.NoError(t, err)
}

func TestLoggerManager_parseLogLevel(t *testing.T) {
	manager := &LoggerManager{}

	tests := []struct {
		level    string
		expected string
		wantErr  bool
	}{
		{"debug", "debug", false},
		{"info", "info", false},
		{"warn", "warn", false},
		{"error", "error", false},
		{"fatal", "fatal", false},
		{"invalid", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			level, err := manager.parseLogLevel(tt.level)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, level.String())
			}
		})
	}
}

func TestLoggerManager_createEncoderConfig(t *testing.T) {
	manager := &LoggerManager{}
	config := manager.createEncoderConfig()

	assert.Equal(t, "timestamp", config.TimeKey)
	assert.Equal(t, "level", config.LevelKey)
	assert.Equal(t, "caller", config.CallerKey)
	assert.Equal(t, "message", config.MessageKey)
	assert.Equal(t, "stacktrace", config.StacktraceKey)
}

// 基准测试
func BenchmarkLogger_Info(b *testing.B) {
	config := &core.LogConfig{
		Level:      "info",
		Format:     "json",
		Output:     "stdout",
		FilePath:   "logs/bench.log",
		MaxSize:    10,
		MaxBackups: 3,
		MaxAge:     7,
	}

	manager, err := NewLoggerManager(config)
	require.NoError(b, err)
	defer manager.Close()

	logger := manager.GetLogger()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logger.Info("基准测试日志",
				String("key1", "value1"),
				Int("key2", 42),
				Bool("key3", true),
			)
		}
	})
}

func BenchmarkLogger_InfoContext(b *testing.B) {
	config := &core.LogConfig{
		Level:      "info",
		Format:     "json",
		Output:     "stdout",
		FilePath:   "logs/bench.log",
		MaxSize:    10,
		MaxBackups: 3,
		MaxAge:     7,
	}

	manager, err := NewLoggerManager(config)
	require.NoError(b, err)
	defer manager.Close()

	logger := manager.GetLogger()
	ctx := context.WithValue(context.Background(), "request_id", "req-123")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logger.InfoContext(ctx, "基准测试上下文日志",
				String("key1", "value1"),
				Int("key2", 42),
			)
		}
	})
}
