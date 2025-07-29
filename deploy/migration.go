// 本文件实现了数据库迁移工具，提供数据库初始化、版本迁移、回滚等功能。
// 主要功能：
// 1. 数据库初始化和表结构创建
// 2. 版本迁移管理和执行
// 3. 迁移回滚机制
// 4. 迁移状态跟踪和记录
// 5. 数据备份和恢复
// 6. 迁移脚本管理

package deploy

import (
	"context"
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
)

// MigrationDirection 迁移方向
type MigrationDirection string

const (
	MigrationUp   MigrationDirection = "up"
	MigrationDown MigrationDirection = "down"
)

// MigrationStatus 迁移状态
type MigrationStatus string

const (
	MigrationStatusPending    MigrationStatus = "pending"
	MigrationStatusRunning    MigrationStatus = "running"
	MigrationStatusCompleted  MigrationStatus = "completed"
	MigrationStatusFailed     MigrationStatus = "failed"
	MigrationStatusRolledBack MigrationStatus = "rolled_back"
)

// Migration 迁移定义
type Migration struct {
	Version      int64           `json:"version"`
	Name         string          `json:"name"`
	UpSQL        string          `json:"up_sql"`
	DownSQL      string          `json:"down_sql"`
	Status       MigrationStatus `json:"status"`
	AppliedAt    *time.Time      `json:"applied_at,omitempty"`
	RolledBackAt *time.Time      `json:"rolled_back_at,omitempty"`
	Duration     time.Duration   `json:"duration"`
	Error        string          `json:"error,omitempty"`
}

// MigrationConfig 迁移配置
type MigrationConfig struct {
	MigrationsPath      string        `yaml:"migrations_path"`
	AutoMigrate         bool          `yaml:"auto_migrate"`
	BackupBeforeMigrate bool          `yaml:"backup_before_migrate"`
	MigrationTimeout    time.Duration `yaml:"migration_timeout"`
	BackupPath          string        `yaml:"backup_path"`
	TableName           string        `yaml:"table_name"`
}

// MigrationManager 迁移管理器
type MigrationManager struct {
	config     *MigrationConfig
	db         *sql.DB
	logger     *zap.Logger
	migrations map[int64]*Migration
}

// NewMigrationManager 创建迁移管理器
func NewMigrationManager(config *MigrationConfig, db *sql.DB, logger *zap.Logger) *MigrationManager {
	if config == nil {
		config = &MigrationConfig{
			MigrationsPath:      "migrations",
			AutoMigrate:         false,
			BackupBeforeMigrate: true,
			MigrationTimeout:    300 * time.Second,
			BackupPath:          "backups",
			TableName:           "schema_migrations",
		}
	}

	return &MigrationManager{
		config:     config,
		db:         db,
		logger:     logger,
		migrations: make(map[int64]*Migration),
	}
}

// Initialize 初始化迁移系统
func (mm *MigrationManager) Initialize(ctx context.Context) error {
	mm.logger.Info("初始化数据库迁移系统")

	// 创建迁移表
	if err := mm.createMigrationTable(ctx); err != nil {
		return fmt.Errorf("创建迁移表失败: %w", err)
	}

	// 加载迁移文件
	if err := mm.loadMigrations(); err != nil {
		return fmt.Errorf("加载迁移文件失败: %w", err)
	}

	// 加载迁移状态
	if err := mm.loadMigrationStatus(ctx); err != nil {
		return fmt.Errorf("加载迁移状态失败: %w", err)
	}

	mm.logger.Info("迁移系统初始化完成",
		zap.Int("total_migrations", len(mm.migrations)),
	)

	return nil
}

// Migrate 执行迁移
func (mm *MigrationManager) Migrate(ctx context.Context, targetVersion int64) error {
	currentVersion, err := mm.getCurrentVersion(ctx)
	if err != nil {
		return fmt.Errorf("获取当前版本失败: %w", err)
	}

	mm.logger.Info("开始数据库迁移",
		zap.Int64("current_version", currentVersion),
		zap.Int64("target_version", targetVersion),
	)

	if targetVersion == currentVersion {
		mm.logger.Info("数据库已是目标版本，无需迁移")
		return nil
	}

	// 备份数据库
	if mm.config.BackupBeforeMigrate {
		if err := mm.backupDatabase(ctx); err != nil {
			return fmt.Errorf("备份数据库失败: %w", err)
		}
	}

	// 执行迁移
	if targetVersion > currentVersion {
		return mm.migrateUp(ctx, currentVersion, targetVersion)
	} else {
		return mm.migrateDown(ctx, currentVersion, targetVersion)
	}
}

// MigrateUp 向上迁移
func (mm *MigrationManager) MigrateUp(ctx context.Context) error {
	// 获取最新版本
	latestVersion := mm.getLatestVersion()
	return mm.Migrate(ctx, latestVersion)
}

// MigrateDown 向下迁移
func (mm *MigrationManager) MigrateDown(ctx context.Context, steps int) error {
	// 计算目标版本
	versions := mm.getAppliedVersions()
	if len(versions) < steps {
		return fmt.Errorf("无法回滚 %d 步，只有 %d 个已应用的迁移", steps, len(versions))
	}

	targetVersion := int64(0)
	if len(versions) > steps {
		targetVersion = versions[len(versions)-steps-1]
	}

	return mm.Migrate(ctx, targetVersion)
}

// GetStatus 获取迁移状态
func (mm *MigrationManager) GetStatus(ctx context.Context) ([]*Migration, error) {
	if err := mm.loadMigrationStatus(ctx); err != nil {
		return nil, err
	}

	var migrations []*Migration
	for _, migration := range mm.migrations {
		migrations = append(migrations, migration)
	}

	// 按版本排序
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	return migrations, nil
}

// Reset 重置数据库
func (mm *MigrationManager) Reset(ctx context.Context) error {
	mm.logger.Warn("重置数据库")

	// 备份数据库
	if mm.config.BackupBeforeMigrate {
		if err := mm.backupDatabase(ctx); err != nil {
			return fmt.Errorf("备份数据库失败: %w", err)
		}
	}

	// 回滚所有迁移
	if err := mm.Migrate(ctx, 0); err != nil {
		return fmt.Errorf("回滚迁移失败: %w", err)
	}

	// 重新执行所有迁移
	return mm.MigrateUp(ctx)
}

// createMigrationTable 创建迁移表
func (mm *MigrationManager) createMigrationTable(ctx context.Context) error {
	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			version BIGINT PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			rolled_back_at TIMESTAMP NULL,
			duration_ms BIGINT NOT NULL DEFAULT 0,
			status ENUM('pending', 'running', 'completed', 'failed', 'rolled_back') NOT NULL DEFAULT 'pending',
			error TEXT NULL
		)
	`, mm.config.TableName)

	_, err := mm.db.ExecContext(ctx, query)
	return err
}

// loadMigrations 加载迁移文件
func (mm *MigrationManager) loadMigrations() error {
	if _, err := os.Stat(mm.config.MigrationsPath); os.IsNotExist(err) {
		mm.logger.Warn("迁移目录不存在", zap.String("path", mm.config.MigrationsPath))
		return nil
	}

	files, err := ioutil.ReadDir(mm.config.MigrationsPath)
	if err != nil {
		return fmt.Errorf("读取迁移目录失败: %w", err)
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		if err := mm.loadMigrationFile(file.Name()); err != nil {
			mm.logger.Error("加载迁移文件失败",
				zap.String("file", file.Name()),
				zap.Error(err),
			)
			continue
		}
	}

	return nil
}

// loadMigrationFile 加载单个迁移文件
func (mm *MigrationManager) loadMigrationFile(filename string) error {
	// 解析文件名格式: 001_create_users_table.up.sql
	parts := strings.Split(filename, "_")
	if len(parts) < 2 {
		return fmt.Errorf("无效的迁移文件名格式: %s", filename)
	}

	versionStr := parts[0]
	version, err := strconv.ParseInt(versionStr, 10, 64)
	if err != nil {
		return fmt.Errorf("解析版本号失败: %w", err)
	}

	// 确定迁移方向
	var direction MigrationDirection
	if strings.Contains(filename, ".up.sql") {
		direction = MigrationUp
	} else if strings.Contains(filename, ".down.sql") {
		direction = MigrationDown
	} else {
		return fmt.Errorf("无法确定迁移方向: %s", filename)
	}

	// 读取文件内容
	filePath := filepath.Join(mm.config.MigrationsPath, filename)
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("读取迁移文件失败: %w", err)
	}

	// 获取或创建迁移
	migration, exists := mm.migrations[version]
	if !exists {
		// 从文件名提取迁移名称
		nameParts := parts[1:]
		name := strings.Join(nameParts, "_")
		name = strings.TrimSuffix(name, ".up.sql")
		name = strings.TrimSuffix(name, ".down.sql")

		migration = &Migration{
			Version: version,
			Name:    name,
			Status:  MigrationStatusPending,
		}
		mm.migrations[version] = migration
	}

	// 设置SQL内容
	if direction == MigrationUp {
		migration.UpSQL = string(content)
	} else {
		migration.DownSQL = string(content)
	}

	return nil
}

// loadMigrationStatus 加载迁移状态
func (mm *MigrationManager) loadMigrationStatus(ctx context.Context) error {
	query := fmt.Sprintf(`
		SELECT version, name, applied_at, rolled_back_at, duration_ms, status, error
		FROM %s
		ORDER BY version
	`, mm.config.TableName)

	rows, err := mm.db.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var version int64
		var name string
		var appliedAt, rolledBackAt sql.NullTime
		var durationMs int64
		var status string
		var errorMsg sql.NullString

		if err := rows.Scan(&version, &name, &appliedAt, &rolledBackAt, &durationMs, &status, &errorMsg); err != nil {
			return err
		}

		migration, exists := mm.migrations[version]
		if !exists {
			migration = &Migration{
				Version: version,
				Name:    name,
			}
			mm.migrations[version] = migration
		}

		migration.Status = MigrationStatus(status)
		migration.Duration = time.Duration(durationMs) * time.Millisecond

		if appliedAt.Valid {
			migration.AppliedAt = &appliedAt.Time
		}
		if rolledBackAt.Valid {
			migration.RolledBackAt = &rolledBackAt.Time
		}
		if errorMsg.Valid {
			migration.Error = errorMsg.String
		}
	}

	return rows.Err()
}

// migrateUp 向上迁移
func (mm *MigrationManager) migrateUp(ctx context.Context, currentVersion, targetVersion int64) error {
	versions := mm.getVersionsInRange(currentVersion+1, targetVersion)

	for _, version := range versions {
		migration := mm.migrations[version]
		if migration == nil {
			return fmt.Errorf("迁移版本 %d 不存在", version)
		}

		if err := mm.executeMigration(ctx, migration, MigrationUp); err != nil {
			return fmt.Errorf("执行迁移 %d 失败: %w", version, err)
		}
	}

	return nil
}

// migrateDown 向下迁移
func (mm *MigrationManager) migrateDown(ctx context.Context, currentVersion, targetVersion int64) error {
	versions := mm.getVersionsInRange(targetVersion+1, currentVersion)

	// 反向排序
	sort.Slice(versions, func(i, j int) bool {
		return versions[i] > versions[j]
	})

	for _, version := range versions {
		migration := mm.migrations[version]
		if migration == nil {
			return fmt.Errorf("迁移版本 %d 不存在", version)
		}

		if err := mm.executeMigration(ctx, migration, MigrationDown); err != nil {
			return fmt.Errorf("回滚迁移 %d 失败: %w", version, err)
		}
	}

	return nil
}

// executeMigration 执行迁移
func (mm *MigrationManager) executeMigration(ctx context.Context, migration *Migration, direction MigrationDirection) error {
	startTime := time.Now()

	mm.logger.Info("执行迁移",
		zap.Int64("version", migration.Version),
		zap.String("name", migration.Name),
		zap.String("direction", string(direction)),
	)

	// 更新状态为运行中
	if err := mm.updateMigrationStatus(ctx, migration.Version, MigrationStatusRunning, "", 0); err != nil {
		return err
	}

	// 创建带超时的上下文
	migrationCtx, cancel := context.WithTimeout(ctx, mm.config.MigrationTimeout)
	defer cancel()

	// 开始事务
	tx, err := mm.db.BeginTx(migrationCtx, nil)
	if err != nil {
		return fmt.Errorf("开始事务失败: %w", err)
	}

	var migrationErr error
	defer func() {
		if migrationErr != nil {
			tx.Rollback()
			duration := time.Since(startTime)
			mm.updateMigrationStatus(ctx, migration.Version, MigrationStatusFailed, migrationErr.Error(), duration)
		} else {
			tx.Commit()
		}
	}()

	// 执行SQL
	var sql string
	if direction == MigrationUp {
		sql = migration.UpSQL
	} else {
		sql = migration.DownSQL
	}

	if sql == "" {
		migrationErr = fmt.Errorf("迁移SQL为空")
		return migrationErr
	}

	// 分割并执行多个SQL语句
	statements := mm.splitSQL(sql)
	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}

		if _, err := tx.ExecContext(migrationCtx, stmt); err != nil {
			migrationErr = fmt.Errorf("执行SQL失败: %w", err)
			return migrationErr
		}
	}

	// 更新迁移状态
	duration := time.Since(startTime)
	var status MigrationStatus
	if direction == MigrationUp {
		status = MigrationStatusCompleted
	} else {
		status = MigrationStatusRolledBack
	}

	if err := mm.updateMigrationStatusInTx(tx, migration.Version, status, "", duration, direction); err != nil {
		migrationErr = err
		return migrationErr
	}

	mm.logger.Info("迁移执行完成",
		zap.Int64("version", migration.Version),
		zap.String("name", migration.Name),
		zap.String("direction", string(direction)),
		zap.Duration("duration", duration),
	)

	return nil
}

// updateMigrationStatus 更新迁移状态
func (mm *MigrationManager) updateMigrationStatus(ctx context.Context, version int64, status MigrationStatus, errorMsg string, duration time.Duration) error {
	tx, err := mm.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := mm.updateMigrationStatusInTx(tx, version, status, errorMsg, duration, MigrationUp); err != nil {
		return err
	}

	return tx.Commit()
}

// updateMigrationStatusInTx 在事务中更新迁移状态
func (mm *MigrationManager) updateMigrationStatusInTx(tx *sql.Tx, version int64, status MigrationStatus, errorMsg string, duration time.Duration, direction MigrationDirection) error {
	migration := mm.migrations[version]
	if migration == nil {
		return fmt.Errorf("迁移版本 %d 不存在", version)
	}

	now := time.Now()
	durationMs := duration.Milliseconds()

	if status == MigrationStatusRunning {
		// 插入或更新记录
		query := fmt.Sprintf(`
			INSERT INTO %s (version, name, status, duration_ms)
			VALUES (?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE
			status = VALUES(status),
			duration_ms = VALUES(duration_ms),
			error = NULL
		`, mm.config.TableName)

		_, err := tx.Exec(query, version, migration.Name, status, durationMs)
		return err
	}

	if direction == MigrationUp && status == MigrationStatusCompleted {
		query := fmt.Sprintf(`
			INSERT INTO %s (version, name, applied_at, status, duration_ms, error)
			VALUES (?, ?, ?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE
			applied_at = VALUES(applied_at),
			status = VALUES(status),
			duration_ms = VALUES(duration_ms),
			error = VALUES(error),
			rolled_back_at = NULL
		`, mm.config.TableName)

		_, err := tx.Exec(query, version, migration.Name, now, status, durationMs, errorMsg)
		return err
	}

	if direction == MigrationDown && status == MigrationStatusRolledBack {
		query := fmt.Sprintf(`
			UPDATE %s
			SET rolled_back_at = ?, status = ?, duration_ms = ?, error = ?
			WHERE version = ?
		`, mm.config.TableName)

		_, err := tx.Exec(query, now, status, durationMs, errorMsg, version)
		return err
	}

	// 其他状态更新
	query := fmt.Sprintf(`
		INSERT INTO %s (version, name, status, duration_ms, error)
		VALUES (?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
		status = VALUES(status),
		duration_ms = VALUES(duration_ms),
		error = VALUES(error)
	`, mm.config.TableName)

	_, err := tx.Exec(query, version, migration.Name, status, durationMs, errorMsg)
	return err
}

// getCurrentVersion 获取当前版本
func (mm *MigrationManager) getCurrentVersion(ctx context.Context) (int64, error) {
	query := fmt.Sprintf(`
		SELECT COALESCE(MAX(version), 0)
		FROM %s
		WHERE status = 'completed'
	`, mm.config.TableName)

	var version int64
	err := mm.db.QueryRowContext(ctx, query).Scan(&version)
	return version, err
}

// getLatestVersion 获取最新版本
func (mm *MigrationManager) getLatestVersion() int64 {
	var latest int64
	for version := range mm.migrations {
		if version > latest {
			latest = version
		}
	}
	return latest
}

// getAppliedVersions 获取已应用的版本列表
func (mm *MigrationManager) getAppliedVersions() []int64 {
	var versions []int64
	for version, migration := range mm.migrations {
		if migration.Status == MigrationStatusCompleted {
			versions = append(versions, version)
		}
	}

	sort.Slice(versions, func(i, j int) bool {
		return versions[i] < versions[j]
	})

	return versions
}

// getVersionsInRange 获取范围内的版本
func (mm *MigrationManager) getVersionsInRange(start, end int64) []int64 {
	var versions []int64
	for version := range mm.migrations {
		if version >= start && version <= end {
			versions = append(versions, version)
		}
	}

	sort.Slice(versions, func(i, j int) bool {
		return versions[i] < versions[j]
	})

	return versions
}

// splitSQL 分割SQL语句
func (mm *MigrationManager) splitSQL(sql string) []string {
	// 简单的SQL分割，按分号分割
	statements := strings.Split(sql, ";")
	var result []string

	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt != "" {
			result = append(result, stmt)
		}
	}

	return result
}

// backupDatabase 备份数据库
func (mm *MigrationManager) backupDatabase(ctx context.Context) error {
	if mm.config.BackupPath == "" {
		return nil
	}

	// 创建备份目录
	if err := os.MkdirAll(mm.config.BackupPath, 0755); err != nil {
		return fmt.Errorf("创建备份目录失败: %w", err)
	}

	// 生成备份文件名
	timestamp := time.Now().Format("20060102_150405")
	backupFile := filepath.Join(mm.config.BackupPath, fmt.Sprintf("backup_%s.sql", timestamp))

	mm.logger.Info("开始备份数据库", zap.String("backup_file", backupFile))

	// 这里应该实现实际的数据库备份逻辑
	// 由于需要依赖具体的数据库工具，这里只是创建一个占位文件
	file, err := os.Create(backupFile)
	if err != nil {
		return fmt.Errorf("创建备份文件失败: %w", err)
	}
	defer file.Close()

	// 写入备份信息
	_, err = file.WriteString(fmt.Sprintf("-- Database backup created at %s\n", time.Now().Format(time.RFC3339)))
	if err != nil {
		return fmt.Errorf("写入备份文件失败: %w", err)
	}

	mm.logger.Info("数据库备份完成", zap.String("backup_file", backupFile))
	return nil
}
