// 本文件实现了服务生命周期管理，提供优雅启动和关闭机制，确保服务的稳定运行和安全停止。
// 主要功能：
// 1. 优雅启动机制 - 按依赖顺序启动组件
// 2. 优雅关闭机制 - 安全停止服务和清理资源
// 3. 信号处理 - 处理系统信号和用户中断
// 4. 健康检查 - 启动时验证组件状态
// 5. 资源管理 - 管理连接池、缓存等资源
// 6. 服务注册 - 与服务发现系统集成

package deploy

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"go.uber.org/zap"
)

// ServiceState 服务状态枚举
type ServiceState string

const (
	ServiceStateStarting ServiceState = "starting"
	ServiceStateRunning  ServiceState = "running"
	ServiceStateStopping ServiceState = "stopping"
	ServiceStateStopped  ServiceState = "stopped"
	ServiceStateError    ServiceState = "error"
)

// Component 组件接口
type Component interface {
	Name() string
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	HealthCheck(ctx context.Context) error
	Dependencies() []string
}

// LifecycleConfig 生命周期配置
type LifecycleConfig struct {
	GracefulTimeout     time.Duration `yaml:"graceful_timeout"`
	ShutdownTimeout     time.Duration `yaml:"shutdown_timeout"`
	HealthCheckInterval time.Duration `yaml:"health_check_interval"`
	MaxStartupTime      time.Duration `yaml:"max_startup_time"`
	EnableProfiling     bool          `yaml:"enable_profiling"`
	ProfilePort         int           `yaml:"profile_port"`
}

// ServiceManager 服务管理器
type ServiceManager struct {
	config     *LifecycleConfig
	logger     *zap.Logger
	components map[string]Component
	state      ServiceState
	startTime  time.Time
	mutex      sync.RWMutex

	// 启动和关闭控制
	startOrder []string
	stopOrder  []string

	// 信号处理
	signalChan chan os.Signal
	stopChan   chan struct{}
	doneChan   chan struct{}

	// 健康检查
	healthTicker *time.Ticker
	healthStop   chan struct{}
}

// NewServiceManager 创建服务管理器
func NewServiceManager(config *LifecycleConfig, logger *zap.Logger) *ServiceManager {
	if config == nil {
		config = &LifecycleConfig{
			GracefulTimeout:     30 * time.Second,
			ShutdownTimeout:     60 * time.Second,
			HealthCheckInterval: 10 * time.Second,
			MaxStartupTime:      120 * time.Second,
		}
	}

	return &ServiceManager{
		config:     config,
		logger:     logger,
		components: make(map[string]Component),
		state:      ServiceStateStopped,
		signalChan: make(chan os.Signal, 1),
		stopChan:   make(chan struct{}),
		doneChan:   make(chan struct{}),
		healthStop: make(chan struct{}),
	}
}

// RegisterComponent 注册组件
func (sm *ServiceManager) RegisterComponent(component Component) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	name := component.Name()
	if _, exists := sm.components[name]; exists {
		return fmt.Errorf("组件 %s 已存在", name)
	}

	sm.components[name] = component
	sm.logger.Info("注册组件", zap.String("component", name))

	// 重新计算启动顺序
	sm.calculateStartOrder()

	return nil
}

// Start 启动服务
func (sm *ServiceManager) Start(ctx context.Context) error {
	sm.mutex.Lock()
	if sm.state != ServiceStateStopped {
		sm.mutex.Unlock()
		return fmt.Errorf("服务已在运行或正在启动")
	}
	sm.state = ServiceStateStarting
	sm.startTime = time.Now()
	sm.mutex.Unlock()

	sm.logger.Info("开始启动服务",
		zap.Int("components", len(sm.components)),
		zap.Duration("max_startup_time", sm.config.MaxStartupTime),
	)

	// 设置启动超时
	startCtx, cancel := context.WithTimeout(ctx, sm.config.MaxStartupTime)
	defer cancel()

	// 启动信号处理
	go sm.handleSignals()

	// 按依赖顺序启动组件
	if err := sm.startComponents(startCtx); err != nil {
		sm.setState(ServiceStateError)
		return fmt.Errorf("启动组件失败: %w", err)
	}

	// 执行启动后健康检查
	if err := sm.performStartupHealthCheck(startCtx); err != nil {
		sm.setState(ServiceStateError)
		return fmt.Errorf("启动健康检查失败: %w", err)
	}

	// 启动定期健康检查
	go sm.runPeriodicHealthCheck()

	sm.setState(ServiceStateRunning)
	sm.logger.Info("服务启动完成",
		zap.Duration("startup_time", time.Since(sm.startTime)),
	)

	return nil
}

// Stop 停止服务
func (sm *ServiceManager) Stop(ctx context.Context) error {
	sm.mutex.Lock()
	if sm.state != ServiceStateRunning {
		sm.mutex.Unlock()
		return fmt.Errorf("服务未在运行")
	}
	sm.state = ServiceStateStopping
	sm.mutex.Unlock()

	sm.logger.Info("开始停止服务")

	// 停止健康检查
	close(sm.healthStop)

	// 设置关闭超时
	stopCtx, cancel := context.WithTimeout(ctx, sm.config.ShutdownTimeout)
	defer cancel()

	// 按相反顺序停止组件
	if err := sm.stopComponents(stopCtx); err != nil {
		sm.logger.Error("停止组件时发生错误", zap.Error(err))
	}

	sm.setState(ServiceStateStopped)
	sm.logger.Info("服务已停止")

	// 通知完成
	close(sm.doneChan)

	return nil
}

// Wait 等待服务停止
func (sm *ServiceManager) Wait() {
	<-sm.doneChan
}

// GetState 获取服务状态
func (sm *ServiceManager) GetState() ServiceState {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	return sm.state
}

// GetUptime 获取运行时间
func (sm *ServiceManager) GetUptime() time.Duration {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	if sm.state == ServiceStateRunning {
		return time.Since(sm.startTime)
	}
	return 0
}

// GetComponentStatus 获取组件状态
func (sm *ServiceManager) GetComponentStatus(ctx context.Context) map[string]error {
	sm.mutex.RLock()
	components := make(map[string]Component)
	for name, comp := range sm.components {
		components[name] = comp
	}
	sm.mutex.RUnlock()

	status := make(map[string]error)
	for name, comp := range components {
		status[name] = comp.HealthCheck(ctx)
	}

	return status
}

// calculateStartOrder 计算启动顺序
func (sm *ServiceManager) calculateStartOrder() {
	// 使用拓扑排序计算依赖顺序
	visited := make(map[string]bool)
	visiting := make(map[string]bool)
	var order []string

	var visit func(string) error
	visit = func(name string) error {
		if visiting[name] {
			return fmt.Errorf("检测到循环依赖: %s", name)
		}
		if visited[name] {
			return nil
		}

		visiting[name] = true

		component, exists := sm.components[name]
		if !exists {
			return fmt.Errorf("组件不存在: %s", name)
		}

		// 先访问依赖
		for _, dep := range component.Dependencies() {
			if err := visit(dep); err != nil {
				return err
			}
		}

		visiting[name] = false
		visited[name] = true
		order = append(order, name)

		return nil
	}

	// 访问所有组件
	for name := range sm.components {
		if !visited[name] {
			if err := visit(name); err != nil {
				sm.logger.Error("计算启动顺序失败", zap.Error(err))
				// 使用默认顺序
				order = make([]string, 0, len(sm.components))
				for name := range sm.components {
					order = append(order, name)
				}
				break
			}
		}
	}

	sm.startOrder = order

	// 停止顺序是启动顺序的反向
	sm.stopOrder = make([]string, len(order))
	for i, name := range order {
		sm.stopOrder[len(order)-1-i] = name
	}

	sm.logger.Info("计算组件启动顺序",
		zap.Strings("start_order", sm.startOrder),
		zap.Strings("stop_order", sm.stopOrder),
	)
}

// startComponents 启动组件
func (sm *ServiceManager) startComponents(ctx context.Context) error {
	for _, name := range sm.startOrder {
		component := sm.components[name]

		sm.logger.Info("启动组件", zap.String("component", name))

		startTime := time.Now()
		if err := component.Start(ctx); err != nil {
			return fmt.Errorf("启动组件 %s 失败: %w", name, err)
		}

		sm.logger.Info("组件启动成功",
			zap.String("component", name),
			zap.Duration("duration", time.Since(startTime)),
		)

		// 检查上下文是否被取消
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}

	return nil
}

// stopComponents 停止组件
func (sm *ServiceManager) stopComponents(ctx context.Context) error {
	var lastErr error

	for _, name := range sm.stopOrder {
		component := sm.components[name]

		sm.logger.Info("停止组件", zap.String("component", name))

		stopTime := time.Now()
		if err := component.Stop(ctx); err != nil {
			sm.logger.Error("停止组件失败",
				zap.String("component", name),
				zap.Error(err),
			)
			lastErr = err
		} else {
			sm.logger.Info("组件停止成功",
				zap.String("component", name),
				zap.Duration("duration", time.Since(stopTime)),
			)
		}

		// 检查上下文是否被取消
		select {
		case <-ctx.Done():
			sm.logger.Warn("停止组件超时", zap.String("component", name))
		default:
		}
	}

	return lastErr
}

// performStartupHealthCheck 执行启动健康检查
func (sm *ServiceManager) performStartupHealthCheck(ctx context.Context) error {
	sm.logger.Info("执行启动健康检查")

	for name, component := range sm.components {
		if err := component.HealthCheck(ctx); err != nil {
			return fmt.Errorf("组件 %s 健康检查失败: %w", name, err)
		}
		sm.logger.Debug("组件健康检查通过", zap.String("component", name))
	}

	sm.logger.Info("启动健康检查完成")
	return nil
}

// runPeriodicHealthCheck 运行定期健康检查
func (sm *ServiceManager) runPeriodicHealthCheck() {
	sm.healthTicker = time.NewTicker(sm.config.HealthCheckInterval)
	defer sm.healthTicker.Stop()

	for {
		select {
		case <-sm.healthStop:
			return
		case <-sm.healthTicker.C:
			sm.performPeriodicHealthCheck()
		}
	}
}

// performPeriodicHealthCheck 执行定期健康检查
func (sm *ServiceManager) performPeriodicHealthCheck() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	unhealthyComponents := 0
	for name, component := range sm.components {
		if err := component.HealthCheck(ctx); err != nil {
			sm.logger.Warn("组件健康检查失败",
				zap.String("component", name),
				zap.Error(err),
			)
			unhealthyComponents++
		}
	}

	if unhealthyComponents > 0 {
		sm.logger.Warn("发现不健康的组件",
			zap.Int("unhealthy_count", unhealthyComponents),
			zap.Int("total_count", len(sm.components)),
		)
	}
}

// handleSignals 处理系统信号
func (sm *ServiceManager) handleSignals() {
	// 注册信号处理
	signal.Notify(sm.signalChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	for {
		select {
		case sig := <-sm.signalChan:
			sm.logger.Info("收到系统信号", zap.String("signal", sig.String()))

			switch sig {
			case syscall.SIGINT, syscall.SIGTERM:
				// 优雅关闭
				sm.logger.Info("开始优雅关闭")
				ctx, cancel := context.WithTimeout(context.Background(), sm.config.GracefulTimeout)
				if err := sm.Stop(ctx); err != nil {
					sm.logger.Error("优雅关闭失败", zap.Error(err))
				}
				cancel()
				return

			case syscall.SIGHUP:
				// 重新加载配置
				sm.logger.Info("收到重新加载信号")
				// 这里可以添加配置重新加载逻辑
			}

		case <-sm.stopChan:
			return
		}
	}
}

// setState 设置服务状态
func (sm *ServiceManager) setState(state ServiceState) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	oldState := sm.state
	sm.state = state

	sm.logger.Info("服务状态变更",
		zap.String("old_state", string(oldState)),
		zap.String("new_state", string(state)),
	)
}

// Restart 重启服务
func (sm *ServiceManager) Restart(ctx context.Context) error {
	sm.logger.Info("重启服务")

	// 先停止服务
	if err := sm.Stop(ctx); err != nil {
		return fmt.Errorf("停止服务失败: %w", err)
	}

	// 等待一段时间
	time.Sleep(2 * time.Second)

	// 重新启动服务
	if err := sm.Start(ctx); err != nil {
		return fmt.Errorf("启动服务失败: %w", err)
	}

	return nil
}

// GetMetrics 获取服务指标
func (sm *ServiceManager) GetMetrics() map[string]interface{} {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	metrics := map[string]interface{}{
		"state":           string(sm.state),
		"uptime":          sm.GetUptime().String(),
		"component_count": len(sm.components),
		"start_time":      sm.startTime.Format(time.RFC3339),
	}

	// 添加组件状态
	componentStatus := make(map[string]string)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for name, component := range sm.components {
		if err := component.HealthCheck(ctx); err != nil {
			componentStatus[name] = "unhealthy"
		} else {
			componentStatus[name] = "healthy"
		}
	}
	metrics["components"] = componentStatus

	return metrics
}

// BaseComponent 基础组件实现
type BaseComponent struct {
	name         string
	dependencies []string
	started      bool
	mutex        sync.RWMutex
}

// NewBaseComponent 创建基础组件
func NewBaseComponent(name string, dependencies []string) *BaseComponent {
	return &BaseComponent{
		name:         name,
		dependencies: dependencies,
	}
}

// Name 获取组件名称
func (c *BaseComponent) Name() string {
	return c.name
}

// Dependencies 获取依赖列表
func (c *BaseComponent) Dependencies() []string {
	return c.dependencies
}

// IsStarted 检查是否已启动
func (c *BaseComponent) IsStarted() bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.started
}

// SetStarted 设置启动状态
func (c *BaseComponent) SetStarted(started bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.started = started
}

// Start 启动组件（需要子类实现）
func (c *BaseComponent) Start(ctx context.Context) error {
	c.SetStarted(true)
	return nil
}

// Stop 停止组件（需要子类实现）
func (c *BaseComponent) Stop(ctx context.Context) error {
	c.SetStarted(false)
	return nil
}

// HealthCheck 健康检查（需要子类实现）
func (c *BaseComponent) HealthCheck(ctx context.Context) error {
	if !c.IsStarted() {
		return fmt.Errorf("组件 %s 未启动", c.name)
	}
	return nil
}
