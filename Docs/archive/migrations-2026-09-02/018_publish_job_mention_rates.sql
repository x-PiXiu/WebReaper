-- 018_publish_job_mention_rates 补充 geo_publish_jobs 提及率+品牌ID字段

ALTER TABLE geo_publish_jobs ADD COLUMN brand_id VARCHAR(64);
ALTER TABLE geo_publish_jobs ADD COLUMN pre_mention_rate DECIMAL(5,2) DEFAULT 0;
ALTER TABLE geo_publish_jobs ADD COLUMN post_mention_rate DECIMAL(5,2) DEFAULT 0;
