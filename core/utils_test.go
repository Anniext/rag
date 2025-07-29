package core

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSanitizeSQL 测试 SQL 清理
func TestSanitizeSQL(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "  SELECT * FROM users  ",
			expected: "SELECT * FROM users",
		},
		{
			input:    "SELECT\n*\nFROM\nusers",
			expected: "SELECT * FROM users",
		},
		{
			input:    "SELECT * FROM users -- comment",
			expected: "SELECT * FROM users",
		},
		{
			input:    "SELECT * FROM users /* comment */",
			expected: "SELECT * FROM users",
		},
		{
			input:    "SELECT   *   FROM   users",
			expected: "SELECT * FROM users",
		},
	}

	for i, test := range tests {
		result := SanitizeSQL(test.input)
		assert.Equal(t, test.expected, result, "Test case %d failed", i)
	}
}

// TestRemoveString 测试字符串移除
func TestRemoveString(t *testing.T) {
	slice := []string{"apple", "banana", "cherry", "banana"}
	result := RemoveString(slice, "banana")

	expected := []string{"apple", "cherry"}
	assert.Equal(t, expected, result)

	// 测试移除不存在的元素
	result = RemoveString(slice, "orange")
	assert.Equal(t, slice, result)

	// 测试空切片
	result = RemoveString([]string{}, "test")
	assert.Empty(t, result)
}

// TestMergeStringSlices 测试字符串切片合并
func TestMergeStringSlices(t *testing.T) {
	slice1 := []string{"a", "b", "c"}
	slice2 := []string{"b", "c", "d"}
	slice3 := []string{"c", "d", "e"}

	result := MergeStringSlices(slice1, slice2, slice3)
	expected := []string{"a", "b", "c", "d", "e"}

	assert.ElementsMatch(t, expected, result)
}

// TestGetStructFields 测试获取结构体字段
func TestGetStructFields(t *testing.T) {
	type TestStruct struct {
		PublicField  string
		AnotherField int
		privateField string // 不应该被包含
	}

	fields := GetStructFields(TestStruct{})
	expected := []string{"PublicField", "AnotherField"}

	assert.ElementsMatch(t, expected, fields)

	// 测试指针类型
	fields = GetStructFields(&TestStruct{})
	assert.ElementsMatch(t, expected, fields)

	// 测试非结构体类型
	fields = GetStructFields("not a struct")
	assert.Nil(t, fields)
}

// TestCopyStruct 测试结构体复制
func TestCopyStruct(t *testing.T) {
	type TestStruct struct {
		Name string
		Age  int
	}

	src := TestStruct{Name: "Alice", Age: 30}
	var dst TestStruct

	err := CopyStruct(src, &dst)
	require.NoError(t, err)
	assert.Equal(t, src, dst)

	// 测试指针源
	err = CopyStruct(&src, &dst)
	require.NoError(t, err)
	assert.Equal(t, src, dst)

	// 测试错误情况：目标不是指针
	err = CopyStruct(src, dst)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "dst must be a pointer")

	// 测试错误情况：类型不匹配
	type DifferentStruct struct {
		Name string
	}
	var differentDst DifferentStruct
	err = CopyStruct(src, &differentDst)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "src and dst must be the same type")
}

// TestMapToStruct 测试 map 转结构体
func TestMapToStruct(t *testing.T) {
	type TestStruct struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
		City string `json:"city"`
	}

	m := map[string]interface{}{
		"name": "Alice",
		"age":  30,
		"city": "Beijing",
	}

	var result TestStruct
	err := MapToStruct(m, &result)
	require.NoError(t, err)

	assert.Equal(t, "Alice", result.Name)
	assert.Equal(t, 30, result.Age)
	assert.Equal(t, "Beijing", result.City)

	// 测试错误情况：目标不是指针
	err = MapToStruct(m, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "dst must be a pointer")

	// 测试部分字段
	partialMap := map[string]interface{}{
		"name": "Bob",
	}

	var partialResult TestStruct
	err = MapToStruct(partialMap, &partialResult)
	require.NoError(t, err)
	assert.Equal(t, "Bob", partialResult.Name)
	assert.Equal(t, 0, partialResult.Age) // 默认值
}

// TestStructToMap 测试结构体转 map
func TestStructToMap(t *testing.T) {
	type TestStruct struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
		City string `json:"city"`
	}

	src := TestStruct{
		Name: "Alice",
		Age:  30,
		City: "Beijing",
	}

	result := StructToMap(src)
	expected := map[string]interface{}{
		"name": "Alice",
		"age":  30,
		"city": "Beijing",
	}

	assert.Equal(t, expected, result)

	// 测试指针类型
	result = StructToMap(&src)
	assert.Equal(t, expected, result)

	// 测试非结构体类型
	result = StructToMap("not a struct")
	assert.Empty(t, result)
}

// TestRetry 测试重试机制
func TestRetry(t *testing.T) {
	// 测试成功情况
	callCount := 0
	err := Retry(3, 10*time.Millisecond, func() error {
		callCount++
		if callCount < 2 {
			return errors.New("temporary error")
		}
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, 2, callCount)

	// 测试失败情况
	callCount = 0
	err = Retry(3, 10*time.Millisecond, func() error {
		callCount++
		return errors.New("persistent error")
	})

	assert.Error(t, err)
	assert.Equal(t, 3, callCount)
	assert.Contains(t, err.Error(), "persistent error")

	// 测试立即成功
	callCount = 0
	err = Retry(3, 10*time.Millisecond, func() error {
		callCount++
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, 1, callCount)
}

// TestTimeoutContext 测试超时上下文
func TestTimeoutContext(t *testing.T) {
	// 测试正常完成
	err := TimeoutContext(100*time.Millisecond, func() error {
		time.Sleep(10 * time.Millisecond)
		return nil
	})
	assert.NoError(t, err)

	// 测试超时
	err = TimeoutContext(50*time.Millisecond, func() error {
		time.Sleep(100 * time.Millisecond)
		return nil
	})
	assert.Error(t, err)
	assert.Equal(t, ErrRequestTimeout, err)

	// 测试函数返回错误
	testErr := errors.New("test error")
	err = TimeoutContext(100*time.Millisecond, func() error {
		return testErr
	})
	assert.Equal(t, testErr, err)
}

// TestSafeGoroutine 测试安全 goroutine
func TestSafeGoroutine(t *testing.T) {
	// 创建一个简单的 logger mock
	var loggedPanic interface{}
	mockLogger := &MockLogger{
		ErrorFunc: func(msg string, fields ...any) {
			if msg == "goroutine panic recovered" {
				for i := 0; i < len(fields); i += 2 {
					if fields[i] == "panic" && i+1 < len(fields) {
						loggedPanic = fields[i+1]
					}
				}
			}
		},
	}

	// 测试正常执行
	executed := false
	SafeGoroutine(func() {
		executed = true
	}, mockLogger)

	// 等待 goroutine 执行完成
	time.Sleep(10 * time.Millisecond)
	assert.True(t, executed)

	// 测试 panic 恢复
	SafeGoroutine(func() {
		panic("test panic")
	}, mockLogger)

	// 等待 goroutine 执行完成
	time.Sleep(10 * time.Millisecond)
	assert.Equal(t, "test panic", loggedPanic)

	// 测试无 logger 的情况
	SafeGoroutine(func() {
		panic("test panic without logger")
	}, nil)

	// 等待 goroutine 执行完成
	time.Sleep(10 * time.Millisecond)
	// 应该不会崩溃
}

// TestGetEnvOrDefault 测试环境变量获取
func TestGetEnvOrDefault(t *testing.T) {
	// 测试默认值
	result := GetEnvOrDefault("NON_EXISTENT_ENV_VAR", "default_value")
	assert.Equal(t, "default_value", result)

	// 设置环境变量并测试
	t.Setenv("TEST_ENV_VAR", "test_value")
	result = GetEnvOrDefault("TEST_ENV_VAR", "default_value")
	assert.Equal(t, "test_value", result)

	// 测试空白值
	t.Setenv("EMPTY_ENV_VAR", "  ")
	result = GetEnvOrDefault("EMPTY_ENV_VAR", "default_value")
	assert.Equal(t, "default_value", result)
}

// MockLogger 用于测试的 mock logger
type MockLogger struct {
	DebugFunc func(msg string, fields ...any)
	InfoFunc  func(msg string, fields ...any)
	WarnFunc  func(msg string, fields ...any)
	ErrorFunc func(msg string, fields ...any)
	FatalFunc func(msg string, fields ...any)
}

func (m *MockLogger) Debug(msg string, fields ...any) {
	if m.DebugFunc != nil {
		m.DebugFunc(msg, fields...)
	}
}

func (m *MockLogger) Info(msg string, fields ...any) {
	if m.InfoFunc != nil {
		m.InfoFunc(msg, fields...)
	}
}

func (m *MockLogger) Warn(msg string, fields ...any) {
	if m.WarnFunc != nil {
		m.WarnFunc(msg, fields...)
	}
}

func (m *MockLogger) Error(msg string, fields ...any) {
	if m.ErrorFunc != nil {
		m.ErrorFunc(msg, fields...)
	}
}

func (m *MockLogger) Fatal(msg string, fields ...any) {
	if m.FatalFunc != nil {
		m.FatalFunc(msg, fields...)
	}
}

// 基准测试
func BenchmarkSanitizeSQL(b *testing.B) {
	sql := "  SELECT   *   FROM   users   WHERE   name   =   'test'   --   comment  "

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		SanitizeSQL(sql)
	}
}

func BenchmarkUniqueStrings(b *testing.B) {
	slice := []string{"a", "b", "c", "a", "b", "c", "d", "e", "f", "d", "e", "f"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		UniqueStrings(slice)
	}
}

func BenchmarkStructToMap(b *testing.B) {
	type TestStruct struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
		City string `json:"city"`
	}

	src := TestStruct{
		Name: "Alice",
		Age:  30,
		City: "Beijing",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		StructToMap(src)
	}
}

func BenchmarkMapToStruct(b *testing.B) {
	type TestStruct struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
		City string `json:"city"`
	}

	m := map[string]interface{}{
		"name": "Alice",
		"age":  30,
		"city": "Beijing",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var result TestStruct
		MapToStruct(m, &result)
	}
}

func BenchmarkFormatDuration(b *testing.B) {
	duration := 123456789 * time.Nanosecond

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		FormatDuration(duration)
	}
}

func BenchmarkFormatBytes(b *testing.B) {
	bytes := int64(1234567890)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		FormatBytes(bytes)
	}
}

func BenchmarkParseSize(b *testing.B) {
	sizeStr := "100MB"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ParseSize(sizeStr)
	}
}
