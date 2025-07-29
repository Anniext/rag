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

func TestNewMemoryCache(t *testing.T) {
	logger := &mockLogger{}
	metrics := &mockMetrics{}

	tests := []struct {
		name    string
		config  *CacheConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: &CacheConfig{
				MaxCacheSize: "10MB",
			},
			wantErr: false,
		},
		{
			name:    "nil config",
			config:  nil,
			wantErr: false,
		},
		{
			name: "invalid size format",
			config: &CacheConfig{
				MaxCacheSize: "invalid",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache, err := NewMemoryCache(tt.config, logger, metrics)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, cache)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, cache)
				if cache != nil {
					cache.Close()
				}
			}
		})
	}
}

func TestMemoryCache_BasicOperations(t *testing.T) {
	logger := &mockLogger{}
	metrics := &mockMetrics{}

	config := &CacheConfig{
		MaxCacheSize: "10MB",
	}

	cache, err := NewMemoryCache(config, logger, metrics)
	require.NoError(t, err)
	defer cache.Close()

	ctx := context.Background()

	// Test Set and Get
	key := "test_key"
	value := "test_value"
	ttl := time.Minute

	err = cache.Set(ctx, key, value, ttl)
	assert.NoError(t, err)

	result, err := cache.Get(ctx, key)
	assert.NoError(t, err)
	assert.Equal(t, value, result)

	// Test non-existent key
	_, err = cache.Get(ctx, "nonexistent")
	assert.Error(t, err)
	assert.Equal(t, core.ErrCacheKeyNotFound, err)

	// Test Delete
	err = cache.Delete(ctx, key)
	assert.NoError(t, err)

	_, err = cache.Get(ctx, key)
	assert.Error(t, err)
	assert.Equal(t, core.ErrCacheKeyNotFound, err)

	// Test delete non-existent key
	err = cache.Delete(ctx, "nonexistent")
	assert.Error(t, err)
	assert.Equal(t, core.ErrCacheKeyNotFound, err)
}

func TestMemoryCache_TTL(t *testing.T) {
	logger := &mockLogger{}
	metrics := &mockMetrics{}

	config := &CacheConfig{
		MaxCacheSize: "10MB",
	}

	cache, err := NewMemoryCache(config, logger, metrics)
	require.NoError(t, err)
	defer cache.Close()

	ctx := context.Background()

	// Test TTL expiration
	key := "ttl_test"
	value := "test_value"
	ttl := 100 * time.Millisecond

	err = cache.Set(ctx, key, value, ttl)
	assert.NoError(t, err)

	// Immediately get should work
	result, err := cache.Get(ctx, key)
	assert.NoError(t, err)
	assert.Equal(t, value, result)

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	// Should be expired now
	_, err = cache.Get(ctx, key)
	assert.Error(t, err)
	assert.Equal(t, core.ErrCacheKeyNotFound, err)
}

func TestMemoryCache_Clear(t *testing.T) {
	logger := &mockLogger{}
	metrics := &mockMetrics{}

	config := &CacheConfig{
		MaxCacheSize: "10MB",
	}

	cache, err := NewMemoryCache(config, logger, metrics)
	require.NoError(t, err)
	defer cache.Close()

	ctx := context.Background()

	// Add some entries
	cache.Set(ctx, "key1", "value1", time.Minute)
	cache.Set(ctx, "key2", "value2", time.Minute)
	cache.Set(ctx, "key3", "value3", time.Minute)

	// Verify they exist
	_, err = cache.Get(ctx, "key1")
	assert.NoError(t, err)

	// Clear cache
	err = cache.Clear(ctx)
	assert.NoError(t, err)

	// Verify all entries are gone
	_, err = cache.Get(ctx, "key1")
	assert.Error(t, err)
	_, err = cache.Get(ctx, "key2")
	assert.Error(t, err)
	_, err = cache.Get(ctx, "key3")
	assert.Error(t, err)

	assert.Equal(t, 0, cache.Size())
}

func TestMemoryCache_Exists(t *testing.T) {
	logger := &mockLogger{}
	metrics := &mockMetrics{}

	config := &CacheConfig{
		MaxCacheSize: "10MB",
	}

	cache, err := NewMemoryCache(config, logger, metrics)
	require.NoError(t, err)
	defer cache.Close()

	ctx := context.Background()

	key := "test_key"
	value := "test_value"

	// Key should not exist initially
	exists, err := cache.Exists(ctx, key)
	assert.NoError(t, err)
	assert.False(t, exists)

	// Set key
	cache.Set(ctx, key, value, time.Minute)

	// Key should exist now
	exists, err = cache.Exists(ctx, key)
	assert.NoError(t, err)
	assert.True(t, exists)

	// Test with expired key
	cache.Set(ctx, "expired_key", "value", 50*time.Millisecond)
	time.Sleep(100 * time.Millisecond)

	exists, err = cache.Exists(ctx, "expired_key")
	assert.NoError(t, err)
	assert.False(t, exists)
}

func TestMemoryCache_Keys(t *testing.T) {
	logger := &mockLogger{}
	metrics := &mockMetrics{}

	config := &CacheConfig{
		MaxCacheSize: "10MB",
	}

	cache, err := NewMemoryCache(config, logger, metrics)
	require.NoError(t, err)
	defer cache.Close()

	ctx := context.Background()

	// Add some keys
	cache.Set(ctx, "user:1", "alice", time.Minute)
	cache.Set(ctx, "user:2", "bob", time.Minute)
	cache.Set(ctx, "post:1", "hello", time.Minute)

	// Get all keys
	keys, err := cache.Keys(ctx, "*")
	assert.NoError(t, err)
	assert.Len(t, keys, 3)

	// Test pattern matching (simple implementation)
	keys, err = cache.Keys(ctx, "user:*")
	assert.NoError(t, err)
	assert.Len(t, keys, 2)

	keys, err = cache.Keys(ctx, "*:1")
	assert.NoError(t, err)
	assert.Len(t, keys, 2)
}

func TestMemoryCache_LRUEviction(t *testing.T) {
	logger := &mockLogger{}
	metrics := &mockMetrics{}

	// Small cache size to trigger eviction
	config := &CacheConfig{
		MaxCacheSize: "1KB",
	}

	cache, err := NewMemoryCache(config, logger, metrics)
	require.NoError(t, err)
	defer cache.Close()

	ctx := context.Background()

	// Fill cache beyond capacity
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("key_%d", i)
		value := fmt.Sprintf("value_%d_with_some_extra_data_to_make_it_larger", i)
		cache.Set(ctx, key, value, time.Minute)
	}

	// Cache should have evicted some entries
	stats := cache.GetStats()
	size := stats["size"].(int)
	assert.True(t, size < 100, "Cache should have evicted some entries")

	// Recently accessed items should still be there
	lastKey := "key_99"
	_, err = cache.Get(ctx, lastKey)
	assert.NoError(t, err, "Most recently added key should still exist")
}

func TestMemoryCache_AccessCount(t *testing.T) {
	logger := &mockLogger{}
	metrics := &mockMetrics{}

	config := &CacheConfig{
		MaxCacheSize: "10MB",
	}

	cache, err := NewMemoryCache(config, logger, metrics)
	require.NoError(t, err)
	defer cache.Close()

	ctx := context.Background()

	key := "test_key"
	value := "test_value"

	// Set key
	cache.Set(ctx, key, value, time.Minute)

	// Access multiple times
	for i := 0; i < 5; i++ {
		cache.Get(ctx, key)
	}

	// Check that access count is updated
	cache.mutex.RLock()
	entry := cache.data[key]
	cache.mutex.RUnlock()

	assert.NotNil(t, entry)
	assert.Equal(t, int64(5), entry.AccessCount)
}

func TestMemoryCache_Stats(t *testing.T) {
	logger := &mockLogger{}
	metrics := &mockMetrics{}

	config := &CacheConfig{
		MaxCacheSize: "10MB",
	}

	cache, err := NewMemoryCache(config, logger, metrics)
	require.NoError(t, err)
	defer cache.Close()

	ctx := context.Background()

	// Add some entries
	cache.Set(ctx, "key1", "value1", time.Minute)
	cache.Set(ctx, "key2", "value2", time.Minute)

	stats := cache.GetStats()
	assert.NotNil(t, stats)

	size, ok := stats["size"].(int)
	assert.True(t, ok)
	assert.Equal(t, 2, size)

	currentSize, ok := stats["current_size"].(int64)
	assert.True(t, ok)
	assert.True(t, currentSize > 0)

	maxSize, ok := stats["max_size"].(int64)
	assert.True(t, ok)
	assert.True(t, maxSize > 0)

	usageRatio, ok := stats["usage_ratio"].(float64)
	assert.True(t, ok)
	assert.True(t, usageRatio >= 0 && usageRatio <= 1)
}

func TestParseSize(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
		wantErr  bool
	}{
		{"", 100 * 1024 * 1024, false}, // default
		{"1024B", 1024, false},
		{"1KB", 1024, false},
		{"1MB", 1024 * 1024, false},
		{"1GB", 1024 * 1024 * 1024, false},
		{"10mb", 10 * 1024 * 1024, false},
		{"invalid", 0, true},
		{"1XB", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := parseSize(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		str     string
		pattern string
		match   bool
	}{
		{"hello", "*", true},
		{"hello", "hello", true},
		{"hello", "world", false},
		{"hello", "h*", true},
		{"hello", "*o", true},
		{"hello", "he*", true},
		{"hello", "*lo", true},
		{"hello", "h*o", false}, // not supported in simple implementation
		{"", "", true},
		{"hello", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.str+"_"+tt.pattern, func(t *testing.T) {
			result := matchPattern(tt.str, tt.pattern)
			assert.Equal(t, tt.match, result)
		})
	}
}
