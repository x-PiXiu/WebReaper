-- 083: 模型差异化计费——generation_specs 表新增 cost_credits 列（27 号优化）
-- 每次调用消耗积分：0=使用服务商返回值（向后兼容）；>0=管理后台配置的固定积分。
-- 典型值：q1=1, q2-pro=3, q3-pro=5（管理后台可调）。
ALTER TABLE generation_specs
  ADD COLUMN IF NOT EXISTS cost_credits INT NOT NULL DEFAULT 0;
