package langchain

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	"pumppill/rag/core"
)

func TestTemplateManager(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	manager := NewTemplateManager(logger)

	// 测试列出默认模板
	templates, err := manager.ListTemplates("")
	if err != nil {
		t.Fatalf("Failed to list templates: %v", err)
	}

	if len(templates) == 0 {
		t.Fatal("Expected default templates to be loaded")
	}

	// 测试加载特定模板
	template, err := manager.LoadTemplate(TemplateTypeSQLGeneration, "1.0")
	if err != nil {
		t.Fatalf("Failed to load SQL generation template: %v", err)
	}

	if template.Type != TemplateTypeSQLGeneration {
		t.Errorf("Expected type %s, got %s", TemplateTypeSQLGeneration, template.Type)
	}

	// 测试渲染模板
	variables := map[string]interface{}{
		"Schema": "CREATE TABLE users (id INT, name VARCHAR(100))",
		"Query":  "查找所有用户",
	}

	rendered, err := manager.RenderTemplate(TemplateTypeSQLGeneration, "1.0", variables)
	if err != nil {
		t.Fatalf("Failed to render template: %v", err)
	}

	if rendered == "" {
		t.Fatal("Rendered template should not be empty")
	}

	// 验证渲染结果包含变量内容
	if !contains(rendered, "CREATE TABLE users") {
		t.Error("Rendered template should contain schema information")
	}

	if !contains(rendered, "查找所有用户") {
		t.Error("Rendered template should contain query information")
	}
}

func TestTemplateBuilder(t *testing.T) {
	template := NewTemplateBuilder().
		WithID("test_template").
		WithName("Test Template").
		WithType(TemplateTypeSQLGeneration).
		WithVersion("1.0").
		WithTemplate("Test template content with {{.Variable}}").
		WithVariables("Variable").
		WithDescription("Test template description").
		WithExample(
			map[string]interface{}{"Variable": "test"},
			"Test template content with test",
			"Test example",
		).
		WithMetadata("author", "test").
		Build()

	if template.ID != "test_template" {
		t.Errorf("Expected ID 'test_template', got '%s'", template.ID)
	}

	if template.Name != "Test Template" {
		t.Errorf("Expected name 'Test Template', got '%s'", template.Name)
	}

	if template.Type != TemplateTypeSQLGeneration {
		t.Errorf("Expected type %s, got %s", TemplateTypeSQLGeneration, template.Type)
	}

	if len(template.Variables) != 1 || template.Variables[0] != "Variable" {
		t.Errorf("Expected variables ['Variable'], got %v", template.Variables)
	}

	if len(template.Examples) != 1 {
		t.Errorf("Expected 1 example, got %d", len(template.Examples))
	}

	if template.Metadata["author"] != "test" {
		t.Errorf("Expected metadata author 'test', got '%v'", template.Metadata["author"])
	}
}

func TestTemplateRegistration(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	manager := NewTemplateManager(logger)

	// 创建自定义模板
	customTemplate := NewTemplateBuilder().
		WithID("custom_test").
		WithName("Custom Test Template").
		WithType(TemplateTypeSQLGeneration).
		WithVersion("2.0").
		WithTemplate("Custom template: {{.CustomVar}}").
		WithVariables("CustomVar").
		Build()

	// 注册模板
	err := manager.RegisterTemplate(customTemplate)
	if err != nil {
		t.Fatalf("Failed to register template: %v", err)
	}

	// 加载注册的模板
	loaded, err := manager.LoadTemplate(TemplateTypeSQLGeneration, "2.0")
	if err != nil {
		t.Fatalf("Failed to load registered template: %v", err)
	}

	if loaded.ID != "custom_test" {
		t.Errorf("Expected ID 'custom_test', got '%s'", loaded.ID)
	}

	// 测试渲染自定义模板
	rendered, err := manager.RenderTemplate(TemplateTypeSQLGeneration, "2.0", map[string]interface{}{
		"CustomVar": "test value",
	})
	if err != nil {
		t.Fatalf("Failed to render custom template: %v", err)
	}

	expected := "Custom template: test value"
	if rendered != expected {
		t.Errorf("Expected '%s', got '%s'", expected, rendered)
	}
}

func TestTemplateUpdate(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	manager := NewTemplateManager(logger)

	// 创建模板
	template := NewTemplateBuilder().
		WithID("update_test").
		WithName("Update Test").
		WithType(TemplateTypeSQLGeneration).
		WithVersion("1.0").
		WithTemplate("Original: {{.Var}}").
		WithVariables("Var").
		Build()

	// 注册模板
	err := manager.RegisterTemplate(template)
	if err != nil {
		t.Fatalf("Failed to register template: %v", err)
	}

	// 更新模板
	template.Template = "Updated: {{.Var}}"
	template.Name = "Updated Test"

	err = manager.UpdateTemplate(template)
	if err != nil {
		t.Fatalf("Failed to update template: %v", err)
	}

	// 验证更新
	loaded, err := manager.LoadTemplate(TemplateTypeSQLGeneration, "1.0")
	if err != nil {
		t.Fatalf("Failed to load updated template: %v", err)
	}

	if loaded.Name != "Updated Test" {
		t.Errorf("Expected name 'Updated Test', got '%s'", loaded.Name)
	}

	// 测试渲染更新后的模板
	rendered, err := manager.RenderTemplate(TemplateTypeSQLGeneration, "1.0", map[string]interface{}{
		"Var": "test",
	})
	if err != nil {
		t.Fatalf("Failed to render updated template: %v", err)
	}

	expected := "Updated: test"
	if rendered != expected {
		t.Errorf("Expected '%s', got '%s'", expected, rendered)
	}
}

func TestTemplateDelete(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	manager := NewTemplateManager(logger)

	// 创建并注册模板
	template := NewTemplateBuilder().
		WithID("delete_test").
		WithName("Delete Test").
		WithType(TemplateTypeSQLGeneration).
		WithVersion("1.0").
		WithTemplate("Delete test: {{.Var}}").
		WithVariables("Var").
		Build()

	err := manager.RegisterTemplate(template)
	if err != nil {
		t.Fatalf("Failed to register template: %v", err)
	}

	// 验证模板存在
	_, err = manager.LoadTemplate(TemplateTypeSQLGeneration, "1.0")
	if err != nil {
		t.Fatalf("Template should exist before deletion: %v", err)
	}

	// 删除模板
	err = manager.DeleteTemplate(TemplateTypeSQLGeneration, "1.0")
	if err != nil {
		t.Fatalf("Failed to delete template: %v", err)
	}

	// 验证模板已删除
	_, err = manager.LoadTemplate(TemplateTypeSQLGeneration, "1.0")
	if err == nil {
		t.Fatal("Template should not exist after deletion")
	}
}

func TestTemplateValidation(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	manager := NewTemplateManager(logger)

	// 测试无效模板
	invalidTemplates := []*PromptTemplate{
		nil,          // nil 模板
		{},           // 空模板
		{ID: "test"}, // 缺少必要字段
		{
			ID:       "test",
			Name:     "Test",
			Type:     TemplateTypeSQLGeneration,
			Version:  "1.0",
			Template: "Invalid template {{.UnclosedVar", // 语法错误
		},
	}

	for i, template := range invalidTemplates {
		err := manager.ValidateTemplate(template)
		if err == nil {
			t.Errorf("Template %d should be invalid", i)
		}
	}

	// 测试有效模板
	validTemplate := NewTemplateBuilder().
		WithID("valid_test").
		WithName("Valid Test").
		WithType(TemplateTypeSQLGeneration).
		WithVersion("1.0").
		WithTemplate("Valid template: {{.Var}}").
		WithVariables("Var").
		Build()

	err := manager.ValidateTemplate(validTemplate)
	if err != nil {
		t.Errorf("Valid template should pass validation: %v", err)
	}
}

func TestTemplateRenderer(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	manager := NewTemplateManager(logger)
	renderer := NewTemplateRenderer(manager, logger)

	ctx := context.Background()

	// 测试 SQL 生成渲染
	schema := &core.SchemaInfo{
		Database: "test_db",
		Tables: []*core.TableInfo{
			{
				Name:    "users",
				Comment: "用户表",
				Columns: []*core.Column{
					{
						Name:    "id",
						Type:    "INT",
						Comment: "用户ID",
					},
					{
						Name:    "name",
						Type:    "VARCHAR(100)",
						Comment: "用户名",
					},
				},
			},
		},
	}

	rendered, err := renderer.RenderSQLGeneration(ctx, "查找所有用户", schema)
	if err != nil {
		t.Fatalf("Failed to render SQL generation template: %v", err)
	}

	if rendered == "" {
		t.Fatal("Rendered template should not be empty")
	}

	// 验证包含预期内容
	if !contains(rendered, "查找所有用户") {
		t.Error("Rendered template should contain query")
	}

	if !contains(rendered, "test_db") {
		t.Error("Rendered template should contain database name")
	}

	// 测试查询解释渲染
	rendered, err = renderer.RenderQueryExplanation(ctx, "SELECT * FROM users", []map[string]interface{}{
		{"id": 1, "name": "test"},
	})
	if err != nil {
		t.Fatalf("Failed to render query explanation template: %v", err)
	}

	if !contains(rendered, "SELECT * FROM users") {
		t.Error("Rendered template should contain SQL")
	}

	// 测试错误分析渲染
	rendered, err = renderer.RenderErrorAnalysis(ctx, "查找用户", "SELECT * FROM user", "Table 'user' doesn't exist")
	if err != nil {
		t.Fatalf("Failed to render error analysis template: %v", err)
	}

	if !contains(rendered, "查找用户") {
		t.Error("Rendered template should contain original query")
	}

	if !contains(rendered, "Table 'user' doesn't exist") {
		t.Error("Rendered template should contain error message")
	}

	// 测试建议渲染
	rendered, err = renderer.RenderSuggestion(ctx, "查找用户", schema)
	if err != nil {
		t.Fatalf("Failed to render suggestion template: %v", err)
	}

	if !contains(rendered, "查找用户") {
		t.Error("Rendered template should contain partial query")
	}
}

func TestGetLatestVersion(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	manager := NewTemplateManager(logger)

	// 注册多个版本的模板
	versions := []string{"1.0", "1.1", "2.0"}

	for i, version := range versions {
		template := NewTemplateBuilder().
			WithID(fmt.Sprintf("version_test_%d", i)).
			WithName(fmt.Sprintf("Version Test %s", version)).
			WithType(TemplateTypeQueryExplanation).
			WithVersion(version).
			WithTemplate(fmt.Sprintf("Version %s: {{.Var}}", version)).
			WithVariables("Var").
			Build()

		// 设置不同的更新时间
		template.UpdatedAt = time.Now().Add(time.Duration(i) * time.Minute)

		err := manager.RegisterTemplate(template)
		if err != nil {
			t.Fatalf("Failed to register template version %s: %v", version, err)
		}
	}

	// 获取最新版本
	latestVersion, err := manager.GetLatestVersion(TemplateTypeQueryExplanation)
	if err != nil {
		t.Fatalf("Failed to get latest version: %v", err)
	}

	if latestVersion != "2.0" {
		t.Errorf("Expected latest version '2.0', got '%s'", latestVersion)
	}
}

func TestMockTemplateManager(t *testing.T) {
	mock := NewMockTemplateManager()

	// 注册模板
	template := NewTemplateBuilder().
		WithID("mock_test").
		WithName("Mock Test").
		WithType(TemplateTypeSQLGeneration).
		WithVersion("1.0").
		WithTemplate("Mock template: {{.Var}}").
		Build()

	err := mock.RegisterTemplate(template)
	if err != nil {
		t.Fatalf("Failed to register template in mock: %v", err)
	}

	// 加载模板
	loaded, err := mock.LoadTemplate(TemplateTypeSQLGeneration, "1.0")
	if err != nil {
		t.Fatalf("Failed to load template from mock: %v", err)
	}

	if loaded.ID != "mock_test" {
		t.Errorf("Expected ID 'mock_test', got '%s'", loaded.ID)
	}

	// 渲染模板
	rendered, err := mock.RenderTemplate(TemplateTypeSQLGeneration, "1.0", map[string]interface{}{
		"Var": "test",
	})
	if err != nil {
		t.Fatalf("Failed to render template in mock: %v", err)
	}

	if rendered == "" {
		t.Fatal("Rendered template should not be empty")
	}

	// 列出模板
	templates, err := mock.ListTemplates(TemplateTypeSQLGeneration)
	if err != nil {
		t.Fatalf("Failed to list templates in mock: %v", err)
	}

	if len(templates) != 1 {
		t.Errorf("Expected 1 template, got %d", len(templates))
	}

	// 删除模板
	err = mock.DeleteTemplate(TemplateTypeSQLGeneration, "1.0")
	if err != nil {
		t.Fatalf("Failed to delete template in mock: %v", err)
	}

	// 验证删除
	_, err = mock.LoadTemplate(TemplateTypeSQLGeneration, "1.0")
	if err == nil {
		t.Fatal("Template should not exist after deletion")
	}
}

// 辅助函数
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) && (s[:len(substr)] == substr ||
			s[len(s)-len(substr):] == substr ||
			strings.Contains(s, substr))))
}

// 基准测试
func BenchmarkTemplateRender(b *testing.B) {
	logger, _ := zap.NewDevelopment()
	manager := NewTemplateManager(logger)

	variables := map[string]interface{}{
		"Schema": "CREATE TABLE users (id INT, name VARCHAR(100))",
		"Query":  "查找所有用户",
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := manager.RenderTemplate(TemplateTypeSQLGeneration, "1.0", variables)
		if err != nil {
			b.Fatalf("Template render failed: %v", err)
		}
	}
}

func BenchmarkTemplateLoad(b *testing.B) {
	logger, _ := zap.NewDevelopment()
	manager := NewTemplateManager(logger)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := manager.LoadTemplate(TemplateTypeSQLGeneration, "1.0")
		if err != nil {
			b.Fatalf("Template load failed: %v", err)
		}
	}
}
