-- 054: 账号角色（merchant=商户自有发布用 / platform=平台工作账号只读搜索用）
-- 搜索等只读操作优先用 platform 账号——风控风险集中到平台可控的账号。

ALTER TABLE geo_accounts ADD COLUMN role VARCHAR(16) DEFAULT 'merchant' NOT NULL COMMENT '账号角色：merchant（商户）/ platform（平台工作账号）';
ALTER TABLE geo_accounts ADD INDEX idx_geo_accounts_role (role);
