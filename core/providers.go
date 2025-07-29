package core

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/go-sql-driver/mysql"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// ConfigProvider 配置服务提供者
type ConfigProvider struct {
	configPath string
}

// NewConfigProvider 创建配置服务提供者
func NewConfigProvider(configPath string) *ConfigProvider {
	return &ConfigProvider{
		configPath: configPath,
	}
}

// Register 注册配置服务
func (p *ConfigProvider) Register(container *Container) error {
	container.RegisterSingleton(ServiceConfig, func() interface{} {
		// 这里需要导入 config 包，但为了避免循环依赖，我们先返回一个占位符
		// 实际实现中应该创建 config.Manager 实例
		return map[string]interface{}{
			"config_path": p.configPath,
		}
	})
	return nil
}

// Boot 启动配置服务
func (p *ConfigProvider) Boot(container *Container) error {
	// 配置服务的启动逻辑
	return nil
}

// LoggerProvider 日志服务提供者
type LoggerProvider struct{}

// NewLoggerProvider 创建日志服务提供者
func NewLoggerProvider() *LoggerProvider {
	return &LoggerProvider{}
}

// Register 注册日志服务
func (p *LoggerProvider) Register(container *Container) error {
	container.RegisterSingleton(ServiceLogger, func() any {
		return p.createLogger(container)
	})
	return nil
}

// Boot 启动日志服务
func (p *LoggerProvider) Boot(container *Container) error {
	return nil
}

// createLogger 创建日志记录器
func (p *LoggerProvider) createLogger(container *Container) Logger {
	// 获取配置
	configService, err := container.Get(ServiceConfig)
	if err != nil {
		// 使用默认配置
		return p.createDefaultLogger()
	}

	// 这里应该从配置中读取日志配置
	// 为了简化，我们先使用默认配置
	_ = configService
	return p.createDefaultLogger()
}

// createDefaultLogger 创建默认日志记录器
func (p *LoggerProvider) createDefaultLogger() Logger {
	// 配置日志输出
	writer := zapcore.AddSync(&lumberjack.Logger{
		Filename:   "logs/rag.log",
		MaxSize:    100, // MB
		MaxBackups: 3,
		MaxAge:     7, // days
		Compress:   true,
	})

	// 配置编码器
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "timestamp"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder

	encoder := zapcore.NewJSONEncoder(encoderConfig)

	// 创建核心
	core := zapcore.NewTee(
		zapcore.NewCore(encoder, writer, zapcore.InfoLevel),
		zapcore.NewCore(encoder, zapcore.AddSync(os.Stdout), zapcore.InfoLevel),
	)

	// 创建 logger
	zapLogger := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))

	return &ZapLogger{logger: zapLogger}
}

// ZapLogger Zap 日志记录器实现
type ZapLogger struct {
	logger *zap.Logger
}

// Debug 记录调试日志
func (l *ZapLogger) Debug(msg string, fields ...any) {
	l.logger.Debug(msg, l.convertFields(fields...)...)
}

// Info 记录信息日志
func (l *ZapLogger) Info(msg string, fields ...any) {
	l.logger.Info(msg, l.convertFields(fields...)...)
}

// Warn 记录警告日志
func (l *ZapLogger) Warn(msg string, fields ...any) {
	l.logger.Warn(msg, l.convertFields(fields...)...)
}

// Error 记录错误日志
func (l *ZapLogger) Error(msg string, fields ...any) {
	l.logger.Error(msg, l.convertFields(fields...)...)
}

// Fatal 记录致命错误日志
func (l *ZapLogger) Fatal(msg string, fields ...any) {
	l.logger.Fatal(msg, l.convertFields(fields...)...)
}

// convertFields 转换字段为 zap.Field
func (l *ZapLogger) convertFields(fields ...any) []zap.Field {
	zapFields := make([]zap.Field, 0, len(fields)/2)

	for i := 0; i < len(fields)-1; i += 2 {
		key, ok := fields[i].(string)
		if !ok {
			continue
		}
		value := fields[i+1]
		zapFields = append(zapFields, zap.Any(key, value))
	}

	return zapFields
}

// DatabaseProvider 数据库服务提供者
type DatabaseProvider struct{}

// NewDatabaseProvider 创建数据库服务提供者
func NewDatabaseProvider() *DatabaseProvider {
	return &DatabaseProvider{}
}

// Register 注册数据库服务
func (p *DatabaseProvider) Register(container *Container) error {
	container.RegisterSingleton(ServiceDatabase, func() any {
		return p.createDatabase(container)
	})
	return nil
}

// Boot 启动数据库服务
func (p *DatabaseProvider) Boot(container *Container) error {
	// 测试数据库连接
	db, err := container.Get(ServiceDatabase)
	if err != nil {
		return fmt.Errorf("获取数据库服务失败: %w", err)
	}

	if sqlDB, ok := db.(*sql.DB); ok {
		if err := sqlDB.Ping(); err != nil {
			return fmt.Errorf("数据库连接测试失败: %w", err)
		}
	}

	return nil
}

// createDatabase 创建数据库连接
func (p *DatabaseProvider) createDatabase(container *Container) *sql.DB {
	// 获取配置
	configService, err := container.Get(ServiceConfig)
	if err != nil {
		log.Fatal("获取配置服务失败:", err)
	}

	// 这里应该从配置中读取数据库配置
	// 为了简化，我们先使用默认配置
	_ = configService

	// 默认数据库连接字符串
	dsn := "root:123456@tcp(127.0.0.1:3306)/pumppill_rag?charset=utf8mb4&parseTime=True&loc=Local"

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal("打开数据库连接失败:", err)
	}

	// 设置连接池参数
	db.SetMaxOpenConns(100)
	db.SetMaxIdleConns(10)

	return db
}

// ErrorHandlerProvider 错误处理器服务提供者
type ErrorHandlerProvider struct{}

// NewErrorHandlerProvider 创建错误处理器服务提供者
func NewErrorHandlerProvider() *ErrorHandlerProvider {
	return &ErrorHandlerProvider{}
}

// Register 注册错误处理器服务
func (p *ErrorHandlerProvider) Register(container *Container) error {
	container.RegisterSingleton(ServiceErrorHandler, func() any {
		return p.createErrorHandler(container)
	})
	return nil
}

// Boot 启动错误处理器服务
func (p *ErrorHandlerProvider) Boot(container *Container) error {
	return nil
}

// createErrorHandler 创建错误处理器
func (p *ErrorHandlerProvider) createErrorHandler(container *Container) *ErrorHandler {
	// 获取日志服务
	loggerService, err := container.Get(ServiceLogger)
	if err != nil {
		log.Fatal("获取日志服务失败:", err)
	}

	logger, ok := loggerService.(Logger)
	if !ok {
		log.Fatal("日志服务类型错误")
	}

	// 获取指标服务（可选）
	var metrics MetricsCollector
	if metricsService, err := container.Get(ServiceMetrics); err == nil {
		if m, ok := metricsService.(MetricsCollector); ok {
			metrics = m
		}
	}

	return NewErrorHandler(logger, metrics)
}

// DefaultServiceProvider 默认服务提供者
type DefaultServiceProvider struct {
	configPath string
}

// NewDefaultServiceProvider 创建默认服务提供者
func NewDefaultServiceProvider(configPath string) *DefaultServiceProvider {
	return &DefaultServiceProvider{
		configPath: configPath,
	}
}

// Register 注册默认服务
func (p *DefaultServiceProvider) Register(container *Container) error {
	// 注册配置服务
	configProvider := NewConfigProvider(p.configPath)
	if err := configProvider.Register(container); err != nil {
		return err
	}

	// 注册日志服务
	loggerProvider := NewLoggerProvider()
	if err := loggerProvider.Register(container); err != nil {
		return err
	}

	// 注册数据库服务
	databaseProvider := NewDatabaseProvider()
	if err := databaseProvider.Register(container); err != nil {
		return err
	}

	// 注册错误处理器服务
	errorHandlerProvider := NewErrorHandlerProvider()
	if err := errorHandlerProvider.Register(container); err != nil {
		return err
	}

	return nil
}

// Boot 启动默认服务
func (p *DefaultServiceProvider) Boot(container *Container) error {
	// 启动配置服务
	configProvider := NewConfigProvider(p.configPath)
	if err := configProvider.Boot(container); err != nil {
		return err
	}

	// 启动日志服务
	loggerProvider := NewLoggerProvider()
	if err := loggerProvider.Boot(container); err != nil {
		return err
	}

	// 启动数据库服务
	databaseProvider := NewDatabaseProvider()
	if err := databaseProvider.Boot(container); err != nil {
		return err
	}

	// 启动错误处理器服务
	errorHandlerProvider := NewErrorHandlerProvider()
	if err := errorHandlerProvider.Boot(container); err != nil {
		return err
	}

	return nil
}
