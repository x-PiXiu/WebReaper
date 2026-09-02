-- 038_generation.sql 统一生成任务（Vidu 全量接入：视频/图片/音频/数字人）
--
-- 设计（Docs/Plans/03 计划文档）：
--   - generation_tasks：统一任务模型——17 端点差异收敛到 type/sub_type/model/params_json
--   - generation_specs：端点/模型注册表——管理后台可覆盖能力向量/启用开关（代码为默认值）
--   - media_assets：素材与产物资产——用户上传图/音频托管（避开 20MB body 限制）+
--     产物 24h URL 转存永久化

CREATE TABLE IF NOT EXISTS generation_tasks (
	id VARCHAR(64) PRIMARY KEY,
	tenant_id VARCHAR(64) NOT NULL,
	brand_id VARCHAR(64) NOT NULL DEFAULT '',
	type VARCHAR(32) NOT NULL,
	sub_type VARCHAR(64) NOT NULL,
	model VARCHAR(64) NOT NULL,
	provider VARCHAR(32) NOT NULL DEFAULT 'vidu',
	provider_task_id VARCHAR(128) NOT NULL DEFAULT '',
	state VARCHAR(16) NOT NULL DEFAULT 'created',
	err_code VARCHAR(64) NOT NULL DEFAULT '',
	err_msg VARCHAR(512) NOT NULL DEFAULT '',
	params_json JSON NULL,
	payload VARCHAR(512) NOT NULL DEFAULT '',
	creations_json JSON NULL,
	credits INT NOT NULL DEFAULT 0,
	off_peak TINYINT(1) NOT NULL DEFAULT 0,
	watermark TINYINT(1) NOT NULL DEFAULT 0,
	callback_received TINYINT(1) NOT NULL DEFAULT 0,
	callback_at DATETIME(3) NULL,
	retry_count INT NOT NULL DEFAULT 0,
	params_hash VARCHAR(64) NOT NULL DEFAULT '',
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	finished_at DATETIME(3) NULL,
	INDEX idx_gen_tenant (tenant_id, created_at),
	INDEX idx_gen_provider_task (provider_task_id),
	INDEX idx_gen_state (state),
	INDEX idx_gen_hash (tenant_id, params_hash)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS generation_specs (
	sub_type VARCHAR(64) NOT NULL,
	model VARCHAR(64) NOT NULL,
	endpoint VARCHAR(128) NOT NULL DEFAULT '',
	enabled TINYINT(1) NOT NULL DEFAULT 1,
	capabilities_json JSON NULL,
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	PRIMARY KEY (sub_type, model)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS media_assets (
	id VARCHAR(64) PRIMARY KEY,
	tenant_id VARCHAR(64) NOT NULL,
	brand_id VARCHAR(64) NOT NULL DEFAULT '',
	owner_type VARCHAR(16) NOT NULL DEFAULT 'material',
	source_url VARCHAR(512) NOT NULL DEFAULT '',
	stored_url VARCHAR(512) NOT NULL DEFAULT '',
	mime VARCHAR(64) NOT NULL DEFAULT '',
	size_bytes BIGINT NOT NULL DEFAULT 0,
	meta_json JSON NULL,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	expires_at DATETIME(3) NULL,
	INDEX idx_asset_tenant (tenant_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
