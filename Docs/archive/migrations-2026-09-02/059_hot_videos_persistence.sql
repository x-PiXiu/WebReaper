-- 059: 热门同款视频持久化（原内存缓存 → DB，支持搜索/排序/定时采集积累）
CREATE TABLE IF NOT EXISTS hot_videos (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL DEFAULT '',
    brand_id VARCHAR(64) NOT NULL DEFAULT '',
    title VARCHAR(512) NOT NULL DEFAULT '',
    url VARCHAR(1024) NOT NULL DEFAULT '',
    platform VARCHAR(32) NOT NULL DEFAULT '',   -- douyin/kuaishou/xiaohongshu/bilibili/web
    hot_point VARCHAR(1024) NOT NULL DEFAULT '', -- 为什么火（一句话）
    topic VARCHAR(512) NOT NULL DEFAULT '',      -- 拍摄同款选题建议
    cover_url VARCHAR(1024) NOT NULL DEFAULT '', -- 封面图（如有）
    author VARCHAR(128) NOT NULL DEFAULT '',     -- 作者
    play_count BIGINT NOT NULL DEFAULT 0,
    digg_count BIGINT NOT NULL DEFAULT 0,
    comment_count BIGINT NOT NULL DEFAULT 0,
    publish_time DATETIME(3) NULL,               -- 原始发布时间（平台排序用）
    source VARCHAR(32) NOT NULL DEFAULT 'search',-- search=通用搜索 / douyin=站内搜索
    created_at DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3),
    UNIQUE INDEX idx_brand_url (brand_id, url(255)), -- 同品牌同 URL 去重
    INDEX idx_tenant_brand (tenant_id, brand_id),
    INDEX idx_brand_platform (brand_id, platform),
    INDEX idx_publish_time (publish_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
