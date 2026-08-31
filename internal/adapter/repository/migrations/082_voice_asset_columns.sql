-- 082: 音色资产扩展列（26 号计划——个人克隆音色从 generation_tasks 物化到 generation_voices 表）
-- 用户"我的音色"改查 scope=clone AND tenant_id=?；官方音色 scope=vidu 不变。
ALTER TABLE generation_voices
  ADD COLUMN IF NOT EXISTS scope VARCHAR(16) NOT NULL DEFAULT 'vidu',   -- vidu(官方表seed) / platform(官方复刻) / clone(用户克隆)
  ADD COLUMN IF NOT EXISTS tenant_id VARCHAR(64) NOT NULL DEFAULT '',   -- clone 行归属；官方行空
  ADD COLUMN IF NOT EXISTS source_task_id VARCHAR(64) NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS status VARCHAR(16) NOT NULL DEFAULT 'active';
