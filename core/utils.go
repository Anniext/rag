package core

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strings"
	"time"
	"unicode"
)

// GenerateRequestID 生成请求ID
func GenerateRequestID() string {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		// 如果随机数生成失败，使用时间戳
		return fmt.Sprintf("req_%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("req_%s", hex.EncodeToString(bytes))
}

// GenerateSessionID 生成会话ID
func GenerateSessionID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("sess_%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("sess_%s", hex.EncodeToString(bytes))
}

// SanitizeSQL 清理 SQL 语句
func SanitizeSQL(sql string) string {
	// 移除多余的空白字符
	sql = strings.TrimSpace(sql)
	sql = regexp.MustCompile(`\s+`).ReplaceAllString(sql, " ")

	// 移除注释
	sql = regexp.MustCompile(`--.*`).ReplaceAllString(sql, "")
	sql = regexp.MustCompile(`/\*.*?\*/`).ReplaceAllString(sql, "")

	// 再次清理空白字符
	return strings.TrimSpace(sql)
}

// ValidateTableName 验证表名
func ValidateTableName(tableName string) bool {
	if tableName == "" {
		return false
	}

	// 表名只能包含字母、数字和下划线，且不能以数字开头
	matched, _ := regexp.MatchString(`^[a-zA-Z_][a-zA-Z0-9_]*$`, tableName)
	return matched
}

// ValidateColumnName 验证列名
func ValidateColumnName(columnName string) bool {
	if columnName == "" {
		return false
	}

	// 列名只能包含字母、数字和下划线，且不能以数字开头
	matched, _ := regexp.MatchString(`^[a-zA-Z_][a-zA-Z0-9_]*$`, columnName)
	return matched
}

// IsValidEmail 验证邮箱格式
func IsValidEmail(email string) bool {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(email)
}

// IsStrongPassword 检查密码强度
func IsStrongPassword(password string) bool {
	if len(password) < MinPasswordLength {
		return false
	}

	var (
		hasUpper   = false
		hasLower   = false
		hasNumber  = false
		hasSpecial = false
	)

	for _, char := range password {
		switch {
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsNumber(char):
			hasNumber = true
		case unicode.IsPunct(char) || unicode.IsSymbol(char):
			hasSpecial = true
		}
	}

	return hasUpper && hasLower && hasNumber && hasSpecial
}

// TruncateString 截断字符串
func TruncateString(s string, maxLength int) string {
	if len(s) <= maxLength {
		return s
	}

	if maxLength <= 3 {
		return s[:maxLength]
	}

	return s[:maxLength-3] + "..."
}

// ContainsString 检查字符串切片是否包含指定字符串
func ContainsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// RemoveString 从字符串切片中移除指定字符串
func RemoveString(slice []string, item string) []string {
	result := make([]string, 0, len(slice))
	for _, s := range slice {
		if s != item {
			result = append(result, s)
		}
	}
	return result
}

// UniqueStrings 去除字符串切片中的重复项
func UniqueStrings(slice []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(slice))

	for _, s := range slice {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}

	return result
}

// MergeStringSlices 合并多个字符串切片并去重
func MergeStringSlices(slices ...[]string) []string {
	var result []string
	for _, slice := range slices {
		result = append(result, slice...)
	}
	return UniqueStrings(result)
}

// FormatDuration 格式化时间间隔
func FormatDuration(d time.Duration) string {
	if d < time.Microsecond {
		return fmt.Sprintf("%.0fns", float64(d.Nanoseconds()))
	}
	if d < time.Millisecond {
		return fmt.Sprintf("%.2fμs", float64(d.Nanoseconds())/1000)
	}
	if d < time.Second {
		return fmt.Sprintf("%.2fms", float64(d.Nanoseconds())/1000000)
	}
	if d < time.Minute {
		return fmt.Sprintf("%.2fs", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%.1fm", d.Minutes())
	}
	return fmt.Sprintf("%.1fh", d.Hours())
}

// FormatBytes 格式化字节数
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	units := []string{"KB", "MB", "GB", "TB", "PB"}
	return fmt.Sprintf("%.1f %s", float64(bytes)/float64(div), units[exp])
}

// GetStructFields 获取结构体字段信息
func GetStructFields(v interface{}) []string {
	t := reflect.TypeOf(v)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if t.Kind() != reflect.Struct {
		return nil
	}

	fields := make([]string, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.IsExported() {
			fields = append(fields, field.Name)
		}
	}

	return fields
}

// CopyStruct 复制结构体
func CopyStruct(src, dst interface{}) error {
	srcValue := reflect.ValueOf(src)
	dstValue := reflect.ValueOf(dst)

	if srcValue.Kind() == reflect.Ptr {
		srcValue = srcValue.Elem()
	}
	if dstValue.Kind() != reflect.Ptr {
		return fmt.Errorf("dst must be a pointer")
	}
	dstValue = dstValue.Elem()

	if srcValue.Type() != dstValue.Type() {
		return fmt.Errorf("src and dst must be the same type")
	}

	dstValue.Set(srcValue)
	return nil
}

// MapToStruct 将 map 转换为结构体
func MapToStruct(m map[string]interface{}, dst interface{}) error {
	dstValue := reflect.ValueOf(dst)
	if dstValue.Kind() != reflect.Ptr {
		return fmt.Errorf("dst must be a pointer")
	}

	dstValue = dstValue.Elem()
	dstType := dstValue.Type()

	for i := 0; i < dstType.NumField(); i++ {
		field := dstType.Field(i)
		fieldValue := dstValue.Field(i)

		if !fieldValue.CanSet() {
			continue
		}

		// 获取字段名（优先使用 json tag）
		fieldName := field.Name
		if jsonTag := field.Tag.Get("json"); jsonTag != "" {
			if parts := strings.Split(jsonTag, ","); len(parts) > 0 && parts[0] != "" {
				fieldName = parts[0]
			}
		}

		if value, exists := m[fieldName]; exists && value != nil {
			valueReflect := reflect.ValueOf(value)
			if valueReflect.Type().AssignableTo(fieldValue.Type()) {
				fieldValue.Set(valueReflect)
			}
		}
	}

	return nil
}

// StructToMap 将结构体转换为 map
func StructToMap(src interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	srcValue := reflect.ValueOf(src)
	if srcValue.Kind() == reflect.Ptr {
		srcValue = srcValue.Elem()
	}

	if srcValue.Kind() != reflect.Struct {
		return result
	}

	srcType := srcValue.Type()
	for i := 0; i < srcType.NumField(); i++ {
		field := srcType.Field(i)
		fieldValue := srcValue.Field(i)

		if !field.IsExported() {
			continue
		}

		// 获取字段名（优先使用 json tag）
		fieldName := field.Name
		if jsonTag := field.Tag.Get("json"); jsonTag != "" {
			if parts := strings.Split(jsonTag, ","); len(parts) > 0 && parts[0] != "" {
				fieldName = parts[0]
			}
		}

		result[fieldName] = fieldValue.Interface()
	}

	return result
}

// Retry 重试函数
func Retry(attempts int, delay time.Duration, fn func() error) error {
	var err error
	for i := 0; i < attempts; i++ {
		if err = fn(); err == nil {
			return nil
		}

		if i < attempts-1 {
			time.Sleep(delay)
			delay *= 2 // 指数退避
		}
	}
	return err
}

// TimeoutContext 创建带超时的上下文执行函数
func TimeoutContext(timeout time.Duration, fn func() error) error {
	done := make(chan error, 1)

	go func() {
		done <- fn()
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		return ErrRequestTimeout
	}
}

// SafeGoroutine 安全地启动 goroutine
func SafeGoroutine(fn func(), logger Logger) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				if logger != nil {
					logger.Error("goroutine panic recovered", "panic", r)
				}
			}
		}()
		fn()
	}()
}

// GetEnvOrDefault 获取环境变量或默认值
func GetEnvOrDefault(key, defaultValue string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return defaultValue
}

// ParseSize 解析大小字符串（如 "100MB"）
func ParseSize(sizeStr string) (int64, error) {
	sizeStr = strings.TrimSpace(strings.ToUpper(sizeStr))

	units := map[string]int64{
		"B":  1,
		"KB": 1024,
		"MB": 1024 * 1024,
		"GB": 1024 * 1024 * 1024,
		"TB": 1024 * 1024 * 1024 * 1024,
	}

	// 按长度排序，确保先匹配较长的单位（如 "GB" 在 "B" 之前）
	unitOrder := []string{"TB", "GB", "MB", "KB", "B"}

	for _, unit := range unitOrder {
		if strings.HasSuffix(sizeStr, unit) {
			numStr := strings.TrimSuffix(sizeStr, unit)
			var num float64
			if _, err := fmt.Sscanf(numStr, "%f", &num); err != nil {
				return 0, fmt.Errorf("invalid size format: %s", sizeStr)
			}
			return int64(num * float64(units[unit])), nil
		}
	}

	// 如果没有单位，假设是字节
	var num int64
	if _, err := fmt.Sscanf(sizeStr, "%d", &num); err != nil {
		return 0, fmt.Errorf("invalid size format: %s", sizeStr)
	}
	return num, nil
}
