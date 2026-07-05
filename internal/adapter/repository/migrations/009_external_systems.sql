-- 009_external_systems.sql  外部推送系统配置表
--
-- 设计动机：把推送目标系统的配置（端点/认证/字段映射）持久化，
-- 支持运行时动态增删，不硬编码任何外部系统。
-- publish_records 表已在 001_init.sql 创建，这里只建 external_systems。

CREATE TABLE IF NOT EXISTS external_systems (
    name          VARCHAR(64) PRIMARY KEY,
    description   VARCHAR(256),
    endpoint      VARCHAR(512),
    method        VARCHAR(8) DEFAULT 'POST',
    headers       TEXT,
    field_mapping TEXT,
    content_type  VARCHAR(32),
    enabled       BOOLEAN DEFAULT TRUE,
    created_at    DATETIME(3),
    updated_at    DATETIME(3)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
