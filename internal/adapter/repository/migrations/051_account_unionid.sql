-- 051: 账号 union_id（跨端账号打通）
-- 抖音开放平台：网站应用/小程序/移动应用是三个不同的 client_key，同一用户在各应用下
-- open_id 不同、union_id 相同（同主体）。未来接入小程序/App 时靠 union_id 合并三端账号。

ALTER TABLE geo_accounts ADD COLUMN union_id VARCHAR(128) DEFAULT '' NOT NULL COMMENT '开放平台维度用户标识（跨应用稳定——三端账号打通用）';
ALTER TABLE geo_accounts ADD INDEX idx_geo_accounts_union_id (union_id);
