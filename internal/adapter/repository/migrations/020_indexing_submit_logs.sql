-- 020_indexing_submit_logs 收录提交日志（审计排查"为什么没被收录"）

CREATE TABLE IF NOT EXISTS indexing_submit_logs (
  id VARCHAR(64) PRIMARY KEY,
  channel VARCHAR(16) NOT NULL DEFAULT '',
  url VARCHAR(512) NOT NULL DEFAULT '',
  status VARCHAR(16) NOT NULL DEFAULT '',
  error_msg TEXT,
  submitted_at DATETIME NOT NULL,
  INDEX idx_submit_logs_channel (channel),
  INDEX idx_submit_logs_status (status),
  INDEX idx_submit_logs_time (submitted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
