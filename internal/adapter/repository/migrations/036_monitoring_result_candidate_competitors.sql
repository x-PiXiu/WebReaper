-- 036_monitoring_result_candidate_competitors.sql 监测结果竞品候选列（P1-4 竞品沉淀）
--
-- 背景：监测时 LLM 客观列出 AI 回答中出现的所有品牌，已配置竞品计入
--   competitor_rates（对比坐标系），但"回答中自然出现的其他品牌"此前被丢弃——
--   导致「从监测结果推荐」的数据源只有已配置竞品，而候选又被"排除已有竞品"
--   过滤，蒸馏永远返回空（逻辑盲区）。此处补列：非自身、非已配置竞品的
--   品牌名沉淀为候选，供竞品推荐接口蒸馏采纳。

ALTER TABLE geo_monitoring_results ADD COLUMN candidate_competitors JSON NULL;
