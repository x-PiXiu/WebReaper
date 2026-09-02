-- 023_content_index_status 内容收录状态字段（收录验证任务每日回写）
-- 注意：MySQL 8.0 不支持 ADD COLUMN IF NOT EXISTS，依赖迁移版本表保证只执行一次。

ALTER TABLE geo_optimized_contents
  ADD COLUMN index_status VARCHAR(16) NOT NULL DEFAULT '' AFTER status,
  ADD COLUMN indexed_at DATETIME NULL AFTER index_status;
