-- 010_agent_autosave.sql  Agent 自动落库配置
--
-- 让 Agent 支持"对话生成结构化数据 → 自动落库为 DataItem"。
-- AutoSave 开关 + FieldMapping 字段映射。

ALTER TABLE agent_configs ADD COLUMN auto_save BOOLEAN DEFAULT FALSE AFTER max_iterations;
ALTER TABLE agent_configs ADD COLUMN field_mapping TEXT AFTER auto_save;
