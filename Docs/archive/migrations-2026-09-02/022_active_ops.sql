-- 022_active_notifications_etc 站内通知 / 租户设置 / LLM 用量计量 + 内容收录状态字段
-- 这些表此前仅存在于 AutoMigrate 模型（MySQL 生产环境不执行 AutoMigrate，需版本化迁移）。

-- 站内通知（主动唤醒：提及率变化/自动复测/排期发布/系统消息）
CREATE TABLE IF NOT EXISTS notifications (
  id VARCHAR(64) PRIMARY KEY,
  tenant_id VARCHAR(64) NOT NULL,
  type VARCHAR(32) NOT NULL DEFAULT 'system',
  title VARCHAR(128) NOT NULL DEFAULT '',
  content TEXT,
  link VARCHAR(256) NOT NULL DEFAULT '',
  is_read TINYINT NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL,
  INDEX idx_notifications_tenant (tenant_id),
  INDEX idx_notifications_read (is_read)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 租户级设置（多租户个性化配置：自动盯盘开关等；复合主键）
CREATE TABLE IF NOT EXISTS tenant_settings (
  tenant_id VARCHAR(64) NOT NULL,
  `key` VARCHAR(64) NOT NULL,
  value VARCHAR(512) NOT NULL DEFAULT '',
  updated_at DATETIME NOT NULL,
  PRIMARY KEY (tenant_id, `key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- LLM 用量计量（经济系统基础：计费/额度预留）
CREATE TABLE IF NOT EXISTS usages (
  id VARCHAR(64) PRIMARY KEY,
  tenant_id VARCHAR(64) NOT NULL,
  feature VARCHAR(32) NOT NULL DEFAULT '',
  model VARCHAR(64) NOT NULL DEFAULT '',
  prompt_tokens INT NOT NULL DEFAULT 0,
  completion_tokens INT NOT NULL DEFAULT 0,
  total_tokens INT NOT NULL DEFAULT 0,
  cost_credits INT NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL,
  INDEX idx_usages_tenant (tenant_id),
  INDEX idx_usages_feature (feature),
  INDEX idx_usages_time (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
