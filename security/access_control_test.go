package security

import (
	"context"
	"testing"
	"time"

	"pumppill/rag/core"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestAccessController_CheckTableAccess(t *testing.T) {
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsCollector{}

	// 设置 mock 期望
	mockLogger.On("Warn", mock.AnythingOfType("string"), mock.Anything).Return()
	mockMetrics.On("RecordHistogram", mock.AnythingOfType("string"), mock.AnythingOfType("float64"), mock.AnythingOfType("map[string]string")).Return()
	mockMetrics.On("IncrementCounter", mock.AnythingOfType("string"), mock.AnythingOfType("map[string]string")).Return()

	accessController := NewAccessController(mockLogger, mockMetrics)

	t.Run("CheckTableAccess_NilUser", func(t *testing.T) {
		err := accessController.CheckTableAccess(context.Background(), nil, "users", "SELECT")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "用户信息不能为空")
	})

	t.Run("CheckTableAccess_NoConfig", func(t *testing.T) {
		user := &core.UserInfo{
			ID:       "123",
			Username: "testuser",
			Roles:    []string{"user"},
		}
		err := accessController.CheckTableAccess(context.Background(), user, "unknown_table", "SELECT")
		assert.NoError(t, err) // 没有配置时默认允许
	})

	t.Run("CheckTableAccess_AllowedRole", func(t *testing.T) {
		user := &core.UserInfo{
			ID:       "123",
			Username: "testuser",
			Roles:    []string{"user"},
		}
		err := accessController.CheckTableAccess(context.Background(), user, "users", "SELECT")
		assert.NoError(t, err)
	})

	t.Run("CheckTableAccess_AdminRole", func(t *testing.T) {
		user := &core.UserInfo{
			ID:       "123",
			Username: "admin",
			Roles:    []string{"admin"},
		}
		err := accessController.CheckTableAccess(context.Background(), user, "users", "SELECT")
		assert.NoError(t, err)
	})

	t.Run("CheckTableAccess_DeniedRole", func(t *testing.T) {
		user := &core.UserInfo{
			ID:       "123",
			Username: "guest",
			Roles:    []string{"guest"},
		}
		err := accessController.CheckTableAccess(context.Background(), user, "users", "SELECT")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "没有权限访问表")
	})
}

func TestAccessController_FilterData(t *testing.T) {
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsCollector{}

	// 设置 mock 期望
	mockMetrics.On("RecordHistogram", mock.AnythingOfType("string"), mock.AnythingOfType("float64"), mock.AnythingOfType("map[string]string")).Return()
	mockMetrics.On("SetGauge", mock.AnythingOfType("string"), mock.AnythingOfType("float64"), mock.AnythingOfType("map[string]string")).Return()

	accessController := NewAccessController(mockLogger, mockMetrics)

	t.Run("FilterData_NilUser", func(t *testing.T) {
		data := []map[string]any{
			{"id": 1, "name": "test"},
		}
		result, err := accessController.FilterData(context.Background(), nil, "users", data)
		assert.NoError(t, err)
		assert.Equal(t, data, result)
	})

	t.Run("FilterData_EmptyData", func(t *testing.T) {
		user := &core.UserInfo{
			ID:       "123",
			Username: "testuser",
			Roles:    []string{"user"},
		}
		data := []map[string]any{}
		result, err := accessController.FilterData(context.Background(), user, "users", data)
		assert.NoError(t, err)
		assert.Equal(t, data, result)
	})

	t.Run("FilterData_AdminUser", func(t *testing.T) {
		user := &core.UserInfo{
			ID:       "123",
			Username: "admin",
			Roles:    []string{"admin"},
		}
		data := []map[string]any{
			{"id": 1, "name": "test", "password": "secret", "email": "test@example.com"},
		}
		result, err := accessController.FilterData(context.Background(), user, "users", data)
		assert.NoError(t, err)
		assert.Equal(t, data, result) // 管理员看到所有数据
	})

	t.Run("FilterData_RegularUser", func(t *testing.T) {
		user := &core.UserInfo{
			ID:       "123",
			Username: "testuser",
			Roles:    []string{"user"},
		}
		data := []map[string]any{
			{"id": 1, "name": "test", "password": "secret", "email": "test@example.com"},
		}
		result, err := accessController.FilterData(context.Background(), user, "users", data)
		assert.NoError(t, err)
		assert.Len(t, result, 1)

		// 检查敏感字段是否被脱敏
		row := result[0]
		assert.Equal(t, 1, row["id"])
		assert.Equal(t, "test", row["name"])
		assert.NotEqual(t, "secret", row["password"])        // 密码应该被脱敏
		assert.NotEqual(t, "test@example.com", row["email"]) // 邮箱应该被脱敏
	})

	t.Run("FilterData_NoTableConfig", func(t *testing.T) {
		user := &core.UserInfo{
			ID:       "123",
			Username: "testuser",
			Roles:    []string{"user"},
		}
		data := []map[string]any{
			{"id": 1, "name": "test", "password": "secret"},
		}
		result, err := accessController.FilterData(context.Background(), user, "unknown_table", data)
		assert.NoError(t, err)
		assert.Len(t, result, 1)

		// 没有表配置时应该应用基本的敏感数据脱敏
		row := result[0]
		assert.NotEqual(t, "secret", row["password"]) // 密码应该被脱敏
	})
}

func TestAccessController_MaskSensitiveData(t *testing.T) {
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsCollector{}

	accessController := NewAccessController(mockLogger, mockMetrics)

	tests := []struct {
		name      string
		fieldName string
		value     any
		expected  string
	}{
		{
			name:      "Password",
			fieldName: "password",
			value:     "secret123",
			expected:  "******",
		},
		{
			name:      "Email",
			fieldName: "email",
			value:     "test@example.com",
			expected:  "tes***@example.com",
		},
		{
			name:      "Phone",
			fieldName: "phone",
			value:     "13812345678",
			expected:  "138****5678",
		},
		{
			name:      "ID Card",
			fieldName: "id_card",
			value:     "123456789012345678",
			expected:  "123456********5678",
		},
		{
			name:      "Non-sensitive",
			fieldName: "name",
			value:     "testuser",
			expected:  "te***er", // 默认脱敏规则：前2位 + *** + 后2位
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := accessController.maskSensitiveData(tt.fieldName, tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAccessController_IsSensitiveField(t *testing.T) {
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsCollector{}

	accessController := NewAccessController(mockLogger, mockMetrics)

	tests := []struct {
		fieldName string
		expected  bool
	}{
		{"password", true},
		{"pwd", true},
		{"email", true},
		{"phone", true},
		{"mobile", true},
		{"id_card", true},
		{"credit_card", true},
		{"name", false},
		{"age", false},
		{"status", false},
	}

	for _, tt := range tests {
		t.Run(tt.fieldName, func(t *testing.T) {
			result := accessController.isSensitiveField(tt.fieldName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAccessController_CheckFieldAccess(t *testing.T) {
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsCollector{}

	accessController := NewAccessController(mockLogger, mockMetrics)

	t.Run("CheckFieldAccess_NilUser", func(t *testing.T) {
		result := accessController.CheckFieldAccess(context.Background(), nil, "users", "name")
		assert.False(t, result)
	})

	t.Run("CheckFieldAccess_AdminUser", func(t *testing.T) {
		user := &core.UserInfo{
			ID:       "123",
			Username: "admin",
			Roles:    []string{"admin"},
		}
		result := accessController.CheckFieldAccess(context.Background(), user, "users", "password")
		assert.True(t, result) // 管理员可以访问所有字段
	})

	t.Run("CheckFieldAccess_RestrictedField", func(t *testing.T) {
		user := &core.UserInfo{
			ID:       "123",
			Username: "testuser",
			Roles:    []string{"user"},
		}
		result := accessController.CheckFieldAccess(context.Background(), user, "users", "password")
		assert.False(t, result) // 普通用户不能访问受限字段
	})

	t.Run("CheckFieldAccess_AllowedField", func(t *testing.T) {
		user := &core.UserInfo{
			ID:       "123",
			Username: "testuser",
			Roles:    []string{"user"},
		}
		result := accessController.CheckFieldAccess(context.Background(), user, "users", "name")
		assert.True(t, result) // 普通用户可以访问非受限字段
	})

	t.Run("CheckFieldAccess_NoTableConfig", func(t *testing.T) {
		user := &core.UserInfo{
			ID:       "123",
			Username: "testuser",
			Roles:    []string{"user"},
		}
		result := accessController.CheckFieldAccess(context.Background(), user, "unknown_table", "any_field")
		assert.True(t, result) // 没有表配置时默认允许
	})
}

func TestAccessController_TablePermissionManagement(t *testing.T) {
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsCollector{}

	accessController := NewAccessController(mockLogger, mockMetrics)

	t.Run("AddTablePermission", func(t *testing.T) {
		permission := TablePermission{
			TableName:        "test_table",
			AllowedRoles:     []string{"user", "admin"},
			RestrictedFields: []string{"secret_field"},
			RowLevelFilter:   "status = 'active'",
		}

		accessController.AddTablePermission(permission)
		permissions := accessController.GetTablePermissions()
		assert.Contains(t, permissions, "test_table")
		assert.Equal(t, permission, permissions["test_table"])
	})

	t.Run("RemoveTablePermission", func(t *testing.T) {
		accessController.RemoveTablePermission("test_table")
		permissions := accessController.GetTablePermissions()
		assert.NotContains(t, permissions, "test_table")
	})

	t.Run("AddMaskingRule", func(t *testing.T) {
		rule := MaskingRule{
			Type:        "partial",
			Pattern:     `^(.{2}).*(.{2})$`,
			Replacement: "$1***$2",
		}

		err := accessController.AddMaskingRule("test_field_.*", rule)
		assert.NoError(t, err)
	})

	t.Run("AddMaskingRule_InvalidPattern", func(t *testing.T) {
		rule := MaskingRule{
			Type:        "partial",
			Pattern:     "[invalid",
			Replacement: "***",
		}

		err := accessController.AddMaskingRule("[invalid", rule)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "无效的字段模式")
	})
}

func TestAccessController_LogDataAccess(t *testing.T) {
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsCollector{}

	// 设置 mock 期望
	mockLogger.On("Info", mock.AnythingOfType("string"), mock.Anything).Return()
	mockMetrics.On("IncrementCounter", mock.AnythingOfType("string"), mock.AnythingOfType("map[string]string")).Return()

	accessController := NewAccessController(mockLogger, mockMetrics)

	t.Run("LogDataAccess", func(t *testing.T) {
		user := &core.UserInfo{
			ID:       "123",
			Username: "testuser",
		}

		accessController.LogDataAccess(context.Background(), user, "users", "SELECT", 10)

		// 验证日志和指标调用
		mockLogger.AssertCalled(t, "Info", "数据访问记录", mock.Anything)
		mockMetrics.AssertCalled(t, "IncrementCounter", "data_access_operations", mock.AnythingOfType("map[string]string"))
	})
}
func TestAccessController_DataGovernance(t *testing.T) {
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsCollector{}

	// 设置 mock 期望
	mockLogger.On("Info", mock.AnythingOfType("string"), mock.Anything).Return()
	mockMetrics.On("RecordHistogram", mock.AnythingOfType("string"), mock.AnythingOfType("float64"), mock.AnythingOfType("map[string]string")).Return()
	mockMetrics.On("IncrementCounter", mock.AnythingOfType("string"), mock.AnythingOfType("map[string]string")).Return()
	mockMetrics.On("SetGauge", mock.AnythingOfType("string"), mock.AnythingOfType("float64"), mock.AnythingOfType("map[string]string")).Return()

	accessController := NewAccessController(mockLogger, mockMetrics)

	t.Run("AddAndGetDataGovernancePolicy", func(t *testing.T) {
		policy := DataGovernancePolicy{
			TableName: "sensitive_table",
			Classifications: []FieldClassification{
				{
					FieldName:      "id",
					Classification: DataClassificationPublic,
					RequiredRoles:  []string{},
				},
				{
					FieldName:      "name",
					Classification: DataClassificationInternal,
					RequiredRoles:  []string{"user", "admin"},
				},
				{
					FieldName:      "ssn",
					Classification: DataClassificationRestricted,
					RequiredRoles:  []string{"admin"},
					MaskingRule: &MaskingRule{
						Type:        "mask",
						Replacement: "***-**-****",
					},
				},
			},
			RetentionPeriod:    365,
			PurgeAfterDays:     400,
			AuditRequired:      true,
			EncryptionRequired: true,
		}

		accessController.AddDataGovernancePolicy(policy)

		retrievedPolicy, exists := accessController.GetDataGovernancePolicy("sensitive_table")
		assert.True(t, exists)
		assert.Equal(t, policy.TableName, retrievedPolicy.TableName)
		assert.Len(t, retrievedPolicy.Classifications, 3)
		assert.True(t, retrievedPolicy.AuditRequired)
		assert.True(t, retrievedPolicy.EncryptionRequired)
	})

	t.Run("ApplyDataGovernance_AdminUser", func(t *testing.T) {
		user := &core.UserInfo{
			ID:       "123",
			Username: "admin",
			Roles:    []string{"admin"},
		}

		data := []map[string]any{
			{
				"id":   1,
				"name": "John Doe",
				"ssn":  "123-45-6789",
			},
		}

		result, err := accessController.ApplyDataGovernance(context.Background(), user, "sensitive_table", data)
		assert.NoError(t, err)
		assert.Len(t, result, 1)

		// 管理员应该能看到所有字段
		row := result[0]
		assert.Equal(t, 1, row["id"])
		assert.Equal(t, "John Doe", row["name"])
		assert.Equal(t, "123-45-6789", row["ssn"]) // 管理员不应用脱敏
	})

	t.Run("ApplyDataGovernance_RegularUser", func(t *testing.T) {
		user := &core.UserInfo{
			ID:       "456",
			Username: "user",
			Roles:    []string{"user"},
		}

		data := []map[string]any{
			{
				"id":   1,
				"name": "John Doe",
				"ssn":  "123-45-6789",
			},
		}

		result, err := accessController.ApplyDataGovernance(context.Background(), user, "sensitive_table", data)
		assert.NoError(t, err)
		assert.Len(t, result, 1)

		// 普通用户应该看到部分字段
		row := result[0]
		assert.Equal(t, 1, row["id"])            // 公开字段
		assert.Equal(t, "John Doe", row["name"]) // 内部字段，用户有权限
		assert.NotContains(t, row, "ssn")        // 限制字段，用户无权限
	})

	t.Run("ApplyDataGovernance_NoPolicy", func(t *testing.T) {
		user := &core.UserInfo{
			ID:       "789",
			Username: "guest",
			Roles:    []string{"guest"},
		}

		data := []map[string]any{
			{"id": 1, "name": "test"},
		}

		result, err := accessController.ApplyDataGovernance(context.Background(), user, "unknown_table", data)
		assert.NoError(t, err)
		assert.Len(t, result, 1)
	})
}

func TestAccessController_DataEncryption(t *testing.T) {
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsCollector{}

	accessController := NewAccessController(mockLogger, mockMetrics)

	// 添加需要加密的策略
	policy := DataGovernancePolicy{
		TableName: "encrypted_table",
		Classifications: []FieldClassification{
			{
				FieldName:      "public_field",
				Classification: DataClassificationPublic,
			},
			{
				FieldName:      "confidential_field",
				Classification: DataClassificationConfidential,
			},
		},
		EncryptionRequired: true,
	}
	accessController.AddDataGovernancePolicy(policy)

	t.Run("EncryptSensitiveData", func(t *testing.T) {
		data := map[string]any{
			"public_field":       "public_value",
			"confidential_field": "secret_value",
		}

		encrypted, err := accessController.EncryptSensitiveData(context.Background(), "encrypted_table", data)
		assert.NoError(t, err)

		assert.Equal(t, "public_value", encrypted["public_field"])
		assert.Equal(t, "encrypted:secret_value", encrypted["confidential_field"])
	})

	t.Run("DecryptSensitiveData_Admin", func(t *testing.T) {
		admin := &core.UserInfo{
			ID:       "123",
			Username: "admin",
			Roles:    []string{"admin"},
		}

		encryptedData := map[string]any{
			"public_field":       "public_value",
			"confidential_field": "encrypted:secret_value",
		}

		decrypted, err := accessController.DecryptSensitiveData(context.Background(), admin, "encrypted_table", encryptedData)
		assert.NoError(t, err)

		assert.Equal(t, "public_value", decrypted["public_field"])
		assert.Equal(t, "secret_value", decrypted["confidential_field"])
	})

	t.Run("DecryptSensitiveData_RegularUser", func(t *testing.T) {
		user := &core.UserInfo{
			ID:       "456",
			Username: "user",
			Roles:    []string{"user"},
		}

		encryptedData := map[string]any{
			"public_field":       "public_value",
			"confidential_field": "encrypted:secret_value",
		}

		// 普通用户不能解密
		decrypted, err := accessController.DecryptSensitiveData(context.Background(), user, "encrypted_table", encryptedData)
		assert.NoError(t, err)

		assert.Equal(t, "public_value", decrypted["public_field"])
		assert.Equal(t, "encrypted:secret_value", decrypted["confidential_field"]) // 保持加密状态
	})

	t.Run("EncryptDecrypt_NoPolicy", func(t *testing.T) {
		data := map[string]any{
			"field1": "value1",
			"field2": "value2",
		}

		// 没有策略时不加密
		encrypted, err := accessController.EncryptSensitiveData(context.Background(), "no_policy_table", data)
		assert.NoError(t, err)
		assert.Equal(t, data, encrypted)

		// 没有策略时不解密
		user := &core.UserInfo{ID: "123", Roles: []string{"admin"}}
		decrypted, err := accessController.DecryptSensitiveData(context.Background(), user, "no_policy_table", data)
		assert.NoError(t, err)
		assert.Equal(t, data, decrypted)
	})
}

func TestAccessController_DataRetention(t *testing.T) {
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsCollector{}

	// 设置 mock 期望
	mockLogger.On("Info", mock.AnythingOfType("string"), mock.Anything).Return()

	accessController := NewAccessController(mockLogger, mockMetrics)

	// 添加数据保留策略
	policy := DataGovernancePolicy{
		TableName:       "retention_table",
		RetentionPeriod: 30, // 30天保留期
		PurgeAfterDays:  35, // 35天后清理
	}
	accessController.AddDataGovernancePolicy(policy)

	t.Run("ValidateDataRetention_WithinPeriod", func(t *testing.T) {
		recentDate := time.Now().AddDate(0, 0, -10) // 10天前的数据
		err := accessController.ValidateDataRetention(context.Background(), "retention_table", recentDate)
		assert.NoError(t, err)
	})

	t.Run("ValidateDataRetention_Expired", func(t *testing.T) {
		oldDate := time.Now().AddDate(0, 0, -40) // 40天前的数据
		err := accessController.ValidateDataRetention(context.Background(), "retention_table", oldDate)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "数据已超过保留期限")
	})

	t.Run("ValidateDataRetention_NoPolicy", func(t *testing.T) {
		oldDate := time.Now().AddDate(0, 0, -100)
		err := accessController.ValidateDataRetention(context.Background(), "no_policy_table", oldDate)
		assert.NoError(t, err) // 没有策略时不验证
	})

	t.Run("GetExpiredRecords", func(t *testing.T) {
		records, err := accessController.GetExpiredRecords(context.Background(), "retention_table")
		assert.NoError(t, err)
		assert.NotNil(t, records)
	})
}

func TestAccessController_GenerateDataAccessReport(t *testing.T) {
	mockLogger := &MockLogger{}
	mockMetrics := &MockMetricsCollector{}

	// 设置 mock 期望
	mockLogger.On("Info", mock.AnythingOfType("string"), mock.Anything).Return()

	accessController := NewAccessController(mockLogger, mockMetrics)

	t.Run("GenerateDataAccessReport", func(t *testing.T) {
		startTime := time.Now().AddDate(0, 0, -7) // 7天前
		endTime := time.Now()

		report, err := accessController.GenerateDataAccessReport(context.Background(), startTime, endTime)
		assert.NoError(t, err)
		assert.NotNil(t, report)

		// 验证报告包含必要的字段
		assert.Contains(t, report, "report_period")
		assert.Contains(t, report, "total_access_count")
		assert.Contains(t, report, "unique_users")
		assert.Contains(t, report, "most_accessed_tables")
		assert.Contains(t, report, "sensitive_data_access")
		assert.Contains(t, report, "policy_violations")
		assert.Contains(t, report, "data_masking_applied")

		// 验证报告期间
		period := report["report_period"].(map[string]string)
		assert.Equal(t, startTime.Format(time.RFC3339), period["start_time"])
		assert.Equal(t, endTime.Format(time.RFC3339), period["end_time"])
	})
}
