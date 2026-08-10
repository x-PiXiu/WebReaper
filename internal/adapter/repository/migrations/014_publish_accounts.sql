-- 014_publish_accounts 多平台发布账号与发布任务
--
-- 设计动机（整洁架构 + 多租户隔离）：
--   社媒平台（知乎/小红书）无内容发布 API，靠扫码登录获取 cookie 后半自动发布。
--   cookie 是高敏感数据，只存 AES-GCM 密文（cookie_encrypted），绝不落明文。
--   所有表强制带 tenant_id 实现行级隔离，配复合索引加速按租户/平台查询。

-- geo_accounts：多租户平台账号（加密 cookie）
CREATE TABLE IF NOT EXISTS geo_accounts (
    id               VARCHAR(64) PRIMARY KEY,
    tenant_id        VARCHAR(64) NOT NULL,
    platform         VARCHAR(32) NOT NULL,
    display_name     VARCHAR(128),
    cookie_encrypted TEXT,
    health           VARCHAR(16) DEFAULT 'active',
    expires_at       DATETIME(3),
    bound_at         DATETIME(3),
    last_used_at     DATETIME(3),
    INDEX idx_geo_acc_tenant (tenant_id),
    INDEX idx_geo_acc_platform (tenant_id, platform),
    INDEX idx_geo_acc_expires (expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- geo_publish_jobs：发布任务记录
CREATE TABLE IF NOT EXISTS geo_publish_jobs (
    id           VARCHAR(64) PRIMARY KEY,
    tenant_id    VARCHAR(64) NOT NULL,
    account_id   VARCHAR(64),
    platform     VARCHAR(32) NOT NULL,
    content_id   VARCHAR(64),
    title        VARCHAR(256),
    content      LONGTEXT,
    mode         VARCHAR(16) DEFAULT 'semi-auto',
    status       VARCHAR(16) DEFAULT 'pending',
    external_url TEXT,
    error_msg    TEXT,
    created_at   DATETIME(3),
    INDEX idx_geo_pj_tenant (tenant_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
