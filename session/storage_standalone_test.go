package session

import (
	"testing"
	"time"
)

// 独立测试，不依赖core包的编译问题

// MockSessionMemory 模拟SessionMemory结构
type MockSessionMemory struct {
	SessionID    string
	UserID       string
	History      []interface{}
	Context      map[string]interface{}
	CreatedAt    time.Time
	LastAccessed time.Time
}

// MockLogger 简单的mock logger实现
type MockLoggerStandalone struct{}

func (m *MockLoggerStandalone) Debug(msg string, args ...interface{}) {}
func (m *MockLoggerStandalone) Info(msg string, args ...interface{})  {}
func (m *MockLoggerStandalone) Warn(msg string, args ...interface{})  {}
func (m *MockLoggerStandalone) Error(msg string, args ...interface{}) {}
func (m *MockLoggerStandalone) Fatal(msg string, args ...interface{}) {}

func TestStorageConfigDefaults(t *testing.T) {
	config := DefaultStorageConfig()

	if config.MaxMemoryMB != 512 {
		t.Errorf("Expected MaxMemoryMB to be 512, got %d", config.MaxMemoryMB)
	}

	if config.MaxSessions != 1000 {
		t.Errorf("Expected MaxSessions to be 1000, got %d", config.MaxSessions)
	}

	if config.CleanupInterval != 5*time.Minute {
		t.Errorf("Expected CleanupInterval to be 5 minutes, got %v", config.CleanupInterval)
	}

	if config.MemoryCheckInterval != 30*time.Second {
		t.Errorf("Expected MemoryCheckInterval to be 30 seconds, got %v", config.MemoryCheckInterval)
	}

	if config.GCThreshold != 0.8 {
		t.Errorf("Expected GCThreshold to be 0.8, got %f", config.GCThreshold)
	}

	if !config.LeakDetectionEnabled {
		t.Error("Expected LeakDetectionEnabled to be true")
	}
}

func TestMemoryStatsStructure(t *testing.T) {
	stats := MemoryStats{
		AllocMB:      100.5,
		TotalAllocMB: 200.0,
		SysMB:        150.0,
		NumGC:        10,
		GCPauseMs:    5.5,
		Goroutines:   50,
		Timestamp:    time.Now(),
	}

	if stats.AllocMB != 100.5 {
		t.Errorf("Expected AllocMB to be 100.5, got %f", stats.AllocMB)
	}

	if stats.Goroutines != 50 {
		t.Errorf("Expected Goroutines to be 50, got %d", stats.Goroutines)
	}
}

func TestSessionStatsStructure(t *testing.T) {
	stats := SessionStats{
		TotalSessions:   100,
		ActiveSessions:  80,
		ExpiredSessions: 20,
		MemoryUsageMB:   50.5,
		LastCleanup:     time.Now(),
	}

	if stats.TotalSessions != 100 {
		t.Errorf("Expected TotalSessions to be 100, got %d", stats.TotalSessions)
	}

	if stats.ActiveSessions != 80 {
		t.Errorf("Expected ActiveSessions to be 80, got %d", stats.ActiveSessions)
	}
}

func TestCacheEntryStructure(t *testing.T) {
	entry := CacheEntry{
		Key:        "test_key",
		Value:      "test_value",
		Size:       100,
		AccessTime: time.Now(),
		HitCount:   5,
		CreatedAt:  time.Now(),
	}

	if entry.Key != "test_key" {
		t.Errorf("Expected Key to be 'test_key', got %s", entry.Key)
	}

	if entry.Size != 100 {
		t.Errorf("Expected Size to be 100, got %d", entry.Size)
	}

	if entry.HitCount != 5 {
		t.Errorf("Expected HitCount to be 5, got %d", entry.HitCount)
	}
}

func TestLeakDetectorCreation(t *testing.T) {
	logger := &MockLoggerStandalone{}
	detector := NewLeakDetector(logger)

	if detector == nil {
		t.Fatal("Expected detector to be created")
	}

	if detector.maxHistory != 100 {
		t.Errorf("Expected maxHistory to be 100, got %d", detector.maxHistory)
	}

	if detector.threshold != 1.5 {
		t.Errorf("Expected threshold to be 1.5, got %f", detector.threshold)
	}

	if len(detector.history) != 0 {
		t.Errorf("Expected history to be empty initially, got %d items", len(detector.history))
	}
}

func TestLeakDetectorCheck(t *testing.T) {
	logger := &MockLoggerStandalone{}
	detector := NewLeakDetector(logger)
	defer detector.Close()

	// 添加一些正常的内存统计
	for i := 0; i < 5; i++ {
		stats := MemoryStats{
			AllocMB:    float64(10 + i),
			Goroutines: 10,
			Timestamp:  time.Now(),
		}
		detector.Check(stats)
	}

	if len(detector.history) != 5 {
		t.Errorf("Expected 5 history entries, got %d", len(detector.history))
	}

	// 测试历史记录限制
	for i := 0; i < 200; i++ {
		stats := MemoryStats{
			AllocMB:    float64(10 + i),
			Goroutines: 10,
			Timestamp:  time.Now(),
		}
		detector.Check(stats)
	}

	if len(detector.history) > detector.maxHistory {
		t.Errorf("Expected history to be limited to %d, got %d", detector.maxHistory, len(detector.history))
	}
}

func TestLeakDetectorReport(t *testing.T) {
	logger := &MockLoggerStandalone{}
	detector := NewLeakDetector(logger)
	defer detector.Close()

	// 测试空报告
	report := detector.GetLeakReport()
	if report["status"] != "no_data" {
		t.Error("Expected status to be 'no_data' for empty detector")
	}

	// 添加一些数据
	stats := MemoryStats{
		AllocMB:    100.0,
		Goroutines: 50,
		NumGC:      10,
		Timestamp:  time.Now(),
	}
	detector.Check(stats)

	report = detector.GetLeakReport()
	if report["current_alloc_mb"] != 100.0 {
		t.Errorf("Expected current_alloc_mb to be 100.0, got %v", report["current_alloc_mb"])
	}

	if report["current_goroutines"] != 50 {
		t.Errorf("Expected current_goroutines to be 50, got %v", report["current_goroutines"])
	}
}

func TestLeakDetection(t *testing.T) {
	logger := &MockLoggerStandalone{}
	detector := NewLeakDetector(logger)
	defer detector.Close()

	// 模拟内存泄漏模式 - 快速增长
	for i := 0; i < 15; i++ {
		stats := MemoryStats{
			AllocMB:    float64(10 * (i + 1)), // 快速增长
			Goroutines: 10 * (i + 1),          // Goroutine也快速增长
			Timestamp:  time.Now(),
		}
		detector.Check(stats)
	}

	if !detector.detectMemoryLeak() {
		t.Error("Expected to detect memory leak with rapid growth pattern")
	}

	report := detector.GetLeakReport()
	if !report["potential_leak"].(bool) {
		t.Error("Expected potential_leak to be true in report")
	}
}

func TestLeakDetectorClose(t *testing.T) {
	logger := &MockLoggerStandalone{}
	detector := NewLeakDetector(logger)

	// 添加一些数据
	stats := MemoryStats{
		AllocMB:   100.0,
		Timestamp: time.Now(),
	}
	detector.Check(stats)

	if len(detector.history) == 0 {
		t.Error("Expected history to have data before close")
	}

	detector.Close()

	if detector.history != nil {
		t.Error("Expected history to be nil after close")
	}
}
