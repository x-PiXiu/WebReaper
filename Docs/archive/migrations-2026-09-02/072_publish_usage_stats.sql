-- 发布使用量统计表
-- 按品牌、平台、日期统计发布次数，用于限速控制
CREATE TABLE IF NOT EXISTS publish_usage_stats (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    tenant_id VARCHAR(64) NOT NULL COMMENT '租户ID',
    brand_id VARCHAR(64) NOT NULL COMMENT '品牌ID',
    platform VARCHAR(32) NOT NULL COMMENT '平台标识',
    publish_date DATE NOT NULL COMMENT '发布日期',
    usage_count INT DEFAULT 0 COMMENT '当日发布次数',
    last_publish_at DATETIME COMMENT '最近一次发布时间',
    UNIQUE KEY uk_tenant_brand_platform_date (tenant_id, brand_id, platform, publish_date),
    INDEX idx_tenant_brand (tenant_id, brand_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
