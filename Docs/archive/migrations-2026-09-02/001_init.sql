-- 001_init.sql 初始建表
-- 对应 adapter/repository/model.go 的 PO 定义

CREATE TABLE IF NOT EXISTS job_posts (
    id           VARCHAR(64) PRIMARY KEY,
    source       VARCHAR(32),
    company      VARCHAR(128),
    position     VARCHAR(128),
    requirements JSON,
    salary       VARCHAR(64),
    raw_html     LONGTEXT,
    url          VARCHAR(512),
    fingerprint  VARCHAR(64) UNIQUE,
    collected_at DATETIME(3),
    created_at   DATETIME(3),
    updated_at   DATETIME(3),
    INDEX idx_job_posts_source (source)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS interview_questions (
    id          VARCHAR(64) PRIMARY KEY,
    job_post_id VARCHAR(64),
    title       VARCHAR(256),
    answer      LONGTEXT,
    difficulty  VARCHAR(16),
    tags        JSON,
    created_at  DATETIME(3),
    updated_at  DATETIME(3),
    INDEX idx_questions_job_post (job_post_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS knowledge (
    id         VARCHAR(64) PRIMARY KEY,
    title      VARCHAR(256),
    content    LONGTEXT,
    summary    VARCHAR(512),
    tags       JSON,
    source_url VARCHAR(512),
    vector_ref VARCHAR(128),
    created_at DATETIME(3),
    updated_at DATETIME(3)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS tasks (
    id         VARCHAR(64) PRIMARY KEY,
    type       VARCHAR(32),
    input      TEXT,
    output     TEXT,
    status     VARCHAR(16),
    error      TEXT,
    created_at DATETIME(3),
    updated_at DATETIME(3),
    INDEX idx_tasks_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS publish_records (
    id           VARCHAR(64) PRIMARY KEY,
    content_id   VARCHAR(64),
    content_type VARCHAR(16),
    platform     VARCHAR(32),
    success      BOOLEAN,
    external_id  VARCHAR(128),
    error_msg    TEXT,
    result_at    DATETIME(3),
    created_at   DATETIME(3),
    updated_at   DATETIME(3),
    INDEX idx_publish_dedup (content_id, content_type, platform),
    INDEX idx_publish_success (success)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
