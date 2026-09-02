-- 账号品牌绑定表
-- 一个账号可以绑定到多个品牌，一个品牌可以绑定多个账号
CREATE TABLE IF NOT EXISTS account_brand_bindings (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    tenant_id VARCHAR(64) NOT NULL COMMENT '租户ID',
    account_id VARCHAR(64) NOT NULL COMMENT '账号ID',
    brand_id VARCHAR(64) NOT NULL COMMENT '品牌ID',
    platform VARCHAR(32) NOT NULL COMMENT '平台标识',
    is_default BOOLEAN DEFAULT FALSE COMMENT '是否默认账号',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uk_account_brand (account_id, brand_id),
    INDEX idx_tenant_brand (tenant_id, brand_id),
    INDEX idx_tenant_account (tenant_id, account_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
