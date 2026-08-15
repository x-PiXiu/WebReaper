-- 竞品情感（迁移 044）：monitoring_results 增加竞品情感映射。
-- 背景：竞品对标从"提及率"升级为"提及率+情感"（自家 vs 竞品并排的语义维度）。
-- 探针解析回答时 LLM 已返回每个品牌的 sentiment，此前只存了自家的——
-- 现把竞品情感一并落库。旧数据该列为 NULL，展示端按中性处理。
ALTER TABLE geo_monitoring_results ADD COLUMN competitor_sentiments JSON NULL;
