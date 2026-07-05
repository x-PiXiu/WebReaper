-- 011_external_system_raw_mode.sql  外部系统推送模式
--
-- 支持 raw（原样转发）和 mapping（字段映射）两种模式。
-- raw 模式：DataItem.Content 已是目标系统请求体 JSON，直接 POST，无需字段映射。

ALTER TABLE external_systems ADD COLUMN mode VARCHAR(16) DEFAULT 'raw' AFTER headers;
ALTER TABLE external_systems ADD COLUMN body_template TEXT AFTER field_mapping;

-- 兼容旧数据：有 field_mapping 的旧系统默认设为 mapping 模式
UPDATE external_systems SET mode = 'mapping' WHERE field_mapping IS NOT NULL AND field_mapping != '';
