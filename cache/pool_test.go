package cache

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockConnection 模拟连接
type mockConnection struct {
	id       int
	valid    bool
	lastUsed time.Time
	closed   bool
	mutex    sync.Mutex
}

func newMockConnection(id int) *mockConnection {
	return &mockConnection{
		id:       id,
		valid:    true,
		lastUsed: time.Now(),
	}
}

func (mc *mockConnection) IsValid() bool {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()
	return mc.valid && !mc.closed
}

func (mc *mockConnection) Close() error {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()
	mc.closed = true
	return nil
}

func (mc *mockConnection) LastUsed() time.Time {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()
	return mc.lastUsed
}

func (mc *mockConnection) SetLastUsed(t time.Time) {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()
	mc.lastUsed = t
}

func (mc *mockConnection) Invalidate() {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()
	mc.valid = false
}

// mockConnectionFactory 模拟连接工厂
type mockConnectionFactory struct {
	counter   int
	mutex     sync.Mutex
	failAfter int // 在创建多少个连接后开始失败
}

func newMockConnectionFactory() *mockConnectionFactory {
	return &mockConnectionFactory{
		failAfter: -1, // 默认不失败
	}
}

func (mcf *mockConnectionFactory) Create(ctx context.Context) (Connection, error) {
	mcf.mutex.Lock()
	defer mcf.mutex.Unlock()

	mcf.counter++
	if mcf.failAfter >= 0 && mcf.counter > mcf.failAfter {
		return nil, assert.AnError
	}

	return newMockConnection(mcf.counter), nil
}

func (mcf *mockConnectionFactory) SetFailAfter(count int) {
	mcf.mutex.Lock()
	defer mcf.mutex.Unlock()
	mcf.failAfter = count
}

func (mcf *mockConnectionFactory) GetCounter() int {
	mcf.mutex.Lock()
	defer mcf.mutex.Unlock()
	return mcf.counter
}

func TestNewPool(t *testing.T) {
	logger := &mockLogger{}
	metrics := &mockMetrics{}

	tests := []struct {
		name    string
		config  *PoolConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: &PoolConfig{
				MinConnections:     2,
				MaxConnections:     10,
				MaxIdleTime:        time.Minute,
				ConnectionTimeout:  5 * time.Second,
				IdleCheckInterval:  time.Minute,
				HealthCheckEnabled: false,
			},
			wantErr: false,
		},
		{
			name:    "nil config uses default",
			config:  nil,
			wantErr: false,
		},
		{
			name: "factory fails",
			config: &PoolConfig{
				MinConnections:     2,
				MaxConnections:     10,
				ConnectionTimeout:  5 * time.Second,
				IdleCheckInterval:  time.Minute,
				HealthCheckEnabled: false,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := newMockConnectionFactory()
			if tt.name == "factory fails" {
				factory.SetFailAfter(0)
			}

			pool, err := NewPool(tt.config, factory.Create, logger, metrics)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, pool)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, pool)
				if pool != nil {
					stats := pool.Stats()
					if tt.config != nil {
						assert.Equal(t, int32(tt.config.MinConnections), stats.TotalConnections)
						assert.Equal(t, int32(tt.config.MinConnections), stats.IdleConnections)
					}
					pool.Close()
				}
			}
		})
	}
}

func TestPool_GetPut(t *testing.T) {
	logger := &mockLogger{}
	metrics := &mockMetrics{}
	factory := newMockConnectionFactory()

	config := &PoolConfig{
		MinConnections:     2,
		MaxConnections:     5,
		MaxIdleTime:        time.Minute,
		ConnectionTimeout:  5 * time.Second,
		IdleCheckInterval:  time.Minute,
		HealthCheckEnabled: false,
	}

	pool, err := NewPool(config, factory.Create, logger, metrics)
	require.NoError(t, err)
	defer pool.Close()

	ctx := context.Background()

	// Test Get
	conn1, err := pool.Get(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, conn1)

	stats := pool.Stats()
	assert.Equal(t, int32(1), stats.ActiveConnections)
	assert.Equal(t, int32(1), stats.IdleConnections)

	// Test Put
	err = pool.Put(conn1)
	assert.NoError(t, err)

	stats = pool.Stats()
	assert.Equal(t, int32(0), stats.ActiveConnections)
	assert.Equal(t, int32(2), stats.IdleConnections)

	// Test Get again (should get a connection)
	conn2, err := pool.Get(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, conn2)

	// Put it back
	err = pool.Put(conn2)
	assert.NoError(t, err)
}

func TestPool_MaxConnections(t *testing.T) {
	logger := &mockLogger{}
	metrics := &mockMetrics{}
	factory := newMockConnectionFactory()

	config := &PoolConfig{
		MinConnections:     1,
		MaxConnections:     2,
		MaxIdleTime:        time.Minute,
		ConnectionTimeout:  100 * time.Millisecond,
		IdleCheckInterval:  time.Minute,
		HealthCheckEnabled: false,
	}

	pool, err := NewPool(config, factory.Create, logger, metrics)
	require.NoError(t, err)
	defer pool.Close()

	ctx := context.Background()

	// Get all available connections
	conn1, err := pool.Get(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, conn1)

	conn2, err := pool.Get(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, conn2)

	// Try to get another connection (should timeout)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	conn3, err := pool.Get(ctx)
	assert.Error(t, err)
	assert.Nil(t, conn3)
	assert.Equal(t, context.DeadlineExceeded, err)

	// Return a connection and try again
	pool.Put(conn1)

	ctx2 := context.Background()
	conn4, err := pool.Get(ctx2)
	assert.NoError(t, err)
	assert.NotNil(t, conn4)
}

func TestPool_InvalidConnection(t *testing.T) {
	logger := &mockLogger{}
	metrics := &mockMetrics{}
	factory := newMockConnectionFactory()

	config := &PoolConfig{
		MinConnections:     1,
		MaxConnections:     5,
		MaxIdleTime:        time.Minute,
		ConnectionTimeout:  5 * time.Second,
		IdleCheckInterval:  time.Minute,
		HealthCheckEnabled: false,
	}

	pool, err := NewPool(config, factory.Create, logger, metrics)
	require.NoError(t, err)
	defer pool.Close()

	ctx := context.Background()

	// Get a connection and invalidate it
	conn, err := pool.Get(ctx)
	require.NoError(t, err)
	mockConn := conn.(*mockConnection)
	mockConn.Invalidate()

	// Put back invalid connection
	err = pool.Put(conn)
	assert.NoError(t, err)

	// Stats should show the connection was removed
	stats := pool.Stats()
	assert.Equal(t, int32(1), stats.TotalConnections) // Min connections maintained
}

func TestPool_ConcurrentAccess(t *testing.T) {
	logger := &mockLogger{}
	metrics := &mockMetrics{}
	factory := newMockConnectionFactory()

	config := &PoolConfig{
		MinConnections:     2,
		MaxConnections:     10,
		MaxIdleTime:        time.Minute,
		ConnectionTimeout:  5 * time.Second,
		IdleCheckInterval:  time.Minute,
		HealthCheckEnabled: false,
	}

	pool, err := NewPool(config, factory.Create, logger, metrics)
	require.NoError(t, err)
	defer pool.Close()

	ctx := context.Background()
	var wg sync.WaitGroup
	errors := make(chan error, 20)

	// Start multiple goroutines to get and put connections
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for j := 0; j < 10; j++ {
				conn, err := pool.Get(ctx)
				if err != nil {
					errors <- err
					return
				}

				// Simulate some work
				time.Sleep(time.Millisecond)

				err = pool.Put(conn)
				if err != nil {
					errors <- err
					return
				}
			}
		}()
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("Unexpected error: %v", err)
	}

	// Verify final state
	stats := pool.Stats()
	assert.True(t, stats.TotalConnections >= int32(config.MinConnections))
	assert.Equal(t, int32(0), stats.ActiveConnections)
}

func TestPool_Close(t *testing.T) {
	logger := &mockLogger{}
	metrics := &mockMetrics{}
	factory := newMockConnectionFactory()

	config := &PoolConfig{
		MinConnections:     2,
		MaxConnections:     5,
		MaxIdleTime:        time.Minute,
		ConnectionTimeout:  5 * time.Second,
		IdleCheckInterval:  time.Minute,
		HealthCheckEnabled: false,
	}

	pool, err := NewPool(config, factory.Create, logger, metrics)
	require.NoError(t, err)

	ctx := context.Background()

	// Get a connection
	conn, err := pool.Get(ctx)
	require.NoError(t, err)

	// Close pool
	err = pool.Close()
	assert.NoError(t, err)

	// Try to get connection from closed pool
	_, err = pool.Get(ctx)
	assert.Error(t, err)

	// Try to put connection back to closed pool
	err = pool.Put(conn)
	assert.Error(t, err)

	// Close again should not error
	err = pool.Close()
	assert.NoError(t, err)
}

func TestRoundRobinBalancer(t *testing.T) {
	balancer := NewRoundRobinBalancer(3)

	// Test normal operation
	assert.Equal(t, 0, balancer.Next())
	assert.Equal(t, 1, balancer.Next())
	assert.Equal(t, 2, balancer.Next())
	assert.Equal(t, 0, balancer.Next()) // Should wrap around

	// Test with no nodes
	emptyBalancer := NewRoundRobinBalancer(0)
	assert.Equal(t, -1, emptyBalancer.Next())

	// Test remove
	balancer.Remove(0)
	assert.Equal(t, 0, balancer.Next())
	assert.Equal(t, 1, balancer.Next()) // Now only 2 nodes

	// Test add
	index := balancer.Add(1.0)
	assert.Equal(t, 2, index)
}

func TestWeightedBalancer(t *testing.T) {
	balancer := NewWeightedBalancer()

	// Test with no nodes
	assert.Equal(t, -1, balancer.Next())

	// Add nodes
	index1 := balancer.Add(1.0)
	index2 := balancer.Add(2.0)
	index3 := balancer.Add(1.0)

	assert.Equal(t, 0, index1)
	assert.Equal(t, 1, index2)
	assert.Equal(t, 2, index3)

	// Test selection (should favor node with higher weight)
	results := make(map[int]int)
	for i := 0; i < 1000; i++ {
		next := balancer.Next()
		results[next]++
	}

	// Node 1 (weight 2.0) should be selected more often
	assert.True(t, results[1] > results[0])
	assert.True(t, results[1] > results[2])

	// Test update
	balancer.Update(0, 3.0)

	// Test remove
	balancer.Remove(1)

	// Should still work
	next := balancer.Next()
	assert.True(t, next >= 0)
}

func TestTokenBucketLimiter(t *testing.T) {
	limiter := NewTokenBucketLimiter(10.0, 5) // 10 tokens per second, capacity 5

	// Should allow initial requests up to capacity
	for i := 0; i < 5; i++ {
		assert.True(t, limiter.Allow(), "Request %d should be allowed", i)
	}

	// Should deny next request (no tokens left)
	assert.False(t, limiter.Allow())

	// Wait for tokens to refill
	time.Sleep(200 * time.Millisecond) // Should add ~2 tokens
	assert.True(t, limiter.Allow())
	assert.True(t, limiter.Allow())
	assert.False(t, limiter.Allow()) // Should be denied again

	// Test Wait method
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := limiter.Wait(ctx)
	elapsed := time.Since(start)

	assert.NoError(t, err)
	assert.True(t, elapsed > 50*time.Millisecond) // Should have waited

	// Test Wait with timeout
	ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel2()

	// Exhaust tokens
	for limiter.Allow() {
	}

	err = limiter.Wait(ctx2)
	assert.Error(t, err)
	assert.Equal(t, context.DeadlineExceeded, err)

	// Test Limit and Burst
	assert.Equal(t, 10.0, limiter.Limit())
	assert.Equal(t, 5, limiter.Burst())
}
