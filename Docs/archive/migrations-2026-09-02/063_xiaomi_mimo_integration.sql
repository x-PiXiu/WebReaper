-- 063_xiaomi_mimo_integration.sql
-- 添加小米MiMo厂商和能力配置

-- 添加小米MiMo厂商（使用实际存在的列）
INSERT INTO integration_vendors (id, name, base_url, api_key, protocol, enabled, updated_at)
VALUES ('xiaomi-mimo', '小米MiMo', 'https://token-plan-cn.xiaomimimo.com/v1', '', 'openai-chat', 1, NOW())
ON DUPLICATE KEY UPDATE name = VALUES(name);

-- 添加小米MiMo能力配置
INSERT INTO integration_capabilities (id, cap_id, vendor_id, endpoint, model, is_default, enabled, updated_at)
VALUES 
-- LLM能力
('llm#xiaomi-mimo', 'llm', 'xiaomi-mimo', '/v1/chat/completions', 'mimo-v2.5-pro', 0, 1, NOW()),
-- TTS能力
('tts#xiaomi-mimo', 'tts', 'xiaomi-mimo', '/v1/chat/completions', 'mimo-v2.5-tts', 0, 1, NOW()),
-- ASR能力
('asr#xiaomi-mimo', 'asr', 'xiaomi-mimo', '/v1/chat/completions', 'mimo-v2.5-asr', 0, 1, NOW()),
-- 声音克隆能力
('voice-clone#xiaomi-mimo', 'voice-clone', 'xiaomi-mimo', '/v1/chat/completions', 'mimo-v2.5-tts-voiceclone', 0, 1, NOW())
ON DUPLICATE KEY UPDATE model = VALUES(model);
