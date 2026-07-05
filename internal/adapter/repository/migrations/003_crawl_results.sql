-- 003_crawl_results.sql 爬取结果表（Agent 采集的数据，含人工审核状态）

CREATE TABLE IF NOT EXISTS crawl_results (
    id           VARCHAR(64) PRIMARY KEY,
    source_url   VARCHAR(512) NOT NULL,
    crawler_type VARCHAR(16),
    title        VARCHAR(256),
    raw_content  LONGTEXT,
    summary      LONGTEXT,
    tags         JSON,
    status       VARCHAR(20) DEFAULT 'pending_review',
    task_id      VARCHAR(64),
    created_at   DATETIME(3),
    updated_at   DATETIME(3),
    INDEX idx_crawl_status (status),
    INDEX idx_crawl_task (task_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
