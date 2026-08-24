-- 066_crawler_optimization.sql
-- 爬虫优化：关键词池 + 品牌关联 + 智能调度
--
-- 改进1: crawler_configs 新增 brand_id（品牌关联）
-- 改进2: keyword_pool（LLM 生成的关键词池，JSON 数组）
-- 改进3: last_keyword_index（关键词轮换指针）
-- 改进4: worker_count（并发 worker 数量）

-- 新增字段
ALTER TABLE crawler_configs ADD COLUMN brand_id VARCHAR(64) NOT NULL DEFAULT '' AFTER tenant_id;
ALTER TABLE crawler_configs ADD COLUMN keyword_pool JSON NULL AFTER extra_keywords;
ALTER TABLE crawler_configs ADD COLUMN last_keyword_index INT NOT NULL DEFAULT 0 AFTER keyword_pool;

-- 索引优化
ALTER TABLE crawler_configs ADD INDEX idx_brand_id (brand_id);
