package deploy

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

// MockComponent 模拟组件
type MockComponent struct {
	*BaseComponent
	startFunc       func(ctx context.Context) error
	stopFunc        func(ctx context.Context) error
	healthCheckFunc func(ctx context.Context) error
	startDelay      time.Duration
	stopDelay       time.Duration
}

// NewMockComponent 创建模拟组件
func NewMockComponent(name string, dependencies []string) *MockComponent {
	return &MockComponent{
		BaseComponent: NewBaseComponent(name, dependencies),
	}
}

// Start 启动组件
func (m *MockComponent) Start(ctx context.Context) error {
	if m.startDelay > 0 {
		time.Sleep(m.startDelay)
	}

	if m.startFunc != nil {
		if err := m.startFunc(ctx); err != nil {
			return err
		}
	}

	return m.BaseComponent.Start(ctx)
}

// Stop 停止组件
func (m *MockComponent) Stop(ctx context.Context) error {
	if m.stopDelay > 0 {
		time.Sleep(m.stopDelay)
	}

	if m.stopFunc != nil {
		if err := m.stopFunc(ctx); err != nil {
			return err
		}
	}

	return m.BaseComponent.Stop(ctx)
}

// HealthCheck 健康检查
func (m *MockComponent) HealthCheck(ctx context.Context) error {
	if m.healthCheckFunc != nil {
		return m.healthCheckFunc(ctx)
	}
	return m.BaseComponent.HealthCheck(ctx)
}

// TestNewServiceManager 测试创建服务管理器
func TestNewServiceManager(t *testing.T) {
	logger := zaptest.NewLogger(t)
	config := &LifecycleConfig{
		GracefulTimeout:     30 * time.Second,
		ShutdownTimeout:     60 * time.Second,
		HealthCheckInterval: 10 * time.Second,
		MaxStartupTime:      120 * time.Second,
	}

	sm := NewServiceManager(config, logger)

	assert.NotNil(t, sm)
	assert.Equal(t, config, sm.config)
	assert.Equal(t, logger, sm.logger)
	assert.Equal(t, ServiceStateStopped, sm.state)
	assert.NotNil(t, sm.components)
	assert.NotNil(t, sm.signalChan)
	assert.NotNil(t, sm.stopChan)
	assert.NotNil(t, sm.doneChan)
}

// TestNewServiceManagerWithNilConfig 测试使用空配置创建服务管理器
func TestNewServiceManagerWithNilConfig(t *testing.T) {
	logger := zaptest.NewLogger(t)

	sm := NewServiceManager(nil, logger)

	assert.NotNil(t, sm)
	assert.NotNil(t, sm.config)
	assert.Equal(t, 30*time.Second, sm.config.GracefulTimeout)
	assert.Equal(t, 60*time.Second, sm.config.ShutdownTimeout)
	assert.Equal(t, 10*time.Second, sm.config.HealthCheckInterval)
	assert.Equal(t, 120*time.Second, sm.config.MaxStartupTime)
}

// TestRegisterComponent 测试注册组件
func TestRegisterComponent(t *testing.T) {
	logger := zaptest.NewLogger(t)
	sm := NewServiceManager(nil, logger)

	comp1 := NewMockComponent("comp1", nil)
	comp2 := NewMockComponent("comp2", []string{"comp1"})

	// 注册第一个组件
	err := sm.RegisterComponent(comp1)
	assert.NoError(t, err)
	assert.Len(t, sm.components, 1)

	// 注册第二个组件
	err = sm.RegisterComponent(comp2)
	assert.NoError(t, err)
	assert.Len(t, sm.components, 2)

	// 重复注册应该失败
	err = sm.RegisterComponent(comp1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "已存在")
}

// TestCalculateStartOrder 测试计算启动顺序
func TestCalculateStartOrder(t *testing.T) {
	logger := zaptest.NewLogger(t)
	sm := NewServiceManager(nil, logger)

	// 创建有依赖关系的组件
	comp1 := NewMockComponent("comp1", nil)
	comp2 := NewMockComponent("comp2", []string{"comp1"})
	comp3 := NewMockComponent("comp3", []string{"comp1", "comp2"})

	sm.RegisterComponent(comp1)
	sm.RegisterComponent(comp2)
	sm.RegisterComponent(comp3)

	// 验证启动顺序
	expectedStartOrder := []string{"comp1", "comp2", "comp3"}
	assert.Equal(t, expectedStartOrder, sm.startOrder)

	// 验证停止顺序（应该是启动顺序的反向）
	expectedStopOrder := []string{"comp3", "comp2", "comp1"}
	assert.Equal(t, expectedStopOrder, sm.stopOrder)
}

// TestStartAndStopService 测试启动和停止服务
func TestStartAndStopService(t *testing.T) {
	logger := zaptest.NewLogger(t)
	config := &LifecycleConfig{
		GracefulTimeout:     5 * time.Second,
		ShutdownTimeout:     10 * time.Second,
		HealthCheckInterval: 1 * time.Second,
		MaxStartupTime:      10 * time.Second,
	}
	sm := NewServiceManager(config, logger)

	// 注册组件
	comp1 := NewMockComponent("comp1", nil)
	comp2 := NewMockComponent("comp2", []string{"comp1"})

	sm.RegisterComponent(comp1)
	sm.RegisterComponent(comp2)

	ctx := context.Background()

	// 测试启动
	err := sm.Start(ctx)
	assert.NoError(t, err)
	assert.Equal(t, ServiceStateRunning, sm.GetState())
	assert.True(t, comp1.IsStarted())
	assert.True(t, comp2.IsStarted())

	// 测试重复启动应该失败
	err = sm.Start(ctx)
	assert.Error(t, err)

	// 测试停止
	err = sm.Stop(ctx)
	assert.NoError(t, err)
	assert.Equal(t, ServiceStateStopped, sm.GetState())
	assert.False(t, comp1.IsStarted())
	assert.False(t, comp2.IsStarted())
}

// TestStartComponentFailure 测试组件启动失败
func TestStartComponentFailure(t *testing.T) {
	logger := zaptest.NewLogger(t)
	sm := NewServiceManager(nil, logger)

	// 创建会启动失败的组件
	comp1 := NewMockComponent("comp1", nil)
	comp1.startFunc = func(ctx context.Context) error {
		return errors.New("启动失败")
	}

	sm.RegisterComponent(comp1)

	ctx := context.Background()

	// 启动应该失败
	err := sm.Start(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "启动组件失败")
	assert.Equal(t, ServiceStateError, sm.GetState())
}

// TestStartTimeout 测试启动超时
func TestStartTimeout(t *testing.T) {
	logger := zaptest.NewLogger(t)
	config := &LifecycleConfig{
		MaxStartupTime: 100 * time.Millisecond,
	}
	sm := NewServiceManager(config, logger)

	// 创建启动缓慢的组件
	comp1 := NewMockComponent("comp1", nil)
	comp1.startDelay = 200 * time.Millisecond

	sm.RegisterComponent(comp1)

	ctx := context.Background()

	// 启动应该超时
	err := sm.Start(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context deadline exceeded")
}

// TestHealthCheck 测试健康检查
func TestHealthCheck(t *testing.T) {
	logger := zaptest.NewLogger(t)
	sm := NewServiceManager(nil, logger)

	// 创建健康的组件
	healthyComp := NewMockComponent("healthy", nil)
	sm.RegisterComponent(healthyComp)

	ctx := context.Background()

	// 启动服务
	err := sm.Start(ctx)
	assert.NoError(t, err)

	// 获取组件状态
	status := sm.GetComponentStatus(ctx)
	assert.Len(t, status, 1)
	assert.NoError(t, status["healthy"])

	// 测试不健康的组件
	unhealthyComp := NewMockComponent("unhealthy", nil)
	unhealthyComp.healthCheckFunc = func(ctx context.Context) error {
		return errors.New("不健康")
	}
	unhealthyComp.SetStarted(true) // 设置为已启动状态

	sm.RegisterComponent(unhealthyComp)

	// 再次获取组件状态
	status = sm.GetComponentStatus(ctx)
	assert.Len(t, status, 2)
	assert.NoError(t, status["healthy"])
	assert.Error(t, status["unhealthy"])
}

// TestGetUptime 测试获取运行时间
func TestGetUptime(t *testing.T) {
	logger := zaptest.NewLogger(t)
	sm := NewServiceManager(nil, logger)

	// 服务未启动时运行时间应该为0
	assert.Equal(t, time.Duration(0), sm.GetUptime())

	// 启动服务
	ctx := context.Background()
	err := sm.Start(ctx)
	assert.NoError(t, err)

	// 等待一段时间
	time.Sleep(10 * time.Millisecond)

	// 运行时间应该大于0
	uptime := sm.GetUptime()
	assert.True(t, uptime > 0)
	assert.True(t, uptime < time.Second) // 应该小于1秒

	// 停止服务
	err = sm.Stop(ctx)
	assert.NoError(t, err)

	// 停止后运行时间应该为0
	assert.Equal(t, time.Duration(0), sm.GetUptime())
}

// TestRestart 测试重启服务
func TestRestart(t *testing.T) {
	logger := zaptest.NewLogger(t)
	sm := NewServiceManager(nil, logger)

	comp := NewMockComponent("comp", nil)
	sm.RegisterComponent(comp)

	ctx := context.Background()

	// 启动服务
	err := sm.Start(ctx)
	assert.NoError(t, err)
	assert.Equal(t, ServiceStateRunning, sm.GetState())

	// 重启服务
	err = sm.Restart(ctx)
	assert.NoError(t, err)
	assert.Equal(t, ServiceStateRunning, sm.GetState())
}

// TestGetMetrics 测试获取服务指标
func TestGetMetrics(t *testing.T) {
	logger := zaptest.NewLogger(t)
	sm := NewServiceManager(nil, logger)

	comp := NewMockComponent("comp", nil)
	sm.RegisterComponent(comp)

	ctx := context.Background()

	// 启动服务
	err := sm.Start(ctx)
	assert.NoError(t, err)

	// 获取指标
	metrics := sm.GetMetrics()
	assert.NotNil(t, metrics)
	assert.Equal(t, "running", metrics["state"])
	assert.Equal(t, 1, metrics["component_count"])
	assert.NotEmpty(t, metrics["uptime"])
	assert.NotEmpty(t, metrics["start_time"])

	components, ok := metrics["components"].(map[string]string)
	assert.True(t, ok)
	assert.Equal(t, "healthy", components["comp"])
}

// TestBaseComponent 测试基础组件
func TestBaseComponent(t *testing.T) {
	comp := NewBaseComponent("test", []string{"dep1", "dep2"})

	assert.Equal(t, "test", comp.Name())
	assert.Equal(t, []string{"dep1", "dep2"}, comp.Dependencies())
	assert.False(t, comp.IsStarted())

	ctx := context.Background()

	// 启动组件
	err := comp.Start(ctx)
	assert.NoError(t, err)
	assert.True(t, comp.IsStarted())

	// 健康检查应该通过
	err = comp.HealthCheck(ctx)
	assert.NoError(t, err)

	// 停止组件
	err = comp.Stop(ctx)
	assert.NoError(t, err)
	assert.False(t, comp.IsStarted())

	// 停止后健康检查应该失败
	err = comp.HealthCheck(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "未启动")
}

// TestConcurrentOperations 测试并发操作
func TestConcurrentOperations(t *testing.T) {
	logger := zaptest.NewLogger(t)
	sm := NewServiceManager(nil, logger)

	comp := NewMockComponent("comp", nil)
	sm.RegisterComponent(comp)

	ctx := context.Background()

	// 启动服务
	err := sm.Start(ctx)
	assert.NoError(t, err)

	// 并发获取状态和指标
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = sm.GetState()
			_ = sm.GetUptime()
			_ = sm.GetMetrics()
			_ = sm.GetComponentStatus(ctx)
		}()
	}

	wg.Wait()

	// 停止服务
	err = sm.Stop(ctx)
	assert.NoError(t, err)
}

// TestCircularDependency 测试循环依赖检测
func TestCircularDependency(t *testing.T) {
	logger := zaptest.NewLogger(t)
	sm := NewServiceManager(nil, logger)

	// 创建循环依赖的组件
	comp1 := NewMockComponent("comp1", []string{"comp2"})
	comp2 := NewMockComponent("comp2", []string{"comp1"})

	sm.RegisterComponent(comp1)
	sm.RegisterComponent(comp2)

	// 应该使用默认顺序（因为检测到循环依赖）
	assert.Len(t, sm.startOrder, 2)
}

// TestStopComponentFailure 测试组件停止失败
func TestStopComponentFailure(t *testing.T) {
	logger := zaptest.NewLogger(t)
	sm := NewServiceManager(nil, logger)

	// 创建停止时会失败的组件
	comp := NewMockComponent("comp", nil)
	comp.stopFunc = func(ctx context.Context) error {
		return errors.New("停止失败")
	}

	sm.RegisterComponent(comp)

	ctx := context.Background()

	// 启动服务
	err := sm.Start(ctx)
	assert.NoError(t, err)

	// 停止服务（应该记录错误但不会失败）
	err = sm.Stop(ctx)
	assert.NoError(t, err) // 停止操作本身不会失败
	assert.Equal(t, ServiceStateStopped, sm.GetState())
}

// TestHealthCheckFailureDuringStartup 测试启动时健康检查失败
func TestHealthCheckFailureDuringStartup(t *testing.T) {
	logger := zaptest.NewLogger(t)
	sm := NewServiceManager(nil, logger)

	// 创建健康检查失败的组件
	comp := NewMockComponent("comp", nil)
	comp.healthCheckFunc = func(ctx context.Context) error {
		return errors.New("健康检查失败")
	}

	sm.RegisterComponent(comp)

	ctx := context.Background()

	// 启动应该失败
	err := sm.Start(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "启动健康检查失败")
	assert.Equal(t, ServiceStateError, sm.GetState())
}

// BenchmarkServiceManagerOperations 基准测试服务管理器操作
func BenchmarkServiceManagerOperations(b *testing.B) {
	logger := zap.NewNop()
	sm := NewServiceManager(nil, logger)

	// 注册多个组件
	for i := 0; i < 10; i++ {
		comp := NewMockComponent(fmt.Sprintf("comp%d", i), nil)
		sm.RegisterComponent(comp)
	}

	ctx := context.Background()
	sm.Start(ctx)
	defer sm.Stop(ctx)

	b.ResetTimer()

	b.Run("GetState", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = sm.GetState()
		}
	})

	b.Run("GetUptime", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = sm.GetUptime()
		}
	})

	b.Run("GetMetrics", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = sm.GetMetrics()
		}
	})

	b.Run("GetComponentStatus", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = sm.GetComponentStatus(ctx)
		}
	})
}
