package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"pumppill/rag/core"

	"github.com/go-redis/redis/v8"
)

// RedisCache Redis 缓存实现
type RedisCache struct {
	client  *redis.Client
	config  *CacheConfig
	logger  core.Logger
	metrics core.MetricsCollector
}

// NewRedisCache 创建新的 Redis 缓存
func NewRedisCache(config *CacheConfig, logger core.Logger, metrics core.MetricsCollector) (*RedisCache, error) {
	if config == nil {
		config = DefaultCacheConfig()
	}

	// 创建 Redis 客户端
	client := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", config.Host, config.Port),
		Password:     config.Password,
		DB:           config.Database,
		PoolSize:     config.PoolSize,
		MinIdleConns: config.MinIdleConns,
		MaxRetries:   config.MaxRetries,
		DialTimeout:  config.DialTimeout,
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
	})

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	cache := &RedisCache{
		client:  client,
		config:  config,
		logger:  logger,
		metrics: metrics,
	}

	logger.Info("Redis cache initialized",
		"host", config.Host,
		"port", config.Port,
		"database", config.Database)

	return cache, nil
}

// Get 获取缓存值
func (r *RedisCache) Get(ctx context.Context, key string) (any, error) {
	start := time.Now()
	defer func() {
		if r.metrics != nil {
			r.metrics.RecordHistogram("cache_operation_duration_seconds",
				time.Since(start).Seconds(),
				map[string]string{"operation": "get", "backend": "redis"})
		}
	}()

	result, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			r.recordMetric("cache_miss", "redis")
			return nil, core.ErrCacheKeyNotFound
		}
		r.recordMetric("cache_error", "redis")
		r.logger.Error("Redis get error", "key", key, "error", err)
		return nil, fmt.Errorf("failed to get from Redis: %w", err)
	}

	// 尝试解析 JSON
	var value any
	if err := json.Unmarshal([]byte(result), &value); err != nil {
		// 如果不是 JSON，直接返回字符串
		value = result
	}

	r.recordMetric("cache_hit", "redis")
	return value, nil
}

// Set 设置缓存值
func (r *RedisCache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	start := time.Now()
	defer func() {
		if r.metrics != nil {
			r.metrics.RecordHistogram("cache_operation_duration_seconds",
				time.Since(start).Seconds(),
				map[string]string{"operation": "set", "backend": "redis"})
		}
	}()

	// 序列化值
	var data []byte
	var err error

	switch v := value.(type) {
	case string:
		data = []byte(v)
	case []byte:
		data = v
	default:
		data, err = json.Marshal(value)
		if err != nil {
			r.recordMetric("cache_error", "redis")
			return fmt.Errorf("failed to marshal value: %w", err)
		}
	}

	// 设置到 Redis
	if err := r.client.Set(ctx, key, data, ttl).Err(); err != nil {
		r.recordMetric("cache_error", "redis")
		r.logger.Error("Redis set error", "key", key, "error", err)
		return fmt.Errorf("failed to set to Redis: %w", err)
	}

	r.recordMetric("cache_set", "redis")
	return nil
}

// Delete 删除缓存值
func (r *RedisCache) Delete(ctx context.Context, key string) error {
	start := time.Now()
	defer func() {
		if r.metrics != nil {
			r.metrics.RecordHistogram("cache_operation_duration_seconds",
				time.Since(start).Seconds(),
				map[string]string{"operation": "delete", "backend": "redis"})
		}
	}()

	result, err := r.client.Del(ctx, key).Result()
	if err != nil {
		r.recordMetric("cache_error", "redis")
		r.logger.Error("Redis delete error", "key", key, "error", err)
		return fmt.Errorf("failed to delete from Redis: %w", err)
	}

	if result == 0 {
		r.recordMetric("cache_miss", "redis")
		return core.ErrCacheKeyNotFound
	}

	r.recordMetric("cache_delete", "redis")
	return nil
}

// Clear 清空缓存
func (r *RedisCache) Clear(ctx context.Context) error {
	start := time.Now()
	defer func() {
		if r.metrics != nil {
			r.metrics.RecordHistogram("cache_operation_duration_seconds",
				time.Since(start).Seconds(),
				map[string]string{"operation": "clear", "backend": "redis"})
		}
	}()

	if err := r.client.FlushDB(ctx).Err(); err != nil {
		r.recordMetric("cache_error", "redis")
		r.logger.Error("Redis clear error", "error", err)
		return fmt.Errorf("failed to clear Redis: %w", err)
	}

	r.recordMetric("cache_clear", "redis")
	return nil
}

// Exists 检查键是否存在
func (r *RedisCache) Exists(ctx context.Context, key string) (bool, error) {
	result, err := r.client.Exists(ctx, key).Result()
	if err != nil {
		r.recordMetric("cache_error", "redis")
		return false, fmt.Errorf("failed to check existence in Redis: %w", err)
	}
	return result > 0, nil
}

// TTL 获取键的剩余生存时间
func (r *RedisCache) TTL(ctx context.Context, key string) (time.Duration, error) {
	result, err := r.client.TTL(ctx, key).Result()
	if err != nil {
		r.recordMetric("cache_error", "redis")
		return 0, fmt.Errorf("failed to get TTL from Redis: %w", err)
	}
	return result, nil
}

// Expire 设置键的过期时间
func (r *RedisCache) Expire(ctx context.Context, key string, ttl time.Duration) error {
	if err := r.client.Expire(ctx, key, ttl).Err(); err != nil {
		r.recordMetric("cache_error", "redis")
		return fmt.Errorf("failed to set expiration in Redis: %w", err)
	}
	return nil
}

// Keys 获取匹配模式的所有键
func (r *RedisCache) Keys(ctx context.Context, pattern string) ([]string, error) {
	result, err := r.client.Keys(ctx, pattern).Result()
	if err != nil {
		r.recordMetric("cache_error", "redis")
		return nil, fmt.Errorf("failed to get keys from Redis: %w", err)
	}
	return result, nil
}

// MGet 批量获取多个键的值
func (r *RedisCache) MGet(ctx context.Context, keys ...string) ([]any, error) {
	if len(keys) == 0 {
		return []any{}, nil
	}

	start := time.Now()
	defer func() {
		if r.metrics != nil {
			r.metrics.RecordHistogram("cache_operation_duration_seconds",
				time.Since(start).Seconds(),
				map[string]string{"operation": "mget", "backend": "redis"})
		}
	}()

	result, err := r.client.MGet(ctx, keys...).Result()
	if err != nil {
		r.recordMetric("cache_error", "redis")
		return nil, fmt.Errorf("failed to mget from Redis: %w", err)
	}

	values := make([]any, len(result))
	for i, val := range result {
		if val == nil {
			values[i] = nil
			continue
		}

		str, ok := val.(string)
		if !ok {
			values[i] = val
			continue
		}

		// 尝试解析 JSON
		var parsed any
		if err := json.Unmarshal([]byte(str), &parsed); err != nil {
			values[i] = str
		} else {
			values[i] = parsed
		}
	}

	r.recordMetric("cache_mget", "redis")
	return values, nil
}

// MSet 批量设置多个键值对
func (r *RedisCache) MSet(ctx context.Context, pairs map[string]any, ttl time.Duration) error {
	if len(pairs) == 0 {
		return nil
	}

	start := time.Now()
	defer func() {
		if r.metrics != nil {
			r.metrics.RecordHistogram("cache_operation_duration_seconds",
				time.Since(start).Seconds(),
				map[string]string{"operation": "mset", "backend": "redis"})
		}
	}()

	// 准备数据
	args := make([]any, 0, len(pairs)*2)
	for key, value := range pairs {
		var data []byte
		var err error

		switch v := value.(type) {
		case string:
			data = []byte(v)
		case []byte:
			data = v
		default:
			data, err = json.Marshal(value)
			if err != nil {
				r.recordMetric("cache_error", "redis")
				return fmt.Errorf("failed to marshal value for key %s: %w", key, err)
			}
		}

		args = append(args, key, data)
	}

	// 批量设置
	if err := r.client.MSet(ctx, args...).Err(); err != nil {
		r.recordMetric("cache_error", "redis")
		return fmt.Errorf("failed to mset to Redis: %w", err)
	}

	// 如果有 TTL，为每个键设置过期时间
	if ttl > 0 {
		pipe := r.client.Pipeline()
		for key := range pairs {
			pipe.Expire(ctx, key, ttl)
		}
		if _, err := pipe.Exec(ctx); err != nil {
			r.logger.Warn("Failed to set TTL for some keys", "error", err)
		}
	}

	r.recordMetric("cache_mset", "redis")
	return nil
}

// GetInfo 获取 Redis 信息
func (r *RedisCache) GetInfo(ctx context.Context) (map[string]string, error) {
	result, err := r.client.Info(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get Redis info: %w", err)
	}

	info := make(map[string]string)
	lines := strings.Split(result, "\r\n")
	for _, line := range lines {
		if strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				info[parts[0]] = parts[1]
			}
		}
	}

	return info, nil
}

// Ping 测试连接
func (r *RedisCache) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

// Close 关闭连接
func (r *RedisCache) Close() error {
	if r.client != nil {
		return r.client.Close()
	}
	return nil
}

// recordMetric 记录指标
func (r *RedisCache) recordMetric(operation, backend string) {
	if r.metrics != nil {
		r.metrics.IncrementCounter("cache_operations_total",
			map[string]string{"operation": operation, "backend": backend})
	}
}
