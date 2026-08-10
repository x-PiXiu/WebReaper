-- 025_billing 经济系统三表：plans（套餐）/ subscriptions（订阅）/ orders（订单）
-- 配额与功能白名单用 JSON 列存储（MySQL 无原生 map 类型）。

CREATE TABLE IF NOT EXISTS plans (
  id VARCHAR(64) PRIMARY KEY,
  name VARCHAR(64) NOT NULL DEFAULT '',
  level VARCHAR(16) NOT NULL DEFAULT '',
  price_cents INT NOT NULL DEFAULT 0,
  quotas TEXT,                       -- JSON: {"monitor":500,"content-gen":50}；-1=无限
  features TEXT,                     -- JSON: ["auto-monitor","video"]
  status VARCHAR(16) NOT NULL DEFAULT 'active',
  sort_order INT NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  INDEX idx_plans_level (level),
  INDEX idx_plans_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS subscriptions (
  id VARCHAR(64) PRIMARY KEY,
  tenant_id VARCHAR(64) NOT NULL,
  plan_id VARCHAR(64) NOT NULL DEFAULT '',
  status VARCHAR(16) NOT NULL DEFAULT 'active',
  period_start DATETIME NOT NULL,
  period_end DATETIME NOT NULL,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  UNIQUE KEY uq_subscriptions_tenant (tenant_id),  -- 一租户一有效订阅
  INDEX idx_subscriptions_plan (plan_id),
  INDEX idx_subscriptions_period_end (period_end)  -- 到期预警查询
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS orders (
  id VARCHAR(64) PRIMARY KEY,
  tenant_id VARCHAR(64) NOT NULL DEFAULT '',
  plan_id VARCHAR(64) NOT NULL DEFAULT '',
  amount_cents INT NOT NULL DEFAULT 0,
  status VARCHAR(16) NOT NULL DEFAULT 'pending',
  payment_gateway VARCHAR(32) NOT NULL DEFAULT '',
  payment_id VARCHAR(128) NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL,
  paid_at DATETIME NULL,
  INDEX idx_orders_tenant (tenant_id),
  INDEX idx_orders_status (status),
  INDEX idx_orders_created (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
