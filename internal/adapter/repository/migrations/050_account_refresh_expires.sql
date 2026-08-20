-- 050: OAuth refresh_token 续期窗口管理（抖音 token 生命周期）
-- 抖音规则：access_token 15 天（到期前自动刷新）；refresh_token 30 天（可续期 5 次，
-- 单次授权最长 195 天）。refresh_expires_at 驱动健康检查在窗口关闭前自动续期。

ALTER TABLE geo_accounts ADD COLUMN refresh_expires_at DATETIME(3) NULL COMMENT 'refresh_token 过期时间（OAuth 续期窗口管理）';
ALTER TABLE geo_accounts ADD INDEX idx_geo_accounts_refresh_expires (refresh_expires_at);
