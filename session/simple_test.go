package session

import (
	"testing"
	"time"
)

// SimpleMockLogger 简单的mock logger实现，不使用testify
type SimpleMockLogger struct{}

func (m *SimpleMockLogger) Debug(msg string, args ...interface{}) {}
func (m *SimpleMockLogger) Info(msg string, args ...interface{})  {}
func (m *SimpleMockLogger) Warn(msg string, args ...interface{})  {}
func (m *SimpleMockLogger) Error(msg string, args ...interface{}) {}
func (m *SimpleMockLogger) Fatal(msg string, args ...interface{}) {}

func TestSimpleStorageCreation(t *testing.T) {
	logger := &SimpleMockLogger{}
	config := DefaultStorageConfig()

	storage := NewStorage(config, logger)
	if storage == nil {
		t.Fatal("Expected storage to be created")
	}

	// 测试配置
	if storage.config.MaxMemoryMB != 512 {
		t.Errorf("Expected MaxMemoryMB to be 512, got %d", storage.config.MaxMemoryMB)
	}

	// 测试初始状态
	if storage.sessions == nil {
		t.Error("Expected sessions map to be initialized")
	}

	if storage.cache == nil {
		t.Error("Expected cache map to be initialized")
	}

	// 关闭存储
	storage.Close()
}

func TestSimpleMemoryStats(t *testing.T) {
	logger := &SimpleMockLogger{}
	storage := NewStorage(nil, logger)
	defer storage.Close()

	stats := storage.GetMemoryStats()

	if stats.AllocMB <= 0 {
		t.Error("Expected positive memory allocation")
	}

	if stats.Goroutines <= 0 {
		t.Error("Expected positive goroutine count")
	}

	if stats.Timestamp.IsZero() {
		t.Error("Expected timestamp to be set")
	}
}

func TestSimpleCacheOperations(t *testing.T) {
	logger := &SimpleMockLogger{}
	storage := NewStorage(nil, logger)
	defer storage.Close()

	// 测试设置缓存
	err := storage.CacheSet("key1", "value1", 100)
	if err != nil {
		t.Fatalf("Failed to set cache: %v", err)
	}

	// 测试获取缓存
	value, exists := storage.CacheGet("key1")
	if !exists {
		t.Error("Expected cache entry to exist")
	}
	if value != "value1" {
		t.Error("Expected cache value to match")
	}

	// 测试缓存删除
	storage.CacheDelete("key1")
	_, exists = storage.CacheGet("key1")
	if exists {
		t.Error("Expected cache entry to be deleted")
	}
}

func TestSimpleLeakDetector(t *testing.T) {
	logger := &SimpleMockLogger{}
	detector := NewLeakDetector(logger)
	defer detector.Close()

	// 测试基本功能
	if detector.maxHistory != 100 {
		t.Errorf("Expected maxHistory to be 100, got %d", detector.maxHistory)
	}

	// 添加一些数据
	stats := MemoryStats{
		AllocMB:    100.0,
		Goroutines: 50,
		NumGC:      10,
		Timestamp:  time.Now(),
	}
	detector.Check(stats)

	if len(detector.history) != 1 {
		t.Errorf("Expected 1 history entry, got %d", len(detector.history))
	}

	// 测试报告
	report := detector.GetLeakReport()
	if report["current_alloc_mb"] != 100.0 {
		t.Errorf("Expected current_alloc_mb to be 100.0, got %v", report["current_alloc_mb"])
	}
}

func TestSimpleCleanup(t *testing.T) {
	logger := &SimpleMockLogger{}
	storage := NewStorage(nil, logger)
	defer storage.Close()

	// 手动触发清理
	storage.TriggerCleanup()

	// 等待清理完成
	time.Sleep(100 * time.Millisecond)

	// 这个测试主要确保TriggerCleanup不会阻塞或崩溃
	stats := storage.GetSessionStats()
	if stats.TotalSessions < 0 {
		t.Error("Expected non-negative total sessions")
	}
}

func TestSimpleCacheStats(t *testing.T) {
	logger := &SimpleMockLogger{}
	storage := NewStorage(nil, logger)
	defer storage.Close()

	// 添加一些缓存条目
	storage.CacheSet("key1", "value1", 100)
	storage.CacheSet("key2", "value2", 200)

	// 访问一些条目
	storage.CacheGet("key1")
	storage.CacheGet("key1")        // 再次访问
	storage.CacheGet("nonexistent") // 不存在的键

	stats := storage.GetCacheStats()

	entries := stats["entries"].(int)
	if entries != 2 {
		t.Errorf("Expected 2 cache entries, got %d", entries)
	}

	totalSize := stats["total_size"].(int64)
	if totalSize != 300 {
		t.Errorf("Expected total size to be 300, got %d", totalSize)
	}

	hits := stats["hits"].(int64)
	if hits != 2 {
		t.Errorf("Expected 2 cache hits, got %d", hits)
	}

	misses := stats["misses"].(int64)
	if misses != 1 {
		t.Errorf("Expected 1 cache miss, got %d", misses)
	}

	hitRate := stats["hit_rate"].(float64)
	expectedHitRate := 2.0 / 3.0
	if hitRate < expectedHitRate-0.01 || hitRate > expectedHitRate+0.01 {
		t.Errorf("Expected hit rate to be approximately %f, got %f", expectedHitRate, hitRate)
	}
}
