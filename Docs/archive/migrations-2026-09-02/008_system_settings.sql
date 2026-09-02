-- 008_system_settings.sql  系统级配置表（key-value，运行时可调）
--
-- 存放爬虫速率、robots 开关等需要 UI 动态修改的配置，
-- 修改后无需重启即生效。
-- 注意：列名用 setting_key 而非 key，因为 KEY 是 MySQL 保留字。

CREATE TABLE IF NOT EXISTS system_settings (
    setting_key VARCHAR(64) PRIMARY KEY,
    value       LONGTEXT,
    updated_at  DATETIME(3)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
