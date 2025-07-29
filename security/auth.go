// 本文件实现了认证授权机制，集成现有的 JWT 认证系统和基于角色的权限控制。
// 主要功能：
// 1. JWT Token 验证和解析
// 2. 基于角色的权限控制 (RBAC)
// 3. API 访问权限验证
// 4. 用户信息管理和缓存

package security

import (
	"context"
	"fmt"
	"github.com/Anniext/rag/core"
	"strings"
	"time"

	"github.com/Anniext/Arkitektur/casbin"
	jwtgo "github.com/dgrijalva/jwt-go"
)

// AuthManager 认证授权管理器
type AuthManager struct {
	jwtSecret   string                // JWT 密钥
	tokenExpiry time.Duration         // Token 有效期
	enableRBAC  bool                  // 是否启用 RBAC
	cache       core.CacheManager     // 缓存管理器
	logger      core.Logger           // 日志记录器
	metrics     core.MetricsCollector // 指标收集器
}

// NewAuthManager 创建认证授权管理器
func NewAuthManager(config *core.SecurityConfig, cache core.CacheManager, logger core.Logger, metrics core.MetricsCollector) *AuthManager {
	return &AuthManager{
		jwtSecret:   config.JWTSecret,
		tokenExpiry: config.TokenExpiry,
		enableRBAC:  config.EnableRBAC,
		cache:       cache,
		logger:      logger,
		metrics:     metrics,
	}
}

// ValidateToken 验证 JWT Token 并返回用户信息
func (am *AuthManager) ValidateToken(ctx context.Context, tokenString string) (*core.UserInfo, error) {
	// 记录验证开始时间
	startTime := time.Now()
	defer func() {
		am.metrics.RecordHistogram("auth_token_validation_duration",
			float64(time.Since(startTime).Milliseconds()),
			map[string]string{"operation": "validate_token"})
	}()

	// 检查 Token 格式
	if tokenString == "" {
		am.metrics.IncrementCounter("auth_token_validation_errors",
			map[string]string{"error": "empty_token"})
		return nil, fmt.Errorf("Token 不能为空")
	}

	// 移除 Bearer 前缀
	tokenString = strings.TrimPrefix(tokenString, "Bearer ")

	// 首先检查缓存中是否有用户信息
	cacheKey := fmt.Sprintf("user_info:%s", tokenString)
	if cached, err := am.cache.Get(ctx, cacheKey); err == nil {
		if userInfo, ok := cached.(*core.UserInfo); ok {
			am.metrics.IncrementCounter("auth_token_cache_hits",
				map[string]string{"operation": "validate_token"})
			return userInfo, nil
		}
	}

	// 解析 JWT Token
	token, err := jwtgo.Parse(tokenString, func(token *jwtgo.Token) (interface{}, error) {
		// 验证签名方法
		if _, ok := token.Method.(*jwtgo.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("无效的签名方法: %v", token.Header["alg"])
		}
		return []byte(am.jwtSecret), nil
	})

	if err != nil {
		am.metrics.IncrementCounter("auth_token_validation_errors",
			map[string]string{"error": "parse_failed"})
		am.logger.Warn("JWT Token 解析失败", "error", err.Error())
		return nil, fmt.Errorf("Token 解析失败: %w", err)
	}

	// 验证 Token 有效性
	if !token.Valid {
		am.metrics.IncrementCounter("auth_token_validation_errors",
			map[string]string{"error": "invalid_token"})
		return nil, fmt.Errorf("Token 无效")
	}

	// 提取 Claims
	claims, ok := token.Claims.(jwtgo.MapClaims)
	if !ok {
		am.metrics.IncrementCounter("auth_token_validation_errors",
			map[string]string{"error": "invalid_claims"})
		return nil, fmt.Errorf("无效的 Token Claims")
	}

	// 验证过期时间
	if exp, ok := claims["exp"].(float64); ok {
		if time.Now().Unix() > int64(exp) {
			am.metrics.IncrementCounter("auth_token_validation_errors",
				map[string]string{"error": "token_expired"})
			return nil, fmt.Errorf("Token 已过期")
		}
	}

	// 构建用户信息
	userInfo := &core.UserInfo{}

	// 提取用户 ID
	if uid, ok := claims["uid"].(float64); ok {
		userInfo.ID = fmt.Sprintf("%.0f", uid)
	} else if uid, ok := claims["sub"].(string); ok {
		userInfo.ID = uid
	}

	// 提取用户名
	if username, ok := claims["username"].(string); ok {
		userInfo.Username = username
	}

	// 提取邮箱
	if email, ok := claims["email"].(string); ok {
		userInfo.Email = email
	}

	// 提取角色
	if roles, ok := claims["roles"].([]any); ok {
		userInfo.Roles = make([]string, len(roles))
		for i, role := range roles {
			if roleStr, ok := role.(string); ok {
				userInfo.Roles[i] = roleStr
			}
		}
	}

	// 提取权限
	if permissions, ok := claims["permissions"].([]any); ok {
		userInfo.Permissions = make([]string, len(permissions))
		for i, perm := range permissions {
			if permStr, ok := perm.(string); ok {
				userInfo.Permissions[i] = permStr
			}
		}
	}

	// 如果没有从 Token 中获取到角色和权限，从数据库或缓存中加载
	if len(userInfo.Roles) == 0 || len(userInfo.Permissions) == 0 {
		if err := am.loadUserRolesAndPermissions(ctx, userInfo); err != nil {
			am.logger.Warn("加载用户角色和权限失败", "user_id", userInfo.ID, "error", err.Error())
		}
	}

	// 缓存用户信息
	if err := am.cache.Set(ctx, cacheKey, userInfo, 5*time.Minute); err != nil {
		am.logger.Warn("缓存用户信息失败", "error", err.Error())
	}

	am.metrics.IncrementCounter("auth_token_validation_success",
		map[string]string{"user_id": userInfo.ID})

	return userInfo, nil
}

// CheckPermission 检查用户是否有指定资源的操作权限
func (am *AuthManager) CheckPermission(ctx context.Context, user *core.UserInfo, resource string, action string) error {
	// 记录权限检查开始时间
	startTime := time.Now()
	defer func() {
		am.metrics.RecordHistogram("auth_permission_check_duration",
			float64(time.Since(startTime).Milliseconds()),
			map[string]string{"resource": resource, "action": action})
	}()

	if user == nil {
		am.metrics.IncrementCounter("auth_permission_check_errors",
			map[string]string{"error": "nil_user", "resource": resource, "action": action})
		return fmt.Errorf("用户信息不能为空")
	}

	// 如果未启用 RBAC，则允许所有操作
	if !am.enableRBAC {
		am.metrics.IncrementCounter("auth_permission_check_success",
			map[string]string{"user_id": user.ID, "resource": resource, "action": action, "method": "rbac_disabled"})
		return nil
	}

	// 构建权限字符串
	permission := fmt.Sprintf("%s:%s", resource, action)

	// 首先检查用户是否有直接权限
	for _, userPerm := range user.Permissions {
		if userPerm == permission || userPerm == "*" || userPerm == fmt.Sprintf("%s:*", resource) {
			am.metrics.IncrementCounter("auth_permission_check_success",
				map[string]string{"user_id": user.ID, "resource": resource, "action": action, "method": "direct_permission"})
			return nil
		}
	}

	// 使用 Casbin 进行权限检查
	if am.enableRBAC {
		casbinClient := casbin.GetDefaultCasbin()
		if casbinClient != nil {
			// 检查用户角色权限
			for _, role := range user.Roles {
				allowed, err := casbinClient.Enforce(role, resource, action)
				if err != nil {
					am.logger.Warn("Casbin 权限检查失败", "role", role, "resource", resource, "action", action, "error", err.Error())
					continue
				}
				if allowed {
					am.metrics.IncrementCounter("auth_permission_check_success",
						map[string]string{"user_id": user.ID, "resource": resource, "action": action, "method": "casbin_role"})
					return nil
				}
			}

			// 检查用户直接权限
			allowed, err := casbinClient.Enforce(user.ID, resource, action)
			if err != nil {
				am.logger.Warn("Casbin 用户权限检查失败", "user_id", user.ID, "resource", resource, "action", action, "error", err.Error())
			} else if allowed {
				am.metrics.IncrementCounter("auth_permission_check_success",
					map[string]string{"user_id": user.ID, "resource": resource, "action": action, "method": "casbin_user"})
				return nil
			}
		}
	}

	// 权限检查失败
	am.metrics.IncrementCounter("auth_permission_check_errors",
		map[string]string{"error": "permission_denied", "user_id": user.ID, "resource": resource, "action": action})

	return fmt.Errorf("用户 %s 没有权限执行 %s 操作在资源 %s 上", user.Username, action, resource)
}

// loadUserRolesAndPermissions 从数据库或缓存中加载用户的角色和权限
func (am *AuthManager) loadUserRolesAndPermissions(ctx context.Context, user *core.UserInfo) error {
	// 检查缓存
	cacheKey := fmt.Sprintf("user_roles_permissions:%s", user.ID)
	if cached, err := am.cache.Get(ctx, cacheKey); err == nil {
		if rolesPerm, ok := cached.(map[string]interface{}); ok {
			if roles, ok := rolesPerm["roles"].([]string); ok {
				user.Roles = roles
			}
			if permissions, ok := rolesPerm["permissions"].([]string); ok {
				user.Permissions = permissions
			}
			return nil
		}
	}

	// 从数据库加载（这里使用默认值作为示例）
	// TODO: 实现从数据库加载用户角色和权限的逻辑
	if len(user.Roles) == 0 {
		user.Roles = []string{"user"} // 默认角色
	}

	if len(user.Permissions) == 0 {
		user.Permissions = []string{
			"rag:query",   // RAG 查询权限
			"schema:read", // Schema 读取权限
		}
	}

	// 缓存角色和权限信息
	rolesPerm := map[string]interface{}{
		"roles":       user.Roles,
		"permissions": user.Permissions,
	}
	if err := am.cache.Set(ctx, cacheKey, rolesPerm, 10*time.Minute); err != nil {
		am.logger.Warn("缓存用户角色权限失败", "error", err.Error())
	}

	return nil
}

// HasRole 检查用户是否具有指定角色
func (am *AuthManager) HasRole(user *core.UserInfo, role string) bool {
	if user == nil {
		return false
	}

	for _, userRole := range user.Roles {
		if userRole == role {
			return true
		}
	}
	return false
}

// HasPermission 检查用户是否具有指定权限
func (am *AuthManager) HasPermission(user *core.UserInfo, permission string) bool {
	if user == nil {
		return false
	}

	for _, userPerm := range user.Permissions {
		if userPerm == permission || userPerm == "*" {
			return true
		}
	}
	return false
}

// IsAdmin 检查用户是否为管理员
func (am *AuthManager) IsAdmin(user *core.UserInfo) bool {
	return am.HasRole(user, "admin") || am.HasRole(user, "administrator")
}

// RefreshUserCache 刷新用户缓存信息
func (am *AuthManager) RefreshUserCache(ctx context.Context, userID string) error {
	// 删除用户相关的缓存
	cacheKeys := []string{
		fmt.Sprintf("user_roles_permissions:%s", userID),
	}

	for _, key := range cacheKeys {
		if err := am.cache.Delete(ctx, key); err != nil {
			am.logger.Warn("删除用户缓存失败", "key", key, "error", err.Error())
		}
	}

	return nil
}

// ValidateAPIAccess 验证 API 访问权限
func (am *AuthManager) ValidateAPIAccess(ctx context.Context, user *core.UserInfo, apiPath string, method string) error {
	// 构建资源名称
	resource := fmt.Sprintf("api:%s", apiPath)
	action := strings.ToLower(method)

	return am.CheckPermission(ctx, user, resource, action)
}
