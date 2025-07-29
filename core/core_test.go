package core

import (
	"strings"
	"testing"
	"time"
)

// TestGenerateRequestID 测试请求ID生成
func TestGenerateRequestID(t *testing.T) {
	id1 := GenerateRequestID()
	id2 := GenerateRequestID()

	if id1 == id2 {
		t.Error("生成的请求ID应该是唯一的")
	}

	if len(id1) == 0 {
		t.Error("请求ID不能为空")
	}

	if !strings.HasPrefix(id1, "req_") {
		t.Error("请求ID应该以 'req_' 开头")
	}
}

// TestGenerateSessionID 测试会话ID生成
func TestGenerateSessionID(t *testing.T) {
	id1 := GenerateSessionID()
	id2 := GenerateSessionID()

	if id1 == id2 {
		t.Error("生成的会话ID应该是唯一的")
	}

	if len(id1) == 0 {
		t.Error("会话ID不能为空")
	}

	if !strings.HasPrefix(id1, "sess_") {
		t.Error("会话ID应该以 'sess_' 开头")
	}
}

// TestValidateTableName 测试表名验证
func TestValidateTableName(t *testing.T) {
	validNames := []string{
		"users",
		"user_profiles",
		"_temp_table",
		"table123",
	}

	invalidNames := []string{
		"",
		"123table",
		"user-profiles",
		"user profiles",
		"user.profiles",
	}

	for _, name := range validNames {
		if !ValidateTableName(name) {
			t.Errorf("表名 '%s' 应该是有效的", name)
		}
	}

	for _, name := range invalidNames {
		if ValidateTableName(name) {
			t.Errorf("表名 '%s' 应该是无效的", name)
		}
	}
}

// TestValidateColumnName 测试列名验证
func TestValidateColumnName(t *testing.T) {
	validNames := []string{
		"id",
		"user_id",
		"_created_at",
		"column123",
	}

	invalidNames := []string{
		"",
		"123column",
		"user-id",
		"user id",
		"user.id",
	}

	for _, name := range validNames {
		if !ValidateColumnName(name) {
			t.Errorf("列名 '%s' 应该是有效的", name)
		}
	}

	for _, name := range invalidNames {
		if ValidateColumnName(name) {
			t.Errorf("列名 '%s' 应该是无效的", name)
		}
	}
}

// TestIsValidEmail 测试邮箱验证
func TestIsValidEmail(t *testing.T) {
	validEmails := []string{
		"user@example.com",
		"test.email@domain.co.uk",
		"user+tag@example.org",
	}

	invalidEmails := []string{
		"",
		"invalid-email",
		"@example.com",
		"user@",
		"user@.com",
	}

	for _, email := range validEmails {
		if !IsValidEmail(email) {
			t.Errorf("邮箱 '%s' 应该是有效的", email)
		}
	}

	for _, email := range invalidEmails {
		if IsValidEmail(email) {
			t.Errorf("邮箱 '%s' 应该是无效的", email)
		}
	}
}

// TestIsStrongPassword 测试密码强度验证
func TestIsStrongPassword(t *testing.T) {
	strongPasswords := []string{
		"Password123!",
		"MyStr0ng@Pass",
		"C0mplex#Password",
	}

	weakPasswords := []string{
		"",
		"123456",
		"password",
		"Password",
		"Password123",
		"password123!",
		"PASSWORD123!",
	}

	for _, password := range strongPasswords {
		if !IsStrongPassword(password) {
			t.Errorf("密码 '%s' 应该是强密码", password)
		}
	}

	for _, password := range weakPasswords {
		if IsStrongPassword(password) {
			t.Errorf("密码 '%s' 应该是弱密码", password)
		}
	}
}

// TestTruncateString 测试字符串截断
func TestTruncateString(t *testing.T) {
	tests := []struct {
		input     string
		maxLength int
		expected  string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "he..."},
		{"hello", 5, "hello"},
		{"hello", 3, "hel"},
		{"hello", 2, "he"},
	}

	for _, test := range tests {
		result := TruncateString(test.input, test.maxLength)
		if result != test.expected {
			t.Errorf("TruncateString('%s', %d) = '%s', expected '%s'",
				test.input, test.maxLength, result, test.expected)
		}
	}
}

// TestContainsString 测试字符串包含检查
func TestContainsString(t *testing.T) {
	slice := []string{"apple", "banana", "cherry"}

	if !ContainsString(slice, "banana") {
		t.Error("应该包含 'banana'")
	}

	if ContainsString(slice, "orange") {
		t.Error("不应该包含 'orange'")
	}
}

// TestUniqueStrings 测试字符串去重
func TestUniqueStrings(t *testing.T) {
	input := []string{"apple", "banana", "apple", "cherry", "banana"}
	expected := []string{"apple", "banana", "cherry"}

	result := UniqueStrings(input)

	if len(result) != len(expected) {
		t.Errorf("期望长度 %d，实际长度 %d", len(expected), len(result))
	}

	for _, item := range expected {
		if !ContainsString(result, item) {
			t.Errorf("结果中应该包含 '%s'", item)
		}
	}
}

// TestFormatDuration 测试时间间隔格式化
func TestFormatDuration(t *testing.T) {
	tests := []struct {
		duration time.Duration
		contains string
	}{
		{100 * time.Nanosecond, "ns"},
		{100 * time.Microsecond, "μs"},
		{100 * time.Millisecond, "ms"},
		{2 * time.Second, "s"},
		{2 * time.Minute, "m"},
		{2 * time.Hour, "h"},
	}

	for _, test := range tests {
		result := FormatDuration(test.duration)
		if !strings.Contains(result, test.contains) {
			t.Errorf("FormatDuration(%v) = '%s', should contain '%s'",
				test.duration, result, test.contains)
		}
	}
}

// TestFormatBytes 测试字节数格式化
func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes    int64
		contains string
	}{
		{512, "B"},
		{1024, "KB"},
		{1024 * 1024, "MB"},
		{1024 * 1024 * 1024, "GB"},
	}

	for _, test := range tests {
		result := FormatBytes(test.bytes)
		if !strings.Contains(result, test.contains) {
			t.Errorf("FormatBytes(%d) = '%s', should contain '%s'",
				test.bytes, result, test.contains)
		}
	}
}

// TestContainer 测试依赖注入容器
func TestContainer(t *testing.T) {
	container := NewContainer()

	// 测试注册和获取服务
	container.Register("test_service", "test_value")

	value, err := container.Get("test_service")
	if err != nil {
		t.Errorf("获取服务失败: %v", err)
	}

	if value != "test_value" {
		t.Errorf("期望 'test_value'，实际 '%v'", value)
	}

	// 测试不存在的服务
	_, err = container.Get("non_existent")
	if err == nil {
		t.Error("获取不存在的服务应该返回错误")
	}

	// 测试单例服务
	counter := 0
	container.RegisterSingleton("counter", func() interface{} {
		counter++
		return counter
	})

	value1, _ := container.Get("counter")
	value2, _ := container.Get("counter")

	if value1 != value2 {
		t.Error("单例服务应该返回相同的实例")
	}

	if counter != 1 {
		t.Errorf("单例工厂函数应该只调用一次，实际调用了 %d 次", counter)
	}
}

// TestRAGError 测试 RAG 错误
func TestRAGError(t *testing.T) {
	err := NewRAGError(ErrorTypeValidation, "TEST_ERROR", "测试错误")

	if err.Type != ErrorTypeValidation {
		t.Errorf("期望错误类型 %s，实际 %s", ErrorTypeValidation, err.Type)
	}

	if err.Code != "TEST_ERROR" {
		t.Errorf("期望错误码 'TEST_ERROR'，实际 '%s'", err.Code)
	}

	if err.Message != "测试错误" {
		t.Errorf("期望错误消息 '测试错误'，实际 '%s'", err.Message)
	}

	// 测试链式调用
	err = err.WithDetails(map[string]any{"field": "value"}).
		WithRequestID("req_123").
		WithUserID("user_456")

	if err.RequestID != "req_123" {
		t.Errorf("期望请求ID 'req_123'，实际 '%s'", err.RequestID)
	}

	if err.UserID != "user_456" {
		t.Errorf("期望用户ID 'user_456'，实际 '%s'", err.UserID)
	}

	if err.Details["field"] != "value" {
		t.Error("错误详情设置失败")
	}
}

// TestParseSize 测试大小解析
func TestParseSize(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
		hasError bool
	}{
		{"100", 100, false},
		{"1KB", 1024, false},
		{"1MB", 1024 * 1024, false},
		{"1GB", 1024 * 1024 * 1024, false},
		{"1.5MB", int64(1.5 * 1024 * 1024), false},
		{"invalid", 0, true},
		{"", 0, true},
	}

	for _, test := range tests {
		result, err := ParseSize(test.input)

		if test.hasError {
			if err == nil {
				t.Errorf("ParseSize('%s') 应该返回错误", test.input)
			}
		} else {
			if err != nil {
				t.Errorf("ParseSize('%s') 不应该返回错误: %v", test.input, err)
			}
			if result != test.expected {
				t.Errorf("ParseSize('%s') = %d, expected %d", test.input, result, test.expected)
			}
		}
	}
}
