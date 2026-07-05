package repository

import (
	"fmt"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"webreaper/internal/config"
)

// NewMySQLDB 创建 MySQL 连接（生产用）。
// dsn 示例："root:password@tcp(127.0.0.1:3306)/webreaper?charset=utf8mb4&parseTime=True&loc=Local"
// 连接后自动应用版本化 SQL 迁移（migrations/*.sql）。
func NewMySQLDB(dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}
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
