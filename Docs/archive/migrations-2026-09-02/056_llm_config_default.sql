-- 056: LLM 配置默认模型标记（is_default 互斥：同 Usage 下只有一条 true）
ALTER TABLE llm_configs ADD COLUMN is_default TINYINT(1) NOT NULL DEFAULT 0;
