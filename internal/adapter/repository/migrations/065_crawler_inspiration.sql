-- 065_crawler_inspiration.sql
-- 灵感广场与爬虫系统基础设施
--
-- 设计（Docs/Plans/13-可规模化爬虫架构设计方案）：
--   - crawler_accounts：平台方账号管理（管理员维护，用于数据爬取）
--   - crawler_configs：爬虫配置（采集间隔、关键词、排序等）
--   - crawler_task_logs：采集任务日志（执行记录、错误追踪）
--   - inspiration_videos：灵感视频数据（热门视频持久化）
--   - brand_inspirations：品牌-视频关联（多对多关系）

-- 平台方账号表（管理员维护，用于统一数据爬取）
CREATE TABLE IF NOT EXISTS crawler_accounts (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    platform VARCHAR(32) NOT NULL,
    account_name VARCHAR(128) NOT NULL,
    cookie_encrypted TEXT NOT NULL,
    user_agent VARCHAR(512) NOT NULL DEFAULT '',
    proxy_address VARCHAR(256) NOT NULL DEFAULT '',
    status VARCHAR(16) NOT NULL DEFAULT 'active',
    last_used_at DATETIME(3) NULL,
    last_health_check_at DATETIME(3) NULL,
    health_check_result VARCHAR(16) NOT NULL DEFAULT 'unknown',
    daily_usage_count INT NOT NULL DEFAULT 0,
    daily_usage_limit INT NOT NULL DEFAULT 50,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    INDEX idx_platform_status (platform, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 爬虫配置表（管理后台可动态修改）
CREATE TABLE IF NOT EXISTS crawler_configs (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    platform VARCHAR(32) NOT NULL,
    tenant_id VARCHAR(64) NOT NULL DEFAULT '',
    enabled TINYINT(1) NOT NULL DEFAULT 1,
    search_keywords JSON NULL,
    extra_keywords JSON NULL,
    crawl_interval_minutes INT NOT NULL DEFAULT 15,
    max_results INT NOT NULL DEFAULT 20,
    sort_by VARCHAR(32) NOT NULL DEFAULT 'popular',
    publish_time VARCHAR(16) NOT NULL DEFAULT 'week',
    enable_comments TINYINT(1) NOT NULL DEFAULT 0,
    enable_refresh TINYINT(1) NOT NULL DEFAULT 1,
    refresh_interval_hours INT NOT NULL DEFAULT 12,
    rate_limit_per_min INT NOT NULL DEFAULT 10,
    proxy_enabled TINYINT(1) NOT NULL DEFAULT 0,
    max_retry_count INT NOT NULL DEFAULT 3,
    last_crawled_at DATETIME(3) NULL,
    last_error TEXT NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    UNIQUE KEY uk_platform_tenant (platform, tenant_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 采集任务日志表
CREATE TABLE IF NOT EXISTS crawler_task_logs (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    task_id VARCHAR(64) NOT NULL,
    platform VARCHAR(32) NOT NULL,
    brand_id VARCHAR(64) NOT NULL DEFAULT '',
    trigger_type VARCHAR(16) NOT NULL DEFAULT 'scheduled',
    status VARCHAR(16) NOT NULL DEFAULT 'running',
    keywords_used JSON NULL,
    videos_found INT NOT NULL DEFAULT 0,
    videos_new INT NOT NULL DEFAULT 0,
    videos_updated INT NOT NULL DEFAULT 0,
    error_message TEXT NULL,
    started_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    finished_at DATETIME(3) NULL,
    duration_ms INT NOT NULL DEFAULT 0,
    INDEX idx_platform (platform),
    INDEX idx_status (status),
    INDEX idx_started (started_at DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 灵感视频数据表
CREATE TABLE IF NOT EXISTS inspiration_videos (
    id VARCHAR(64) PRIMARY KEY,
    platform VARCHAR(16) NOT NULL DEFAULT 'douyin',
    platform_video_id VARCHAR(128) NOT NULL DEFAULT '',
    title VARCHAR(512) NOT NULL DEFAULT '',
    description TEXT NULL,
    cover_url VARCHAR(1024) NOT NULL DEFAULT '',
    video_url VARCHAR(1024) NOT NULL DEFAULT '',
    author VARCHAR(128) NOT NULL DEFAULT '',
    author_avatar VARCHAR(1024) NOT NULL DEFAULT '',
    duration INT NOT NULL DEFAULT 0,
    publish_time DATETIME(3) NULL,
    play_count BIGINT NOT NULL DEFAULT 0,
    digg_count BIGINT NOT NULL DEFAULT 0,
    comment_count BIGINT NOT NULL DEFAULT 0,
    share_count BIGINT NOT NULL DEFAULT 0,
    collect_count BIGINT NOT NULL DEFAULT 0,
    topics JSON NULL,
    music_name VARCHAR(256) NOT NULL DEFAULT '',
    music_author VARCHAR(256) NOT NULL DEFAULT '',
    sentiment VARCHAR(16) NOT NULL DEFAULT 'neutral',
    viral_score DOUBLE NOT NULL DEFAULT 0,
    is_pinned TINYINT(1) NOT NULL DEFAULT 0,
    is_recommended TINYINT(1) NOT NULL DEFAULT 0,
    admin_note VARCHAR(512) NOT NULL DEFAULT '',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    last_refreshed_at DATETIME(3) NULL,
    UNIQUE KEY uk_platform_video (platform, platform_video_id),
    INDEX idx_viral_score (viral_score DESC),
    INDEX idx_publish_time (publish_time DESC),
    INDEX idx_play_count (play_count DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 品牌-视频关联表（多对多）
CREATE TABLE IF NOT EXISTS brand_inspirations (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    brand_id VARCHAR(64) NOT NULL,
    video_id VARCHAR(64) NOT NULL,
    search_keyword VARCHAR(256) NOT NULL DEFAULT '',
    relevance_score DOUBLE NOT NULL DEFAULT 0,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    UNIQUE KEY uk_brand_video (brand_id, video_id),
    INDEX idx_brand_id (brand_id),
    INDEX idx_video_id (video_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 默认爬虫配置（种子数据）
INSERT INTO crawler_configs (platform, enabled, crawl_interval_minutes, max_results, sort_by, publish_time)
VALUES
    ('douyin', 1, 15, 20, 'popular', 'week'),
    ('kuaishou', 0, 15, 20, 'popular', 'week'),
    ('bilibili', 0, 15, 20, 'click', 'week')
ON DUPLICATE KEY UPDATE platform = VALUES(platform);
