package core

import (
	"fmt"
	"reflect"
	"sync"
)

// Container 依赖注入容器
type Container struct {
	services map[string]any
	mutex    sync.RWMutex
}

// NewContainer 创建新的容器
func NewContainer() *Container {
	return &Container{
		services: make(map[string]any),
	}
}

// Register 注册服务
func (c *Container) Register(name string, service any) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.services[name] = service
}

// RegisterSingleton 注册单例服务
func (c *Container) RegisterSingleton(name string, factory func() any) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// 延迟初始化
	c.services[name] = &singletonWrapper{
		factory: factory,
	}
}

// Get 获取服务
func (c *Container) Get(name string) (any, error) {
	c.mutex.RLock()
	service, exists := c.services[name]
	c.mutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("服务 '%s' 未注册", name)
	}

	// 检查是否为单例包装器
	if wrapper, ok := service.(*singletonWrapper); ok {
		return wrapper.getInstance(), nil
	}

	return service, nil
}

// MustGet 获取服务，如果不存在则 panic
func (c *Container) MustGet(name string) any {
	service, err := c.Get(name)
	if err != nil {
		panic(err)
	}
	return service
}

// GetAs 获取服务并转换为指定类型
func (c *Container) GetAs(name string, target any) error {
	service, err := c.Get(name)
	if err != nil {
		return err
	}

	targetValue := reflect.ValueOf(target)
	if targetValue.Kind() != reflect.Ptr {
		return fmt.Errorf("target 必须是指针类型")
	}

	serviceValue := reflect.ValueOf(service)
	targetType := targetValue.Elem().Type()

	if !serviceValue.Type().AssignableTo(targetType) {
		return fmt.Errorf("服务类型 %s 不能赋值给 %s", serviceValue.Type(), targetType)
	}

	targetValue.Elem().Set(serviceValue)
	return nil
}

// Has 检查服务是否存在
func (c *Container) Has(name string) bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	_, exists := c.services[name]
	return exists
}

// Remove 移除服务
func (c *Container) Remove(name string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	delete(c.services, name)
}

// Clear 清空所有服务
func (c *Container) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.services = make(map[string]interface{})
}

// GetServiceNames 获取所有服务名称
func (c *Container) GetServiceNames() []string {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	names := make([]string, 0, len(c.services))
	for name := range c.services {
		names = append(names, name)
	}
	return names
}

// singletonWrapper 单例包装器
type singletonWrapper struct {
	factory  func() any
	instance any
	once     sync.Once
}

// getInstance 获取单例实例
func (w *singletonWrapper) getInstance() interface{} {
	w.once.Do(func() {
		w.instance = w.factory()
	})
	return w.instance
}

// ServiceProvider 服务提供者接口，用于定义服务的注册和启动流程。
// 实现该接口的类型可以将服务注册到容器中，并在应用启动时进行初始化操作。
type ServiceProvider interface {
	// Register 用于将服务注册到依赖注入容器中，通常在应用启动前调用。
	// 参数 container：依赖注入容器实例。
	// 返回 error：注册过程中发生的错误，如果没有错误则返回 nil。
	Register(container *Container) error

	// Boot 用于在服务注册后进行初始化或启动操作，通常在应用启动时调用。
	// 参数 container：依赖注入容器实例。
	// 返回 error：启动过程中发生的错误，如果没有错误则返回 nil。
	Boot(container *Container) error
}

// Application 应用程序结构，负责管理依赖注入容器和服务提供者
type Application struct {
	container *Container        // 依赖注入容器，负责管理所有服务实例
	providers []ServiceProvider // 服务提供者列表，用于注册和启动各类服务
	booted    bool              // 标记应用程序是否已启动，防止重复启动
}

// NewApplication 创建新的应用程序
func NewApplication() *Application {
	return &Application{
		container: NewContainer(),
		providers: make([]ServiceProvider, 0),
	}
}

// GetContainer 获取容器
func (app *Application) GetContainer() *Container {
	return app.container
}

// RegisterProvider 注册服务提供者
func (app *Application) RegisterProvider(provider ServiceProvider) error {
	app.providers = append(app.providers, provider)
	return provider.Register(app.container)
}

// Boot 启动应用程序
func (app *Application) Boot() error {
	if app.booted {
		return nil
	}

	for _, provider := range app.providers {
		if err := provider.Boot(app.container); err != nil {
			return fmt.Errorf("启动服务提供者失败: %w", err)
		}
	}

	app.booted = true
	return nil
}

// IsBooted 检查是否已启动
func (app *Application) IsBooted() bool {
	return app.booted
}

// 全局容器实例
var globalContainer = NewContainer()

// RegisterGlobal 在全局容器中注册服务
func RegisterGlobal(name string, service interface{}) {
	globalContainer.Register(name, service)
}

// RegisterSingletonGlobal 在全局容器中注册单例服务
func RegisterSingletonGlobal(name string, factory func() interface{}) {
	globalContainer.RegisterSingleton(name, factory)
}

// GetGlobal 从全局容器获取服务
func GetGlobal(name string) (interface{}, error) {
	return globalContainer.Get(name)
}

// MustGetGlobal 从全局容器获取服务，如果不存在则 panic
func MustGetGlobal(name string) interface{} {
	return globalContainer.MustGet(name)
}

// GetAsGlobal 从全局容器获取服务并转换为指定类型
func GetAsGlobal(name string, target interface{}) error {
	return globalContainer.GetAs(name, target)
}

// 常用服务名称常量
const (
	ServiceConfig          = "config"
	ServiceLogger          = "logger"
	ServiceDatabase        = "database"
	ServiceCache           = "cache"
	ServiceSchemaManager   = "schema_manager"
	ServiceLangChain       = "langchain_manager"
	ServiceQueryProcessor  = "query_processor"
	ServiceMCPServer       = "mcp_server"
	ServiceSecurityManager = "security_manager"
	ServiceMetrics         = "metrics"
	ServiceErrorHandler    = "error_handler"
)
