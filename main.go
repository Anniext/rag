package main

import (
	"context"
	"fmt"
	"github.com/Anniext/rag/core"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// 创建应用程序
	app := core.NewApplication()

	// 获取配置文件路径
	configPath := getConfigPath()

	// 注册默认服务提供者
	defaultProvider := core.NewDefaultServiceProvider(configPath)
	if err := app.RegisterProvider(defaultProvider); err != nil {
		log.Fatal("注册默认服务提供者失败:", err)
	}
	// 启动应用程序
	if err := app.Boot(); err != nil {
		log.Fatal("启动应用程序失败:", err)
	}

	// 获取服务
	container := app.GetContainer()

	// 获取日志服务
	loggerService, err := container.Get(core.ServiceLogger)
	if err != nil {
		log.Fatal("获取日志服务失败:", err)
	}

	logger, ok := loggerService.(core.Logger)
	if !ok {
		log.Fatal("日志服务类型错误")
	}

	logger.Info("RAG 系统启动成功",
		"config_path", configPath,
		"services", container.GetServiceNames(),
	)

	// 创建上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动服务
	if err := startServices(ctx, container, logger); err != nil {
		logger.Error("启动服务失败", "error", err)
		return
	}

	// 等待信号
	waitForSignal(cancel, logger)

	logger.Info("RAG 系统正在关闭...")

	// 停止服务
	stopServices(ctx, container, logger)

	logger.Info("RAG 系统已关闭")
}

// getConfigPath 获取配置文件路径
func getConfigPath() string {
	if path := os.Getenv("RAG_CONFIG_PATH"); path != "" {
		return path
	}

	// 检查默认路径
	defaultPaths := []string{
		"config/rag.yaml",
		"rag/config/rag.yaml",
		"./rag.yaml",
	}

	for _, path := range defaultPaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return "rag/config/rag.yaml"
}

// startServices 启动服务
func startServices(ctx context.Context, container *core.Container, logger core.Logger) error {
	logger.Info("正在启动服务...")

	// 这里可以启动各种服务，如 MCP 服务器、HTTP 服务器等
	// 由于这些服务还没有实现，我们先记录一下

	services := []string{
		"Schema Manager",
		"LangChain Manager",
		"Query Processor",
		"MCP Server",
		"Cache Manager",
		"Security Manager",
	}

	for _, service := range services {
		logger.Info("服务已准备就绪", "service", service)
	}

	return nil
}

// stopServices 停止服务
func stopServices(ctx context.Context, container *core.Container, logger core.Logger) {
	logger.Info("正在停止服务...")

	// 这里可以停止各种服务
	// 由于这些服务还没有实现，我们先记录一下

	services := []string{
		"MCP Server",
		"Cache Manager",
		"Query Processor",
		"LangChain Manager",
		"Schema Manager",
	}

	for _, service := range services {
		logger.Info("服务已停止", "service", service)
	}
}

// waitForSignal 等待系统信号
func waitForSignal(cancel context.CancelFunc, logger core.Logger) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigChan
	logger.Info("收到系统信号", "signal", sig.String())
	cancel()
}

// 演示如何使用容器
func demonstrateContainer() {
	fmt.Println("=== 依赖注入容器演示 ===")

	// 创建容器
	container := core.NewContainer()

	// 注册服务
	container.Register("greeting", "Hello, World!")

	// 注册单例服务
	container.RegisterSingleton("counter", func() interface{} {
		return &Counter{value: 0}
	})

	// 获取服务
	greeting, err := container.Get("greeting")
	if err != nil {
		fmt.Printf("获取服务失败: %v\n", err)
		return
	}
	fmt.Printf("Greeting: %s\n", greeting)

	// 获取单例服务
	counter1, _ := container.Get("counter")
	counter2, _ := container.Get("counter")

	c1 := counter1.(*Counter)
	c2 := counter2.(*Counter)

	c1.Increment()
	fmt.Printf("Counter1: %d\n", c1.GetValue())
	fmt.Printf("Counter2: %d\n", c2.GetValue()) // 应该是相同的值，因为是单例

	// 检查服务是否存在
	fmt.Printf("Has greeting: %v\n", container.Has("greeting"))
	fmt.Printf("Has unknown: %v\n", container.Has("unknown"))

	// 获取所有服务名称
	fmt.Printf("Services: %v\n", container.GetServiceNames())
}

// Counter 计数器示例
type Counter struct {
	value int
}

// Increment 增加计数
func (c *Counter) Increment() {
	c.value++
}

// GetValue 获取计数值
func (c *Counter) GetValue() int {
	return c.value
}
