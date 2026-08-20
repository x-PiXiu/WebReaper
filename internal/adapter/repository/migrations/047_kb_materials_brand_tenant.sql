-- 047: 知识库品牌化（获客智能体转型）
-- kb_materials 加 brand_id / tenant_id 列——品牌私有素材（商户上传）与行业公共池（admin 采集）共存。
-- brand_id 为空 = 行业公共池（现有行为不变）；非空 = 品牌私有（仅该品牌检索可见）。

ALTER TABLE kb_materials ADD COLUMN brand_id VARCHAR(64) DEFAULT '' NOT NULL COMMENT '品牌私有素材归属（空=行业公共池）';
ALTER TABLE kb_materials ADD COLUMN tenant_id VARCHAR(64) DEFAULT '' NOT NULL COMMENT '租户隔离（品牌私有素材必填）';
ALTER TABLE kb_materials ADD INDEX idx_kb_materials_brand (brand_id);
