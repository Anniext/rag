package deploy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// TestNewMigrationManager 测试创建迁移管理器
func TestNewMigrationManager(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	logger := zaptest.NewLogger(t)

	config := &MigrationConfig{
		MigrationsPath:      "test_migrations",
		AutoMigrate:         true,
		BackupBeforeMigrate: false,
		MigrationTimeout:    60 * time.Second,
		BackupPath:          "test_backups",
		TableName:           "test_migrations",
	}

	mm := NewMigrationManager(config, db, logger)

	assert.NotNil(t, mm)
	assert.Equal(t, config, mm.config)
	assert.Equal(t, db, mm.db)
	assert.Equal(t, logger, mm.logger)
	assert.NotNil(t, mm.migrations)
}

// TestNewMigrationManagerWithNilConfig 测试使用空配置创建迁移管理器
func TestNewMigrationManagerWithNilConfig(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	logger := zaptest.NewLogger(t)

	mm := NewMigrationManager(nil, db, logger)

	assert.NotNil(t, mm)
	assert.NotNil(t, mm.config)
	assert.Equal(t, "migrations", mm.config.MigrationsPath)
	assert.Equal(t, false, mm.config.AutoMigrate)
	assert.Equal(t, true, mm.config.BackupBeforeMigrate)
	assert.Equal(t, 300*time.Second, mm.config.MigrationTimeout)
	assert.Equal(t, "backups", mm.config.BackupPath)
	assert.Equal(t, "schema_migrations", mm.config.TableName)
}

// TestInitialize 测试初始化迁移系统
func TestInitialize(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	logger := zaptest.NewLogger(t)

	// 创建临时迁移目录
	tempDir := t.TempDir()
	migrationsDir := filepath.Join(tempDir, "migrations")
	err = os.MkdirAll(migrationsDir, 0755)
	require.NoError(t, err)

	// 创建测试迁移文件
	upSQL := "CREATE TABLE users (id INT PRIMARY KEY, name VARCHAR(255));"
	downSQL := "DROP TABLE users;"

	err = os.WriteFile(filepath.Join(migrationsDir, "001_create_users.up.sql"), []byte(upSQL), 0644)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(migrationsDir, "001_create_users.down.sql"), []byte(downSQL), 0644)
	require.NoError(t, err)

	config := &MigrationConfig{
		MigrationsPath: migrationsDir,
		TableName:      "schema_migrations",
	}

	mm := NewMigrationManager(config, db, logger)

	// 模拟创建迁移表
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS schema_migrations").
		WillReturnResult(sqlmock.NewResult(0, 0))

	// 模拟加载迁移状态
	mock.ExpectQuery("SELECT version, name, applied_at, rolled_back_at, duration_ms, status, error FROM schema_migrations").
		WillReturnRows(sqlmock.NewRows([]string{"version", "name", "applied_at", "rolled_back_at", "duration_ms", "status", "error"}))

	ctx := context.Background()
	err = mm.Initialize(ctx)
	assert.NoError(t, err)

	// 验证迁移已加载
	assert.Len(t, mm.migrations, 1)
	migration := mm.migrations[1]
	assert.NotNil(t, migration)
	assert.Equal(t, int64(1), migration.Version)
	assert.Equal(t, "create_users", migration.Name)
	assert.Equal(t, upSQL, migration.UpSQL)
	assert.Equal(t, downSQL, migration.DownSQL)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestLoadMigrationFile 测试加载迁移文件
func TestLoadMigrationFile(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	logger := zaptest.NewLogger(t)

	// 创建临时迁移目录
	tempDir := t.TempDir()
	migrationsDir := filepath.Join(tempDir, "migrations")
	err = os.MkdirAll(migrationsDir, 0755)
	require.NoError(t, err)

	config := &MigrationConfig{
		MigrationsPath: migrationsDir,
	}

	mm := NewMigrationManager(config, db, logger)

	// 创建测试迁移文件
	upSQL := "CREATE TABLE test (id INT);"
	err = os.WriteFile(filepath.Join(migrationsDir, "002_create_test.up.sql"), []byte(upSQL), 0644)
	require.NoError(t, err)

	// 测试加载 up 文件
	err = mm.loadMigrationFile("002_create_test.up.sql")
	assert.NoError(t, err)

	migration := mm.migrations[2]
	assert.NotNil(t, migration)
	assert.Equal(t, int64(2), migration.Version)
	assert.Equal(t, "create_test", migration.Name)
	assert.Equal(t, upSQL, migration.UpSQL)
	assert.Equal(t, "", migration.DownSQL)

	// 创建 down 文件
	downSQL := "DROP TABLE test;"
	err = os.WriteFile(filepath.Join(migrationsDir, "002_create_test.down.sql"), []byte(downSQL), 0644)
	require.NoError(t, err)

	// 测试加载 down 文件
	err = mm.loadMigrationFile("002_create_test.down.sql")
	assert.NoError(t, err)

	migration = mm.migrations[2]
	assert.Equal(t, upSQL, migration.UpSQL)
	assert.Equal(t, downSQL, migration.DownSQL)
}

// TestLoadMigrationFileInvalidFormat 测试加载无效格式的迁移文件
func TestLoadMigrationFileInvalidFormat(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	logger := zaptest.NewLogger(t)
	mm := NewMigrationManager(nil, db, logger)

	// 测试无效文件名
	err = mm.loadMigrationFile("invalid.sql")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "无效的迁移文件名格式")

	// 测试无效版本号
	err = mm.loadMigrationFile("abc_test.up.sql")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "解析版本号失败")

	// 测试无法确定方向
	err = mm.loadMigrationFile("001_test.sql")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "无法确定迁移方向")
}

// TestGetCurrentVersion 测试获取当前版本
func TestGetCurrentVersion(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	logger := zaptest.NewLogger(t)
	mm := NewMigrationManager(nil, db, logger)

	ctx := context.Background()

	// 模拟查询当前版本
	mock.ExpectQuery("SELECT COALESCE\\(MAX\\(version\\), 0\\) FROM schema_migrations WHERE status = 'completed'").
		WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow(5))

	version, err := mm.getCurrentVersion(ctx)
	assert.NoError(t, err)
	assert.Equal(t, int64(5), version)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestGetLatestVersion 测试获取最新版本
func TestGetLatestVersion(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	logger := zaptest.NewLogger(t)
	mm := NewMigrationManager(nil, db, logger)

	// 添加一些迁移
	mm.migrations[1] = &Migration{Version: 1}
	mm.migrations[3] = &Migration{Version: 3}
	mm.migrations[2] = &Migration{Version: 2}

	latest := mm.getLatestVersion()
	assert.Equal(t, int64(3), latest)
}

// TestGetAppliedVersions 测试获取已应用的版本列表
func TestGetAppliedVersions(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	logger := zaptest.NewLogger(t)
	mm := NewMigrationManager(nil, db, logger)

	// 添加一些迁移
	mm.migrations[1] = &Migration{Version: 1, Status: MigrationStatusCompleted}
	mm.migrations[2] = &Migration{Version: 2, Status: MigrationStatusPending}
	mm.migrations[3] = &Migration{Version: 3, Status: MigrationStatusCompleted}

	versions := mm.getAppliedVersions()
	assert.Equal(t, []int64{1, 3}, versions)
}

// TestGetVersionsInRange 测试获取范围内的版本
func TestGetVersionsInRange(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	logger := zaptest.NewLogger(t)
	mm := NewMigrationManager(nil, db, logger)

	// 添加一些迁移
	mm.migrations[1] = &Migration{Version: 1}
	mm.migrations[2] = &Migration{Version: 2}
	mm.migrations[3] = &Migration{Version: 3}
	mm.migrations[5] = &Migration{Version: 5}

	versions := mm.getVersionsInRange(2, 4)
	assert.Equal(t, []int64{2, 3}, versions)
}

// TestSplitSQL 测试分割SQL语句
func TestSplitSQL(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	logger := zaptest.NewLogger(t)
	mm := NewMigrationManager(nil, db, logger)

	sql := `
		CREATE TABLE users (id INT);
		CREATE TABLE posts (id INT);
		
		INSERT INTO users VALUES (1);
	`

	statements := mm.splitSQL(sql)
	expected := []string{
		"CREATE TABLE users (id INT)",
		"CREATE TABLE posts (id INT)",
		"INSERT INTO users VALUES (1)",
	}

	assert.Equal(t, expected, statements)
}

// TestMigrateUp 测试向上迁移
func TestMigrateUp(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	logger := zaptest.NewLogger(t)
	config := &MigrationConfig{
		BackupBeforeMigrate: false,
		TableName:           "schema_migrations",
	}
	mm := NewMigrationManager(config, db, logger)

	// 添加迁移
	mm.migrations[1] = &Migration{
		Version: 1,
		Name:    "create_users",
		UpSQL:   "CREATE TABLE users (id INT);",
		Status:  MigrationStatusPending,
	}

	ctx := context.Background()

	// 模拟获取当前版本
	mock.ExpectQuery("SELECT COALESCE\\(MAX\\(version\\), 0\\) FROM schema_migrations WHERE status = 'completed'").
		WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow(0))

	// 模拟更新状态为运行中
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO schema_migrations").
		WithArgs(1, "create_users", MigrationStatusRunning, int64(0)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	// 模拟执行迁移
	mock.ExpectBegin()
	mock.ExpectExec("CREATE TABLE users \\(id INT\\)").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO schema_migrations").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err = mm.Migrate(ctx, 1)
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestMigrateDown 测试向下迁移
func TestMigrateDown(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	logger := zaptest.NewLogger(t)
	config := &MigrationConfig{
		BackupBeforeMigrate: false,
		TableName:           "schema_migrations",
	}
	mm := NewMigrationManager(config, db, logger)

	// 添加迁移
	mm.migrations[1] = &Migration{
		Version: 1,
		Name:    "create_users",
		DownSQL: "DROP TABLE users;",
		Status:  MigrationStatusCompleted,
	}

	ctx := context.Background()

	// 模拟获取当前版本
	mock.ExpectQuery("SELECT COALESCE\\(MAX\\(version\\), 0\\) FROM schema_migrations WHERE status = 'completed'").
		WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow(1))

	// 模拟更新状态为运行中
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO schema_migrations").
		WithArgs(1, "create_users", MigrationStatusRunning, int64(0)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	// 模拟执行迁移
	mock.ExpectBegin()
	mock.ExpectExec("DROP TABLE users").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("UPDATE schema_migrations").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err = mm.Migrate(ctx, 0)
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestMigrateSameVersion 测试迁移到相同版本
func TestMigrateSameVersion(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	logger := zaptest.NewLogger(t)
	mm := NewMigrationManager(nil, db, logger)

	ctx := context.Background()

	// 模拟获取当前版本
	mock.ExpectQuery("SELECT COALESCE\\(MAX\\(version\\), 0\\) FROM schema_migrations WHERE status = 'completed'").
		WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow(5))

	err = mm.Migrate(ctx, 5)
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestMigrationFailure 测试迁移失败
func TestMigrationFailure(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	logger := zaptest.NewLogger(t)
	config := &MigrationConfig{
		BackupBeforeMigrate: false,
		TableName:           "schema_migrations",
	}
	mm := NewMigrationManager(config, db, logger)

	// 添加迁移
	mm.migrations[1] = &Migration{
		Version: 1,
		Name:    "create_users",
		UpSQL:   "INVALID SQL;",
		Status:  MigrationStatusPending,
	}

	ctx := context.Background()

	// 模拟获取当前版本
	mock.ExpectQuery("SELECT COALESCE\\(MAX\\(version\\), 0\\) FROM schema_migrations WHERE status = 'completed'").
		WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow(0))

	// 模拟更新状态为运行中
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO schema_migrations").
		WithArgs(1, "create_users", MigrationStatusRunning, int64(0)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	// 模拟执行迁移失败
	mock.ExpectBegin()
	mock.ExpectExec("INVALID SQL").
		WillReturnError(fmt.Errorf("SQL语法错误"))
	mock.ExpectRollback()

	// 模拟更新失败状态
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO schema_migrations").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err = mm.Migrate(ctx, 1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "执行迁移 1 失败")

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestGetStatus 测试获取迁移状态
func TestGetStatus(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	logger := zaptest.NewLogger(t)
	mm := NewMigrationManager(nil, db, logger)

	// 添加一些迁移
	mm.migrations[1] = &Migration{Version: 1, Name: "create_users"}
	mm.migrations[2] = &Migration{Version: 2, Name: "create_posts"}

	ctx := context.Background()

	// 模拟查询迁移状态
	rows := sqlmock.NewRows([]string{"version", "name", "applied_at", "rolled_back_at", "duration_ms", "status", "error"}).
		AddRow(1, "create_users", time.Now(), nil, 1000, "completed", nil).
		AddRow(2, "create_posts", nil, nil, 0, "pending", nil)

	mock.ExpectQuery("SELECT version, name, applied_at, rolled_back_at, duration_ms, status, error FROM schema_migrations").
		WillReturnRows(rows)

	migrations, err := mm.GetStatus(ctx)
	assert.NoError(t, err)
	assert.Len(t, migrations, 2)

	// 验证排序
	assert.Equal(t, int64(1), migrations[0].Version)
	assert.Equal(t, int64(2), migrations[1].Version)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestBackupDatabase 测试备份数据库
func TestBackupDatabase(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	logger := zaptest.NewLogger(t)

	// 创建临时备份目录
	tempDir := t.TempDir()
	config := &MigrationConfig{
		BackupPath: tempDir,
	}

	mm := NewMigrationManager(config, db, logger)

	ctx := context.Background()
	err = mm.backupDatabase(ctx)
	assert.NoError(t, err)

	// 验证备份文件是否创建
	files, err := os.ReadDir(tempDir)
	assert.NoError(t, err)
	assert.Len(t, files, 1)
	assert.Contains(t, files[0].Name(), "backup_")
	assert.Contains(t, files[0].Name(), ".sql")
}

// TestBackupDatabaseNoPath 测试没有备份路径时的备份
func TestBackupDatabaseNoPath(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	logger := zaptest.NewLogger(t)
	config := &MigrationConfig{
		BackupPath: "",
	}

	mm := NewMigrationManager(config, db, logger)

	ctx := context.Background()
	err = mm.backupDatabase(ctx)
	assert.NoError(t, err) // 应该成功但不做任何事
}

// TestMigrateUpToLatest 测试迁移到最新版本
func TestMigrateUpToLatest(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	logger := zaptest.NewLogger(t)
	config := &MigrationConfig{
		BackupBeforeMigrate: false,
		TableName:           "schema_migrations",
	}
	mm := NewMigrationManager(config, db, logger)

	// 添加迁移
	mm.migrations[1] = &Migration{
		Version: 1,
		Name:    "create_users",
		UpSQL:   "CREATE TABLE users (id INT);",
		Status:  MigrationStatusPending,
	}
	mm.migrations[2] = &Migration{
		Version: 2,
		Name:    "create_posts",
		UpSQL:   "CREATE TABLE posts (id INT);",
		Status:  MigrationStatusPending,
	}

	ctx := context.Background()

	// 模拟获取当前版本
	mock.ExpectQuery("SELECT COALESCE\\(MAX\\(version\\), 0\\) FROM schema_migrations WHERE status = 'completed'").
		WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow(0))

	// 模拟执行第一个迁移
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO schema_migrations").
		WithArgs(1, "create_users", MigrationStatusRunning, int64(0)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	mock.ExpectBegin()
	mock.ExpectExec("CREATE TABLE users").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO schema_migrations").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	// 模拟执行第二个迁移
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO schema_migrations").
		WithArgs(2, "create_posts", MigrationStatusRunning, int64(0)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	mock.ExpectBegin()
	mock.ExpectExec("CREATE TABLE posts").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO schema_migrations").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err = mm.MigrateUp(ctx)
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestMigrateDownSteps 测试按步数向下迁移
func TestMigrateDownSteps(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	logger := zaptest.NewLogger(t)
	config := &MigrationConfig{
		BackupBeforeMigrate: false,
		TableName:           "schema_migrations",
	}
	mm := NewMigrationManager(config, db, logger)

	// 添加迁移
	mm.migrations[1] = &Migration{Version: 1, Status: MigrationStatusCompleted}
	mm.migrations[2] = &Migration{Version: 2, Status: MigrationStatusCompleted}
	mm.migrations[3] = &Migration{Version: 3, Status: MigrationStatusCompleted}

	ctx := context.Background()

	// 测试回滚步数过多（不需要数据库查询）
	err = mm.MigrateDown(ctx, 5)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "无法回滚 5 步")

	assert.NoError(t, mock.ExpectationsWereMet())
}

// BenchmarkMigrationOperations 基准测试迁移操作
func BenchmarkMigrationOperations(b *testing.B) {
	db, _, err := sqlmock.New()
	require.NoError(b, err)
	defer db.Close()

	logger := zaptest.NewLogger(b)
	mm := NewMigrationManager(nil, db, logger)

	// 添加一些迁移
	for i := 1; i <= 100; i++ {
		mm.migrations[int64(i)] = &Migration{
			Version: int64(i),
			Name:    fmt.Sprintf("migration_%d", i),
			Status:  MigrationStatusCompleted,
		}
	}

	b.ResetTimer()

	b.Run("GetLatestVersion", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = mm.getLatestVersion()
		}
	})

	b.Run("GetAppliedVersions", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = mm.getAppliedVersions()
		}
	})

	b.Run("GetVersionsInRange", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = mm.getVersionsInRange(10, 50)
		}
	})

	b.Run("SplitSQL", func(b *testing.B) {
		sql := "CREATE TABLE test1 (id INT); CREATE TABLE test2 (id INT); INSERT INTO test1 VALUES (1);"
		for i := 0; i < b.N; i++ {
			_ = mm.splitSQL(sql)
		}
	})
}
