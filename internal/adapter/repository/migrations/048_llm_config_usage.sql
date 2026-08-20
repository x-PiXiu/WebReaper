-- 048: LLM 配置加用途标签（获客智能体转型：视觉模型与聊天模型独立配置）
-- Usage 字段区分模型用途："" = 聊天/内容（默认），"vision" = 视觉模型（浏览器截图分析）。
-- 两套模型独立配置互不影响：聊天模型坏了浏览器 Agent 不受影响，反之亦然。

ALTER TABLE llm_configs ADD COLUMN usage VARCHAR(32) DEFAULT '' NOT NULL COMMENT '用途标签：空=聊天模型，vision=视觉模型';
ALTER TABLE llm_configs ADD INDEX idx_llm_configs_usage (usage);
