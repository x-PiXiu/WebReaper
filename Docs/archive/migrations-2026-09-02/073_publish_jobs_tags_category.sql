-- 073: 发布任务补标签/分区字段（Plan-14 对接修复 #4/#5）
-- 向导此前将 tags/category 拼进正文兜底；现贯通为独立字段供 RPA 消费
-- （B站独立标签框必填 ≥1、投稿分区必选）。
ALTER TABLE geo_publish_jobs ADD COLUMN tags_json TEXT NULL COMMENT '标签列表（JSON 数组）';
ALTER TABLE geo_publish_jobs ADD COLUMN category VARCHAR(64) NULL COMMENT '平台分区（B站投稿必选）';
