-- 049: 账号 OAuth 授权绑定（获客智能体：官方 API 通道）
-- geo_accounts 加 OAuth 字段——抖音开放平台等官方授权账号与浏览器扫码账号共存，
-- 发布时按 auth_type 路由：oauth → API 通道，cookie → 浏览器 RPA 通道。
-- 存量账号 auth_type 默认 cookie（向后兼容，行为不变）。

ALTER TABLE geo_accounts ADD COLUMN auth_type VARCHAR(16) DEFAULT 'cookie' NOT NULL COMMENT '绑定方式：cookie（扫码浏览器）/ oauth（官方授权）';
ALTER TABLE geo_accounts ADD COLUMN access_token_enc TEXT COMMENT 'OAuth access_token 密文（AES-GCM）';
ALTER TABLE geo_accounts ADD COLUMN refresh_token_enc TEXT COMMENT 'OAuth refresh_token 密文（AES-GCM）';
ALTER TABLE geo_accounts ADD COLUMN open_id VARCHAR(128) DEFAULT '' NOT NULL COMMENT '平台用户唯一标识（OAuth 授权返回）';
ALTER TABLE geo_accounts ADD INDEX idx_geo_accounts_open_id (open_id);
