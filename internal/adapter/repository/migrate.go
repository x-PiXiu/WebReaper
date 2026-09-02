package repository

import (
	"context"
	"embed"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strings"

	"gorm.io/gorm"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// migrationFile 表示一个待执行的迁移文件。
type migrationFile struct {
	version string // 文件名前缀，如 "001"
	name    string // 完整文件名，如 "001_init.sql"
	content string // SQL 内容
}

// migrationsTableName 是 WebReaper 自己的迁移记录表名。
// 用 webreaper_ 前缀避免与同库的其他项目（如 AgentCore）的 schema_migrations 冲突。
const migrationsTableName = "webreaper_schema_migrations"

// RunMigrations 执行所有未应用的数据库迁移。
//
// 机制：
//  1. 读取 webreaper_schema_migrations 表中已应用的版本号
//  2. 遍历内嵌的 migrations/*.sql（按版本号排序）
//  3. 跳过已应用的，执行未应用的
//  4. 每个迁移执行后记录版本号
func RunMigrations(db *gorm.DB) error {
	// 1. 确保迁移记录表存在（用 webreaper_ 前缀，避免与同库其他项目冲突）
	if err := db.Exec(fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		version VARCHAR(16) PRIMARY KEY,
		applied_at DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`, migrationsTableName)).Error; err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}

	// 2. 读取已应用版本
	var applied []string
	if err := db.Table(migrationsTableName).Pluck("version", &applied).Error; err != nil {
		return fmt.Errorf("read applied migrations: %w", err)
	}
	appliedSet := make(map[string]bool, len(applied))
	for _, v := range applied {
		appliedSet[v] = true
	}

	// 3. 加载并排序迁移文件
	migrations, err := loadMigrations()
	if err != nil {
		return fmt.Errorf("load migrations: %w", err)
	}

	// 4. 逐个执行未应用的迁移
	// 2026-09-02 迁移整合：83 个增量迁移（001~089）已合并为 000_baseline.sql
	//（从完整库导出的最终形态——含全部 ALTER 产物 + 种子数据），旧文件备份在
	// Docs/archive/migrations-2026-09-02/。今后新迁移从 001 开始编号。
	//
	// 已有库兼容（webreaper_schema_migrations 里已有 001~089 记录）：
	//   → 000 未标记 → 先执行 000（幂等——表已存在时 DROP+CREATE 重建，
	//     数据会丢！）——不能让已有库跑 000。
	//   → 因此：库中已有任何旧迁移记录时，000 视为已应用（跳过）。
	if len(applied) > 0 && !appliedSet["000"] {
		appliedSet["000"] = true
		_ = db.Exec(fmt.Sprintf("INSERT IGNORE INTO %s (version) VALUES ('000')", migrationsTableName)).Error
	}

	for i := 0; i < len(migrations); i++ {
		m := migrations[i]
		if appliedSet[m.version] {
			continue
		}
		if err := executeMigration(db, m); err != nil {
			return fmt.Errorf("apply migration %s: %w", m.name, err)
		}
		// 同版本多文件全部顺序执行，只在最后一个执行后写记录（INSERT IGNORE 幂等）
		isLastOfVersion := i+1 >= len(migrations) || migrations[i+1].version != m.version
		if isLastOfVersion {
			if err := db.Exec(fmt.Sprintf("INSERT IGNORE INTO %s (version) VALUES (?)", migrationsTableName), m.version).Error; err != nil {
				return fmt.Errorf("record migration %s: %w", m.name, err)
			}
		}
	}

	// 兼容性修复：对齐 PO 与实际表结构的列名/列差异（幂等——列已存在则跳过）。
	// 根因：早期迁移（022）建的列名与后来修改的 GORM PO 不一致。
	if err := fixColumnMismatches(db); err != nil {
		return fmt.Errorf("fix column mismatches: %w", err)
	}
	// 兼容性修复：统一全库表 collation 到 utf8mb4_unicode_ci（幂等——已统一的表跳过）。
	// 根因：早期迁移建表只写 CHARSET=utf8mb4 未指定 COLLATE，MySQL 8 服务器默认
	// utf8mb4_0900_ai_ci，与显式 unicode_ci 的新表（065+）JOIN 比较时报
	// Error 1267 Illegal mix of collations（如 admin/inspirations/stats 的
	// geo_brands ↔ brand_inspirations 关联）。
	if err := fixCollationMismatch(db); err != nil {
		return fmt.Errorf("fix collation mismatch: %w", err)
	}
	return nil
}

// fixCollationMismatch 把 collation 非 utf8mb4_unicode_ci 的基表统一 CONVERT 过去。
// 统一到多数派（065+ 迁移显式声明的 unicode_ci）；CONVERT 会重建表，大表耗时——
// 单表失败仅日志不阻断（下次启动重试，幂等）。
func fixCollationMismatch(db *gorm.DB) error {
	var tables []string
	if err := db.Raw(`SELECT table_name FROM information_schema.tables
		WHERE table_schema = DATABASE() AND table_type = 'BASE TABLE'
		  AND table_collation <> 'utf8mb4_unicode_ci'`).Scan(&tables).Error; err != nil {
		return err
	}
	for _, t := range tables {
		if err := db.Exec(fmt.Sprintf("ALTER TABLE `%s` CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci", t)).Error; err != nil {
			log.Printf("[migrate] 表 %s collation 统一失败（跳过，下次重试）: %v", t, err)
			continue
		}
	}
	if len(tables) > 0 {
		log.Printf("[migrate] 已统一 %d 张表的 collation 到 utf8mb4_unicode_ci（消除新老表 JOIN 的 Error 1267）", len(tables))
	}
	return nil
}

// loadMigrations 从内嵌文件系统加载所有迁移 SQL，按版本号排序。
func loadMigrations() ([]migrationFile, error) {
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return nil, err
	}
	var files []migrationFile
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		content, err := migrationsFS.ReadFile("migrations/" + e.Name())
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", e.Name(), err)
		}
		// 版本号 = 文件名下划线前的部分
		version := e.Name()
		if idx := strings.Index(version, "_"); idx > 0 {
			version = version[:idx]
		}
		files = append(files, migrationFile{
			version: version,
			name:    e.Name(),
			content: string(content),
		})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].version < files[j].version })
	return files, nil
}

// executeMigration 执行单个迁移文件（按分号分割语句逐条执行）。
// 注意：简单实现，不支持多行触发器/存储过程等含分号的复杂语句。
// 当前迁移都是简单 DDL，足够用。
//
// 幂等性：ADD COLUMN / ADD INDEX 若目标已存在则跳过该语句——
// 历史上出现过"建表迁移已含该列、但补丁迁移重复 ADD"（如 015 撞 014 更新），
// 全新库会报 Duplicate column name。此处统一免疫，无需改迁移文件。
func executeMigration(db *gorm.DB, m migrationFile) error {
	stmts := splitStatements(m.content)
	for _, stmt := range stmts {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if skipIfDDLTargetExists(db, stmt) {
			continue
		}
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("execute statement: %w", err)
		}
	}
	return nil
}

// 幂等 DDL 匹配：ALTER TABLE x ADD COLUMN col / ADD [UNIQUE] INDEX name
// （迁移 SQL 均为裸标识符，无需支持反引号包裹的表名/列名）
var (
	addColumnRe = regexp.MustCompile(`(?i)^ALTER\s+TABLE\s+(\w+)\s+ADD\s+COLUMN\s+(\w+)`)
	addIndexRe  = regexp.MustCompile(`(?i)^ALTER\s+TABLE\s+(\w+)\s+ADD\s+(UNIQUE\s+|FULLTEXT\s+|SPATIAL\s+)?(?:INDEX|KEY)\s+(\w+)`)
)

// skipIfDDLTargetExists 判断 ALTER 目标（列/索引）是否已存在——已存在则跳过整条语句。
// 复合语句（一个 ALTER 多个 ADD）以第一个 ADD COLUMN/ADD INDEX 的目标为准。
func skipIfDDLTargetExists(db *gorm.DB, stmt string) bool {
	if m := addColumnRe.FindStringSubmatch(stmt); m != nil {
		return columnExists(db, m[1], m[2])
	}
	if m := addIndexRe.FindStringSubmatch(stmt); m != nil {
		// 捕获组：1=表名，2=UNIQUE/FULLTEXT 等前缀（可为空），3=索引名
		return indexExists(db, m[1], m[3])
	}
	return false
}

// splitStatements 按分号分割 SQL 语句（简单的分行处理）。
func splitStatements(sqlText string) []string {
	// 去除注释行
	var lines []string
	for _, line := range strings.Split(sqlText, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "--") {
			continue
		}
		lines = append(lines, line)
	}
	cleaned := strings.Join(lines, "\n")
	return strings.Split(cleaned, ";")
}

// PendingMigrationsCount 返回尚未应用的迁移数量（供测试/日志用）。
func PendingMigrationsCount(db *gorm.DB) (int, error) {
	migrations, err := loadMigrations()
	if err != nil {
		return 0, err
	}
	var applied []string
	ctx := context.Background()
	_ = db.WithContext(ctx).Table(migrationsTableName).Pluck("version", &applied).Error
	appliedSet := make(map[string]bool, len(applied))
	for _, v := range applied {
		appliedSet[v] = true
	}
	count := 0
	for _, m := range migrations {
		if !appliedSet[m.version] {
			count++
		}
	}
	return count, nil
}

// columnExists 检查列是否存在（幂等 ALTER 的前置条件）。
func columnExists(db *gorm.DB, table, column string) bool {
	var n int64
	db.Raw("SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND COLUMN_NAME = ?", table, column).Scan(&n)
	return n > 0
}

// indexExists 检查索引是否存在（幂等 ADD INDEX 的前置条件）。
func indexExists(db *gorm.DB, table, index string) bool {
	var n int64
	db.Raw("SELECT COUNT(*) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND INDEX_NAME = ?", table, index).Scan(&n)
	return n > 0
}

// fixColumnMismatches 修复 PO 与实际表结构的列名/列差异（幂等）。
//
// 根因：早期迁移 022 建的列名与后来修改的 GORM PO 不一致：
//   - tenant_settings: `key` → setting_key（PO 用 SettingKey）
//   - usages: feature → scene（PO 用 Scene），缺 user_id/llm_config_name/llm_calls，多 cost_credits
//
// 幂等：每次启动都跑，列已对齐的 ALTER 被 columnExists 跳过。
func fixColumnMismatches(db *gorm.DB) error {
	type fix struct {
		check func() bool
		exec  string
	}

	// tenant_settings: `key` → setting_key
	if columnExists(db, "tenant_settings", "key") && !columnExists(db, "tenant_settings", "setting_key") {
		if err := db.Exec("ALTER TABLE tenant_settings CHANGE COLUMN `key` setting_key VARCHAR(64) NOT NULL").Error; err != nil {
			return fmt.Errorf("fix tenant_settings.key→setting_key: %w", err)
		}
	}

	// usages: feature → scene
	if columnExists(db, "usages", "feature") && !columnExists(db, "usages", "scene") {
		if err := db.Exec("ALTER TABLE usages CHANGE COLUMN feature scene VARCHAR(32) NOT NULL DEFAULT ''").Error; err != nil {
			return fmt.Errorf("fix usages.feature→scene: %w", err)
		}
	}

	// usages: 加缺失列（PO 有但迁移没建）
	for col, ddl := range map[string]string{
		"user_id":         "ADD COLUMN user_id VARCHAR(64) NOT NULL DEFAULT '' AFTER tenant_id",
		"llm_config_name": "ADD COLUMN llm_config_name VARCHAR(64) NOT NULL DEFAULT '' AFTER scene",
		"llm_calls":       "ADD COLUMN llm_calls INT NOT NULL DEFAULT 0 AFTER total_tokens",
	} {
		if !columnExists(db, "usages", col) {
			if err := db.Exec("ALTER TABLE usages " + ddl).Error; err != nil {
				return fmt.Errorf("fix usages add %s: %w", col, err)
			}
		}
	}

	// usages: 删多余列 cost_credits（PO 没有）
	if columnExists(db, "usages", "cost_credits") {
		if err := db.Exec("ALTER TABLE usages DROP COLUMN cost_credits").Error; err != nil {
			return fmt.Errorf("fix usages drop cost_credits: %w", err)
		}
	}

	return nil
}
