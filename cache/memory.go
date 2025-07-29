package cache

import (
	"context"
	"fmt"
	"sync"
	"time"

	"pumppill/rag/core"
)

// MemoryCache 内存缓存实现
type MemoryCache struct {
	data        map[string]*CacheEntry
	mutex       sync.RWMutex
	config      *CacheConfig
	logger      core.Logger
	metrics     core.MetricsCollector
	stopCh      chan struct{}
	maxSize     int64
	currentSize int64
}

// NewMemoryCache 创建新的内存缓存
func NewMemoryCache(config *CacheConfig, logger core.Logger, metrics core.MetricsCollector) (*MemoryCache, error) {
	if config == nil {
		config = DefaultCacheConfig()
	}

	// 解析最大缓存大小
	maxSize, err := parseSize(config.MaxCacheSize)
	if err != nil {
		return nil, fmt.Errorf("invalid max cache size: %w", err)
	}

	cache := &MemoryCache{
		data:    make(map[string]*CacheEntry),
		config:  config,
		logger:  logger,
		metrics: metrics,
		stopCh:  make(chan struct{}),
		maxSize: maxSize,
	}

	// 启动清理协程
	go cache.cleanupExpired()

	logger.Info("Memory cache initialized", "max_size", config.MaxCacheSize)
	return cache, nil
}

// Get 获取缓存值
func (m *MemoryCache) Get(ctx context.Context, key string) (any, error) {
	start := time.Now()
	defer func() {
		if m.metrics != nil {
			m.metrics.RecordHistogram("cache_operation_duration_seconds",
				time.Since(start).Seconds(),
				map[string]string{"operation": "get", "backend": "memory"})
		}
	}()

	m.mutex.RLock()
	entry, exists := m.data[key]
	m.mutex.RUnlock()

	if !exists {
		m.recordMetric("cache_miss", "memory")
		return nil, core.ErrCacheKeyNotFound
	}

	// 检查是否过期
	if entry.IsExpired() {
		// 异步删除过期条目
		go func() {
			m.mutex.Lock()
			delete(m.data, key)
			m.currentSize -= m.estimateSize(entry)
			m.mutex.Unlock()
		}()

		m.recordMetric("cache_miss", "memory")
		return nil, core.ErrCacheKeyNotFound
	}

	// 更新访问统计
	m.mutex.Lock()
	entry.AccessCount++
	entry.LastAccess = time.Now()
	m.mutex.Unlock()

	m.recordMetric("cache_hit", "memory")
	return entry.Value, nil
}

// Set 设置缓存值
func (m *MemoryCache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	start := time.Now()
	defer func() {
		if m.metrics != nil {
			m.metrics.RecordHistogram("cache_operation_duration_seconds",
				time.Since(start).Seconds(),
				map[string]string{"operation": "set", "backend": "memory"})
		}
	}()

	now := time.Now()
	entry := &CacheEntry{
		Key:         key,
		Value:       value,
		TTL:         ttl,
		CreatedAt:   now,
		ExpiresAt:   now.Add(ttl),
		AccessCount: 0,
		LastAccess:  now,
	}

	entrySize := m.estimateSize(entry)

	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 检查是否需要清理空间
	if m.currentSize+entrySize > m.maxSize {
		m.evictLRU(entrySize)
	}

	// 如果键已存在，先减去旧条目的大小
	if oldEntry, exists := m.data[key]; exists {
		m.currentSize -= m.estimateSize(oldEntry)
	}

	m.data[key] = entry
	m.currentSize += entrySize

	m.recordMetric("cache_set", "memory")
	return nil
}

// Delete 删除缓存值
func (m *MemoryCache) Delete(ctx context.Context, key string) error {
	start := time.Now()
	defer func() {
		if m.metrics != nil {
			m.metrics.RecordHistogram("cache_operation_duration_seconds",
				time.Since(start).Seconds(),
				map[string]string{"operation": "delete", "backend": "memory"})
		}
	}()

	m.mutex.Lock()
	defer m.mutex.Unlock()

	entry, exists := m.data[key]
	if !exists {
		m.recordMetric("cache_miss", "memory")
		return core.ErrCacheKeyNotFound
	}

	delete(m.data, key)
	m.currentSize -= m.estimateSize(entry)

	m.recordMetric("cache_delete", "memory")
	return nil
}

// Clear 清空缓存
func (m *MemoryCache) Clear(ctx context.Context) error {
	start := time.Now()
	defer func() {
		if m.metrics != nil {
			m.metrics.RecordHistogram("cache_operation_duration_seconds",
				time.Since(start).Seconds(),
				map[string]string{"operation": "clear", "backend": "memory"})
		}
	}()

	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.data = make(map[string]*CacheEntry)
	m.currentSize = 0

	m.recordMetric("cache_clear", "memory")
	return nil
}

// Exists 检查键是否存在
func (m *MemoryCache) Exists(ctx context.Context, key string) (bool, error) {
	m.mutex.RLock()
	entry, exists := m.data[key]
	m.mutex.RUnlock()

	if !exists {
		return false, nil
	}

	// 检查是否过期
	if entry.IsExpired() {
		// 异步删除过期条目
		go func() {
			m.mutex.Lock()
			delete(m.data, key)
			m.currentSize -= m.estimateSize(entry)
			m.mutex.Unlock()
		}()
		return false, nil
	}

	return true, nil
}

// Keys 获取所有键
func (m *MemoryCache) Keys(ctx context.Context, pattern string) ([]string, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var keys []string
	for key, entry := range m.data {
		if !entry.IsExpired() {
			if pattern == "*" || matchPattern(key, pattern) {
				keys = append(keys, key)
			}
		}
	}

	return keys, nil
}

// Size 获取缓存大小
func (m *MemoryCache) Size() int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return len(m.data)
}

// GetStats 获取缓存统计信息
func (m *MemoryCache) GetStats() map[string]any {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	stats := map[string]any{
		"size":         len(m.data),
		"current_size": m.currentSize,
		"max_size":     m.maxSize,
		"usage_ratio":  float64(m.currentSize) / float64(m.maxSize),
	}

	return stats
}

// Close 关闭缓存
func (m *MemoryCache) Close() error {
	close(m.stopCh)
	return nil
}

// cleanupExpired 清理过期条目
func (m *MemoryCache) cleanupExpired() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.removeExpired()
		case <-m.stopCh:
			return
		}
	}
}

// removeExpired 移除过期条目
func (m *MemoryCache) removeExpired() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	now := time.Now()
	var expiredKeys []string

	for key, entry := range m.data {
		if now.After(entry.ExpiresAt) {
			expiredKeys = append(expiredKeys, key)
		}
	}

	for _, key := range expiredKeys {
		if entry, exists := m.data[key]; exists {
			delete(m.data, key)
			m.currentSize -= m.estimateSize(entry)
		}
	}

	if len(expiredKeys) > 0 {
		m.logger.Debug("Cleaned up expired cache entries", "count", len(expiredKeys))
		if m.metrics != nil {
			m.metrics.IncrementCounter("cache_expired_total",
				map[string]string{"backend": "memory"})
		}
	}
}

// evictLRU 使用 LRU 策略驱逐条目
func (m *MemoryCache) evictLRU(neededSize int64) {
	// 计算需要释放的空间
	targetSize := m.maxSize - neededSize
	if targetSize < 0 {
		targetSize = m.maxSize / 2 // 释放一半空间
	}

	// 收集所有条目并按最后访问时间排序
	type entryInfo struct {
		key        string
		entry      *CacheEntry
		size       int64
		lastAccess time.Time
	}

	var entries []entryInfo
	for key, entry := range m.data {
		entries = append(entries, entryInfo{
			key:        key,
			entry:      entry,
			size:       m.estimateSize(entry),
			lastAccess: entry.LastAccess,
		})
	}

	// 按最后访问时间排序（最久未访问的在前面）
	for i := 0; i < len(entries)-1; i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[i].lastAccess.After(entries[j].lastAccess) {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}

	// 驱逐最久未访问的条目
	var evictedCount int
	var freedSize int64
	for _, info := range entries {
		if m.currentSize-freedSize <= targetSize {
			break
		}

		delete(m.data, info.key)
		freedSize += info.size
		evictedCount++
	}

	m.currentSize -= freedSize

	if evictedCount > 0 {
		m.logger.Debug("Evicted cache entries",
			"count", evictedCount,
			"freed_size", freedSize)
		if m.metrics != nil {
			m.metrics.IncrementCounter("cache_evicted_total",
				map[string]string{"backend": "memory"})
		}
	}
}

// estimateSize 估算条目大小
func (m *MemoryCache) estimateSize(entry *CacheEntry) int64 {
	size := int64(len(entry.Key))

	switch v := entry.Value.(type) {
	case string:
		size += int64(len(v))
	case []byte:
		size += int64(len(v))
	case int, int32, int64, float32, float64, bool:
		size += 8
	default:
		// 对于复杂类型，使用一个估算值
		size += 100
	}

	return size + 64 // 加上结构体本身的大小
}

// recordMetric 记录指标
func (m *MemoryCache) recordMetric(operation, backend string) {
	if m.metrics != nil {
		m.metrics.IncrementCounter("cache_operations_total",
			map[string]string{"operation": operation, "backend": backend})
	}
}

// parseSize 解析大小字符串
func parseSize(sizeStr string) (int64, error) {
	if sizeStr == "" {
		return 100 * 1024 * 1024, nil // 默认 100MB
	}

	var size int64
	var unit string

	n, err := fmt.Sscanf(sizeStr, "%d%s", &size, &unit)
	if err != nil || n != 2 {
		return 0, fmt.Errorf("invalid size format: %s", sizeStr)
	}

	switch unit {
	case "B", "b":
		return size, nil
	case "KB", "kb", "K", "k":
		return size * 1024, nil
	case "MB", "mb", "M", "m":
		return size * 1024 * 1024, nil
	case "GB", "gb", "G", "g":
		return size * 1024 * 1024 * 1024, nil
	default:
		return 0, fmt.Errorf("unsupported size unit: %s", unit)
	}
}

// matchPattern 简单的模式匹配
func matchPattern(str, pattern string) bool {
	if pattern == "*" {
		return true
	}

	// 简单实现，只支持前缀和后缀匹配
	if len(pattern) == 0 {
		return len(str) == 0
	}

	if pattern[0] == '*' {
		suffix := pattern[1:]
		return len(str) >= len(suffix) && str[len(str)-len(suffix):] == suffix
	}

	if pattern[len(pattern)-1] == '*' {
		prefix := pattern[:len(pattern)-1]
		return len(str) >= len(prefix) && str[:len(prefix)] == prefix
	}

	return str == pattern
}
