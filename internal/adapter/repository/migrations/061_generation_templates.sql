-- 061_generation_templates.sql
-- 创建generation_templates表，用于存储生成模板配置
-- 管理后台可以动态增删改查，不是硬编码在代码中

CREATE TABLE IF NOT EXISTS generation_templates (
    id VARCHAR(50) PRIMARY KEY,
    tenant_id VARCHAR(50) DEFAULT '' COMMENT '租户ID（空=全局模板）',
    name VARCHAR(100) NOT NULL COMMENT '模板名称',
    description TEXT COMMENT '模板描述',
    icon VARCHAR(20) DEFAULT '' COMMENT '模板图标',
    sub_type VARCHAR(50) NOT NULL COMMENT '端点类型（img2video/text2video/digital_human/...）',
    default_params JSON COMMENT '默认参数（duration/resolution/...）',
    required_materials JSON COMMENT '必需素材类型（image/video/audio）',
    optional_materials JSON COMMENT '可选素材类型',
    sort_order INT DEFAULT 0 COMMENT '排序',
    enabled BOOLEAN DEFAULT TRUE COMMENT '是否启用',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_tenant_id (tenant_id),
    INDEX idx_sub_type (sub_type),
    INDEX idx_enabled (enabled),
    INDEX idx_sort_order (sort_order)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 插入默认模板（全局模板，tenant_id为空）
INSERT INTO generation_templates (id, tenant_id, name, description, icon, sub_type, default_params, required_materials, optional_materials, sort_order, enabled) VALUES
('brand_promo', '', '品牌宣传视频', '4秒品牌Logo动画视频，适合社交媒体宣传', '🎬', 'img2video', '{"duration":4,"resolution":"720p"}', '["image"]', '[]', 1, TRUE),
('product_intro', '', '产品介绍视频', '8秒产品展示视频，详细展示产品特点', '📦', 'text2video', '{"duration":8,"resolution":"720p"}', '[]', '["image"]', 2, TRUE),
('digital_human', '', '数字人口播', '数字人口播视频，适合产品介绍、客服回复', '🤖', 'digital_human', '{"resolution":"720p"}', '["image"]', '["audio"]', 3, TRUE),
('lip_sync', '', '对口型视频', '真人出镜对口型视频，适合口播内容', '🎤', 'lip_sync', '{}', '["video"]', '["audio"]', 4, TRUE),
('tts_audio', '', '语音合成', '文本转语音，适合旁白、解说', '🔊', 'tts', '{"voice_setting_voice_id":"default"}', '[]', '[]', 5, TRUE)
ON DUPLICATE KEY UPDATE name = VALUES(name);
