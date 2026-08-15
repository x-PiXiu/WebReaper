-- 首选率与语义降级标记（迁移 045）：
-- 1) first_pick_count：被提及且位次=1 的采样数。"首选率"（FirstPickCount/SampleCount）
--    是"从被收录到被首选引用"两阶段叙事的核心进阶指标；此前后端只有均值位次，
--    前端用 avg_position==1 近似首选率（平均位次=1 ≠ 每次都排第一，整数截断放大误差）。
-- 2) semantic_degraded：采样中出现过解析降级（解析 LLM 失败/JSON 损坏 → 字符串匹配兜底）。
--    降级时情感/位次缺失——此前静默失真（商户看到的"中性/未提及"可能只是降级产物），
--    现标记落库对商户可见。
-- 旧数据缺省 0/false（增量兼容，与 sentiment 迁移同模式）。
ALTER TABLE geo_monitoring_results ADD COLUMN first_pick_count INT NOT NULL DEFAULT 0;
ALTER TABLE geo_monitoring_results ADD COLUMN semantic_degraded BOOLEAN NOT NULL DEFAULT FALSE;
