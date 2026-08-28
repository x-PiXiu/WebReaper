-- 075: 发布任务可见性（YouTube 等平台：public/unlisted/private；空=默认公开）
ALTER TABLE geo_publish_jobs ADD COLUMN privacy VARCHAR(16) NOT NULL DEFAULT '' COMMENT '可见性（youtube: public/unlisted/private；空=默认公开）';
