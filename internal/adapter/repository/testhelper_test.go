package repository

import (
	"testing"

	"gorm.io/gorm"
)

// newTestDB 为 repository 测试创建一个隔离的 SQLite 内存库。
//
// 设计要点：
//   - 用 :memory: 内存库，进程结束即销毁，测试间完全隔离，零外部依赖。
//   - 复用 NewSQLiteDB 的 AutoMigrate（按 PO 标签建表），保证表结构与 PO 一致。
//   - 不 mock：直接执行真实 SQL，比 mock 更能暴露 GORM 映射/列名类 bug
//     （这正是 system_setting_repo 那个 bug 的教训——mock 测不出列名不符）。
//
// 覆盖边界（诚实界定）：
//   - ✅ 能测：PO 字段映射、GORM 结构化查询逻辑、状态流转、NotFound。
//   - ⚠️ 部分能测：原生 SQL 的列名匹配（SQLite 跑得通），但 SQLite 对保留字
//     宽容（如 key 不报错），MySQL 专属语法（ON DUPLICATE KEY 等）不支持。
//   - 保留字/MySQL 专属语法的正确性靠 code review + e2e 联调兜底。
func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := NewSQLiteDB(":memory:")
	if err != nil {
		t.Fatalf("创建测试 DB 失败: %v", err)
	}
	return db
}
