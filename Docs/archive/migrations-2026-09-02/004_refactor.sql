-- 004_refactor.sql 通用数据采集平台重构
-- 新建通用表，不删旧表（向后兼容）

-- 通用数据项（替代 job_posts/questions/knowledge/crawl_results）
CREATE TABLE IF NOT EXISTS data_items (
    id            VARCHAR(64) PRIMARY KEY,
    collection_id VARCHAR(64),
    title         VARCHAR(512),
    content       LONGTEXT,
    summary       TEXT,
    tags          JSON,
    source_url    VARCHAR(512),
    raw_content   LONGTEXT,
    status        VARCHAR(20) DEFAULT 'pending_review',
    metadata      JSON,
    created_at    DATETIME(3),
    updated_at    DATETIME(3),
    INDEX idx_items_collection (collection_id),
    INDEX idx_items_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 采集集合
CREATE TABLE IF NOT EXISTS collections (
    id         VARCHAR(64) PRIMARY KEY,
    name       VARCHAR(128),
    agent_name VARCHAR(64),
    task_id    VARCHAR(64),
    status     VARCHAR(20) DEFAULT 'collecting',
    item_count INT DEFAULT 0,
    created_at DATETIME(3),
    updated_at DATETIME(3),
    INDEX idx_collections_task (task_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Agent 配置
CREATE TABLE IF NOT EXISTS agent_configs (
    name           VARCHAR(64) PRIMARY KEY,
    system_prompt  TEXT,
    tools          JSON,
    model          VARCHAR(64),
    max_iterations INT DEFAULT 10,
    created_at     DATETIME(3),
    updated_at     DATETIME(3)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
