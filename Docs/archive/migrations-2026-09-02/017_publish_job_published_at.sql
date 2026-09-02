-- 017_publish_job_published_at 补充 geo_publish_jobs.published_at 列

ALTER TABLE geo_publish_jobs ADD COLUMN published_at DATETIME(3);
