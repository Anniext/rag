package cache

import (
	"context"
	"fmt"
	"testing"
	"time"

	"pumppill/rag/core"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockLogger 模拟日志记录器
type mockLogger struct{}

func (m *mockLogger) Debug(msg string, fields ...any) {}
func (m *mockLogger) Info(msg string, fields ...any)  {}
func (m *mockLogger) Warn(msg string, fields ...any)  {}
func (m *mockLogger) Error(msg string, fields ...any) {}
func (m *mockLogger) Fatal(msg string, fields ...any) {}

// mockMetrics 模拟指标收集器
type mockMetrics struct{}

func (m *mockMetrics) IncrementCounter(name string, labels map[string]string)               {}
func (m *mockMetrics) RecordHistogram(name string, value float64, labels map[string]string) {}
func (m *mockMetrics) SetGauge(name string, value float64, labels map[string]string)        {}

func TestNewManager(t *testing.T) {
	logger := &mockLogger{}
	metrics := &mockMetrics{}

	tests := []struct {
		name    string
		config  *CacheConfig
		wantErr bool
	}{
		{
			name: "memory cache",
			config: &CacheConfig{
				Type:         CacheTypeMemory,
				MaxCacheSize: "10MB",
			},
			wantErr: false,
		},
		{
			name:    "default config",
			config:  nil,
			wantErr: false,
		},
		{
			name: "invalid cache type",
			config: &CacheConfig{
				Type: "invalid",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager, err := NewManager(tt.config, logger, metrics)
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

func TestManager_BasicOperations(t *testing.T) {
	logger := &mockLogger{}
	metrics := &mockMetrics{}

	config := &CacheConfig{
		Type:         CacheTypeMemory,
		MaxCacheSize: "10MB",
		QueryTTL:     time.Minute,
		SchemaTTL:    time.Hour,
	}

	manager, err := NewManager(config, logger, metrics)
	require.NoError(t, err)
	defer manager.Close()

	ctx := context.Background()

	// Test Set and Get
	key := "test_key"
	value := "test_value"

	err = manager.Set(ctx, key, value, time.Minute)
	assert.NoError(t, err)

	result, err := manager.Get(ctx, key)
	assert.NoError(t, err)
	assert.Equal(t, value, result)

	// Test Delete
	err = manager.Delete(ctx, key)
	assert.NoError(t, err)

	_, err = manager.Get(ctx, key)
	assert.Error(t, err)
	assert.Equal(t, core.ErrCacheKeyNotFound, err)

	// Test Clear
	err = manager.Set(ctx, "key1", "value1", time.Minute)
	assert.NoError(t, err)
	err = manager.Set(ctx, "key2", "value2", time.Minute)
	assert.NoError(t, err)

	err = manager.Clear(ctx)
	assert.NoError(t, err)

	_, err = manager.Get(ctx, "key1")
	assert.Error(t, err)
	_, err = manager.Get(ctx, "key2")
	assert.Error(t, err)
}

func TestManager_SchemaCache(t *testing.T) {
	logger := &mockLogger{}
	metrics := &mockMetrics{}

	config := &CacheConfig{
		Type:         CacheTypeMemory,
		MaxCacheSize: "10MB",
		SchemaTTL:    time.Hour,
	}

	manager, err := NewManager(config, logger, metrics)
	require.NoError(t, err)
	defer manager.Close()

	ctx := context.Background()

	// Test Schema Cache
	schemaKey := "users"
	schemaValue := map[string]any{
		"table":   "users",
		"columns": []string{"id", "name", "email"},
	}

	err = manager.SetSchemaCache(ctx, schemaKey, schemaValue)
	assert.NoError(t, err)

	result, err := manager.GetSchemaCache(ctx, schemaKey)
	assert.NoError(t, err)
	assert.Equal(t, schemaValue, result)
}

func TestManager_QueryCache(t *testing.T) {
	logger := &mockLogger{}
	metrics := &mockMetrics{}

	config := &CacheConfig{
		Type:         CacheTypeMemory,
		MaxCacheSize: "10MB",
		QueryTTL:     time.Minute,
	}

	manager, err := NewManager(config, logger, metrics)
	require.NoError(t, err)
	defer manager.Close()

	ctx := context.Background()

	// Test Query Cache
	queryKey := "SELECT * FROM users"
	queryResult := []map[string]any{
		{"id": 1, "name": "Alice", "email": "alice@example.com"},
		{"id": 2, "name": "Bob", "email": "bob@example.com"},
	}

	err = manager.SetQueryCache(ctx, queryKey, queryResult)
	assert.NoError(t, err)

	result, err := manager.GetQueryCache(ctx, queryKey)
	assert.NoError(t, err)
	assert.Equal(t, queryResult, result)
}

func TestManager_Stats(t *testing.T) {
	logger := &mockLogger{}
	metrics := &mockMetrics{}

	config := &CacheConfig{
		Type:         CacheTypeMemory,
		MaxCacheSize: "10MB",
		QueryTTL:     time.Minute,
	}

	manager, err := NewManager(config, logger, metrics)
	require.NoError(t, err)
	defer manager.Close()

	ctx := context.Background()

	// 执行一些操作
	manager.Set(ctx, "key1", "value1", time.Minute)
	manager.Get(ctx, "key1")
	manager.Get(ctx, "nonexistent")

	stats := manager.GetStats()
	assert.NotNil(t, stats)
	assert.True(t, stats.Hits > 0)
	assert.True(t, stats.Misses > 0)
	assert.True(t, stats.Sets > 0)
}

func TestManager_TTL(t *testing.T) {
	logger := &mockLogger{}
	metrics := &mockMetrics{}

	config := &CacheConfig{
		Type:         CacheTypeMemory,
		MaxCacheSize: "10MB",
	}

	manager, err := NewManager(config, logger, metrics)
	require.NoError(t, err)
	defer manager.Close()

	ctx := context.Background()

	// Test TTL
	key := "ttl_test"
	value := "test_value"
	ttl := 100 * time.Millisecond

	err = manager.Set(ctx, key, value, ttl)
	assert.NoError(t, err)

	// 立即获取应该成功
	result, err := manager.Get(ctx, key)
	assert.NoError(t, err)
	assert.Equal(t, value, result)

	// 等待过期
	time.Sleep(150 * time.Millisecond)

	// 过期后获取应该失败
	_, err = manager.Get(ctx, key)
	assert.Error(t, err)
	assert.Equal(t, core.ErrCacheKeyNotFound, err)
}

func TestManager_ConcurrentAccess(t *testing.T) {
	logger := &mockLogger{}
	metrics := &mockMetrics{}

	config := &CacheConfig{
		Type:         CacheTypeMemory,
		MaxCacheSize: "10MB",
		QueryTTL:     time.Minute,
	}

	manager, err := NewManager(config, logger, metrics)
	require.NoError(t, err)
	defer manager.Close()

	ctx := context.Background()

	// 并发读写测试
	done := make(chan bool, 10)

	// 启动多个写协程
	for i := 0; i < 5; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				key := fmt.Sprintf("key_%d_%d", id, j)
				value := fmt.Sprintf("value_%d_%d", id, j)
				manager.Set(ctx, key, value, time.Minute)
			}
			done <- true
		}(i)
	}

	// 启动多个读协程
	for i := 0; i < 5; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				key := fmt.Sprintf("key_%d_%d", id, j)
				manager.Get(ctx, key)
			}
			done <- true
		}(i)
	}

	// 等待所有协程完成
	for i := 0; i < 10; i++ {
		<-done
	}

	// 验证没有竞态条件导致的问题
	stats := manager.GetStats()
	assert.NotNil(t, stats)
}
