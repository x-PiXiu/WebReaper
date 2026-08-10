-- 013_geo_core.sql  GEO 核心表（品牌/关键词/监测结果/优化内容）
--
-- 多租户：所有表带 tenant_id + 索引，强制按租户隔离。

CREATE TABLE IF NOT EXISTS geo_brands (
    id           VARCHAR(64) PRIMARY KEY,
    tenant_id    VARCHAR(64) NOT NULL,
    name         VARCHAR(128) NOT NULL,
    positioning  TEXT,
    core_selling JSON,
    competitors  JSON,
    created_at   DATETIME(3),
    INDEX idx_geo_brands_tenant (tenant_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS geo_keywords (
    id         VARCHAR(64) PRIMARY KEY,
    tenant_id  VARCHAR(64) NOT NULL,
    brand_id   VARCHAR(64) NOT NULL,
    term       VARCHAR(256) NOT NULL,
    intent     VARCHAR(32),
    created_at DATETIME(3),
    INDEX idx_geo_keywords_tenant (tenant_id),
    INDEX idx_geo_keywords_brand (brand_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS geo_monitoring_results (
    id            VARCHAR(64) PRIMARY KEY,
    tenant_id     VARCHAR(64) NOT NULL,
    brand_id      VARCHAR(64) NOT NULL,
    keyword_id    VARCHAR(64) NOT NULL,
    engine_name   VARCHAR(64) NOT NULL,
    sample_count  INT NOT NULL DEFAULT 0,
    mention_count INT NOT NULL DEFAULT 0,
    mention_rate  DECIMAL(4,3) NOT NULL DEFAULT 0,
    avg_position  INT NOT NULL DEFAULT 0,
    sentiment     VARCHAR(16),
    competitors   JSON,
    confidence    DECIMAL(4,3) NOT NULL DEFAULT 0,
    probed_at     DATETIME(3),
    raw_sample    TEXT,
    INDEX idx_geo_mr_tenant_brand (tenant_id, brand_id),
    INDEX idx_geo_mr_keyword (keyword_id),
    INDEX idx_geo_mr_probed (probed_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS geo_optimized_contents (
    id             VARCHAR(64) PRIMARY KEY,
    tenant_id      VARCHAR(64) NOT NULL,
    brand_id       VARCHAR(64) NOT NULL,
    keyword_id     VARCHAR(64),
    original_text  LONGTEXT,
    optimized_text LONGTEXT,
    version        INT NOT NULL DEFAULT 1,
    score_total    DECIMAL(5,2),
    authority      DECIMAL(5,2),
    specificity    DECIMAL(5,2),
    structure      DECIMAL(5,2),
    uniqueness     DECIMAL(5,2),
    recency        DECIMAL(5,2),
    status         VARCHAR(32) NOT NULL DEFAULT 'draft',
    created_at     DATETIME(3),
    INDEX idx_geo_oc_tenant_brand (tenant_id, brand_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
