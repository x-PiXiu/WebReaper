-- 043_knowledge_materials.sql 平台知识库（按行业采集的素材，带来源溯源）
--
-- 需求链路：平台按行业采集网页内容入库（保留来源 URL + 原文）
--   → 用户生成时按"品牌行业 + 关键词"检索素材（带来源）
--   → 素材 + 来源 → 规格化 system prompt → 上游 LLM 生成
--
-- 设计要点（详见 Docs/Plans/04-平台知识库与素材溯源生成.md）：
--   - url_fingerprint 唯一索引：持久化去重（替代爬虫装饰器的内存 map，重启不丢）
--   - embedding JSON 列：向量检索（Go 侧余弦计算）；P2 可迁移 Milvus
--   - 平台级（无 tenant_id）：按行业组织，多租户共享检索

CREATE TABLE IF NOT EXISTS kb_materials (
    id              VARCHAR(64) PRIMARY KEY,
    industry        VARCHAR(64) NOT NULL,
    source_url      VARCHAR(1024) NOT NULL,
    url_fingerprint VARCHAR(64) NOT NULL,
    title           VARCHAR(512),
    content         MEDIUMTEXT,
    summary         TEXT,
    tags            JSON,
    crawl_keyword   VARCHAR(256),
    embedding       JSON,
    status          VARCHAR(16) NOT NULL DEFAULT 'active',
    created_at      DATETIME(3),
    UNIQUE KEY uk_kb_fingerprint (url_fingerprint),
    INDEX idx_kb_industry (industry),
    INDEX idx_kb_created (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 品牌行业字段（知识库检索的过滤维度；空值兼容 = 检索时从定位推断）
ALTER TABLE geo_brands ADD COLUMN industry VARCHAR(64) DEFAULT '' AFTER competitors;
