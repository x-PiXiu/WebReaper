-- 074: 灵感视频本地转存地址（站内播放——爬虫热门视频下载到本地，不依赖原站防盗链）
ALTER TABLE inspiration_videos ADD COLUMN local_video_url VARCHAR(1024) NOT NULL DEFAULT '' COMMENT '本地转存地址（空=未转存回落原始链接）';
