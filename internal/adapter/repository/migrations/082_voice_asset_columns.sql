-- 082: 音色资产扩展列（26 号计划——个人克隆音色从 generation_tasks 物化到 generation_voices 表）
-- 用户"我的音色"改查 scope=clone AND tenant_id=?；官方音色 scope=vidu 不变。
-- 注意：MySQL 不支持 ADD COLUMN IF NOT EXISTS，需要逐列检查后添加。
-- 使用存储过程或逐条执行（迁移器会逐条执行）。

-- scope 列
SET @col_exists = (SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'generation_voices' AND COLUMN_NAME = 'scope');
SET @sql = IF(@col_exists = 0, 'ALTER TABLE generation_voices ADD COLUMN scope VARCHAR(16) NOT NULL DEFAULT ''vidu''', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- tenant_id 列
SET @col_exists = (SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'generation_voices' AND COLUMN_NAME = 'tenant_id');
SET @sql = IF(@col_exists = 0, 'ALTER TABLE generation_voices ADD COLUMN tenant_id VARCHAR(64) NOT NULL DEFAULT ''''', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- source_task_id 列
SET @col_exists = (SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'generation_voices' AND COLUMN_NAME = 'source_task_id');
SET @sql = IF(@col_exists = 0, 'ALTER TABLE generation_voices ADD COLUMN source_task_id VARCHAR(64) NOT NULL DEFAULT ''''', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- status 列
SET @col_exists = (SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'generation_voices' AND COLUMN_NAME = 'status');
SET @sql = IF(@col_exists = 0, 'ALTER TABLE generation_voices ADD COLUMN status VARCHAR(16) NOT NULL DEFAULT ''active''', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
