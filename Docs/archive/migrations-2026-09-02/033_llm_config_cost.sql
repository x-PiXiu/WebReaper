-- 033_llm_config_cost.sql LLM 配置按引擎成本（P1-1：多引擎监测成本分析）
--
-- 背景：监测接入多引擎（豆包/千问/DeepSeek 等）后，成本分析仍按全局单一参考价
--   （LLM_COST_PER_MToken 默认 ¥1/百万 tokens）估算——豆包（~¥0.2）与 GPT 级
--   （~¥3）成本差异无法体现。给 llm_configs 加按引擎单价，成本报表按引擎细分。
-- 单位：分/百万 tokens（默认 100 = ¥1，与全局参考价一致，兼容存量数据）。

ALTER TABLE llm_configs ADD COLUMN cost_per_mtok INT NOT NULL DEFAULT 100;
