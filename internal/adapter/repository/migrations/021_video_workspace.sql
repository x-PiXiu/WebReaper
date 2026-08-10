-- 021_video_workspace 视频生成工作台（生成任务 + 发布任务）
-- 生成任务状态机：pending → generating → dubbing → composing → ready / failed
-- 表由 AutoMigrate 模型 &VideoTaskPO{}/&VideoJobPO{} 冗余声明（migrations 优先）。

CREATE TABLE IF NOT EXISTS video_tasks (
  id VARCHAR(64) PRIMARY KEY,
  tenant_id VARCHAR(64) NOT NULL,
  brand_id VARCHAR(64) NOT NULL DEFAULT '',
  mode VARCHAR(16) NOT NULL DEFAULT 'text',
  prompt TEXT,
  material_url TEXT,
  status VARCHAR(16) NOT NULL DEFAULT 'pending',
  video_url TEXT,
  voice_text TEXT,
  voice_url TEXT,
  final_url TEXT,
  duration_sec INT NOT NULL DEFAULT 0,
  error TEXT,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  INDEX idx_video_tasks_tenant (tenant_id),
  INDEX idx_video_tasks_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS video_jobs (
  id VARCHAR(64) PRIMARY KEY,
  tenant_id VARCHAR(64) NOT NULL,
  task_id VARCHAR(64) NOT NULL,
  account_id VARCHAR(64) NOT NULL DEFAULT '',
  platform VARCHAR(32) NOT NULL DEFAULT '',
  status VARCHAR(16) NOT NULL DEFAULT 'pending',
  external_url TEXT,
  error TEXT,
  created_at DATETIME NOT NULL,
  INDEX idx_video_jobs_tenant (tenant_id),
  INDEX idx_video_jobs_task (task_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
