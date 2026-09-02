-- 026_publish_job_scheduled_at 排期发布字段（定时发送功能）
-- 539f8ec 提交加了 GORM 模型字段，但 MySQL 生产环境用版本化迁移，
-- 缺此列导致 ListScheduledDue 查询报 Error 1054。

ALTER TABLE geo_publish_jobs
  ADD COLUMN scheduled_at DATETIME NULL AFTER post_mention_rate,
  ADD INDEX idx_publish_jobs_scheduled_at (scheduled_at);
