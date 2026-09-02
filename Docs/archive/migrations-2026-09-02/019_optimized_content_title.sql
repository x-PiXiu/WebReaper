-- 019_optimized_content_title 优化内容增加标题字段（发布到平台用）

ALTER TABLE geo_optimized_contents ADD COLUMN title VARCHAR(256) NOT NULL DEFAULT '' AFTER id;
