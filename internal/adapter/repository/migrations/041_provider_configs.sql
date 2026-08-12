-- 041_provider_configs 厂商配置（按厂商区分管理——管理后台可设置 Vidu API Key）
--
-- 背景：生成厂商（Vidu 等）的 API Key 此前只来自环境变量/配置文件，无法在
-- 管理后台动态设置。本表作为厂商配置的 DB 事实源：装配时优先 DB（环境变量兜底），
-- 保存后对已装配厂商热生效（provider.SetAPIKey，无需重启）。
--
-- 安全：GET 返回掩码（masked_key）；PUT 时 api_key 为空 = 不修改。

CREATE TABLE IF NOT EXISTS provider_configs (
  provider VARCHAR(32) PRIMARY KEY,        -- vidu / …
  api_key VARCHAR(512) NOT NULL DEFAULT '', -- 厂商 API Key（明文存 DB——后台管理场景，运维可控）
  base_url VARCHAR(256) NOT NULL DEFAULT '',
  enabled TINYINT(1) NOT NULL DEFAULT 1,
  extra_json TEXT NULL, -- 扩展字段（JSON 文本；TEXT 规避 MySQL 3140 空串错误）
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
