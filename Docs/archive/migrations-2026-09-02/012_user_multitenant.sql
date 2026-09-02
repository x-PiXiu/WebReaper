-- 012_user_multitenant.sql  用户表多租户改造
--
-- 为 webreaper_users 加 role 和 tenant_id 列，支持商户端/管理端角色分流。
-- 现有用户默认设为 admin（向后兼容：旧数据不丢失权限）。

ALTER TABLE webreaper_users
    ADD COLUMN role     VARCHAR(32) NOT NULL DEFAULT 'admin',
    ADD COLUMN tenant_id VARCHAR(64) NOT NULL DEFAULT '';

-- 加索引便于按租户/角色查询
CREATE INDEX idx_webreaper_users_role     ON webreaper_users (role);
CREATE INDEX idx_webreaper_users_tenant   ON webreaper_users (tenant_id);
