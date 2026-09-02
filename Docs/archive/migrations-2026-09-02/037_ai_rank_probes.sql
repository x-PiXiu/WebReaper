-- 037_ai_rank_probes.sql AI 榜单探查结果表（附近同行 AI 榜数据源）
--
-- 背景：监测回答中本地小店极少出现 → AI 榜稀疏，用户看不到对比压力。
--   新增"AI 榜单探查"：本地化中性问法（不点名）→ AI 真实搜索回答 →
--   附近 POI 名单归因匹配（名单不喂给 LLM，仅解析阶段匹配，无诱导）→
--   结果缓存 24h（探查消耗 LLM/地图配额，手动刷新可强制重跑）。

CREATE TABLE IF NOT EXISTS geo_ai_rank_probes (
	id VARCHAR(64) PRIMARY KEY,
	tenant_id VARCHAR(64) NOT NULL,
	brand_id VARCHAR(64) NOT NULL,
	results JSON NULL,
	sample_count INT NOT NULL DEFAULT 0,
	probed_at DATETIME(3) NOT NULL,
	expire_at DATETIME(3) NOT NULL,
	INDEX idx_ai_rank_brand (brand_id, probed_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
