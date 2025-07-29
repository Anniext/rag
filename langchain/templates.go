// 本文件实现了 LangChain 的提示模板系统，支持 SQL 生成、查询解释、错误分析、建议生成、查询优化等多种链式任务的提示模板管理与渲染。
// 主要功能：
// 1. 定义模板类型枚举，覆盖常见的链式任务场景。
// 2. 提示模板结构体包含模板元数据、内容、变量、默认值等，便于灵活扩展和管理。
// 3. 支持模板的渲染、变量替换、版本管理等功能。
// 4. 线程安全，支持并发访问和渲染。
// 5. 所有结构体字段和方法均有详细中文注释，便于理解和维护。

package langchain

import (
	"bytes"
	"context"
	"fmt"
	"github.com/Anniext/rag/core"
	"strings"
	"sync"
	"text/template"
	"time"

	"go.uber.org/zap"
)

type TemplateType string

// TemplateType 模板类型枚举，表示不同的提示模板用途
// 支持 SQL 生成、查询解释、错误分析、建议生成、查询优化等场景
// 可扩展更多类型
const (
	TemplateTypeSQLGeneration    TemplateType = "sql_generation"    // SQL 生成
	TemplateTypeQueryExplanation TemplateType = "query_explanation" // 查���解释
	TemplateTypeErrorAnalysis    TemplateType = "error_analysis"    // 错误分析
	TemplateTypeSuggestion       TemplateType = "suggestion"        // 查询建议
	TemplateTypeOptimization     TemplateType = "optimization"      // 查询优化
)

// PromptTemplate 提示模板结构体，包含模板的所有元数据和内容
// ID: 模板唯一标识
// Name: 模板名称
// Type: 模板类型
// Version: 模板版本
// Template: 模板内容
// Variables: 变量列表
// Description: 模板描述
// Examples: 示例列表
// Metadata: 额外元数据
// CreatedAt: 创建时间
// UpdatedAt: 更新时间
type PromptTemplate struct {
	ID          string                 `json:"id"`          // 模板唯一标识
	Name        string                 `json:"name"`        // 模板名称
	Type        TemplateType           `json:"type"`        // 模板类型
	Version     string                 `json:"version"`     // 模板版本
	Template    string                 `json:"template"`    // 模板内容
	Variables   []string               `json:"variables"`   // 变量列表
	Description string                 `json:"description"` // 模板描述
	Examples    []*TemplateExample     `json:"examples"`    // 示例列表
	Metadata    map[string]interface{} `json:"metadata"`    // 额外元数据
	CreatedAt   time.Time              `json:"created_at"`  // 创建时间
	UpdatedAt   time.Time              `json:"updated_at"`  // 更新时间

	compiled *template.Template // 编译后的模板对象
	mutex    sync.RWMutex       // 读写锁，保护并发访问
}

// TemplateExample 模板示例结构体，描述输入输出及说明
// Input: 示例输入变量
// Output: 示例输出结果
// Description: 示例说明
type TemplateExample struct {
	Input       map[string]interface{} `json:"input"`       // 示例输入变量
	Output      string                 `json:"output"`      // 示例输出结果
	Description string                 `json:"description"` // 示例说明
}

// TemplateManager 模板管理器接口，定义模板的增删查改及渲染等操作
// LoadTemplate: 加载指定类型和版本的模板
// RegisterTemplate: 注册新模板
// UpdateTemplate: 更新模板
// DeleteTemplate: 删除模板
// ListTemplates: 列出指定类型的所有模板
// RenderTemplate: 渲染模板
// GetLatestVersion: 获取最新版本号
// ValidateTemplate: 验证模板合法性
type TemplateManager interface {
	LoadTemplate(templateType TemplateType, version string) (*PromptTemplate, error)                            // 加载指定类型和版本的模板
	RegisterTemplate(template *PromptTemplate) error                                                            // 注册新模板
	UpdateTemplate(template *PromptTemplate) error                                                              // 更新模板
	DeleteTemplate(templateType TemplateType, version string) error                                             // 删除模板
	ListTemplates(templateType TemplateType) ([]*PromptTemplate, error)                                         // 列出指定类型的所有模板
	RenderTemplate(templateType TemplateType, version string, variables map[string]interface{}) (string, error) // 渲染模板
	GetLatestVersion(templateType TemplateType) (string, error)                                                 // 获取最新版本号
	ValidateTemplate(template *PromptTemplate) error                                                            // 验证模板合法性
}

// templateManagerImpl 模板管理器实现，负责模板的存储和操作
// templates: 模板存储，key为 type:version
// logger: 日志记录器
// mutex: 读写锁，保护并发访问
type templateManagerImpl struct {
	templates map[string]*PromptTemplate // 模板存储，key为 type:version
	logger    *zap.Logger                // 日志记录器
	mutex     sync.RWMutex               // 读写锁，保护并发访问
}

// NewTemplateManager 创建模板管理器实例，并加载默认模板
func NewTemplateManager(logger *zap.Logger) TemplateManager {
	manager := &templateManagerImpl{
		templates: make(map[string]*PromptTemplate),
		logger:    logger,
	}

	// 加载默认模板
	manager.loadDefaultTemplates()

	logger.Info("Template manager created")

	return manager
}

// loadDefaultTemplates 加载内置的默认模板到管理器
func (tm *templateManagerImpl) loadDefaultTemplates() {
	defaultTemplates := []*PromptTemplate{
		{
			ID:          "sql_gen_v1",
			Name:        "SQL Generation Template v1",
			Type:        TemplateTypeSQLGeneration,
			Version:     "1.0",
			Description: "基础 SQL 生成模板",
			Template: `你是一个专业的 SQL 查询生成助手。基于以下数据库 schema 和用户查询，生成准确的 SQL 语句。
	
	数据库 Schema:
	{{.Schema}}
	
	用户查询: {{.Query}}
	
	请生成对应的 SQL 语句，并确保：
	1. 语法正确
	2. 使用适当的表连接
	3. 包含必要的过滤条件
	4. 优化查询性能
	
	SQL:`,
			Variables: []string{"Schema", "Query"},
			Examples: []*TemplateExample{
				{
					Input: map[string]interface{}{
						"Schema": "CREATE TABLE users (id INT, name VARCHAR(100), email VARCHAR(100))",
						"Query":  "查找所有用户的姓名和邮箱",
					},
					Output:      "SELECT name, email FROM users;",
					Description: "简单的查询示例",
				},
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},

		{
			ID:          "query_explain_v1",
			Name:        "Query Explanation Template v1",
			Type:        TemplateTypeQueryExplanation,
			Version:     "1.0",
			Description: "查询解释模板",
			Template: `请解释以下 SQL 查询的含义和执行逻辑：
	
	SQL: {{.SQL}}
	
	{{if .Results}}查询结果: {{.Results}}{{end}}
	
	请用简洁易懂的中文解释这个查询做了什么，返回了什么数据。`,
			Variables: []string{"SQL", "Results"},
			Examples: []*TemplateExample{
				{
					Input: map[string]interface{}{
						"SQL": "SELECT name, email FROM users WHERE age > 18;",
					},
					Output:      "这个查询从用户表中选择年龄大于18岁的用户的姓名和邮箱信息。",
					Description: "查询解释示例",
				},
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},

		{
			ID:          "error_analysis_v1",
			Name:        "Error Analysis Template v1",
			Type:        TemplateTypeErrorAnalysis,
			Version:     "1.0",
			Description: "错误分析模板",
			Template: `分析以下 SQL 查询错误并提供修复建议：
	
	原始查询: {{.Query}}
	生成的 SQL: {{.SQL}}
	错误信息: {{.Error}}
	
	请分析错误原因并提供：
	1. 错误的具体原因
	2. 修复建议
	3. 正确的 SQL 语句（如果可能）
	
	分析结果:`,
			Variables: []string{"Query", "SQL", "Error"},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},

		{
			ID:          "suggestion_v1",
			Name:        "Query Suggestion Template v1",
			Type:        TemplateTypeSuggestion,
			Version:     "1.0",
			Description: "查询建议模板",
			Template: `基于用户的部分输入，提供查询建议：
	
	用户输入: {{.PartialQuery}}
	数据库 Schema: {{.Schema}}
	
	请提供 3-5 个可能的查询建议，格式如下：
	1. [建议的查询描述] - [对应的 SQL 语句]
	
	建议:`,
			Variables: []string{"PartialQuery", "Schema"},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},

		{
			ID:          "optimization_v1",
			Name:        "Query Optimization Template v1",
			Type:        TemplateTypeOptimization,
			Version:     "1.0",
			Description: "查询优化模板",
			Template: `分析以下 SQL 查询并提供性能优化建议：
	
	SQL 查询: {{.SQL}}
	执行计划: {{.ExecutionPlan}}
	性能指标: {{.Metrics}}
	
	请提供：
	1. 性能瓶颈分析
	2. 优化建议
	3. 优化后的 SQL（如果适用）
	4. 索引建议
	
	优化分析:`,
			Variables: []string{"SQL", "ExecutionPlan", "Metrics"},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	for _, tmpl := range defaultTemplates {
		if err := tm.RegisterTemplate(tmpl); err != nil {
			tm.logger.Error("Failed to register default template",
				zap.String("template_id", tmpl.ID),
				zap.Error(err),
			)
		}
	}

	tm.logger.Info("Default templates loaded", zap.Int("count", len(defaultTemplates)))
}

// RegisterTemplate 注册新模板到管理器，先校验再编译
func (tm *templateManagerImpl) RegisterTemplate(template *PromptTemplate) error {
	if err := tm.ValidateTemplate(template); err != nil {
		return fmt.Errorf("template validation failed: %w", err)
	}

	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	key := tm.getTemplateKey(template.Type, template.Version)

	// 编译模板内容
	compiled, err := tm.compileTemplate(template)
	if err != nil {
		return fmt.Errorf("failed to compile template: %w", err)
	}

	template.compiled = compiled
	template.UpdatedAt = time.Now()

	tm.templates[key] = template

	tm.logger.Info("Template registered",
		zap.String("type", string(template.Type)),
		zap.String("version", template.Version),
		zap.String("name", template.Name),
	)

	return nil
}

// LoadTemplate 加载指定类型和版本的模板，返回副本避免并发修改
func (tm *templateManagerImpl) LoadTemplate(templateType TemplateType, version string) (*PromptTemplate, error) {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()

	key := tm.getTemplateKey(templateType, version)
	template, exists := tm.templates[key]
	if !exists {
		return nil, fmt.Errorf("template not found: %s:%s", templateType, version)
	}

	// 返回副本
	return tm.copyTemplate(template), nil
}

// UpdateTemplate 更新已存在的模板，重新编译并刷新时间
func (tm *templateManagerImpl) UpdateTemplate(template *PromptTemplate) error {
	if err := tm.ValidateTemplate(template); err != nil {
		return fmt.Errorf("template validation failed: %w", err)
	}

	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	key := tm.getTemplateKey(template.Type, template.Version)

	if _, exists := tm.templates[key]; !exists {
		return fmt.Errorf("template not found: %s:%s", template.Type, template.Version)
	}

	// 编译模板内容
	compiled, err := tm.compileTemplate(template)
	if err != nil {
		return fmt.Errorf("failed to compile template: %w", err)
	}

	template.compiled = compiled
	template.UpdatedAt = time.Now()

	tm.templates[key] = template

	tm.logger.Info("Template updated",
		zap.String("type", string(template.Type)),
		zap.String("version", template.Version),
	)

	return nil
}

// DeleteTemplate 删除指定类型和版本的模板
func (tm *templateManagerImpl) DeleteTemplate(templateType TemplateType, version string) error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	key := tm.getTemplateKey(templateType, version)

	if _, exists := tm.templates[key]; !exists {
		return fmt.Errorf("template not found: %s:%s", templateType, version)
	}

	delete(tm.templates, key)

	tm.logger.Info("Template deleted",
		zap.String("type", string(templateType)),
		zap.String("version", version),
	)

	return nil
}

// ListTemplates 列出所有指定类型的模板（如类型为空则全部返回）
func (tm *templateManagerImpl) ListTemplates(templateType TemplateType) ([]*PromptTemplate, error) {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()

	var templates []*PromptTemplate

	for _, template := range tm.templates {
		if templateType == "" || template.Type == templateType {
			templates = append(templates, tm.copyTemplate(template))
		}
	}

	return templates, nil
}

// RenderTemplate 渲染指定类型和版本的模板，变量由调用方传入
func (tm *templateManagerImpl) RenderTemplate(templateType TemplateType, version string, variables map[string]interface{}) (string, error) {
	template, err := tm.LoadTemplate(templateType, version)
	if err != nil {
		return "", err
	}

	return tm.renderTemplate(template, variables)
}

// GetLatestVersion 获取指定类型模板的最新版本（按更新时间排序）
func (tm *templateManagerImpl) GetLatestVersion(templateType TemplateType) (string, error) {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()

	var latestVersion string
	var latestTime time.Time

	for _, template := range tm.templates {
		if template.Type == templateType {
			if template.UpdatedAt.After(latestTime) {
				latestVersion = template.Version
				latestTime = template.UpdatedAt
			}
		}
	}

	if latestVersion == "" {
		return "", fmt.Errorf("no templates found for type: %s", templateType)
	}

	return latestVersion, nil
}

// ValidateTemplate 校验模板合法性，包括字段和语法
func (tm *templateManagerImpl) ValidateTemplate(template *PromptTemplate) error {
	if template == nil {
		return fmt.Errorf("template cannot be nil")
	}

	if template.ID == "" {
		return fmt.Errorf("template ID is required")
	}

	if template.Name == "" {
		return fmt.Errorf("template name is required")
	}

	if template.Type == "" {
		return fmt.Errorf("template type is required")
	}

	if template.Version == "" {
		return fmt.Errorf("template version is required")
	}

	if template.Template == "" {
		return fmt.Errorf("template content is required")
	}

	// 验证模板语法
	_, err := tm.compileTemplate(template)
	if err != nil {
		return fmt.Errorf("template syntax error: %w", err)
	}

	return nil
}

// compileTemplate 编译模板内容为 template.Template 对象
func (tm *templateManagerImpl) compileTemplate(tmpl *PromptTemplate) (*template.Template, error) {
	compiled, err := template.New(tmpl.ID).Parse(tmpl.Template)
	if err != nil {
		return nil, err
	}

	return compiled, nil
}

// renderTemplate 渲染模板内容，变量由调用方传入
func (tm *templateManagerImpl) renderTemplate(template *PromptTemplate, variables map[string]interface{}) (string, error) {
	template.mutex.RLock()
	defer template.mutex.RUnlock()

	if template.compiled == nil {
		compiled, err := tm.compileTemplate(template)
		if err != nil {
			return "", fmt.Errorf("failed to compile template: %w", err)
		}
		template.compiled = compiled
	}

	var buf bytes.Buffer
	if err := template.compiled.Execute(&buf, variables); err != nil {
		return "", fmt.Errorf("failed to render template: %w", err)
	}

	return buf.String(), nil
}

// getTemplateKey 生成模板存储的唯一键（类型:版本）
func (tm *templateManagerImpl) getTemplateKey(templateType TemplateType, version string) string {
	return fmt.Sprintf("%s:%s", templateType, version)
}

// copyTemplate 复制模板对象，避免并发修改原始数据
func (tm *templateManagerImpl) copyTemplate(template *PromptTemplate) *PromptTemplate {
	copyTemplate := &PromptTemplate{
		ID:          template.ID,
		Name:        template.Name,
		Type:        template.Type,
		Version:     template.Version,
		Template:    template.Template,
		Variables:   make([]string, len(template.Variables)),
		Description: template.Description,
		Examples:    make([]*TemplateExample, len(template.Examples)),
		Metadata:    make(map[string]interface{}),
		CreatedAt:   template.CreatedAt,
		UpdatedAt:   template.UpdatedAt,
	}

	copy(copyTemplate.Variables, template.Variables)
	copy(copyTemplate.Examples, template.Examples)

	for k, v := range template.Metadata {
		copyTemplate.Metadata[k] = v
	}

	return copyTemplate
}

// TemplateBuilder 模板构建器，链式设置模板各字段
// template: 待构建的模板对象
type TemplateBuilder struct {
	template *PromptTemplate
}

// NewTemplateBuilder 创建模板构建器实例
func NewTemplateBuilder() *TemplateBuilder {
	return &TemplateBuilder{
		template: &PromptTemplate{
			Variables: make([]string, 0),
			Examples:  make([]*TemplateExample, 0),
			Metadata:  make(map[string]interface{}),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}
}

// WithID 设置模板ID
func (b *TemplateBuilder) WithID(id string) *TemplateBuilder {
	b.template.ID = id
	return b
}

// WithName 设置模板名称
func (b *TemplateBuilder) WithName(name string) *TemplateBuilder {
	b.template.Name = name
	return b
}

// WithType 设置模板类型
func (b *TemplateBuilder) WithType(templateType TemplateType) *TemplateBuilder {
	b.template.Type = templateType
	return b
}

// WithVersion 设置模板版本
func (b *TemplateBuilder) WithVersion(version string) *TemplateBuilder {
	b.template.Version = version
	return b
}

// WithTemplate 设置模板内容
func (b *TemplateBuilder) WithTemplate(template string) *TemplateBuilder {
	b.template.Template = template
	return b
}

// WithVariables 设置模板变量
func (b *TemplateBuilder) WithVariables(variables ...string) *TemplateBuilder {
	b.template.Variables = variables
	return b
}

// WithDescription 设置模板描述
func (b *TemplateBuilder) WithDescription(description string) *TemplateBuilder {
	b.template.Description = description
	return b
}

// WithExample 添���模板示例
func (b *TemplateBuilder) WithExample(input map[string]interface{}, output, description string) *TemplateBuilder {
	example := &TemplateExample{
		Input:       input,
		Output:      output,
		Description: description,
	}
	b.template.Examples = append(b.template.Examples, example)
	return b
}

// WithMetadata 设置模板元数据
func (b *TemplateBuilder) WithMetadata(key string, value interface{}) *TemplateBuilder {
	b.template.Metadata[key] = value
	return b
}

// Build 构建并返回模板对象
func (b *TemplateBuilder) Build() *PromptTemplate {
	return b.template
}

// TemplateRenderer 用于根据指定模板类型和变量渲染提示内容的渲染器。
// 该结构体封装了模板管理器和日志记录器，负责调用模板管理器获取模板、渲染模板内容，并记录相关日志。
// manager 字段用于操作和管理模板（如获取最新模板、渲染模板等）。
// logger 字段用于记录渲染过程中的信息、警告或错误，便于排查和监控。
type TemplateRenderer struct {
	manager TemplateManager // 模板管理器接口，负责模板的加载、渲染等操作
	logger  *zap.Logger     // 日志记录器，用于记录渲染相关日志
}

// NewTemplateRenderer 创建模板渲染器实例
func NewTemplateRenderer(manager TemplateManager, logger *zap.Logger) *TemplateRenderer {
	return &TemplateRenderer{
		manager: manager,
		logger:  logger,
	}
}

// RenderSQLGeneration 渲染 SQL 生成模板，传入查询和 schema
func (r *TemplateRenderer) RenderSQLGeneration(ctx context.Context, query string, schema *core.SchemaInfo) (string, error) {
	variables := map[string]interface{}{
		"Query":  query,
		"Schema": r.formatSchema(schema),
	}

	version, err := r.manager.GetLatestVersion(TemplateTypeSQLGeneration)
	if err != nil {
		return "", fmt.Errorf("failed to get latest SQL generation template: %w", err)
	}

	return r.manager.RenderTemplate(TemplateTypeSQLGeneration, version, variables)
}

// RenderQueryExplanation 渲染查询解释模板，传入 SQL 和结果
func (r *TemplateRenderer) RenderQueryExplanation(ctx context.Context, sql string, results interface{}) (string, error) {
	variables := map[string]interface{}{
		"SQL":     sql,
		"Results": r.formatResults(results),
	}

	version, err := r.manager.GetLatestVersion(TemplateTypeQueryExplanation)
	if err != nil {
		return "", fmt.Errorf("failed to get latest query explanation template: %w", err)
	}

	return r.manager.RenderTemplate(TemplateTypeQueryExplanation, version, variables)
}

// RenderErrorAnalysis 渲染错误分析模板，传入原始查询、SQL和错误信息
func (r *TemplateRenderer) RenderErrorAnalysis(ctx context.Context, query, sql, errorMsg string) (string, error) {
	variables := map[string]interface{}{
		"Query": query,
		"SQL":   sql,
		"Error": errorMsg,
	}

	version, err := r.manager.GetLatestVersion(TemplateTypeErrorAnalysis)
	if err != nil {
		return "", fmt.Errorf("failed to get latest error analysis template: %w", err)
	}

	return r.manager.RenderTemplate(TemplateTypeErrorAnalysis, version, variables)
}

// RenderSuggestion 渲染查询建议模板，传入部分查询和 schema
func (r *TemplateRenderer) RenderSuggestion(ctx context.Context, partialQuery string, schema *core.SchemaInfo) (string, error) {
	variables := map[string]interface{}{
		"PartialQuery": partialQuery,
		"Schema":       r.formatSchema(schema),
	}

	version, err := r.manager.GetLatestVersion(TemplateTypeSuggestion)
	if err != nil {
		return "", fmt.Errorf("failed to get latest suggestion template: %w", err)
	}

	return r.manager.RenderTemplate(TemplateTypeSuggestion, version, variables)
}

// formatSchema 格式化 Schema 信息为字符串，便于模板渲染
func (r *TemplateRenderer) formatSchema(schema *core.SchemaInfo) string {
	if schema == nil {
		return "No schema information available"
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("Database: %s\n\n", schema.Database))

	for _, table := range schema.Tables {
		builder.WriteString(fmt.Sprintf("Table: %s", table.Name))
		if table.Comment != "" {
			builder.WriteString(fmt.Sprintf(" -- %s", table.Comment))
		}
		builder.WriteString("\n")

		for _, column := range table.Columns {
			builder.WriteString(fmt.Sprintf("  %s %s", column.Name, column.Type))
			if !column.Nullable {
				builder.WriteString(" NOT NULL")
			}
			if column.IsPrimaryKey {
				builder.WriteString(" PRIMARY KEY")
			}
			if column.Comment != "" {
				builder.WriteString(fmt.Sprintf(" -- %s", column.Comment))
			}
			builder.WriteString("\n")
		}
		builder.WriteString("\n")
	}

	return builder.String()
}

// formatResults 格式化查询结果为字符串，便于模板渲染
func (r *TemplateRenderer) formatResults(results interface{}) string {
	if results == nil {
		return "No results"
	}

	// 这里可以根据实际需要实现更复杂的格式化逻辑
	return fmt.Sprintf("%v", results)
}

// MockTemplateManager 模拟模板管理器（用于测试），实现 TemplateManager 接口
type MockTemplateManager struct {
	templates map[string]*PromptTemplate // 模板存储
	mutex     sync.RWMutex               // 读写锁
}

// NewMockTemplateManager 创建模拟模板管理器实例
func NewMockTemplateManager() *MockTemplateManager {
	return &MockTemplateManager{
		templates: make(map[string]*PromptTemplate),
	}
}

// LoadTemplate 加载指定类型和版本的模板
func (m *MockTemplateManager) LoadTemplate(templateType TemplateType, version string) (*PromptTemplate, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	key := fmt.Sprintf("%s:%s", templateType, version)
	template, exists := m.templates[key]
	if !exists {
		return nil, fmt.Errorf("template not found: %s", key)
	}

	return template, nil
}

// RegisterTemplate 注册新模板
func (m *MockTemplateManager) RegisterTemplate(template *PromptTemplate) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	key := fmt.Sprintf("%s:%s", template.Type, template.Version)
	m.templates[key] = template
	return nil
}

// UpdateTemplate 更新模板（直接调用注册方法）
func (m *MockTemplateManager) UpdateTemplate(template *PromptTemplate) error {
	return m.RegisterTemplate(template)
}

// DeleteTemplate 删除指定类型和版本的模板
func (m *MockTemplateManager) DeleteTemplate(templateType TemplateType, version string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	key := fmt.Sprintf("%s:%s", templateType, version)
	delete(m.templates, key)
	return nil
}

// ListTemplates 列出所有指定类型的模板
func (m *MockTemplateManager) ListTemplates(templateType TemplateType) ([]*PromptTemplate, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var templates []*PromptTemplate
	for _, template := range m.templates {
		if templateType == "" || template.Type == templateType {
			templates = append(templates, template)
		}
	}

	return templates, nil
}

// RenderTemplate 渲染模板（模拟实现，返回模板名和变量）
func (m *MockTemplateManager) RenderTemplate(templateType TemplateType, version string, variables map[string]interface{}) (string, error) {
	template, err := m.LoadTemplate(templateType, version)
	if err != nil {
		return "", err
	}

	// 简单的模拟渲染
	return fmt.Sprintf("Rendered template: %s with variables: %v", template.Name, variables), nil
}

// GetLatestVersion 获取指定类型模板的最新版本（模拟实现，返回第一个找到的版本）
func (m *MockTemplateManager) GetLatestVersion(templateType TemplateType) (string, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	for _, template := range m.templates {
		if template.Type == templateType {
			return template.Version, nil
		}
	}

	return "", fmt.Errorf("no templates found for type: %s", templateType)
}

// ValidateTemplate 校验模板（模拟实现，仅判空）
func (m *MockTemplateManager) ValidateTemplate(template *PromptTemplate) error {
	if template == nil {
		return fmt.Errorf("template cannot be nil")
	}
	return nil
}
