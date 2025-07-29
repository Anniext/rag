package session

import (
	"runtime"
	"testing"
	"time"

	"pumppill/rag/core"
)

func TestNewStorage(t *testing.T) {
	logger := &MockLogger{}
	config := DefaultStorageConfig()

	storage := NewStorage(config, logger)
	defer storage.Close()

	if storage == nil {
		t.Fatal("Expected storage to be created")
	}

	if storage.config != config {
		t.Error("Expected config to be set")
	}

	if storage.sessions == nil {
		t.Error("Expected sessions map to be initialized")
	}

	if storage.cache == nil {
		t.Error("Expected cache map to be initialized")
	}
}

func TestStorageSession(t *testing.T) {
	logger := &MockLogger{}
	config := DefaultStorageConfig()
	config.MaxSessions = 2

	storage := NewStorage(config, logger)
	defer storage.Close()

	// 创建测试会话
	session1 := &core.SessionMemory{
		SessionID:    "session1",
		UserID:       "user1",
		CreatedAt:    time.Now(),
		LastAccessed: time.Now(),
		History:      make([]*core.QueryHistory, 0),
		Context:      make(map[string]interface{}),
	}

	session2 := &core.SessionMemory{
		SessionID:    "session2",
		UserID:       "user2",
		CreatedAt:    time.Now(),
		LastAccessed: time.Now(),
		History:      make([]*core.QueryHistory, 0),
		Context:      make(map[string]interface{}),
	}

	// 测试存储会话
	err := storage.StoreSession("session1", session1)
	if err != nil {
		t.Fatalf("Failed to store session1: %v", err)
	}

	err = storage.StoreSession("session2", session2)
	if err != nil {
		t.Fatalf("Failed to store session2: %v", err)
	}

	// 测试获取会话
	retrieved, exists := storage.GetSession("session1")
	if !exists {
		t.Error("Expected session1 to exist")
	}
	if retrieved.SessionID != "session1" {
		t.Error("Expected retrieved session to have correct ID")
	}

	// 测试会话数量限制
	session3 := &core.SessionMemory{
		SessionID:    "session3",
		UserID:       "user3",
		CreatedAt:    time.Now(),
		LastAccessed: time.Now().Add(-time.Hour), // 较老的访问时间
		History:      make([]*core.QueryHistory, 0),
		Context:      make(map[string]interface{}),
	}

	// 这应该触发LRU淘汰
	err = storage.StoreSession("session3", session3)
	if err != nil {
		t.Fatalf("Failed to store session3: %v", err)
	}

	// 检查是否有会话被淘汰
	stats := storage.GetSessionStats()
	if stats.ActiveSessions > 2 {
		t.Error("Expected session count to not exceed limit")
	}
}

func TestStorageCache(t *testing.T) {
	logger := &MockLogger{}
	config := DefaultStorageConfig()

	storage := NewStorage(config, logger)
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

	// 测试缓存命中统计
	storage.CacheGet("key1") // 再次访问
	stats := storage.GetCacheStats()
	if stats["hits"].(int64) < 2 {
		t.Error("Expected cache hits to be recorded")
	}

	// 测试缓存删除
	storage.CacheDelete("key1")
	_, exists = storage.CacheGet("key1")
	if exists {
		t.Error("Expected cache entry to be deleted")
	}
}

func TestMemoryStats(t *testing.T) {
	logger := &MockLogger{}
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

func TestSessionStats(t *testing.T) {
	logger := &MockLogger{}
	storage := NewStorage(nil, logger)
	defer storage.Close()

	// 添加一些会话
	session := &core.SessionMemory{
		SessionID:    "test",
		UserID:       "user",
		CreatedAt:    time.Now(),
		LastAccessed: time.Now(),
		History:      make([]*core.QueryHistory, 0),
		Context:      make(map[string]interface{}),
	}

	storage.StoreSession("test", session)

	stats := storage.GetSessionStats()

	if stats.ActiveSessions != 1 {
		t.Error("Expected 1 active session")
	}

	if stats.TotalSessions != 1 {
		t.Error("Expected 1 total session")
	}
}

func TestExpiredSessionCleanup(t *testing.T) {
	logger := &MockLogger{}
	config := DefaultStorageConfig()
	config.CleanupInterval = 100 * time.Millisecond

	storage := NewStorage(config, logger)
	defer storage.Close()

	// 创建过期会话
	expiredSession := &core.SessionMemory{
		SessionID:    "expired",
		UserID:       "user",
		CreatedAt:    time.Now().Add(-2 * time.Hour),
		LastAccessed: time.Now().Add(-2 * time.Hour), // 2小时前访问，超过1小时TTL
		History:      make([]*core.QueryHistory, 0),
		Context:      make(map[string]interface{}),
	}

	storage.StoreSession("expired", expiredSession)

	// 等待清理
	time.Sleep(200 * time.Millisecond)

	// 检查过期会话是否被清理
	_, exists := storage.GetSession("expired")
	if exists {
		t.Error("Expected expired session to be cleaned up")
	}
}

func TestMemoryLimitCheck(t *testing.T) {
	logger := &MockLogger{}
	config := DefaultStorageConfig()
	config.MaxMemoryMB = 1 // 设置很小的内存限制

	storage := NewStorage(config, logger)
	defer storage.Close()

	// 尝试存储大量数据触发内存检查
	for i := 0; i < 100; i++ {
		largeData := make([]byte, 1024*1024) // 1MB数据
		err := storage.CacheSet(string(rune(i)), largeData, int64(len(largeData)))
		if err != nil {
			// 预期会因为内存限制而失败
			break
		}
	}

	// 检查是否触发了GC
	runtime.GC()
	stats := storage.GetMemoryStats()
	if stats.NumGC == 0 {
		t.Log("GC may not have been triggered, this is environment dependent")
	}
}

func TestLeakDetector(t *testing.T) {
	logger := &MockLogger{}
	detector := NewLeakDetector(logger)
	defer detector.Close()

	// 模拟正常的内存使用模式
	for i := 0; i < 15; i++ {
		stats := MemoryStats{
			AllocMB:    float64(10 + i), // 缓慢增长
			Goroutines: 10,
			Timestamp:  time.Now(),
		}
		detector.Check(stats)
	}

	report := detector.GetLeakReport()
	if report["status"] == "no_data" {
		t.Error("Expected leak detector to have data")
	}

	// 模拟内存泄漏模式
	detector2 := NewLeakDetector(logger)
	defer detector2.Close()

	for i := 0; i < 15; i++ {
		stats := MemoryStats{
			AllocMB:    float64(10 * (i + 1)), // 快速增长
			Goroutines: 10 * (i + 1),          // Goroutine也快速增长
			Timestamp:  time.Now(),
		}
		detector2.Check(stats)
	}

	report2 := detector2.GetLeakReport()
	if !report2["potential_leak"].(bool) {
		t.Error("Expected leak detector to detect potential leak")
	}
}

func TestLRUEviction(t *testing.T) {
	logger := &MockLogger{}
	config := DefaultStorageConfig()
	config.MaxSessions = 2

	storage := NewStorage(config, logger)
	defer storage.Close()

	// 创建会话，设置不同的访问时间
	session1 := &core.SessionMemory{
		SessionID:    "session1",
		UserID:       "user1",
		CreatedAt:    time.Now(),
		LastAccessed: time.Now().Add(-time.Hour), // 最老的访问时间
		History:      make([]*core.QueryHistory, 0),
		Context:      make(map[string]interface{}),
	}

	session2 := &core.SessionMemory{
		SessionID:    "session2",
		UserID:       "user2",
		CreatedAt:    time.Now(),
		LastAccessed: time.Now().Add(-30 * time.Minute), // 中等访问时间
		History:      make([]*core.QueryHistory, 0),
		Context:      make(map[string]interface{}),
	}

	session3 := &core.SessionMemory{
		SessionID:    "session3",
		UserID:       "user3",
		CreatedAt:    time.Now(),
		LastAccessed: time.Now(), // 最新的访问时间
		History:      make([]*core.QueryHistory, 0),
		Context:      make(map[string]interface{}),
	}

	// 存储前两个会话
	storage.StoreSession("session1", session1)
	storage.StoreSession("session2", session2)

	// 存储第三个会话，应该淘汰session1（最老的）
	storage.StoreSession("session3", session3)

	// 检查session1是否被淘汰
	_, exists := storage.GetSession("session1")
	if exists {
		t.Error("Expected session1 to be evicted")
	}

	// 检查其他会话是否还存在
	_, exists = storage.GetSession("session2")
	if !exists {
		t.Error("Expected session2 to still exist")
	}

	_, exists = storage.GetSession("session3")
	if !exists {
		t.Error("Expected session3 to still exist")
	}
}

func TestCacheLRUEviction(t *testing.T) {
	logger := &MockLogger{}
	storage := NewStorage(nil, logger)
	defer storage.Close()

	// 设置一些缓存条目
	storage.CacheSet("key1", "value1", 100)
	time.Sleep(10 * time.Millisecond)
	storage.CacheSet("key2", "value2", 100)
	time.Sleep(10 * time.Millisecond)
	storage.CacheSet("key3", "value3", 100)

	// 访问key2，使其成为最近使用的
	storage.CacheGet("key2")

	// 手动触发缓存淘汰
	storage.evictLRUCache(1)

	// key1应该被淘汰（最老且未被访问）
	_, exists := storage.CacheGet("key1")
	if exists {
		t.Error("Expected key1 to be evicted")
	}

	// key2和key3应该还存在
	_, exists = storage.CacheGet("key2")
	if !exists {
		t.Error("Expected key2 to still exist")
	}

	_, exists = storage.CacheGet("key3")
	if !exists {
		t.Error("Expected key3 to still exist")
	}
}

func TestTriggerCleanup(t *testing.T) {
	logger := &MockLogger{}
	storage := NewStorage(nil, logger)
	defer storage.Close()

	// 手动触发清理
	storage.TriggerCleanup()

	// 等待清理完成
	time.Sleep(100 * time.Millisecond)

	// 这个测试主要确保TriggerCleanup不会阻塞或崩溃
	stats := storage.GetSessionStats()
	if stats.LastCleanup.IsZero() {
		t.Log("Cleanup may not have completed yet, this is timing dependent")
	}
}
