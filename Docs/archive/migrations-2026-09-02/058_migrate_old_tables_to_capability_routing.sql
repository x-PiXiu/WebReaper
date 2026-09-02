-- 058: 迁移旧表数据到能力路由新表（provider_configs/llm_configs → integration_vendors/capabilities）
-- 幂等：INSERT IGNORE 跳过已迁移数据（主键冲突）

-- ① 从 provider_configs 迁移厂商（Vidu/Tavily/ZPAY/ASR）
INSERT IGNORE INTO integration_vendors (id, name, base_url, api_key, protocol, enabled, updated_at)
SELECT
    provider,
    CASE provider
        WHEN 'vidu' THEN 'Vidu'
        WHEN 'tavily' THEN 'Tavily'
        WHEN 'zpay' THEN 'ZPAY'
        WHEN 'asr' THEN 'ASR'
        ELSE provider
    END,
    base_url,
    api_key,
    CASE provider
        WHEN 'vidu' THEN 'vidu'
        ELSE 'openai'
    END,
    enabled,
    updated_at
FROM provider_configs
WHERE provider IN ('vidu', 'tavily', 'zpay', 'asr');

-- ② 从 provider_configs 迁移能力路由（ASR 条目）
INSERT IGNORE INTO integration_capabilities (id, cap_id, vendor_id, endpoint, model, is_default, enabled, extra_json, updated_at)
SELECT
    CONCAT('asr#', provider),
    'asr',
    provider,
    base_url,
    COALESCE(JSON_UNQUOTE(JSON_EXTRACT(extra_json, '$.model')), ''),
    CASE WHEN provider = 'asr' THEN 1 ELSE 0 END,
    enabled,
    extra_json,
    updated_at
FROM provider_configs
WHERE provider = 'asr' AND api_key != '';

-- ③ 从 llm_configs 迁移厂商（provider 字段非空且在 integration_vendors 中不存在）
INSERT IGNORE INTO integration_vendors (id, name, base_url, api_key, protocol, enabled, updated_at)
SELECT DISTINCT
    COALESCE(NULLIF(provider, ''), 'default'),
    COALESCE(NULLIF(provider, ''), 'default'),
    base_url,
    api_key,
    'openai',
    1,
    updated_at
FROM llm_configs
WHERE api_key != ''
  AND COALESCE(NULLIF(provider, ''), 'default') NOT IN (SELECT id FROM integration_vendors);

-- ④ 从 llm_configs 迁移能力路由（llm-chat 条目，usage 为空的聊天模型）
INSERT IGNORE INTO integration_capabilities (id, cap_id, vendor_id, endpoint, model, is_default, enabled, extra_json, updated_at)
SELECT
    CONCAT('llm-chat#', COALESCE(NULLIF(provider, ''), 'default')),
    'llm-chat',
    COALESCE(NULLIF(provider, ''), 'default'),
    CONCAT(base_url, '/chat/completions'),
    model,
    CASE WHEN is_default = 1 OR name = 'default' THEN 1 ELSE 0 END,
    1,
    '{}',
    updated_at
FROM llm_configs
WHERE api_key != ''
  AND (`usage` = '' OR `usage` IS NULL);

-- ⑤ 从 llm_configs 迁移视觉模型能力路由（llm-vision 条目）
INSERT IGNORE INTO integration_capabilities (id, cap_id, vendor_id, endpoint, model, is_default, enabled, extra_json, updated_at)
SELECT
    CONCAT('llm-vision#', COALESCE(NULLIF(provider, ''), 'default')),
    'llm-vision',
    COALESCE(NULLIF(provider, ''), 'default'),
    CONCAT(base_url, '/chat/completions'),
    model,
    0,
    1,
    '{}',
    updated_at
FROM llm_configs
WHERE api_key != ''
  AND `usage` = 'vision';
