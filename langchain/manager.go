// 本文件实现了 LangChain 管理器，负责链的创建、缓存、内存管理、执行等核心功能。
// 主要功能：
// 1. 管理 LLM 客户端、模板管理器、模板渲染器等核心组件。
// 2. 支持链的创建、缓存、并发安全访问。
// 3. 支持链式内存的管理与并发安全。
// 4. 提供链的执行、生命周期管理等接口。
// 5. 所有结构体字段和方法均有详细中文注释，便于理解和维护。

package langchain

import (
	"context" // 上下文管理
	"fmt"     // 格式化输出
	"github.com/Anniext/rag/core"
	"sync" // 并发锁
	"time" // 时间处理

	"go.uber.org/zap" // 日志库
)

// langChainManagerImpl LangChain 管理器实现
// 负责管理 LLM 客户端、模板管理器、链缓存、内存管理等
// 支持链的创建、执行、缓存和并发安全访问
// 提供链式内存的管理与并发安全
// 所有字段均有详细说明
// llmClient: 大语言模型客户端
// templateManager: 模板管理器
// templateRenderer: 模板渲染器
// chains: 链缓存，按链类型存储
// memories: 链式内存缓存
// mutex/memoryMutex: 并发安全锁
// logger: 日志记录器
type langChainManagerImpl struct {
	llmClient        LLMClient
	templateManager  TemplateManager
	templateRenderer *TemplateRenderer
	logger           *zap.Logger

	// Chain 缓存
	chains map[string]core.Chain
	mutex  sync.RWMutex

	// 内存管理
	memories    map[string]core.Memory
	memoryMutex sync.RWMutex
}

// NewLangChainManager 创建 LangChain 管理器
func NewLangChainManager(llmClient LLMClient, templateManager TemplateManager, logger *zap.Logger) core.LangChainManager {
	templateRenderer := NewTemplateRenderer(templateManager, logger)

	manager := &langChainManagerImpl{
		llmClient:        llmClient,
		templateManager:  templateManager,
		templateRenderer: templateRenderer,
		logger:           logger,
		chains:           make(map[string]core.Chain),
		memories:         make(map[string]core.Memory),
	}

	logger.Info("LangChain manager created")

	return manager
}

// CreateChain 创建 Chain
func (m *langChainManagerImpl) CreateChain(ctx context.Context, config *core.ChainConfig) (core.Chain, error) {
	if config == nil {
		return nil, fmt.Errorf("chain config is required")
	}

	if config.Type == "" {
		return nil, fmt.Errorf("chain type is required")
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 检查是否已存在相同配置的 Chain
	chainKey := m.getChainKey(config)
	if existingChain, exists := m.chains[chainKey]; exists {
		m.logger.Debug("Reusing existing chain", zap.String("type", config.Type))
		return existingChain, nil
	}

	// 创建新的 Chain
	var chain core.Chain
	var err error

	switch config.Type {
	case "sql_generation":
		chain, err = m.createSQLGenerationChain(config)
	case "query_explanation":
		chain, err = m.createQueryExplanationChain(config)
	case "error_analysis":
		chain, err = m.createErrorAnalysisChain(config)
	case "suggestion":
		chain, err = m.createSuggestionChain(config)
	case "optimization":
		chain, err = m.createOptimizationChain(config)
	default:
		return nil, fmt.Errorf("unsupported chain type: %s", config.Type)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create chain: %w", err)
	}

	// 缓存 Chain
	m.chains[chainKey] = chain

	m.logger.Info("Chain created",
		zap.String("type", config.Type),
		zap.String("key", chainKey),
	)

	return chain, nil
}

// ExecuteChain 执行 Chain
func (m *langChainManagerImpl) ExecuteChain(ctx context.Context, chain core.Chain, input *core.ChainInput) (*core.ChainOutput, error) {
	if chain == nil {
		return nil, fmt.Errorf("chain is required")
	}

	if input == nil {
		return nil, fmt.Errorf("chain input is required")
	}

	startTime := time.Now()

	// 执行 Chain
	output, err := chain.Run(ctx, input)
	if err != nil {
		m.logger.Error("Chain execution failed",
			zap.Error(err),
			zap.Duration("duration", time.Since(startTime)),
		)
		return nil, fmt.Errorf("chain execution failed: %w", err)
	}

	executionTime := time.Since(startTime)

	// 添加执行元数据
	if output.Metadata == nil {
		output.Metadata = make(map[string]any)
	}
	output.Metadata["execution_time"] = executionTime
	output.Metadata["executed_at"] = time.Now()

	m.logger.Debug("Chain executed successfully",
		zap.Duration("execution_time", executionTime),
	)

	return output, nil
}

// GetMemory 获取内存
func (m *langChainManagerImpl) GetMemory(sessionID string) (core.Memory, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("session ID is required")
	}

	m.memoryMutex.RLock()
	memory, exists := m.memories[sessionID]
	m.memoryMutex.RUnlock()

	if !exists {
		// 创建新的内存
		memory = NewSessionMemory(sessionID)

		m.memoryMutex.Lock()
		m.memories[sessionID] = memory
		m.memoryMutex.Unlock()

		m.logger.Debug("Created new memory for session", zap.String("session_id", sessionID))
	}

	return memory, nil
}

// UpdateMemory 更新内存
func (m *langChainManagerImpl) UpdateMemory(sessionID string, memory core.Memory) error {
	if sessionID == "" {
		return fmt.Errorf("session ID is required")
	}

	if memory == nil {
		return fmt.Errorf("memory is required")
	}

	m.memoryMutex.Lock()
	defer m.memoryMutex.Unlock()

	m.memories[sessionID] = memory

	m.logger.Debug("Memory updated for session", zap.String("session_id", sessionID))

	return nil
}

// createSQLGenerationChain 创建 SQL 生成 Chain
func (m *langChainManagerImpl) createSQLGenerationChain(config *core.ChainConfig) (core.Chain, error) {
	chain := &sqlGenerationChain{
		baseChain: baseChain{
			chainType:        "sql_generation",
			llmClient:        m.llmClient,
			templateRenderer: m.templateRenderer,
			logger:           m.logger,
			config:           config,
			tools:            make([]core.Tool, 0),
		},
	}

	// 添加默认工具
	if err := chain.AddTool(NewSQLGeneratorTool(m.llmClient, m.templateRenderer, m.logger)); err != nil {
		return nil, fmt.Errorf("failed to add SQL generator tool: %w", err)
	}

	return chain, nil
}

// createQueryExplanationChain 创建查询解释 Chain
func (m *langChainManagerImpl) createQueryExplanationChain(config *core.ChainConfig) (core.Chain, error) {
	chain := &queryExplanationChain{
		baseChain: baseChain{
			chainType:        "query_explanation",
			llmClient:        m.llmClient,
			templateRenderer: m.templateRenderer,
			logger:           m.logger,
			config:           config,
			tools:            make([]core.Tool, 0),
		},
	}

	// 添加默认���具
	if err := chain.AddTool(NewQueryExplanationTool(m.llmClient, m.templateRenderer, m.logger)); err != nil {
		return nil, fmt.Errorf("failed to add query explanation tool: %w", err)
	}

	return chain, nil
}

// createErrorAnalysisChain 创建错误分析 Chain
func (m *langChainManagerImpl) createErrorAnalysisChain(config *core.ChainConfig) (core.Chain, error) {
	chain := &errorAnalysisChain{
		baseChain: baseChain{
			chainType:        "error_analysis",
			llmClient:        m.llmClient,
			templateRenderer: m.templateRenderer,
			logger:           m.logger,
			config:           config,
			tools:            make([]core.Tool, 0),
		},
	}

	// 添加默认工具
	if err := chain.AddTool(NewErrorAnalysisTool(m.llmClient, m.templateRenderer, m.logger)); err != nil {
		return nil, fmt.Errorf("failed to add error analysis tool: %w", err)
	}

	return chain, nil
}

// createSuggestionChain 创建建议 Chain
func (m *langChainManagerImpl) createSuggestionChain(config *core.ChainConfig) (core.Chain, error) {
	chain := &suggestionChain{
		baseChain: baseChain{
			chainType:        "suggestion",
			llmClient:        m.llmClient,
			templateRenderer: m.templateRenderer,
			logger:           m.logger,
			config:           config,
			tools:            make([]core.Tool, 0),
		},
	}

	// 添加默认工具
	if err := chain.AddTool(NewSuggestionTool(m.llmClient, m.templateRenderer, m.logger)); err != nil {
		return nil, fmt.Errorf("failed to add suggestion tool: %w", err)
	}

	return chain, nil
}

// createOptimizationChain 创建优化 Chain
func (m *langChainManagerImpl) createOptimizationChain(config *core.ChainConfig) (core.Chain, error) {
	chain := &optimizationChain{
		baseChain: baseChain{
			chainType:        "optimization",
			llmClient:        m.llmClient,
			templateRenderer: m.templateRenderer,
			logger:           m.logger,
			config:           config,
			tools:            make([]core.Tool, 0),
		},
	}

	// 添加默认工具
	if err := chain.AddTool(NewOptimizationTool(m.llmClient, m.templateRenderer, m.logger)); err != nil {
		return nil, fmt.Errorf("failed to add optimization tool: %w", err)
	}

	return chain, nil
}

// getChainKey 获取 Chain 键
func (m *langChainManagerImpl) getChainKey(config *core.ChainConfig) string {
	return fmt.Sprintf("%s:%s", config.Type, config.LLMConfig.Model)
}

// SessionMemory 会话内存实现
// 负责管理单个会话的历史记录和上下文信息
// 支持并发安全访问和修改
// 提供会话内存的创建、清空、过期检查等功能
// 所有字段均有详细说明
// sessionID: 会话唯一标识
// history: 查询历史记录
// context: 上下文信息
// mutex: 并发安全锁
// createdAt: 创建时间
// lastAccessed: 最后访问时间
type SessionMemory struct {
	sessionID    string
	history      []*core.QueryHistory
	context      map[string]interface{}
	mutex        sync.RWMutex
	createdAt    time.Time
	lastAccessed time.Time
}

// NewSessionMemory 创建会话内存
func NewSessionMemory(sessionID string) core.Memory {
	return &SessionMemory{
		sessionID:    sessionID,
		history:      make([]*core.QueryHistory, 0),
		context:      make(map[string]interface{}),
		createdAt:    time.Now(),
		lastAccessed: time.Now(),
	}
}

// GetHistory 获取历史记录
func (m *SessionMemory) GetHistory() []*core.QueryHistory {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	m.lastAccessed = time.Now()

	// 返回副本
	history := make([]*core.QueryHistory, len(m.history))
	copy(history, m.history)

	return history
}

// AddHistory 添加历史记录
func (m *SessionMemory) AddHistory(history *core.QueryHistory) {
	if history == nil {
		return
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.history = append(m.history, history)
	m.lastAccessed = time.Now()

	// 限制历史记录数量
	maxHistory := 100
	if len(m.history) > maxHistory {
		m.history = m.history[len(m.history)-maxHistory:]
	}
}

// GetContext 获取上下文
func (m *SessionMemory) GetContext() map[string]interface{} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	m.lastAccessed = time.Now()

	// 返回副本
	context := make(map[string]interface{})
	for k, v := range m.context {
		context[k] = v
	}

	return context
}

// SetContext 设置上下文
func (m *SessionMemory) SetContext(key string, value interface{}) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.context[key] = value
	m.lastAccessed = time.Now()
}

// Clear 清空内存
func (m *SessionMemory) Clear() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.history = make([]*core.QueryHistory, 0)
	m.context = make(map[string]interface{})
	m.lastAccessed = time.Now()
}

// GetSessionID 获取会话 ID
func (m *SessionMemory) GetSessionID() string {
	return m.sessionID
}

// GetCreatedAt 获取创建时间
func (m *SessionMemory) GetCreatedAt() time.Time {
	return m.createdAt
}

// GetLastAccessed 获取最后访问时间
func (m *SessionMemory) GetLastAccessed() time.Time {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return m.lastAccessed
}

// IsExpired 检查是否过期
func (m *SessionMemory) IsExpired(ttl time.Duration) bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return time.Since(m.lastAccessed) > ttl
}

// GetStats 获取统计信息
func (m *SessionMemory) GetStats() map[string]interface{} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return map[string]interface{}{
		"session_id":    m.sessionID,
		"history_count": len(m.history),
		"context_keys":  len(m.context),
		"created_at":    m.createdAt,
		"last_accessed": m.lastAccessed,
		"age":           time.Since(m.createdAt),
		"idle_time":     time.Since(m.lastAccessed),
	}
}

// MockLangChainManager 模拟 LangChain 管理器（用于测试）
type MockLangChainManager struct {
	chains   map[string]core.Chain
	memories map[string]core.Memory
	mutex    sync.RWMutex
}

// NewMockLangChainManager 创建模拟 LangChain 管理器
func NewMockLangChainManager() *MockLangChainManager {
	return &MockLangChainManager{
		chains:   make(map[string]core.Chain),
		memories: make(map[string]core.Memory),
	}
}

// CreateChain 创建 Chain
func (m *MockLangChainManager) CreateChain(ctx context.Context, config *core.ChainConfig) (core.Chain, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	key := fmt.Sprintf("%s:%s", config.Type, config.LLMConfig.Model)

	chain := NewMockChain(config.Type)
	m.chains[key] = chain

	return chain, nil
}

// ExecuteChain 执行 Chain
func (m *MockLangChainManager) ExecuteChain(ctx context.Context, chain core.Chain, input *core.ChainInput) (*core.ChainOutput, error) {
	return chain.Run(ctx, input)
}

// GetMemory 获取内存
func (m *MockLangChainManager) GetMemory(sessionID string) (core.Memory, error) {
	m.mutex.RLock()
	memory, exists := m.memories[sessionID]
	m.mutex.RUnlock()

	if !exists {
		memory = NewSessionMemory(sessionID)

		m.mutex.Lock()
		m.memories[sessionID] = memory
		m.mutex.Unlock()
	}

	return memory, nil
}

// UpdateMemory 更新内存
func (m *MockLangChainManager) UpdateMemory(sessionID string, memory core.Memory) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.memories[sessionID] = memory
	return nil
}
