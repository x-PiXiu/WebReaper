-- 024_prompt_templates 提示词模板仓库（内容生成/优化的系统提示词可管理、可热更新）

CREATE TABLE IF NOT EXISTS prompt_templates (
  `key` VARCHAR(64) PRIMARY KEY,
  version INT NOT NULL DEFAULT 1,
  content LONGTEXT,
  updated_at DATETIME NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
