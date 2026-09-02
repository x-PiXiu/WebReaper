-- 055: 官方音色库（Vidu 语音合成音色表——seed 进 DB 供客户端查询/筛选）
-- 端点：GET /api/v1/generation/voices?language=&q=
CREATE TABLE IF NOT EXISTS generation_voices (
    voice_id VARCHAR(128) PRIMARY KEY,
    language VARCHAR(64) NOT NULL DEFAULT '',
    name VARCHAR(128) NOT NULL DEFAULT '',
    sample_url VARCHAR(512) NOT NULL DEFAULT '',
    created_at DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
