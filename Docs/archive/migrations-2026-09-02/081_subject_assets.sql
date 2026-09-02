-- 081: 主体资产表（26 号计划——分身/环境从 generation_tasks 物化到独立资产表）
-- 任务表回归"过程记录"语义，终态成功的对象快照落独立资产表；任务随便清理，资产永生。
CREATE TABLE IF NOT EXISTS subject_assets (
  id               VARCHAR(64)  NOT NULL PRIMARY KEY,
  tenant_id        VARCHAR(64)  NOT NULL,
  scope            VARCHAR(16)  NOT NULL DEFAULT 'personal',  -- personal / official
  kind             VARCHAR(16)  NOT NULL DEFAULT 'person',    -- person / scene
  name             VARCHAR(128) NOT NULL,
  server_id        VARCHAR(128) NOT NULL,                     -- Vidu 主体 id
  portrait_url     VARCHAR(512) NOT NULL DEFAULT '',
  avatar_video_url VARCHAR(512) NOT NULL DEFAULT '',           -- 链式形象视频产物
  voice_id         VARCHAR(128) NOT NULL DEFAULT '',
  tags             VARCHAR(512) NOT NULL DEFAULT '',
  sort_order       INT          NOT NULL DEFAULT 0,
  status           VARCHAR(16)  NOT NULL DEFAULT 'active',    -- active / disabled
  source_task_id   VARCHAR(64)  NOT NULL DEFAULT '',           -- 溯源任务 ID
  created_at       DATETIME     NOT NULL,
  updated_at       DATETIME     NOT NULL,
  UNIQUE KEY uk_server (server_id),
  KEY idx_tenant_scope (tenant_id, scope, kind, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
