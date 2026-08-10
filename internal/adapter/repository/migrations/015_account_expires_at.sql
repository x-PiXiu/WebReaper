-- 015_account_expires_at 补充 geo_accounts.expires_at 列
--
-- 原因：014 迁移用 CREATE TABLE IF NOT EXISTS，表已存在时不会加新列。
-- 本迁移直接 ALTER TABLE 补列。迁移记录表保证只执行一次，无需幂等。
-- （不用 ADD COLUMN IF NOT EXISTS，因为 MySQL 5.7 和 8.0 早期版本不支持该语法）

ALTER TABLE geo_accounts ADD COLUMN expires_at DATETIME(3);
ALTER TABLE geo_accounts ADD INDEX idx_geo_acc_expires (expires_at);
