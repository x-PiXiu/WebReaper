-- 030_monitoring_result_sources.sql 监测结果引用来源（归因生命线 P5-01）
--
-- 背景：AI"提到品牌"≠"引用我们的内容"。新增来源字段后：
--   · sources JSON：AI 回答中提到的来源链接/平台名（去重）
--   · self_source_count：来源里包含自营公开站域名的次数（>0 = 内容真的被引用）
-- 数据沉淀后可用于评分校准（P5-02：发布内容被引用次数 vs GEO 评分相关性）。

ALTER TABLE geo_monitoring_results
    ADD COLUMN sources JSON NULL,
    ADD COLUMN self_source_count INT NOT NULL DEFAULT 0;
