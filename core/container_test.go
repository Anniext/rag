package core

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewContainer 测试创建容器
func TestNewContainer(t *testing.T) {
	container := NewContainer()
	assert.NotNil(t, container)
	assert.NotNil(t, container.services)
}

// TestRegisterAndGet 测试注册和获取服务
func TestRegisterAndGet(t *testing.T) {
	container := NewContainer()

	// 注册字符串服务
	container.Register("test_string", "hello world")

	// 获取服务
	service, err := container.Get("test_string")
	require.NoError(t, err)
	assert.Equal(t, "hello world", service)

	// 获取不存在的服务
	_, err = container.Get("non_existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "服务 'non_existent' 未注册")
}

// TestMustGet 测试 MustGet 方法
func TestMustGet(t *testing.T) {
	container := NewContainer()
	container.Register("test_service", "test_value")

	// 正常获取
	service := container.MustGet("test_service")
	assert.Equal(t, "test_value", service)

	// 获取不存在的服务应该 panic
	assert.Panics(t, func() {
		container.MustGet("non_existent")
	})
}

// TestRegisterSingleton 测试单例注册
func TestRegisterSingleton(t *testing.T) {
	container := NewContainer()

	counter := 0
	container.RegisterSingleton("counter", func() any {
		counter++
		return counter
	})

	// 多次获取应该返回相同的实例
	service1, err := container.Get("counter")
	require.NoError(t, err)
	assert.Equal(t, 1, service1)

	service2, err := container.Get("counter")
	require.NoError(t, err)
	assert.Equal(t, 1, service2)

	// 工厂函数应该只调用一次
	assert.Equal(t, 1, counter)
}

// TestGetAs 测试类型转换获取
func TestGetAs(t *testing.T) {
	container := NewContainer()

	// 注册字符串服务
	container.Register("test_string", "hello")

	// 正确类型转换
	var result string
	err := container.GetAs("test_string", &result)
	require.NoError(t, err)
	assert.Equal(t, "hello", result)

	// 错误的目标类型（不是指针）
	var notPointer string
	err = container.GetAs("test_string", notPointer)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "target 必须是指针类型")

	// 类型不匹配
	var intResult int
	err = container.GetAs("test_string", &intResult)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不能赋值给")

	// 服务不存在
	var nonExistent string
	err = container.GetAs("non_existent", &nonExistent)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "未注册")
}

// TestHas 测试服务存在检查
func TestHas(t *testing.T) {
	container := NewContainer()

	// 服务不存在
	assert.False(t, container.Has("test_service"))

	// 注册服务
	container.Register("test_service", "test_value")
	assert.True(t, container.Has("test_service"))
}

// TestRemove 测试移除服务
func TestRemove(t *testing.T) {
	container := NewContainer()

	// 注册服务
	container.Register("test_service", "test_value")
	assert.True(t, container.Has("test_service"))

	// 移除服务
	container.Remove("test_service")
	assert.False(t, container.Has("test_service"))

	// 移除不存在的服务不应该出错
	container.Remove("non_existent")
}

// TestClear 测试清空容器
func TestClear(t *testing.T) {
	container := NewContainer()

	// 注册多个服务
	container.Register("service1", "value1")
	container.Register("service2", "value2")
	container.Register("service3", "value3")

	assert.True(t, container.Has("service1"))
	assert.True(t, container.Has("service2"))
	assert.True(t, container.Has("service3"))

	// 清空容器
	container.Clear()

	assert.False(t, container.Has("service1"))
	assert.False(t, container.Has("service2"))
	assert.False(t, container.Has("service3"))
}

// TestGetServiceNames 测试获取服务名称列表
func TestGetServiceNames(t *testing.T) {
	container := NewContainer()

	// 空容器
	names := container.GetServiceNames()
	assert.Empty(t, names)

	// 注册服务
	container.Register("service1", "value1")
	container.Register("service2", "value2")
	container.Register("service3", "value3")

	names = container.GetServiceNames()
	assert.Len(t, names, 3)
	assert.ElementsMatch(t, []string{"service1", "service2", "service3"}, names)
}

// TestServiceProvider 测试服务提供者
func TestServiceProvider(t *testing.T) {
	// 创建测试服务提供者
	provider := &testServiceProvider{}

	app := NewApplication()
	assert.NotNil(t, app.GetContainer())

	// 注册服务提供者
	err := app.RegisterProvider(provider)
	require.NoError(t, err)

	// 验证服务已注册
	assert.True(t, app.GetContainer().Has("test_service"))

	// 启动应用
	err = app.Boot()
	require.NoError(t, err)
	assert.True(t, app.IsBooted())

	// 验证服务已启动
	service, err := app.GetContainer().Get("test_service")
	require.NoError(t, err)
	testService := service.(*testService)
	assert.True(t, testService.Started)

	// 重复启动不应该出错
	err = app.Boot()
	require.NoError(t, err)
}

// TestServiceProviderError 测试服务提供者错误处理
func TestServiceProviderError(t *testing.T) {
	// 创建会出错的服务提供者
	provider := &errorServiceProvider{}

	app := NewApplication()

	// 注册时出错
	err := app.RegisterProvider(provider)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "register error")

	// 创建启动时出错的服务提供者
	bootErrorProvider := &bootErrorServiceProvider{}
	app2 := NewApplication()

	err = app2.RegisterProvider(bootErrorProvider)
	require.NoError(t, err)

	// 启动时出错
	err = app2.Boot()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "启动服务提供者失败")
	assert.Contains(t, err.Error(), "boot error")
}

// TestGlobalContainer 测试全局容器
func TestGlobalContainer(t *testing.T) {
	// 清空全局容器
	globalContainer.Clear()

	// 注册全局服务
	RegisterGlobal("global_service", "global_value")

	// 获取全局服务
	service, err := GetGlobal("global_service")
	require.NoError(t, err)
	assert.Equal(t, "global_value", service)

	// MustGet 全局服务
	service = MustGetGlobal("global_service")
	assert.Equal(t, "global_value", service)

	// GetAs 全局服务
	var result string
	err = GetAsGlobal("global_service", &result)
	require.NoError(t, err)
	assert.Equal(t, "global_value", result)

	// 注册全局单例
	counter := 0
	RegisterSingletonGlobal("global_counter", func() interface{} {
		counter++
		return counter
	})

	service1, err := GetGlobal("global_counter")
	require.NoError(t, err)
	service2, err := GetGlobal("global_counter")
	require.NoError(t, err)

	assert.Equal(t, service1, service2)
	assert.Equal(t, 1, counter)

	// 清空全局容器
	globalContainer.Clear()
}

// TestConcurrentAccess 测试并发访问
func TestConcurrentAccess(t *testing.T) {
	container := NewContainer()

	// 并发注册和获取服务
	done := make(chan bool, 100)

	// 启动多个 goroutine 进行并发操作
	for i := 0; i < 50; i++ {
		go func(id int) {
			serviceName := fmt.Sprintf("service_%d", id)
			serviceValue := fmt.Sprintf("value_%d", id)

			container.Register(serviceName, serviceValue)

			service, err := container.Get(serviceName)
			assert.NoError(t, err)
			assert.Equal(t, serviceValue, service)

			done <- true
		}(i)
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 50; i++ {
		<-done
	}

	// 验证所有服务都已注册
	names := container.GetServiceNames()
	assert.Len(t, names, 50)
}

// testService 测试服务结构
type testService struct {
	Name    string
	Started bool
}

// testServiceProvider 测试服务提供者
type testServiceProvider struct{}

func (p *testServiceProvider) Register(container *Container) error {
	container.Register("test_service", &testService{Name: "test"})
	return nil
}

func (p *testServiceProvider) Boot(container *Container) error {
	service, err := container.Get("test_service")
	if err != nil {
		return err
	}

	testService := service.(*testService)
	testService.Started = true
	return nil
}

// errorServiceProvider 注册时出错的服务提供者
type errorServiceProvider struct{}

func (p *errorServiceProvider) Register(container *Container) error {
	return errors.New("register error")
}

func (p *errorServiceProvider) Boot(container *Container) error {
	return nil
}

// bootErrorServiceProvider 启动时出错的服务提供者
type bootErrorServiceProvider struct{}

func (p *bootErrorServiceProvider) Register(container *Container) error {
	container.Register("boot_error_service", "test")
	return nil
}

func (p *bootErrorServiceProvider) Boot(container *Container) error {
	return errors.New("boot error")
}

// 基准测试
func BenchmarkContainerRegister(b *testing.B) {
	container := NewContainer()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		serviceName := fmt.Sprintf("service_%d", i)
		container.Register(serviceName, "test_value")
	}
}

func BenchmarkContainerGet(b *testing.B) {
	container := NewContainer()

	// 预注册一些服务
	for i := 0; i < 1000; i++ {
		serviceName := fmt.Sprintf("service_%d", i)
		container.Register(serviceName, "test_value")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		serviceName := fmt.Sprintf("service_%d", i%1000)
		container.Get(serviceName)
	}
}

func BenchmarkContainerSingleton(b *testing.B) {
	container := NewContainer()

	counter := 0
	container.RegisterSingleton("counter", func() any {
		counter++
		return counter
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		container.Get("counter")
	}
}

func BenchmarkGlobalContainer(b *testing.B) {
	RegisterGlobal("bench_service", "bench_value")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		GetGlobal("bench_service")
	}
}
