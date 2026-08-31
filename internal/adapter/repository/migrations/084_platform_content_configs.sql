-- 084: 平台内容配置表（27号优化——运营可调，替代硬编码DefaultPlatformConfigs）
CREATE TABLE IF NOT EXISTS platform_content_configs (
  platform VARCHAR(32) NOT NULL PRIMARY KEY,
  config_json JSON NOT NULL,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Seed 默认配置
INSERT INTO platform_content_configs (platform, config_json) VALUES
('douyin', '{"max_title_length":30,"max_description_length":2000,"max_tag_count":3,"allow_emoji":true,"emoji_density":0.05,"max_new_lines":5,"require_cta":true,"default_tags":["#推荐","#种草","#好物"],"cta_templates":["\n\n👍 觉得有用点个赞吧","\n\n❤️ 喜欢的话关注我","\n\n💬 评论区见"]}'),
('kuaishou', '{"max_title_length":20,"max_description_length":1500,"max_tag_count":2,"allow_emoji":false,"emoji_density":0,"max_new_lines":3,"require_cta":true,"default_tags":["#推荐","#好物"],"cta_templates":["\n\n觉得有用点个赞吧","\n\n喜欢的话关注我"]}'),
('xiaohongshu', '{"max_title_length":20,"max_description_length":1000,"max_tag_count":0,"allow_emoji":true,"emoji_density":0.15,"max_new_lines":10,"require_cta":false,"default_tags":["#推荐","#种草","#好物"],"cta_templates":[]}'),
('weixin', '{"max_title_length":16,"max_description_length":50000,"max_tag_count":0,"allow_emoji":false,"emoji_density":0,"max_new_lines":0,"require_cta":true,"default_tags":[],"cta_templates":["\n\n觉得有用点个赞吧","\n\n喜欢的话关注我"]}'),
('bilibili', '{"max_title_length":50,"max_description_length":5000,"max_tag_count":3,"allow_emoji":true,"emoji_density":0.1,"max_new_lines":8,"require_cta":false,"default_tags":["#bilibili"],"cta_templates":[]}')
ON DUPLICATE KEY UPDATE config_json = VALUES(config_json);
