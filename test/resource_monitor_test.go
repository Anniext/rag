package test

import (
	"context"
	"fmt"
	"github.com/Anniext/rag/core"
	"os"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// ResourceMonitor 资源监控器
type ResourceMonitor struct {
	logger         core.Logger
	isRunning      bool
	stopChan       chan struct{}
	metrics        *ResourceMetrics
	metricsMutex   sync.RWMutex
	sampleInterval time.Duration
}

// ResourceMetrics 资源指标
type ResourceMetrics struct {
	StartTime          time.Time
	EndTime            time.Time
	Samples            []*ResourceSample
	PeakMemory         uint64
	PeakGoroutines     int
	TotalGCPause       time.Duration
	TotalGCCount       uint32
	MemoryGrowthRate   float64 // MB/s
	GoroutineGrowth    int
	CPUUsagePercent    float64
	MemoryLeakDetected bool
}

// ResourceSample 资源采样
type ResourceSample struct {
	Timestamp      time.Time
	MemoryUsage    uint64
	GoroutineCount int
	GCCount        uint32
	GCPauseTotal   time.Duration
	HeapObjects    uint64
	StackInUse     uint64
	SystemMemory   uint64
}

// NewResourceMonitor 创建资源监控器
func NewResourceMonitor(sampleInterval time.Duration) *ResourceMonitor {
	zapLogger, _ := zap.NewDevelopment()
	logger := NewZapLoggerAdapter(zapLogger)

	return &ResourceMonitor{
		logger:         logger,
		stopChan:       make(chan struct{}),
		sampleInterval: sampleInterval,
		metrics: &ResourceMetrics{
			Samples: make([]*ResourceSample, 0),
		},
	}
}

// Start 开始监控
func (rm *ResourceMonitor) Start() {
	rm.metricsMutex.Lock()
	if rm.isRunning {
		rm.metricsMutex.Unlock()
		return
	}
	rm.isRunning = true
	rm.metrics.StartTime = time.Now()
	rm.metricsMutex.Unlock()

	go rm.monitorLoop()
}

// Stop 停止监控
func (rm *ResourceMonitor) Stop() {
	rm.metricsMutex.Lock()
	if !rm.isRunning {
		rm.metricsMutex.Unlock()
		return
	}
	rm.isRunning = false
	rm.metrics.EndTime = time.Now()
	rm.metricsMutex.Unlock()

	close(rm.stopChan)
	rm.analyzeMetrics()
}

// monitorLoop 监控循环
func (rm *ResourceMonitor) monitorLoop() {
	ticker := time.NewTicker(rm.sampleInterval)
	defer ticker.Stop()

	for {
		select {
		case <-rm.stopChan:
			return
		case <-ticker.C:
			rm.collectSample()
		}
	}
}

// collectSample 收集样本
func (rm *ResourceMonitor) collectSample() {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	sample := &ResourceSample{
		Timestamp:      time.Now(),
		MemoryUsage:    memStats.Alloc,
		GoroutineCount: runtime.NumGoroutine(),
		GCCount:        memStats.NumGC,
		GCPauseTotal:   time.Duration(memStats.PauseTotalNs),
		HeapObjects:    memStats.HeapObjects,
		StackInUse:     memStats.StackInuse,
		SystemMemory:   memStats.Sys,
	}

	rm.metricsMutex.Lock()
	rm.metrics.Samples = append(rm.metrics.Samples, sample)

	// 更新峰值
	if sample.MemoryUsage > rm.metrics.PeakMemory {
		rm.metrics.PeakMemory = sample.MemoryUsage
	}
	if sample.GoroutineCount > rm.metrics.PeakGoroutines {
		rm.metrics.PeakGoroutines = sample.GoroutineCount
	}

	rm.metricsMutex.Unlock()
}

// analyzeMetrics 分析指标
func (rm *ResourceMonitor) analyzeMetrics() {
	rm.metricsMutex.Lock()
	defer rm.metricsMutex.Unlock()

	if len(rm.metrics.Samples) < 2 {
		return
	}

	firstSample := rm.metrics.Samples[0]
	lastSample := rm.metrics.Samples[len(rm.metrics.Samples)-1]

	// 计算内存增长率
	duration := lastSample.Timestamp.Sub(firstSample.Timestamp).Seconds()
	if duration > 0 {
		memoryGrowthBytes := float64(lastSample.MemoryUsage) - float64(firstSample.MemoryUsage)
		rm.metrics.MemoryGrowthRate = (memoryGrowthBytes / 1024 / 1024) / duration // MB/s
	}

	// 计算协程增长
	rm.metrics.GoroutineGrowth = lastSample.GoroutineCount - firstSample.GoroutineCount

	// 计算总 GC 统计
	rm.metrics.TotalGCCount = lastSample.GCCount - firstSample.GCCount
	rm.metrics.TotalGCPause = lastSample.GCPauseTotal - firstSample.GCPauseTotal

	// 检测内存泄漏
	rm.detectMemoryLeak()
}

// detectMemoryLeak 检测内存泄漏
func (rm *ResourceMonitor) detectMemoryLeak() {
	if len(rm.metrics.Samples) < 10 {
		return
	}

	// 检查内存是否持续增长
	recentSamples := rm.metrics.Samples[len(rm.metrics.Samples)-10:]
	increasingCount := 0

	for i := 1; i < len(recentSamples); i++ {
		if recentSamples[i].MemoryUsage > recentSamples[i-1].MemoryUsage {
			increasingCount++
		}
	}

	// 如果最近90%的采样都在增长，可能存在内存泄漏
	if float64(increasingCount)/float64(len(recentSamples)-1) > 0.9 {
		rm.metrics.MemoryLeakDetected = true
	}
}

// GetMetrics 获取指标
func (rm *ResourceMonitor) GetMetrics() *ResourceMetrics {
	rm.metricsMutex.RLock()
	defer rm.metricsMutex.RUnlock()

	// 返回副本
	metrics := *rm.metrics
	metrics.Samples = make([]*ResourceSample, len(rm.metrics.Samples))
	copy(metrics.Samples, rm.metrics.Samples)

	return &metrics
}

// PrintReport 打印报告
func (rm *ResourceMonitor) PrintReport() {
	metrics := rm.GetMetrics()

	rm.logger.Info("资源监控报告",
		"duration", metrics.EndTime.Sub(metrics.StartTime),
		"samples_count", len(metrics.Samples),
		"peak_memory_mb", metrics.PeakMemory/1024/1024,
		"peak_goroutines", metrics.PeakGoroutines,
		"memory_growth_rate_mb_per_sec", fmt.Sprintf("%.2f", metrics.MemoryGrowthRate),
		"goroutine_growth", metrics.GoroutineGrowth,
		"total_gc_count", metrics.TotalGCCount,
		"total_gc_pause", metrics.TotalGCPause,
		"memory_leak_detected", metrics.MemoryLeakDetected,
	)

	// 打印详细的内存使用趋势
	if len(metrics.Samples) > 0 {
		rm.logger.Info("内存使用趋势")
		sampleStep := len(metrics.Samples) / 10
		if sampleStep == 0 {
			sampleStep = 1
		}

		for i := 0; i < len(metrics.Samples); i += sampleStep {
			sample := metrics.Samples[i]
			rm.logger.Info("内存采样",
				"time", sample.Timestamp.Format("15:04:05"),
				"memory_mb", sample.MemoryUsage/1024/1024,
				"goroutines", sample.GoroutineCount,
				"heap_objects", sample.HeapObjects,
			)
		}
	}
}

// TestResourceMonitoring 测试资源监控
func TestResourceMonitoring(t *testing.T) {
	if os.Getenv("RESOURCE_MONITOR_TEST") != "true" {
		t.Skip("跳过资源监控测试，设置 RESOURCE_MONITOR_TEST=true 启用")
	}

	// 创建资源监控器
	monitor := NewResourceMonitor(100 * time.Millisecond)

	// 开始监控
	monitor.Start()

	// 执行一些内存密集型操作
	ctx := context.Background()
	suite := NewComprehensivePerformanceTestSuite()
	err := suite.Setup()
	require.NoError(t, err)
	defer suite.Cleanup()

	// 模拟负载
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for j := 0; j < 100; j++ {
				// 创建一些临时对象
				data := make([]byte, 1024*1024) // 1MB
				_ = data

				// 执行查询
				request := &core.QueryRequest{
					Query:     fmt.Sprintf("测试查询 worker_%d query_%d", workerID, j),
					RequestID: fmt.Sprintf("monitor_test_%d_%d", workerID, j),
				}

				if suite.queryProcessor != nil {
					suite.queryProcessor.ProcessQuery(ctx, request)
				}

				time.Sleep(10 * time.Millisecond)
			}
		}(i)
	}

	wg.Wait()

	// 停止监控
	monitor.Stop()

	// 获取并验证指标
	metrics := monitor.GetMetrics()
	require.NotNil(t, metrics)
	require.True(t, len(metrics.Samples) > 0, "应该收集到资源样本")
	require.True(t, metrics.PeakMemory > 0, "应该记录到内存使用")
	require.True(t, metrics.PeakGoroutines > 0, "应该记录到协程数量")

	// 打印报告
	monitor.PrintReport()

	// 验证内存使用合理
	peakMemoryMB := metrics.PeakMemory / 1024 / 1024
	require.True(t, peakMemoryMB < 2000, "内存使用过高: %d MB", peakMemoryMB)

	// 验证协程数量合理
	require.True(t, metrics.PeakGoroutines < 1000, "协程数量过多: %d", metrics.PeakGoroutines)

	// 检查内存泄漏
	if metrics.MemoryLeakDetected {
		t.Logf("警告: 检测到可能的内存泄漏")
	}
}

// TestMemoryLeakDetection 测试内存泄漏检测
func TestMemoryLeakDetection(t *testing.T) {
	if os.Getenv("RESOURCE_MONITOR_TEST") != "true" {
		t.Skip("跳过资源监控测试，设置 RESOURCE_MONITOR_TEST=true 启用")
	}

	monitor := NewResourceMonitor(50 * time.Millisecond)
	monitor.Start()

	// 模拟内存泄漏
	var leakedData [][]byte
	for i := 0; i < 100; i++ {
		// 持续分配内存但不释放
		data := make([]byte, 1024*1024) // 1MB
		leakedData = append(leakedData, data)
		time.Sleep(20 * time.Millisecond)
	}

	time.Sleep(2 * time.Second) // 等待监控收集足够的样本

	monitor.Stop()

	metrics := monitor.GetMetrics()
	monitor.PrintReport()

	// 验证内存泄漏检测
	t.Logf("内存增长率: %.2f MB/s", metrics.MemoryGrowthRate)
	t.Logf("内存泄漏检测: %v", metrics.MemoryLeakDetected)

	// 清理泄漏的内存
	leakedData = nil
	runtime.GC()
}

// TestGoroutineLeakDetection 测试协程泄漏检测
func TestGoroutineLeakDetection(t *testing.T) {
	if os.Getenv("RESOURCE_MONITOR_TEST") != "true" {
		t.Skip("跳过资源监控测试，设置 RESOURCE_MONITOR_TEST=true 启用")
	}

	monitor := NewResourceMonitor(100 * time.Millisecond)
	monitor.Start()

	initialGoroutines := runtime.NumGoroutine()

	// 创建一些不会退出的协程（模拟协程泄漏）
	var channels []chan struct{}
	for i := 0; i < 50; i++ {
		ch := make(chan struct{})
		channels = append(channels, ch)

		go func() {
			<-ch // 永远等待
		}()
	}

	time.Sleep(2 * time.Second)

	monitor.Stop()

	metrics := monitor.GetMetrics()
	monitor.PrintReport()

	currentGoroutines := runtime.NumGoroutine()
	goroutineIncrease := currentGoroutines - initialGoroutines

	t.Logf("初始协程数: %d", initialGoroutines)
	t.Logf("当前协程数: %d", currentGoroutines)
	t.Logf("协程增长: %d", goroutineIncrease)
	t.Logf("监控记录的协程增长: %d", metrics.GoroutineGrowth)

	// 验证协程泄漏检测
	require.True(t, goroutineIncrease >= 50, "应该检测到协程增长")

	// 清理协程
	for _, ch := range channels {
		close(ch)
	}

	time.Sleep(100 * time.Millisecond) // 等待协程退出
}

// TestLongTermResourceStability 测试长期资源稳定性
func TestLongTermResourceStability(t *testing.T) {
	if os.Getenv("LONG_TERM_STABILITY_TEST") != "true" {
		t.Skip("跳过长期稳定性测试，设置 LONG_TERM_STABILITY_TEST=true 启用")
	}

	monitor := NewResourceMonitor(1 * time.Second)
	monitor.Start()

	// 运行长期稳定性测试
	testDuration := 10 * time.Minute
	if duration := os.Getenv("STABILITY_TEST_DURATION"); duration != "" {
		if d, err := time.ParseDuration(duration); err == nil {
			testDuration = d
		}
	}

	t.Logf("开始长期稳定性测试，持续时间: %v", testDuration)

	ctx, cancel := context.WithTimeout(context.Background(), testDuration)
	defer cancel()

	suite := NewComprehensivePerformanceTestSuite()
	err := suite.Setup()
	require.NoError(t, err)
	defer suite.Cleanup()

	// 启动持续负载
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			queryCount := 0
			for {
				select {
				case <-ctx.Done():
					return
				default:
					request := &core.QueryRequest{
						Query:     fmt.Sprintf("长期稳定性测试 worker_%d query_%d", workerID, queryCount),
						RequestID: fmt.Sprintf("stability_%d_%d", workerID, queryCount),
					}

					if suite.queryProcessor != nil {
						suite.queryProcessor.ProcessQuery(ctx, request)
					}

					queryCount++
					time.Sleep(100 * time.Millisecond)
				}
			}
		}(i)
	}

	wg.Wait()
	monitor.Stop()

	metrics := monitor.GetMetrics()
	monitor.PrintReport()

	// 验证长期稳定性
	require.False(t, metrics.MemoryLeakDetected, "长期运行不应该有内存泄漏")
	require.True(t, metrics.MemoryGrowthRate < 1.0, "内存增长率应该小于1MB/s: %.2f", metrics.MemoryGrowthRate)
	require.True(t, metrics.GoroutineGrowth < 100, "协程增长应该小于100: %d", metrics.GoroutineGrowth)

	// 验证 GC 效率
	if metrics.TotalGCCount > 0 {
		avgGCPause := metrics.TotalGCPause / time.Duration(metrics.TotalGCCount)
		require.True(t, avgGCPause < 10*time.Millisecond, "平均GC暂停时间过长: %v", avgGCPause)
	}
}
