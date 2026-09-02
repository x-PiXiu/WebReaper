-- 品牌发布配置表
-- 每个品牌在每个平台可以有不同的发布配置（账号绑定、限速、默认标签等）
CREATE TABLE IF NOT EXISTS brand_publish_configs (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    tenant_id VARCHAR(64) NOT NULL COMMENT '租户ID',
    brand_id VARCHAR(64) NOT NULL COMMENT '品牌ID',
    platform VARCHAR(32) NOT NULL COMMENT '平台标识（douyin/kuaishou/xiaohongshu/weixin/bilibili）',
    account_ids JSON COMMENT '绑定的账号ID列表',
    rate_limit JSON COMMENT '限速配置 {"max_per_day":5,"max_per_hour":2,"min_interval":1800}',
    default_tags JSON COMMENT '品牌默认标签',
    default_persona VARCHAR(64) COMMENT '默认人设ID',
    is_active BOOLEAN DEFAULT TRUE COMMENT '是否启用',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uk_tenant_brand_platform (tenant_id, brand_id, platform),
    INDEX idx_tenant_brand (tenant_id, brand_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
