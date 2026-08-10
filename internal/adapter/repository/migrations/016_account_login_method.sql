-- 016_account_login_method 补充 geo_accounts.login_method 列

ALTER TABLE geo_accounts ADD COLUMN login_method VARCHAR(16);
