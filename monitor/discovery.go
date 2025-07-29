// 本文件实现了服务发现功能，提供服务注册、发现、健康检查和负载均衡等功能。
// 主要功能：
// 1. 服务注册和注销
// 2. 服务发现和查询
// 3. 服务健康状态监控
// 4. 负载均衡策略
// 5. 服务变更通知
// 6. 故障转移和自动恢复

package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"sort"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ServiceStatus 服务状态枚举
type ServiceStatus string

const (
	ServiceStatusUp       ServiceStatus = "up"
	ServiceStatusDown     ServiceStatus = "down"
	ServiceStatusStarting ServiceStatus = "starting"
	ServiceStatusStopping ServiceStatus = "stopping"
	ServiceStatusUnknown  ServiceStatus = "unknown"
)

// LoadBalanceStrategy 负载均衡策略
type LoadBalanceStrategy string

const (
	LoadBalanceRoundRobin LoadBalanceStrategy = "round_robin"
	LoadBalanceRandom     LoadBalanceStrategy = "random"
	LoadBalanceWeighted   LoadBalanceStrategy = "weighted"
	LoadBalanceLeastConn  LoadBalanceStrategy = "least_conn"
)

// ServiceInstance 服务实例
type ServiceInstance struct {
	ID           string                 `json:"id"`
	ServiceName  string                 `json:"service_name"`
	Host         string                 `json:"host"`
	Port         int                    `json:"port"`
	Tags         []string               `json:"tags"`
	Metadata     map[string]string      `json:"metadata"`
	Status       ServiceStatus          `json:"status"`
	Weight       int                    `json:"weight"`
	Version      string                 `json:"version"`
	HealthCheck  *ServiceHealthCheck    `json:"health_check"`
	RegisteredAt time.Time              `json:"registered_at"`
	LastSeen     time.Time              `json:"last_seen"`
	Connections  int                    `json:"connections"`
	Metrics      map[string]interface{} `json:"metrics"`
}

// ServiceHealthCheck 服务健康检查配置
type ServiceHealthCheck struct {
	Type     string            `json:"type"`     // http, tcp, grpc
	URL      string            `json:"url"`      // 健康检查URL
	Interval time.Duration     `json:"interval"` // 检查间隔
	Timeout  time.Duration     `json:"timeout"`  // 超时时间
	Retries  int               `json:"retries"`  // 重试次数
	Headers  map[string]string `json:"headers"`  // HTTP头
}

// ServiceQuery 服务查询条件
type ServiceQuery struct {
	ServiceName string            `json:"service_name"`
	Tags        []string          `json:"tags"`
	Status      ServiceStatus     `json:"status"`
	Metadata    map[string]string `json:"metadata"`
	Version     string            `json:"version"`
	Limit       int               `json:"limit"`
}

// ServiceChangeEvent 服务变更事件
type ServiceChangeEvent struct {
	Type        string           `json:"type"` // register, deregister, status_change
	ServiceName string           `json:"service_name"`
	Instance    *ServiceInstance `json:"instance"`
	OldStatus   ServiceStatus    `json:"old_status,omitempty"`
	NewStatus   ServiceStatus    `json:"new_status,omitempty"`
	Timestamp   time.Time        `json:"timestamp"`
}

// ServiceChangeHandler 服务变更处理函数
type ServiceChangeHandler func(event ServiceChangeEvent) error

// ServiceRegistry 服务注册表接口
type ServiceRegistry interface {
	Register(ctx context.Context, instance *ServiceInstance) error
	Deregister(ctx context.Context, serviceID string) error
	Discover(ctx context.Context, query *ServiceQuery) ([]*ServiceInstance, error)
	GetService(ctx context.Context, serviceID string) (*ServiceInstance, error)
	UpdateStatus(ctx context.Context, serviceID string, status ServiceStatus) error
	Subscribe(ctx context.Context, serviceName string, handler ServiceChangeHandler) error
	Unsubscribe(ctx context.Context, serviceName string) error
	GetAllServices(ctx context.Context) (map[string][]*ServiceInstance, error)
}

// LoadBalancer 负载均衡器接口
type LoadBalancer interface {
	Select(instances []*ServiceInstance) (*ServiceInstance, error)
	UpdateConnections(instanceID string, connections int) error
	GetStrategy() LoadBalanceStrategy
	SetStrategy(strategy LoadBalanceStrategy)
}

// ServiceDiscovery 服务发现管理器
type ServiceDiscovery struct {
	registry      ServiceRegistry
	loadBalancer  LoadBalancer
	healthChecker *ServiceHealthChecker
	logger        Logger
	config        *ServiceDiscoveryConfig

	// 内部状态
	services    map[string]map[string]*ServiceInstance // serviceName -> instanceID -> instance
	subscribers map[string][]ServiceChangeHandler      // serviceName -> handlers
	mutex       sync.RWMutex
	running     bool
	stopChan    chan struct{}

	// 健康检查
	healthCheckTicker *time.Ticker
	healthCheckStop   chan struct{}
}

// ServiceDiscoveryConfig 服务发现配置
type ServiceDiscoveryConfig struct {
	HealthCheckInterval    time.Duration       `yaml:"health_check_interval"`
	HealthCheckTimeout     time.Duration       `yaml:"health_check_timeout"`
	HealthCheckRetries     int                 `yaml:"health_check_retries"`
	ServiceTTL             time.Duration       `yaml:"service_ttl"`
	LoadBalanceStrategy    LoadBalanceStrategy `yaml:"load_balance_strategy"`
	EnableHealthCheck      bool                `yaml:"enable_health_check"`
	MaxServices            int                 `yaml:"max_services"`
	MaxInstancesPerService int                 `yaml:"max_instances_per_service"`
}

// NewServiceDiscovery 创建服务发现管理器
func NewServiceDiscovery(config *ServiceDiscoveryConfig, logger Logger) *ServiceDiscovery {
	if config == nil {
		config = &ServiceDiscoveryConfig{
			HealthCheckInterval:    30 * time.Second,
			HealthCheckTimeout:     10 * time.Second,
			HealthCheckRetries:     3,
			ServiceTTL:             5 * time.Minute,
			LoadBalanceStrategy:    LoadBalanceRoundRobin,
			EnableHealthCheck:      true,
			MaxServices:            100,
			MaxInstancesPerService: 10,
		}
	}

	sd := &ServiceDiscovery{
		logger:          logger,
		config:          config,
		services:        make(map[string]map[string]*ServiceInstance),
		subscribers:     make(map[string][]ServiceChangeHandler),
		stopChan:        make(chan struct{}),
		healthCheckStop: make(chan struct{}),
	}

	// 创建内存注册表
	sd.registry = NewMemoryServiceRegistry(sd)

	// 创建负载均衡器
	sd.loadBalancer = NewRoundRobinLoadBalancer()
	sd.loadBalancer.SetStrategy(config.LoadBalanceStrategy)

	// 创建健康检查器
	sd.healthChecker = NewServiceHealthChecker(config, logger)

	return sd
}

// Start 启动服务发现
func (sd *ServiceDiscovery) Start(ctx context.Context) error {
	sd.mutex.Lock()
	if sd.running {
		sd.mutex.Unlock()
		return fmt.Errorf("服务发现已在运行")
	}
	sd.running = true
	sd.mutex.Unlock()

	sd.logger.Info("启动服务发现",
		zap.Duration("health_check_interval", sd.config.HealthCheckInterval),
		zap.String("load_balance_strategy", string(sd.config.LoadBalanceStrategy)),
	)

	// 启动健康检查
	if sd.config.EnableHealthCheck {
		go sd.runHealthCheck(ctx)
	}

	// 启动服务清理
	go sd.runServiceCleanup(ctx)

	return nil
}

// Stop 停止服务发现
func (sd *ServiceDiscovery) Stop(ctx context.Context) error {
	sd.mutex.Lock()
	if !sd.running {
		sd.mutex.Unlock()
		return nil
	}
	sd.running = false
	sd.mutex.Unlock()

	sd.logger.Info("停止服务发现")

	// 停止健康检查
	close(sd.healthCheckStop)

	// 停止服务清理
	close(sd.stopChan)

	return nil
}

// RegisterService 注册服务
func (sd *ServiceDiscovery) RegisterService(ctx context.Context, instance *ServiceInstance) error {
	// 验证服务实例
	if err := sd.validateServiceInstance(instance); err != nil {
		return fmt.Errorf("服务实例验证失败: %w", err)
	}

	// 设置默认值
	sd.setDefaultValues(instance)

	// 注册到注册表
	if err := sd.registry.Register(ctx, instance); err != nil {
		return fmt.Errorf("注册服务失败: %w", err)
	}

	sd.logger.Info("服务注册成功",
		zap.String("service_id", instance.ID),
		zap.String("service_name", instance.ServiceName),
		zap.String("address", fmt.Sprintf("%s:%d", instance.Host, instance.Port)),
	)

	return nil
}

// DeregisterService 注销服务
func (sd *ServiceDiscovery) DeregisterService(ctx context.Context, serviceID string) error {
	if err := sd.registry.Deregister(ctx, serviceID); err != nil {
		return fmt.Errorf("注销服务失败: %w", err)
	}

	sd.logger.Info("服务注销成功", zap.String("service_id", serviceID))
	return nil
}

// DiscoverServices 发现服务
func (sd *ServiceDiscovery) DiscoverServices(ctx context.Context, query *ServiceQuery) ([]*ServiceInstance, error) {
	instances, err := sd.registry.Discover(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("服务发现失败: %w", err)
	}

	// 过滤健康的服务实例
	var healthyInstances []*ServiceInstance
	for _, instance := range instances {
		if instance.Status == ServiceStatusUp {
			healthyInstances = append(healthyInstances, instance)
		}
	}

	sd.logger.Debug("服务发现完成",
		zap.String("service_name", query.ServiceName),
		zap.Int("total_instances", len(instances)),
		zap.Int("healthy_instances", len(healthyInstances)),
	)

	return healthyInstances, nil
}

// SelectService 选择服务实例（负载均衡）
func (sd *ServiceDiscovery) SelectService(ctx context.Context, serviceName string) (*ServiceInstance, error) {
	// 发现服务实例
	query := &ServiceQuery{
		ServiceName: serviceName,
		Status:      ServiceStatusUp,
	}

	instances, err := sd.DiscoverServices(ctx, query)
	if err != nil {
		return nil, err
	}

	if len(instances) == 0 {
		return nil, fmt.Errorf("没有可用的服务实例: %s", serviceName)
	}

	// 使用负载均衡器选择实例
	selected, err := sd.loadBalancer.Select(instances)
	if err != nil {
		return nil, fmt.Errorf("负载均衡选择失败: %w", err)
	}

	sd.logger.Debug("服务实例选择完成",
		zap.String("service_name", serviceName),
		zap.String("selected_instance", selected.ID),
		zap.String("address", fmt.Sprintf("%s:%d", selected.Host, selected.Port)),
	)

	return selected, nil
}

// GetServiceHealth 获取服务健康状态
func (sd *ServiceDiscovery) GetServiceHealth(ctx context.Context, serviceName string) (map[string]*HealthCheckResult, error) {
	query := &ServiceQuery{ServiceName: serviceName}
	instances, err := sd.registry.Discover(ctx, query)
	if err != nil {
		return nil, err
	}

	results := make(map[string]*HealthCheckResult)
	for _, instance := range instances {
		if instance.HealthCheck != nil {
			result := sd.healthChecker.CheckInstance(ctx, instance)
			results[instance.ID] = result
		}
	}

	return results, nil
}

// SubscribeServiceChanges 订阅服务变更
func (sd *ServiceDiscovery) SubscribeServiceChanges(ctx context.Context, serviceName string, handler ServiceChangeHandler) error {
	return sd.registry.Subscribe(ctx, serviceName, handler)
}

// UnsubscribeServiceChanges 取消订阅服务变更
func (sd *ServiceDiscovery) UnsubscribeServiceChanges(ctx context.Context, serviceName string) error {
	return sd.registry.Unsubscribe(ctx, serviceName)
}

// GetAllServices 获取所有服务
func (sd *ServiceDiscovery) GetAllServices(ctx context.Context) (map[string][]*ServiceInstance, error) {
	return sd.registry.GetAllServices(ctx)
}

// validateServiceInstance 验证服务实例
func (sd *ServiceDiscovery) validateServiceInstance(instance *ServiceInstance) error {
	if instance.ID == "" {
		return fmt.Errorf("服务ID不能为空")
	}

	if instance.ServiceName == "" {
		return fmt.Errorf("服务名称不能为空")
	}

	if instance.Host == "" {
		return fmt.Errorf("服务主机不能为空")
	}

	if instance.Port <= 0 || instance.Port > 65535 {
		return fmt.Errorf("无效的端口号: %d", instance.Port)
	}

	// 验证主机地址
	if net.ParseIP(instance.Host) == nil {
		// 如果不是IP地址，检查是否是有效的主机名
		if _, err := net.LookupHost(instance.Host); err != nil {
			return fmt.Errorf("无效的主机地址: %s", instance.Host)
		}
	}

	return nil
}

// setDefaultValues 设置默认值
func (sd *ServiceDiscovery) setDefaultValues(instance *ServiceInstance) {
	now := time.Now()

	if instance.Status == "" {
		instance.Status = ServiceStatusUp
	}

	if instance.Weight <= 0 {
		instance.Weight = 100
	}

	if instance.RegisteredAt.IsZero() {
		instance.RegisteredAt = now
	}

	instance.LastSeen = now

	if instance.Metadata == nil {
		instance.Metadata = make(map[string]string)
	}

	if instance.Metrics == nil {
		instance.Metrics = make(map[string]interface{})
	}

	// 设置默认健康检查
	if instance.HealthCheck == nil {
		instance.HealthCheck = &ServiceHealthCheck{
			Type:     "http",
			URL:      fmt.Sprintf("http://%s:%d/health", instance.Host, instance.Port),
			Interval: sd.config.HealthCheckInterval,
			Timeout:  sd.config.HealthCheckTimeout,
			Retries:  sd.config.HealthCheckRetries,
		}
	}
}

// runHealthCheck 运行健康检查
func (sd *ServiceDiscovery) runHealthCheck(ctx context.Context) {
	sd.healthCheckTicker = time.NewTicker(sd.config.HealthCheckInterval)
	defer sd.healthCheckTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-sd.healthCheckStop:
			return
		case <-sd.healthCheckTicker.C:
			sd.performHealthChecks(ctx)
		}
	}
}

// performHealthChecks 执行健康检查
func (sd *ServiceDiscovery) performHealthChecks(ctx context.Context) {
	allServices, err := sd.registry.GetAllServices(ctx)
	if err != nil {
		sd.logger.Error("获取服务列表失败", zap.Error(err))
		return
	}

	var wg sync.WaitGroup
	for _, instances := range allServices {
		for _, instance := range instances {
			if instance.HealthCheck != nil {
				wg.Add(1)
				go func(inst *ServiceInstance) {
					defer wg.Done()
					sd.checkInstanceHealth(ctx, inst)
				}(instance)
			}
		}
	}

	wg.Wait()
}

// checkInstanceHealth 检查实例健康状态
func (sd *ServiceDiscovery) checkInstanceHealth(ctx context.Context, instance *ServiceInstance) {
	result := sd.healthChecker.CheckInstance(ctx, instance)

	var newStatus ServiceStatus
	if result.Status == HealthStatusHealthy {
		newStatus = ServiceStatusUp
	} else {
		newStatus = ServiceStatusDown
	}

	// 如果状态发生变化，更新注册表
	if instance.Status != newStatus {
		oldStatus := instance.Status
		if err := sd.registry.UpdateStatus(ctx, instance.ID, newStatus); err != nil {
			sd.logger.Error("更新服务状态失败",
				zap.String("service_id", instance.ID),
				zap.String("new_status", string(newStatus)),
				zap.Error(err),
			)
		} else {
			sd.logger.Info("服务状态已更新",
				zap.String("service_id", instance.ID),
				zap.String("service_name", instance.ServiceName),
				zap.String("old_status", string(oldStatus)),
				zap.String("new_status", string(newStatus)),
			)
		}
	}

	// 更新最后检查时间
	instance.LastSeen = time.Now()
}

// runServiceCleanup 运行服务清理
func (sd *ServiceDiscovery) runServiceCleanup(ctx context.Context) {
	ticker := time.NewTicker(sd.config.ServiceTTL / 2)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-sd.stopChan:
			return
		case <-ticker.C:
			sd.cleanupExpiredServices(ctx)
		}
	}
}

// cleanupExpiredServices 清理过期服务
func (sd *ServiceDiscovery) cleanupExpiredServices(ctx context.Context) {
	allServices, err := sd.registry.GetAllServices(ctx)
	if err != nil {
		sd.logger.Error("获取服务列表失败", zap.Error(err))
		return
	}

	now := time.Now()
	for _, instances := range allServices {
		for _, instance := range instances {
			if now.Sub(instance.LastSeen) > sd.config.ServiceTTL {
				if err := sd.registry.Deregister(ctx, instance.ID); err != nil {
					sd.logger.Error("清理过期服务失败",
						zap.String("service_id", instance.ID),
						zap.Error(err),
					)
				} else {
					sd.logger.Info("清理过期服务",
						zap.String("service_id", instance.ID),
						zap.String("service_name", instance.ServiceName),
						zap.Duration("expired_duration", now.Sub(instance.LastSeen)),
					)
				}
			}
		}
	}
}

// HTTPServiceDiscoveryHandler HTTP服务发现处理器
func (sd *ServiceDiscovery) HTTPServiceDiscoveryHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		switch r.Method {
		case http.MethodGet:
			sd.handleDiscoverServices(w, r, ctx)
		case http.MethodPost:
			sd.handleRegisterService(w, r, ctx)
		case http.MethodDelete:
			sd.handleDeregisterService(w, r, ctx)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

// handleDiscoverServices 处理服务发现请求
func (sd *ServiceDiscovery) handleDiscoverServices(w http.ResponseWriter, r *http.Request, ctx context.Context) {
	serviceName := r.URL.Query().Get("service")
	if serviceName == "" {
		// 返回所有服务
		services, err := sd.GetAllServices(ctx)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(services)
		return
	}

	// 发现指定服务
	query := &ServiceQuery{ServiceName: serviceName}
	instances, err := sd.DiscoverServices(ctx, query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(instances)
}

// handleRegisterService 处理服务注册请求
func (sd *ServiceDiscovery) handleRegisterService(w http.ResponseWriter, r *http.Request, ctx context.Context) {
	var instance ServiceInstance
	if err := json.NewDecoder(r.Body).Decode(&instance); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := sd.RegisterService(ctx, &instance); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"status":     "registered",
		"service_id": instance.ID,
	})
}

// handleDeregisterService 处理服务注销请求
func (sd *ServiceDiscovery) handleDeregisterService(w http.ResponseWriter, r *http.Request, ctx context.Context) {
	serviceID := r.URL.Query().Get("id")
	if serviceID == "" {
		http.Error(w, "Service ID is required", http.StatusBadRequest)
		return
	}

	if err := sd.DeregisterService(ctx, serviceID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":     "deregistered",
		"service_id": serviceID,
	})
}

// MemoryServiceRegistry 内存服务注册表
type MemoryServiceRegistry struct {
	discovery   *ServiceDiscovery
	services    map[string]map[string]*ServiceInstance // serviceName -> instanceID -> instance
	subscribers map[string][]ServiceChangeHandler      // serviceName -> handlers
	mutex       sync.RWMutex
}

// NewMemoryServiceRegistry 创建内存服务注册表
func NewMemoryServiceRegistry(discovery *ServiceDiscovery) *MemoryServiceRegistry {
	return &MemoryServiceRegistry{
		discovery:   discovery,
		services:    make(map[string]map[string]*ServiceInstance),
		subscribers: make(map[string][]ServiceChangeHandler),
	}
}

// Register 注册服务
func (r *MemoryServiceRegistry) Register(ctx context.Context, instance *ServiceInstance) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	serviceName := instance.ServiceName
	if r.services[serviceName] == nil {
		r.services[serviceName] = make(map[string]*ServiceInstance)
	}

	// 检查服务实例数量限制
	if len(r.services[serviceName]) >= r.discovery.config.MaxInstancesPerService {
		return fmt.Errorf("服务实例数量超过限制: %d", r.discovery.config.MaxInstancesPerService)
	}

	// 检查服务总数限制
	if len(r.services) >= r.discovery.config.MaxServices {
		return fmt.Errorf("服务数量超过限制: %d", r.discovery.config.MaxServices)
	}

	r.services[serviceName][instance.ID] = instance

	// 通知订阅者
	r.notifySubscribers(ServiceChangeEvent{
		Type:        "register",
		ServiceName: serviceName,
		Instance:    instance,
		Timestamp:   time.Now(),
	})

	return nil
}

// Deregister 注销服务
func (r *MemoryServiceRegistry) Deregister(ctx context.Context, serviceID string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// 查找服务实例
	var foundInstance *ServiceInstance
	var foundServiceName string

	for serviceName, instances := range r.services {
		if instance, exists := instances[serviceID]; exists {
			foundInstance = instance
			foundServiceName = serviceName
			delete(instances, serviceID)

			// 如果服务没有实例了，删除服务
			if len(instances) == 0 {
				delete(r.services, serviceName)
			}
			break
		}
	}

	if foundInstance == nil {
		return fmt.Errorf("服务实例不存在: %s", serviceID)
	}

	// 通知订阅者
	r.notifySubscribers(ServiceChangeEvent{
		Type:        "deregister",
		ServiceName: foundServiceName,
		Instance:    foundInstance,
		Timestamp:   time.Now(),
	})

	return nil
}

// Discover 发现服务
func (r *MemoryServiceRegistry) Discover(ctx context.Context, query *ServiceQuery) ([]*ServiceInstance, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	var instances []*ServiceInstance

	// 如果指定了服务名称
	if query.ServiceName != "" {
		if serviceInstances, exists := r.services[query.ServiceName]; exists {
			for _, instance := range serviceInstances {
				if r.matchesQuery(instance, query) {
					instances = append(instances, instance)
				}
			}
		}
	} else {
		// 搜索所有服务
		for _, serviceInstances := range r.services {
			for _, instance := range serviceInstances {
				if r.matchesQuery(instance, query) {
					instances = append(instances, instance)
				}
			}
		}
	}

	// 排序和限制
	sort.Slice(instances, func(i, j int) bool {
		return instances[i].RegisteredAt.Before(instances[j].RegisteredAt)
	})

	if query.Limit > 0 && len(instances) > query.Limit {
		instances = instances[:query.Limit]
	}

	return instances, nil
}

// GetService 获取服务
func (r *MemoryServiceRegistry) GetService(ctx context.Context, serviceID string) (*ServiceInstance, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	for _, instances := range r.services {
		if instance, exists := instances[serviceID]; exists {
			return instance, nil
		}
	}

	return nil, fmt.Errorf("服务实例不存在: %s", serviceID)
}

// UpdateStatus 更新服务状态
func (r *MemoryServiceRegistry) UpdateStatus(ctx context.Context, serviceID string, status ServiceStatus) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	for serviceName, instances := range r.services {
		if instance, exists := instances[serviceID]; exists {
			oldStatus := instance.Status
			instance.Status = status
			instance.LastSeen = time.Now()

			// 通知订阅者
			r.notifySubscribers(ServiceChangeEvent{
				Type:        "status_change",
				ServiceName: serviceName,
				Instance:    instance,
				OldStatus:   oldStatus,
				NewStatus:   status,
				Timestamp:   time.Now(),
			})

			return nil
		}
	}

	return fmt.Errorf("服务实例不存在: %s", serviceID)
}

// Subscribe 订阅服务变更
func (r *MemoryServiceRegistry) Subscribe(ctx context.Context, serviceName string, handler ServiceChangeHandler) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.subscribers[serviceName] == nil {
		r.subscribers[serviceName] = make([]ServiceChangeHandler, 0)
	}
	r.subscribers[serviceName] = append(r.subscribers[serviceName], handler)

	return nil
}

// Unsubscribe 取消订阅服务变更
func (r *MemoryServiceRegistry) Unsubscribe(ctx context.Context, serviceName string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	delete(r.subscribers, serviceName)
	return nil
}

// GetAllServices 获取所有服务
func (r *MemoryServiceRegistry) GetAllServices(ctx context.Context) (map[string][]*ServiceInstance, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	result := make(map[string][]*ServiceInstance)
	for serviceName, instances := range r.services {
		serviceInstances := make([]*ServiceInstance, 0, len(instances))
		for _, instance := range instances {
			serviceInstances = append(serviceInstances, instance)
		}
		result[serviceName] = serviceInstances
	}

	return result, nil
}

// matchesQuery 检查实例是否匹配查询条件
func (r *MemoryServiceRegistry) matchesQuery(instance *ServiceInstance, query *ServiceQuery) bool {
	// 检查状态
	if query.Status != "" && instance.Status != query.Status {
		return false
	}

	// 检查版本
	if query.Version != "" && instance.Version != query.Version {
		return false
	}

	// 检查标签
	if len(query.Tags) > 0 {
		instanceTags := make(map[string]bool)
		for _, tag := range instance.Tags {
			instanceTags[tag] = true
		}

		for _, requiredTag := range query.Tags {
			if !instanceTags[requiredTag] {
				return false
			}
		}
	}

	// 检查元数据
	if len(query.Metadata) > 0 {
		for key, value := range query.Metadata {
			if instance.Metadata[key] != value {
				return false
			}
		}
	}

	return true
}

// notifySubscribers 通知订阅者
func (r *MemoryServiceRegistry) notifySubscribers(event ServiceChangeEvent) {
	if handlers, exists := r.subscribers[event.ServiceName]; exists {
		for _, handler := range handlers {
			go func(h ServiceChangeHandler) {
				if err := h(event); err != nil {
					r.discovery.logger.Error("服务变更通知处理失败",
						zap.String("service_name", event.ServiceName),
						zap.String("event_type", event.Type),
						zap.Error(err),
					)
				}
			}(handler)
		}
	}

	// 通知全局订阅者
	if handlers, exists := r.subscribers["*"]; exists {
		for _, handler := range handlers {
			go func(h ServiceChangeHandler) {
				if err := h(event); err != nil {
					r.discovery.logger.Error("全局服务变更通知处理失败",
						zap.String("service_name", event.ServiceName),
						zap.String("event_type", event.Type),
						zap.Error(err),
					)
				}
			}(handler)
		}
	}
}

// RoundRobinLoadBalancer 轮询负载均衡器
type RoundRobinLoadBalancer struct {
	strategy LoadBalanceStrategy
	counters map[string]int // serviceName -> counter
	mutex    sync.Mutex
}

// NewRoundRobinLoadBalancer 创建轮询负载均衡器
func NewRoundRobinLoadBalancer() *RoundRobinLoadBalancer {
	return &RoundRobinLoadBalancer{
		strategy: LoadBalanceRoundRobin,
		counters: make(map[string]int),
	}
}

// Select 选择服务实例
func (lb *RoundRobinLoadBalancer) Select(instances []*ServiceInstance) (*ServiceInstance, error) {
	if len(instances) == 0 {
		return nil, fmt.Errorf("没有可用的服务实例")
	}

	switch lb.strategy {
	case LoadBalanceRoundRobin:
		return lb.selectRoundRobin(instances)
	case LoadBalanceRandom:
		return lb.selectRandom(instances)
	case LoadBalanceWeighted:
		return lb.selectWeighted(instances)
	case LoadBalanceLeastConn:
		return lb.selectLeastConn(instances)
	default:
		return lb.selectRoundRobin(instances)
	}
}

// selectRoundRobin 轮询选择
func (lb *RoundRobinLoadBalancer) selectRoundRobin(instances []*ServiceInstance) (*ServiceInstance, error) {
	lb.mutex.Lock()
	defer lb.mutex.Unlock()

	if len(instances) == 0 {
		return nil, fmt.Errorf("没有可用的服务实例")
	}

	serviceName := instances[0].ServiceName
	counter := lb.counters[serviceName]
	selected := instances[counter%len(instances)]
	lb.counters[serviceName] = counter + 1

	return selected, nil
}

// selectRandom 随机选择
func (lb *RoundRobinLoadBalancer) selectRandom(instances []*ServiceInstance) (*ServiceInstance, error) {
	if len(instances) == 0 {
		return nil, fmt.Errorf("没有可用的服务实例")
	}

	index := rand.Intn(len(instances))
	return instances[index], nil
}

// selectWeighted 加权选择
func (lb *RoundRobinLoadBalancer) selectWeighted(instances []*ServiceInstance) (*ServiceInstance, error) {
	if len(instances) == 0 {
		return nil, fmt.Errorf("没有可用的服务实例")
	}

	// 计算总权重
	totalWeight := 0
	for _, instance := range instances {
		totalWeight += instance.Weight
	}

	if totalWeight == 0 {
		// 如果所有权重都为0，使用轮询
		return lb.selectRoundRobin(instances)
	}

	// 随机选择
	random := rand.Intn(totalWeight)
	currentWeight := 0

	for _, instance := range instances {
		currentWeight += instance.Weight
		if random < currentWeight {
			return instance, nil
		}
	}

	// 默认返回第一个
	return instances[0], nil
}

// selectLeastConn 最少连接选择
func (lb *RoundRobinLoadBalancer) selectLeastConn(instances []*ServiceInstance) (*ServiceInstance, error) {
	if len(instances) == 0 {
		return nil, fmt.Errorf("没有可用的服务实例")
	}

	var selected *ServiceInstance
	minConnections := int(^uint(0) >> 1) // 最大整数

	for _, instance := range instances {
		if instance.Connections < minConnections {
			minConnections = instance.Connections
			selected = instance
		}
	}

	if selected == nil {
		selected = instances[0]
	}

	return selected, nil
}

// UpdateConnections 更新连接数
func (lb *RoundRobinLoadBalancer) UpdateConnections(instanceID string, connections int) error {
	// 这里需要实现连接数更新逻辑
	// 由于是内存实现，可以通过回调或其他方式更新
	return nil
}

// GetStrategy 获取策略
func (lb *RoundRobinLoadBalancer) GetStrategy() LoadBalanceStrategy {
	return lb.strategy
}

// SetStrategy 设置策略
func (lb *RoundRobinLoadBalancer) SetStrategy(strategy LoadBalanceStrategy) {
	lb.strategy = strategy
}
