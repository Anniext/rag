package test

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/Anniext/rag/core"
	"go.uber.org/zap"
	"os"
	"strconv"
	"time"
)

// 共享的测试工具函数和 Mock 实现

// MockSchemaLoader Mock Schema 加载器
type MockSchemaLoader struct {
	db *sql.DB
}

func (m *MockSchemaLoader) LoadSchema(ctx context.Context) (*core.SchemaInfo, error) {
	return &core.SchemaInfo{
		Database: "test_db",
		Tables: []*core.TableInfo{
			{
				Name:    "users",
				Comment: "用户表",
				Columns: []*core.Column{
					{Name: "id", Type: "int", IsPrimaryKey: true},
					{Name: "name", Type: "varchar(100)"},
					{Name: "email", Type: "varchar(255)"},
					{Name: "age", Type: "int"},
				},
			},
			{
				Name:    "orders",
				Comment: "订单表",
				Columns: []*core.Column{
					{Name: "id", Type: "int", IsPrimaryKey: true},
					{Name: "user_id", Type: "int"},
					{Name: "amount", Type: "decimal(10,2)"},
					{Name: "created_at", Type: "datetime"},
				},
			},
		},
		Version:   "1.0",
		UpdatedAt: time.Now(),
	}, nil
}

func (m *MockSchemaLoader) GetTableInfo(ctx context.Context, tableName string) (*core.TableInfo, error) {
	schema, err := m.LoadSchema(ctx)
	if err != nil {
		return nil, err
	}

	for _, table := range schema.Tables {
		if table.Name == tableName {
			return table, nil
		}
	}

	return nil, fmt.Errorf("表 %s 不存在", tableName)
}

func (m *MockSchemaLoader) GetTableStatistics(ctx context.Context, tableName string) (map[string]interface{}, error) {
	return map[string]interface{}{
		"row_count":   1000,
		"data_length": 1024 * 1024,
		"index_count": 3,
	}, nil
}

func (m *MockSchemaLoader) ValidateConnection(ctx context.Context) error {
	if m.db != nil {
		return m.db.PingContext(ctx)
	}
	return nil
}

func (m *MockSchemaLoader) Close() error {
	return nil
}

func (m *MockSchemaLoader) GetTableNames(ctx context.Context) ([]string, error) {
	return []string{"users", "orders", "products", "categories"}, nil
}

// MockMetricsCollector Mock 指标收集器
type MockMetricsCollector struct{}

func (m *MockMetricsCollector) IncrementCounter(name string, labels map[string]string) {}
func (m *MockMetricsCollector) RecordHistogram(name string, value float64, labels map[string]string) {
}
func (m *MockMetricsCollector) SetGauge(name string, value float64, labels map[string]string) {}

// MockQueryProcessor Mock 查询处理器
type MockQueryProcessor struct {
	logger core.Logger
}

func (m *MockQueryProcessor) ProcessQuery(ctx context.Context, request *core.QueryRequest) (*core.QueryResponse, error) {
	// 模拟查询处理时间
	processingTime := time.Duration(10+len(request.Query)) * time.Millisecond
	time.Sleep(processingTime)

	// 模拟查询结果
	var data []map[string]any
	switch {
	case len(request.Query) > 50:
		// 复杂查询，返回更多数据
		for i := 0; i < 100; i++ {
			data = append(data, map[string]any{
				"id":   i + 1,
				"name": fmt.Sprintf("用户%d", i+1),
				"age":  20 + i%50,
			})
		}
	case len(request.Query) > 20:
		// 中等查询
		for i := 0; i < 10; i++ {
			data = append(data, map[string]any{
				"id":   i + 1,
				"name": fmt.Sprintf("用户%d", i+1),
			})
		}
	default:
		// 简单查询
		data = append(data, map[string]any{
			"count": 42,
		})
	}

	return &core.QueryResponse{
		Success:     true,
		Data:        data,
		SQL:         fmt.Sprintf("SELECT * FROM users WHERE name LIKE '%%%s%%'", request.Query),
		Explanation: fmt.Sprintf("查询包含 '%s' 的用户", request.Query),
		Metadata: &core.QueryMetadata{
			ExecutionTime: processingTime,
			RowCount:      len(data),
		},
		RequestID: request.RequestID,
	}, nil
}

func (m *MockQueryProcessor) GetSuggestions(ctx context.Context, partial string) ([]string, error) {
	return []string{
		"查询用户信息",
		"统计订单数量",
		"分析销售数据",
	}, nil
}

func (m *MockQueryProcessor) ExplainQuery(ctx context.Context, query string) (*core.QueryExplanation, error) {
	return &core.QueryExplanation{
		Intent:     "查询",
		Tables:     []string{"users"},
		Columns:    []string{"id", "name", "email"},
		Conditions: []string{"name LIKE '%test%'"},
		Operations: []string{"SELECT"},
		Complexity: "simple",
	}, nil
}

// 辅助函数
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvDurationOrDefault(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

func getEnvBoolOrDefault(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	units := []string{"KB", "MB", "GB", "TB"}
	return fmt.Sprintf("%.1f %s", float64(bytes)/float64(div), units[exp])
}

// ZapLoggerAdapter zap logger 适配器，实现 core.Logger 接口
type ZapLoggerAdapter struct {
	logger *zap.Logger
}

// NewZapLoggerAdapter 创建 zap logger 适配器
func NewZapLoggerAdapter(logger *zap.Logger) core.Logger {
	return &ZapLoggerAdapter{logger: logger}
}

func (z *ZapLoggerAdapter) Debug(msg string, fields ...any) {
	z.logger.Debug(msg, z.convertFields(fields...)...)
}

func (z *ZapLoggerAdapter) Info(msg string, fields ...any) {
	z.logger.Info(msg, z.convertFields(fields...)...)
}

func (z *ZapLoggerAdapter) Warn(msg string, fields ...any) {
	z.logger.Warn(msg, z.convertFields(fields...)...)
}

func (z *ZapLoggerAdapter) Error(msg string, fields ...any) {
	z.logger.Error(msg, z.convertFields(fields...)...)
}

func (z *ZapLoggerAdapter) Fatal(msg string, fields ...any) {
	z.logger.Fatal(msg, z.convertFields(fields...)...)
}

// convertFields 将 any 类型的字段转换为 zap.Field
func (z *ZapLoggerAdapter) convertFields(fields ...any) []zap.Field {
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
