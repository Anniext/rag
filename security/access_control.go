// 本文件实现了数据访问控制机制，包括基于用户角色的数据访问限制、敏感数据脱敏和审计日志。
// 主要功能：
// 1. 基于用户角色的数据访问限制
// 2. 敏感数据脱敏和保护
// 3. 数据行级权限控制
// 4. 查询结果过滤

package security

import (
	"context"
	"fmt"
	"github.com/Anniext/rag/core"
	"regexp"
	"strings"
	"time"
)

// AccessController 数据访问控制器
type AccessController struct {
	logger                 core.Logger                     // 日志记录器
	metrics                core.MetricsCollector           // 指标收集器
	sensitiveFields        map[string]*regexp.Regexp       // 敏感字段模式
	maskingRules           map[string]MaskingRule          // 脱敏规则
	tablePermissions       map[string]TablePermission      // 表权限配置
	dataGovernancePolicies map[string]DataGovernancePolicy // 数据治理策略
}

// MaskingRule 脱敏规则
type MaskingRule struct {
	Type        string // 脱敏类型：mask, hash, remove, partial
	Pattern     string // 脱敏模式
	Replacement string // 替换内容
}

// TablePermission 表权限配置
type TablePermission struct {
	TableName        string   // 表名
	AllowedRoles     []string // 允许访问的角色
	RestrictedFields []string // 受限字段
	RowLevelFilter   string   // 行级过滤条件
}

// DataFilter 数据过滤器
type DataFilter struct {
	User      *core.UserInfo // 用户信息
	TableName string         // 表名
	Operation string         // 操作类型
}

// NewAccessController 创建数据访问控制器
func NewAccessController(logger core.Logger, metrics core.MetricsCollector) *AccessController {
	ac := &AccessController{
		logger:                 logger,
		metrics:                metrics,
		sensitiveFields:        make(map[string]*regexp.Regexp),
		maskingRules:           make(map[string]MaskingRule),
		tablePermissions:       make(map[string]TablePermission),
		dataGovernancePolicies: make(map[string]DataGovernancePolicy),
	}

	// 初始化默认配置
	ac.initDefaultConfig()

	return ac
}

// initDefaultConfig 初始化默认配置
func (ac *AccessController) initDefaultConfig() {
	// 初始化敏感字段模式
	sensitivePatterns := map[string]string{
		"password":     `(?i)(password|pwd|passwd|pass)`,
		"email":        `(?i)(email|mail|e_mail)`,
		"phone":        `(?i)(phone|mobile|tel|telephone)`,
		"id_card":      `(?i)(id_card|identity|id_number|card_no)`,
		"credit_card":  `(?i)(credit_card|card_number|card_no)`,
		"bank_account": `(?i)(bank_account|account_no|account_number)`,
		"address":      `(?i)(address|addr|location)`,
		"name":         `(?i)(real_name|full_name|user_name|username)`,
		"ip_address":   `(?i)(ip_address|ip_addr|client_ip)`,
		"token":        `(?i)(token|access_token|refresh_token|api_key)`,
	}

	for key, pattern := range sensitivePatterns {
		if regex, err := regexp.Compile(pattern); err == nil {
			ac.sensitiveFields[key] = regex
		}
	}

	// 初始化脱敏规则
	ac.maskingRules = map[string]MaskingRule{
		"password": {
			Type:        "mask",
			Pattern:     ".*",
			Replacement: "******",
		},
		"email": {
			Type:        "partial",
			Pattern:     `^(.{1,3}).*@(.*)$`,
			Replacement: "$1***@$2",
		},
		"phone": {
			Type:        "partial",
			Pattern:     `^(\d{3})\d{4}(\d{4})$`,
			Replacement: "$1****$2",
		},
		"id_card": {
			Type:        "partial",
			Pattern:     `^(\d{6})\d{8}(\d{4})$`,
			Replacement: "$1********$2",
		},
		"credit_card": {
			Type:        "partial",
			Pattern:     `^(\d{4})\d{8}(\d{4})$`,
			Replacement: "$1********$2",
		},
		"bank_account": {
			Type:        "mask",
			Pattern:     ".*",
			Replacement: "****",
		},
		"address": {
			Type:        "partial",
			Pattern:     `^(.{1,10}).*$`,
			Replacement: "$1***",
		},
		"ip_address": {
			Type:        "partial",
			Pattern:     `^(\d+\.\d+)\.\d+\.\d+$`,
			Replacement: "$1.*.*",
		},
		"token": {
			Type:        "mask",
			Pattern:     ".*",
			Replacement: "***",
		},
	}

	// 初始化表权限配置示例
	ac.tablePermissions = map[string]TablePermission{
		"users": {
			TableName:        "users",
			AllowedRoles:     []string{"admin", "user_manager", "user"},
			RestrictedFields: []string{"password", "email", "phone"},
			RowLevelFilter:   "", // 可以设置如 "status = 'active'"
		},
		"user_profiles": {
			TableName:        "user_profiles",
			AllowedRoles:     []string{"admin", "user_manager", "user"},
			RestrictedFields: []string{"real_name", "id_card", "address"},
			RowLevelFilter:   "",
		},
		"system_logs": {
			TableName:        "system_logs",
			AllowedRoles:     []string{"admin", "system_manager"},
			RestrictedFields: []string{"ip_address", "user_agent"},
			RowLevelFilter:   "",
		},
	}
}

// CheckTableAccess 检查表访问权限
func (ac *AccessController) CheckTableAccess(ctx context.Context, user *core.UserInfo, tableName string, operation string) error {
	// 记录检查开始时间
	startTime := time.Now()
	defer func() {
		ac.metrics.RecordHistogram("access_control_table_check_duration",
			float64(time.Since(startTime).Milliseconds()),
			map[string]string{"table": tableName, "operation": operation})
	}()

	if user == nil {
		ac.metrics.IncrementCounter("access_control_table_check_errors",
			map[string]string{"error": "nil_user", "table": tableName, "operation": operation})
		return fmt.Errorf("用户信息不能为空")
	}

	// 检查表权限配置
	tablePermission, exists := ac.tablePermissions[tableName]
	if !exists {
		// 如果没有配置，默认允许所有用户访问
		ac.metrics.IncrementCounter("access_control_table_check_success",
			map[string]string{"user_id": user.ID, "table": tableName, "operation": operation, "method": "no_config"})
		return nil
	}

	// 检查用户角色是否在允许列表中
	hasAccess := false
	for _, userRole := range user.Roles {
		for _, allowedRole := range tablePermission.AllowedRoles {
			if userRole == allowedRole || userRole == "admin" {
				hasAccess = true
				break
			}
		}
		if hasAccess {
			break
		}
	}

	if !hasAccess {
		ac.metrics.IncrementCounter("access_control_table_check_errors",
			map[string]string{"error": "access_denied", "user_id": user.ID, "table": tableName, "operation": operation})
		ac.logger.Warn("用户缺少表访问权限",
			"user_id", user.ID,
			"username", user.Username,
			"table", tableName,
			"operation", operation,
			"user_roles", strings.Join(user.Roles, ","),
			"allowed_roles", strings.Join(tablePermission.AllowedRoles, ","))
		return fmt.Errorf("用户 %s 没有权限访问表 %s", user.Username, tableName)
	}

	ac.metrics.IncrementCounter("access_control_table_check_success",
		map[string]string{"user_id": user.ID, "table": tableName, "operation": operation, "method": "role_based"})

	return nil
}

// FilterData 过滤查询结果数据
func (ac *AccessController) FilterData(ctx context.Context, user *core.UserInfo, tableName string, data []map[string]any) ([]map[string]any, error) {
	// 记录过滤开始时间
	startTime := time.Now()
	defer func() {
		ac.metrics.RecordHistogram("access_control_data_filter_duration",
			float64(time.Since(startTime).Milliseconds()),
			map[string]string{"table": tableName})
	}()

	if user == nil || len(data) == 0 {
		return data, nil
	}

	// 获取表权限配置
	tablePermission, exists := ac.tablePermissions[tableName]
	if !exists {
		// 没有配置则进行基本的敏感数据脱敏
		return ac.applySensitiveDataMasking(data), nil
	}

	// 检查用户是否为管理员，管理员可以看到所有数据
	isAdmin := ac.isAdmin(user)

	filteredData := make([]map[string]any, 0, len(data))

	for _, row := range data {
		// 应用行级过滤
		if !isAdmin && tablePermission.RowLevelFilter != "" {
			if !ac.applyRowLevelFilter(user, row, tablePermission.RowLevelFilter) {
				continue // 跳过不符合条件的行
			}
		}

		// 应用字段级过滤和脱敏
		filteredRow := ac.applyFieldLevelFilter(user, row, tablePermission.RestrictedFields, isAdmin)
		filteredData = append(filteredData, filteredRow)
	}

	ac.metrics.SetGauge("access_control_filtered_rows",
		float64(len(data)-len(filteredData)),
		map[string]string{"table": tableName, "user_id": user.ID})

	return filteredData, nil
}

// applyRowLevelFilter 应用行级过滤
func (ac *AccessController) applyRowLevelFilter(user *core.UserInfo, row map[string]any, filter string) bool {
	// 这里应该实现行级过滤逻辑
	// 简化实现：如果过滤条件包含用户ID，则检查该行是否属于当前用户
	if strings.Contains(filter, "user_id") {
		if userID, exists := row["user_id"]; exists {
			return fmt.Sprintf("%v", userID) == user.ID
		}
	}

	// 默认允许访问
	return true
}

// applyFieldLevelFilter 应用字段级过滤和脱敏
func (ac *AccessController) applyFieldLevelFilter(user *core.UserInfo, row map[string]any, restrictedFields []string, isAdmin bool) map[string]any {
	filteredRow := make(map[string]any)

	for key, value := range row {
		// 如果是管理员，直接复制所有字段
		if isAdmin {
			filteredRow[key] = value
			continue
		}

		// 检查是否为受限字段
		isRestricted := false
		for _, restrictedField := range restrictedFields {
			if strings.EqualFold(key, restrictedField) {
				isRestricted = true
				break
			}
		}

		if isRestricted {
			// 应用脱敏规则
			filteredRow[key] = ac.maskSensitiveData(key, value)
		} else {
			// 检查是否为敏感字段
			if ac.isSensitiveField(key) {
				filteredRow[key] = ac.maskSensitiveData(key, value)
			} else {
				filteredRow[key] = value
			}
		}
	}

	return filteredRow
}

// applySensitiveDataMasking 应用敏感数据脱敏
func (ac *AccessController) applySensitiveDataMasking(data []map[string]any) []map[string]any {
	maskedData := make([]map[string]any, len(data))

	for i, row := range data {
		maskedRow := make(map[string]any)
		for key, value := range row {
			if ac.isSensitiveField(key) {
				maskedRow[key] = ac.maskSensitiveData(key, value)
			} else {
				maskedRow[key] = value
			}
		}
		maskedData[i] = maskedRow
	}

	return maskedData
}

// isSensitiveField 检查是否为敏感字段
func (ac *AccessController) isSensitiveField(fieldName string) bool {
	for _, pattern := range ac.sensitiveFields {
		if pattern.MatchString(fieldName) {
			return true
		}
	}
	return false
}

// maskSensitiveData 脱敏敏感数据
func (ac *AccessController) maskSensitiveData(fieldName string, value any) any {
	if value == nil {
		return nil
	}

	valueStr := fmt.Sprintf("%v", value)
	if valueStr == "" {
		return value
	}

	// 查找匹配的脱敏规则
	for ruleKey, rule := range ac.maskingRules {
		if pattern, exists := ac.sensitiveFields[ruleKey]; exists {
			if pattern.MatchString(fieldName) {
				return ac.applyMaskingRule(valueStr, rule)
			}
		}
	}

	// 默认脱敏规则
	if len(valueStr) > 6 {
		return valueStr[:2] + "***" + valueStr[len(valueStr)-2:]
	} else if len(valueStr) > 2 {
		return valueStr[:1] + "***"
	}

	return "***"
}

// applyMaskingRule 应用脱敏规则
func (ac *AccessController) applyMaskingRule(value string, rule MaskingRule) string {
	switch rule.Type {
	case "mask":
		return rule.Replacement
	case "remove":
		return ""
	case "partial":
		if rule.Pattern != "" {
			if regex, err := regexp.Compile(rule.Pattern); err == nil {
				return regex.ReplaceAllString(value, rule.Replacement)
			}
		}
		return rule.Replacement
	case "hash":
		// 这里可以实现哈希脱敏
		return "hash_" + fmt.Sprintf("%x", len(value))
	default:
		return "***"
	}
}

// isAdmin 检查用户是否为管理员
func (ac *AccessController) isAdmin(user *core.UserInfo) bool {
	for _, role := range user.Roles {
		if role == "admin" || role == "administrator" || role == "super_admin" {
			return true
		}
	}
	return false
}

// AddTablePermission 添加表权限配置
func (ac *AccessController) AddTablePermission(permission TablePermission) {
	ac.tablePermissions[permission.TableName] = permission
}

// RemoveTablePermission 移除表权限配置
func (ac *AccessController) RemoveTablePermission(tableName string) {
	delete(ac.tablePermissions, tableName)
}

// AddMaskingRule 添加脱敏规则
func (ac *AccessController) AddMaskingRule(fieldPattern string, rule MaskingRule) error {
	if regex, err := regexp.Compile(fieldPattern); err == nil {
		key := fmt.Sprintf("custom_%d", len(ac.sensitiveFields))
		ac.sensitiveFields[key] = regex
		ac.maskingRules[key] = rule
		return nil
	} else {
		return fmt.Errorf("无效的字段模式: %w", err)
	}
}

// GetTablePermissions 获取所有表权限配置
func (ac *AccessController) GetTablePermissions() map[string]TablePermission {
	// 返回副本以防止外部修改
	permissions := make(map[string]TablePermission)
	for k, v := range ac.tablePermissions {
		permissions[k] = v
	}
	return permissions
}

// CheckFieldAccess 检查字段访问权限
func (ac *AccessController) CheckFieldAccess(ctx context.Context, user *core.UserInfo, tableName string, fieldName string) bool {
	if user == nil {
		return false
	}

	// 管理员可以访问所有字段
	if ac.isAdmin(user) {
		return true
	}

	// 检查表权限配置
	if tablePermission, exists := ac.tablePermissions[tableName]; exists {
		// 检查是否为受限字段
		for _, restrictedField := range tablePermission.RestrictedFields {
			if strings.EqualFold(fieldName, restrictedField) {
				return false
			}
		}
	}

	return true
}

// LogDataAccess 记录数据访问日志
func (ac *AccessController) LogDataAccess(ctx context.Context, user *core.UserInfo, tableName string, operation string, rowCount int) {
	ac.logger.Info("数据访问记录",
		"user_id", user.ID,
		"username", user.Username,
		"table", tableName,
		"operation", operation,
		"row_count", rowCount,
		"timestamp", time.Now().Format(time.RFC3339))

	ac.metrics.IncrementCounter("data_access_operations",
		map[string]string{
			"user_id":   user.ID,
			"table":     tableName,
			"operation": operation,
		})
}

// DataClassification 数据分类级别
type DataClassification string

const (
	DataClassificationPublic       DataClassification = "public"       // 公开数据
	DataClassificationInternal     DataClassification = "internal"     // 内部数据
	DataClassificationConfidential DataClassification = "confidential" // 机密数据
	DataClassificationRestricted   DataClassification = "restricted"   // 限制数据
)

// FieldClassification 字段分类配置
type FieldClassification struct {
	FieldName      string             `json:"field_name"`     // 字段名
	Classification DataClassification `json:"classification"` // 分类级别
	RequiredRoles  []string           `json:"required_roles"` // 需要的角色
	MaskingRule    *MaskingRule       `json:"masking_rule"`   // 脱敏规则
}

// DataGovernancePolicy 数据治理策略
type DataGovernancePolicy struct {
	TableName          string                `json:"table_name"`          // 表名
	Classifications    []FieldClassification `json:"classifications"`     // 字段分类
	RetentionPeriod    int                   `json:"retention_period"`    // 数据保留期（天）
	PurgeAfterDays     int                   `json:"purge_after_days"`    // 清理期限（天）
	AuditRequired      bool                  `json:"audit_required"`      // 是否需要审计
	EncryptionRequired bool                  `json:"encryption_required"` // 是否需要加密
}

// AddDataGovernancePolicy 添加数据治理策略
func (ac *AccessController) AddDataGovernancePolicy(policy DataGovernancePolicy) {
	// 这里可以将策略存储到数据库或配置文件中
	// 简化实现：存储到内存中
	if ac.dataGovernancePolicies == nil {
		ac.dataGovernancePolicies = make(map[string]DataGovernancePolicy)
	}
	ac.dataGovernancePolicies[policy.TableName] = policy
}

// GetDataGovernancePolicy 获取数据治理策略
func (ac *AccessController) GetDataGovernancePolicy(tableName string) (*DataGovernancePolicy, bool) {
	if ac.dataGovernancePolicies == nil {
		return nil, false
	}
	policy, exists := ac.dataGovernancePolicies[tableName]
	return &policy, exists
}

// ApplyDataGovernance 应用数据治理策略
func (ac *AccessController) ApplyDataGovernance(ctx context.Context, user *core.UserInfo, tableName string, data []map[string]any) ([]map[string]any, error) {
	policy, exists := ac.GetDataGovernancePolicy(tableName)
	if !exists {
		// 没有策略时使用默认处理
		return ac.FilterData(ctx, user, tableName, data)
	}

	// 检查用户是否有权限访问该表
	if err := ac.CheckTableAccess(ctx, user, tableName, "SELECT"); err != nil {
		return nil, err
	}

	filteredData := make([]map[string]any, 0, len(data))

	for _, row := range data {
		filteredRow := make(map[string]any)

		for fieldName, value := range row {
			// 查找字段分类
			classification := ac.getFieldClassification(policy, fieldName)

			// 检查用户是否有权限访问该字段
			if ac.hasFieldAccess(user, classification) {
				// 应用脱敏规则
				if classification.MaskingRule != nil && !ac.isAdmin(user) {
					filteredRow[fieldName] = ac.applyMaskingRule(fmt.Sprintf("%v", value), *classification.MaskingRule)
				} else {
					filteredRow[fieldName] = value
				}
			}
			// 如果没有权限，字段将被完全过滤掉
		}

		filteredData = append(filteredData, filteredRow)
	}

	// 记录数据治理审计日志
	if policy.AuditRequired {
		ac.logDataGovernanceAccess(ctx, user, tableName, len(data), len(filteredData))
	}

	return filteredData, nil
}

// getFieldClassification 获取字段分类
func (ac *AccessController) getFieldClassification(policy *DataGovernancePolicy, fieldName string) FieldClassification {
	for _, classification := range policy.Classifications {
		if classification.FieldName == fieldName {
			return classification
		}
	}

	// 默认分类
	return FieldClassification{
		FieldName:      fieldName,
		Classification: DataClassificationPublic,
		RequiredRoles:  []string{},
		MaskingRule:    nil,
	}
}

// hasFieldAccess 检查用户是否有字段访问权限
func (ac *AccessController) hasFieldAccess(user *core.UserInfo, classification FieldClassification) bool {
	if user == nil {
		return false
	}

	// 管理员可以访问所有字段
	if ac.isAdmin(user) {
		return true
	}

	// 如果没有角色要求，允许访问
	if len(classification.RequiredRoles) == 0 {
		return true
	}

	// 检查用户是否有必需的角色
	for _, requiredRole := range classification.RequiredRoles {
		for _, userRole := range user.Roles {
			if userRole == requiredRole {
				return true
			}
		}
	}

	return false
}

// logDataGovernanceAccess 记录数据治理访问日志
func (ac *AccessController) logDataGovernanceAccess(ctx context.Context, user *core.UserInfo, tableName string, originalCount, filteredCount int) {
	ac.logger.Info("数据治理访问记录",
		"user_id", user.ID,
		"username", user.Username,
		"table", tableName,
		"original_count", originalCount,
		"filtered_count", filteredCount,
		"filtered_ratio", float64(filteredCount)/float64(originalCount),
		"timestamp", time.Now().Format(time.RFC3339))

	ac.metrics.IncrementCounter("data_governance_access",
		map[string]string{
			"user_id": user.ID,
			"table":   tableName,
		})

	ac.metrics.SetGauge("data_governance_filter_ratio",
		float64(filteredCount)/float64(originalCount),
		map[string]string{
			"table": tableName,
		})
}

// ValidateDataRetention 验证数据保留策略
func (ac *AccessController) ValidateDataRetention(ctx context.Context, tableName string, recordDate time.Time) error {
	policy, exists := ac.GetDataGovernancePolicy(tableName)
	if !exists {
		return nil // 没有策略时不进行验证
	}

	if policy.RetentionPeriod > 0 {
		retentionDeadline := recordDate.AddDate(0, 0, policy.RetentionPeriod)
		if time.Now().After(retentionDeadline) {
			return fmt.Errorf("数据已超过保留期限，应该被清理")
		}
	}

	return nil
}

// GetExpiredRecords 获取过期记录
func (ac *AccessController) GetExpiredRecords(ctx context.Context, tableName string) ([]string, error) {
	policy, exists := ac.GetDataGovernancePolicy(tableName)
	if !exists || policy.PurgeAfterDays <= 0 {
		return []string{}, nil
	}

	// 这里应该实现查询过期记录的逻辑
	// 简化实现：返回空列表
	expiredRecords := []string{}

	ac.logger.Info("查询过期记录",
		"table", tableName,
		"purge_after_days", policy.PurgeAfterDays,
		"expired_count", len(expiredRecords))

	return expiredRecords, nil
}

// EncryptSensitiveData 加密敏感数据
func (ac *AccessController) EncryptSensitiveData(ctx context.Context, tableName string, data map[string]any) (map[string]any, error) {
	policy, exists := ac.GetDataGovernancePolicy(tableName)
	if !exists || !policy.EncryptionRequired {
		return data, nil
	}

	encryptedData := make(map[string]any)
	for key, value := range data {
		classification := ac.getFieldClassification(policy, key)

		// 对机密和限制级别的数据进行加密
		if classification.Classification == DataClassificationConfidential ||
			classification.Classification == DataClassificationRestricted {
			// 这里应该实现真正的加密逻辑
			// 简化实现：添加加密标记
			encryptedData[key] = fmt.Sprintf("encrypted:%v", value)
		} else {
			encryptedData[key] = value
		}
	}

	return encryptedData, nil
}

// DecryptSensitiveData 解密敏感数据
func (ac *AccessController) DecryptSensitiveData(ctx context.Context, user *core.UserInfo, tableName string, data map[string]any) (map[string]any, error) {
	policy, exists := ac.GetDataGovernancePolicy(tableName)
	if !exists || !policy.EncryptionRequired {
		return data, nil
	}

	// 检查用户是否有解密权限
	if !ac.isAdmin(user) {
		// 非管理员用户不能解密数据
		return data, nil
	}

	decryptedData := make(map[string]any)
	for key, value := range data {
		if valueStr, ok := value.(string); ok && strings.HasPrefix(valueStr, "encrypted:") {
			// 这里应该实现真正的解密逻辑
			// 简化实现：移除加密标记
			decryptedData[key] = strings.TrimPrefix(valueStr, "encrypted:")
		} else {
			decryptedData[key] = value
		}
	}

	return decryptedData, nil
}

// GenerateDataAccessReport 生成数据访问报告
func (ac *AccessController) GenerateDataAccessReport(ctx context.Context, startTime, endTime time.Time) (map[string]any, error) {
	// 这里应该实现从审计日志中统计数据访问情况
	// 简化实现：返回基本统计信息

	report := map[string]any{
		"report_period": map[string]string{
			"start_time": startTime.Format(time.RFC3339),
			"end_time":   endTime.Format(time.RFC3339),
		},
		"total_access_count":    0,
		"unique_users":          0,
		"most_accessed_tables":  []string{},
		"sensitive_data_access": 0,
		"policy_violations":     0,
		"data_masking_applied":  0,
	}

	ac.logger.Info("生成数据访问报告",
		"start_time", startTime.Format(time.RFC3339),
		"end_time", endTime.Format(time.RFC3339),
		"report", report)

	return report, nil
}

// 添加数据治理策略存储字段到 AccessController 结构体
// 注意：这需要在结构体定义中添加，但为了不破坏现有代码，这里用注释说明
// dataGovernancePolicies map[string]DataGovernancePolicy // 数据治理策略
