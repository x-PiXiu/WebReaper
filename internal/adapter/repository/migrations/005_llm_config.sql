-- 005_llm_config.sql  LLM 配置独立分离
--
-- 设计动机：把 LLM 连接配置（厂商/apiKey/baseURL/model）从 AgentConfig 中抽出，
-- 成为独立聚合根。多个 Agent 可引用同一个 LLMConfig（多对一）。
-- 新增厂商/模型只需新增一条 llm_configs 记录，符合开闭原则。
--
-- agent_configs 不删旧 model 列（向后兼容），新增 llm_config_name 列引用 llm_configs.name。

CREATE TABLE IF NOT EXISTS llm_configs (
    name       VARCHAR(64) PRIMARY KEY,
    provider   VARCHAR(32),
    api_key    VARCHAR(256),
    base_url   VARCHAR(256),
    model      VARCHAR(64),
    created_at DATETIME(3),
    updated_at DATETIME(3)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- agent_configs 新增 llm_config_name 列（保留旧 model 列，向后兼容）
ALTER TABLE agent_configs ADD COLUMN llm_config_name VARCHAR(64) DEFAULT NULL AFTER model;
