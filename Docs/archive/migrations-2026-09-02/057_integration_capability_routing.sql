-- 057: 能力路由模型（统一配置查询——厂商一行，能力多行，切换 is_default 秒级生效）
CREATE TABLE IF NOT EXISTS integration_vendors (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(128) NOT NULL DEFAULT '',
    base_url VARCHAR(512) NOT NULL DEFAULT '',
    api_key VARCHAR(512) NOT NULL DEFAULT '',
    protocol VARCHAR(32) NOT NULL DEFAULT 'openai',
    enabled TINYINT(1) NOT NULL DEFAULT 1,
    updated_at DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS integration_capabilities (
    id VARCHAR(128) PRIMARY KEY,            -- "{cap_id}#{vendor_id}"
    cap_id VARCHAR(64) NOT NULL DEFAULT '', -- "asr" / "llm-chat" / "tts" / ...
    vendor_id VARCHAR(64) NOT NULL DEFAULT '',
    endpoint VARCHAR(512) NOT NULL DEFAULT '',
    model VARCHAR(128) NOT NULL DEFAULT '',
    is_default TINYINT(1) NOT NULL DEFAULT 0,
    enabled TINYINT(1) NOT NULL DEFAULT 1,
    extra_json TEXT NULL,
    updated_at DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    INDEX idx_cap_id (cap_id),
    INDEX idx_vendor_id (vendor_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
