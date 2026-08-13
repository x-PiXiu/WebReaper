package repository

import (
	"fmt"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"webreaper/internal/config"
)

// NewMySQLDB 创建 MySQL 连接（生产用）。
// dsn 示例："root:password@tcp(127.0.0.1:3306)/webreaper?charset=utf8mb4&parseTime=True&loc=Local"
// 连接后自动应用版本化 SQL 迁移（migrations/*.sql）。
//
// 连接池配置（防止 "bad connection"——MySQL 服务端空闲超时断开连接后，
// GORM 连接池复用了已断开的连接导致报错）：
//   - ConnMaxLifetime < MySQL 的 wait_timeout（通常 8h，设 3h 留余量）
//   - ConnMaxIdleTime 也要 < wait_timeout（空闲连接主动回收）
func NewMySQLDB(dsn string) (*gorm.DB, error) {
	var db *gorm.DB
	var err error
	// 连接重试（容器启动时 MySQL 可能还没就绪——最多重试 30s，每 2s 一次）
	for attempt := 1; attempt <= 15; attempt++ {
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Warn),
		})
		if err == nil {
			sqlDB, _ := db.DB()
			if pingErr := sqlDB.Ping(); pingErr == nil {
				break
			} else {
				err = pingErr
			}
		}
		if attempt < 15 {
			fmt.Printf("[db] MySQL 连接重试 %d/15（%v），2s 后重试...\n", attempt, err)
			time.Sleep(2 * time.Second)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("MySQL 连接失败（重试 30s）: %w", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(3 * time.Hour)
	sqlDB.SetConnMaxIdleTime(30 * time.Minute)
	if err := applyMigrations(db); err != nil {
		return nil, fmt.Errorf("apply migrations: %w", err)
	}
	return db, nil
}

// NewMySQLDBFromConfig 从 Config 对象创建 MySQL 连接。
// 推荐使用此函数：DSN 由 DBConfig.DSN() 构造，避免手工拼接。
func NewMySQLDBFromConfig(cfg config.DBConfig) (*gorm.DB, error) {
	return NewMySQLDB(cfg.DSN())
}

// NewSQLiteDB 创建 SQLite 连接（测试/开发用）。
// path 为 ":memory:" 时用内存库，进程结束即销毁，适合测试。
func NewSQLiteDB(path string) (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := autoMigrate(db); err != nil {
		return nil, fmt.Errorf("auto migrate: %w", err)
	}
	return db, nil
}

// autoMigrate 自动建表/补字段。
//
// 生产环境改用 RunMigrations（基于版本化 SQL 文件，迁移可控）。
// SQLite（测试用）不支持部分迁移 SQL 语法，仍用 AutoMigrate。
func autoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(allModels()...)
}

// applyMigrations 应用版本化 SQL 迁移（生产环境用）。
// 对应 internal/adapter/repository/migrations/*.sql。
func applyMigrations(db *gorm.DB) error {
	return RunMigrations(db)
}
