package cache

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// CacheType 缓存类型
type CacheType string

const (
	CacheTypeMemory CacheType = "memory"
	CacheTypeRedis  CacheType = "redis"
	CacheTypeHybrid CacheType = "hybrid"
)

// CacheConfig 缓存配置
type CacheConfig struct {
	Type         CacheType     `yaml:"type" json:"type"`
	Host         string        `yaml:"host" json:"host"`
	Port         int           `yaml:"port" json:"port"`
	Password     string        `yaml:"password" json:"password"`
	Database     int           `yaml:"database" json:"database"`
	SchemaTTL    time.Duration `yaml:"schema_ttl" json:"schema_ttl"`
	QueryTTL     time.Duration `yaml:"query_result_ttl" json:"query_result_ttl"`
	MaxCacheSize string        `yaml:"max_cache_size" json:"max_cache_size"`
	PoolSize     int           `yaml:"pool_size" json:"pool_size"`
	MinIdleConns int           `yaml:"min_idle_conns" json:"min_idle_conns"`
	MaxRetries   int           `yaml:"max_retries" json:"max_retries"`
	DialTimeout  time.Duration `yaml:"dial_timeout" json:"dial_timeout"`
	ReadTimeout  time.Duration `yaml:"read_timeout" json:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout" json:"write_timeout"`
}

// DefaultCacheConfig 默认缓存配置
func DefaultCacheConfig() *CacheConfig {
	return &CacheConfig{
		Type:         CacheTypeMemory,
		Host:         "localhost",
		Port:         6379,
		Password:     "",
		Database:     0,
		SchemaTTL:    time.Hour,
		QueryTTL:     10 * time.Minute,
		MaxCacheSize: "100MB",
		PoolSize:     10,
		MinIdleConns: 5,
		MaxRetries:   3,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	}
}

// CacheStats 缓存统计信息
type CacheStats struct {
	Hits        int64     `json:"hits"`
	Misses      int64     `json:"misses"`
	Sets        int64     `json:"sets"`
	Deletes     int64     `json:"deletes"`
	Errors      int64     `json:"errors"`
	Size        int64     `json:"size"`
	LastUpdated time.Time `json:"last_updated"`
}

// CacheEntry 缓存条目
type CacheEntry struct {
	Key         string        `json:"key"`
	Value       any           `json:"value"`
	TTL         time.Duration `json:"ttl"`
	CreatedAt   time.Time     `json:"created_at"`
	ExpiresAt   time.Time     `json:"expires_at"`
	AccessCount int64         `json:"access_count"`
	LastAccess  time.Time     `json:"last_access"`
}

// IsExpired 检查缓存条目是否过期
func (e *CacheEntry) IsExpired() bool {
	return time.Now().After(e.ExpiresAt)
}

// Manager 多层缓存管理器
type Manager struct {
	config    *CacheConfig
	primary   core.CacheManager // 主缓存（通常是 Redis）
	secondary core.CacheManager // 二级缓存（通常是内存缓存）
	logger    core.Logger
	metrics   core.MetricsCollector
	stats     *CacheStats
	statsMux  sync.RWMutex
}

// NewManager 创建新的缓存管理器
func NewManager(config *CacheConfig, logger core.Logger, metrics core.MetricsCollector) (*Manager, error) {
	if config == nil {
		config = DefaultCacheConfig()
	}

	manager := &Manager{
		config:  config,
		logger:  logger,
		metrics: metrics,
		stats: &CacheStats{
			LastUpdated: time.Now(),
		},
	}

	// 初始化主缓存
	var err error
	switch config.Type {
	case CacheTypeRedis:
		manager.primary, err = NewRedisCache(config, logger, metrics)
		if err != nil {
			return nil, fmt.Errorf("failed to create Redis cache: %w", err)
		}
	case CacheTypeMemory:
		manager.primary, err = NewMemoryCache(config, logger, metrics)
		if err != nil {
			return nil, fmt.Errorf("failed to create memory cache: %w", err)
		}
	case CacheTypeHybrid:
		// 混合模式：Redis 作为主缓存，内存作为二级缓存
		manager.primary, err = NewRedisCache(config, logger, metrics)
		if err != nil {
			return nil, fmt.Errorf("failed to create Redis cache: %w", err)
		}
		manager.secondary, err = NewMemoryCache(config, logger, metrics)
		if err != nil {
			return nil, fmt.Errorf("failed to create memory cache: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported cache type: %s", config.Type)
	}

	return manager, nil
}

// Get 获取缓存值
func (m *Manager) Get(ctx context.Context, key string) (any, error) {
	// 先尝试从二级缓存获取（如果存在）
	if m.secondary != nil {
		if value, err := m.secondary.Get(ctx, key); err == nil {
			m.updateStats(func(s *CacheStats) {
				s.Hits++
			})
			m.recordMetrics("cache_hit", map[string]string{"level": "secondary"})
			return value, nil
		}
	}

	// 从主缓存获取
	value, err := m.primary.Get(ctx, key)
	if err != nil {
		m.updateStats(func(s *CacheStats) {
			s.Misses++
			s.Errors++
		})
		m.recordMetrics("cache_miss", map[string]string{"level": "primary"})
		return nil, err
	}

	// 如果有二级缓存，将值同步到二级缓存
	if m.secondary != nil {
		go func() {
			if err := m.secondary.Set(context.Background(), key, value, m.config.QueryTTL); err != nil {
				m.logger.Warn("Failed to sync to secondary cache", "key", key, "error", err)
			}
		}()
	}

	m.updateStats(func(s *CacheStats) {
		s.Hits++
	})
	m.recordMetrics("cache_hit", map[string]string{"level": "primary"})
	return value, nil
}

// Set 设置缓存值
func (m *Manager) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	// 设置到主缓存
	if err := m.primary.Set(ctx, key, value, ttl); err != nil {
		m.updateStats(func(s *CacheStats) {
			s.Errors++
		})
		return err
	}

	// 如果有二级缓存，也设置到二级缓存
	if m.secondary != nil {
		go func() {
			if err := m.secondary.Set(context.Background(), key, value, ttl); err != nil {
				m.logger.Warn("Failed to set to secondary cache", "key", key, "error", err)
			}
		}()
	}

	m.updateStats(func(s *CacheStats) {
		s.Sets++
	})
	m.recordMetrics("cache_set", map[string]string{"level": "primary"})
	return nil
}

// Delete 删除缓存值
func (m *Manager) Delete(ctx context.Context, key string) error {
	// 从主缓存删除
	if err := m.primary.Delete(ctx, key); err != nil {
		m.updateStats(func(s *CacheStats) {
			s.Errors++
		})
		return err
	}

	// 如果有二级缓存，也从二级缓存删除
	if m.secondary != nil {
		go func() {
			if err := m.secondary.Delete(context.Background(), key); err != nil {
				m.logger.Warn("Failed to delete from secondary cache", "key", key, "error", err)
			}
		}()
	}

	m.updateStats(func(s *CacheStats) {
		s.Deletes++
	})
	m.recordMetrics("cache_delete", map[string]string{"level": "primary"})
	return nil
}

// Clear 清空缓存
func (m *Manager) Clear(ctx context.Context) error {
	// 清空主缓存
	if err := m.primary.Clear(ctx); err != nil {
		m.updateStats(func(s *CacheStats) {
			s.Errors++
		})
		return err
	}

	// 如果有二级缓存，也清空二级缓存
	if m.secondary != nil {
		go func() {
			if err := m.secondary.Clear(context.Background()); err != nil {
				m.logger.Warn("Failed to clear secondary cache", "error", err)
			}
		}()
	}

	m.recordMetrics("cache_clear", map[string]string{"level": "all"})
	return nil
}

// GetStats 获取缓存统计信息
func (m *Manager) GetStats() *CacheStats {
	m.statsMux.RLock()
	defer m.statsMux.RUnlock()

	stats := *m.stats
	return &stats
}

// updateStats 更新统计信息
func (m *Manager) updateStats(fn func(*CacheStats)) {
	m.statsMux.Lock()
	defer m.statsMux.Unlock()

	fn(m.stats)
	m.stats.LastUpdated = time.Now()
}

// recordMetrics 记录指标
func (m *Manager) recordMetrics(operation string, labels map[string]string) {
	if m.metrics != nil {
		m.metrics.IncrementCounter("cache_operations_total", labels)
	}
}

// GetSchemaCache 获取 Schema 缓存
func (m *Manager) GetSchemaCache(ctx context.Context, key string) (any, error) {
	return m.Get(ctx, fmt.Sprintf("schema:%s", key))
}

// SetSchemaCache 设置 Schema 缓存
func (m *Manager) SetSchemaCache(ctx context.Context, key string, value any) error {
	return m.Set(ctx, fmt.Sprintf("schema:%s", key), value, m.config.SchemaTTL)
}

// GetQueryCache 获取查询结果缓存
func (m *Manager) GetQueryCache(ctx context.Context, key string) (any, error) {
	return m.Get(ctx, fmt.Sprintf("query:%s", key))
}

// SetQueryCache 设置查询结果缓存
func (m *Manager) SetQueryCache(ctx context.Context, key string, value any) error {
	return m.Set(ctx, fmt.Sprintf("query:%s", key), value, m.config.QueryTTL)
}

// Close 关闭缓存管理器
func (m *Manager) Close() error {
	var errs []error

	if closer, ok := m.primary.(interface{ Close() error }); ok {
		if err := closer.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close primary cache: %w", err))
		}
	}

	if m.secondary != nil {
		if closer, ok := m.secondary.(interface{ Close() error }); ok {
			if err := closer.Close(); err != nil {
				errs = append(errs, fmt.Errorf("failed to close secondary cache: %w", err))
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing cache manager: %v", errs)
	}

	return nil
}
