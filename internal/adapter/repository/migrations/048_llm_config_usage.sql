-- 048: LLM 配置加用途标签（获客智能体转型：视觉模型与聊天模型独立配置）
-- Usage 字段区分模型用途："" = 聊天/内容（默认），"vision" = 视觉模型（浏览器截图分析）。
-- 两套模型独立配置互不影响：聊天模型坏了浏览器 Agent 不受影响，反之亦然。
-- 幂等：列已存在时跳过（迁移可能部分执行后中断）。

-- 列不存在时才添加
SET @col_exists = (
  SELECT COUNT(*) FROM information_schema.COLUMNS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'llm_configs' AND COLUMN_NAME = 'usage'
);
SET @sql_add = IF(@col_exists = 0,
  'ALTER TABLE llm_configs ADD COLUMN `usage` VARCHAR(32) DEFAULT '''' NOT NULL COMMENT ''用途标签''',
  'SELECT 1'
);
PREPARE stmt_add FROM @sql_add;
EXECUTE stmt_add;
DEALLOCATE PREPARE stmt_add;

-- 索引不存在时才添加
SET @idx_exists = (
  SELECT COUNT(*) FROM information_schema.STATISTICS
  WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'llm_configs' AND INDEX_NAME = 'idx_llm_configs_usage_col'
);
SET @sql_idx = IF(@idx_exists = 0,
  'ALTER TABLE llm_configs ADD INDEX idx_llm_configs_usage_col (`usage`)',
  'SELECT 1'
);
PREPARE stmt_idx FROM @sql_idx;
EXECUTE stmt_idx;
DEALLOCATE PREPARE stmt_idx;
