-- 042_publish_job_content_type 发布任务内容形态字段（Platform × ContentType 双维度）
--
-- 背景：小红书创作平台支持 image/video/article/audio 四种 target，同一账号
-- 可发多种形态。PublishJob 加 ContentType（内容形态）+ MediaURLs（媒体文件）
-- + CoverURL（封面），适配多形态发布。知乎默认 article（纯文本，MediaURLs 空）。

ALTER TABLE geo_publish_jobs ADD COLUMN content_type VARCHAR(16) NOT NULL DEFAULT '';
ALTER TABLE geo_publish_jobs ADD COLUMN media_urls_json TEXT NULL;
ALTER TABLE geo_publish_jobs ADD COLUMN cover_url TEXT NULL;
