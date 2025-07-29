// 本文件实现了结构化日志记录系统，提供统一的日志接口和管理功能。
// 支持多种日志级别、格式化输出、文件轮转和归档机制。
// 主要功能：
// 1. 基于 zap 的高性能结构化日志记录
// 2. 支持控制台和文件输出
// 3. 日志文件轮转和归档管理
// 4. 不同级别的日志过滤
// 5. 上下文感知的日志记录
// 6. 性能监控和错误追踪

package monitor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"pumppill/rag/core"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Logger 日志记录器接口，定义统一的日志记录方法
type Logger interface {
	Debug(msg string, fields ...zap.Field)
	Info(msg string, fields ...zap.Field)
	Warn(msg string, fields ...zap.Field)
	Error(msg string, fields ...zap.Field)
	Fatal(msg string, fields ...zap.Field)

	// 带上下文的日志记录
	DebugContext(ctx context.Context, msg string, fields ...zap.Field)
	InfoContext(ctx context.Context, msg string, fields ...zap.Field)
	WarnContext(ctx context.Context, msg string, fields ...zap.Field)
	ErrorContext(ctx context.Context, msg string, fields ...zap.Field)

	// 结构化日志记录
	With(fields ...zap.Field) Logger
	Named(name string) Logger

	// 同步和关闭
	Sync() error
	Close() error
}

// LoggerManager 日志管理器，负责创建和管理日志记录器
type LoggerManager struct {
	config    *core.LogConfig
	zapLogger *zap.Logger
	mutex     sync.RWMutex
	writers   []zapcore.WriteSyncer
	closed    bool
}

// NewLoggerManager 创建日志管理器实例
func NewLoggerManager(config *core.LogConfig) (*LoggerManager, error) {
	if config == nil {
		return nil, fmt.Errorf("日志配置不能为空")
	}

	manager := &LoggerManager{
		config:  config,
		writers: make([]zapcore.WriteSyncer, 0),
	}

	if err := manager.initialize(); err != nil {
		return nil, fmt.Errorf("初始化日志管理器失败: %w", err)
	}

	return manager, nil
}

// initialize 初始化日志系统
func (lm *LoggerManager) initialize() error {
	// 创建编码器配置
	encoderConfig := lm.createEncoderConfig()

	// 创建编码器
	var encoder zapcore.Encoder
	switch lm.config.Format {
	case "json":
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	case "console":
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	default:
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	}

	// 创建写入器
	writeSyncers, err := lm.createWriteSyncers()
	if err != nil {
		return fmt.Errorf("创建写入器失败: %w", err)
	}
	lm.writers = writeSyncers

	// 创建日志级别
	level, err := lm.parseLogLevel(lm.config.Level)
	if err != nil {
		return fmt.Errorf("解析日志级别失败: %w", err)
	}

	// 创建核心
	core := zapcore.NewTee(
		lm.createCores(encoder, writeSyncers, level)...,
	)

	// 创建 zap logger
	lm.zapLogger = zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))

	return nil
}

// createEncoderConfig 创建编码器配置
func (lm *LoggerManager) createEncoderConfig() zapcore.EncoderConfig {
	config := zap.NewProductionEncoderConfig()

	// 时间格式
	config.TimeKey = "timestamp"
	config.EncodeTime = zapcore.ISO8601TimeEncoder

	// 级别格式
	config.LevelKey = "level"
	config.EncodeLevel = zapcore.LowercaseLevelEncoder

	// 调用者信息
	config.CallerKey = "caller"
	config.EncodeCaller = zapcore.ShortCallerEncoder

	// 消息键
	config.MessageKey = "message"

	// 堆栈跟踪
	config.StacktraceKey = "stacktrace"

	return config
}

// createWriteSyncers 创建写入器
func (lm *LoggerManager) createWriteSyncers() ([]zapcore.WriteSyncer, error) {
	var syncers []zapcore.WriteSyncer

	switch lm.config.Output {
	case "stdout":
		syncers = append(syncers, zapcore.AddSync(os.Stdout))
	case "stderr":
		syncers = append(syncers, zapcore.AddSync(os.Stderr))
	case "file":
		fileSyncer, err := lm.createFileSyncer()
		if err != nil {
			return nil, err
		}
		syncers = append(syncers, fileSyncer)
	case "both":
		// 同时输出到控制台和文件
		syncers = append(syncers, zapcore.AddSync(os.Stdout))
		fileSyncer, err := lm.createFileSyncer()
		if err != nil {
			return nil, err
		}
		syncers = append(syncers, fileSyncer)
	default:
		syncers = append(syncers, zapcore.AddSync(os.Stdout))
	}

	return syncers, nil
}

// createFileSyncer 创建文件写入器，支持日志轮转
func (lm *LoggerManager) createFileSyncer() (zapcore.WriteSyncer, error) {
	// 确保日志目录存在
	logDir := filepath.Dir(lm.config.FilePath)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("创建日志目录失败: %w", err)
	}

	// 创建 lumberjack 轮转写入器
	lumberjackLogger := &lumberjack.Logger{
		Filename:   lm.config.FilePath,
		MaxSize:    lm.config.MaxSize,    // MB
		MaxBackups: lm.config.MaxBackups, // 保留的旧文件数量
		MaxAge:     lm.config.MaxAge,     // 天数
		Compress:   true,                 // 压缩旧文件
		LocalTime:  true,                 // 使用本地时间
	}

	return zapcore.AddSync(lumberjackLogger), nil
}

// createCores 创建日志核心
func (lm *LoggerManager) createCores(encoder zapcore.Encoder, syncers []zapcore.WriteSyncer, level zapcore.Level) []zapcore.Core {
	cores := make([]zapcore.Core, 0, len(syncers))

	for _, syncer := range syncers {
		core := zapcore.NewCore(encoder, syncer, level)
		cores = append(cores, core)
	}

	return cores
}

// parseLogLevel 解析日志级别
func (lm *LoggerManager) parseLogLevel(levelStr string) (zapcore.Level, error) {
	switch levelStr {
	case "debug":
		return zapcore.DebugLevel, nil
	case "info":
		return zapcore.InfoLevel, nil
	case "warn":
		return zapcore.WarnLevel, nil
	case "error":
		return zapcore.ErrorLevel, nil
	case "fatal":
		return zapcore.FatalLevel, nil
	default:
		return zapcore.InfoLevel, fmt.Errorf("不支持的日志级别: %s", levelStr)
	}
}

// GetLogger 获取日志记录器
func (lm *LoggerManager) GetLogger() Logger {
	lm.mutex.RLock()
	defer lm.mutex.RUnlock()

	if lm.closed {
		// 返回一个空的日志记录器
		return &noopLogger{}
	}

	return &zapLoggerWrapper{
		logger: lm.zapLogger,
	}
}

// GetNamedLogger 获取命名的日志记录器
func (lm *LoggerManager) GetNamedLogger(name string) Logger {
	lm.mutex.RLock()
	defer lm.mutex.RUnlock()

	if lm.closed {
		return &noopLogger{}
	}

	return &zapLoggerWrapper{
		logger: lm.zapLogger.Named(name),
	}
}

// Sync 同步所有日志输出
func (lm *LoggerManager) Sync() error {
	lm.mutex.RLock()
	defer lm.mutex.RUnlock()

	if lm.closed {
		return nil
	}

	return lm.zapLogger.Sync()
}

// Close 关闭日志管理器
func (lm *LoggerManager) Close() error {
	lm.mutex.Lock()
	defer lm.mutex.Unlock()

	if lm.closed {
		return nil
	}

	lm.closed = true

	// 同步日志
	if err := lm.zapLogger.Sync(); err != nil {
		return fmt.Errorf("同步日志失败: %w", err)
	}

	// 关闭写入器
	for _, writer := range lm.writers {
		if closer, ok := writer.(interface{ Close() error }); ok {
			if err := closer.Close(); err != nil {
				return fmt.Errorf("关闭写入器失败: %w", err)
			}
		}
	}

	return nil
}

// UpdateConfig 更新日志配置（热更新）
func (lm *LoggerManager) UpdateConfig(config *core.LogConfig) error {
	lm.mutex.Lock()
	defer lm.mutex.Unlock()

	if lm.closed {
		return fmt.Errorf("日志管理器已关闭")
	}

	// 保存旧的配置和logger
	oldConfig := lm.config
	oldLogger := lm.zapLogger
	oldWriters := lm.writers

	// 更新配置
	lm.config = config

	// 重新初始化
	if err := lm.initialize(); err != nil {
		// 恢复旧配置
		lm.config = oldConfig
		lm.zapLogger = oldLogger
		lm.writers = oldWriters
		return fmt.Errorf("更新日志配置失败: %w", err)
	}

	// 同步并关闭旧的logger
	if oldLogger != nil {
		oldLogger.Sync()
	}

	// 关闭旧的写入器
	for _, writer := range oldWriters {
		if closer, ok := writer.(interface{ Close() error }); ok {
			closer.Close()
		}
	}

	return nil
}

// zapLoggerWrapper zap日志记录器包装器
type zapLoggerWrapper struct {
	logger *zap.Logger
}

// Debug 记录调试级别日志
func (w *zapLoggerWrapper) Debug(msg string, fields ...zap.Field) {
	w.logger.Debug(msg, fields...)
}

// Info 记录信息级别日志
func (w *zapLoggerWrapper) Info(msg string, fields ...zap.Field) {
	w.logger.Info(msg, fields...)
}

// Warn 记录警告级别日志
func (w *zapLoggerWrapper) Warn(msg string, fields ...zap.Field) {
	w.logger.Warn(msg, fields...)
}

// Error 记录错误级别日志
func (w *zapLoggerWrapper) Error(msg string, fields ...zap.Field) {
	w.logger.Error(msg, fields...)
}

// Fatal 记录致命级别日志
func (w *zapLoggerWrapper) Fatal(msg string, fields ...zap.Field) {
	w.logger.Fatal(msg, fields...)
}

// DebugContext 带上下文的调试日志
func (w *zapLoggerWrapper) DebugContext(ctx context.Context, msg string, fields ...zap.Field) {
	fields = w.addContextFields(ctx, fields)
	w.logger.Debug(msg, fields...)
}

// InfoContext 带上下文的信息日志
func (w *zapLoggerWrapper) InfoContext(ctx context.Context, msg string, fields ...zap.Field) {
	fields = w.addContextFields(ctx, fields)
	w.logger.Info(msg, fields...)
}

// WarnContext 带上下文的警告日志
func (w *zapLoggerWrapper) WarnContext(ctx context.Context, msg string, fields ...zap.Field) {
	fields = w.addContextFields(ctx, fields)
	w.logger.Warn(msg, fields...)
}

// ErrorContext 带上下文的错误日志
func (w *zapLoggerWrapper) ErrorContext(ctx context.Context, msg string, fields ...zap.Field) {
	fields = w.addContextFields(ctx, fields)
	w.logger.Error(msg, fields...)
}

// With 添加字段
func (w *zapLoggerWrapper) With(fields ...zap.Field) Logger {
	return &zapLoggerWrapper{
		logger: w.logger.With(fields...),
	}
}

// Named 创建命名的日志记录器
func (w *zapLoggerWrapper) Named(name string) Logger {
	return &zapLoggerWrapper{
		logger: w.logger.Named(name),
	}
}

// Sync 同步日志输出
func (w *zapLoggerWrapper) Sync() error {
	return w.logger.Sync()
}

// Close 关闭日志记录器
func (w *zapLoggerWrapper) Close() error {
	return w.logger.Sync()
}

// addContextFields 从上下文中提取字段
func (w *zapLoggerWrapper) addContextFields(ctx context.Context, fields []zap.Field) []zap.Field {
	if ctx == nil {
		return fields
	}

	// 提取请求ID
	if requestID := ctx.Value("request_id"); requestID != nil {
		if id, ok := requestID.(string); ok {
			fields = append(fields, zap.String("request_id", id))
		}
	}

	// 提取用户ID
	if userID := ctx.Value("user_id"); userID != nil {
		if id, ok := userID.(string); ok {
			fields = append(fields, zap.String("user_id", id))
		}
	}

	// 提取会话ID
	if sessionID := ctx.Value("session_id"); sessionID != nil {
		if id, ok := sessionID.(string); ok {
			fields = append(fields, zap.String("session_id", id))
		}
	}

	// 提取跟踪ID
	if traceID := ctx.Value("trace_id"); traceID != nil {
		if id, ok := traceID.(string); ok {
			fields = append(fields, zap.String("trace_id", id))
		}
	}

	return fields
}

// noopLogger 空操作日志记录器，用于关闭状态
type noopLogger struct{}

func (n *noopLogger) Debug(msg string, fields ...zap.Field)                             {}
func (n *noopLogger) Info(msg string, fields ...zap.Field)                              {}
func (n *noopLogger) Warn(msg string, fields ...zap.Field)                              {}
func (n *noopLogger) Error(msg string, fields ...zap.Field)                             {}
func (n *noopLogger) Fatal(msg string, fields ...zap.Field)                             {}
func (n *noopLogger) DebugContext(ctx context.Context, msg string, fields ...zap.Field) {}
func (n *noopLogger) InfoContext(ctx context.Context, msg string, fields ...zap.Field)  {}
func (n *noopLogger) WarnContext(ctx context.Context, msg string, fields ...zap.Field)  {}
func (n *noopLogger) ErrorContext(ctx context.Context, msg string, fields ...zap.Field) {}
func (n *noopLogger) With(fields ...zap.Field) Logger                                   { return n }
func (n *noopLogger) Named(name string) Logger                                          { return n }
func (n *noopLogger) Sync() error                                                       { return nil }
func (n *noopLogger) Close() error                                                      { return nil }

// 预定义的日志字段构造函数
func String(key, val string) zap.Field {
	return zap.String(key, val)
}

func Int(key string, val int) zap.Field {
	return zap.Int(key, val)
}

func Int64(key string, val int64) zap.Field {
	return zap.Int64(key, val)
}

func Float64(key string, val float64) zap.Field {
	return zap.Float64(key, val)
}

func Bool(key string, val bool) zap.Field {
	return zap.Bool(key, val)
}

func Duration(key string, val time.Duration) zap.Field {
	return zap.Duration(key, val)
}

func Time(key string, val time.Time) zap.Field {
	return zap.Time(key, val)
}

func Error(err error) zap.Field {
	return zap.Error(err)
}

func Any(key string, val interface{}) zap.Field {
	return zap.Any(key, val)
}

// 全局日志管理器实例
var (
	globalLoggerManager *LoggerManager
	globalLoggerMutex   sync.RWMutex
)

// InitGlobalLogger 初始化全局日志管理器
func InitGlobalLogger(config *core.LogConfig) error {
	globalLoggerMutex.Lock()
	defer globalLoggerMutex.Unlock()

	// 关闭旧的日志管理器
	if globalLoggerManager != nil {
		globalLoggerManager.Close()
	}

	// 创建新的日志管理器
	manager, err := NewLoggerManager(config)
	if err != nil {
		return err
	}

	globalLoggerManager = manager
	return nil
}

// GetGlobalLogger 获取全局日志记录器
func GetGlobalLogger() Logger {
	globalLoggerMutex.RLock()
	defer globalLoggerMutex.RUnlock()

	if globalLoggerManager == nil {
		return &noopLogger{}
	}

	return globalLoggerManager.GetLogger()
}

// GetNamedGlobalLogger 获取命名的全局日志记录器
func GetNamedGlobalLogger(name string) Logger {
	globalLoggerMutex.RLock()
	defer globalLoggerMutex.RUnlock()

	if globalLoggerManager == nil {
		return &noopLogger{}
	}

	return globalLoggerManager.GetNamedLogger(name)
}

// SyncGlobalLogger 同步全局日志
func SyncGlobalLogger() error {
	globalLoggerMutex.RLock()
	defer globalLoggerMutex.RUnlock()

	if globalLoggerManager == nil {
		return nil
	}

	return globalLoggerManager.Sync()
}

// CloseGlobalLogger 关闭全局日志管理器
func CloseGlobalLogger() error {
	globalLoggerMutex.Lock()
	defer globalLoggerMutex.Unlock()

	if globalLoggerManager == nil {
		return nil
	}

	err := globalLoggerManager.Close()
	globalLoggerManager = nil
	return err
}
