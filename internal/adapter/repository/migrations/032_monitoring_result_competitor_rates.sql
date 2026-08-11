-- 032_monitoring_result_competitor_rates.sql 监测结果竞品提及率列（P5-01 补全）
--
-- 背景：PO 模型含 CompetitorRates（{竞品名: 提及率} JSON），但 030 迁移只补了
--   sources/self_source_count，漏了 competitor_rates 列——导致带该字段的监测
--   结果 Save 全部失败（Error 1054 Unknown column）且错误被忽略（数据静默丢失，
--   AI 榜/总览/建议永远读不到监测数据）。此处补列，与 PO 映射对齐。

ALTER TABLE geo_monitoring_results ADD COLUMN competitor_rates JSON NULL;
