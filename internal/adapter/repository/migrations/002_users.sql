-- 002_users.sql WebReaper 用户表（认证用）
-- 表名用 webreaper_users 前缀，避免与同库的 AgentCore users 表冲突。

DROP TABLE IF EXISTS webreaper_users;

CREATE TABLE webreaper_users (
    id            VARCHAR(64) PRIMARY KEY,
    username      VARCHAR(64) NOT NULL,
    password_hash VARCHAR(128) NOT NULL,
    created_at    DATETIME(3),
    updated_at    DATETIME(3),
    UNIQUE INDEX idx_webreaper_users_username (username)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
