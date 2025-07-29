package security

import (
	"context"
	"testing"
	"time"

	"pumppill/rag/core"

	"github.com/dgrijalva/jwt-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockCacheManager 模拟缓存管理器
type MockCacheManager struct {
	mock.Mock
}

func (m *MockCacheManager) Get(ctx context.Context, key string) (interface{}, error) {
	args := m.Called(ctx, key)
	return args.Get(0), args.Error(1)
}

func (m *MockCacheManager) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	args := m.Called(ctx, key, value, ttl)
	return args.Error(0)
}

func (m *MockCacheManager) Delete(ctx context.Context, key string) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

func (m *MockCacheManager) Clear(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// MockLogger 模拟日志记录器
type MockLogger struct {
	mock.Mock
}

func (m *MockLogger) Debug(msg string, fields ...any) {
	m.Called(msg, fields)
}

func (m *MockLogger) Info(msg string, fields ...any) {
	m.Called(msg, fields)
}

func (m *MockLogger) Warn(msg string, fields ...any) {
	m.Called(msg, fields)
}

func (m *MockLogger) Error(msg string, fields ...any) {
	m.Called(msg, fields)
}

func (m *MockLogger) Fatal(msg string, fields ...any) {
	m.Called(msg, fields)
}

// MockMetricsCollector 模拟指标收集器
type MockMetricsCollector struct {
	mock.Mock
}

func (m *MockMetricsCollector) IncrementCounter(name string, labels map[string]string) {
	m.Called(name, labels)
}

func (m *MockMetricsCollector) RecordHistogram(name string, value float64, labels map[string]string) {
	m.Called(name, value, labels)
}

func (m *MockMetricsCollector) SetGauge(name string, value float64, labels map[string]string) {
	m.Called(name, value, labels)
}

func TestAuthManager_ValidateToken(t *testing.T) {
	// 准备测试数据
	jwtSecret := "test-secret-key"
	config := &core.SecurityConfig{
		JWTSecret:   jwtSecret,
		TokenExpiry: time.Hour,
		EnableRBAC:  true,
	}

	mockCache := &MockCacheManager{}
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsCollector{}

	// 设置 mock 期望
	mockCache.On("Get", mock.Anything, mock.AnythingOfType("string")).Return(nil, assert.AnError)
	mockCache.On("Set", mock.Anything, mock.AnythingOfType("string"), mock.Anything, mock.AnythingOfType("time.Duration")).Return(nil)
	mockLogger.On("Warn", mock.AnythingOfType("string"), mock.Anything).Return()
	mockLogger.On("Debug", mock.AnythingOfType("string"), mock.Anything).Return()
	mockMetrics.On("RecordHistogram", mock.AnythingOfType("string"), mock.AnythingOfType("float64"), mock.AnythingOfType("map[string]string")).Return()
	mockMetrics.On("IncrementCounter", mock.AnythingOfType("string"), mock.AnythingOfType("map[string]string")).Return()

	authManager := NewAuthManager(config, mockCache, mockLogger, mockMetrics)

	t.Run("ValidateToken_Success", func(t *testing.T) {
		// 创建有效的 JWT Token
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"uid":      123,
			"username": "testuser",
			"email":    "test@example.com",
			"roles":    []string{"user"},
			"exp":      time.Now().Add(time.Hour).Unix(),
		})

		tokenString, err := token.SignedString([]byte(jwtSecret))
		assert.NoError(t, err)

		// 验证 Token
		userInfo, err := authManager.ValidateToken(context.Background(), tokenString)
		assert.NoError(t, err)
		assert.NotNil(t, userInfo)
		assert.Equal(t, "123", userInfo.ID)
		assert.Equal(t, "testuser", userInfo.Username)
		assert.Equal(t, "test@example.com", userInfo.Email)
		assert.Contains(t, userInfo.Roles, "user")
	})

	t.Run("ValidateToken_EmptyToken", func(t *testing.T) {
		userInfo, err := authManager.ValidateToken(context.Background(), "")
		assert.Error(t, err)
		assert.Nil(t, userInfo)
		assert.Contains(t, err.Error(), "Token 不能为空")
	})

	t.Run("ValidateToken_InvalidToken", func(t *testing.T) {
		userInfo, err := authManager.ValidateToken(context.Background(), "invalid-token")
		assert.Error(t, err)
		assert.Nil(t, userInfo)
		assert.Contains(t, err.Error(), "Token 解析失败")
	})

	t.Run("ValidateToken_ExpiredToken", func(t *testing.T) {
		// 创建过期的 JWT Token
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"uid": 123,
			"exp": time.Now().Add(-time.Hour).Unix(), // 过期时间设置为1小时前
		})

		tokenString, err := token.SignedString([]byte(jwtSecret))
		assert.NoError(t, err)

		userInfo, err := authManager.ValidateToken(context.Background(), tokenString)
		assert.Error(t, err)
		assert.Nil(t, userInfo)
		assert.Contains(t, err.Error(), "Token 解析失败")
	})
}

func TestAuthManager_CheckPermission(t *testing.T) {
	config := &core.SecurityConfig{
		JWTSecret:   "test-secret",
		TokenExpiry: time.Hour,
		EnableRBAC:  true,
	}

	mockCache := &MockCacheManager{}
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsCollector{}

	// 设置 mock 期望
	mockLogger.On("Warn", mock.AnythingOfType("string"), mock.Anything).Return()
	mockLogger.On("Debug", mock.AnythingOfType("string"), mock.Anything).Return()
	mockMetrics.On("RecordHistogram", mock.AnythingOfType("string"), mock.AnythingOfType("float64"), mock.AnythingOfType("map[string]string")).Return()
	mockMetrics.On("IncrementCounter", mock.AnythingOfType("string"), mock.AnythingOfType("map[string]string")).Return()

	authManager := NewAuthManager(config, mockCache, mockLogger, mockMetrics)

	t.Run("CheckPermission_NilUser", func(t *testing.T) {
		err := authManager.CheckPermission(context.Background(), nil, "resource", "action")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "用户信息不能为空")
	})

	t.Run("CheckPermission_DirectPermission", func(t *testing.T) {
		user := &core.UserInfo{
			ID:          "123",
			Username:    "testuser",
			Permissions: []string{"resource:action"},
		}

		err := authManager.CheckPermission(context.Background(), user, "resource", "action")
		assert.NoError(t, err)
	})

	t.Run("CheckPermission_WildcardPermission", func(t *testing.T) {
		user := &core.UserInfo{
			ID:          "123",
			Username:    "testuser",
			Permissions: []string{"*"},
		}

		err := authManager.CheckPermission(context.Background(), user, "resource", "action")
		assert.NoError(t, err)
	})

	t.Run("CheckPermission_ResourceWildcard", func(t *testing.T) {
		user := &core.UserInfo{
			ID:          "123",
			Username:    "testuser",
			Permissions: []string{"resource:*"},
		}

		err := authManager.CheckPermission(context.Background(), user, "resource", "action")
		assert.NoError(t, err)
	})

	t.Run("CheckPermission_NoPermission", func(t *testing.T) {
		user := &core.UserInfo{
			ID:          "123",
			Username:    "testuser",
			Permissions: []string{"other:action"},
		}

		err := authManager.CheckPermission(context.Background(), user, "resource", "action")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "没有权限执行")
	})
}

func TestAuthManager_HasRole(t *testing.T) {
	config := &core.SecurityConfig{
		JWTSecret:   "test-secret",
		TokenExpiry: time.Hour,
		EnableRBAC:  true,
	}

	mockCache := &MockCacheManager{}
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsCollector{}

	authManager := NewAuthManager(config, mockCache, mockLogger, mockMetrics)

	t.Run("HasRole_Success", func(t *testing.T) {
		user := &core.UserInfo{
			ID:    "123",
			Roles: []string{"admin", "user"},
		}

		assert.True(t, authManager.HasRole(user, "admin"))
		assert.True(t, authManager.HasRole(user, "user"))
		assert.False(t, authManager.HasRole(user, "manager"))
	})

	t.Run("HasRole_NilUser", func(t *testing.T) {
		assert.False(t, authManager.HasRole(nil, "admin"))
	})
}

func TestAuthManager_IsAdmin(t *testing.T) {
	config := &core.SecurityConfig{
		JWTSecret:   "test-secret",
		TokenExpiry: time.Hour,
		EnableRBAC:  true,
	}

	mockCache := &MockCacheManager{}
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsCollector{}

	authManager := NewAuthManager(config, mockCache, mockLogger, mockMetrics)

	t.Run("IsAdmin_True", func(t *testing.T) {
		user := &core.UserInfo{
			ID:    "123",
			Roles: []string{"admin"},
		}

		assert.True(t, authManager.IsAdmin(user))
	})

	t.Run("IsAdmin_Administrator", func(t *testing.T) {
		user := &core.UserInfo{
			ID:    "123",
			Roles: []string{"administrator"},
		}

		assert.True(t, authManager.IsAdmin(user))
	})

	t.Run("IsAdmin_False", func(t *testing.T) {
		user := &core.UserInfo{
			ID:    "123",
			Roles: []string{"user"},
		}

		assert.False(t, authManager.IsAdmin(user))
	})
}

func TestAuthManager_ValidateAPIAccess(t *testing.T) {
	config := &core.SecurityConfig{
		JWTSecret:   "test-secret",
		TokenExpiry: time.Hour,
		EnableRBAC:  true,
	}

	mockCache := &MockCacheManager{}
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsCollector{}

	// 设置 mock 期望
	mockLogger.On("Warn", mock.AnythingOfType("string"), mock.Anything).Return()
	mockMetrics.On("RecordHistogram", mock.AnythingOfType("string"), mock.AnythingOfType("float64"), mock.AnythingOfType("map[string]string")).Return()
	mockMetrics.On("IncrementCounter", mock.AnythingOfType("string"), mock.AnythingOfType("map[string]string")).Return()

	authManager := NewAuthManager(config, mockCache, mockLogger, mockMetrics)

	t.Run("ValidateAPIAccess_Success", func(t *testing.T) {
		user := &core.UserInfo{
			ID:          "123",
			Username:    "testuser",
			Permissions: []string{"api:/test:get"},
		}

		err := authManager.ValidateAPIAccess(context.Background(), user, "/test", "GET")
		assert.NoError(t, err)
	})

	t.Run("ValidateAPIAccess_NoPermission", func(t *testing.T) {
		user := &core.UserInfo{
			ID:          "123",
			Username:    "testuser",
			Permissions: []string{"api:/other:get"},
		}

		err := authManager.ValidateAPIAccess(context.Background(), user, "/test", "GET")
		assert.Error(t, err)
	})
}
