package session

import (
	"fmt"
	"github.com/Anniext/rag/core"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// StorageConfig 存储配置
type StorageConfig struct {
	MaxMemoryMB          int64         `yaml:"max_memory_mb" json:"max_memory_mb"`                   // 最大内存使用量(MB)
	MaxSessions          int           `yaml:"max_sessions" json:"max_sessions"`                     // 最大会话数量
	CleanupInterval      time.Duration `yaml:"cleanup_interval" json:"cleanup_interval"`             // 清理间隔
	MemoryCheckInterval  time.Duration `yaml:"memory_check_interval" json:"memory_check_interval"`   // 内存检查间隔
	GCThreshold          float64       `yaml:"gc_threshold" json:"gc_threshold"`                     // GC触发阈值
	LeakDetectionEnabled bool          `yaml:"leak_detection_enabled" json:"leak_detection_enabled"` // 是否启用内存泄漏检测
}

// DefaultStorageConfig 默认存储配置
func DefaultStorageConfig() *StorageConfig {
	return &StorageConfig{
		MaxMemoryMB:          512,
		MaxSessions:          1000,
		CleanupInterval:      5 * time.Minute,
		MemoryCheckInterval:  30 * time.Second,
		GCThreshold:          0.8,
		LeakDetectionEnabled: true,
	}
}

// MemoryStats 内存统计信息
type MemoryStats struct {
	AllocMB      float64   `json:"alloc_mb"`       // 当前分配内存(MB)
	TotalAllocMB float64   `json:"total_alloc_mb"` // 总分配内存(MB)
	SysMB        float64   `json:"sys_mb"`         // 系统内存(MB)
	NumGC        uint32    `json:"num_gc"`         // GC次数
	GCPauseMs    float64   `json:"gc_pause_ms"`    // GC暂停时间(ms)
	Goroutines   int       `json:"goroutines"`     // Goroutine数量
	Timestamp    time.Time `json:"timestamp"`      // 时间戳
}

// SessionStats 会话统计信息
type SessionStats struct {
	TotalSessions   int64     `json:"total_sessions"`   // 总会话数
	ActiveSessions  int64     `json:"active_sessions"`  // 活跃会话数
	ExpiredSessions int64     `json:"expired_sessions"` // 过期会话数
	MemoryUsageMB   float64   `json:"memory_usage_mb"`  // 内存使用量(MB)
	LastCleanup     time.Time `json:"last_cleanup"`     // 最后清理时间
}

// CacheEntry 缓存条目
type CacheEntry struct {
	Key        string      `json:"key"`
	Value      interface{} `json:"value"`
	Size       int64       `json:"size"`        // 条目大小(字节)
	AccessTime time.Time   `json:"access_time"` // 最后访问时间
	HitCount   int64       `json:"hit_count"`   // 命中次数
	CreatedAt  time.Time   `json:"created_at"`  // 创建时间
}

// Storage 存储管理器
type Storage struct {
	config     *StorageConfig
	sessions   map[string]*core.SessionMemory
	cache      map[string]*CacheEntry
	mutex      sync.RWMutex
	cacheMutex sync.RWMutex

	// 统计信息
	sessionStats SessionStats
	memoryStats  MemoryStats

	// 原子计数器
	totalSessions   int64
	activeSessions  int64
	expiredSessions int64
	cacheHits       int64
	cacheMisses     int64

	// 控制通道
	stopCh    chan struct{}
	cleanupCh chan struct{}

	// 内存泄漏检测
	leakDetector *LeakDetector

	logger core.Logger
}

// NewStorage 创建存储管理器
func NewStorage(config *StorageConfig, logger core.Logger) *Storage {
	if config == nil {
		config = DefaultStorageConfig()
	}

	storage := &Storage{
		config:    config,
		sessions:  make(map[string]*core.SessionMemory),
		cache:     make(map[string]*CacheEntry),
		stopCh:    make(chan struct{}),
		cleanupCh: make(chan struct{}),
		logger:    logger,
	}

	if config.LeakDetectionEnabled {
		storage.leakDetector = NewLeakDetector(logger)
	}

	// 启动后台任务
	go storage.backgroundCleanup()
	go storage.memoryMonitor()

	return storage
}

// StoreSession 存储会话
func (s *Storage) StoreSession(sessionID string, session *core.SessionMemory) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// 检查会话数量限制
	if len(s.sessions) >= s.config.MaxSessions {
		if err := s.evictLRUSessions(1); err != nil {
			return fmt.Errorf("failed to evict sessions: %w", err)
		}
	}

	// 检查内存使用量
	if err := s.checkMemoryUsage(); err != nil {
		return fmt.Errorf("memory usage check failed: %w", err)
	}

	s.sessions[sessionID] = session
	atomic.AddInt64(&s.totalSessions, 1)
	atomic.AddInt64(&s.activeSessions, 1)

	s.logger.Debug("Session stored", "session_id", sessionID)
	return nil
}

// GetSession 获取会话
func (s *Storage) GetSession(sessionID string) (*core.SessionMemory, bool) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	session, exists := s.sessions[sessionID]
	if !exists {
		return nil, false
	}

	// 检查会话是否过期 (使用1小时作为默认TTL)
	if time.Since(session.LastAccessed) > time.Hour {
		s.mutex.RUnlock()
		s.mutex.Lock()
		delete(s.sessions, sessionID)
		atomic.AddInt64(&s.activeSessions, -1)
		atomic.AddInt64(&s.expiredSessions, 1)
		s.mutex.Unlock()
		s.mutex.RLock()
		return nil, false
	}

	// 更新访问时间
	session.LastAccessed = time.Now()
	return session, true
}

// RemoveSession 移除会话
func (s *Storage) RemoveSession(sessionID string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, exists := s.sessions[sessionID]; exists {
		delete(s.sessions, sessionID)
		atomic.AddInt64(&s.activeSessions, -1)
		s.logger.Debug("Session removed", "session_id", sessionID)
	}
}

// CacheSet 设置缓存
func (s *Storage) CacheSet(key string, value interface{}, size int64) error {
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()

	// 检查内存使用量
	if err := s.checkMemoryUsage(); err != nil {
		// 尝试清理缓存
		s.evictLRUCache(1)
	}

	entry := &CacheEntry{
		Key:        key,
		Value:      value,
		Size:       size,
		AccessTime: time.Now(),
		HitCount:   0,
		CreatedAt:  time.Now(),
	}

	s.cache[key] = entry
	s.logger.Debug("Cache entry set", "key", key, "size", size)
	return nil
}

// CacheGet 获取缓存
func (s *Storage) CacheGet(key string) (interface{}, bool) {
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()

	entry, exists := s.cache[key]
	if !exists {
		atomic.AddInt64(&s.cacheMisses, 1)
		return nil, false
	}

	// 更新访问信息
	entry.AccessTime = time.Now()
	entry.HitCount++
	atomic.AddInt64(&s.cacheHits, 1)

	return entry.Value, true
}

// CacheDelete 删除缓存
func (s *Storage) CacheDelete(key string) {
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()

	delete(s.cache, key)
	s.logger.Debug("Cache entry deleted", "key", key)
}

// GetMemoryStats 获取内存统计信息
func (s *Storage) GetMemoryStats() MemoryStats {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return MemoryStats{
		AllocMB:      float64(m.Alloc) / 1024 / 1024,
		TotalAllocMB: float64(m.TotalAlloc) / 1024 / 1024,
		SysMB:        float64(m.Sys) / 1024 / 1024,
		NumGC:        m.NumGC,
		GCPauseMs:    float64(m.PauseNs[(m.NumGC+255)%256]) / 1000000,
		Goroutines:   runtime.NumGoroutine(),
		Timestamp:    time.Now(),
	}
}

// GetSessionStats 获取会话统计信息
func (s *Storage) GetSessionStats() SessionStats {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	var memoryUsage float64
	for _, session := range s.sessions {
		memoryUsage += s.estimateSessionMemoryUsage(session)
	}

	return SessionStats{
		TotalSessions:   atomic.LoadInt64(&s.totalSessions),
		ActiveSessions:  atomic.LoadInt64(&s.activeSessions),
		ExpiredSessions: atomic.LoadInt64(&s.expiredSessions),
		MemoryUsageMB:   memoryUsage / 1024 / 1024,
		LastCleanup:     s.sessionStats.LastCleanup,
	}
}

// checkMemoryUsage 检查内存使用量
func (s *Storage) checkMemoryUsage() error {
	stats := s.GetMemoryStats()
	maxMemoryBytes := s.config.MaxMemoryMB * 1024 * 1024

	if stats.AllocMB*1024*1024 > float64(maxMemoryBytes)*s.config.GCThreshold {
		s.logger.Warn("Memory usage high, triggering GC",
			"current_mb", stats.AllocMB,
			"max_mb", s.config.MaxMemoryMB,
			"threshold", s.config.GCThreshold)

		runtime.GC()

		// 重新检查
		newStats := s.GetMemoryStats()
		if newStats.AllocMB*1024*1024 > float64(maxMemoryBytes) {
			return core.NewRAGError(core.ErrorTypeInternal, "MEMORY_LIMIT_EXCEEDED", "内存使用量超出限制")
		}
	}

	return nil
}

// evictLRUSessions 淘汰最近最少使用的会话
func (s *Storage) evictLRUSessions(count int) error {
	if len(s.sessions) == 0 {
		return nil
	}

	// 按最后访问时间排序
	type sessionInfo struct {
		id           string
		lastAccessed time.Time
	}

	var sessions []sessionInfo
	for id, session := range s.sessions {
		sessions = append(sessions, sessionInfo{
			id:           id,
			lastAccessed: session.LastAccessed,
		})
	}

	// 排序，最老的在前面
	for i := 0; i < len(sessions)-1; i++ {
		for j := i + 1; j < len(sessions); j++ {
			if sessions[i].lastAccessed.After(sessions[j].lastAccessed) {
				sessions[i], sessions[j] = sessions[j], sessions[i]
			}
		}
	}

	// 删除最老的会话
	evicted := 0
	for i := 0; i < len(sessions) && evicted < count; i++ {
		delete(s.sessions, sessions[i].id)
		atomic.AddInt64(&s.activeSessions, -1)
		evicted++
		s.logger.Debug("Session evicted", "session_id", sessions[i].id)
	}

	return nil
}

// evictLRUCache 淘汰最近最少使用的缓存
func (s *Storage) evictLRUCache(count int) {
	if len(s.cache) == 0 {
		return
	}

	// 按访问时间排序
	type cacheInfo struct {
		key        string
		accessTime time.Time
		hitCount   int64
	}

	var entries []cacheInfo
	for key, entry := range s.cache {
		entries = append(entries, cacheInfo{
			key:        key,
			accessTime: entry.AccessTime,
			hitCount:   entry.HitCount,
		})
	}

	// 排序，优先淘汰访问时间最老且命中次数最少的
	for i := 0; i < len(entries)-1; i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[i].accessTime.After(entries[j].accessTime) ||
				(entries[i].accessTime.Equal(entries[j].accessTime) && entries[i].hitCount > entries[j].hitCount) {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}

	// 删除最老的缓存条目
	evicted := 0
	for i := 0; i < len(entries) && evicted < count; i++ {
		delete(s.cache, entries[i].key)
		evicted++
		s.logger.Debug("Cache entry evicted", "key", entries[i].key)
	}
}

// backgroundCleanup 后台清理任务
func (s *Storage) backgroundCleanup() {
	ticker := time.NewTicker(s.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.cleanup()
		case <-s.cleanupCh:
			s.cleanup()
		case <-s.stopCh:
			return
		}
	}
}

// cleanup 执行清理操作
func (s *Storage) cleanup() {
	s.logger.Debug("Starting cleanup")

	// 清理过期会话
	s.cleanupExpiredSessions()

	// 清理过期缓存
	s.cleanupExpiredCache()

	// 检查内存使用量
	if err := s.checkMemoryUsage(); err != nil {
		s.logger.Warn("Memory usage high after cleanup", "error", err)
	}

	// 更新统计信息
	s.sessionStats.LastCleanup = time.Now()

	s.logger.Debug("Cleanup completed")
}

// cleanupExpiredSessions 清理过期会话
func (s *Storage) cleanupExpiredSessions() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	var expiredSessions []string
	for id, session := range s.sessions {
		if time.Since(session.LastAccessed) > time.Hour {
			expiredSessions = append(expiredSessions, id)
		}
	}

	for _, id := range expiredSessions {
		delete(s.sessions, id)
		atomic.AddInt64(&s.activeSessions, -1)
		atomic.AddInt64(&s.expiredSessions, 1)
	}

	if len(expiredSessions) > 0 {
		s.logger.Debug("Expired sessions cleaned up", "count", len(expiredSessions))
	}
}

// cleanupExpiredCache 清理过期缓存
func (s *Storage) cleanupExpiredCache() {
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()

	// 清理超过1小时未访问的缓存
	expireTime := time.Now().Add(-time.Hour)
	var expiredKeys []string

	for key, entry := range s.cache {
		if entry.AccessTime.Before(expireTime) {
			expiredKeys = append(expiredKeys, key)
		}
	}

	for _, key := range expiredKeys {
		delete(s.cache, key)
	}

	if len(expiredKeys) > 0 {
		s.logger.Debug("Expired cache entries cleaned up", "count", len(expiredKeys))
	}
}

// memoryMonitor 内存监控
func (s *Storage) memoryMonitor() {
	ticker := time.NewTicker(s.config.MemoryCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.monitorMemory()
		case <-s.stopCh:
			return
		}
	}
}

// monitorMemory 监控内存使用情况
func (s *Storage) monitorMemory() {
	stats := s.GetMemoryStats()
	s.memoryStats = stats

	// 检查内存泄漏
	if s.leakDetector != nil {
		s.leakDetector.Check(stats)
	}

	// 记录内存统计信息
	s.logger.Debug("Memory stats",
		"alloc_mb", stats.AllocMB,
		"sys_mb", stats.SysMB,
		"num_gc", stats.NumGC,
		"goroutines", stats.Goroutines)

	// 如果内存使用过高，触发清理
	maxMemoryMB := float64(s.config.MaxMemoryMB)
	if stats.AllocMB > maxMemoryMB*s.config.GCThreshold {
		s.logger.Warn("High memory usage detected, triggering cleanup",
			"current_mb", stats.AllocMB,
			"max_mb", maxMemoryMB)

		select {
		case s.cleanupCh <- struct{}{}:
		default:
		}
	}
}

// TriggerCleanup 手动触发清理
func (s *Storage) TriggerCleanup() {
	select {
	case s.cleanupCh <- struct{}{}:
	default:
	}
}

// Close 关闭存储管理器
func (s *Storage) Close() error {
	close(s.stopCh)

	if s.leakDetector != nil {
		s.leakDetector.Close()
	}

	s.logger.Info("Storage manager closed")
	return nil
}

// GetCacheStats 获取缓存统计信息
func (s *Storage) GetCacheStats() map[string]interface{} {
	s.cacheMutex.RLock()
	defer s.cacheMutex.RUnlock()

	var totalSize int64
	for _, entry := range s.cache {
		totalSize += entry.Size
	}

	hits := atomic.LoadInt64(&s.cacheHits)
	misses := atomic.LoadInt64(&s.cacheMisses)
	total := hits + misses

	var hitRate float64
	if total > 0 {
		hitRate = float64(hits) / float64(total)
	}

	return map[string]interface{}{
		"entries":    len(s.cache),
		"total_size": totalSize,
		"hits":       hits,
		"misses":     misses,
		"hit_rate":   hitRate,
	}
}

// LeakDetector 内存泄漏检测器
type LeakDetector struct {
	history    []MemoryStats
	maxHistory int
	threshold  float64 // 内存增长阈值
	logger     core.Logger
	mutex      sync.RWMutex
}

// NewLeakDetector 创建内存泄漏检测器
func NewLeakDetector(logger core.Logger) *LeakDetector {
	return &LeakDetector{
		history:    make([]MemoryStats, 0),
		maxHistory: 100, // 保留最近100次记录
		threshold:  1.5, // 内存增长超过50%认为可能泄漏
		logger:     logger,
	}
}

// Check 检查内存泄漏
func (ld *LeakDetector) Check(stats MemoryStats) {
	ld.mutex.Lock()
	defer ld.mutex.Unlock()

	// 添加到历史记录
	ld.history = append(ld.history, stats)
	if len(ld.history) > ld.maxHistory {
		ld.history = ld.history[1:]
	}

	// 需要至少10个数据点才能检测
	if len(ld.history) < 10 {
		return
	}

	// 检查内存是否持续增长
	if ld.detectMemoryLeak() {
		ld.logger.Warn("Potential memory leak detected",
			"current_alloc_mb", stats.AllocMB,
			"goroutines", stats.Goroutines,
			"gc_count", stats.NumGC)
	}
}

// detectMemoryLeak 检测内存泄漏
func (ld *LeakDetector) detectMemoryLeak() bool {
	if len(ld.history) < 10 {
		return false
	}

	// 获取最近10个数据点
	recent := ld.history[len(ld.history)-10:]

	// 计算内存增长趋势
	var totalGrowth float64
	growthCount := 0

	for i := 1; i < len(recent); i++ {
		growth := recent[i].AllocMB / recent[i-1].AllocMB
		if growth > 1.0 {
			totalGrowth += growth
			growthCount++
		}
	}

	// 如果大部分时间内存都在增长，且平均增长率超过阈值
	if growthCount >= 7 && totalGrowth/float64(growthCount) > ld.threshold {
		return true
	}

	// 检查Goroutine数量是否异常增长
	goroutineGrowth := float64(recent[len(recent)-1].Goroutines) / float64(recent[0].Goroutines)
	if goroutineGrowth > 2.0 { // Goroutine数量翻倍
		return true
	}

	return false
}

// GetLeakReport 获取泄漏报告
func (ld *LeakDetector) GetLeakReport() map[string]interface{} {
	ld.mutex.RLock()
	defer ld.mutex.RUnlock()

	if len(ld.history) == 0 {
		return map[string]interface{}{
			"status": "no_data",
		}
	}

	latest := ld.history[len(ld.history)-1]

	var avgAlloc, avgGoroutines float64
	for _, stats := range ld.history {
		avgAlloc += stats.AllocMB
		avgGoroutines += float64(stats.Goroutines)
	}
	avgAlloc /= float64(len(ld.history))
	avgGoroutines /= float64(len(ld.history))

	return map[string]interface{}{
		"current_alloc_mb":   latest.AllocMB,
		"average_alloc_mb":   avgAlloc,
		"current_goroutines": latest.Goroutines,
		"average_goroutines": avgGoroutines,
		"gc_count":           latest.NumGC,
		"data_points":        len(ld.history),
		"potential_leak":     ld.detectMemoryLeak(),
	}
}

// Close 关闭泄漏检测器
func (ld *LeakDetector) Close() {
	ld.mutex.Lock()
	defer ld.mutex.Unlock()

	ld.history = nil
}

// estimateSessionMemoryUsage 估算会话内存使用量
func (s *Storage) estimateSessionMemoryUsage(session *core.SessionMemory) float64 {
	if session == nil {
		return 0
	}

	// 基础结构大小
	size := 200.0 // 基础字段大小估算

	// SessionID 和 UserID
	size += float64(len(session.SessionID) + len(session.UserID))

	// History 大小估算
	for _, history := range session.History {
		if history != nil {
			size += 100 // 基础历史记录大小
			size += float64(len(history.Query))
			size += float64(len(history.SQL))
			size += float64(len(history.Error))
		}
	}

	// Context 大小估算 (粗略估算)
	for key, value := range session.Context {
		size += float64(len(key))
		if str, ok := value.(string); ok {
			size += float64(len(str))
		} else {
			size += 50 // 其他类型的粗略估算
		}
	}

	return size
}
